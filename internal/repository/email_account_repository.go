package repository

import (
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"database/sql"
	"fmt"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/models"
)

type EmailAccountRepository struct {
	db *sql.DB
}

func NewEmailAccountRepository(db *sql.DB) *EmailAccountRepository {
	return &EmailAccountRepository{db: db}
}

func (r *EmailAccountRepository) Create(account *models.EmailAccount) (int, error) {
	query := database.ConvertPlaceholders(`
		INSERT INTO email_accounts (
			email_address, display_name, smtp_host, smtp_port, smtp_username,
			smtp_password_encrypted, imap_host, imap_port, imap_username,
			imap_password_encrypted, queue_id, is_active, created_at, created_by,
			updated_at, updated_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING id`)

	var id int
	err := r.db.QueryRow(query,
		account.EmailAddress,
		account.DisplayName,
		account.SMTPHost,
		account.SMTPPort,
		account.SMTPUsername,
		account.SMTPPasswordEncrypted,
		account.IMAPHost,
		account.IMAPPort,
		account.IMAPUsername,
		account.IMAPPasswordEncrypted,
		account.QueueID,
		account.IsActive,
		time.Now(),
		account.CreatedBy,
		time.Now(),
		account.UpdatedBy,
	).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf("failed to create email account: %w", err)
	}

	return id, nil
}

func (r *EmailAccountRepository) GetByID(id int) (*models.EmailAccount, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, email_address, display_name, smtp_host, smtp_port, smtp_username,
			smtp_password_encrypted, imap_host, imap_port, imap_username,
			imap_password_encrypted, queue_id, is_active, created_at, created_by,
			updated_at, updated_by
		FROM email_accounts
		WHERE id = $1`)

	account := &models.EmailAccount{}
	err := r.db.QueryRow(query, id).Scan(
		&account.ID,
		&account.EmailAddress,
		&account.DisplayName,
		&account.SMTPHost,
		&account.SMTPPort,
		&account.SMTPUsername,
		&account.SMTPPasswordEncrypted,
		&account.IMAPHost,
		&account.IMAPPort,
		&account.IMAPUsername,
		&account.IMAPPasswordEncrypted,
		&account.QueueID,
		&account.IsActive,
		&account.CreatedAt,
		&account.CreatedBy,
		&account.UpdatedAt,
		&account.UpdatedBy,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("email account not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get email account: %w", err)
	}

	return account, nil
}

func (r *EmailAccountRepository) GetByEmail(email string) (*models.EmailAccount, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, email_address, display_name, smtp_host, smtp_port, smtp_username,
			smtp_password_encrypted, imap_host, imap_port, imap_username,
			imap_password_encrypted, queue_id, is_active, created_at, created_by,
			updated_at, updated_by
		FROM email_accounts
		WHERE email_address = $1`)

	account := &models.EmailAccount{}
	err := r.db.QueryRow(query, email).Scan(
		&account.ID,
		&account.EmailAddress,
		&account.DisplayName,
		&account.SMTPHost,
		&account.SMTPPort,
		&account.SMTPUsername,
		&account.SMTPPasswordEncrypted,
		&account.IMAPHost,
		&account.IMAPPort,
		&account.IMAPUsername,
		&account.IMAPPasswordEncrypted,
		&account.QueueID,
		&account.IsActive,
		&account.CreatedAt,
		&account.CreatedBy,
		&account.UpdatedAt,
		&account.UpdatedBy,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("email account not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get email account: %w", err)
	}

	return account, nil
}

func (r *EmailAccountRepository) GetActiveAccounts() ([]*models.EmailAccount, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, email_address, display_name, smtp_host, smtp_port, smtp_username,
			smtp_password_encrypted, imap_host, imap_port, imap_username,
			imap_password_encrypted, queue_id, is_active, created_at, created_by,
			updated_at, updated_by
		FROM email_accounts
		WHERE is_active = true
		ORDER BY email_address`)

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get active email accounts: %w", err)
	}
	defer rows.Close()

	var accounts []*models.EmailAccount
	for rows.Next() {
		account := &models.EmailAccount{}
		err := rows.Scan(
			&account.ID,
			&account.EmailAddress,
			&account.DisplayName,
			&account.SMTPHost,
			&account.SMTPPort,
			&account.SMTPUsername,
			&account.SMTPPasswordEncrypted,
			&account.IMAPHost,
			&account.IMAPPort,
			&account.IMAPUsername,
			&account.IMAPPasswordEncrypted,
			&account.QueueID,
			&account.IsActive,
			&account.CreatedAt,
			&account.CreatedBy,
			&account.UpdatedAt,
			&account.UpdatedBy,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan email account: %w", err)
		}
		accounts = append(accounts, account)
	}

	return accounts, nil
}

func (r *EmailAccountRepository) Update(account *models.EmailAccount) error {
	query := database.ConvertPlaceholders(`
		UPDATE email_accounts SET
			email_address = $2,
			display_name = $3,
			smtp_host = $4,
			smtp_port = $5,
			smtp_username = $6,
			smtp_password_encrypted = $7,
			imap_host = $8,
			imap_port = $9,
			imap_username = $10,
			imap_password_encrypted = $11,
			queue_id = $12,
			is_active = $13,
			updated_at = $14,
			updated_by = $15
		WHERE id = $1`)

	_, err := r.db.Exec(query,
		account.ID,
		account.EmailAddress,
		account.DisplayName,
		account.SMTPHost,
		account.SMTPPort,
		account.SMTPUsername,
		account.SMTPPasswordEncrypted,
		account.IMAPHost,
		account.IMAPPort,
		account.IMAPUsername,
		account.IMAPPasswordEncrypted,
		account.QueueID,
		account.IsActive,
		time.Now(),
		account.UpdatedBy,
	)

	if err != nil {
		return fmt.Errorf("failed to update email account: %w", err)
	}

	return nil
}

func (r *EmailAccountRepository) Delete(id int) error {
	query := database.ConvertPlaceholders(`DELETE FROM email_accounts WHERE id = $1`)
	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete email account: %w", err)
	}
	return nil
}