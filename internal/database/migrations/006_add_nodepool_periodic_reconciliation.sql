-- Migration: 006_add_nodepool_periodic_reconciliation.sql
-- Description: Add periodic reconciliation scheduling for nodepools
-- Reason: Enable health-aware reconciliation intervals for nodepools (30s unhealthy, 5m stable)
-- Date: 2025-12-10

-- ============================================================================
-- NODEPOOL PERIODIC RECONCILIATION SYSTEM
-- ============================================================================
-- This migration adds periodic reconciliation scheduling for nodepools,
-- mirroring the existing cluster reconciliation architecture.
--
-- Features:
-- - Independent per-nodepool scheduling
-- - Health-aware intervals (30s for unhealthy, 5m for stable)
-- - Automatic schedule creation on nodepool insert
-- - Automatic health tracking on status changes
-- - Generation mismatch detection
-- ============================================================================

-- ============================================================================
-- 1. CREATE NODEPOOL RECONCILIATION SCHEDULE TABLE
-- ============================================================================

CREATE TABLE nodepool_reconciliation_schedule (
    id SERIAL PRIMARY KEY,
    nodepool_id UUID NOT NULL REFERENCES nodepools(id) ON DELETE CASCADE,
    last_reconciled_at TIMESTAMP,
    next_reconcile_at TIMESTAMP,
    reconcile_interval INTERVAL DEFAULT '5 minutes',
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    -- Health-aware interval configuration
    healthy_interval INTERVAL DEFAULT '5 minutes',
    unhealthy_interval INTERVAL DEFAULT '30 seconds',
    adaptive_enabled BOOLEAN DEFAULT TRUE,
    last_health_check TIMESTAMP,
    is_healthy BOOLEAN DEFAULT NULL,  -- NULL = unknown, true = healthy, false = unhealthy

    -- One schedule per nodepool
    UNIQUE(nodepool_id)
);

-- Indexes for efficient scheduler queries
CREATE INDEX idx_nodepool_reconciliation_schedule_next_reconcile
    ON nodepool_reconciliation_schedule(next_reconcile_at, enabled)
    WHERE enabled = TRUE;

CREATE INDEX idx_nodepool_reconciliation_schedule_nodepool_id
    ON nodepool_reconciliation_schedule(nodepool_id);

COMMENT ON TABLE nodepool_reconciliation_schedule IS
    'NodePool-centric reconciliation schedule (parallel to cluster reconciliation). Each nodepool has independent scheduling with health-aware intervals (30s unhealthy, 5m healthy).';

-- ============================================================================
-- 2. HEALTH CHECK FUNCTION
-- ============================================================================

CREATE OR REPLACE FUNCTION is_nodepool_healthy(p_nodepool_id UUID)
RETURNS BOOLEAN AS $$
DECLARE
    nodepool_age INTERVAL;
    nodepool_status TEXT := NULL;
    condition_record RECORD;
    available_status TEXT := NULL;
    ready_status TEXT := NULL;
BEGIN
    -- Get nodepool age
    SELECT NOW() - created_at INTO nodepool_age
    FROM nodepools
    WHERE id = p_nodepool_id AND deleted_at IS NULL;

    -- If nodepool not found, consider unhealthy
    IF NOT FOUND THEN
        RETURN FALSE;
    END IF;

    -- New nodepools (< 2 hours old) need attention (fast reconciliation)
    IF nodepool_age < INTERVAL '2 hours' THEN
        RETURN FALSE;
    END IF;

    -- Check nodepool status conditions for Available=True AND Ready=True
    -- (mirrors cluster health check logic)
    FOR condition_record IN
        SELECT condition_data
        FROM nodepools np,
             jsonb_array_elements(COALESCE(np.status->'conditions', '[]'::jsonb)) AS condition_data
        WHERE np.id = p_nodepool_id
          AND np.deleted_at IS NULL
    LOOP
        -- Extract condition type and status
        IF condition_record.condition_data->>'type' = 'Available' THEN
            available_status := condition_record.condition_data->>'status';
        END IF;

        IF condition_record.condition_data->>'type' = 'Ready' THEN
            ready_status := condition_record.condition_data->>'status';
        END IF;
    END LOOP;

    -- NodePool is healthy only if both Available=True AND Ready=True
    IF available_status = 'True' AND ready_status = 'True' THEN
        RETURN TRUE;
    ELSE
        RETURN FALSE;
    END IF;

EXCEPTION
    WHEN OTHERS THEN
        -- If any error occurs during health check, consider nodepool unhealthy (safe default)
        RETURN FALSE;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION is_nodepool_healthy(UUID) IS
    'Checks if nodepool is healthy. Returns FALSE for: new nodepools (< 2 hours old), nodepools without Ready=True AND Available=True conditions. Mirrors cluster health check pattern.';

-- ============================================================================
-- 3. UPDATE HEALTH STATUS FUNCTION
-- ============================================================================

CREATE OR REPLACE FUNCTION update_nodepool_health_status(p_nodepool_id UUID)
RETURNS VOID AS $$
DECLARE
    nodepool_health BOOLEAN;
BEGIN
    -- Get current nodepool health
    nodepool_health := is_nodepool_healthy(p_nodepool_id);

    -- Update the reconciliation schedule with health status
    UPDATE nodepool_reconciliation_schedule
    SET
        is_healthy = nodepool_health,
        last_health_check = NOW(),
        updated_at = NOW()
    WHERE nodepool_id = p_nodepool_id;

    -- If no schedule exists, create one with defaults
    IF NOT FOUND THEN
        INSERT INTO nodepool_reconciliation_schedule (
            nodepool_id,
            is_healthy,
            last_health_check,
            next_reconcile_at,
            reconcile_interval,
            enabled,
            healthy_interval,
            unhealthy_interval,
            adaptive_enabled
        ) VALUES (
            p_nodepool_id,
            nodepool_health,
            NOW(),
            NOW() + INTERVAL '1 minute',
            '5 minutes'::INTERVAL,
            TRUE,
            '5 minutes'::INTERVAL,
            '30 seconds'::INTERVAL,
            TRUE
        );
    END IF;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION update_nodepool_health_status(UUID) IS
    'Updates nodepool health status in reconciliation schedule. Creates schedule if missing. Mirrors cluster health status update pattern.';

-- ============================================================================
-- 4. FIND NODEPOOLS NEEDING RECONCILIATION
-- ============================================================================

CREATE OR REPLACE FUNCTION find_nodepools_needing_reconciliation()
RETURNS TABLE (
    nodepool_id UUID,
    reason CHARACTER VARYING,
    last_reconciled_at TIMESTAMP WITHOUT TIME ZONE,
    nodepool_generation BIGINT
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        nrs.nodepool_id,
        CASE
            WHEN nrs.last_reconciled_at IS NULL THEN 'never_reconciled'::CHARACTER VARYING
            WHEN nrs.adaptive_enabled = TRUE AND nrs.is_healthy = FALSE AND nrs.next_reconcile_at <= NOW() THEN 'unhealthy_reconciliation'::CHARACTER VARYING
            WHEN nrs.adaptive_enabled = TRUE AND nrs.is_healthy = TRUE AND nrs.next_reconcile_at <= NOW() THEN 'healthy_reconciliation'::CHARACTER VARYING
            WHEN nrs.next_reconcile_at <= NOW() THEN 'periodic_reconciliation'::CHARACTER VARYING
            WHEN np.generation > COALESCE(
                (SELECT MAX(observed_generation) FROM nodepool_controller_status ncs WHERE ncs.nodepool_id = nrs.nodepool_id),
                0
            ) THEN 'generation_mismatch'::CHARACTER VARYING
            ELSE 'unknown'::CHARACTER VARYING
        END as reason,
        nrs.last_reconciled_at,
        np.generation as nodepool_generation
    FROM nodepool_reconciliation_schedule nrs
    JOIN nodepools np ON np.id = nrs.nodepool_id
    WHERE
        nrs.enabled = TRUE
        AND np.deleted_at IS NULL
        AND (
            -- Never reconciled
            nrs.last_reconciled_at IS NULL
            -- Scheduled reconciliation time has passed
            OR nrs.next_reconcile_at <= NOW()
            -- NodePool generation changed since any controller last saw it
            OR np.generation > COALESCE(
                (SELECT MAX(observed_generation) FROM nodepool_controller_status ncs WHERE ncs.nodepool_id = nrs.nodepool_id),
                0
            )
        )
    ORDER BY
        -- Priority: unhealthy nodepools first, then by next_reconcile_at
        (CASE
            WHEN nrs.adaptive_enabled = TRUE AND nrs.is_healthy = FALSE THEN 0
            WHEN nrs.adaptive_enabled = TRUE AND nrs.is_healthy = TRUE THEN 1
            ELSE 2
        END),
        nrs.next_reconcile_at ASC NULLS FIRST;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION find_nodepools_needing_reconciliation() IS
    'Finds nodepools needing reconciliation. Prioritizes unhealthy nodepools. Returns reason, last reconciled time, and generation. Mirrors cluster reconciliation finder pattern.';

-- ============================================================================
-- 5. UPDATE RECONCILIATION SCHEDULE FUNCTION
-- ============================================================================

CREATE OR REPLACE FUNCTION update_nodepool_reconciliation_schedule(
    p_nodepool_id UUID
) RETURNS VOID AS $$
DECLARE
    nodepool_health BOOLEAN;
    appropriate_interval INTERVAL;
BEGIN
    -- First, update the nodepool health status
    PERFORM update_nodepool_health_status(p_nodepool_id);

    -- Get the current health status to determine appropriate interval
    SELECT is_healthy INTO nodepool_health
    FROM nodepool_reconciliation_schedule
    WHERE nodepool_id = p_nodepool_id;

    -- If no schedule exists, create one
    IF NOT FOUND THEN
        -- For new nodepools, assume unhealthy (need faster reconciliation)
        nodepool_health := FALSE;

        INSERT INTO nodepool_reconciliation_schedule (
            nodepool_id,
            next_reconcile_at,
            reconcile_interval,
            enabled,
            healthy_interval,
            unhealthy_interval,
            adaptive_enabled,
            is_healthy,
            last_health_check
        ) VALUES (
            p_nodepool_id,
            NOW() + INTERVAL '30 seconds',  -- Fast initial reconciliation
            '5 minutes'::INTERVAL,          -- Default fallback
            TRUE,
            '5 minutes'::INTERVAL,          -- Healthy interval
            '30 seconds'::INTERVAL,         -- Unhealthy interval
            TRUE,
            nodepool_health,
            NOW()
        );
    ELSE
        -- Determine appropriate interval based on health
        IF nodepool_health = TRUE THEN
            appropriate_interval := (SELECT healthy_interval FROM nodepool_reconciliation_schedule WHERE nodepool_id = p_nodepool_id);
        ELSE
            appropriate_interval := (SELECT unhealthy_interval FROM nodepool_reconciliation_schedule WHERE nodepool_id = p_nodepool_id);
        END IF;

        -- Update existing schedule with health-aware interval
        UPDATE nodepool_reconciliation_schedule
        SET
            last_reconciled_at = NOW(),
            next_reconcile_at = NOW() + appropriate_interval,
            updated_at = NOW()
        WHERE nodepool_id = p_nodepool_id;
    END IF;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION update_nodepool_reconciliation_schedule(UUID) IS
    'Updates reconciliation schedule with health-aware intervals (30s unhealthy, 5m healthy). Creates schedule if missing. Mirrors cluster schedule update pattern.';

-- ============================================================================
-- 6. AUTO-CREATE SCHEDULE TRIGGER
-- ============================================================================

CREATE OR REPLACE FUNCTION create_default_nodepool_reconciliation()
RETURNS TRIGGER AS $$
BEGIN
    -- Create reconciliation schedule for new nodepool
    INSERT INTO nodepool_reconciliation_schedule (
        nodepool_id,
        next_reconcile_at,
        reconcile_interval,
        enabled,
        healthy_interval,
        unhealthy_interval,
        adaptive_enabled
    ) VALUES (
        NEW.id,
        NOW() + INTERVAL '1 minute',    -- Start reconciliation after 1 minute
        '5 minutes'::INTERVAL,
        TRUE,
        '5 minutes'::INTERVAL,
        '30 seconds'::INTERVAL,
        TRUE
    )
    ON CONFLICT (nodepool_id) DO NOTHING;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to create schedule automatically when nodepool is created
CREATE TRIGGER trigger_create_nodepool_reconciliation
    AFTER INSERT ON nodepools
    FOR EACH ROW
    EXECUTE FUNCTION create_default_nodepool_reconciliation();

COMMENT ON FUNCTION create_default_nodepool_reconciliation() IS
    'Trigger function that creates default reconciliation schedule for new nodepools. Mirrors cluster reconciliation trigger pattern.';

-- ============================================================================
-- 7. HEALTH UPDATE TRIGGER
-- ============================================================================

CREATE OR REPLACE FUNCTION trigger_nodepool_health_update()
RETURNS TRIGGER AS $$
BEGIN
    -- Only process UPDATE operations where status actually changed
    IF TG_OP = 'UPDATE' AND OLD.status IS DISTINCT FROM NEW.status THEN
        -- Update nodepool health status based on new conditions
        PERFORM update_nodepool_health_status(NEW.id);
    END IF;

    -- For INSERT operations (new nodepools), also update health
    IF TG_OP = 'INSERT' THEN
        PERFORM update_nodepool_health_status(NEW.id);
    END IF;

    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

-- Add trigger to automatically update health status when nodepool status changes
CREATE TRIGGER trigger_nodepool_health_status_update
    AFTER INSERT OR UPDATE ON nodepools
    FOR EACH ROW
    EXECUTE FUNCTION trigger_nodepool_health_update();

COMMENT ON FUNCTION trigger_nodepool_health_update() IS
    'Trigger function that automatically updates nodepool health status when nodepool status changes. Ensures reconciliation intervals adjust automatically. Mirrors cluster health trigger pattern.';

-- ============================================================================
-- 8. POPULATE SCHEDULES FOR EXISTING NODEPOOLS
-- ============================================================================

-- Create schedules for all existing nodepools
INSERT INTO nodepool_reconciliation_schedule (
    nodepool_id,
    next_reconcile_at,
    reconcile_interval,
    enabled,
    healthy_interval,
    unhealthy_interval,
    adaptive_enabled
)
SELECT
    id,
    NOW() + INTERVAL '1 minute',
    '5 minutes'::INTERVAL,
    TRUE,
    '5 minutes'::INTERVAL,
    '30 seconds'::INTERVAL,
    TRUE
FROM nodepools
WHERE deleted_at IS NULL
ON CONFLICT (nodepool_id) DO NOTHING;

-- Update health status for all existing nodepools
DO $$
DECLARE
    nodepool_record RECORD;
BEGIN
    FOR nodepool_record IN
        SELECT id FROM nodepools WHERE deleted_at IS NULL
    LOOP
        PERFORM update_nodepool_health_status(nodepool_record.id);
    END LOOP;
END $$;

-- ============================================================================
-- VERIFICATION
-- ============================================================================
-- Verify table created:
--   SELECT COUNT(*) FROM nodepool_reconciliation_schedule;
--   Expected: >= number of existing nodepools
--
-- Verify indexes:
--   SELECT indexname FROM pg_indexes WHERE tablename = 'nodepool_reconciliation_schedule';
--   Expected: 2 indexes
--
-- Verify functions:
--   SELECT proname FROM pg_proc WHERE proname LIKE '%nodepool%reconciliation%';
--   Expected: 6 functions
--
-- Verify triggers:
--   SELECT tgname FROM pg_trigger WHERE tgname LIKE '%nodepool%reconciliation%';
--   Expected: 2 triggers
--
-- Test health check:
--   SELECT is_nodepool_healthy((SELECT id FROM nodepools LIMIT 1));
--
-- Test find function:
--   SELECT * FROM find_nodepools_needing_reconciliation();
-- ============================================================================
