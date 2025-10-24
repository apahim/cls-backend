-- =============================================================================
-- FIX RECONCILIATION INTERVALS FOR HEALTH-AWARE SCHEDULING
-- =============================================================================
-- This migration fixes the reconciliation scheduling to properly use 30-second
-- intervals for unhealthy clusters and 5-minute intervals for healthy clusters.
--
-- Problem: update_cluster_reconciliation_schedule() was always using 5-minute
-- interval instead of checking cluster health and using appropriate interval.

-- Updated cluster reconciliation schedule (health-aware)
CREATE OR REPLACE FUNCTION update_cluster_reconciliation_schedule(
    p_cluster_id UUID
) RETURNS VOID AS $$
DECLARE
    cluster_health BOOLEAN;
    appropriate_interval INTERVAL;
BEGIN
    -- First, update the cluster health status
    PERFORM update_cluster_health_status(p_cluster_id);

    -- Get the current health status to determine appropriate interval
    SELECT is_healthy INTO cluster_health
    FROM reconciliation_schedule
    WHERE cluster_id = p_cluster_id;

    -- If no schedule exists, create one with complete adaptive reconciliation settings
    IF NOT FOUND THEN
        -- For new clusters, assume unhealthy (need faster reconciliation)
        cluster_health := FALSE;

        INSERT INTO reconciliation_schedule (
            cluster_id,
            next_reconcile_at,
            reconcile_interval,
            enabled,
            healthy_interval,
            unhealthy_interval,
            adaptive_enabled,
            is_healthy,
            last_health_check
        ) VALUES (
            p_cluster_id,
            NOW() + INTERVAL '10 seconds',  -- Start with fast interval for new clusters
            '5 minutes'::INTERVAL,          -- Default fallback
            TRUE,
            '5 minutes'::INTERVAL,          -- Healthy interval
            '10 seconds'::INTERVAL,         -- Unhealthy interval
            TRUE,                           -- Enable adaptive scheduling
            cluster_health,                 -- Current health status
            NOW()                          -- Health check timestamp
        );
    ELSE
        -- Determine appropriate interval based on health
        IF cluster_health = TRUE THEN
            appropriate_interval := (SELECT healthy_interval FROM reconciliation_schedule WHERE cluster_id = p_cluster_id);
        ELSE
            appropriate_interval := (SELECT unhealthy_interval FROM reconciliation_schedule WHERE cluster_id = p_cluster_id);
        END IF;

        -- Update existing schedule with health-aware interval
        UPDATE reconciliation_schedule
        SET
            last_reconciled_at = NOW(),
            next_reconcile_at = NOW() + appropriate_interval,  -- Use health-aware interval!
            updated_at = NOW()
        WHERE cluster_id = p_cluster_id;
    END IF;
END;
$$ LANGUAGE plpgsql;

-- Function to automatically update health status when cluster status changes
CREATE OR REPLACE FUNCTION trigger_cluster_health_update()
RETURNS TRIGGER AS $$
BEGIN
    -- Only process UPDATE operations where status actually changed
    IF TG_OP = 'UPDATE' AND OLD.status IS DISTINCT FROM NEW.status THEN
        -- Update cluster health status based on new conditions
        PERFORM update_cluster_health_status(NEW.id);
    END IF;

    -- For INSERT operations (new clusters), also update health
    IF TG_OP = 'INSERT' THEN
        PERFORM update_cluster_health_status(NEW.id);
    END IF;

    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

-- Add trigger to automatically update health status when cluster status changes
DROP TRIGGER IF EXISTS trigger_cluster_health_status_update ON clusters;
CREATE TRIGGER trigger_cluster_health_status_update
    AFTER INSERT OR UPDATE ON clusters
    FOR EACH ROW
    EXECUTE FUNCTION trigger_cluster_health_update();

-- Function to reset all existing clusters to use fast reconciliation initially
-- This ensures existing unavailable clusters get 30-second intervals immediately
CREATE OR REPLACE FUNCTION reset_unavailable_clusters_to_fast_reconciliation()
RETURNS VOID AS $$
DECLARE
    cluster_record RECORD;
BEGIN
    -- Find all clusters that are not healthy and update their reconciliation
    FOR cluster_record IN
        SELECT c.id
        FROM clusters c
        LEFT JOIN reconciliation_schedule rs ON rs.cluster_id = c.id
        WHERE c.deleted_at IS NULL
          AND (rs.is_healthy IS NULL OR rs.is_healthy = FALSE)
    LOOP
        -- Update health status and reconciliation schedule
        PERFORM update_cluster_health_status(cluster_record.id);

        -- Force immediate reconciliation for unhealthy clusters
        UPDATE reconciliation_schedule
        SET
            next_reconcile_at = NOW() + INTERVAL '10 seconds',  -- Very fast for immediate effect
            updated_at = NOW()
        WHERE cluster_id = cluster_record.id
          AND (is_healthy IS NULL OR is_healthy = FALSE);
    END LOOP;
END;
$$ LANGUAGE plpgsql;

-- Execute the reset function to fix existing clusters immediately
SELECT reset_unavailable_clusters_to_fast_reconciliation();

-- Add comments for documentation
COMMENT ON FUNCTION update_cluster_reconciliation_schedule(UUID) IS 'Updates reconciliation schedule with health-aware intervals (10s unhealthy, 5m healthy)';
COMMENT ON FUNCTION trigger_cluster_health_update() IS 'Automatically updates cluster health status when cluster status conditions change';
COMMENT ON TRIGGER trigger_cluster_health_status_update ON clusters IS 'Triggers health status update when cluster status changes';
COMMENT ON FUNCTION reset_unavailable_clusters_to_fast_reconciliation() IS 'One-time function to reset existing unavailable clusters to fast reconciliation';

-- =============================================================================
-- MIGRATION COMPLETED
-- =============================================================================
--
-- This migration fixes the reconciliation frequency issue:
-- 1. ✅ Fixed update_cluster_reconciliation_schedule() to use health-aware intervals
-- 2. ✅ Added automatic health status updates when cluster status changes
-- 3. ✅ Reset existing unavailable clusters to use 10-second intervals
-- 4. ✅ New clusters start with 10-second intervals until healthy
--
-- Expected behavior after this migration:
-- - Unavailable clusters: 10-second reconciliation intervals
-- - Healthy clusters: 5-minute reconciliation intervals
-- - Automatic transition between intervals based on cluster health
-- - Immediate effect for existing clusters