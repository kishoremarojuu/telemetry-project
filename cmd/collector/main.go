package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/segmentio/kafka-go"
	"log"
	"math/rand"
	"time"
)

// GPUMetric represents telemetry data from a GPU
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

// CollectorService handles polling and publishing metrics
type CollectorService struct {
	nodes        []string
	kafkaWriter  *kafka.Writer
	pollInterval time.Duration
}

func NewCollectorService(nodes []string, kafkaBroker string) *CollectorService {
	writer := &kafka.Writer{
		Addr:         kafka.TCP(kafkaBroker),
		Topic:        "gpu-telemetry",
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond,
		RequiredAcks: kafka.RequireOne,
	}

	return &CollectorService{
		nodes:        nodes,
		kafkaWriter:  writer,
		pollInterval: 30 * time.Second,
	}
}

// CollectMetrics simulates collecting metrics from DCGM exporters
func (c *CollectorService) CollectMetrics(nodeID string) ([]GPUMetric, error) {
	// In production, this would HTTP GET from DCGM exporter endpoint
	// For now, we'll simulate realistic GPU metrics

	numGPUs := 8 // DGX typically has 8 GPUs
	metrics := make([]GPUMetric, numGPUs)

	for i := 0; i < numGPUs; i++ {
		// Simulate realistic GPU metrics with some variation
		baseTemp := 65.0 + rand.Float64()*30.0           // 65-95°C
		basePower := 250.0 + rand.Float64()*100.0        // 250-350W
		memTotal := 80000.0                              // 80GB for A100
		memUsed := memTotal * (0.3 + rand.Float64()*0.6) // 30-90% usage

		metrics[i] = GPUMetric{
			NodeID:             nodeID,
			GPUIndex:           i,
			TemperatureCelsius: baseTemp,
			PowerWatts:         basePower,
			MemoryUsedMB:       memUsed,
			MemoryTotalMB:      memTotal,
			UtilizationPercent: rand.Float64() * 100.0,
			SMClockMHz:         1410 + rand.Intn(200), // 1410-1610 MHz
			CollectedAt:        time.Now(),
		}
	}

	return metrics, nil
}

// PublishToKafka sends metrics to Kafka
func (c *CollectorService) PublishToKafka(ctx context.Context, metrics []GPUMetric) error {
	messages := make([]kafka.Message, len(metrics))

	for i, metric := range metrics {
		data, err := json.Marshal(metric)
		if err != nil {
			return fmt.Errorf("failed to marshal metric: %w", err)
		}

		messages[i] = kafka.Message{
			Key:   []byte(fmt.Sprintf("%s-gpu-%d", metric.NodeID, metric.GPUIndex)),
			Value: data,
			Time:  metric.CollectedAt,
		}
	}

	err := c.kafkaWriter.WriteMessages(ctx, messages...)
	if err != nil {
		return fmt.Errorf("failed to write to kafka: %w", err)
	}

	log.Printf("Published %d metrics to Kafka", len(metrics))
	return nil
}

// Run starts the collection loop
func (c *CollectorService) Run(ctx context.Context) error {
	ticker := time.NewTicker(c.pollInterval)
	defer ticker.Stop()

	log.Printf("Starting collector service, polling %d nodes every %s",
		len(c.nodes), c.pollInterval)

	// Collect immediately on startup
	c.collectFromAllNodes(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Println("Collector service shutting down")
			return c.kafkaWriter.Close()
		case <-ticker.C:
			c.collectFromAllNodes(ctx)
		}
	}
}

func (c *CollectorService) collectFromAllNodes(ctx context.Context) {
	for _, nodeID := range c.nodes {
		metrics, err := c.CollectMetrics(nodeID)
		if err != nil {
			log.Printf("Error collecting from %s: %v", nodeID, err)
			continue
		}

		if err := c.PublishToKafka(ctx, metrics); err != nil {
			log.Printf("Error publishing metrics from %s: %v", nodeID, err)
		} else {
			log.Printf("Successfully collected and published metrics from %s", nodeID)
		}
	}
}

func main() {
	nodes := []string{"node-1", "node-2"}
	kafkaBroker := "localhost:9093"

	collector := NewCollectorService(nodes, kafkaBroker)

	ctx := context.Background()
	if err := collector.Run(ctx); err != nil {
		log.Fatalf("Collector service failed: %v", err)
	}
}
