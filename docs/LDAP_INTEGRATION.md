# LDAP/Active Directory Integration

GOTRS supports enterprise authentication through LDAP and Active Directory integration. This allows users to authenticate using their existing directory credentials and automatically maps groups to GOTRS roles.

## Features

- **Multiple LDAP Server Support**: Active Directory, OpenLDAP, 389 Directory Server
- **Secure Authentication**: TLS/SSL support with certificate validation
- **Group-based Role Mapping**: Automatic role assignment based on LDAP groups
- **Fallback Authentication**: Optional local authentication when LDAP is unavailable
- **User Synchronization**: Automatic user profile updates from LDAP
- **Connection Health Monitoring**: Built-in connection testing and monitoring

## Configuration

### Environment Variables

Configure LDAP through environment variables in your `.env` file:

```bash
# Enable LDAP authentication
LDAP_ENABLED=true
LDAP_TYPE=active_directory  # or openldap, 389ds

# Connection settings
LDAP_HOST=dc.company.com
LDAP_PORT=389
LDAP_USE_TLS=true
LDAP_SKIP_TLS_VERIFY=false  # Set to true for self-signed certificates
LDAP_TIMEOUT=30

# Bind credentials (service account)
LDAP_BIND_DN=cn=gotrs-service,ou=Service Accounts,dc=company,dc=com
LDAP_BIND_PASSWORD=your-bind-password

# Search settings
LDAP_BASE_DN=dc=company,dc=com
LDAP_USER_FILTER=(sAMAccountName=%s)  # AD format
LDAP_GROUP_BASE_DN=ou=Groups,dc=company,dc=com
LDAP_GROUP_FILTER=(&(objectClass=group)(member=%s))

# Attribute mappings
LDAP_EMAIL_ATTRIBUTE=mail
LDAP_FIRST_NAME_ATTRIBUTE=givenName
LDAP_LAST_NAME_ATTRIBUTE=sn
LDAP_DISPLAY_NAME_ATTRIBUTE=displayName
LDAP_GROUP_ATTRIBUTE=cn

# Role mappings
LDAP_ADMIN_GROUPS=Domain Admins,GOTRS Administrators
LDAP_AGENT_GROUPS=Support Team,IT Helpdesk

# Active Directory specific
LDAP_IS_ACTIVE_DIRECTORY=true
LDAP_DOMAIN=company.com
```

### Configuration Templates

GOTRS includes pre-configured templates for common LDAP servers:

#### Active Directory
```bash
LDAP_TYPE=active_directory
LDAP_USER_FILTER=(sAMAccountName=%s)
LDAP_GROUP_FILTER=(&(objectClass=group)(member=%s))
LDAP_IS_ACTIVE_DIRECTORY=true
```

#### OpenLDAP
```bash
LDAP_TYPE=openldap
LDAP_USER_FILTER=(uid=%s)
LDAP_GROUP_FILTER=(&(objectClass=groupOfNames)(member=%s))
```

#### 389 Directory Server
```bash
LDAP_TYPE=389ds
LDAP_USER_FILTER=(uid=%s)
LDAP_GROUP_FILTER=(&(objectClass=groupOfUniqueNames)(uniqueMember=%s))
```

## API Endpoints

### Configuration Management

- `GET /api/v1/admin/ldap/config` - Get current LDAP configuration
- `POST /api/v1/admin/ldap/config` - Set LDAP configuration
- `PUT /api/v1/admin/ldap/config` - Update LDAP configuration

### Connection Testing

- `POST /api/v1/admin/ldap/test` - Test LDAP connection
- `POST /api/v1/admin/ldap/test-auth` - Test authentication with credentials
- `GET /api/v1/admin/ldap/health` - Check LDAP health status

### User Management

- `GET /api/v1/admin/ldap/users/:username` - Get user info from LDAP
- `POST /api/v1/admin/ldap/users/:username/sync` - Synchronize user from LDAP
- `GET /api/v1/admin/ldap/users/:username/groups` - Get user groups

### Templates and Documentation

- `GET /api/v1/admin/ldap/templates` - List available configuration templates
- `GET /api/v1/admin/ldap/templates/:type` - Get specific template
- `GET /api/v1/admin/ldap/stats` - Get LDAP usage statistics

## Role Mapping

GOTRS maps LDAP groups to three roles:

### Admin Role
- Full system access
- User management
- System configuration
- Maps to groups in `LDAP_ADMIN_GROUPS`

### Agent Role
- Ticket management
- Customer interaction
- Queue management
- Maps to groups in `LDAP_AGENT_GROUPS`

### Customer Role
- Default role for authenticated users
- Limited to own tickets
- Cannot access admin functions

## Security Features

### Connection Security
- TLS encryption support
- Certificate validation (can be disabled for testing)
- Connection timeout protection
- Service account authentication

### Authentication Security
- Password never stored locally
- HMAC signature verification for API calls
- JWT token generation after successful LDAP auth
- Optional fallback to local authentication

### LDAP Injection Prevention
- Automatic LDAP filter escaping
- Input validation and sanitization
- Safe DN construction

## Troubleshooting

### Connection Issues

1. **Test connectivity**:
```bash
curl -X POST http://localhost:8080/api/v1/admin/ldap/test \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>"
```

2. **Check TLS settings**:
- Ensure `LDAP_USE_TLS=true` for StartTLS
- Set `LDAP_SKIP_TLS_VERIFY=true` for self-signed certificates
- Use `LDAP_USE_SSL=true` for LDAPS (port 636)

3. **Verify service account**:
- Test bind DN and password
- Ensure service account has read permissions
- Check account is not expired or disabled

### Authentication Issues

1. **Test user authentication**:
```bash
curl -X POST http://localhost:8080/api/v1/admin/ldap/test-auth \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","password":"testpass"}' \
  -H "Authorization: Bearer <token>"
```

2. **Check user filter**:
- Verify `LDAP_USER_FILTER` matches your schema
- Test with different username formats
- Check base DN permissions

3. **Verify group memberships**:
```bash
curl -X GET http://localhost:8080/api/v1/admin/ldap/users/testuser/groups \
  -H "Authorization: Bearer <token>"
```

### Common Error Messages

| Error | Cause | Solution |
|-------|-------|----------|
| "Connection failed" | Network/firewall issues | Check host, port, firewall rules |
| "TLS handshake failed" | Certificate issues | Check TLS settings, certificates |
| "Bind failed" | Service account issues | Verify bind DN and password |
| "User not found" | Filter/DN issues | Check user filter and base DN |
| "Invalid credentials" | User password wrong | User should check password |
| "No groups found" | Group configuration | Check group base DN and filter |

## Performance Considerations

### Connection Pooling
GOTRS creates new connections for each authentication to avoid connection state issues. For high-volume environments, consider:

- Dedicated LDAP read replicas
- Load balancers for LDAP servers
- Connection timeout optimization

### Caching
User information is not cached by default. For improved performance:

- Enable local user synchronization
- Use JWT token expiration for session management
- Consider read-only LDAP replicas

### Monitoring
Monitor LDAP integration through:

- `/api/v1/admin/ldap/health` endpoint
- Authentication success/failure rates
- Connection response times
- Error logs in application logs

## Advanced Configuration

### Multiple Domains (AD)
For multi-domain Active Directory environments:

```bash
LDAP_USER_FILTER=(|(sAMAccountName=%s)(userPrincipalName=%s@domain1.com)(userPrincipalName=%s@domain2.com))
```

### Custom Attributes
Map custom LDAP attributes to user fields:

```bash
LDAP_DISPLAY_NAME_ATTRIBUTE=cn
LDAP_DEPARTMENT_ATTRIBUTE=department
LDAP_TITLE_ATTRIBUTE=title
```

### Group Nesting
For nested group support (AD):

```bash
LDAP_GROUP_FILTER=(&(objectClass=group)(member:1.2.840.113556.1.4.1941:=%s))
```

### SSL/TLS Configuration
For LDAPS (port 636):

```bash
LDAP_PORT=636
LDAP_USE_SSL=true
LDAP_USE_TLS=false
```

## Integration Examples

### Docker Compose
```yaml
services:
  gotrs:
    environment:
      - LDAP_ENABLED=true
      - LDAP_HOST=dc.company.com
      - LDAP_BIND_DN=cn=gotrs,cn=Users,dc=company,dc=com
      - LDAP_BIND_PASSWORD=${LDAP_PASSWORD}
      - LDAP_BASE_DN=dc=company,dc=com
    secrets:
      - ldap_password
```

### Kubernetes
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: ldap-config
data:
  bind-password: <base64-encoded-password>
---
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: gotrs
        env:
        - name: LDAP_BIND_PASSWORD
          valueFrom:
            secretKeyRef:
              name: ldap-config
              key: bind-password
```

## Migration from Other Systems

### From OTRS
OTRS LDAP configurations can be migrated:

1. Export OTRS LDAP settings
2. Map to GOTRS environment variables
3. Test configuration before go-live
4. Verify user role mappings

### From Email Authentication
For systems moving from email-only auth:

1. Configure LDAP alongside existing auth
2. Enable fallback authentication
3. Migrate users gradually
4. Disable email auth after verification

## Best Practices

1. **Use service accounts**: Never use personal accounts for LDAP binding
2. **Limit permissions**: Service account needs only read access
3. **Monitor health**: Regularly check LDAP connection status
4. **Test changes**: Always test configuration changes in non-production
5. **Backup configs**: Store LDAP configuration in version control
6. **Rotate passwords**: Regularly rotate service account passwords
7. **Use TLS**: Always encrypt LDAP traffic in production
8. **Group naming**: Use consistent group naming conventions