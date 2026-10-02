package selfservice

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// errTokenInvalid means a link is unknown, expired, already used, or no longer
// matches the account it was issued for. Callers show one generic message.
var errTokenInvalid = errors.New("token invalid or expired")

// errAccountExists means a registration was completed for an address that
// already has a customer account.
var errAccountExists = errors.New("customer account already exists")

// store holds the SQL for self-service. Tokens are stored as SHA-256 hashes,
// so a database read never yields a usable link.
type store struct {
	db *sql.DB
}

// tokenRow is a valid (unused, unexpired) gk_auth_token row.
type tokenRow struct {
	ID            int64
	UserID        sql.NullInt64
	CustomerLogin sql.NullString
	Email         string
}

// newRawToken returns a 256-bit random token, hex encoded, for use in a link.
func newRawToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// hashToken is what gk_auth_token.token stores for a raw link token.
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func now() time.Time { return time.Now().UTC() }

// findAccounts returns the valid accounts of accountType whose login or email
// matches identifier. Agents keep their email in user_preferences (UserEmail);
// accounts without an email address cannot receive a link and are skipped.
func (s store) findAccounts(ctx context.Context, accountType, identifier string) ([]Account, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if accountType == UserAgent {
		rows, err = s.db.QueryContext(ctx, database.ConvertPlaceholders(`
			SELECT u.id, u.login, u.first_name, u.last_name, up.preferences_value
			FROM users u
			JOIN user_preferences up ON up.user_id = u.id AND up.preferences_key = 'UserEmail'
			WHERE u.valid_id = 1 AND (LOWER(u.login) = LOWER(?) OR up.preferences_value = ?)
			ORDER BY u.id
			LIMIT 5`), identifier, []byte(identifier))
	} else {
		rows, err = s.db.QueryContext(ctx, database.ConvertPlaceholders(`
			SELECT id, login, first_name, last_name, email
			FROM customer_user
			WHERE valid_id = 1 AND (LOWER(login) = LOWER(?) OR LOWER(email) = LOWER(?))
			ORDER BY id
			LIMIT 5`), identifier, identifier)
	}
	if err != nil {
		return nil, fmt.Errorf("find %s accounts: %w", accountType, err)
	}
	defer rows.Close()

	seen := map[int]bool{}
	var out []Account
	for rows.Next() {
		var (
			a     Account
			email []byte
		)
		if err := rows.Scan(&a.ID, &a.Login, &a.FirstName, &a.LastName, &email); err != nil {
			return nil, fmt.Errorf("scan %s account: %w", accountType, err)
		}
		a.Type = accountType
		a.Email = strings.TrimSpace(string(email))
		if a.Email == "" || seen[a.ID] {
			continue
		}
		seen[a.ID] = true
		out = append(out, a)
	}
	return out, rows.Err()
}

// accountForToken reloads the account a reset token was issued for and checks
// it is still valid and still has the email address the link was sent to.
func (s store) accountForToken(ctx context.Context, userType string, tok *tokenRow) (*Account, error) {
	var (
		a     Account
		email []byte
		err   error
	)
	switch {
	case userType == UserAgent && tok.UserID.Valid:
		err = s.db.QueryRowContext(ctx, database.ConvertPlaceholders(`
			SELECT u.id, u.login, u.first_name, u.last_name, up.preferences_value
			FROM users u
			JOIN user_preferences up ON up.user_id = u.id AND up.preferences_key = 'UserEmail'
			WHERE u.id = ? AND u.valid_id = 1`), tok.UserID.Int64).Scan(&a.ID, &a.Login, &a.FirstName, &a.LastName, &email)
	case userType == UserCustomer && tok.CustomerLogin.Valid:
		err = s.db.QueryRowContext(ctx, database.ConvertPlaceholders(`
			SELECT id, login, first_name, last_name, email
			FROM customer_user
			WHERE login = ? AND valid_id = 1`), tok.CustomerLogin.String).Scan(&a.ID, &a.Login, &a.FirstName, &a.LastName, &email)
	default:
		return nil, errTokenInvalid
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errTokenInvalid
	}
	if err != nil {
		return nil, fmt.Errorf("load %s account: %w", userType, err)
	}
	a.Type = userType
	a.Email = strings.TrimSpace(string(email))
	if !strings.EqualFold(a.Email, tok.Email) {
		return nil, errTokenInvalid
	}
	return &a, nil
}

// purgeStaleTokens deletes tokens that expired or were used more than a day ago.
func purgeStaleTokens(ctx context.Context, tx *sql.Tx, t time.Time) error {
	cutoff := t.Add(-24 * time.Hour)
	_, err := tx.ExecContext(ctx, database.ConvertPlaceholders(
		"DELETE FROM gk_auth_token WHERE expires_at < ? OR (used_at IS NOT NULL AND used_at < ?)"), cutoff, cutoff)
	if err != nil {
		return fmt.Errorf("purge stale tokens: %w", err)
	}
	return nil
}

// issueResetToken stores a new reset token for acct, revoking any earlier
// unused reset token for the same account, and returns the raw token.
func (s store) issueResetToken(ctx context.Context, acct Account) (string, error) {
	raw, err := newRawToken()
	if err != nil {
		return "", err
	}
	t := now()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit

	if err := purgeStaleTokens(ctx, tx, t); err != nil {
		return "", err
	}

	var (
		userID        any
		customerLogin any
		revoke        string
		revokeArg     any
	)
	if acct.Type == UserAgent {
		userID = acct.ID
		revoke = "UPDATE gk_auth_token SET used_at = ? WHERE token_type = ? AND user_type = ? AND user_id = ? AND used_at IS NULL"
		revokeArg = acct.ID
	} else {
		customerLogin = acct.Login
		revoke = "UPDATE gk_auth_token SET used_at = ? WHERE token_type = ? AND user_type = ? AND customer_login = ? AND used_at IS NULL"
		revokeArg = acct.Login
	}
	if _, err := tx.ExecContext(ctx, database.ConvertPlaceholders(revoke), t, TokenPasswordReset, acct.Type, revokeArg); err != nil {
		return "", fmt.Errorf("revoke earlier reset tokens: %w", err)
	}
	if _, err := tx.ExecContext(ctx, database.ConvertPlaceholders(`
		INSERT INTO gk_auth_token (token, token_type, user_type, user_id, customer_login, email, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`),
		hashToken(raw), TokenPasswordReset, acct.Type, userID, customerLogin, acct.Email, t.Add(ResetTokenTTL), t,
	); err != nil {
		return "", fmt.Errorf("insert reset token: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit reset token: %w", err)
	}
	return raw, nil
}

// lookupToken returns the unused, unexpired token of the given type and
// account type for raw, or errTokenInvalid.
func (s store) lookupToken(ctx context.Context, raw, tokenType, userType string) (*tokenRow, error) {
	if raw == "" {
		return nil, errTokenInvalid
	}
	var tok tokenRow
	err := s.db.QueryRowContext(ctx, database.ConvertPlaceholders(`
		SELECT id, user_id, customer_login, email
		FROM gk_auth_token
		WHERE token = ? AND token_type = ? AND user_type = ? AND used_at IS NULL AND expires_at > ?`),
		hashToken(raw), tokenType, userType, now(),
	).Scan(&tok.ID, &tok.UserID, &tok.CustomerLogin, &tok.Email)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errTokenInvalid
	}
	if err != nil {
		return nil, fmt.Errorf("lookup token: %w", err)
	}
	return &tok, nil
}

// consumeToken marks the token used inside tx. Only one caller can win.
func consumeToken(ctx context.Context, tx *sql.Tx, id int64, t time.Time) error {
	res, err := tx.ExecContext(ctx, database.ConvertPlaceholders(
		"UPDATE gk_auth_token SET used_at = ? WHERE id = ? AND used_at IS NULL AND expires_at > ?"), t, id, t)
	if err != nil {
		return fmt.Errorf("consume token: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("consume token: %w", err)
	}
	if n != 1 {
		return errTokenInvalid
	}
	return nil
}

// resetPassword consumes the token and stores the new password hash in one
// transaction, so a token sets a password at most once.
func (s store) resetPassword(ctx context.Context, tok *tokenRow, acct *Account, pwHash string) error {
	t := now()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit

	if err := consumeToken(ctx, tx, tok.ID, t); err != nil {
		return err
	}
	var res sql.Result
	if acct.Type == UserAgent {
		// change_by is left alone (as OTRS's SetPassword does): there is no
		// acting user, and change_by = own id would make the row reference
		// itself, which MySQL then refuses to delete.
		res, err = tx.ExecContext(ctx, database.ConvertPlaceholders(
			"UPDATE users SET pw = ?, change_time = ? WHERE id = ? AND valid_id = 1"),
			pwHash, t, acct.ID)
	} else {
		res, err = tx.ExecContext(ctx, database.ConvertPlaceholders(
			"UPDATE customer_user SET pw = ?, change_time = ? WHERE login = ? AND valid_id = 1"),
			pwHash, t, acct.Login)
	}
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil || n != 1 {
		return errTokenInvalid
	}
	return tx.Commit()
}

// killSessions deletes every session of the account so a stolen session does
// not outlive the password it was opened with.
func (s store) killSessions(ctx context.Context, acct *Account) (int, error) {
	rows, err := s.db.QueryContext(ctx, database.ConvertPlaceholders(`
		SELECT DISTINCT l.session_id
		FROM sessions l
		JOIN sessions t ON t.session_id = l.session_id
		WHERE l.data_key = 'UserLogin' AND l.data_value = ?
		  AND t.data_key = 'UserType' AND t.data_value = ?`), acct.Login, acct.sessionUserType())
	if err != nil {
		return 0, fmt.Errorf("find sessions: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return 0, fmt.Errorf("scan session: %w", err)
		}
		ids = append(ids, id)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("find sessions: %w", err)
	}
	for _, id := range ids {
		if _, err := s.db.ExecContext(ctx, database.ConvertPlaceholders("DELETE FROM sessions WHERE session_id = ?"), id); err != nil {
			return 0, fmt.Errorf("delete session: %w", err)
		}
	}
	return len(ids), nil
}

// customerExists reports whether a customer account uses email as login or email.
func customerExists(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, email string) (bool, error) {
	var n int
	err := q.QueryRowContext(ctx, database.ConvertPlaceholders(
		"SELECT COUNT(*) FROM customer_user WHERE LOWER(login) = LOWER(?) OR LOWER(email) = LOWER(?)"), email, email).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("check customer: %w", err)
	}
	return n > 0, nil
}

// createRegistration records a pending sign-up and its confirmation token,
// returning the raw token for the confirmation link.
func (s store) createRegistration(ctx context.Context, email, firstName, lastName string) (string, error) {
	raw, err := newRawToken()
	if err != nil {
		return "", err
	}
	hashed := hashToken(raw)
	t := now()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit

	if err := purgeStaleTokens(ctx, tx, t); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, database.ConvertPlaceholders(
		"DELETE FROM gk_registration_request WHERE status = ? AND created_at < ?"),
		StatusPending, t.Add(-VerifyTokenTTL-24*time.Hour)); err != nil {
		return "", fmt.Errorf("purge stale registrations: %w", err)
	}
	if _, err := tx.ExecContext(ctx, database.ConvertPlaceholders(
		"UPDATE gk_auth_token SET used_at = ? WHERE token_type = ? AND user_type = ? AND email = ? AND used_at IS NULL"),
		t, TokenEmailVerify, UserCustomer, email); err != nil {
		return "", fmt.Errorf("revoke earlier confirmation tokens: %w", err)
	}
	if _, err := tx.ExecContext(ctx, database.ConvertPlaceholders(`
		INSERT INTO gk_registration_request (email, first_name, last_name, status, approval_token, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`),
		email, firstName, lastName, StatusPending, hashed, t); err != nil {
		return "", fmt.Errorf("insert registration: %w", err)
	}
	if _, err := tx.ExecContext(ctx, database.ConvertPlaceholders(`
		INSERT INTO gk_auth_token (token, token_type, user_type, email, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`),
		hashed, TokenEmailVerify, UserCustomer, email, t.Add(VerifyTokenTTL), t); err != nil {
		return "", fmt.Errorf("insert confirmation token: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit registration: %w", err)
	}
	return raw, nil
}

// pendingRegistration is the sign-up a confirmation token belongs to.
type pendingRegistration struct {
	ID        int64
	Email     string
	FirstName string
	LastName  string
}

// registrationForToken returns the pending sign-up for a valid confirmation token.
func (s store) registrationForToken(ctx context.Context, raw string) (*tokenRow, *pendingRegistration, error) {
	tok, err := s.lookupToken(ctx, raw, TokenEmailVerify, UserCustomer)
	if err != nil {
		return nil, nil, err
	}
	var reg pendingRegistration
	err = s.db.QueryRowContext(ctx, database.ConvertPlaceholders(`
		SELECT id, email, first_name, last_name
		FROM gk_registration_request
		WHERE approval_token = ? AND status = ?`), hashToken(raw), StatusPending,
	).Scan(&reg.ID, &reg.Email, &reg.FirstName, &reg.LastName)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, errTokenInvalid
	}
	if err != nil {
		return nil, nil, fmt.Errorf("load registration: %w", err)
	}
	if !strings.EqualFold(reg.Email, tok.Email) {
		return nil, nil, errTokenInvalid
	}
	return tok, &reg, nil
}

// completeRegistration consumes the confirmation token and creates the
// customer account (login = email, customer ID = email, as OTRS's
// CustomerPanelCreateAccount does) in one transaction.
func (s store) completeRegistration(ctx context.Context, tok *tokenRow, reg *pendingRegistration, pwHash string) error {
	t := now()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit

	if err := consumeToken(ctx, tx, tok.ID, t); err != nil {
		return err
	}
	exists, err := customerExists(ctx, tx, reg.Email)
	if err != nil {
		return err
	}
	if exists {
		return errAccountExists
	}
	if _, err := tx.ExecContext(ctx, database.ConvertPlaceholders(`
		INSERT INTO customer_user (login, email, customer_id, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, ?, ?, ?, 1, ?, 1, ?, 1)`),
		reg.Email, reg.Email, reg.Email, pwHash, reg.FirstName, reg.LastName, t, t); err != nil {
		return fmt.Errorf("create customer: %w", err)
	}
	if _, err := tx.ExecContext(ctx, database.ConvertPlaceholders(
		"UPDATE gk_registration_request SET status = ?, approved_at = ? WHERE id = ?"),
		StatusApproved, t, reg.ID); err != nil {
		return fmt.Errorf("close registration: %w", err)
	}
	return tx.Commit()
}
