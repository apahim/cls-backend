-- Simplify Adaptive Reconciliation System
-- Remove complex health-aware reconciliation in favor of simple binary state model
-- Two states: "needs-attention" (new clusters + errors) vs "stable" (everything else)
-- Intervals: 30s for needs-attention, 5m for stable

-- =============================================================================
-- REMOVE COMPLEX ADAPTIVE RECONCILIATION COLUMNS
-- =============================================================================

-- Remove complex health-aware columns from reconciliation_schedule table
ALTER TABLE reconciliation_schedule
DROP COLUMN IF EXISTS healthy_interval,
DROP COLUMN IF EXISTS unhealthy_interval,
DROP COLUMN IF EXISTS adaptive_enabled,
DROP COLUMN IF EXISTS last_health_check,
DROP COLUMN IF EXISTS is_healthy;

-- =============================================================================
-- SIMPLIFIED RECONCILIATION FUNCTIONS
-- =============================================================================

-- Simple function to determine if cluster needs attention (binary state model)
CREATE OR REPLACE FUNCTION cluster_needs_attention(p_cluster_id UUID)
RETURNS BOOLEAN AS $$
DECLARE
    cluster_age INTERVAL;
    cluster_status TEXT;
BEGIN
    -- Get cluster age and status
    SELECT
        NOW() - c.created_at,
        c.status->>'phase'
    INTO cluster_age, cluster_status
    FROM clusters c
    WHERE c.id = p_cluster_id AND c.deleted_at IS NULL;

    -- If cluster not found, consider it needs attention
    IF NOT FOUND THEN
        RETURN TRUE;
    END IF;

    -- Needs attention if:
    -- 1. New cluster (less than 2 hours old)
    -- 2. Has error status
    IF cluster_age < INTERVAL '2 hours' THEN
        RETURN TRUE;
    END IF;

    IF cluster_status IN ('Error', 'Failed', 'Unknown') THEN
        RETURN TRUE;
    END IF;

    -- Otherwise stable
    RETURN FALSE;
END;
$$ LANGUAGE plpgsql;

-- Simplified reconciliation interval determination
CREATE OR REPLACE FUNCTION get_reconcile_interval(p_cluster_id UUID)
RETURNS INTERVAL AS $$
BEGIN
    -- Simple binary decision
    IF cluster_needs_attention(p_cluster_id) THEN
        RETURN INTERVAL '30 seconds';  -- Needs attention
    ELSE
        RETURN INTERVAL '5 minutes';   -- Stable
    END IF;
END;
$$ LANGUAGE plpgsql;

-- Simplified cluster reconciliation schedule update
CREATE OR REPLACE FUNCTION update_cluster_reconciliation_schedule(
    p_cluster_id UUID
) RETURNS VOID AS $$
DECLARE
    current_interval INTERVAL;
BEGIN
    -- Get appropriate interval using simplified logic
    current_interval := get_reconcile_interval(p_cluster_id);

    -- Update or insert reconciliation schedule with simplified logic
    INSERT INTO reconciliation_schedule (
        cluster_id,
        last_reconciled_at,
        next_reconcile_at,
        reconcile_interval,
        enabled
    ) VALUES (
        p_cluster_id,
        NOW(),
        NOW() + current_interval,
        current_interval,
        TRUE
    )
    ON CONFLICT (cluster_id) DO UPDATE SET
        last_reconciled_at = NOW(),
        next_reconcile_at = NOW() + current_interval,
        reconcile_interval = current_interval,
        updated_at = NOW();
END;
$$ LANGUAGE plpgsql;

-- Simplified function to find clusters needing reconciliation
CREATE OR REPLACE FUNCTION find_clusters_needing_reconciliation()
RETURNS TABLE (
    cluster_id UUID,
    reason CHARACTER VARYING,
    last_reconciled_at TIMESTAMP WITHOUT TIME ZONE,
    cluster_generation BIGINT
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        rs.cluster_id,
        CASE
            WHEN rs.last_reconciled_at IS NULL THEN 'never_reconciled'::CHARACTER VARYING
            WHEN rs.next_reconcile_at <= NOW() THEN 'scheduled_reconciliation'::CHARACTER VARYING
            WHEN c.generation > COALESCE(
                (SELECT MAX(observed_generation) FROM controller_status cs WHERE cs.cluster_id = rs.cluster_id),
                0
            ) THEN 'generation_mismatch'::CHARACTER VARYING
            ELSE 'unknown'::CHARACTER VARYING
        END as reason,
        rs.last_reconciled_at,
        c.generation as cluster_generation
    FROM reconciliation_schedule rs
    JOIN clusters c ON c.id = rs.cluster_id
    WHERE
        rs.enabled = TRUE
        AND c.deleted_at IS NULL
        AND (
            -- Never reconciled
            rs.last_reconciled_at IS NULL
            -- Scheduled reconciliation time has passed
            OR rs.next_reconcile_at <= NOW()
            -- Cluster generation changed since any controller last saw it
            OR c.generation > COALESCE(
                (SELECT MAX(observed_generation) FROM controller_status cs WHERE cs.cluster_id = rs.cluster_id),
                0
            )
        )
    ORDER BY
        -- Priority: clusters needing attention first
        (CASE WHEN cluster_needs_attention(rs.cluster_id) THEN 0 ELSE 1 END),
        rs.next_reconcile_at ASC NULLS FIRST;
END;
$$ LANGUAGE plpgsql;

-- Simplified cluster reconciliation creation for new clusters
CREATE OR REPLACE FUNCTION create_default_cluster_reconciliation()
RETURNS TRIGGER AS $$
DECLARE
    initial_interval INTERVAL;
BEGIN
    -- Determine initial interval using simplified logic
    initial_interval := get_reconcile_interval(NEW.id);

    -- Create simple reconciliation schedule
    INSERT INTO reconciliation_schedule (
        cluster_id,
        next_reconcile_at,
        reconcile_interval,
        enabled
    ) VALUES (
        NEW.id,
        NOW() + INTERVAL '1 minute',  -- Initial reconciliation in 1 minute
        initial_interval,
        TRUE
    )
    ON CONFLICT (cluster_id) DO NOTHING;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- =============================================================================
-- REMOVE COMPLEX HEALTH EVALUATION FUNCTIONS
-- =============================================================================

-- Drop complex health evaluation functions that are no longer needed
DROP FUNCTION IF EXISTS is_cluster_healthy(UUID);
DROP FUNCTION IF EXISTS update_cluster_health_status(UUID);

-- =============================================================================
-- UPDATE COMMENTS
-- =============================================================================

COMMENT ON TABLE reconciliation_schedule IS 'Simplified cluster reconciliation schedule using binary state model (needs-attention vs stable)';
COMMENT ON FUNCTION cluster_needs_attention(UUID) IS 'Determines if cluster needs attention using simple binary logic (new clusters + error status)';
COMMENT ON FUNCTION get_reconcile_interval(UUID) IS 'Returns appropriate reconciliation interval: 30s for needs-attention, 5m for stable';
COMMENT ON FUNCTION update_cluster_reconciliation_schedule(UUID) IS 'Updates reconciliation schedule using simplified binary state model';
COMMENT ON FUNCTION find_clusters_needing_reconciliation() IS 'Finds clusters needing reconciliation with simplified priority logic';
COMMENT ON FUNCTION create_default_cluster_reconciliation() IS 'Creates default reconciliation schedule for new clusters using simplified logic';