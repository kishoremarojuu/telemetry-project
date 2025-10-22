package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

type APIServer struct {
	db     *sql.DB
	router *mux.Router
}

type NodeHealth struct {
	NodeID       string    `json:"node_id"`
	Hostname     string    `json:"hostname"`
	Status       string    `json:"status"`
	Datacenter   string    `json:"datacenter"`
	LastSeen     time.Time `json:"last_seen"`
	ActiveAlerts int       `json:"active_alerts"`
}

type MetricResponse struct {
	NodeID             string    `json:"node_id"`
	GPUIndex           int       `json:"gpu_index"`
	TemperatureCelsius float64   `json:"temperature_celsius"`
	PowerWatts         float64   `json:"power_watts"`
	MemoryUsedMB       float64   `json:"memory_used_mb"`
	MemoryTotalMB      float64   `json:"memory_total_mb"`
	UtilizationPercent float64   `json:"utilization_percent"`
	CollectedAt        time.Time `json:"collected_at"`
}

type AlertResponse struct {
	ID             int       `json:"id"`
	NodeID         string    `json:"node_id"`
	GPUIndex       int       `json:"gpu_index"`
	AlertType      string    `json:"alert_type"`
	Severity       string    `json:"severity"`
	Message        string    `json:"message"`
	ThresholdValue float64   `json:"threshold_value"`
	ActualValue    float64   `json:"actual_value"`
	Status         string    `json:"status"`
	TriggeredAt    time.Time `json:"triggered_at"`
}

func NewAPIServer(dbConnStr string) (*APIServer, error) {
	db, err := sql.Open("postgres", dbConnStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	server := &APIServer{
		db:     db,
		router: mux.NewRouter(),
	}

	server.setupRoutes()
	return server, nil
}

func (s *APIServer) setupRoutes() {
	// Health check
	s.router.HandleFunc("/health", s.healthCheck).Methods("GET")

	// Node endpoints
	s.router.HandleFunc("/api/v1/nodes", s.getAllNodes).Methods("GET")
	s.router.HandleFunc("/api/v1/nodes/{node_id}", s.getNodeHealth).Methods("GET")
	s.router.HandleFunc("/api/v1/nodes/{node_id}/metrics", s.getNodeMetrics).Methods("GET")

	// Alert endpoints
	s.router.HandleFunc("/api/v1/alerts", s.getAlerts).Methods("GET")
	s.router.HandleFunc("/api/v1/alerts/active", s.getActiveAlerts).Methods("GET")
	s.router.HandleFunc("/api/v1/alerts/{alert_id}/resolve", s.resolveAlert).Methods("POST")

	// Metrics endpoints
	s.router.HandleFunc("/api/v1/metrics/latest", s.getLatestMetrics).Methods("GET")
}

func (s *APIServer) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

func (s *APIServer) getAllNodes(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT n.node_id, n.hostname, n.status, n.datacenter, n.last_seen,
		       COALESCE(COUNT(a.id), 0) as active_alerts
		FROM gpu_nodes n
		LEFT JOIN alerts a ON n.node_id = a.node_id AND a.status = 'active'
		GROUP BY n.node_id, n.hostname, n.status, n.datacenter, n.last_seen
		ORDER BY n.node_id
	`

	rows, err := s.db.Query(query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var nodes []NodeHealth
	for rows.Next() {
		var node NodeHealth
		if err := rows.Scan(&node.NodeID, &node.Hostname, &node.Status,
			&node.Datacenter, &node.LastSeen, &node.ActiveAlerts); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		nodes = append(nodes, node)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(nodes)
}

func (s *APIServer) getNodeHealth(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	nodeID := vars["node_id"]

	query := `
		SELECT n.node_id, n.hostname, n.status, n.datacenter, n.last_seen,
		       COALESCE(COUNT(a.id), 0) as active_alerts
		FROM gpu_nodes n
		LEFT JOIN alerts a ON n.node_id = a.node_id AND a.status = 'active'
		WHERE n.node_id = $1
		GROUP BY n.node_id, n.hostname, n.status, n.datacenter, n.last_seen
	`

	var node NodeHealth
	err := s.db.QueryRow(query, nodeID).Scan(
		&node.NodeID, &node.Hostname, &node.Status,
		&node.Datacenter, &node.LastSeen, &node.ActiveAlerts,
	)

	if err == sql.ErrNoRows {
		http.Error(w, "Node not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(node)
}

func (s *APIServer) getNodeMetrics(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	nodeID := vars["node_id"]

	// Get query parameters for time range
	limit := r.URL.Query().Get("limit")
	if limit == "" {
		limit = "100"
	}

	query := `
		SELECT node_id, gpu_index, temperature_celsius, power_watts,
		       memory_used_mb, memory_total_mb, utilization_percent, collected_at
		FROM gpu_metrics
		WHERE node_id = $1
		ORDER BY collected_at DESC
		LIMIT ` + limit

	rows, err := s.db.Query(query, nodeID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var metrics []MetricResponse
	for rows.Next() {
		var m MetricResponse
		if err := rows.Scan(&m.NodeID, &m.GPUIndex, &m.TemperatureCelsius,
			&m.PowerWatts, &m.MemoryUsedMB, &m.MemoryTotalMB,
			&m.UtilizationPercent, &m.CollectedAt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		metrics = append(metrics, m)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

func (s *APIServer) getAlerts(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT id, node_id, gpu_index, alert_type, severity, message,
		       threshold_value, actual_value, status, triggered_at
		FROM alerts
		ORDER BY triggered_at DESC
		LIMIT 100
	`

	rows, err := s.db.Query(query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var alerts []AlertResponse
	for rows.Next() {
		var a AlertResponse
		if err := rows.Scan(&a.ID, &a.NodeID, &a.GPUIndex, &a.AlertType,
			&a.Severity, &a.Message, &a.ThresholdValue, &a.ActualValue,
			&a.Status, &a.TriggeredAt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		alerts = append(alerts, a)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(alerts)
}

func (s *APIServer) getActiveAlerts(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT id, node_id, gpu_index, alert_type, severity, message,
		       threshold_value, actual_value, status, triggered_at
		FROM alerts
		WHERE status = 'active'
		ORDER BY severity DESC, triggered_at DESC
	`

	rows, err := s.db.Query(query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var alerts []AlertResponse
	for rows.Next() {
		var a AlertResponse
		if err := rows.Scan(&a.ID, &a.NodeID, &a.GPUIndex, &a.AlertType,
			&a.Severity, &a.Message, &a.ThresholdValue, &a.ActualValue,
			&a.Status, &a.TriggeredAt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		alerts = append(alerts, a)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(alerts)
}

func (s *APIServer) resolveAlert(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	alertID := vars["alert_id"]

	query := `
		UPDATE alerts
		SET status = 'resolved', resolved_at = NOW()
		WHERE id = $1
		RETURNING id
	`

	var id int
	err := s.db.QueryRow(query, alertID).Scan(&id)
	if err == sql.ErrNoRows {
		http.Error(w, "Alert not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":  "Alert resolved",
		"alert_id": id,
	})
}

func (s *APIServer) getLatestMetrics(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT node_id, gpu_index, temperature_celsius, power_watts,
		       memory_used_mb, memory_total_mb, utilization_percent, collected_at
		FROM latest_gpu_metrics
		ORDER BY node_id, gpu_index
	`

	rows, err := s.db.Query(query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var metrics []MetricResponse
	for rows.Next() {
		var m MetricResponse
		if err := rows.Scan(&m.NodeID, &m.GPUIndex, &m.TemperatureCelsius,
			&m.PowerWatts, &m.MemoryUsedMB, &m.MemoryTotalMB,
			&m.UtilizationPercent, &m.CollectedAt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		metrics = append(metrics, m)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

func (s *APIServer) Start(port string) error {
	log.Printf("Starting API server on port %s", port)
	return http.ListenAndServe(":"+port, s.router)
}

func main() {
	dbConnStr := "host=localhost port=5432 user=telemetry password=telemetry123 dbname=gpu_telemetry sslmode=disable"

	server, err := NewAPIServer(dbConnStr)
	if err != nil {
		log.Fatalf("Failed to create API server: %v", err)
	}

	log.Println("API Server started successfully")
	log.Println("Available endpoints:")
	log.Println("  GET  /health")
	log.Println("  GET  /api/v1/nodes")
	log.Println("  GET  /api/v1/nodes/{node_id}")
	log.Println("  GET  /api/v1/nodes/{node_id}/metrics")
	log.Println("  GET  /api/v1/alerts")
	log.Println("  GET  /api/v1/alerts/active")
	log.Println("  POST /api/v1/alerts/{alert_id}/resolve")
	log.Println("  GET  /api/v1/metrics/latest")

	if err := server.Start("8080"); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
