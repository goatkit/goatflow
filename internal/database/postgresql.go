package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// PostgreSQLDatabase implements IDatabase for PostgreSQL.
type PostgreSQLDatabase struct {
	config DatabaseConfig
	db     *sql.DB
}

// NewPostgreSQLDatabase creates a new PostgreSQL database instance.
func NewPostgreSQLDatabase(config DatabaseConfig) *PostgreSQLDatabase {
	return &PostgreSQLDatabase{
		config: config,
	}
}

// Connect establishes connection to PostgreSQL database.
func (p *PostgreSQLDatabase) Connect() error {
	dsn := p.buildDSN()

	var err error
	p.db, err = sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("failed to open PostgreSQL connection: %w", err)
	}

	// Configure connection pool
	if p.config.MaxOpenConns > 0 {
		p.db.SetMaxOpenConns(p.config.MaxOpenConns)
	}
	if p.config.MaxIdleConns > 0 {
		p.db.SetMaxIdleConns(p.config.MaxIdleConns)
	}
	if p.config.ConnMaxLifetime > 0 {
		p.db.SetConnMaxLifetime(p.config.ConnMaxLifetime)
	}
	if p.config.ConnMaxIdleTime > 0 {
		p.db.SetConnMaxIdleTime(p.config.ConnMaxIdleTime)
	}

	return p.Ping()
}

// Close closes the database connection.
func (p *PostgreSQLDatabase) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

// Ping tests the database connection.
func (p *PostgreSQLDatabase) Ping() error {
	if p.db == nil {
		return fmt.Errorf("database connection not established")
	}
	return p.db.Ping()
}

// GetType returns the database type.
func (p *PostgreSQLDatabase) GetType() DatabaseType {
	return PostgreSQL
}

// GetConfig returns the database configuration.
func (p *PostgreSQLDatabase) GetConfig() DatabaseConfig {
	return p.config
}

// Query executes a query and returns rows.
func (p *PostgreSQLDatabase) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return p.db.QueryContext(ctx, query, args...)
}

// QueryRow executes a query and returns a single row.
func (p *PostgreSQLDatabase) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return p.db.QueryRowContext(ctx, query, args...)
}

// Exec executes a query and returns the result.
func (p *PostgreSQLDatabase) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return p.db.ExecContext(ctx, query, args...)
}

// Begin starts a transaction.
func (p *PostgreSQLDatabase) Begin(ctx context.Context) (ITransaction, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &PostgreSQLTransaction{tx: tx}, nil
}

// BeginTx starts a transaction with options.
func (p *PostgreSQLDatabase) BeginTx(ctx context.Context, opts *sql.TxOptions) (ITransaction, error) {
	tx, err := p.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &PostgreSQLTransaction{tx: tx}, nil
}

// TableExists checks if a table exists.
func (p *PostgreSQLDatabase) TableExists(ctx context.Context, tableName string) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = $1
		)`

	var exists bool
	err := p.db.QueryRowContext(ctx, query, tableName).Scan(&exists)
	return exists, err
}

// GetTableColumns returns column information for a table.
func (p *PostgreSQLDatabase) GetTableColumns(ctx context.Context, tableName string) ([]ColumnInfo, error) {
	query := `
		SELECT 
			c.column_name,
			c.data_type,
			c.is_nullable = 'YES' as is_nullable,
			c.column_default,
			c.character_maximum_length,
			COALESCE(pk.is_primary, false) as is_primary_key,
			CASE WHEN c.column_default LIKE 'nextval%' THEN true ELSE false END as is_auto_increment
		FROM information_schema.columns c
		LEFT JOIN (
			SELECT 
				a.attname as column_name,
				true as is_primary
			FROM pg_index i
			JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
			JOIN pg_class t ON t.oid = i.indrelid
			WHERE i.indisprimary 
			AND t.relname = $1
		) pk ON pk.column_name = c.column_name
		WHERE c.table_schema = 'public' AND c.table_name = $1
		ORDER BY c.ordinal_position`

	rows, err := p.db.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var col ColumnInfo
		var defaultValue sql.NullString
		var maxLength sql.NullInt64

		err := rows.Scan(
			&col.Name,
			&col.DataType,
			&col.IsNullable,
			&defaultValue,
			&maxLength,
			&col.IsPrimaryKey,
			&col.IsAutoIncrement,
		)
		if err != nil {
			return nil, err
		}

		if defaultValue.Valid {
			col.DefaultValue = &defaultValue.String
		}
		if maxLength.Valid {
			length := int(maxLength.Int64)
			col.MaxLength = &length
		}

		columns = append(columns, col)
	}

	return columns, rows.Err()
}

// CreateTable creates a table from definition.
func (p *PostgreSQLDatabase) CreateTable(ctx context.Context, definition *TableDefinition) error {
	sql := p.buildCreateTableSQL(definition)
	_, err := p.db.ExecContext(ctx, sql)
	return err
}

// DropTable drops a table.
func (p *PostgreSQLDatabase) DropTable(ctx context.Context, tableName string) error {
	query := fmt.Sprintf("DROP TABLE IF EXISTS %s", p.Quote(tableName))
	_, err := p.db.ExecContext(ctx, query)
	return err
}

// CreateIndex creates an index.
func (p *PostgreSQLDatabase) CreateIndex(ctx context.Context, tableName, indexName string, columns []string, unique bool) error {
	uniqueClause := ""
	if unique {
		uniqueClause = "UNIQUE "
	}

	quotedColumns := make([]string, len(columns))
	for i, col := range columns {
		quotedColumns[i] = p.Quote(col)
	}

	query := fmt.Sprintf("CREATE %sINDEX %s ON %s (%s)",
		uniqueClause,
		p.Quote(indexName),
		p.Quote(tableName),
		strings.Join(quotedColumns, ", "))

	_, err := p.db.ExecContext(ctx, query)
	return err
}

// DropIndex drops an index.
func (p *PostgreSQLDatabase) DropIndex(ctx context.Context, tableName, indexName string) error {
	query := fmt.Sprintf("DROP INDEX IF EXISTS %s", p.Quote(indexName))
	_, err := p.db.ExecContext(ctx, query)
	return err
}

// Quote quotes an identifier.
func (p *PostgreSQLDatabase) Quote(identifier string) string {
	return fmt.Sprintf(`"%s"`, identifier)
}

// QuoteValue quotes a value.
func (p *PostgreSQLDatabase) QuoteValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return fmt.Sprintf("'%s'", strings.ReplaceAll(v, "'", "''"))
	case nil:
		return "NULL"
	default:
		return fmt.Sprintf("'%v'", v)
	}
}

// BuildInsert builds an INSERT statement.
func (p *PostgreSQLDatabase) BuildInsert(tableName string, data map[string]interface{}) (string, []interface{}) {
	columns := make([]string, 0, len(data))
	placeholders := make([]string, 0, len(data))
	values := make([]interface{}, 0, len(data))

	i := 1
	for col, val := range data {
		columns = append(columns, p.Quote(col))
		placeholders = append(placeholders, fmt.Sprintf("$%d", i))
		values = append(values, val)
		i++
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		p.Quote(tableName),
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "))

	return query, values
}

// BuildUpdate builds an UPDATE statement.
func (p *PostgreSQLDatabase) BuildUpdate(tableName string, data map[string]interface{}, where string, whereArgs []interface{}) (string, []interface{}) {
	setParts := make([]string, 0, len(data))
	values := make([]interface{}, 0, len(data)+len(whereArgs))

	i := 1
	for col, val := range data {
		setParts = append(setParts, fmt.Sprintf("%s = $%d", p.Quote(col), i))
		values = append(values, val)
		i++
	}

	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s",
		p.Quote(tableName),
		strings.Join(setParts, ", "),
		where)

	// Append WHERE clause arguments
	values = append(values, whereArgs...)

	return query, values
}

// BuildSelect builds a SELECT statement.
func (p *PostgreSQLDatabase) BuildSelect(tableName string, columns []string, where string, orderBy string, limit int) string {
	quotedColumns := make([]string, len(columns))
	for i, col := range columns {
		quotedColumns[i] = p.Quote(col)
	}

	query := fmt.Sprintf("SELECT %s FROM %s",
		strings.Join(quotedColumns, ", "),
		p.Quote(tableName))

	if where != "" {
		query += " WHERE " + where
	}

	if orderBy != "" {
		query += " ORDER BY " + orderBy
	}

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	return query
}

// GetLimitClause returns database-specific LIMIT clause.
func (p *PostgreSQLDatabase) GetLimitClause(limit, offset int) string {
	if limit > 0 && offset > 0 {
		return fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)
	} else if limit > 0 {
		return fmt.Sprintf("LIMIT %d", limit)
	} else if offset > 0 {
		return fmt.Sprintf("OFFSET %d", offset)
	}
	return ""
}

// GetDateFunction returns current date function.
func (p *PostgreSQLDatabase) GetDateFunction() string {
	return "NOW()"
}

// GetConcatFunction returns concatenation function.
func (p *PostgreSQLDatabase) GetConcatFunction(fields []string) string {
	return fmt.Sprintf("CONCAT(%s)", strings.Join(fields, ", "))
}

// SupportsReturning returns true if database supports RETURNING clause.
func (p *PostgreSQLDatabase) SupportsReturning() bool {
	return true
}

// Stats returns database connection statistics.
func (p *PostgreSQLDatabase) Stats() sql.DBStats {
	return p.db.Stats()
}

// IsHealthy checks if database is healthy.
func (p *PostgreSQLDatabase) IsHealthy() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return p.db.PingContext(ctx) == nil
}

// buildDSN builds the PostgreSQL connection string.
func (p *PostgreSQLDatabase) buildDSN() string {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		p.config.Host, p.config.Port, p.config.Username, p.config.Password, p.config.Database, p.config.SSLMode)

	// Add additional options
	for key, value := range p.config.Options {
		dsn += fmt.Sprintf(" %s=%s", key, value)
	}

	return dsn
}

// buildCreateTableSQL builds CREATE TABLE SQL for PostgreSQL.
func (p *PostgreSQLDatabase) buildCreateTableSQL(def *TableDefinition) string {
	var parts []string

	// Columns
	for _, col := range def.Columns {
		parts = append(parts, p.buildColumnSQL(col))
	}

	// Primary key constraints
	var pkColumns []string
	for _, col := range def.Columns {
		if col.PrimaryKey {
			pkColumns = append(pkColumns, p.Quote(col.Name))
		}
	}
	if len(pkColumns) > 0 {
		parts = append(parts, fmt.Sprintf("PRIMARY KEY (%s)", strings.Join(pkColumns, ", ")))
	}

	// Other constraints
	for _, constraint := range def.Constraints {
		parts = append(parts, p.buildConstraintSQL(constraint))
	}

	return fmt.Sprintf("CREATE TABLE %s (\n  %s\n)", p.Quote(def.Name), strings.Join(parts, ",\n  "))
}

// buildColumnSQL builds column definition SQL.
func (p *PostgreSQLDatabase) buildColumnSQL(col ColumnDefinition) string {
	var parts []string

	parts = append(parts, p.Quote(col.Name))

	// Data type
	dataType := col.DataType
	if col.Size != nil {
		dataType += fmt.Sprintf("(%d)", *col.Size)
	} else if col.Precision != nil && col.Scale != nil {
		dataType += fmt.Sprintf("(%d,%d)", *col.Precision, *col.Scale)
	} else if col.Precision != nil {
		dataType += fmt.Sprintf("(%d)", *col.Precision)
	}
	parts = append(parts, dataType)

	// NOT NULL
	if col.NotNull {
		parts = append(parts, "NOT NULL")
	}

	// DEFAULT
	if col.DefaultValue != nil {
		parts = append(parts, fmt.Sprintf("DEFAULT %s", *col.DefaultValue))
	}

	return strings.Join(parts, " ")
}

// buildConstraintSQL builds constraint SQL.
func (p *PostgreSQLDatabase) buildConstraintSQL(constraint ConstraintDefinition) string {
	switch constraint.Type {
	case "FOREIGN_KEY":
		fk := fmt.Sprintf("CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (%s)",
			p.Quote(constraint.Name),
			strings.Join(constraint.Columns, ", "),
			*constraint.ReferenceTable,
			strings.Join(constraint.ReferenceColumns, ", "))

		if constraint.OnDelete != nil {
			fk += fmt.Sprintf(" ON DELETE %s", *constraint.OnDelete)
		}
		if constraint.OnUpdate != nil {
			fk += fmt.Sprintf(" ON UPDATE %s", *constraint.OnUpdate)
		}

		return fk
	case "UNIQUE":
		return fmt.Sprintf("CONSTRAINT %s UNIQUE (%s)",
			p.Quote(constraint.Name),
			strings.Join(constraint.Columns, ", "))
	default:
		return ""
	}
}

// PostgreSQLTransaction implements ITransaction for PostgreSQL.
type PostgreSQLTransaction struct {
	tx *sql.Tx
}

func (t *PostgreSQLTransaction) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return t.tx.QueryContext(ctx, query, args...)
}

func (t *PostgreSQLTransaction) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return t.tx.QueryRowContext(ctx, query, args...)
}

func (t *PostgreSQLTransaction) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return t.tx.ExecContext(ctx, query, args...)
}

func (t *PostgreSQLTransaction) Commit() error {
	return t.tx.Commit()
}

func (t *PostgreSQLTransaction) Rollback() error {
	return t.tx.Rollback()
}
