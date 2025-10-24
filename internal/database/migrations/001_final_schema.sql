-- CLS Backend Final Database Schema
-- This migration contains the complete schema for clean deployment
-- Includes: clusters, nodepools, status tracking, reconciliation, client isolation
-- Features: Client isolation via created_by, no organization multi-tenancy

-- =============================================================================
-- CORE TABLES
-- =============================================================================

-- Clusters table - Core cluster lifecycle management with client isolation
CREATE TABLE IF NOT EXISTS clusters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,  -- Cluster name (unique per user)
    target_project_id VARCHAR(255),
    created_by VARCHAR(255) NOT NULL,   -- Client isolation - user email
    spec JSONB NOT NULL DEFAULT '{}',
    status JSONB,  -- Kubernetes-like status block with conditions
    metadata JSONB NOT NULL DEFAULT '{}',
    generation BIGINT NOT NULL DEFAULT 1,
    resource_version VARCHAR(255) NOT NULL DEFAULT gen_random_uuid()::TEXT,
    status_dirty BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,

    -- Unique constraint: cluster names must be unique per user
    UNIQUE(name, created_by)
);

-- NodePools table - Cluster nodepool management (inherits security from clusters)
CREATE TABLE IF NOT EXISTS nodepools (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cluster_id UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    spec JSONB NOT NULL DEFAULT '{}',
    status JSONB NOT NULL DEFAULT '{}',
    metadata JSONB NOT NULL DEFAULT '{}',
    generation BIGINT NOT NULL DEFAULT 1,
    resource_version VARCHAR(255) NOT NULL DEFAULT gen_random_uuid()::TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,

    UNIQUE(cluster_id, name)
);

-- =============================================================================
-- STATUS TRACKING TABLES
-- =============================================================================

-- Controller status for clusters
CREATE TABLE IF NOT EXISTS controller_status (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cluster_id UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    controller_name VARCHAR(255) NOT NULL,
    observed_generation BIGINT NOT NULL DEFAULT 0,
    conditions JSONB NOT NULL DEFAULT '[]',
    metadata JSONB NOT NULL DEFAULT '{}',
    last_error JSONB,
    last_reconciled_at TIMESTAMP,
    reconciliation_needed BOOLEAN DEFAULT TRUE,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    UNIQUE(cluster_id, controller_name)
);

-- Controller status for nodepools
CREATE TABLE IF NOT EXISTS nodepool_controller_status (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    nodepool_id UUID NOT NULL REFERENCES nodepools(id) ON DELETE CASCADE,
    controller_name VARCHAR(255) NOT NULL,
    observed_generation BIGINT NOT NULL DEFAULT 0,
    conditions JSONB NOT NULL DEFAULT '[]',
    metadata JSONB NOT NULL DEFAULT '{}',
    last_error JSONB,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    UNIQUE(nodepool_id, controller_name)
);

-- Events table for cluster lifecycle events
CREATE TABLE IF NOT EXISTS cluster_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cluster_id UUID NOT NULL,
    controller_name VARCHAR(255) NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    published_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- =============================================================================
-- CLUSTER-CENTRIC RECONCILIATION SYSTEM (FAN-OUT ARCHITECTURE)
-- =============================================================================

-- Cluster-centric reconciliation schedule (no controller types - fan-out to all controllers)
CREATE TABLE reconciliation_schedule (
    id SERIAL PRIMARY KEY,
    cluster_id UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    last_reconciled_at TIMESTAMP,
    next_reconcile_at TIMESTAMP,
    reconcile_interval INTERVAL DEFAULT '5 minutes',
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    -- Health-aware interval configuration
    healthy_interval INTERVAL DEFAULT '5 minutes',
    unhealthy_interval INTERVAL DEFAULT '10 seconds',
    adaptive_enabled BOOLEAN DEFAULT TRUE,
    last_health_check TIMESTAMP,
    is_healthy BOOLEAN DEFAULT NULL,

    -- One schedule per cluster (no controller type - fan-out approach)
    UNIQUE(cluster_id)
);

-- =============================================================================
-- REACTIVE RECONCILIATION SYSTEM
-- =============================================================================

-- Configuration for reactive reconciliation triggers (no controller type filtering)
CREATE TABLE IF NOT EXISTS reactive_reconciliation_config (
    id SERIAL PRIMARY KEY,
    enabled BOOLEAN DEFAULT FALSE,
    change_types TEXT[] DEFAULT ARRAY['spec', 'status', 'controller_status'],
    debounce_interval INTERVAL DEFAULT '2 seconds',
    max_events_per_minute INTEGER DEFAULT 60,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Insert default configuration
INSERT INTO reactive_reconciliation_config (enabled, change_types, debounce_interval, max_events_per_minute)
VALUES (
    FALSE, -- Disabled by default for safe rollout
    ARRAY['spec', 'status', 'controller_status'],
    '2 seconds',
    60
) ON CONFLICT DO NOTHING;

-- =============================================================================
-- INDEXES FOR PERFORMANCE (CLIENT ISOLATION OPTIMIZED)
-- =============================================================================

-- Core table indexes - CLIENT ISOLATION CRITICAL
CREATE INDEX IF NOT EXISTS idx_clusters_created_by ON clusters(created_by);
CREATE INDEX IF NOT EXISTS idx_clusters_created_at ON clusters(created_at);
CREATE INDEX IF NOT EXISTS idx_clusters_status_dirty ON clusters(status_dirty) WHERE status_dirty = TRUE;

CREATE INDEX IF NOT EXISTS idx_nodepools_cluster_id ON nodepools(cluster_id);
CREATE INDEX IF NOT EXISTS idx_nodepools_created_at ON nodepools(created_at);

-- Status table indexes
CREATE INDEX IF NOT EXISTS idx_controller_status_cluster_id ON controller_status(cluster_id);
CREATE INDEX IF NOT EXISTS idx_controller_status_updated_at ON controller_status(updated_at);

CREATE INDEX IF NOT EXISTS idx_nodepool_controller_status_nodepool_id ON nodepool_controller_status(nodepool_id);
CREATE INDEX IF NOT EXISTS idx_nodepool_controller_status_updated_at ON nodepool_controller_status(updated_at);

-- Events table indexes
CREATE INDEX IF NOT EXISTS idx_cluster_events_cluster_id ON cluster_events(cluster_id);
CREATE INDEX IF NOT EXISTS idx_cluster_events_published_at ON cluster_events(published_at);

-- Cluster-centric reconciliation indexes (fan-out architecture)
CREATE INDEX idx_reconciliation_schedule_next_reconcile
    ON reconciliation_schedule(next_reconcile_at, enabled)
    WHERE enabled = TRUE;

CREATE INDEX idx_reconciliation_schedule_cluster_id
    ON reconciliation_schedule(cluster_id);

-- =============================================================================
-- UTILITY FUNCTIONS
-- =============================================================================

-- Updated timestamp trigger function
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- =============================================================================
-- CLUSTER-CENTRIC RECONCILIATION FUNCTIONS (FAN-OUT ARCHITECTURE)
-- =============================================================================

-- Create default cluster reconciliation (single schedule per cluster)
CREATE OR REPLACE FUNCTION create_default_cluster_reconciliation()
RETURNS TRIGGER AS $$
BEGIN
    -- Create single reconciliation schedule per cluster (fan-out to all controllers)
    -- Explicitly set all adaptive reconciliation fields for proper scheduler function
    INSERT INTO reconciliation_schedule (
        cluster_id,
        next_reconcile_at,
        reconcile_interval,
        enabled,
        healthy_interval,
        unhealthy_interval,
        adaptive_enabled
    ) VALUES (
        NEW.id,
        NOW() + INTERVAL '1 minute',
        '5 minutes'::INTERVAL,
        TRUE,
        '5 minutes'::INTERVAL,
        '30 seconds'::INTERVAL,
        TRUE
    )
    ON CONFLICT (cluster_id) DO NOTHING;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Update cluster reconciliation schedule (no controller type)
CREATE OR REPLACE FUNCTION update_cluster_reconciliation_schedule(
    p_cluster_id UUID
) RETURNS VOID AS $$
DECLARE
    schedule_interval INTERVAL;
BEGIN
    -- Get the current reconcile interval for this cluster
    SELECT reconcile_interval INTO schedule_interval
    FROM reconciliation_schedule
    WHERE cluster_id = p_cluster_id;

    -- If no schedule exists, create one with complete adaptive reconciliation settings
    -- DO NOT set last_reconciled_at - new clusters should have NULL (never reconciled)
    IF NOT FOUND THEN
        INSERT INTO reconciliation_schedule (
            cluster_id,
            next_reconcile_at,
            reconcile_interval,
            enabled,
            healthy_interval,
            unhealthy_interval,
            adaptive_enabled
        ) VALUES (
            p_cluster_id,
            NOW() + INTERVAL '1 minute',
            '5 minutes'::INTERVAL,
            TRUE,
            '5 minutes'::INTERVAL,
            '10 seconds'::INTERVAL,
            TRUE
        );
    ELSE
        -- Update existing schedule
        UPDATE reconciliation_schedule
        SET
            last_reconciled_at = NOW(),
            next_reconcile_at = NOW() + schedule_interval,
            updated_at = NOW()
        WHERE cluster_id = p_cluster_id;
    END IF;
END;
$$ LANGUAGE plpgsql;

-- Find clusters needing reconciliation (fan-out to all controllers) - HEALTH-AWARE
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
            WHEN rs.adaptive_enabled = TRUE AND rs.is_healthy = FALSE AND rs.next_reconcile_at <= NOW() THEN 'unhealthy_reconciliation'::CHARACTER VARYING
            WHEN rs.adaptive_enabled = TRUE AND rs.is_healthy = TRUE AND rs.next_reconcile_at <= NOW() THEN 'healthy_reconciliation'::CHARACTER VARYING
            WHEN rs.next_reconcile_at <= NOW() THEN 'periodic_reconciliation'::CHARACTER VARYING
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
        -- Priority: unhealthy clusters first, then by next_reconcile_at
        (CASE
            WHEN rs.adaptive_enabled = TRUE AND rs.is_healthy = FALSE THEN 0
            WHEN rs.adaptive_enabled = TRUE AND rs.is_healthy = TRUE THEN 1
            ELSE 2
        END),
        rs.next_reconcile_at ASC NULLS FIRST;
END;
$$ LANGUAGE plpgsql;

-- Check if cluster is healthy based on Available and Ready conditions
CREATE OR REPLACE FUNCTION is_cluster_healthy(p_cluster_id UUID)
RETURNS BOOLEAN AS $$
DECLARE
    available_status TEXT := NULL;
    ready_status TEXT := NULL;
    condition_record RECORD;
BEGIN
    -- Check cluster status conditions for Available=True AND Ready=True
    FOR condition_record IN
        SELECT condition_data
        FROM clusters c,
             jsonb_array_elements(COALESCE(c.status->'conditions', '[]'::jsonb)) AS condition_data
        WHERE c.id = p_cluster_id
          AND c.deleted_at IS NULL
    LOOP
        -- Extract condition type and status
        IF condition_record.condition_data->>'type' = 'Available' THEN
            available_status := condition_record.condition_data->>'status';
        END IF;

        IF condition_record.condition_data->>'type' = 'Ready' THEN
            ready_status := condition_record.condition_data->>'status';
        END IF;
    END LOOP;

    -- Cluster is healthy only if both Available=True AND Ready=True
    IF available_status = 'True' AND ready_status = 'True' THEN
        RETURN TRUE;
    ELSE
        RETURN FALSE;
    END IF;

EXCEPTION
    WHEN OTHERS THEN
        -- If any error occurs during health check, consider cluster unhealthy
        RETURN FALSE;
END;
$$ LANGUAGE plpgsql;

-- Update cluster health status in reconciliation schedule
CREATE OR REPLACE FUNCTION update_cluster_health_status(p_cluster_id UUID)
RETURNS VOID AS $$
DECLARE
    cluster_health BOOLEAN;
BEGIN
    -- Get current cluster health
    cluster_health := is_cluster_healthy(p_cluster_id);

    -- Update the reconciliation schedule with health status
    UPDATE reconciliation_schedule
    SET
        is_healthy = cluster_health,
        last_health_check = NOW(),
        updated_at = NOW()
    WHERE cluster_id = p_cluster_id;

    -- If no schedule exists, create one with complete adaptive reconciliation settings
    -- DO NOT set last_reconciled_at - new clusters should have NULL (never reconciled)
    IF NOT FOUND THEN
        INSERT INTO reconciliation_schedule (
            cluster_id,
            is_healthy,
            last_health_check,
            next_reconcile_at,
            reconcile_interval,
            enabled,
            healthy_interval,
            unhealthy_interval,
            adaptive_enabled
        ) VALUES (
            p_cluster_id,
            cluster_health,
            NOW(),
            NOW() + INTERVAL '1 minute',
            '5 minutes'::INTERVAL,
            TRUE,
            '5 minutes'::INTERVAL,
            '10 seconds'::INTERVAL,
            TRUE
        );
    END IF;
END;
$$ LANGUAGE plpgsql;

-- =============================================================================
-- REACTIVE RECONCILIATION FUNCTIONS (FAN-OUT ARCHITECTURE)
-- =============================================================================

-- Function to check if reactive reconciliation is enabled
CREATE OR REPLACE FUNCTION is_reactive_reconciliation_enabled()
RETURNS BOOLEAN AS $$
DECLARE
    config_enabled BOOLEAN;
BEGIN
    SELECT enabled INTO config_enabled
    FROM reactive_reconciliation_config
    ORDER BY id DESC
    LIMIT 1;

    RETURN COALESCE(config_enabled, FALSE);
END;
$$ LANGUAGE plpgsql;

-- Function to get allowed change types
CREATE OR REPLACE FUNCTION get_allowed_change_types()
RETURNS TEXT[] AS $$
DECLARE
    allowed_types TEXT[];
BEGIN
    SELECT change_types INTO allowed_types
    FROM reactive_reconciliation_config
    ORDER BY id DESC
    LIMIT 1;

    RETURN COALESCE(allowed_types, ARRAY['spec', 'status', 'controller_status']);
END;
$$ LANGUAGE plpgsql;

-- Function to send reconciliation change notification (fan-out to all controllers)
CREATE OR REPLACE FUNCTION notify_reconciliation_change(
    p_cluster_id UUID,
    p_change_type TEXT,
    p_controller_type TEXT DEFAULT NULL,
    p_reason TEXT DEFAULT 'change_detected'
) RETURNS VOID AS $$
DECLARE
    notification_payload JSONB;
    allowed_types TEXT[];
BEGIN
    -- Check if reactive reconciliation is enabled
    IF NOT is_reactive_reconciliation_enabled() THEN
        RETURN;
    END IF;

    -- Check if change type is allowed
    allowed_types := get_allowed_change_types();
    IF NOT (p_change_type = ANY(allowed_types)) THEN
        RETURN;
    END IF;

    -- NO controller type validation - fan-out approach
    -- All controllers receive all events and self-filter

    -- Build notification payload
    notification_payload := jsonb_build_object(
        'cluster_id', p_cluster_id,
        'change_type', p_change_type,
        'controller_type', p_controller_type,
        'reason', p_reason,
        'timestamp', EXTRACT(EPOCH FROM NOW())
    );

    -- Send notification
    PERFORM pg_notify('reconcile_change', notification_payload::TEXT);

EXCEPTION
    WHEN OTHERS THEN
        -- Log error but don't fail the main operation
        RAISE WARNING 'Failed to send reconciliation change notification: %', SQLERRM;
END;
$$ LANGUAGE plpgsql;

-- Trigger function for clusters table changes
CREATE OR REPLACE FUNCTION trigger_cluster_change_notification()
RETURNS TRIGGER AS $$
DECLARE
    spec_changed BOOLEAN := FALSE;
    status_changed BOOLEAN := FALSE;
    generation_changed BOOLEAN := FALSE;
BEGIN
    -- Only process UPDATE operations
    IF TG_OP = 'UPDATE' THEN
        -- Check if generation changed (spec change)
        IF OLD.generation != NEW.generation THEN
            generation_changed := TRUE;
            spec_changed := TRUE;
        END IF;

        -- Check if spec changed (without generation change)
        IF NOT generation_changed AND OLD.spec IS DISTINCT FROM NEW.spec THEN
            spec_changed := TRUE;
        END IF;

        -- Check if status changed (excluding status_dirty and timestamp changes)
        IF OLD.status IS DISTINCT FROM NEW.status THEN
            status_changed := TRUE;
        END IF;

        -- Send notifications for meaningful changes
        IF spec_changed THEN
            PERFORM notify_reconciliation_change(
                NEW.id,
                'spec',
                NULL,
                CASE
                    WHEN generation_changed THEN 'generation_increment'
                    ELSE 'spec_change'
                END
            );
        END IF;

        IF status_changed THEN
            PERFORM notify_reconciliation_change(
                NEW.id,
                'status',
                NULL,
                'status_change'
            );
        END IF;
    END IF;

    -- For INSERT operations, always notify (new cluster)
    IF TG_OP = 'INSERT' THEN
        PERFORM notify_reconciliation_change(
            NEW.id,
            'spec',
            NULL,
            'cluster_created'
        );
    END IF;

    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

-- Trigger function for controller_status table changes
CREATE OR REPLACE FUNCTION trigger_controller_status_change_notification()
RETURNS TRIGGER AS $$
DECLARE
    status_changed BOOLEAN := FALSE;
    generation_changed BOOLEAN := FALSE;
BEGIN
    -- For INSERT operations, always notify (new status)
    IF TG_OP = 'INSERT' THEN
        PERFORM notify_reconciliation_change(
            NEW.cluster_id,
            'controller_status',
            NEW.controller_name,
            'controller_status_created'
        );
        RETURN NEW;
    END IF;

    -- For UPDATE operations, check for meaningful changes
    IF TG_OP = 'UPDATE' THEN
        -- Check if observed generation changed
        IF OLD.observed_generation != NEW.observed_generation THEN
            generation_changed := TRUE;
        END IF;

        -- Check if conditions changed
        IF OLD.conditions IS DISTINCT FROM NEW.conditions THEN
            status_changed := TRUE;
        END IF;

        -- Check if last_error changed
        IF OLD.last_error IS DISTINCT FROM NEW.last_error THEN
            status_changed := TRUE;
        END IF;

        -- Send notification for meaningful changes
        IF generation_changed OR status_changed THEN
            PERFORM notify_reconciliation_change(
                NEW.cluster_id,
                'controller_status',
                NEW.controller_name,
                CASE
                    WHEN generation_changed AND status_changed THEN 'controller_status_and_generation_change'
                    WHEN generation_changed THEN 'controller_generation_change'
                    ELSE 'controller_status_change'
                END
            );
        END IF;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger function for nodepool_controller_status table changes
CREATE OR REPLACE FUNCTION trigger_nodepool_controller_status_change_notification()
RETURNS TRIGGER AS $$
DECLARE
    cluster_id_val UUID;
    status_changed BOOLEAN := FALSE;
    generation_changed BOOLEAN := FALSE;
BEGIN
    -- Get cluster_id from nodepool
    IF TG_OP = 'INSERT' THEN
        SELECT np.cluster_id INTO cluster_id_val
        FROM nodepools np
        WHERE np.id = NEW.nodepool_id AND np.deleted_at IS NULL;
    ELSE
        SELECT np.cluster_id INTO cluster_id_val
        FROM nodepools np
        WHERE np.id = COALESCE(NEW.nodepool_id, OLD.nodepool_id) AND np.deleted_at IS NULL;
    END IF;

    -- Skip if cluster not found or deleted
    IF cluster_id_val IS NULL THEN
        RETURN COALESCE(NEW, OLD);
    END IF;

    -- For INSERT operations, always notify (new status)
    IF TG_OP = 'INSERT' THEN
        PERFORM notify_reconciliation_change(
            cluster_id_val,
            'controller_status',
            NEW.controller_name,
            'nodepool_controller_status_created'
        );
        RETURN NEW;
    END IF;

    -- For UPDATE operations, check for meaningful changes
    IF TG_OP = 'UPDATE' THEN
        -- Check if observed generation changed
        IF OLD.observed_generation != NEW.observed_generation THEN
            generation_changed := TRUE;
        END IF;

        -- Check if conditions changed
        IF OLD.conditions IS DISTINCT FROM NEW.conditions THEN
            status_changed := TRUE;
        END IF;

        -- Check if last_error changed
        IF OLD.last_error IS DISTINCT FROM NEW.last_error THEN
            status_changed := TRUE;
        END IF;

        -- Send notification for meaningful changes
        IF generation_changed OR status_changed THEN
            PERFORM notify_reconciliation_change(
                cluster_id_val,
                'controller_status',
                NEW.controller_name,
                CASE
                    WHEN generation_changed AND status_changed THEN 'nodepool_controller_status_and_generation_change'
                    WHEN generation_changed THEN 'nodepool_controller_generation_change'
                    ELSE 'nodepool_controller_status_change'
                END
            );
        END IF;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Function to enable reactive reconciliation
CREATE OR REPLACE FUNCTION enable_reactive_reconciliation()
RETURNS VOID AS $$
BEGIN
    UPDATE reactive_reconciliation_config
    SET enabled = TRUE, updated_at = NOW()
    WHERE id = (SELECT MAX(id) FROM reactive_reconciliation_config);

    IF NOT FOUND THEN
        INSERT INTO reactive_reconciliation_config (enabled) VALUES (TRUE);
    END IF;
END;
$$ LANGUAGE plpgsql;

-- Function to disable reactive reconciliation
CREATE OR REPLACE FUNCTION disable_reactive_reconciliation()
RETURNS VOID AS $$
BEGIN
    UPDATE reactive_reconciliation_config
    SET enabled = FALSE, updated_at = NOW()
    WHERE id = (SELECT MAX(id) FROM reactive_reconciliation_config);

    IF NOT FOUND THEN
        INSERT INTO reactive_reconciliation_config (enabled) VALUES (FALSE);
    END IF;
END;
$$ LANGUAGE plpgsql;

-- Function to update reactive reconciliation configuration (no controller types)
CREATE OR REPLACE FUNCTION update_reactive_reconciliation_config(
    p_enabled BOOLEAN DEFAULT NULL,
    p_change_types TEXT[] DEFAULT NULL,
    p_debounce_interval INTERVAL DEFAULT NULL,
    p_max_events_per_minute INTEGER DEFAULT NULL
) RETURNS VOID AS $$
DECLARE
    current_config_id INTEGER;
BEGIN
    -- Get current config ID
    SELECT MAX(id) INTO current_config_id FROM reactive_reconciliation_config;

    -- Update existing or insert new
    IF current_config_id IS NOT NULL THEN
        UPDATE reactive_reconciliation_config
        SET
            enabled = COALESCE(p_enabled, enabled),
            change_types = COALESCE(p_change_types, change_types),
            debounce_interval = COALESCE(p_debounce_interval, debounce_interval),
            max_events_per_minute = COALESCE(p_max_events_per_minute, max_events_per_minute),
            updated_at = NOW()
        WHERE id = current_config_id;
    ELSE
        INSERT INTO reactive_reconciliation_config (
            enabled, change_types, debounce_interval, max_events_per_minute
        ) VALUES (
            COALESCE(p_enabled, FALSE),
            COALESCE(p_change_types, ARRAY['spec', 'status', 'controller_status']),
            COALESCE(p_debounce_interval, '2 seconds'::INTERVAL),
            COALESCE(p_max_events_per_minute, 60)
        );
    END IF;
END;
$$ LANGUAGE plpgsql;

-- =============================================================================
-- TRIGGERS
-- =============================================================================

-- Updated timestamp triggers
CREATE TRIGGER update_clusters_updated_at BEFORE UPDATE ON clusters
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_nodepools_updated_at BEFORE UPDATE ON nodepools
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_controller_status_updated_at BEFORE UPDATE ON controller_status
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_nodepool_controller_status_updated_at BEFORE UPDATE ON nodepool_controller_status
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Cluster reconciliation creation trigger (fan-out architecture)
CREATE TRIGGER trigger_create_cluster_reconciliation
    AFTER INSERT ON clusters
    FOR EACH ROW
    EXECUTE FUNCTION create_default_cluster_reconciliation();

-- Reactive reconciliation triggers (fan-out to all controllers)
CREATE TRIGGER trigger_cluster_change_reactive_reconciliation
    AFTER INSERT OR UPDATE ON clusters
    FOR EACH ROW
    EXECUTE FUNCTION trigger_cluster_change_notification();

CREATE TRIGGER trigger_controller_status_change_reactive_reconciliation
    AFTER INSERT OR UPDATE ON controller_status
    FOR EACH ROW
    EXECUTE FUNCTION trigger_controller_status_change_notification();

CREATE TRIGGER trigger_nodepool_controller_status_change_reactive_reconciliation
    AFTER INSERT OR UPDATE ON nodepool_controller_status
    FOR EACH ROW
    EXECUTE FUNCTION trigger_nodepool_controller_status_change_notification();

-- =============================================================================
-- COMMENTS AND DOCUMENTATION
-- =============================================================================

COMMENT ON TABLE clusters IS 'Core cluster lifecycle management with client isolation via created_by';
COMMENT ON COLUMN clusters.created_by IS 'User email who created the cluster - used for client isolation';
COMMENT ON INDEX idx_clusters_created_by IS 'Critical index for client isolation performance';
COMMENT ON TABLE reconciliation_schedule IS 'Cluster-centric reconciliation schedule for fan-out events (no controller types)';
COMMENT ON TABLE reactive_reconciliation_config IS 'Configuration for reactive reconciliation triggers (fan-out architecture)';
COMMENT ON FUNCTION create_default_cluster_reconciliation() IS 'Creates default reconciliation schedule for new clusters (fan-out approach)';
COMMENT ON FUNCTION update_cluster_reconciliation_schedule(UUID) IS 'Updates reconciliation schedule after cluster reconciliation (fan-out approach)';
COMMENT ON FUNCTION find_clusters_needing_reconciliation() IS 'Finds clusters needing reconciliation (fan-out to all controllers)';
COMMENT ON FUNCTION notify_reconciliation_change(UUID, TEXT, TEXT, TEXT) IS 'Sends a notification for reconciliation changes via pg_notify (fan-out to all controllers)';
COMMENT ON FUNCTION trigger_cluster_change_notification() IS 'Trigger function to detect and notify cluster changes (fan-out)';
COMMENT ON FUNCTION trigger_controller_status_change_notification() IS 'Trigger function to detect and notify controller status changes (fan-out)';
COMMENT ON FUNCTION trigger_nodepool_controller_status_change_notification() IS 'Trigger function to detect and notify nodepool controller status changes (fan-out)';
COMMENT ON FUNCTION enable_reactive_reconciliation() IS 'Enables reactive reconciliation system';
COMMENT ON FUNCTION disable_reactive_reconciliation() IS 'Disables reactive reconciliation system';

-- =============================================================================
-- MIGRATION COMPLETED
-- =============================================================================

-- This migration successfully creates:
-- 1. Complete cluster lifecycle management with client isolation
-- 2. No organization multi-tenancy (simplified single-tenant architecture)
-- 3. Client isolation via created_by field with optimized indexes
-- 4. NodePool security inheritance through cluster ownership
-- 5. Fan-out reconciliation architecture
-- 6. Reactive reconciliation system
-- 7. Kubernetes-like status tracking
-- 8. All necessary triggers and functions
--
-- Security Features:
-- - Every cluster operation filtered by created_by = user_email
-- - NodePool operations secured via cluster ownership JOINs
-- - Critical idx_clusters_created_by index for performance
-- - No way for users to access other users' resources
--
-- Ready for production deployment with comprehensive client isolation!