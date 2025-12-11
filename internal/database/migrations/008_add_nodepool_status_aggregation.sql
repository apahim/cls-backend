-- =============================================================================
-- NODEPOOL STATUS AGGREGATION SUPPORT
-- =============================================================================
-- This migration adds status aggregation support for nodepools, mirroring
-- the proven cluster status aggregation pattern. It enables:
--   1. Cached status with dirty flag optimization
--   2. Independent nodepool status (separate from cluster status)
--   3. Kubernetes-like Ready/Available conditions
--   4. Generation-aware status aggregation
--
-- CRITICAL: This migration REPLACES the trigger that marks clusters dirty
-- when nodepool_controller_status changes. The new trigger marks the
-- NODEPOOL dirty instead, enabling independent nodepool status aggregation.
--
-- Migration: 008
-- Created: 2025-12-11
-- =============================================================================

-- -----------------------------------------------------------------------------
-- 1. Add status_dirty column to nodepools table
-- -----------------------------------------------------------------------------
-- This column enables the dirty flag caching pattern:
--   - TRUE: Status needs recalculation (slow path ~5-10ms)
--   - FALSE: Use cached status from 'status' column (fast path <1ms)

ALTER TABLE nodepools ADD COLUMN IF NOT EXISTS status_dirty BOOLEAN DEFAULT TRUE;

COMMENT ON COLUMN nodepools.status_dirty IS
    'Triggers status recalculation when TRUE. Set by triggers when nodepool_controller_status changes.';

-- -----------------------------------------------------------------------------
-- 2. Create partial index for efficient dirty status queries
-- -----------------------------------------------------------------------------
-- Partial index only includes rows where status_dirty = TRUE
-- This makes queries for dirty nodepools extremely fast

CREATE INDEX IF NOT EXISTS idx_nodepools_status_dirty ON nodepools(status_dirty)
    WHERE status_dirty = TRUE;

COMMENT ON INDEX idx_nodepools_status_dirty IS
    'Partial index for efficient queries of nodepools needing status recalculation';

-- -----------------------------------------------------------------------------
-- 3. Create function to mark nodepool as dirty
-- -----------------------------------------------------------------------------
-- This function is called by the trigger when nodepool_controller_status changes

CREATE OR REPLACE FUNCTION mark_nodepool_status_dirty()
RETURNS TRIGGER AS $$
BEGIN
    -- Mark the nodepool as dirty so status aggregation will be triggered
    UPDATE nodepools
    SET status_dirty = TRUE, updated_at = NOW()
    WHERE id = NEW.nodepool_id AND deleted_at IS NULL;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION mark_nodepool_status_dirty() IS
    'Marks nodepool as dirty when controller status changes, triggering status recalculation';

-- -----------------------------------------------------------------------------
-- 4. CRITICAL: Replace trigger for independent nodepool status
-- -----------------------------------------------------------------------------
-- Migration 002 created a trigger that marks the parent CLUSTER dirty when
-- nodepool_controller_status changes. This was correct for cluster-level
-- status aggregation, but INCORRECT for independent nodepool status.
--
-- We now REPLACE that trigger with one that marks the NODEPOOL dirty instead.

-- Drop the old trigger that marks cluster dirty
DROP TRIGGER IF EXISTS nodepool_controller_status_dirty_trigger
    ON nodepool_controller_status;

-- Create new trigger to mark NODEPOOL dirty (not cluster)
CREATE TRIGGER nodepool_controller_status_dirty_trigger
    AFTER INSERT OR UPDATE ON nodepool_controller_status
    FOR EACH ROW
    EXECUTE FUNCTION mark_nodepool_status_dirty();

COMMENT ON TRIGGER nodepool_controller_status_dirty_trigger ON nodepool_controller_status IS
    'Automatically marks nodepools dirty when controller status changes for status aggregation';

-- -----------------------------------------------------------------------------
-- 5. Backfill existing nodepools with dirty flag
-- -----------------------------------------------------------------------------
-- Set all existing nodepools to dirty so they get fresh status on next read

UPDATE nodepools
SET status_dirty = TRUE
WHERE status_dirty IS NULL OR status_dirty = FALSE;

-- -----------------------------------------------------------------------------
-- 6. Migration complete
-- -----------------------------------------------------------------------------
-- Summary of changes:
--   ✓ Added status_dirty column to nodepools table
--   ✓ Created partial index for efficient dirty queries
--   ✓ Created mark_nodepool_status_dirty() trigger function
--   ✓ REPLACED trigger to mark nodepool dirty (not cluster)
--   ✓ Backfilled existing nodepools with dirty=TRUE
--
-- Result: NodePools now have independent status aggregation with dirty flag
-- caching, matching the proven cluster status aggregation pattern.
-- =============================================================================
