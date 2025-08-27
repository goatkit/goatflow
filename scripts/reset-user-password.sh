#!/bin/bash
# Reset GOTRS user password and ensure account is enabled
# Usage: ./scripts/reset-user-password.sh <username> <password>

set -e

USERNAME="$1"
PASSWORD="$2"

if [ -z "$USERNAME" ] || [ -z "$PASSWORD" ]; then
    echo "Usage: $0 <username> <password>"
    echo "Example: $0 root@localhost admin123"
    exit 1
fi

echo "🔑 Resetting password for user: $USERNAME"

# Generate bcrypt hash for the password using a temporary file
TEMP_FILE=$(mktemp --suffix=.go)
cat > "$TEMP_FILE" <<'EOF'
package main

import (
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"log"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Password required as argument")
	}
	password := os.Args[1]
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s", hash)
}
EOF

HASH=$(go run "$TEMP_FILE" "$PASSWORD")
rm -f "$TEMP_FILE"

if [ -z "$HASH" ]; then
    echo "❌ Failed to generate password hash"
    exit 1
fi

echo "✅ Generated password hash"

# Load environment variables from .env file
if [ -f .env ]; then
    export $(grep -v '^#' .env | xargs)
fi

# Set default values if not in .env
DB_USER=${DB_USER:-gotrs_user}
DB_NAME=${DB_NAME:-gotrs}

# Use the container wrapper to execute SQL
echo "🔄 Updating user password and status..."

SQL="UPDATE users SET 
  pw = '$HASH',
  valid_id = 1,
  change_time = CURRENT_TIMESTAMP,
  change_by = 1
WHERE login = '$USERNAME';"

echo "$SQL" | ./scripts/container-wrapper.sh exec -i gotrs-postgres psql -h localhost -U "$DB_USER" -d "$DB_NAME"

# Verify the update
RESULT=$(echo "SELECT login, CASE WHEN pw IS NOT NULL THEN 'SET' ELSE 'NULL' END as password_status, valid_id FROM users WHERE login = '$USERNAME';" | \
    ./scripts/container-wrapper.sh exec -i gotrs-postgres psql -h localhost -U "$DB_USER" -d "$DB_NAME" -t)

if echo "$RESULT" | grep -q "SET.*1"; then
    echo "✅ Password reset successful!"
    echo "   Username: $USERNAME"
    echo "   Password: $PASSWORD"
    echo "   Status: Enabled (valid_id=1)"
    echo ""
    echo "🌐 You can now log in at: http://localhost"
else
    echo "❌ Password reset may have failed. User status:"
    echo "$RESULT"
    exit 1
fi