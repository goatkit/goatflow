package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/database"

	"github.com/gotrs-io/gotrs-ce/internal/models"
)

type EmailTemplateRepository struct {
	db *sql.DB
}

func NewEmailTemplateRepository(db *sql.DB) *EmailTemplateRepository {
	return &EmailTemplateRepository{db: db}
}

func (r *EmailTemplateRepository) Create(template *models.EmailTemplate) (int, error) {
	query := database.ConvertPlaceholders(`
		INSERT INTO email_templates (
			template_name, subject_template, body_template, template_type,
			is_active, created_at, created_by, updated_at, updated_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id`)

	var id int
	err := r.db.QueryRow(query,
		template.TemplateName,
		template.SubjectTemplate,
		template.BodyTemplate,
		template.TemplateType,
		template.IsActive,
		time.Now(),
		template.CreatedBy,
		time.Now(),
		template.UpdatedBy,
	).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf("failed to create email template: %w", err)
	}

	return id, nil
}

func (r *EmailTemplateRepository) GetByID(id int) (*models.EmailTemplate, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, template_name, subject_template, body_template, template_type,
			is_active, created_at, created_by, updated_at, updated_by
		FROM email_templates
		WHERE id = ?`)

	template := &models.EmailTemplate{}
	err := r.db.QueryRow(query, id).Scan(
		&template.ID,
		&template.TemplateName,
		&template.SubjectTemplate,
		&template.BodyTemplate,
		&template.TemplateType,
		&template.IsActive,
		&template.CreatedAt,
		&template.CreatedBy,
		&template.UpdatedAt,
		&template.UpdatedBy,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("email template not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get email template: %w", err)
	}

	return template, nil
}

func (r *EmailTemplateRepository) GetByName(name string) (*models.EmailTemplate, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, template_name, subject_template, body_template, template_type,
			is_active, created_at, created_by, updated_at, updated_by
		FROM email_templates
		WHERE template_name = ? AND is_active = true`)

	template := &models.EmailTemplate{}
	err := r.db.QueryRow(query, name).Scan(
		&template.ID,
		&template.TemplateName,
		&template.SubjectTemplate,
		&template.BodyTemplate,
		&template.TemplateType,
		&template.IsActive,
		&template.CreatedAt,
		&template.CreatedBy,
		&template.UpdatedAt,
		&template.UpdatedBy,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("email template not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get email template: %w", err)
	}

	return template, nil
}

func (r *EmailTemplateRepository) GetByType(templateType string) ([]*models.EmailTemplate, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, template_name, subject_template, body_template, template_type,
			is_active, created_at, created_by, updated_at, updated_by
		FROM email_templates
		WHERE template_type = ? AND is_active = true
		ORDER BY template_name`)

	rows, err := r.db.Query(query, templateType)
	if err != nil {
		return nil, fmt.Errorf("failed to get templates by type: %w", err)
	}
	defer rows.Close()

	var templates []*models.EmailTemplate
	for rows.Next() {
		template := &models.EmailTemplate{}
		err := rows.Scan(
			&template.ID,
			&template.TemplateName,
			&template.SubjectTemplate,
			&template.BodyTemplate,
			&template.TemplateType,
			&template.IsActive,
			&template.CreatedAt,
			&template.CreatedBy,
			&template.UpdatedAt,
			&template.UpdatedBy,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan email template: %w", err)
		}
		templates = append(templates, template)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating templates by type: %w", err)
	}

	return templates, nil
}

func (r *EmailTemplateRepository) GetAll() ([]*models.EmailTemplate, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, template_name, subject_template, body_template, template_type,
			is_active, created_at, created_by, updated_at, updated_by
		FROM email_templates
		ORDER BY template_name`)

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get all templates: %w", err)
	}
	defer rows.Close()

	var templates []*models.EmailTemplate
	for rows.Next() {
		template := &models.EmailTemplate{}
		err := rows.Scan(
			&template.ID,
			&template.TemplateName,
			&template.SubjectTemplate,
			&template.BodyTemplate,
			&template.TemplateType,
			&template.IsActive,
			&template.CreatedAt,
			&template.CreatedBy,
			&template.UpdatedAt,
			&template.UpdatedBy,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan email template: %w", err)
		}
		templates = append(templates, template)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating all templates: %w", err)
	}

	return templates, nil
}

func (r *EmailTemplateRepository) Update(template *models.EmailTemplate) error {
	query := database.ConvertPlaceholders(`
		UPDATE email_templates SET
			template_name = ?,
			subject_template = ?,
			body_template = ?,
			template_type = ?,
			is_active = ?,
			updated_at = ?,
			updated_by = ?
		WHERE id = ?`)

	_, err := r.db.Exec(query,
		template.ID,
		template.TemplateName,
		template.SubjectTemplate,
		template.BodyTemplate,
		template.TemplateType,
		template.IsActive,
		time.Now(),
		template.UpdatedBy,
	)

	if err != nil {
		return fmt.Errorf("failed to update email template: %w", err)
	}

	return nil
}

func (r *EmailTemplateRepository) Delete(id int) error {
	query := database.ConvertPlaceholders(`DELETE FROM email_templates WHERE id = ?`)
	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete email template: %w", err)
	}
	return nil
}
