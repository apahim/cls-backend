# CLS Backend Database Migrations

## Single Migration Approach

This project uses a **single consolidated migration** for clean, reliable deployments with **client isolation**.

### Migration File

- **`001_final_schema.sql`** - Complete database schema including:
  - Core tables (clusters, nodepools, status tracking)
  - **Client isolation via `created_by` field**
  - **No organization multi-tenancy** (simplified architecture)
  - Cluster-centric reconciliation system (fan-out architecture)
  - Reactive reconciliation triggers
  - Status aggregation system with caching
  - **Optimized indexes for client isolation performance**
  - All utility functions and triggers

### Benefits

1. **Simplified Deployment**: Single migration to apply
2. **No Order Dependencies**: No complex migration chains
3. **Atomic Schema**: All schema changes in one transaction
4. **Fresh Deployments**: Perfect for new environments
5. **Reduced Failure Points**: Fewer migration steps to fail

### Usage

For fresh deployments, the migration system will automatically apply `001_final_schema.sql` which creates the complete schema with client isolation in a single operation.

#### Client Isolation Features
- **Clusters**: Only accessible by the user who created them (`created_by = user_email`)
- **NodePools**: Secured through cluster ownership (users can only access nodepools in their clusters)
- **Performance**: Fast queries thanks to optimized `idx_clusters_created_by` index
- **Security**: Zero-trust model - every operation validates ownership

### Migration History

This consolidated migration represents the final evolved schema with client isolation:
- ✅ Complete schema with client isolation
- ✅ Organization multi-tenancy **removed** (simplified architecture)
- ✅ Reactive reconciliation triggers
- ✅ Fan-out architecture (no controller type dependencies)
- ✅ Client isolation via `created_by` field

### Schema Highlights

- **🔒 Client Isolation**: Every user sees only their own clusters and nodepools
- **🚀 Performance**: `idx_clusters_created_by` index for fast client filtering
- **📡 Event-driven**: Pub/Sub integration for controller events
- **🔄 Reconciliation**: Centralized controller scheduling with fan-out
- **📊 Status Aggregation**: Automated cluster health calculation
- **🗂️ Extensible**: JSONB fields for flexible metadata storage
- **🛡️ Security**: NodePool access inherited through cluster ownership

### Development

For development environments, you can safely drop and recreate the entire database:

```sql
DROP SCHEMA public CASCADE;
CREATE SCHEMA public;
-- Run migration system to apply 001_final_schema.sql
```

Or apply the migration directly:

```bash
psql -d your_database -f 001_final_schema.sql
```

### Testing Client Isolation

You can verify client isolation is working by:

1. Creating clusters with different `user_email` values
2. Attempting to access other users' clusters (should return 404/401)
3. Checking that nodepool operations require cluster ownership

This approach ensures consistent schema state and security across all environments.