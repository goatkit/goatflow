#!/bin/bash
# Extract data (INSERT statements) from OTRS MySQL dump for import

INPUT_FILE="/home/nigel/git/goatkit/otrs_sqldump_ng20250825_1732.sql"
OUTPUT_FILE="schema/import/otrs_data.sql"

echo "Extracting data from OTRS dump..."

# Create import directory if it doesn't exist
mkdir -p schema/import

# Extract only INSERT statements, converting MySQL to PostgreSQL syntax
grep "^INSERT INTO" "$INPUT_FILE" | \
sed 's/`//g' | \
sed "s/\\\\'/\'\'/g" | \
head -1000 > "$OUTPUT_FILE"

LINE_COUNT=$(wc -l < "$OUTPUT_FILE")
echo "Extracted $LINE_COUNT INSERT statements to $OUTPUT_FILE"

# Show sample of what tables have data
echo ""
echo "Tables with data:"
grep "^INSERT INTO" "$OUTPUT_FILE" | sed 's/INSERT INTO //' | sed 's/ .*//' | sort -u | head -20