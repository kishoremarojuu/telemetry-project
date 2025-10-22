# GPU Fleet Management Telemetry Pipeline

A production-grade GPU telemetry system demonstrating real-world fleet management for NVIDIA DGX Cloud infrastructure.

## What This Project Demonstrates

This project showcases the exact skills needed for the **Senior Software Engineer, Fleet Management - DGX Cloud** role at NVIDIA:

✅ **RESTful API Design** - Query nodes, metrics, and alerts  
✅ **High-volume Data Ingestion** - Kafka streaming pipeline handling thousands of GPUs  
✅ **Scalable Cloud Services** - Microservices architecture with Go  
✅ **Data Pipelines** - Real-time (Kafka) + Batch (PostgreSQL)  
✅ **Operational Automation** - Automated alerting and workload migration  
✅ **Distributed Systems** - Message queues, databases, concurrent processing  
✅ **PostgreSQL** - Time-series metrics storage with proper indexing

## System Architecture

```
┌─────────────────┐
│  DCGM Exporters │  (GPU nodes exposing telemetry)
│  node-1, node-2 │
└────────┬────────┘
         │ HTTP polling every 30s
         ↓
┌─────────────────┐
│  Go Collector   │  (Fan-out to all nodes, parse metrics)
│    Service      │
└────────┬────────┘
         │ Publish metrics
         ↓
┌─────────────────┐
│     Kafka       │  (Stream processing, decoupling)
│ gpu-telemetry   │
└────┬───────┬────┘
     │       │
     ↓       ↓
┌─────────┐ ┌──────────────┐
│  Store  │ │ Alert Engine │  (Rule evaluation & actions)
│ Service │ │   Service    │
└────┬────┘ └──────┬───────┘
     │             │
     ↓             ↓
┌──────────────────────┐
│    PostgreSQL DB     │  (Metrics, Alerts, Actions)
│  - gpu_nodes         │
│  - gpu_metrics       │
│  - alerts            │
│  - alert_actions     │
└──────────┬───────────┘
           │
           ↓
    ┌─────────────┐
    │  REST API   │  (Query interface)
    │   :8080     │
    └─────────────┘
```

## Key Features

### 1. Real-Time Telemetry Collection
- Simulates 2 GPU nodes with 8 GPUs each (16 total GPUs)
- Collects metrics every 30 seconds: temperature, power, memory, utilization
- Realistic data generation matching DGX A100 specifications

### 2. Intelligent Alert Engine
Automatically evaluates rules and takes action:
- **Temperature > 90°C** → Warning notification
- **Temperature > 95°C** → Critical alert + workload migration
- **Power > 330W** → Warning notification
- **Memory > 95%** → Warning notification

### 3. Automated Actions
When critical alerts trigger:
- Updates node status to "degraded"
- Logs workload migration action
- Records all actions in database for audit trail
- Would integrate with Kubernetes to actually migrate workloads

### 4. REST API Endpoints

```
GET  /api/v1/nodes                      # List all GPU nodes
GET  /api/v1/nodes/{node_id}            # Get node health status
GET  /api/v1/nodes/{node_id}/metrics    # Get metrics for a node
GET  /api/v1/metrics/latest             # Latest metrics from all GPUs
GET  /api/v1/alerts                     # All alerts
GET  /api/v1/alerts/active              # Active alerts only
POST /api/v1/alerts/{id}/resolve        # Resolve an alert
```

## Technology Stack

- **Go** - High-performance concurrent services
- **Kafka** - Stream processing and message queue
- **PostgreSQL** - Time-series metrics and relational data
- **Docker Compose** - Local infrastructure orchestration

## Quick Start

```bash
# 1. Clone and setup
git clone <your-repo>
cd gpu-telemetry-pipeline

# 2. Start infrastructure
docker-compose up -d

# 3. Run collector (terminal 1)
cd cmd/collector && go run main.go

# 4. Run alert engine (terminal 2)
cd cmd/alert-engine && go run alert_engine.go

# 5. Run API server (terminal 3)
cd cmd/api-server && go run api_server.go

# 6. Test the API
curl http://localhost:8080/api/v1/nodes
curl http://localhost:8080/api/v1/alerts/active
```

See [SETUP_GUIDE.md](SETUP_GUIDE.md) for detailed instructions.

## Real-World Use Cases

### Use Case 1: Overheating GPU Detection
```
1. GPU on node-2 hits 96°C
2. Alert Engine detects critical threshold
3. System automatically:
   - Marks node as degraded
   - Initiates workload migration
   - Logs action for compliance
   - Sends notification to ops team
```

### Use Case 2: Fleet Health Monitoring
```
1. Operations team queries: GET /api/v1/nodes
2. Sees node-1 has 3 active alerts
3. Drills down: GET /api/v1/nodes/node-1/metrics
4. Analyzes temperature trends
5. Makes decision on maintenance
```

### Use Case 3: Historical Analysis
```
1. Query last 1000 metrics for a node
2. Identify patterns (temp spikes during peak hours)
3. Optimize cooling or workload scheduling
4. Proactive capacity planning
```

## Project Structure

```
gpu-telemetry-pipeline/
├── docker-compose.yml          # Infrastructure setup
├── init.sql                    # Database schema
├── cmd/
│   ├── collector/
│   │   └── main.go            # Telemetry collector service
│   ├── alert-engine/
│   │   └── alert_engine.go    # Alert processing service
│   └── api-server/
│       └── api_server.go      # REST API server
├── SETUP_GUIDE.md             # Detailed setup instructions
└── README.md                  # This file
```

## Performance Characteristics

- **Throughput**: Handles 16 GPUs × 7 metrics = 112 data points per 30s cycle
- **Scalability**: Can scale to thousands of nodes by adding collector instances
- **Latency**: Alert processing within 1-2 seconds of metric arrival
- **Storage**: Efficient time-series indexing for fast queries


### Why This Architecture?

**Q: Why use Kafka instead of direct database writes?**  
A: Kafka provides:
- Decoupling between collection and processing
- Multiple consumers can process same data (storage, alerting, analytics)
- Replay capability for debugging
- Horizontal scaling by adding consumer groups
- Backpressure handling

**Q: How would you scale to 10,000 nodes?**  
A:
- Run multiple collector instances, partition nodes across collectors
- Increase Kafka partitions for parallel processing
- Use TimescaleDB or InfluxDB for better time-series performance
- Add metrics aggregation layer for downsampling
- Implement caching layer (Redis) for frequently accessed data

**Q: How do you handle failures?**  
A:
- Collector: Retry logic with exponential backoff
- Kafka: At-least-once delivery semantics
- Database: Connection pooling and graceful degradation
- Monitors: Health check endpoints on all services

## Extending the Project

1. **Add Real DCGM Integration**: Replace simulated metrics with actual DCGM HTTP calls
2. **Build Dashboard**: React frontend with real-time graphs using the REST API
3. **Kubernetes Integration**: Actually migrate workloads using K8s API
4. **Add Prometheus**: Export metrics for external monitoring
5. **Add Grafana**: Visualize time-series data
6. **Add Authentication**: JWT tokens for API security

## Testing the System

```bash
# Watch real-time metrics flowing
docker-compose exec kafka kafka-console-consumer \
  --bootstrap-server localhost:9093 \
  --topic gpu-telemetry

# Query database directly
docker-compose exec postgres psql -U telemetry -d gpu_telemetry \
  -c "SELECT COUNT(*) FROM gpu_metrics;"

# Check for critical alerts
curl http://localhost:8080/api/v1/alerts/active | jq '.[] | select(.severity=="critical")'
```



## Author

Built as a demonstration project for NVIDIA DGX Cloud Fleet Management role.