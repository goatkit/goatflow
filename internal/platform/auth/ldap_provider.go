package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/ldap"
	"github.com/goatkit/goatflow/internal/platform/lookups"
	platformmodels "github.com/goatkit/goatflow/internal/platform/models"
)

// ldapAccountCreator is the user id recorded as create_by for agent accounts
// created on first LDAP login: the OTRS system user (root@localhost).
const ldapAccountCreator = 1

// adminGroupName is the GoatFlow group whose membership makes an agent an
// administrator (see AuthService.checkAdminGroup).
const adminGroupName = "admin"

// LDAPAuthProvider authenticates agents against an LDAP / Active Directory
// server and maps them to GoatFlow agent accounts (OTRS Auth::Module LDAP with
// optional AuthSyncModule-style account creation and attribute sync).
type LDAPAuthProvider struct {
	client   *ldap.Client
	cfg      *ldap.Config
	db       *sql.DB
	userRepo UserLookup
}

// NewLDAPAuthProvider creates an LDAP provider for a validated configuration.
func NewLDAPAuthProvider(cfg *ldap.Config, db *sql.DB, userRepo UserLookup) (*LDAPAuthProvider, error) {
	if db == nil || userRepo == nil {
		return nil, errors.New("ldap auth provider needs the database and user repository")
	}
	client, err := ldap.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return &LDAPAuthProvider{client: client, cfg: cfg, db: db, userRepo: userRepo}, nil
}

// Authenticate verifies the password with the directory and returns the
// matching GoatFlow agent. A login with no directory entry returns
// ErrUserNotFound so the next provider in Auth::Providers is tried.
func (p *LDAPAuthProvider) Authenticate(ctx context.Context, username, password string) (*platformmodels.User, error) {
	du, err := p.client.Authenticate(ctx, username, password)
	switch {
	case err == nil:
	case errors.Is(err, ldap.ErrUserNotFound):
		return nil, ErrUserNotFound
	case errors.Is(err, ldap.ErrInvalidCredentials):
		return nil, ErrInvalidCredentials
	case errors.Is(err, ldap.ErrNotAuthorized), errors.Is(err, ldap.ErrAmbiguousUser):
		log.Printf("ldap: login %q refused: %v", username, err)
		return nil, ErrInvalidCredentials
	default:
		log.Printf("ldap: login %q failed: %v", username, err)
		return nil, ErrAuthBackendFailed
	}

	userID, err := p.ensureAgent(ctx, du)
	if err != nil {
		if !errors.Is(err, ErrUserNotFound) && !errors.Is(err, ErrUserDisabled) {
			log.Printf("ldap: agent account for %q: %v", du.Username, err)
			err = ErrAuthBackendFailed
		}
		return nil, err
	}
	if len(p.cfg.AdminGroups) > 0 {
		if err := p.syncAdminGroup(ctx, userID, p.client.IsAdmin(du)); err != nil {
			log.Printf("ldap: admin group sync for %q: %v", du.Username, err)
			return nil, ErrAuthBackendFailed
		}
	}

	user, err := p.userRepo.GetByLogin(du.Username)
	if err != nil {
		log.Printf("ldap: load agent %q: %v", du.Username, err)
		return nil, ErrAuthBackendFailed
	}
	user.Password = ""
	return user, nil
}

// ensureAgent returns the id of the agent whose login is the directory
// username, creating or updating the account when configured to.
func (p *LDAPAuthProvider) ensureAgent(ctx context.Context, du *ldap.User) (int64, error) {
	validID, err := lookups.ID(ctx, p.db, lookups.ValidLookup, "valid")
	if err != nil {
		return 0, err
	}
	first, last := agentNames(du)

	var id int64
	var rowValid int
	err = p.db.QueryRowContext(ctx, database.ConvertPlaceholders(
		"SELECT id, valid_id FROM users WHERE login = ?"), du.Username).Scan(&id, &rowValid)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if !p.cfg.AutoCreateUsers {
			log.Printf("ldap: %q authenticated but has no GoatFlow agent account and LDAP_AUTO_CREATE_USERS is off", du.Username)
			return 0, ErrUserNotFound
		}
		return p.createAgent(ctx, du, first, last, validID)
	case err != nil:
		return 0, err
	case rowValid != validID:
		return 0, ErrUserDisabled
	}

	if p.cfg.AutoUpdateUsers {
		if err := p.updateAgent(ctx, id, du.Email, first, last); err != nil {
			return 0, err
		}
	}
	return id, nil
}

func (p *LDAPAuthProvider) createAgent(ctx context.Context, du *ldap.User, first, last string, validID int) (int64, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now()
	// pw is empty: these accounts can only log in through LDAP.
	id, err := database.GetAdapter().InsertWithReturningTx(tx, database.ConvertPlaceholders(`
		INSERT INTO users (login, pw, title, first_name, last_name, valid_id,
			create_time, create_by, change_time, change_by)
		VALUES (?, '', '', ?, ?, ?, ?, ?, ?, ?) RETURNING id`),
		du.Username, first, last, validID, now, ldapAccountCreator, now, ldapAccountCreator)
	if err != nil {
		return 0, fmt.Errorf("create agent: %w", err)
	}
	if du.Email != "" {
		if _, err := tx.ExecContext(ctx, database.ConvertPlaceholders(
			"INSERT INTO user_preferences (user_id, preferences_key, preferences_value) VALUES (?, 'UserEmail', ?)"),
			id, []byte(du.Email)); err != nil {
			return 0, fmt.Errorf("store email: %w", err)
		}
	}
	for _, name := range p.cfg.InitialGroups {
		var gid int64
		err := tx.QueryRowContext(ctx, database.ConvertPlaceholders(
			"SELECT id FROM `groups` WHERE name = ? AND valid_id = ?"), name, validID).Scan(&gid)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("LDAP_INITIAL_GROUPS: group %q does not exist or is invalid", name)
		}
		if err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, database.ConvertPlaceholders(`
			INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
			VALUES (?, ?, 'rw', ?, ?, ?, ?)`), id, gid, now, ldapAccountCreator, now, ldapAccountCreator); err != nil {
			return 0, fmt.Errorf("add to group %q: %w", name, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	log.Printf("ldap: created agent account %q (id %d) on first login", du.Username, id)
	return id, nil
}

func (p *LDAPAuthProvider) updateAgent(ctx context.Context, id int64, email, first, last string) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, database.ConvertPlaceholders(
		"UPDATE users SET first_name = ?, last_name = ?, change_time = ?, change_by = ? WHERE id = ?"),
		first, last, time.Now(), id, id); err != nil {
		return fmt.Errorf("update agent names: %w", err)
	}
	if email != "" {
		if _, err := tx.ExecContext(ctx, database.ConvertPlaceholders(
			"DELETE FROM user_preferences WHERE user_id = ? AND preferences_key = 'UserEmail'"), id); err != nil {
			return fmt.Errorf("update email: %w", err)
		}
		if _, err := tx.ExecContext(ctx, database.ConvertPlaceholders(
			"INSERT INTO user_preferences (user_id, preferences_key, preferences_value) VALUES (?, 'UserEmail', ?)"),
			id, []byte(email)); err != nil {
			return fmt.Errorf("update email: %w", err)
		}
	}
	return tx.Commit()
}

// syncAdminGroup makes membership of the GoatFlow admin group follow
// membership of LDAP_ADMIN_GROUPS.
func (p *LDAPAuthProvider) syncAdminGroup(ctx context.Context, userID int64, isAdmin bool) error {
	var gid int64
	if err := p.db.QueryRowContext(ctx, database.ConvertPlaceholders(
		"SELECT id FROM `groups` WHERE name = ?"), adminGroupName).Scan(&gid); err != nil {
		return fmt.Errorf("find group %q: %w", adminGroupName, err)
	}
	if !isAdmin {
		_, err := p.db.ExecContext(ctx, database.ConvertPlaceholders(
			"DELETE FROM group_user WHERE user_id = ? AND group_id = ?"), userID, gid)
		return err
	}
	var n int
	if err := p.db.QueryRowContext(ctx, database.ConvertPlaceholders(
		"SELECT COUNT(*) FROM group_user WHERE user_id = ? AND group_id = ? AND permission_key = 'rw'"),
		userID, gid).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	now := time.Now()
	_, err := p.db.ExecContext(ctx, database.ConvertPlaceholders(`
		INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
		VALUES (?, ?, 'rw', ?, ?, ?, ?)`), userID, gid, now, ldapAccountCreator, now, ldapAccountCreator)
	return err
}

// agentNames picks first/last name from the directory attributes, falling
// back to the display name and finally the login (first_name is NOT NULL).
func agentNames(du *ldap.User) (first, last string) {
	first, last = du.FirstName, du.LastName
	if first == "" && last == "" && du.DisplayName != "" {
		if i := strings.LastIndex(du.DisplayName, " "); i > 0 {
			first, last = du.DisplayName[:i], du.DisplayName[i+1:]
		} else {
			first = du.DisplayName
		}
	}
	if first == "" {
		first = du.Username
	}
	return truncateRunes(first, 100), truncateRunes(last, 100)
}

func truncateRunes(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// GetUser returns the GoatFlow agent with the given login.
func (p *LDAPAuthProvider) GetUser(ctx context.Context, identifier string) (*platformmodels.User, error) {
	user, err := p.userRepo.GetByLogin(identifier)
	if err != nil {
		return nil, ErrUserNotFound
	}
	user.Password = ""
	return user, nil
}

// ValidateToken is not supported: sessions are JWTs issued after login.
func (p *LDAPAuthProvider) ValidateToken(ctx context.Context, token string) (*platformmodels.User, error) {
	return nil, ErrAuthBackendFailed
}

// Name returns the name of this auth provider.
func (p *LDAPAuthProvider) Name() string {
	return "LDAP"
}

// Priority returns the priority of this provider.
func (p *LDAPAuthProvider) Priority() int {
	return 5
}

// LDAPStartupCheck validates the LDAP settings when LDAP_ENABLED is true and
// logs transport-security warnings. It does not contact the server, so a
// directory outage never blocks startup.
func LDAPStartupCheck() error {
	if !ldap.Enabled() {
		return nil
	}
	cfg, err := ldap.LoadFromEnvironment()
	if err != nil {
		return err
	}
	client, err := ldap.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("invalid LDAP configuration: %w", err)
	}
	client.WarnInsecure()
	return nil
}

func init() {
	_ = RegisterProvider("ldap", func(deps ProviderDependencies) (AuthProvider, error) {
		if !ldap.Enabled() {
			return nil, errors.New("ldap is listed in Auth::Providers but LDAP_ENABLED is not true")
		}
		cfg, err := ldap.LoadFromEnvironment()
		if err != nil {
			return nil, err
		}
		userRepo := deps.UserRepo
		if userRepo == nil && deps.DB != nil {
			userRepo = getUserRepo(deps.DB)
		}
		return NewLDAPAuthProvider(cfg, deps.DB, userRepo)
	})
}
