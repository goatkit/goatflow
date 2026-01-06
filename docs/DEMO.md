# GOTRS Demo Instance Specification

## Overview

The GOTRS demo instance provides a fully functional environment for evaluation, testing, and training purposes. It features realistic data, simulated activity, and comprehensive showcasing of all features.

## Public Demo Instances

### Primary Instance
- **URL**: https://try.gotrs.io
- **Region**: US East (Virginia)
- **Reset Schedule**: Daily at 2:00 AM UTC
- **Uptime Target**: 99.9%

### Regional Instances
- **EU**: https://demo-eu.gotrs.io (Frankfurt)
- **APAC**: https://demo-ap.gotrs.io (Singapore)
- **Partner**: https://partner.gotrs.io (Restricted access)

## Demo Accounts

### Quick Access Credentials

Demo credentials are configured via environment variables when DEMO_MODE=true.
See `.env.example` for configuration details.

| Role | Description |
|------|-------------|
| **Admin** | Full system administration |
| **Senior Agent** | Experienced support agent |
| **Junior Agent** | New support agent |
| **Team Lead** | Team management access |
| **Customer** | VIP customer account |
| **Basic User** | Standard customer |

Note: Actual demo usernames and passwords are provided on the demo instance login page when demo mode is enabled.

### Test Credit Cards
For testing payment/billing features:
- Visa: 4242 4242 4242 4242
- MasterCard: 5555 5555 5555 4444
- Amex: 3782 822463 10005
- All cards: Any future date, any CVC

## Demo Data Structure

### Organizations

#### TechCorp Solutions (Enterprise)
- **Industry**: Software Development
- **Agents**: 15
- **Customers**: 250
- **Tickets/Month**: ~500
- **SLA**: Premium (1 hour response)
- **Features**: All modules enabled

#### Global Retail Inc (Large Business)
- **Industry**: E-commerce
- **Agents**: 8
- **Customers**: 500
- **Tickets/Month**: ~300
- **SLA**: Standard (4 hour response)
- **Features**: Standard modules

#### City Medical Center (Healthcare)
- **Industry**: Healthcare
- **Agents**: 5
- **Customers**: 100
- **Tickets/Month**: ~150
- **SLA**: Premium (HIPAA compliant)
- **Features**: Compliance modules

#### EduTech Academy (Education)
- **Industry**: Education
- **Agents**: 3
- **Customers**: 150
- **Tickets/Month**: ~100
- **SLA**: Basic (24 hour response)
- **Features**: Basic modules

### Ticket Distribution

```yaml
Status Distribution:
  New: 15%        # ~75 tickets
  Open: 35%       # ~175 tickets
  Pending: 20%    # ~100 tickets
  Resolved: 25%   # ~125 tickets
  Closed: 5%      # ~25 tickets

Priority Distribution:
  Critical: 5%    # ~25 tickets
  High: 20%       # ~100 tickets
  Normal: 60%     # ~300 tickets
  Low: 15%        # ~75 tickets

Category Distribution:
  Technical Support: 40%
  Billing: 20%
  Feature Request: 15%
  Bug Report: 15%
  General Inquiry: 10%

Age Distribution:
  < 1 hour: 5%
  < 24 hours: 20%
  < 1 week: 30%
  < 1 month: 25%
  > 1 month: 20%
```

### Sample Ticket Scenarios

#### Scenario 1: Critical Production Issue
- **Title**: "API Gateway returning 503 errors"
- **Customer**: Enterprise customer
- **Priority**: Critical
- **SLA**: 1 hour response
- **Thread**: 8 responses, escalated, resolved
- **Attachments**: Error logs, screenshots

#### Scenario 2: Billing Dispute
- **Title**: "Incorrect invoice amount for last month"
- **Customer**: Standard customer
- **Priority**: High
- **Thread**: 5 responses, refund processed
- **Attachments**: Invoice PDF, payment records

#### Scenario 3: Feature Request
- **Title**: "Request for bulk export functionality"
- **Customer**: VIP customer
- **Priority**: Normal
- **Thread**: 3 responses, added to roadmap
- **Tags**: feature-request, export, planned

#### Scenario 4: Password Reset
- **Title**: "Unable to reset password"
- **Customer**: Basic user
- **Priority**: Normal
- **Thread**: 2 responses, resolved
- **Resolution**: Quick resolution

## Demo Features

### Interactive Elements

#### Guided Tours
Available tours:
1. **Agent Onboarding** (15 min)
   - Dashboard overview
   - Ticket handling
   - Customer communication
   - Reporting basics

2. **Admin Setup** (20 min)
   - System configuration
   - User management
   - Workflow creation
   - Integration setup

3. **Customer Portal** (10 min)
   - Ticket submission
   - Status tracking
   - Knowledge base
   - Profile management

#### Live Simulation
```javascript
// Simulation parameters
{
  "newTicketsPerHour": "2-5",
  "customerRepliesPerHour": "5-10",
  "agentResponsesPerHour": "10-20",
  "statusChangesPerHour": "5-15",
  "realTimeNotifications": true,
  "simulateBusinessHours": true
}
```

### Demo-Specific Features

#### Demo Banner
```html
<div class="demo-banner">
  ⚠️ This is a demo instance. Data resets daily at 2 AM UTC.
  <a href="/demo-guide">View Demo Guide</a>
</div>
```

#### Quick Actions Panel
- Reset my demo data
- Generate sample tickets
- Trigger workflow examples
- Simulate escalation
- Test notifications

#### Feature Showcase
- **Workflow Designer**: Pre-built workflow examples
- **Report Builder**: Sample reports and dashboards
- **Integration Hub**: Mock integrations active
- **AI Features**: Demonstration mode enabled
- **Multi-language**: All languages available

## Technical Specifications

### Infrastructure

```yaml
Demo Environment:
  Compute:
    Type: Kubernetes cluster
    Nodes: 3
    CPU: 8 cores per node
    Memory: 32GB per node
    
  Database:
    Type: PostgreSQL 14
    Size: 100GB SSD
    Replicas: 1 read replica
    
  Cache:
    Type: Valkey 7
    Memory: 4GB
    
  Storage:
    Type: S3-compatible
    Size: 50GB
    CDN: CloudFront
    
  Monitoring:
    Metrics: Prometheus
    Logs: ELK Stack
    Uptime: StatusPage
```

### Performance Targets

| Metric | Target | Current |
|--------|--------|---------|
| Page Load Time | < 2s | 1.5s |
| API Response (p95) | < 200ms | 150ms |
| Concurrent Users | 1000+ | 1500 |
| Uptime | 99.9% | 99.95% |
| Reset Time | < 5 min | 3 min |

## Data Generation

### Demo Data Generator Tool

```go
// tools/demo-generator/config.yaml
generator:
  tickets:
    count: 500
    vary_dates: true
    include_attachments: true
    realistic_content: true
    
  users:
    agents: 30
    customers: 200
    admins: 5
    
  organizations:
    count: 10
    types: [enterprise, business, nonprofit, education]
    
  history:
    days: 90
    realistic_patterns: true
    business_hours: true
```

### Content Templates

#### Ticket Templates
- Technical issues (40%)
- Account/billing (20%)
- Feature requests (15%)
- Bug reports (15%)
- General inquiries (10%)

#### Response Templates
- Initial acknowledgment
- Information request
- Troubleshooting steps
- Resolution confirmation
- Follow-up check

## Security & Privacy

### Data Protection
- No real customer data
- Generated emails use .demo domain
- Passwords reset daily
- No external email sending
- Sanitized file uploads

### Rate Limiting
```caddyfile
# Demo instance rate limits (Caddyfile)
{
    servers {
        max_header_size 4kb
    }
}

try.gotrs.io {
    rate_limit {
        zone demo {
            key {remote_host}
            events 100
            window 1m
        }
    }
}
```

### Restrictions
- Max 10 tickets per session
- Max 5MB file uploads
- No admin system changes
- No email configuration changes
- Read-only API for certain endpoints

## Analytics & Metrics

### Public Dashboard
Available at: https://try.gotrs.io/metrics

**Displayed Metrics:**
- Total tickets processed
- Average response time
- Customer satisfaction score
- SLA compliance rate
- Active users count
- Feature usage statistics

### Usage Tracking
```sql
-- Anonymous usage analytics
CREATE TABLE demo_analytics (
    session_id UUID,
    feature_used VARCHAR(100),
    timestamp TIMESTAMP,
    duration_seconds INTEGER,
    -- No PII collected
);
```

## Demo Reset Process

### Daily Reset Workflow

```bash
#!/bin/bash
# /scripts/demo-reset.sh

# 1. Backup interesting sessions
pg_dump gotrs_demo > /backups/demo_$(date +%Y%m%d).sql

# 2. Stop simulations
systemctl stop gotrs-simulator

# 3. Reset database
psql gotrs_demo < /data/demo-baseline.sql

# 4. Generate fresh data
/usr/local/bin/demo-generator \
  --config /etc/gotrs/demo-config.yaml \
  --randomize

# 5. Clear caches
valkey-cli FLUSHALL

# 6. Restart services
systemctl restart gotrs-demo
systemctl start gotrs-simulator

# 7. Warm up caches
curl -X POST http://localhost/api/admin/cache/warm

# 8. Update status page
curl -X POST https://status.gotrs.io/api/update \
  -d '{"component": "demo", "status": "operational"}'
```

### Data Retention
- Session analytics: 30 days
- Error logs: 7 days
- Interesting patterns: Archived
- Performance metrics: 90 days

## Testing & Development

### Using Demo for Testing

```javascript
// Example: Playwright test using demo
test('ticket creation workflow', async ({ page }) => {
  await page.goto('https://try.gotrs.io');
  await page.click('text=Quick Login as Agent');
  await page.click('text=New Ticket');
  // ... test continues
});
```

### API Testing
```bash
# Get demo API token
curl -X POST https://try.gotrs.io/api/auth/demo-token \
  -H "Content-Type: application/json" \
  -d '{"role": "agent"}'

# Use in tests
export GOTRS_API_TOKEN=demo_token_xxx
make test-contracts
```

### Load Testing
```javascript
// K6 load test configuration
export let options = {
  stages: [
    { duration: '2m', target: 100 },
    { duration: '5m', target: 100 },
    { duration: '2m', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<200'],
  },
};
```

## Feedback & Support

### Feedback Collection
- In-app feedback widget
- Post-session survey
- Feature voting system
- Bug report integration

### Demo Support
- **Documentation**: https://try.gotrs.io/docs
- **Video Guides**: https://try.gotrs.io/videos
- **Live Chat**: Available 9 AM - 5 PM EST
- **Email**: demo@gotrs.io

## Future Enhancements

### Planned Features
- [ ] Industry-specific demo data
- [ ] A/B testing capabilities
- [ ] Personalized demo experiences
- [ ] Integration playground
- [ ] Performance benchmark tool
- [ ] Migration preview tool
- [ ] Custom demo instances
- [ ] Demo data export

### Roadmap
- **Q3 2025**: Basic demo with 500 tickets
- **Q4 2025**: Live simulation engine
- **Q1 2026**: Industry templates
- **Q2 2026**: Personalized demos
- **Q3 2026**: White-label demos

---

*Demo instance maintained by the GOTRS Team*
*Last updated: August 2025*