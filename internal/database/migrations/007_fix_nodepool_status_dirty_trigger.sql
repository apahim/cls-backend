-- =============================================================================
-- FIX NODEPOOL CONTROLLER STATUS DIRTY TRIGGER
-- =============================================================================
-- This migration fixes the mark_cluster_status_dirty() trigger function to
-- properly handle the nodepool_controller_status table, which has nodepool_id
-- instead of cluster_id. The trigger now looks up the cluster_id via the
-- nodepools table.

CREATE OR REPLACE FUNCTION mark_cluster_status_dirty()
RETURNS TRIGGER AS $$
DECLARE
    v_cluster_id UUID;
BEGIN
    -- For controller_status table, cluster_id is directly available
    IF TG_TABLE_NAME = 'controller_status' THEN
        UPDATE clusters
        SET status_dirty = TRUE, updated_at = NOW()
        WHERE id = NEW.cluster_id;
    -- For nodepool_controller_status table, get cluster_id via nodepool
    ELSIF TG_TABLE_NAME = 'nodepool_controller_status' THEN
        SELECT cluster_id INTO v_cluster_id
        FROM nodepools
        WHERE id = NEW.nodepool_id;

        IF v_cluster_id IS NOT NULL THEN
            UPDATE clusters
            SET status_dirty = TRUE, updated_at = NOW()
            WHERE id = v_cluster_id;
        END IF;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Update comment
COMMENT ON FUNCTION mark_cluster_status_dirty() IS 'Marks cluster as dirty when controller status changes. Handles both controller_status (direct cluster_id) and nodepool_controller_status (lookup via nodepools table)';