-- =============================================================================
-- ADD MISSING STATUS DIRTY TRIGGER
-- =============================================================================
-- This migration adds the missing function and trigger that marks clusters as
-- dirty when controller status updates occur, which is required for proper
-- status aggregation.

-- Function to mark cluster status as dirty when controller status changes
CREATE OR REPLACE FUNCTION mark_cluster_status_dirty()
RETURNS TRIGGER AS $$
BEGIN
    -- Mark the cluster as dirty so status aggregation will be triggered
    UPDATE clusters
    SET status_dirty = TRUE, updated_at = NOW()
    WHERE id = NEW.cluster_id;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to automatically mark clusters dirty when controller status changes
CREATE TRIGGER controller_status_dirty_trigger
    AFTER INSERT OR UPDATE ON controller_status
    FOR EACH ROW
    EXECUTE FUNCTION mark_cluster_status_dirty();

-- Also add trigger for nodepool controller status changes
CREATE TRIGGER nodepool_controller_status_dirty_trigger
    AFTER INSERT OR UPDATE ON nodepool_controller_status
    FOR EACH ROW
    EXECUTE FUNCTION mark_cluster_status_dirty();

-- Add comments for documentation
COMMENT ON FUNCTION mark_cluster_status_dirty() IS 'Marks cluster as dirty when controller status changes to trigger aggregation';
COMMENT ON TRIGGER controller_status_dirty_trigger ON controller_status IS 'Automatically marks clusters dirty for status aggregation';
COMMENT ON TRIGGER nodepool_controller_status_dirty_trigger ON nodepool_controller_status IS 'Automatically marks clusters dirty for nodepool status aggregation';