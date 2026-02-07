# GoatFlow Licensing Strategy

## Overview

GoatFlow employs a dual licensing model that balances open source principles with sustainable business development. This approach ensures the project remains accessible to all while providing a path for commercial sustainability.

## Dual License Model

### 1. Community Edition (CE) - Apache License 2.0

The Community Edition is released under the Apache License 2.0, chosen for its:

- **Permissive nature**: Allows commercial use, modification, and distribution
- **Patent protection**: Includes express patent grant
- **Compatibility**: Works well with other open source licenses
- **Enterprise friendly**: No copyleft requirements
- **Attribution**: Requires preservation of copyright notices

**What's Included in CE:**
- ✅ Full core ticketing functionality
- ✅ Complete API access
- ✅ All standard integrations
- ✅ Plugin framework
- ✅ Docker/Kubernetes deployment
- ✅ Community support
- ✅ Security updates
- ✅ Bug fixes

**License Text:**
```
Copyright 2025 Gibbsoft Ltd and Contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
```

### 2. Enterprise Edition (EE) - Commercial License

The Enterprise Edition is available under a commercial license for organizations requiring:

- Advanced enterprise features
- Professional support with SLA
- Legal indemnification
- White-labeling rights
- Priority bug fixes
- Direct influence on roadmap

**Enterprise-Only Features:**
- 💼 Advanced ITSM modules
- 💼 Multi-tenancy support
- 💼 Advanced compliance tools (HIPAA, SOC2)
- 💼 Enterprise SSO providers
- 💼 High availability clustering
- 💼 Advanced analytics and BI
- 💼 White-label customization
- 💼 Priority support queue

## Pricing Structure

### Community Edition
**Free Forever**
- Unlimited users
- Unlimited tickets
- Full functionality
- Community support
- Self-hosted only

### Enterprise Edition

#### Starter Plan
**$499/month** (up to 25 agents)
- All CE features
- Business hours support
- Basic SSO (SAML/LDAP)
- Backup automation
- 99.5% SLA

#### Professional Plan
**$1,499/month** (up to 100 agents)
- All Starter features
- 24/7 support
- Advanced ITSM modules
- Custom workflows
- API priority
- 99.9% SLA

#### Enterprise Plan
**$4,999/month** (unlimited agents)
- All Professional features
- Dedicated support engineer
- White-labeling
- Multi-tenancy
- Custom development
- 99.99% SLA

#### Cloud Hosted (SaaS)
**+50% of plan price**
- Fully managed infrastructure
- Automatic updates
- Backups included
- Geographic redundancy
- DDoS protection

### Volume Discounts
- 100-500 agents: 10% discount
- 500-1000 agents: 20% discount
- 1000+ agents: Custom pricing

### Non-Profit & Education
- 50% discount on all plans
- Free CE hosting support
- Training resources

## Contributor License Agreement (CLA)

To maintain the dual licensing model, contributors must sign a CLA that:

1. **Grants copyright license** to the project maintainers
2. **Grants patent license** for any patents in contributions
3. **Confirms originality** of contributed work
4. **Allows dual licensing** of contributions

**CLA Process:**
1. Fork the repository
2. Make your changes
3. Sign CLA electronically (automated via GitHub)
4. Submit pull request
5. Contribution reviewed and merged

## Code Separation

### Repository Structure
```
goatflow/
├── core/           # Apache 2.0 - All CE code
├── enterprise/     # Commercial - EE-only features
├── shared/         # Apache 2.0 - Shared libraries
└── plugins/
    ├── community/  # Various licenses
    └── enterprise/ # Commercial
```

### Build Process
```bash
# Build Community Edition
make build-ce

# Build Enterprise Edition (requires license key)
GOATFLOW_LICENSE_KEY=xxx make build-ee
```

## Revenue Allocation

### Revenue Distribution
- **40%** - Core development
- **20%** - Security & infrastructure
- **15%** - Community programs
- **15%** - Marketing & growth
- **10%** - Legal & compliance

### Community Investment
- Bug bounty program
- Contributor rewards
- Conference sponsorships
- Documentation improvements
- Training materials

## License Compliance

### For Users

#### Using CE
- ✅ Use for any purpose (commercial or non-commercial)
- ✅ Modify and customize
- ✅ Distribute modifications
- ✅ Create proprietary plugins
- ⚠️ Must preserve copyright notices
- ⚠️ Must include license text
- ❌ Cannot use GoatFlow trademarks without permission

#### Using EE
- ✅ All rights granted by commercial license
- ✅ White-labeling (if licensed)
- ✅ Redistribution (if licensed)
- ⚠️ Subject to license terms
- ❌ Cannot share license key
- ❌ Cannot exceed licensed agent count

### For Contributors
- Must sign CLA before first contribution
- Retain copyright to contributions
- Grant necessary rights for dual licensing
- Can use contributions in other projects

## Trademark Policy

### Protected Marks
- "GoatFlow" name and logo
- "GoatKit" name and logo
- Associated service marks

### Permitted Use
- Factual references to the software
- Community user groups
- Training materials (with attribution)
- Blogs and articles

### Requires Permission
- Commercial training services
- Hosted service offerings
- Modified distributions
- Product names including "GoatFlow" or "GoatKit"

## Migration from Other Licenses

### From OTRS (GPL)
- No license conflict for users
- Clean-room implementation
- No code reuse from OTRS
- Database schema compatibility only

### To GoatFlow CE
- Apache 2.0 compatible with most licenses
- Can integrate with proprietary systems
- No viral license effects

### To GoatFlow EE
- Commercial license supersedes Apache 2.0
- Additional rights and features
- Professional support included

## Frequently Asked Questions

### General Licensing

**Q: Can I use GoatFlow CE in my commercial product?**
A: Yes, Apache 2.0 allows commercial use.

**Q: Do I need to open source my modifications?**
A: No, Apache 2.0 doesn't require this.

**Q: Can I remove the GoatFlow branding?**
A: In CE, you must preserve copyright notices. In EE with white-label license, yes.

### Enterprise Edition

**Q: What happens if I exceed my agent limit?**
A: Grace period of 30 days to upgrade, then reduced to CE features.

**Q: Can I switch from EE to CE?**
A: Yes, but you'll lose EE-only features and support.

**Q: Is the source code available for EE?**
A: Yes, to licensed customers under commercial terms.

### Contributing

**Q: Why do I need to sign a CLA?**
A: To enable dual licensing and protect the project legally.

**Q: Can I contribute to EE features?**
A: EE contributions require additional agreements.

**Q: Do I get compensated for contributions?**
A: Major contributors may receive rewards, swag, or recognition.

## Legal Notices

### Warranty Disclaimer
THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED.

### Limitation of Liability
IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY.

### Indemnification
Enterprise Edition includes indemnification terms in the commercial agreement.

## Contact

### Licensing Inquiries
- Email: hello@goatflow.io
- Phone: +1-555-GoatFlow-LIC
- Web: https://goatflow.io/licensing

### Legal Questions
- Email: hello@goatflow.io
- Address: GoatFlow Legal Dept, [Address]

### Sales
- Email: hello@goatflow.io
- Phone: +1-555-GoatFlow-BUY
- Web: https://goatflow.io/pricing

---

*Last updated: August 2025*
*Version: 1.0*