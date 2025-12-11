-- Migration: 005_remove_nodepool_trigger_for_separate_events.sql
-- Description: Remove nodepool reactive reconciliation trigger
-- Reason: Switching to API-handler based event publishing (following cluster pattern)
-- Author: Claude Code
-- Date: 2025-12-10

-- ============================================================================
-- REMOVE NODEPOOL REACTIVE RECONCILIATION TRIGGER
-- ============================================================================
-- This migration removes the database trigger that published cluster.reconcile
-- events when nodepools changed. We're switching to explicit API handler
-- publishing to a dedicated nodepool-events topic.
--
-- Changes:
-- 1. Drop trigger on nodepools table
-- 2. Drop trigger function
-- 3. Update reactive_reconciliation_config to remove nodepool_spec
--
-- Benefits:
-- - Clean separation: NodePool events go to nodepool-events topic
-- - Follows cluster pattern: Events published from API handlers only
-- - No cross-triggering: NodePool changes don't trigger cluster reconciliation
-- ============================================================================

-- Drop the trigger first (dependent on function)
DROP TRIGGER IF EXISTS trigger_nodepool_change_reactive_reconciliation ON nodepools;

-- Drop the trigger function
DROP FUNCTION IF EXISTS trigger_nodepool_change_notification();

-- Update reactive reconciliation config to remove nodepool_spec
-- This is the list of change types that trigger cluster reconciliation
UPDATE reactive_reconciliation_config
SET
    change_types = ARRAY['spec', 'status', 'controller_status'],
    updated_at = NOW()
WHERE id = 1;

-- Add comment explaining the change
COMMENT ON TABLE reactive_reconciliation_config IS
    'Configuration for reactive reconciliation system. Supported change types: spec (cluster spec changes), status (cluster status changes), controller_status (controller status updates). NodePool events now published directly from API handlers to nodepool-events topic.';

-- ============================================================================
-- VERIFICATION QUERIES
-- ============================================================================
-- Run these queries to verify the migration was applied correctly:
--
-- 1. Verify trigger removed:
--    SELECT count(*) FROM pg_trigger WHERE tgname = 'trigger_nodepool_change_reactive_reconciliation';
--    Expected: 0
--
-- 2. Verify function removed:
--    SELECT count(*) FROM pg_proc WHERE proname = 'trigger_nodepool_change_notification';
--    Expected: 0
--
-- 3. Verify config updated:
--    SELECT change_types FROM reactive_reconciliation_config WHERE id = 1;
--    Expected: {spec,status,controller_status}
-- ============================================================================
