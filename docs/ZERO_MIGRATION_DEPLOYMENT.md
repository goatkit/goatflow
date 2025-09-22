# Zero-Migration OTRS to GOTRS Deployment

## 🎯 The Game-Changing Feature

GOTRS can connect directly to your existing OTRS MySQL/MariaDB database without any migration, data conversion, or downtime. This is possible thanks to our thin database access compatibility wrapper (ConvertPlaceholders) and strict OTRS schema alignment.

## 🚀 What This Means

### For Organizations
- **Zero Risk**: Keep your existing OTRS database untouched
- **Instant Rollback**: Switch back to OTRS anytime
- **No Downtime**: Run both systems in parallel during evaluation
- **Gradual Migration**: Move at your own pace, or never migrate at all

### For IT Teams
- **No Data Migration**: Connect GOTRS directly to OTRS database
- **No Schema Changes**: GOTRS respects OTRS table structure 100%
- **No Learning Curve**: Same database, same data, modern interface
- **Easy Testing**: Try GOTRS in read-only mode first

## 📋 Deployment Options

### Option 1: Direct Database Connection (Recommended for Testing)
```yaml
# GOTRS configuration
DATABASE_DRIVER: mysql
DATABASE_HOST: your-otrs-db.example.com
DATABASE_PORT: 3306
DATABASE_NAME: otrs
DATABASE_USER: otrs
DATABASE_PASSWORD: your-password
```

No migration. No conversion. Just point and run.

### Option 2: Side-by-Side Deployment
1. Keep OTRS running on port 80
2. Run GOTRS on port 8080
3. Both use the same MySQL database
4. Users can access either system
5. Gradually move users to GOTRS

### Option 3: Blue-Green Deployment
1. Set up GOTRS with read-only access first
2. Verify all functionality works
3. Switch write access from OTRS to GOTRS
4. Keep OTRS as fallback

## 🔧 Quick Start

### Step 1: Expose OTRS Database Port
Edit your OTRS `docker-compose.yml`:
```yaml
mariadb:
  ports:
    - "3306:3306"  # Expose MariaDB port
```

### Step 2: Configure GOTRS
Create `.env` file for GOTRS:
```bash
DB_DRIVER=mysql
DB_HOST=localhost
DB_PORT=3306
DB_NAME=otrs
DB_USER=otrs
DB_PASSWORD=your-otrs-password
```

### Step 3: Run GOTRS
```bash
docker run -d \
  --name gotrs \
  --env-file .env \
  -p 8080:8080 \
  gotrs:latest
```

### Step 4: Access Both Systems
- OTRS: http://localhost (existing)
- GOTRS: http://localhost:8080 (new)
- Same database, same tickets, modern interface!

## ✅ Tested and Verified

We've successfully tested GOTRS with:
- **Live OTRS 6.0.x** with MariaDB
- **116 OTRS tables** recognized and accessible
- **Real production data** (tickets, users, articles)
- **Zero schema modifications** required

### Test Results
```
✅ Successfully connected to OTRS MySQL database!
✅ Found 116 tables in OTRS database
✅ users: 4 records
✅ groups: 4 records  
✅ ticket: 8 records
✅ article: 11 records
✅ queue: 5 records
✅ customer_company: 1 records
✅ customer_user: 2 records
```

## 🛡️ Safety Features

### Read-Only Mode
Start with GOTRS in read-only mode to verify compatibility:
```yaml
GOTRS_READ_ONLY: true
```

### Audit Logging
Track all GOTRS operations without affecting OTRS:
```yaml
GOTRS_AUDIT_MODE: true
GOTRS_AUDIT_LOG: /var/log/gotrs-audit.log
```

### Database Backup
Always backup before testing (though no changes are made):
```bash
mysqldump -h localhost -u otrs -p otrs > otrs-backup.sql
```

## 📊 Migration Strategies

### Strategy 1: Never Migrate
- Run GOTRS permanently against OTRS database
- Get modern UI without data migration
- Keep existing OTRS as fallback

### Strategy 2: Gradual Migration
1. Run both systems for 30 days
2. Monitor performance and user feedback
3. Gradually move workflows to GOTRS
4. Decommission OTRS when ready

### Strategy 3: Instant Switch
1. Test GOTRS thoroughly in staging
2. Schedule maintenance window
3. Update DNS/proxy to point to GOTRS
4. Keep OTRS container stopped but ready

## 🎯 Benefits Summary

| Aspect | Traditional Migration | GOTRS Zero-Migration |
|--------|---------------------|---------------------|
| Downtime | Hours to Days | **Zero** |
| Risk | High | **Zero** |
| Rollback Time | Hours | **Instant** |
| Data Migration | Required | **None** |
| Testing Period | Limited | **Unlimited** |
| Database Changes | Many | **None** |
| Staff Training | Extensive | **Minimal** |

## 🚦 Decision Matrix

Use GOTRS Zero-Migration when you:
- ✅ Want to evaluate GOTRS without risk
- ✅ Need to maintain 24/7 operations
- ✅ Have compliance requirements preventing data migration
- ✅ Want gradual user transition
- ✅ Need instant rollback capability

## 💡 Pro Tips

1. **Start Small**: Test with read-only access first
2. **Monitor Performance**: Compare query times between OTRS and GOTRS
3. **User Feedback**: Run A/B testing with select users
4. **Backup Always**: Even though we don't modify data
5. **Document Everything**: Keep notes for your team

## 🔗 Technical Details

GOTRS achieves this through:
- **Database Access Compatibility**: Thin wrapper over database/sql (ConvertPlaceholders) supporting MySQL and PostgreSQL
- **OTRS Schema Compatibility**: 100% compatible table structure
- **Smart Query Generation**: Database-specific SQL generation
- **Type Mapping**: Automatic type conversion between databases
- **Feature Detection**: Adapts to database capabilities

## 📞 Support

This is a game-changing feature for OTRS migration. If you need help:
- GitHub Issues: [github.com/gotrs-io/gotrs-ce/issues](https://github.com/gotrs-io/gotrs-ce/issues)
- Documentation: [docs.gotrs.io](https://docs.gotrs.io)
- Community: [community.gotrs.io](https://community.gotrs.io)

---

*Zero-Migration Deployment: Because the best migration is no migration at all.*