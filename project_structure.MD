# Project Structure & File Organization

## Complete Directory Layout

```
gpu-telemetry-pipeline/
│
├── README.md                          # Project overview and quick start
├── SETUP_GUIDE.md                     # Detailed setup instructions
├── PROJECT_STRUCTURE.md               # This file - explains organization
├── Makefile                           # Convenient commands for development
├── docker-compose.yml                 # Infrastructure orchestration
├── init.sql                           # Database schema initialization
├── test_api.sh                        # API testing script
│
├── cmd/                               # All executable services
│   │
│   ├── collector/                     # Telemetry Collector Service
│   │   ├── main.go                   # Collector implementation
│   │   ├── go.mod                    # Go dependencies
│   │   └── go.sum                    # Dependency checksums
│   │
│   ├── alert-engine/                  # Alert Processing Service
│   │   ├── alert_engine.go           # Alert engine implementation
│   │   ├── go.mod                    # Go dependencies
│   │   └── go.sum                    # Dependency checksums
│   │
│   └── api-server/                    # REST API Service
│       ├── api_server.go             # API server implementation
│       ├── go.mod                    # Go dependencies
│       └── go.sum                    # Dependency checksums
│
└── docs/                              # Additional documentation (optional)
    ├── architecture.md
    ├── api_reference.md
    └── deployment.md
```

## File Descriptions

### Root Level Files

#### README.md
- **Purpose**: First file developers see; provides project overview
- **Contents**:
    - What the project does
    - Architecture diagram
    - Quick start guide
    - Key features
    - Technology stack

#### SETUP_GUIDE.md
- **Purpose**: Detailed step-by-step setup instructions
- **Contents**:
    - Prerequisites
    - Infrastructure setup
    - Service configuration
    - Testing procedures
    - Troubleshooting

#### docker-compose.yml
- **Purpose**: Defines all infrastructure services
- **Services**:
    - PostgreSQL (database)
    - Zookeeper (Kafka dependency)
    - Kafka (message broker)
    - Mock DCGM exporters (for testing)
- **Networks**: Custom bridge network for service communication
- **Volumes**: Persistent storage for PostgreSQL

#### init.sql
- **Purpose**: Database schema and initial data
- **Contents**:
    - Table definitions (gpu_nodes, gpu_metrics, alerts, alert_actions)
    - Indexes for performance
    - Views for common queries
    - Sample data (2 GPU nodes)

#### Makefile
- **Purpose**: Simplifies common development tasks
- **Commands**:
    - `make setup` - Initialize Go modules
    - `make start-infra` - Start Docker services
    - `make run-*` - Run individual services
    - `make test` - Test API endpoints
    - `make clean` - Clean up everything

#### test_api.sh
- **Purpose**: Automated API testing
- **Features**:
    - Tests all REST endpoints
    - Color-coded output
    - HTTP status code validation
    - JSON formatting

### Service Files

#### cmd/collector/main.go
**Purpose**: Collects telemetry from GPU nodes

**Key Components**:
- `GPUMetric` struct - Data model for metrics
- `CollectorService` - Main service logic
- `CollectMetrics()` - Simulates DCGM polling
- `PublishToKafka()` - Sends metrics to Kafka
- `Run()` - Main collection loop (30s intervals)

**Dependencies**:
- `github.com/segmentio/kafka-go` - Kafka client

**Configuration**:
- Node list: `[]string{"node-1", "node-2"}`
- Kafka broker: `localhost:9093`
- Poll interval: `30 seconds`

#### cmd/alert-engine/alert_engine.go
**Purpose**: Processes metrics and generates alerts

**Key Components**:
- `AlertEngine` struct - Main service
- `EvaluateRules()` - Alert rule evaluation
- `StoreMetric()` - Saves to database
- `CreateAlert()` - Creates alert records
- `TakeAction()` - Automated responses
- `Run()` - Kafka consumer loop

**Alert Rules**:
- Temperature > 90°C → Warning
- Temperature > 95°C → Critical
- Power > 330W → Warning
- Memory > 95% → Warning

**Dependencies**:
- `github.com/segmentio/kafka-go` - Kafka consumer
- `github.com/lib/pq` - PostgreSQL driver

**Configuration**:
- Database: `postgresql://telemetry:telemetry123@localhost:5432/gpu_telemetry`
- Kafka: `localhost:9093`
- Consumer group: `alert-engine`

#### cmd/api-server/api_server.go
**Purpose**: REST API for querying data

**Key Components**:
- `APIServer` struct - HTTP server
- Route handlers for all endpoints
- Database query methods
- JSON response formatting

**Endpoints**:
```
GET  /health                           - Health check
GET  /api/v1/nodes                     - List nodes
GET  /api/v1/nodes/{node_id}           - Node details
GET  /api/v1/nodes/{node_id}/metrics   - Node metrics
GET  /api/v1/metrics/latest            - Latest from all
GET  /api/v1/alerts                    - All alerts
GET  /api/v1/alerts/active             - Active alerts
POST /api/v1/alerts/{id}/resolve       - Resolve alert
```

**Dependencies**:
- `github.com/gorilla/mux` - HTTP router
- `github.com/lib/pq` - PostgreSQL driver

**Configuration**:
- Port: `8080`
- Database: Same as alert engine

## Data Flow

```
1. Collector polls nodes
   └─> Generates GPUMetric structs
       └─> Publishes to Kafka topic "gpu-telemetry"

2. Alert Engine consumes from Kafka
   ├─> Stores metric in PostgreSQL (gpu_metrics table)
   └─> Evaluates alert rules
       └─> If threshold exceeded:
           ├─> Creates alert (alerts table)
           ├─> Takes action (alert_actions table)
           └─> Updates node status if critical

3. API Server queries PostgreSQL
   └─> Returns JSON responses to clients
```

## Database Schema

### gpu_nodes
Tracks all GPU nodes in the fleet
- `node_id` (PK) - Unique identifier
- `hostname` - DNS name
- `datacenter` - Location
- `status` - healthy/degraded/offline
- `last_seen` - Last telemetry timestamp

### gpu_metrics
Time-series telemetry data
- `id` (PK) - Auto-increment
- `node_id` (FK) - References gpu_nodes
- `gpu_index` - GPU number (0-7)
- `temperature_celsius` - Temperature
- `power_watts` - Power consumption
- `memory_used_mb` - Memory usage
- `memory_total_mb` - Total memory
- `utilization_percent` - GPU utilization
- `sm_clock_mhz` - Clock speed
- `collected_at` - Timestamp

**Indexes**:
- `(node_id, collected_at)` - For node-specific queries
- `(collected_at)` - For time-range queries

### alerts
Alert records
- `id` (PK) - Alert identifier
- `node_id` (FK) - Affected node
- `gpu_index` - Affected GPU
- `alert_type` - Type of alert
- `severity` - warning/critical
- `message` - Human-readable description
- `threshold_value` - Rule threshold
- `actual_value` - Measured value
- `status` - active/resolved
- `triggered_at` - When created
- `resolved_at` - When resolved

### alert_actions
Automated actions taken
- `id` (PK)
- `alert_id` (FK) - References alerts
- `action_type` - Type of action
- `action_status` - pending/executed/failed
- `action_details` - JSON metadata
- `executed_at` - Timestamp

## Configuration Files

### go.mod (per service)
Defines Go module and dependencies

Example for collector:
```go
module gpu-telemetry/collector

go 1.21

require github.com/segmentio/kafka-go v0.4.46
```

### docker-compose.yml Key Settings

**PostgreSQL**:
- Port: 5432
- User: telemetry
- Password: telemetry123
- Database: gpu_telemetry
- Volume: Persists data

**Kafka**:
- Port: 9093 (external)
- Auto-create topics: enabled
- Replication: 1 (single broker)

**Zookeeper**:
- Port: 2181
- Required for Kafka coordination

## Adding New Features

### To add a new alert rule:
1. Edit `cmd/alert-engine/alert_engine.go`
2. Add condition in `EvaluateRules()` function
3. Define threshold and severity
4. Restart alert engine

### To add a new API endpoint:
1. Edit `cmd/api-server/api_server.go`
2. Add route in `setupRoutes()`
3. Implement handler function
4. Add database query if needed
5. Restart API server

### To add a new metric:
1. Update `GPUMetric` struct in all services
2. Update `init.sql` schema (add column)
3. Update collector to generate metric
4. Update alert rules if needed
5. Restart all services

## Development Workflow

### Initial Setup
```bash
make setup          # Install Go dependencies
make start-infra    # Start Docker services
```

### Running Services
```bash
# Terminal 1
make run-collector

# Terminal 2
make run-alert

# Terminal 3
make run-api
```

### Testing
```bash
make test           # Test API
make test-infra     # Verify infrastructure
```

### Monitoring
```bash
make logs-kafka     # Watch Kafka messages
make db-metrics     # View latest metrics
make db-alerts      # View active alerts
```

### Cleanup
```bash
make stop-infra     # Stop containers
make clean          # Remove all data
```

## Best Practices

1. **Keep services independent**: Each service should run standalone
2. **Use environment variables**: For production, externalize configuration
3. **Log everything**: Use structured logging for debugging
4. **Handle errors gracefully**: Never crash on single failure
5. **Test incrementally**: Start one service at a time
6. **Monitor resource usage**: Check Docker stats
7. **Version control**: Commit working states frequently

## Next Steps

Once this base system works, consider:

1. **Add real DCGM integration**: Replace mock metrics
2. **Add authentication**: JWT tokens for API
3. **Add dashboard**: React frontend
4. **Add more nodes**: Scale testing
5. **Add Prometheus**: Export metrics
6. **Deploy to K8s**: Production deployment
7. **Add CI/CD**: Automated testing
8. **Add distributed tracing**: OpenTelemetry