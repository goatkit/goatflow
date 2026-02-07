#!/bin/bash
# Extract clean schema from OTRS MySQL dump and convert to PostgreSQL

INPUT_FILE="/home/nigel/git/goatkit/otrs_sqldump_ng20250825_1732.sql"
OUTPUT_MYSQL="schema/baseline/otrs_mysql_structure.sql"
OUTPUT_PG="schema/baseline/otrs_complete.sql"

echo "Extracting MySQL structure from OTRS dump..."

# Extract CREATE TABLE statements and their content
awk '
/^CREATE TABLE/ {
    in_table = 1
}
in_table {
    print
}
/^\) ENGINE=/ && in_table {
    print ""
    in_table = 0
}
' "$INPUT_FILE" > "$OUTPUT_MYSQL"

echo "Converting MySQL to PostgreSQL..."

# Basic MySQL to PostgreSQL conversion
cat "$OUTPUT_MYSQL" | \
sed 's/`//g' | \
sed 's/ENGINE=InnoDB.*$/;/' | \
sed 's/ AUTO_INCREMENT/ SERIAL/' | \
sed 's/ int([0-9]*) NOT NULL AUTO_INCREMENT/ SERIAL PRIMARY KEY/' | \
sed 's/ bigint([0-9]*) NOT NULL AUTO_INCREMENT/ BIGSERIAL PRIMARY KEY/' | \
sed 's/ int([0-9]*)/ INTEGER/g' | \
sed 's/ bigint([0-9]*)/ BIGINT/g' | \
sed 's/ smallint([0-9]*)/ SMALLINT/g' | \
sed 's/ tinyint([0-9]*)/ SMALLINT/g' | \
sed 's/ varchar(/ VARCHAR(/g' | \
sed 's/ datetime/ TIMESTAMP/g' | \
sed 's/ longtext/ TEXT/g' | \
sed 's/ mediumtext/ TEXT/g' | \
sed 's/ longblob/ BYTEA/g' | \
sed 's/ DEFAULT CHARSET=utf8//' | \
sed 's/UNIQUE KEY/UNIQUE INDEX/g' | \
sed 's/KEY /INDEX /g' | \
sed '/^INDEX FK_/d' | \
sed '/^CONSTRAINT FK_/d' > "$OUTPUT_PG"

# Count tables
TABLE_COUNT=$(grep -c "^CREATE TABLE" "$OUTPUT_PG")

echo "Extracted $TABLE_COUNT tables"
echo "MySQL structure saved to: $OUTPUT_MYSQL"
echo "PostgreSQL structure saved to: $OUTPUT_PG"

# List tables for verification
echo ""
echo "Tables extracted:"
grep "^CREATE TABLE" "$OUTPUT_PG" | sed 's/CREATE TABLE /  - /' | sed 's/ (//' | head -20
echo "  ... and $(($TABLE_COUNT - 20)) more"