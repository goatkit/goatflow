#!/bin/bash
#
# Generate k8s/secrets.yaml from template with secure random values
# This runs alongside the synthesize command to generate Kubernetes secrets
#

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if template exists
if [ ! -f "k8s/secrets.yaml.template" ]; then
    echo "Error: k8s/secrets.yaml.template not found"
    exit 1
fi

# Function to generate secure random password
generate_password() {
    local length=${1:-32}
    openssl rand -base64 $length | tr -d '\n'
}

# Function to base64 encode a string
base64_encode() {
    echo -n "$1" | base64 | tr -d '\n'
}

echo "🔐 Generating Kubernetes secrets..."

# Generate secure random values
DB_PASSWORD=$(generate_password 24)
JWT_SECRET=$(generate_password 32)
JWT_REFRESH_SECRET=$(generate_password 32)
REDIS_PASSWORD=$(generate_password 20)
EMAIL_PASSWORD=$(generate_password 20)
ZINC_PASSWORD=$(generate_password 20)
SESSION_SECRET=$(generate_password 32)
ENCRYPTION_KEY=$(generate_password 32)
POSTGRES_PASSWORD=$DB_PASSWORD  # Same as DB_PASSWORD
POSTGRES_REPLICATION_PASSWORD=$(generate_password 24)

# Base64 encode all values
DB_PASSWORD_BASE64=$(base64_encode "$DB_PASSWORD")
JWT_SECRET_BASE64=$(base64_encode "$JWT_SECRET")
JWT_REFRESH_SECRET_BASE64=$(base64_encode "$JWT_REFRESH_SECRET")
REDIS_PASSWORD_BASE64=$(base64_encode "$REDIS_PASSWORD")
EMAIL_PASSWORD_BASE64=$(base64_encode "$EMAIL_PASSWORD")
ZINC_PASSWORD_BASE64=$(base64_encode "$ZINC_PASSWORD")
SESSION_SECRET_BASE64=$(base64_encode "$SESSION_SECRET")
ENCRYPTION_KEY_BASE64=$(base64_encode "$ENCRYPTION_KEY")
POSTGRES_USER_BASE64=$(base64_encode "gotrs")
POSTGRES_PASSWORD_BASE64=$(base64_encode "$POSTGRES_PASSWORD")
POSTGRES_REPLICATION_PASSWORD_BASE64=$(base64_encode "$POSTGRES_REPLICATION_PASSWORD")

# Create k8s/secrets.yaml from template
cp k8s/secrets.yaml.template k8s/secrets.yaml

# Replace placeholders with actual base64 encoded values
sed -i "s|{{.DB_PASSWORD_BASE64}}|$DB_PASSWORD_BASE64|g" k8s/secrets.yaml
sed -i "s|{{.JWT_SECRET_BASE64}}|$JWT_SECRET_BASE64|g" k8s/secrets.yaml
sed -i "s|{{.JWT_REFRESH_SECRET_BASE64}}|$JWT_REFRESH_SECRET_BASE64|g" k8s/secrets.yaml
sed -i "s|{{.REDIS_PASSWORD_BASE64}}|$REDIS_PASSWORD_BASE64|g" k8s/secrets.yaml
sed -i "s|{{.EMAIL_PASSWORD_BASE64}}|$EMAIL_PASSWORD_BASE64|g" k8s/secrets.yaml
sed -i "s|{{.ZINC_PASSWORD_BASE64}}|$ZINC_PASSWORD_BASE64|g" k8s/secrets.yaml
sed -i "s|{{.SESSION_SECRET_BASE64}}|$SESSION_SECRET_BASE64|g" k8s/secrets.yaml
sed -i "s|{{.ENCRYPTION_KEY_BASE64}}|$ENCRYPTION_KEY_BASE64|g" k8s/secrets.yaml
sed -i "s|{{.POSTGRES_USER_BASE64}}|$POSTGRES_USER_BASE64|g" k8s/secrets.yaml
sed -i "s|{{.POSTGRES_PASSWORD_BASE64}}|$POSTGRES_PASSWORD_BASE64|g" k8s/secrets.yaml
sed -i "s|{{.POSTGRES_REPLICATION_PASSWORD_BASE64}}|$POSTGRES_REPLICATION_PASSWORD_BASE64|g" k8s/secrets.yaml

echo -e "${GREEN}✅ Generated k8s/secrets.yaml with secure random values${NC}"
echo -e "${YELLOW}💡 Remember: k8s/secrets.yaml is git-ignored for security${NC}"
echo ""
echo "To deploy to Kubernetes:"
echo "  kubectl apply -f k8s/secrets.yaml"
echo ""
echo "To rotate secrets:"
echo "  make k8s-secrets"