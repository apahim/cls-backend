-- Remove Organization Multi-Tenancy Migration
-- This migration removes all organization-based multi-tenancy implementation
-- Transitioning to simplified access control using only created_by field

-- =============================================================================
-- DROP ORGANIZATION-RELATED TRIGGERS
-- =============================================================================

-- Drop organization field synchronization trigger
DROP TRIGGER IF EXISTS sync_organization_fields_trigger ON clusters;

-- =============================================================================
-- DROP ORGANIZATION-RELATED VIEWS
-- =============================================================================

-- Drop organization information view
DROP VIEW IF EXISTS organization_info;

-- =============================================================================
-- DROP ORGANIZATION-RELATED FUNCTIONS
-- =============================================================================

-- Drop organization utility functions
DROP FUNCTION IF EXISTS get_organization_id_from_domain(VARCHAR(255));
DROP FUNCTION IF EXISTS get_organization_domain_from_id(VARCHAR(100));
DROP FUNCTION IF EXISTS auto_populate_organization_id();

-- Drop organization query functions
DROP FUNCTION IF EXISTS list_clusters_by_organization_id(VARCHAR(100), INT, INT, VARCHAR(50), VARCHAR(50));
DROP FUNCTION IF EXISTS count_clusters_by_organization_id(VARCHAR(100), VARCHAR(50), VARCHAR(50));
DROP FUNCTION IF EXISTS get_cluster_by_id_and_organization(UUID, VARCHAR(100));

-- =============================================================================
-- DROP ORGANIZATION-RELATED INDEXES
-- =============================================================================

-- Drop organization-related indexes from clusters table
DROP INDEX IF EXISTS idx_clusters_organization_domain;
DROP INDEX IF EXISTS idx_clusters_organization_id;

-- Drop cluster ownership indexes
DROP INDEX IF EXISTS idx_cluster_ownership_organization_domain;
DROP INDEX IF EXISTS idx_cluster_ownership_organization_id;

-- Drop organization mapping indexes
DROP INDEX IF EXISTS idx_organization_mapping_id;

-- =============================================================================
-- REMOVE ORGANIZATION COLUMNS FROM CLUSTERS TABLE
-- =============================================================================

-- Drop organization-related unique constraints
ALTER TABLE clusters DROP CONSTRAINT IF EXISTS clusters_name_organization_unique;
ALTER TABLE clusters DROP CONSTRAINT IF EXISTS clusters_name_organization_id_unique;

-- Remove organization columns from clusters table
ALTER TABLE clusters DROP COLUMN IF EXISTS organization_domain;
ALTER TABLE clusters DROP COLUMN IF EXISTS organization_id;

-- Add global unique constraint for cluster names (simplified approach)
ALTER TABLE clusters ADD CONSTRAINT clusters_name_unique UNIQUE(name);

-- =============================================================================
-- DROP ORGANIZATION-RELATED TABLES
-- =============================================================================

-- Drop cluster ownership table (references clusters, so drop first)
DROP TABLE IF EXISTS cluster_ownership;

-- Drop organization mapping table
DROP TABLE IF EXISTS organization_mapping;

-- Drop organizations table
DROP TABLE IF EXISTS organizations;

-- =============================================================================
-- UPDATE COMMENTS AND DOCUMENTATION
-- =============================================================================

COMMENT ON TABLE clusters IS 'Core cluster lifecycle management with simplified access control';
COMMENT ON COLUMN clusters.created_by IS 'User who created the cluster - used for future external authorization integration';

-- =============================================================================
-- VERIFICATION QUERIES (FOR MANUAL VERIFICATION)
-- =============================================================================

-- Verify organization tables are removed
-- SELECT table_name FROM information_schema.tables
-- WHERE table_schema = 'public' AND table_name LIKE '%organization%';

-- Verify organization columns are removed from clusters
-- SELECT column_name FROM information_schema.columns
-- WHERE table_schema = 'public' AND table_name = 'clusters' AND column_name LIKE '%organization%';

-- Verify clusters table structure
-- \d clusters;

-- =============================================================================
-- MIGRATION COMPLETED
-- =============================================================================

-- This migration successfully removes:
-- 1. All organization-related tables (organizations, cluster_ownership, organization_mapping)
-- 2. Organization columns from clusters table (organization_domain, organization_id)
-- 3. Organization-related functions, triggers, indexes, and views
-- 4. Organization-based unique constraints
--
-- Keeps:
-- 1. created_by field in clusters table for future external authorization
-- 2. Global unique constraint on cluster names
-- 3. All core cluster functionality intact