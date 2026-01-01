// Package dashboard provides real-time dashboard components and handlers.
package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// MetricsCollector collects and broadcasts real-time metrics.
type MetricsCollector struct {
	db         interface{}
	clients    map[*Client]bool
	clientsMu  sync.RWMutex
	broadcast  chan MetricUpdate
	register   chan *Client
	unregister chan *Client
	metrics    *SystemMetrics
	metricsMu  sync.RWMutex
	collectors map[string]CollectorFunc
	updateRate time.Duration
}

// Client represents a WebSocket client.
type Client struct {
	ID        string
	conn      *websocket.Conn
	send      chan []byte
	collector *MetricsCollector
	filters   []string // Which metrics to receive
	role      string   // User role for access control
}

// SystemMetrics holds all system metrics.
type SystemMetrics struct {
	Timestamp     time.Time              `json:"timestamp"`
	System        SystemStats            `json:"system"`
	Tickets       TicketMetrics          `json:"tickets"`
	Users         UserMetrics            `json:"users"`
	Performance   PerformanceMetrics     `json:"performance"`
	Queues        []QueueMetrics         `json:"queues"`
	SLA           SLAMetrics             `json:"sla"`
	Trends        TrendMetrics           `json:"trends"`
	Alerts        []Alert                `json:"alerts"`
	CustomMetrics map[string]interface{} `json:"custom"`
}

// SystemStats represents system-level statistics.
type SystemStats struct {
	Uptime         time.Duration `json:"uptime"`
	DatabaseStatus string        `json:"database_status"`
	CacheHitRatio  float64       `json:"cache_hit_ratio"`
	ActiveSessions int           `json:"active_sessions"`
	MemoryUsage    float64       `json:"memory_usage_percent"`
	CPUUsage       float64       `json:"cpu_usage_percent"`
	DiskUsage      float64       `json:"disk_usage_percent"`
	ErrorRate      float64       `json:"error_rate"`
	RequestsPerSec float64       `json:"requests_per_sec"`
}

// TicketMetrics represents ticket-related metrics.
type TicketMetrics struct {
	TotalOpen         int     `json:"total_open"`
	TotalClosed       int     `json:"total_closed"`
	TotalPending      int     `json:"total_pending"`
	CreatedToday      int     `json:"created_today"`
	ClosedToday       int     `json:"closed_today"`
	AvgResponseTime   float64 `json:"avg_response_time_hours"`
	AvgResolutionTime float64 `json:"avg_resolution_time_hours"`
	EscalatedCount    int     `json:"escalated_count"`
	OverdueCount      int     `json:"overdue_count"`
	UnassignedCount   int     `json:"unassigned_count"`
}

// UserMetrics represents user-related metrics.
type UserMetrics struct {
	TotalUsers       int     `json:"total_users"`
	ActiveUsers      int     `json:"active_users"`
	OnlineNow        int     `json:"online_now"`
	NewToday         int     `json:"new_today"`
	AgentsAvailable  int     `json:"agents_available"`
	AgentsBusy       int     `json:"agents_busy"`
	AgentsOffline    int     `json:"agents_offline"`
	AvgSessionLength float64 `json:"avg_session_length_minutes"`
}

// QueueMetrics represents metrics for a specific queue.
type QueueMetrics struct {
	QueueID         int     `json:"queue_id"`
	QueueName       string  `json:"queue_name"`
	TicketsWaiting  int     `json:"tickets_waiting"`
	AvgWaitTime     float64 `json:"avg_wait_time_minutes"`
	LongestWaitTime float64 `json:"longest_wait_time_minutes"`
	AgentsAssigned  int     `json:"agents_assigned"`
	ProcessingRate  float64 `json:"processing_rate_per_hour"`
}

// PerformanceMetrics represents system performance metrics.
type PerformanceMetrics struct {
	AvgResponseTime float64            `json:"avg_response_time_ms"`
	P95ResponseTime float64            `json:"p95_response_time_ms"`
	P99ResponseTime float64            `json:"p99_response_time_ms"`
	ErrorRate       float64            `json:"error_rate_percent"`
	Throughput      float64            `json:"throughput_requests_per_sec"`
	DatabaseQueries int                `json:"database_queries_per_sec"`
	SlowQueries     int                `json:"slow_queries_count"`
	EndpointMetrics map[string]float64 `json:"endpoint_metrics"`
}

// SLAMetrics represents SLA compliance metrics.
type SLAMetrics struct {
	ComplianceRate  float64              `json:"compliance_rate_percent"`
	BreachedTickets int                  `json:"breached_tickets"`
	AtRiskTickets   int                  `json:"at_risk_tickets"`
	MetTargets      int                  `json:"met_targets"`
	TotalTargets    int                  `json:"total_targets"`
	ByPriority      map[string]SLAStatus `json:"by_priority"`
}

// SLAStatus represents SLA status for a priority level.
type SLAStatus struct {
	Priority       string  `json:"priority"`
	ComplianceRate float64 `json:"compliance_rate"`
	AvgTime        float64 `json:"avg_time_hours"`
	TargetTime     float64 `json:"target_time_hours"`
}

// TrendMetrics represents trend analysis.
type TrendMetrics struct {
	TicketTrend       Trend `json:"ticket_trend"` // up, down, stable
	ResponseTrend     Trend `json:"response_trend"`
	ResolutionTrend   Trend `json:"resolution_trend"`
	UserGrowthTrend   Trend `json:"user_growth_trend"`
	SatisfactionTrend Trend `json:"satisfaction_trend"`
}

// Trend represents a metric trend.
type Trend struct {
	Direction    string  `json:"direction"` // up, down, stable
	Change       float64 `json:"change_percent"`
	LastValue    float64 `json:"last_value"`
	CurrentValue float64 `json:"current_value"`
	Period       string  `json:"period"` // hour, day, week, month
}

// Alert represents a system alert.
type Alert struct {
	ID           string    `json:"id"`
	Level        string    `json:"level"` // info, warning, error, critical
	Type         string    `json:"type"`
	Message      string    `json:"message"`
	Timestamp    time.Time `json:"timestamp"`
	AffectedItem string    `json:"affected_item"`
	Action       string    `json:"action_required"`
}

// MetricUpdate represents a real-time metric update.
type MetricUpdate struct {
	Type      string      `json:"type"`
	Metric    string      `json:"metric"`
	Value     interface{} `json:"value"`
	Timestamp time.Time   `json:"timestamp"`
	Delta     interface{} `json:"delta,omitempty"`
}

// CollectorFunc is a function that collects specific metrics.
type CollectorFunc func(context.Context) (interface{}, error)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Configure appropriately for production
	},
}

// NewMetricsCollector creates a new metrics collector.
func NewMetricsCollector(db interface{}) *MetricsCollector {
	mc := &MetricsCollector{
		db:         db,
		clients:    make(map[*Client]bool),
		broadcast:  make(chan MetricUpdate, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		metrics:    &SystemMetrics{},
		collectors: make(map[string]CollectorFunc),
		updateRate: 5 * time.Second,
	}

	// Register default collectors
	mc.registerDefaultCollectors()

	return mc
}

// Start starts the metrics collector.
func (mc *MetricsCollector) Start(ctx context.Context) {
	go mc.run(ctx)
	go mc.collectMetrics(ctx)
}

// run handles client connections and broadcasts.
func (mc *MetricsCollector) run(ctx context.Context) {
	ticker := time.NewTicker(mc.updateRate)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case client := <-mc.register:
			mc.clientsMu.Lock()
			mc.clients[client] = true
			mc.clientsMu.Unlock()
			log.Printf("Client %s connected. Total clients: %d", client.ID, len(mc.clients))

			// Send initial metrics
			mc.sendInitialMetrics(client)

		case client := <-mc.unregister:
			mc.clientsMu.Lock()
			if _, ok := mc.clients[client]; ok {
				delete(mc.clients, client)
				close(client.send)
				mc.clientsMu.Unlock()
				log.Printf("Client %s disconnected. Total clients: %d", client.ID, len(mc.clients))
			} else {
				mc.clientsMu.Unlock()
			}

		case update := <-mc.broadcast:
			mc.broadcastUpdate(update)

		case <-ticker.C:
			mc.broadcastFullMetrics()
		}
	}
}

// collectMetrics continuously collects metrics.
func (mc *MetricsCollector) collectMetrics(ctx context.Context) {
	ticker := time.NewTicker(mc.updateRate)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mc.updateAllMetrics(ctx)
		}
	}
}

// updateAllMetrics updates all metrics.
func (mc *MetricsCollector) updateAllMetrics(ctx context.Context) {
	mc.metricsMu.Lock()
	defer mc.metricsMu.Unlock()

	mc.metrics.Timestamp = time.Now()

	// Update each metric category
	for name, collector := range mc.collectors {
		if data, err := collector(ctx); err == nil {
			switch name {
			case "system":
				if stats, ok := data.(SystemStats); ok {
					mc.metrics.System = stats
				}
			case "tickets":
				if metrics, ok := data.(TicketMetrics); ok {
					mc.metrics.Tickets = metrics
				}
			case "users":
				if metrics, ok := data.(UserMetrics); ok {
					mc.metrics.Users = metrics
				}
			case "performance":
				if metrics, ok := data.(PerformanceMetrics); ok {
					mc.metrics.Performance = metrics
				}
			case "queues":
				if metrics, ok := data.([]QueueMetrics); ok {
					mc.metrics.Queues = metrics
				}
			case "sla":
				if metrics, ok := data.(SLAMetrics); ok {
					mc.metrics.SLA = metrics
				}
			case "trends":
				if metrics, ok := data.(TrendMetrics); ok {
					mc.metrics.Trends = metrics
				}
			case "alerts":
				if alerts, ok := data.([]Alert); ok {
					mc.metrics.Alerts = alerts
				}
			default:
				if mc.metrics.CustomMetrics == nil {
					mc.metrics.CustomMetrics = make(map[string]interface{})
				}
				mc.metrics.CustomMetrics[name] = data
			}
		} else {
			log.Printf("Error collecting %s metrics: %v", name, err)
		}
	}
}

// broadcastUpdate sends a metric update to all clients.
func (mc *MetricsCollector) broadcastUpdate(update MetricUpdate) {
	message, err := json.Marshal(update)
	if err != nil {
		log.Printf("Error marshaling update: %v", err)
		return
	}

	mc.clientsMu.RLock()
	defer mc.clientsMu.RUnlock()

	for client := range mc.clients {
		if mc.shouldSendToClient(client, update.Type) {
			select {
			case client.send <- message:
			default:
				// Client's send channel is full, close it
				close(client.send)
				delete(mc.clients, client)
			}
		}
	}
}

// broadcastFullMetrics sends full metrics to all clients.
func (mc *MetricsCollector) broadcastFullMetrics() {
	mc.metricsMu.RLock()
	message, err := json.Marshal(mc.metrics)
	mc.metricsMu.RUnlock()

	if err != nil {
		log.Printf("Error marshaling metrics: %v", err)
		return
	}

	mc.clientsMu.RLock()
	defer mc.clientsMu.RUnlock()

	for client := range mc.clients {
		select {
		case client.send <- message:
		default:
			// Client's send channel is full, close it
			close(client.send)
			delete(mc.clients, client)
		}
	}
}

// sendInitialMetrics sends initial metrics to a new client.
func (mc *MetricsCollector) sendInitialMetrics(client *Client) {
	mc.metricsMu.RLock()
	message, err := json.Marshal(mc.metrics)
	mc.metricsMu.RUnlock()

	if err != nil {
		log.Printf("Error marshaling initial metrics: %v", err)
		return
	}

	select {
	case client.send <- message:
	default:
		log.Printf("Failed to send initial metrics to client %s", client.ID)
	}
}

// shouldSendToClient checks if a metric should be sent to a client.
func (mc *MetricsCollector) shouldSendToClient(client *Client, metricType string) bool {
	// Check if client has filters
	if len(client.filters) == 0 {
		return true // No filters, send everything
	}

	// Check if metric type is in client's filters
	for _, filter := range client.filters {
		if filter == metricType || filter == "*" {
			return true
		}
	}

	return false
}

// RegisterCollector registers a custom metric collector.
func (mc *MetricsCollector) RegisterCollector(name string, collector CollectorFunc) {
	mc.collectors[name] = collector
}

// registerDefaultCollectors registers the default metric collectors.
func (mc *MetricsCollector) registerDefaultCollectors() {
	// System metrics collector
	mc.collectors["system"] = func(ctx context.Context) (interface{}, error) {
		// TODO: Implement actual system metrics collection
		return SystemStats{
			Uptime:         time.Since(time.Now().Add(-24 * time.Hour)),
			DatabaseStatus: "healthy",
			CacheHitRatio:  0.85,
			ActiveSessions: 42,
			MemoryUsage:    45.2,
			CPUUsage:       23.5,
			DiskUsage:      67.8,
			ErrorRate:      0.02,
			RequestsPerSec: 125.4,
		}, nil
	}

	// Ticket metrics collector
	mc.collectors["tickets"] = func(ctx context.Context) (interface{}, error) {
		// TODO: Implement actual database queries
		return TicketMetrics{
			TotalOpen:         234,
			TotalClosed:       1567,
			TotalPending:      45,
			CreatedToday:      23,
			ClosedToday:       19,
			AvgResponseTime:   2.5,
			AvgResolutionTime: 24.3,
			EscalatedCount:    8,
			OverdueCount:      12,
			UnassignedCount:   7,
		}, nil
	}

	// User metrics collector
	mc.collectors["users"] = func(ctx context.Context) (interface{}, error) {
		// TODO: Implement actual database queries
		return UserMetrics{
			TotalUsers:       1250,
			ActiveUsers:      890,
			OnlineNow:        42,
			NewToday:         5,
			AgentsAvailable:  8,
			AgentsBusy:       12,
			AgentsOffline:    5,
			AvgSessionLength: 35.7,
		}, nil
	}

	// Add more default collectors...
}

// HandleWebSocket handles WebSocket connections.
func (mc *MetricsCollector) HandleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := &Client{
		ID:        fmt.Sprintf("client_%d", time.Now().UnixNano()),
		conn:      conn,
		send:      make(chan []byte, 256),
		collector: mc,
		filters:   []string{}, // Can be configured based on request params
		role:      c.GetString("user_role"),
	}

	mc.register <- client

	// Start goroutines for reading and writing
	go client.readPump()
	go client.writePump()
}

// readPump pumps messages from the WebSocket connection.
func (c *Client) readPump() {
	defer func() {
		c.collector.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Handle client messages (e.g., filter updates)
		c.handleMessage(message)
	}
}

// writePump pumps messages to the WebSocket connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the current WebSocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte("\n"))
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage handles messages from the client.
func (c *Client) handleMessage(message []byte) {
	var msg map[string]interface{}
	if err := json.Unmarshal(message, &msg); err != nil {
		log.Printf("Error parsing client message: %v", err)
		return
	}

	// Handle different message types
	if msgType, ok := msg["type"].(string); ok {
		switch msgType {
		case "subscribe":
			if filters, ok := msg["filters"].([]interface{}); ok {
				c.filters = make([]string, len(filters))
				for i, f := range filters {
					if str, ok := f.(string); ok {
						c.filters[i] = str
					}
				}
			}
		case "unsubscribe":
			c.filters = []string{}
		case "ping":
			// Send pong response
			pong := map[string]string{"type": "pong"}
			if data, err := json.Marshal(pong); err == nil {
				c.send <- data
			}
		}
	}
}
