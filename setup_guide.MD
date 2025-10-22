# GPU Telemetry Pipeline - Complete Setup Guide

This project demonstrates a production-grade GPU telemetry pipeline similar to what you'd build at NVIDIA for DGX Cloud fleet management.

## Architecture Overview

```
DCGM Exporters → Go Collector → Kafka → Alert Engine → PostgreSQL
                                   ↓
                              REST API → Query/Monitor
```

## Prerequisites

- Docker & Docker Compose
- Go 1.21+ (for running services locally)
- curl or Postman (for testing APIs)

## Step 1: Start Infrastructure

Create a project directory and add all the files:

```bash
mkdir gpu-telemetry-pipeline
cd gpu-telemetry-pipeline

# Create necessary files
touch docker-compose.yml init.sql
mkdir -p cmd/collector cmd/alert-engine cmd/api-server
```

Start all infrastructure services:

```bash
docker-compose up -d
```

Wait 30 seconds for services to initialize, then verify:

```bash
# Check all services are running
docker-compose ps

# Check PostgreSQL
docker-compose exec postgres psql -U telemetry -d gpu_telemetry -c "\dt"

# Check Kafka
docker-compose exec kafka kafka-topics --bootstrap-server localhost:9093 --list
```

## Step 2: Setup Go Projects

### Initialize Go modules for each service

**Collector Service:**
```bash
cd cmd/collector
go mod init gpu-telemetry/collector
go get github.com/segmentio/kafka-go
```

**Alert Engine:**
```bash
cd ../alert-engine
go mod init gpu-telemetry/alert-engine
go get github.com/segmentio/kafka-go
go get github.com/lib/pq
```

**API Server:**
```bash
cd ../api-server
go mod init gpu-telemetry/api-server
go get github.com/gorilla/mux
go get github.com/lib/pq
```

## Step 3: Run the Services

Open **3 separate terminals** for each service:

**Terminal 1 - Collector Service:**
```bash
cd cmd/collector
go run main.go
```

Expected output:
```
Starting collector service, polling 2 nodes every 30s
Successfully collected and published metrics from node-1
Successfully collected and published metrics from node-2
Published 16 metrics to Kafka
```

**Terminal 2 - Alert Engine:**
```bash
cd cmd/alert-engine
go run alert_engine.go
```

Expected output:
```
Alert Engine started, consuming from Kafka...
Created alert ID=1: [warning] high_temperature on node-1 GPU 3
⚠️  WARNING: Sending notification for high_temperature on node-1 GPU 3
🚨 CRITICAL ACTION: Initiating workload migration from node-2 GPU 5
```

**Terminal 3 - API Server:**
```bash
cd cmd/api-server
go run api_server.go
```

Expected output:
```
Starting API server on port 8080
API Server started successfully
```

## Step 4: Test the System

### Check Node Health
```bash
curl http://localhost:8080/api/v1/nodes
```

Expected response:
```json
[
  {
    "node_id": "node-1",
    "hostname": "dgx-gpu-01.nvidia.com",
    "status": "healthy",
    "datacenter": "us-west-1",
    "last_seen": "2025-10-18T10:30:00Z",
    "active_alerts": 2
  }
]
```

### Get Latest Metrics
```bash
curl http://localhost:8080/api/v1/metrics/latest
```

### Get Active Alerts
```bash
curl http://localhost:8080/api/v1/alerts/active
```

Expected response:
```json
[
  {
    "id": 5,
    "node_id": "node-2",
    "gpu_index": 5,
    "alert_type": "high_temperature",
    "severity": "critical",
    "message": "GPU temperature is 96.3°C",
    "threshold_value": 90.0,
    "actual_value": 96.3,
    "status": "active",
    "triggered_at": "2025-10-18T10:32:15Z"
  }
]
```

### Get Metrics for Specific Node
```bash
curl http://localhost:8080/api/v1/nodes/node-1/metrics?limit=10
```

### Resolve an Alert
```bash
curl -X POST http://localhost:8080/api/v1/alerts/5/resolve
```

## Step 5: Monitor the System

### Watch Kafka Messages
```bash
docker-compose exec kafka kafka-console-consumer \
  --bootstrap-server localhost:9093 \
  --topic gpu-telemetry \
  --from-beginning
```

### Check Database Directly
```bash
docker-compose exec postgres psql -U telemetry -d gpu_telemetry

-- View recent metrics
SELECT node_id, gpu_index, temperature_celsius, collected_at 
FROM gpu_metrics 
ORDER BY collected_at DESC 
LIMIT 10;

-- View active alerts
SELECT * FROM alerts WHERE status = 'active';

-- View alert actions taken
SELECT a.alert_type, a.severity, aa.action_type, aa.action_details
FROM alerts a
JOIN alert_actions aa ON a.id = aa.alert_id
ORDER BY a.triggered_at DESC;
```

## Understanding the Data Flow

1. **Collector** (every 30s):
    - Simulates polling DCGM exporters on each node
    - Generates realistic GPU metrics (temp, power, memory, utilization)
    - Publishes to Kafka topic `gpu-telemetry`

2. **Kafka**:
    - Streams metrics to multiple consumers
    - Enables real-time processing and historical replay

3. **Alert Engine**:
    - Consumes from Kafka
    - Stores all metrics in PostgreSQL
    - Evaluates alerting rules:
        - Temperature > 90°C → Warning
        - Temperature > 95°C → Critical (triggers workload migration)
        - Power > 330W → Warning
        - Memory > 95% → Warning
    - Takes automated actions
    - Logs actions to database

4. **REST API**:
    - Provides query interface to stored data
    - Real-time node health status
    - Historical metrics
    - Alert management

## Common Issues & Troubleshooting

### Kafka Connection Refused
Wait 30-60 seconds after `docker-compose up` for Kafka to fully initialize.

### PostgreSQL Connection Error
Verify database is running:
```bash
docker-compose logs postgres
```

### No Metrics Appearing
Check collector logs for errors. Ensure Kafka topic was created:
```bash
docker-compose exec kafka kafka-topics --bootstrap-server localhost:9093 --list
```

## Customization Ideas

1. **Add More Alert Rules**: Edit `EvaluateRules()` in alert_engine.go
2. **Add Real DCGM Integration**: Replace simulated metrics with actual HTTP calls
3. **Add Dashboard**: Build React frontend consuming REST API
4. **Add Prometheus Integration**: Export metrics in Prometheus format
5. **Add Workload Migration**: Implement actual Kubernetes job migration

## Interview Talking Points

When discussing this project in your interview:

1. **Scalability**: "I designed this to handle thousands of nodes by using Kafka for decoupling and horizontal scaling"

2. **Real-time Processing**: "The alert engine processes metrics in real-time, triggering automated actions within seconds"

3. **Data Pipeline**: "Implemented both online (Kafka streaming) and offline (PostgreSQL batch) data paths"

4. **Operational Automation**: "Built rules engine that automatically migrates workloads when GPUs become unhealthy"

5. **Production Patterns**: "Used proper error handling, connection pooling, and graceful shutdown"

## Next Steps

- Add authentication to REST API
- Implement distributed tracing
- Add metrics aggregation and downsampling for long-term storage
- Build React dashboard
- Add integration tests
- Deploy to Kubernetes

## Stopping the System

```bash
# Stop all services gracefully
docker-compose down

# Stop and remove all data
docker-compose down -v
```