-- =============================================================================
-- ADD CREATED_BY TO NODEPOOLS TABLE
-- =============================================================================
-- This migration adds the created_by column to the nodepools table for
-- consistency with the clusters table. This enables:
--   1. Tracking which user created each nodepool
--   2. Consistent API response structure (nodepools match clusters)
--   3. Future external authorization integration
--   4. Audit trail for nodepool creation
--
-- Migration: 009
-- Created: 2025-12-11
-- =============================================================================

-- -----------------------------------------------------------------------------
-- 1. Add created_by column to nodepools table
-- -----------------------------------------------------------------------------
-- This column stores the email/identifier of the user who created the nodepool.
-- It's populated from the X-User-Email header in the API request.
--
-- NULLABLE: Existing nodepools won't have created_by, so we allow NULL initially.
-- New nodepools will always have created_by set (enforced in application code).

ALTER TABLE nodepools ADD COLUMN IF NOT EXISTS created_by VARCHAR(255);

COMMENT ON COLUMN nodepools.created_by IS
    'Email/identifier of the user who created this nodepool. Populated from X-User-Email header.';

-- -----------------------------------------------------------------------------
-- 2. Backfill created_by from parent cluster
-- -----------------------------------------------------------------------------
-- For existing nodepools without created_by, we backfill from the parent cluster.
-- This ensures consistency and allows existing nodepools to have creator information.

UPDATE nodepools np
SET created_by = c.created_by
FROM clusters c
WHERE np.cluster_id = c.id
  AND np.created_by IS NULL
  AND c.created_by IS NOT NULL;

-- -----------------------------------------------------------------------------
-- 3. Migration complete
-- -----------------------------------------------------------------------------
-- Summary of changes:
--   ✓ Added created_by column to nodepools table
--   ✓ Backfilled created_by from parent clusters for existing nodepools
--
-- Result: NodePools now track creator information for audit and authorization.
-- =============================================================================
