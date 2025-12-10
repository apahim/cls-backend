-- Migration: 004_add_nodepool_reactive_reconciliation.sql
-- Description: Add reactive reconciliation triggers for nodepool lifecycle events
-- Author: Claude Code
-- Date: 2025-12-08

-- ============================================================================
-- NODEPOOL REACTIVE RECONCILIATION
-- ============================================================================
-- This migration adds database triggers to detect nodepool changes
-- (create, update, delete) and trigger cluster-level reconciliation events.
--
-- When nodepools change, this triggers the reactive reconciliation system
-- to publish cluster.reconcile events, allowing controllers to respond
-- quickly to nodepool lifecycle changes.
-- ============================================================================

-- Function: trigger_nodepool_change_notification()
-- Purpose: Detect nodepool changes and trigger cluster reconciliation
-- Triggers on: INSERT, UPDATE, DELETE on nodepools table
CREATE OR REPLACE FUNCTION trigger_nodepool_change_notification()
RETURNS TRIGGER AS $$
DECLARE
    cluster_id_val UUID;
    change_reason TEXT;
BEGIN
    -- Get cluster_id from nodepool (handles both NEW and OLD for DELETE operations)
    cluster_id_val := COALESCE(NEW.cluster_id, OLD.cluster_id);

    -- Skip if cluster_id not found (shouldn't happen with FK constraint, but be safe)
    IF cluster_id_val IS NULL THEN
        RETURN COALESCE(NEW, OLD);
    END IF;

    -- Determine the reason based on operation type and what changed
    IF TG_OP = 'INSERT' THEN
        -- New nodepool created
        change_reason := 'nodepool_created';

    ELSIF TG_OP = 'DELETE' THEN
        -- NodePool deleted (hard delete - rare, usually soft delete via deleted_at)
        change_reason := 'nodepool_deleted';

    ELSIF TG_OP = 'UPDATE' THEN
        -- Check what changed in the update

        -- 1. Generation changed (indicates spec was updated)
        IF OLD.generation != NEW.generation THEN
            change_reason := 'nodepool_generation_increment';

        -- 2. Spec changed without generation change (shouldn't normally happen, but handle it)
        ELSIF OLD.spec IS DISTINCT FROM NEW.spec THEN
            change_reason := 'nodepool_spec_change';

        -- 3. Soft delete (deleted_at timestamp set)
        ELSIF OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL THEN
            change_reason := 'nodepool_deleted';

        ELSE
            -- Other changes (name, updated_at, etc.) - don't trigger reconciliation
            -- These are metadata changes that don't require controller action
            RETURN NEW;
        END IF;
    END IF;

    -- Verify the parent cluster exists and is not deleted before sending notification
    -- This prevents reconciliation events for nodepools whose clusters are being deleted
    IF EXISTS (
        SELECT 1 FROM clusters
        WHERE id = cluster_id_val
        AND deleted_at IS NULL
    ) THEN
        -- Send notification to trigger cluster-level reconciliation
        -- This uses the existing notify_reconciliation_change() helper function
        -- which publishes to the 'reconcile_change' PostgreSQL notification channel
        PERFORM notify_reconciliation_change(
            cluster_id_val,          -- cluster_id: triggers reconciliation for this cluster
            'nodepool_spec',         -- change_type: identifies this as a nodepool change
            NULL,                    -- controller_type: NULL for nodepool changes
            change_reason            -- reason: specific event that triggered notification
        );
    END IF;

    -- Return appropriate row for trigger chain
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

-- Create the trigger on nodepools table
-- Fires AFTER INSERT, UPDATE, or DELETE operations
-- FOR EACH ROW ensures we get individual nodepool changes, not statement-level
CREATE TRIGGER trigger_nodepool_change_reactive_reconciliation
    AFTER INSERT OR UPDATE OR DELETE ON nodepools
    FOR EACH ROW
    EXECUTE FUNCTION trigger_nodepool_change_notification();

-- Add documentation comments
COMMENT ON FUNCTION trigger_nodepool_change_notification() IS
    'Reactive reconciliation: Detects nodepool lifecycle changes (create, update, delete) and triggers cluster reconciliation events via notify_reconciliation_change()';

COMMENT ON TRIGGER trigger_nodepool_change_reactive_reconciliation ON nodepools IS
    'Triggers cluster-level reconciliation when nodepools are created, updated (spec/generation changes), or deleted (soft or hard delete)';

-- ============================================================================
-- UPDATE REACTIVE RECONCILIATION CONFIGURATION
-- ============================================================================
-- Update the default reactive reconciliation config to include nodepool_spec
-- as a recognized change type. This is for documentation purposes - the
-- DatabaseChangeListener already handles all change types generically.

UPDATE reactive_reconciliation_config
SET
    change_types = ARRAY['spec', 'status', 'controller_status', 'nodepool_spec'],
    updated_at = NOW()
WHERE id = 1;

-- Add comment explaining the config update
COMMENT ON TABLE reactive_reconciliation_config IS
    'Configuration for reactive reconciliation system. Supported change types: spec (cluster spec changes), status (cluster status changes), controller_status (controller status updates), nodepool_spec (nodepool lifecycle changes)';

-- ============================================================================
-- VERIFICATION QUERIES
-- ============================================================================
-- Run these queries to verify the migration was applied correctly:
--
-- 1. Check trigger function exists:
--    SELECT proname, prosrc FROM pg_proc WHERE proname = 'trigger_nodepool_change_notification';
--
-- 2. Check trigger exists on nodepools table:
--    SELECT tgname, tgtype, tgenabled FROM pg_trigger WHERE tgname = 'trigger_nodepool_change_reactive_reconciliation';
--
-- 3. Check reactive config updated:
--    SELECT change_types FROM reactive_reconciliation_config WHERE id = 1;
--
-- 4. Test trigger fires correctly:
--    BEGIN;
--    INSERT INTO nodepools (cluster_id, name, spec, generation)
--    VALUES ((SELECT id FROM clusters LIMIT 1), 'test-np', '{"replicas": 3}'::jsonb, 1);
--    -- Should see pg_notify('reconcile_change', ...) in logs if reactive reconciliation enabled
--    ROLLBACK;
-- ============================================================================
