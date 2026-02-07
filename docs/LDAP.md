# LDAP/Active Directory Integration

GoatFlow provides comprehensive LDAP and Active Directory integration for enterprise authentication and user management.

## Features

- **Multi-server support** with connection pooling and failover
- **Flexible attribute mapping** supporting both Active Directory and generic LDAP
- **Real-time authentication** with caching and performance optimization
- **Background synchronization** with configurable intervals
- **Role-based access control** through LDAP group membership mapping
- **Comprehensive audit logging** for all authentication attempts
- **Bulk import capabilities** with dry-run testing
- **Connection testing and validation** with detailed error reporting
- **User/group mapping management** with automatic provisioning/deprovisioning
- **TLS/StartTLS encryption** support for secure connections

## Quick Start

### 1. Development Setup with OpenLDAP

GoatFlow includes a pre-configured OpenLDAP container for development and testing:

```bash
# Start the full stack including OpenLDAP
make up

# Or start with tools (includes phpLDAPadmin)
docker-compose --profile tools up -d
```

The development LDAP server includes:
- **Domain**: `goatflow.local`
- **Base DN**: `dc=goatflow,dc=local`
- **Admin User**: `cn=admin,dc=goatflow,dc=local` (password: set `LDAP_ADMIN_PASSWORD` in `.env`)
- **Readonly User**: `cn=readonly,dc=goatflow,dc=local` (password: set `LDAP_READONLY_PASSWORD` in `.env`)
- **phpLDAPadmin**: http://localhost:8091 (for browsing LDAP data)

### 2. Test Users

The development LDAP server comes with sample users:

| Username | Email | Role | Department | Groups |
|----------|-------|------|------------|--------|
| `jadmin` | john.admin@goatflow.local | System Administrator | IT | Domain Admins, IT Team, Users |
| `smitchell` | sarah.mitchell@goatflow.local | IT Manager | IT | IT Team, Agents, Managers, Users |
| `mwilson` | mike.wilson@goatflow.local | Senior Support Agent | Support | Support Team, Agents, Managers, Users |
| `lchen` | lisa.chen@goatflow.local | Support Agent | Support | Support Team, Agents, Users |
| `djohnson` | david.johnson@goatflow.local | Junior Support Agent | Support | Support Team, Agents, Users |
| `arodriguez` | alex.rodriguez@goatflow.local | Senior Developer | IT | IT Team, Developers, Users |
| `ethompson` | emma.thompson@goatflow.local | QA Engineer | IT | IT Team, QA Team, Users |
| `rtaylor` | robert.taylor@contractor.goatflow.local | Contractor | IT | Developers, Users |
| `jdavis` | jennifer.davis@goatflow.local | Sales Manager | Sales | Managers, Users |
| `canderson` | chris.anderson@goatflow.local | Customer Success Manager | Support | Support Team, Agents, Managers, Users |

All test users use the password: `password123`

### 3. Configuration

Configure LDAP integration via environment variables or the API:

```bash
# Environment variables (.env file)
LDAP_HOST=openldap
LDAP_PORT=389
LDAP_BASE_DN=dc=goatflow,dc=local
LDAP_BIND_DN=cn=readonly,dc=goatflow,dc=local
LDAP_BIND_PASSWORD=readonly123

# User search configuration
LDAP_USER_SEARCH_BASE=ou=Users,dc=goatflow,dc=local
LDAP_USER_FILTER=(&(objectClass=inetOrgPerson)(uid={username}))

# Group search configuration
LDAP_GROUP_SEARCH_BASE=ou=Groups,dc=goatflow,dc=local
LDAP_GROUP_FILTER=(objectClass=groupOfNames)

# Attribute mapping
LDAP_ATTR_USERNAME=uid
LDAP_ATTR_EMAIL=mail
LDAP_ATTR_FIRST_NAME=givenName
LDAP_ATTR_LAST_NAME=sn
LDAP_ATTR_DISPLAY_NAME=displayName

# Synchronization
LDAP_AUTO_CREATE_USERS=true
LDAP_AUTO_UPDATE_USERS=true
LDAP_SYNC_INTERVAL=1h

# Role mapping
LDAP_ADMIN_GROUPS=Domain Admins,IT Team
LDAP_AGENT_GROUPS=Agents,Support Team,Managers
LDAP_USER_GROUPS=Users
```

## API Usage

### Configure LDAP

```bash
curl -X POST http://localhost:8080/api/v1/ldap/configure \
  -H "Content-Type: application/json" \
  -d '{
    "host": "openldap",
    "port": 389,
    "base_dn": "dc=goatflow,dc=local",
    "bind_dn": "cn=readonly,dc=goatflow,dc=local",
    "bind_password": "readonly123",
    "user_search_base": "ou=Users,dc=goatflow,dc=local",
    "user_filter": "(&(objectClass=inetOrgPerson)(uid={username}))",
    "auto_create_users": true,
    "auto_update_users": true,
    "attribute_map": {
      "username": "uid",
      "email": "mail",
      "first_name": "givenName",
      "last_name": "sn",
      "display_name": "displayName"
    }
  }'
```

### Test Connection

```bash
curl -X POST http://localhost:8080/api/v1/ldap/test \
  -H "Content-Type: application/json" \
  -d '{
    "host": "openldap",
    "port": 389,
    "base_dn": "dc=goatflow,dc=local",
    "bind_dn": "cn=readonly,dc=goatflow,dc=local",
    "bind_password": "readonly123"
  }'
```

### Authenticate User

```bash
curl -X POST http://localhost:8080/api/v1/ldap/authenticate \
  -H "Content-Type: application/json" \
  -d '{
    "username": "jadmin",
    "password": "password123"
  }'
```

### Get User Information

```bash
curl http://localhost:8080/api/v1/ldap/users/jadmin
```

### Synchronize Users

```bash
curl -X POST http://localhost:8080/api/v1/ldap/sync/users
```

### Import Specific Users

```bash
curl -X POST http://localhost:8080/api/v1/ldap/import/users \
  -H "Content-Type: application/json" \
  -d '{
    "usernames": ["jadmin", "smitchell", "mwilson"],
    "dry_run": false
  }'
```

## Testing

### Unit Tests

```bash
# Run unit tests
make toolbox-exec ARGS="go test ./internal/service -v -run TestLDAPService"
```

### Integration Tests

Integration tests require a running OpenLDAP server:

```bash
# Start OpenLDAP container
make up

# Run integration tests
LDAP_INTEGRATION_TESTS=true make toolbox-exec ARGS="go test ./internal/service -v -run TestLDAPIntegration"

# Run with race detection
LDAP_INTEGRATION_TESTS=true make toolbox-exec ARGS="go test ./internal/service -v -race -run TestLDAPIntegration"

# Run benchmarks
LDAP_INTEGRATION_TESTS=true make toolbox-exec ARGS="go test ./internal/service -v -bench=BenchmarkLDAP"
```

### Test Coverage

```bash
# Generate coverage report
LDAP_INTEGRATION_TESTS=true make toolbox-exec ARGS="go test ./internal/service -coverprofile=coverage.out"
make toolbox-exec ARGS="go tool cover -html=coverage.out"
```

## Production Configuration

### Active Directory

```env
# Active Directory configuration
LDAP_HOST=ad.company.com
LDAP_PORT=389
LDAP_BASE_DN=dc=company,dc=com
LDAP_BIND_DN=cn=goatflow-service,ou=Service Accounts,dc=company,dc=com
LDAP_BIND_PASSWORD=secure-service-password

# User search
LDAP_USER_SEARCH_BASE=ou=Users,dc=company,dc=com
LDAP_USER_FILTER=(&(objectClass=user)(sAMAccountName={username}))

# Group search
LDAP_GROUP_SEARCH_BASE=ou=Groups,dc=company,dc=com
LDAP_GROUP_FILTER=(objectClass=group)

# Active Directory attribute mapping
LDAP_ATTR_USERNAME=sAMAccountName
LDAP_ATTR_EMAIL=mail
LDAP_ATTR_FIRST_NAME=givenName
LDAP_ATTR_LAST_NAME=sn
LDAP_ATTR_DISPLAY_NAME=displayName
LDAP_ATTR_GROUPS=memberOf

# Security
LDAP_USE_TLS=true
LDAP_START_TLS=false
LDAP_INSECURE_SKIP_VERIFY=false

# Role mapping based on AD groups
LDAP_ADMIN_GROUPS=Domain Admins,GoatFlow Administrators
LDAP_AGENT_GROUPS=GoatFlow Agents,Help Desk,Support Team
LDAP_USER_GROUPS=Domain Users
```

### OpenLDAP/Generic LDAP

```env
# Generic LDAP configuration
LDAP_HOST=ldap.company.com
LDAP_PORT=389
LDAP_BASE_DN=dc=company,dc=com
LDAP_BIND_DN=cn=goatflow-bind,ou=System,dc=company,dc=com
LDAP_BIND_PASSWORD=secure-bind-password

# User search
LDAP_USER_SEARCH_BASE=ou=People,dc=company,dc=com
LDAP_USER_FILTER=(&(objectClass=inetOrgPerson)(uid={username}))

# Group search
LDAP_GROUP_SEARCH_BASE=ou=Groups,dc=company,dc=com
LDAP_GROUP_FILTER=(objectClass=groupOfNames)

# Standard LDAP attribute mapping
LDAP_ATTR_USERNAME=uid
LDAP_ATTR_EMAIL=mail
LDAP_ATTR_FIRST_NAME=givenName
LDAP_ATTR_LAST_NAME=sn
LDAP_ATTR_DISPLAY_NAME=displayName
LDAP_ATTR_GROUPS=memberOf

# Role mapping
LDAP_ADMIN_GROUPS=admins,system-administrators
LDAP_AGENT_GROUPS=support,helpdesk,agents
LDAP_USER_GROUPS=users,employees
```

## Security Considerations

### Connection Security

- **Always use TLS/StartTLS** in production environments
- **Validate certificates** (set `LDAP_INSECURE_SKIP_VERIFY=false`)
- **Use dedicated service accounts** with minimal required privileges
- **Rotate service account passwords** regularly

### Access Control

- **Principle of least privilege**: Service account should only have read access
- **Network security**: Restrict LDAP server access to GoatFlow servers only
- **Audit logging**: Monitor all LDAP authentication attempts
- **Account lockout**: Implement account lockout policies

### Data Protection

- **Encrypt sensitive configuration** values in production
- **Secure credential storage** using vault systems
- **Regular security audits** of LDAP integration
- **Compliance requirements** (GDPR, HIPAA, SOX)

## Monitoring and Troubleshooting

### Health Checks

The LDAP service provides health check endpoints:

```bash
# Check LDAP configuration status
curl http://localhost:8080/api/v1/ldap/config

# Check sync status
curl http://localhost:8080/api/v1/ldap/sync/status

# View authentication logs
curl http://localhost:8080/api/v1/ldap/logs/auth?limit=50
```

### Common Issues

#### Connection Timeouts

```bash
# Test network connectivity
telnet ldap.company.com 389

# Check DNS resolution
nslookup ldap.company.com

# Verify firewall rules
curl -v telnet://ldap.company.com:389
```

#### Authentication Failures

```bash
# Test with ldapsearch
ldapsearch -x -H ldap://ldap.company.com \
  -D "cn=service,dc=company,dc=com" \
  -w "password" \
  -b "dc=company,dc=com" \
  "(uid=testuser)"

# Check user DN format
ldapsearch -x -H ldap://ldap.company.com \
  -D "cn=service,dc=company,dc=com" \
  -w "password" \
  -b "ou=Users,dc=company,dc=com" \
  "(uid=testuser)" dn
```

#### Certificate Issues

```bash
# Test TLS connection
openssl s_client -connect ldap.company.com:636 -showcerts

# Verify certificate chain
openssl verify -CAfile /path/to/ca.crt /path/to/ldap.crt
```

### Logging

Enable detailed LDAP logging:

```env
LOG_LEVEL=debug
LDAP_DEBUG=true
```

Log files will contain:
- Connection attempts and results
- Authentication successes/failures
- Sync operations and results
- Performance metrics
- Error details and stack traces

## Performance Optimization

### Connection Pooling

```go
// Configure connection pool settings
config := &LDAPConfig{
    MaxConnections: 10,
    MaxIdleConnections: 5,
    ConnectionTimeout: 30 * time.Second,
    IdleTimeout: 5 * time.Minute,
}
```

### Caching

- **User authentication cache**: 5 minutes default
- **Group membership cache**: 15 minutes default
- **User attribute cache**: 10 minutes default

### Sync Optimization

```env
# Optimize sync performance
LDAP_SYNC_INTERVAL=2h           # Reduce sync frequency
LDAP_SYNC_BATCH_SIZE=100        # Process in batches
LDAP_SYNC_PARALLEL_WORKERS=3    # Parallel processing
```

## Advanced Configuration

### Custom Attribute Mapping

```json
{
  "attribute_map": {
    "username": "sAMAccountName",
    "email": "mail",
    "first_name": "givenName", 
    "last_name": "sn",
    "display_name": "displayName",
    "phone": "telephoneNumber",
    "department": "department",
    "title": "title",
    "manager": "manager",
    "employee_id": "employeeNumber",
    "groups": "memberOf",
    "object_guid": "objectGUID",
    "object_sid": "objectSid"
  }
}
```

### Multi-Domain Support

```env
# Primary domain
LDAP_HOST=dc1.company.com
LDAP_BASE_DN=dc=company,dc=com

# Additional domains
LDAP_SECONDARY_HOSTS=dc2.company.com,dc3.company.com
LDAP_FAILOVER_ENABLED=true
LDAP_FAILOVER_TIMEOUT=10s
```

### Group-Based Role Mapping

```json
{
  "role_mappings": [
    {
      "ldap_groups": ["Domain Admins", "GoatFlow Administrators"],
      "goatflow_role": "admin",
      "priority": 1
    },
    {
      "ldap_groups": ["Support Team", "Help Desk"],
      "goatflow_role": "agent", 
      "priority": 2
    },
    {
      "ldap_groups": ["Domain Users", "Employees"],
      "goatflow_role": "user",
      "priority": 3
    }
  ]
}
```

## Migration Guide

### From Local Authentication

1. **Phase 1**: Configure LDAP alongside existing authentication
2. **Phase 2**: Import existing users and map to LDAP accounts
3. **Phase 3**: Switch authentication method to LDAP
4. **Phase 4**: Disable local authentication (optional)

### From Other LDAP Solutions

1. **Export user mappings** from existing system
2. **Configure GoatFlow LDAP** with same/similar schema
3. **Import user mappings** using bulk import API
4. **Test authentication** with sample users
5. **Switch over** during maintenance window

## Support

For LDAP integration support:

- **Documentation**: Check this guide and API documentation
- **Logs**: Enable debug logging for detailed troubleshooting
- **Testing**: Use provided integration tests to validate setup
- **Community**: Ask questions in GoatFlow community forums
- **Enterprise**: Contact support for enterprise LDAP assistance