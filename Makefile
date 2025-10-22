.PHONY: help setup start-infra stop-infra run-collector run-alert run-api test clean

help:
	@echo "GPU Telemetry Pipeline - Available Commands"
	@echo ""
	@echo "Setup:"
	@echo "  make setup          - Initialize Go modules for all services"
	@echo "  make start-infra    - Start Docker infrastructure (Postgres, Kafka, Zookeeper)"
	@echo ""
	@echo "Run Services:"
	@echo "  make run-collector  - Run the telemetry collector service"
	@echo "  make run-alert      - Run the alert engine service"
	@echo "  make run-api        - Run the REST API server"
	@echo ""
	@echo "Testing:"
	@echo "  make test           - Run API tests"
	@echo "  make test-infra     - Verify infrastructure is running"
	@echo ""
	@echo "Cleanup:"
	@echo "  make stop-infra     - Stop Docker infrastructure"
	@echo "  make clean          - Stop infrastructure and remove volumes"
	@echo ""
	@echo "Monitoring:"
	@echo "  make logs-kafka     - Watch Kafka messages"
	@echo "  make logs-db        - View database logs"
	@echo "  make db-shell       - Connect to PostgreSQL"

setup:
	@echo "Setting up Go modules..."
	cd cmd/collector && go mod init gpu-telemetry/collector 2>/dev/null || true
	cd cmd/collector && go get github.com/segmentio/kafka-go
	@echo ""
	cd cmd/alert-engine && go mod init gpu-telemetry/alert-engine 2>/dev/null || true
	cd cmd/alert-engine && go get github.com/segmentio/kafka-go github.com/lib/pq
	@echo ""
	cd cmd/api-server && go mod init gpu-telemetry/api-server 2>/dev/null || true
	cd cmd/api-server && go get github.com/gorilla/mux github.com/lib/pq
	@echo ""
	@echo "✓ Setup complete!"

start-infra:
	@echo "Starting infrastructure services..."
	docker-compose up -d
	@echo ""
	@echo "Waiting for services to initialize (30 seconds)..."
	@sleep 30
	@echo ""
	@echo "✓ Infrastructure started!"
	@echo ""
	@echo "Services running:"
	@docker-compose ps

stop-infra:
	@echo "Stopping infrastructure..."
	docker-compose down
	@echo "✓ Infrastructure stopped"

clean:
	@echo "Cleaning up (removing all data)..."
	docker-compose down -v
	@echo "✓ Cleanup complete"

run-collector:
	@echo "Starting Collector Service..."
	@echo "This will poll GPU nodes every 30 seconds"
	@echo "Press Ctrl+C to stop"
	@echo ""
	cd cmd/collector && go run main.go

run-alert:
	@echo "Starting Alert Engine..."
	@echo "This will consume from Kafka and evaluate alert rules"
	@echo "Press Ctrl+C to stop"
	@echo ""
	cd cmd/alert-engine && go run alert_engine.go

run-api:
	@echo "Starting REST API Server on port 8080..."
	@echo "API will be available at http://localhost:8080"
	@echo "Press Ctrl+C to stop"
	@echo ""
	cd cmd/api-server && go run api_server.go

test:
	@echo "Running API tests..."
	@chmod +x test_api.sh
	@./test_api.sh

test-infra:
	@echo "Testing infrastructure..."
	@echo ""
	@echo "1. PostgreSQL:"
	@docker-compose exec -T postgres psql -U telemetry -d gpu_telemetry -c "SELECT COUNT(*) FROM gpu_nodes;" || echo "✗ PostgreSQL not ready"
	@echo ""
	@echo "2. Kafka:"
	@docker-compose exec -T kafka kafka-topics --bootstrap-server localhost:9093 --list || echo "✗ Kafka not ready"
	@echo ""
	@echo "3. Services Status:"
	@docker-compose ps

logs-kafka:
	@echo "Watching Kafka messages (Ctrl+C to stop)..."
	docker-compose exec kafka kafka-console-consumer \
		--bootstrap-server localhost:9093 \
		--topic gpu-telemetry \
		--from-beginning

logs-db:
	@echo "Database logs:"
	docker-compose logs --tail=50 -f postgres

db-shell:
	@echo "Connecting to PostgreSQL..."
	@echo "Useful commands:"
	@echo "  \\dt                     - List tables"
	@echo "  SELECT * FROM alerts;   - View alerts"
	@echo "  \\q                      - Quit"
	@echo ""
	docker-compose exec postgres psql -U telemetry -d gpu_telemetry

db-metrics:
	@echo "Recent metrics from database:"
	@docker-compose exec -T postgres psql -U telemetry -d gpu_telemetry -c "\
		SELECT node_id, gpu_index, \
		       ROUND(temperature_celsius::numeric, 1) as temp, \
		       ROUND(power_watts::numeric, 1) as power, \
		       collected_at \
		FROM gpu_metrics \
		ORDER BY collected_at DESC \
		LIMIT 20;"

db-alerts:
	@echo "Active alerts from database:"
	@docker-compose exec -T postgres psql -U telemetry -d gpu_telemetry -c "\
		SELECT id, node_id, gpu_index, alert_type, severity, \
		       ROUND(actual_value::numeric, 1) as value, \
		       triggered_at \
		FROM alerts \
		WHERE status = 'active' \
		ORDER BY triggered_at DESC;"

quick-start: start-infra
	@echo ""
	@echo "Infrastructure started! Now open 3 terminals and run:"
	@echo ""
	@echo "Terminal 1: make run-collector"
	@echo "Terminal 2: make run-alert"
	@echo "Terminal 3: make run-api"
	@echo ""
	@echo "Then test with: make test"