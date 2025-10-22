-- GPU Nodes Table
CREATE TABLE IF NOT EXISTS gpu_nodes (
                                         id SERIAL PRIMARY KEY,
                                         node_id VARCHAR(50) UNIQUE NOT NULL,
    hostname VARCHAR(255),
    datacenter VARCHAR(100),
    status VARCHAR(20) DEFAULT 'healthy',
    last_seen TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );

-- GPU Metrics Table (time-series data)
CREATE TABLE IF NOT EXISTS gpu_metrics (
                                           id BIGSERIAL PRIMARY KEY,
                                           node_id VARCHAR(50) NOT NULL,
    gpu_index INT NOT NULL,
    temperature_celsius FLOAT,
    power_watts FLOAT,
    memory_used_mb FLOAT,
    memory_total_mb FLOAT,
    utilization_percent FLOAT,
    sm_clock_mhz INT,
    collected_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (node_id) REFERENCES gpu_nodes(node_id) ON DELETE CASCADE
    );

-- Create index for time-series queries
CREATE INDEX idx_metrics_node_time ON gpu_metrics(node_id, collected_at DESC);
CREATE INDEX idx_metrics_collected_at ON gpu_metrics(collected_at DESC);

-- Alerts Table
CREATE TABLE IF NOT EXISTS alerts (
                                      id SERIAL PRIMARY KEY,
                                      node_id VARCHAR(50) NOT NULL,
    gpu_index INT,
    alert_type VARCHAR(50) NOT NULL,
    severity VARCHAR(20) NOT NULL,
    message TEXT NOT NULL,
    threshold_value FLOAT,
    actual_value FLOAT,
    status VARCHAR(20) DEFAULT 'active',
    triggered_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    resolved_at TIMESTAMP,
    FOREIGN KEY (node_id) REFERENCES gpu_nodes(node_id) ON DELETE CASCADE
    );

CREATE INDEX idx_alerts_status ON alerts(status, triggered_at DESC);
CREATE INDEX idx_alerts_node ON alerts(node_id, triggered_at DESC);

-- Alert Actions Table (tracks what actions were taken)
CREATE TABLE IF NOT EXISTS alert_actions (
                                             id SERIAL PRIMARY KEY,
                                             alert_id INT NOT NULL,
                                             action_type VARCHAR(50) NOT NULL,
    action_status VARCHAR(20) DEFAULT 'pending',
    action_details JSONB,
    executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (alert_id) REFERENCES alerts(id) ON DELETE CASCADE
    );

-- Insert sample nodes
INSERT INTO gpu_nodes (node_id, hostname, datacenter, status) VALUES
                                                                  ('node-1', 'dgx-gpu-01.nvidia.com', 'us-west-1', 'healthy'),
                                                                  ('node-2', 'dgx-gpu-02.nvidia.com', 'us-west-1', 'healthy')
    ON CONFLICT (node_id) DO NOTHING;

-- Create a view for latest metrics per node
CREATE OR REPLACE VIEW latest_gpu_metrics AS
SELECT DISTINCT ON (node_id, gpu_index)
    node_id,
    gpu_index,
    temperature_celsius,
    power_watts,
    memory_used_mb,
    memory_total_mb,
    utilization_percent,
    sm_clock_mhz,
    collected_at
FROM gpu_metrics
ORDER BY node_id, gpu_index, collected_at DESC;