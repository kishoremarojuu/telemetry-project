package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
	"github.com/segmentio/kafka-go"
)

type GPUMetric struct {
	NodeID             string    `json:"node_id"`
	GPUIndex           int       `json:"gpu_index"`
	TemperatureCelsius float64   `json:"temperature_celsius"`
	PowerWatts         float64   `json:"power_watts"`
	MemoryUsedMB       float64   `json:"memory_used_mb"`
	MemoryTotalMB      float64   `json:"memory_total_mb"`
	UtilizationPercent float64   `json:"utilization_percent"`
	SMClockMHz         int       `json:"sm_clock_mhz"`
	CollectedAt        time.Time `json:"collected_at"`
}

type Alert struct {
	NodeID         string
	GPUIndex       int
	AlertType      string
	Severity       string
	Message        string
	ThresholdValue float64
	ActualValue    float64
}

type AlertEngine struct {
	db          *sql.DB
	kafkaReader *kafka.Reader
}

func NewAlertEngine(dbConnStr, kafkaBroker string) (*AlertEngine, error) {
	db, err := sql.Open("postgres", dbConnStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     []string{kafkaBroker},
		Topic:       "gpu-telemetry",
		GroupID:     "alert-engine",
		MinBytes:    1,
		MaxBytes:    10e6,
		StartOffset: kafka.LastOffset,
	})

	return &AlertEngine{
		db:          db,
		kafkaReader: reader,
	}, nil
}

// EvaluateRules checks metrics against thresholds
func (ae *AlertEngine) EvaluateRules(metric GPUMetric) []Alert {
	var alerts []Alert

	// Rule 1: High temperature (> 90°C)
	if metric.TemperatureCelsius > 90.0 {
		severity := "warning"
		if metric.TemperatureCelsius > 95.0 {
			severity = "critical"
		}

		alerts = append(alerts, Alert{
			NodeID:         metric.NodeID,
			GPUIndex:       metric.GPUIndex,
			AlertType:      "high_temperature",
			Severity:       severity,
			Message:        fmt.Sprintf("GPU temperature is %.1f°C", metric.TemperatureCelsius),
			ThresholdValue: 90.0,
			ActualValue:    metric.TemperatureCelsius,
		})
	}

	// Rule 2: High power consumption (> 330W)
	if metric.PowerWatts > 330.0 {
		alerts = append(alerts, Alert{
			NodeID:         metric.NodeID,
			GPUIndex:       metric.GPUIndex,
			AlertType:      "high_power",
			Severity:       "warning",
			Message:        fmt.Sprintf("GPU power consumption is %.1fW", metric.PowerWatts),
			ThresholdValue: 330.0,
			ActualValue:    metric.PowerWatts,
		})
	}

	// Rule 3: High memory usage (> 95%)
	memoryPercent := (metric.MemoryUsedMB / metric.MemoryTotalMB) * 100.0
	if memoryPercent > 95.0 {
		alerts = append(alerts, Alert{
			NodeID:         metric.NodeID,
			GPUIndex:       metric.GPUIndex,
			AlertType:      "high_memory",
			Severity:       "warning",
			Message:        fmt.Sprintf("GPU memory usage is %.1f%%", memoryPercent),
			ThresholdValue: 95.0,
			ActualValue:    memoryPercent,
		})
	}

	return alerts
}

// StoreMetric saves metric to database
func (ae *AlertEngine) StoreMetric(metric GPUMetric) error {
	query := `
		INSERT INTO gpu_metrics (
			node_id, gpu_index, temperature_celsius, power_watts,
			memory_used_mb, memory_total_mb, utilization_percent,
			sm_clock_mhz, collected_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := ae.db.Exec(query,
		metric.NodeID,
		metric.GPUIndex,
		metric.TemperatureCelsius,
		metric.PowerWatts,
		metric.MemoryUsedMB,
		metric.MemoryTotalMB,
		metric.UtilizationPercent,
		metric.SMClockMHz,
		metric.CollectedAt,
	)

	return err
}

// CreateAlert saves alert to database
func (ae *AlertEngine) CreateAlert(alert Alert) error {
	query := `
		INSERT INTO alerts (
			node_id, gpu_index, alert_type, severity, message,
			threshold_value, actual_value, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, 'active')
		RETURNING id
	`

	var alertID int
	err := ae.db.QueryRow(query,
		alert.NodeID,
		alert.GPUIndex,
		alert.AlertType,
		alert.Severity,
		alert.Message,
		alert.ThresholdValue,
		alert.ActualValue,
	).Scan(&alertID)

	if err != nil {
		return err
	}

	log.Printf("Created alert ID=%d: [%s] %s on %s GPU %d",
		alertID, alert.Severity, alert.AlertType, alert.NodeID, alert.GPUIndex)

	// Take automated actions based on severity
	return ae.TakeAction(alertID, alert)
}

// TakeAction performs automated responses to alerts
func (ae *AlertEngine) TakeAction(alertID int, alert Alert) error {
	var actionType string
	var actionDetails map[string]interface{}

	switch alert.Severity {
	case "critical":
		// Critical: mark node as degraded, trigger workload migration
		actionType = "workload_migration"
		actionDetails = map[string]interface{}{
			"action":    "migrate_workloads",
			"from_node": alert.NodeID,
			"from_gpu":  alert.GPUIndex,
			"reason":    alert.Message,
		}

		// Update node status
		_, err := ae.db.Exec(
			"UPDATE gpu_nodes SET status = 'degraded' WHERE node_id = $1",
			alert.NodeID,
		)
		if err != nil {
			log.Printf("Failed to update node status: %v", err)
		}

		log.Printf("🚨 CRITICAL ACTION: Initiating workload migration from %s GPU %d",
			alert.NodeID, alert.GPUIndex)

	case "warning":
		actionType = "notification"
		actionDetails = map[string]interface{}{
			"action":  "send_notification",
			"channel": "slack",
			"message": alert.Message,
		}
		log.Printf("⚠️  WARNING: Sending notification for %s on %s GPU %d",
			alert.AlertType, alert.NodeID, alert.GPUIndex)
	}

	// Log action to database
	detailsJSON, _ := json.Marshal(actionDetails)
	_, err := ae.db.Exec(`
		INSERT INTO alert_actions (alert_id, action_type, action_status, action_details)
		VALUES ($1, $2, 'executed', $3)
	`, alertID, actionType, detailsJSON)

	return err
}

// Run starts consuming from Kafka
func (ae *AlertEngine) Run(ctx context.Context) error {
	log.Println("Alert Engine started, consuming from Kafka...")

	for {
		select {
		case <-ctx.Done():
			log.Println("Alert Engine shutting down")
			ae.kafkaReader.Close()
			ae.db.Close()
			return nil

		default:
			msg, err := ae.kafkaReader.FetchMessage(ctx)
			if err != nil {
				log.Printf("Error fetching message: %v", err)
				continue
			}

			var metric GPUMetric
			if err := json.Unmarshal(msg.Value, &metric); err != nil {
				log.Printf("Error unmarshaling metric: %v", err)
				ae.kafkaReader.CommitMessages(ctx, msg)
				continue
			}

			// Store metric
			if err := ae.StoreMetric(metric); err != nil {
				log.Printf("Error storing metric: %v", err)
			}

			// Evaluate alert rules
			alerts := ae.EvaluateRules(metric)
			for _, alert := range alerts {
				if err := ae.CreateAlert(alert); err != nil {
					log.Printf("Error creating alert: %v", err)
				}
			}

			// Commit message
			ae.kafkaReader.CommitMessages(ctx, msg)
		}
	}
}

func main() {
	dbConnStr := "host=localhost port=5432 user=telemetry password=telemetry123 dbname=gpu_telemetry sslmode=disable"
	kafkaBroker := "localhost:9093"

	engine, err := NewAlertEngine(dbConnStr, kafkaBroker)
	if err != nil {
		log.Fatalf("Failed to create alert engine: %v", err)
	}

	ctx := context.Background()
	if err := engine.Run(ctx); err != nil {
		log.Fatalf("Alert engine failed: %v", err)
	}
}
