package repository

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/mailaccountmeta"
	"github.com/goatkit/goatflow/internal/models"
)

const mailAccountSelect = `
	SELECT id, login, pw, host, account_type, queue_id, trusted,
	       imap_folder, comments, valid_id, create_time, create_by,
	       change_time, change_by
	FROM mail_account`

type EmailAccountRepository struct {
	db *sql.DB
}

func NewEmailAccountRepository(db *sql.DB) *EmailAccountRepository {
	return &EmailAccountRepository{db: db}
}

func (r *EmailAccountRepository) Create(account *models.EmailAccount) (int, error) {
	if account == nil {
		return 0, fmt.Errorf("email account is nil")
	}
	acct := normalizeAccount(account)
	if err := validateEmailAccount(acct); err != nil {
		return 0, err
	}

	now := time.Now().UTC()
	queueID := queueIDForDispatch(acct)
	trusted := trustedToSmallInt(acct)
	validID := defaultValidID(acct)
	imapFolder := stringValue(acct.IMAPFolder)
	commentValue := commentSQLValue(acct)
	createdBy := actorID(acct.CreatedBy, 1)
	updatedBy := actorID(acct.UpdatedBy, createdBy)

	query := database.ConvertPlaceholders(`
		INSERT INTO mail_account (
			login, pw, host, account_type, queue_id, trusted,
			imap_folder, comments, valid_id, create_time, create_by,
			change_time, change_by
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
		RETURNING id`)

	var id int
	err := r.db.QueryRow(query,
		acct.Login,
		acct.PasswordEncrypted,
		acct.Host,
		acct.AccountType,
		queueID,
		trusted,
		imapFolder,
		commentValue,
		validID,
		now,
		createdBy,
		now,
		updatedBy,
	).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf("failed to create email account: %w", err)
	}

	return id, nil
}

func (r *EmailAccountRepository) GetByID(id int) (*models.EmailAccount, error) {
	query := database.ConvertPlaceholders(mailAccountSelect + " WHERE id = ?")
	account, err := scanEmailAccount(r.db.QueryRow(query, id))
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("email account not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get email account: %w", err)
	}
	return account, nil
}

// GetByLogin returns an account by login (mailbox username).
func (r *EmailAccountRepository) GetByLogin(login string) (*models.EmailAccount, error) {
	query := database.ConvertPlaceholders(mailAccountSelect + " WHERE login = ?")
	account, err := scanEmailAccount(r.db.QueryRow(query, login))
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("email account not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get email account: %w", err)
	}
	return account, nil
}

// GetByEmail exists for backward compatibility; it delegates to GetByLogin.
func (r *EmailAccountRepository) GetByEmail(email string) (*models.EmailAccount, error) {
	return r.GetByLogin(email)
}

func (r *EmailAccountRepository) GetActiveAccounts() ([]*models.EmailAccount, error) {
	query := database.ConvertPlaceholders(mailAccountSelect + " WHERE valid_id = 1 ORDER BY login")
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get active email accounts: %w", err)
	}
	defer rows.Close()

	var accounts []*models.EmailAccount
	for rows.Next() {
		account, scanErr := scanEmailAccount(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("failed to scan email account: %w", scanErr)
		}
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating email accounts: %w", err)
	}

	return accounts, nil
}

func (r *EmailAccountRepository) Update(account *models.EmailAccount) error {
	if account == nil {
		return fmt.Errorf("email account is nil")
	}
	acct := normalizeAccount(account)
	if err := validateEmailAccount(acct); err != nil {
		return err
	}

	now := time.Now().UTC()
	queueID := queueIDForDispatch(acct)
	trusted := trustedToSmallInt(acct)
	validID := defaultValidID(acct)
	imapFolder := stringValue(acct.IMAPFolder)
	commentValue := commentSQLValue(acct)
	updatedBy := actorID(acct.UpdatedBy, acct.CreatedBy)

	query := database.ConvertPlaceholders(`
		UPDATE mail_account SET
			login = ?,
			pw = ?,
			host = ?,
			account_type = ?,
			queue_id = ?,
			trusted = ?,
			imap_folder = ?,
			comments = ?,
			valid_id = ?,
			change_time = ?,
			change_by = ?
		WHERE id = ?`)

	_, err := r.db.Exec(query,
		acct.ID,
		acct.Login,
		acct.PasswordEncrypted,
		acct.Host,
		acct.AccountType,
		queueID,
		trusted,
		imapFolder,
		commentValue,
		validID,
		now,
		updatedBy,
	)

	if err != nil {
		return fmt.Errorf("failed to update email account: %w", err)
	}

	return nil
}

func (r *EmailAccountRepository) Delete(id int) error {
	query := database.ConvertPlaceholders(`DELETE FROM mail_account WHERE id = ?`)
	if _, err := r.db.Exec(query, id); err != nil {
		return fmt.Errorf("failed to delete email account: %w", err)
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanEmailAccount(scanner rowScanner) (*models.EmailAccount, error) {
	var (
		account             models.EmailAccount
		trusted             sql.NullInt64
		imapFolder, comment sql.NullString
	)

	err := scanner.Scan(
		&account.ID,
		&account.Login,
		&account.PasswordEncrypted,
		&account.Host,
		&account.AccountType,
		&account.QueueID,
		&trusted,
		&imapFolder,
		&comment,
		&account.ValidID,
		&account.CreatedAt,
		&account.CreatedBy,
		&account.UpdatedAt,
		&account.UpdatedBy,
	)
	if err != nil {
		return nil, err
	}

	account.AccountType = normalizeAccountType(account.AccountType)
	account.DispatchingMode = dispatchingModeFromQueue(account.QueueID)
	account.Trusted = trusted.Int64 != 0
	account.AllowTrustedHeaders = account.Trusted
	account.IsActive = account.ValidID == 1
	if imapFolder.Valid {
		folder := imapFolder.String
		account.IMAPFolder = &folder
	}
	if comment.Valid {
		baseComment, meta := mailaccountmeta.DecodeComment(comment.String)
		if baseComment != "" {
			account.Comments = stringPtr(baseComment)
		}
		applyMailAccountMetadata(&account, meta)
	}
	return &account, nil
}

func dispatchingModeFromQueue(queueID int) string {
	if queueID == 0 {
		return "from"
	}
	return "queue"
}

func queueIDForDispatch(account *models.EmailAccount) int {
	if account == nil {
		return 0
	}
	if strings.EqualFold(account.DispatchingMode, "from") {
		return 0
	}
	return account.QueueID
}

func trustedToSmallInt(account *models.EmailAccount) int {
	if account == nil {
		return 0
	}
	if account.Trusted || account.AllowTrustedHeaders {
		return 1
	}
	return 0
}

func normalizeAccount(account *models.EmailAccount) *models.EmailAccount {
	if account == nil {
		return &models.EmailAccount{}
	}
	account.AccountType = normalizeAccountType(account.AccountType)
	if strings.EqualFold(account.DispatchingMode, "from") {
		account.DispatchingMode = "from"
	}
	if account.DispatchingMode == "" {
		account.DispatchingMode = "queue"
	}
	if account.Trusted {
		account.AllowTrustedHeaders = true
	}
	if account.AllowTrustedHeaders {
		account.Trusted = true
	}
	if account.PollIntervalSeconds < 0 {
		account.PollIntervalSeconds = 0
	}
	return account
}

func normalizeAccountType(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "POP3"
	}
	return strings.ToUpper(trimmed)
}

func validateEmailAccount(account *models.EmailAccount) error {
	if account == nil {
		return fmt.Errorf("email account is nil")
	}
	if strings.TrimSpace(account.Login) == "" {
		return fmt.Errorf("mail account login is required")
	}
	if strings.TrimSpace(account.PasswordEncrypted) == "" {
		return fmt.Errorf("mail account password is required")
	}
	if strings.TrimSpace(account.Host) == "" {
		return fmt.Errorf("mail account host is required")
	}
	if strings.TrimSpace(account.AccountType) == "" {
		return fmt.Errorf("mail account type is required")
	}
	if !strings.EqualFold(account.DispatchingMode, "from") && account.QueueID == 0 {
		return fmt.Errorf("queue_id required when dispatching mode is queue")
	}
	return nil
}

func stringValue(ptr *string) interface{} {
	if ptr == nil {
		return nil
	}
	return *ptr
}

func commentSQLValue(account *models.EmailAccount) interface{} {
	if account == nil {
		return nil
	}
	baseComment := stringFromPtr(account.Comments)
	meta := metadataFromAccount(account)
	serialized := mailaccountmeta.EncodeComment(baseComment, meta)
	if serialized == "" {
		return nil
	}
	return serialized
}

func stringFromPtr(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	v := value
	return &v
}

func defaultValidID(account *models.EmailAccount) int {
	if account == nil {
		return 1
	}
	if account.ValidID > 0 {
		return account.ValidID
	}
	if account.IsActive {
		return 1
	}
	return 2
}

func actorID(primary, secondary int) int {
	if primary > 0 {
		return primary
	}
	if secondary > 0 {
		return secondary
	}
	return 1
}

func metadataFromAccount(account *models.EmailAccount) mailaccountmeta.Metadata {
	var meta mailaccountmeta.Metadata
	if account == nil {
		return meta
	}
	mode := strings.ToLower(strings.TrimSpace(account.DispatchingMode))
	if mode != "" && mode != "queue" {
		meta.DispatchingMode = mode
	}
	if account.AllowTrustedHeaders {
		allow := true
		meta.AllowTrustedHeaders = &allow
	}
	if account.PollIntervalSeconds > 0 {
		poll := account.PollIntervalSeconds
		meta.PollIntervalSeconds = &poll
	}
	return meta
}

func applyMailAccountMetadata(account *models.EmailAccount, meta mailaccountmeta.Metadata) {
	if account == nil {
		return
	}
	if meta.DispatchingMode != "" {
		account.DispatchingMode = meta.DispatchingMode
	}
	if meta.AllowTrustedHeaders != nil {
		account.AllowTrustedHeaders = *meta.AllowTrustedHeaders
		if *meta.AllowTrustedHeaders {
			account.Trusted = true
		}
	}
	if meta.PollIntervalSeconds != nil {
		account.PollIntervalSeconds = *meta.PollIntervalSeconds
	}
}
