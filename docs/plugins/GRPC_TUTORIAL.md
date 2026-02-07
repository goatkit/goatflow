# gRPC Plugin Tutorial

Build a full-featured plugin using gRPC for maximum flexibility.

## Why gRPC?

gRPC plugins run as separate processes, giving you:

- **Full Go stdlib** - All packages, goroutines, channels
- **Any language** - Go, Python, Node.js, Rust, etc.
- **Direct network** - No Host API restrictions
- **Unlimited memory** - Not sandboxed
- **Easy debugging** - Standard debuggers work

Trade-off: Slightly higher latency than WASM (~1ms per call).

## Prerequisites

- Go 1.21+
- Protocol Buffers compiler (`protoc`)
- GoatFlow running locally

## Step 1: Scaffold the Plugin

```bash
cd /path/to/goatflow/plugins
gk plugin init analytics --type grpc
cd analytics
```

This creates:
```
analytics/
├── manifest.json
├── main.go
├── plugin.go
├── build.sh
├── proto/
│   └── plugin.proto
└── README.md
```

## Step 2: Define the Manifest

Edit `manifest.json`:

```json
{
  "name": "analytics",
  "version": "1.0.0",
  "description": "Advanced ticket analytics and reporting",
  "author": "Your Name",
  "license": "Apache-2.0",
  
  "type": "grpc",
  "grpc": {
    "binary": "analytics",
    "health_check": true
  },
  
  "routes": [
    {
      "method": "GET",
      "path": "/api/plugins/analytics/reports",
      "handler": "list_reports",
      "middleware": ["auth", "admin"]
    },
    {
      "method": "GET",
      "path": "/api/plugins/analytics/reports/:id",
      "handler": "get_report",
      "middleware": ["auth", "admin"]
    },
    {
      "method": "POST",
      "path": "/api/plugins/analytics/reports/:id/run",
      "handler": "run_report",
      "middleware": ["auth", "admin"]
    }
  ],
  
  "widgets": [
    {
      "id": "analytics-summary",
      "title": "Analytics Summary",
      "handler": "render_summary",
      "location": "admin_home",
      "size": "large"
    }
  ],
  
  "jobs": [
    {
      "id": "daily-report",
      "schedule": "0 6 * * *",
      "handler": "generate_daily_report",
      "description": "Generates daily analytics report"
    }
  ],
  
  "permissions": ["db:read", "http:external", "cache:read", "cache:write"]
}
```

## Step 3: Implement the Plugin

### main.go - Entry Point

```go
package main

import (
	"log"
	"net"
	"os"
	
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	
	pb "github.com/goatkit/goatflow/pkg/plugin/proto"
)

func main() {
	// Get socket path from environment
	socketPath := os.Getenv("GOATFLOW_PLUGIN_SOCKET")
	if socketPath == "" {
		socketPath = "/tmp/goatflow-plugin-analytics.sock"
	}
	
	// Clean up old socket
	os.Remove(socketPath)
	
	// Listen on Unix socket
	lis, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	
	// Create gRPC server
	server := grpc.NewServer()
	
	// Register plugin service
	plugin := NewAnalyticsPlugin()
	pb.RegisterPluginServiceServer(server, plugin)
	
	// Register health check
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(server, healthServer)
	healthServer.SetServingStatus("plugin", grpc_health_v1.HealthCheckResponse_SERVING)
	
	log.Printf("Analytics plugin listening on %s", socketPath)
	
	if err := server.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
```

### plugin.go - Business Logic

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	
	pb "github.com/goatkit/goatflow/pkg/plugin/proto"
)

type AnalyticsPlugin struct {
	pb.UnimplementedPluginServiceServer
	host pb.HostServiceClient
}

func NewAnalyticsPlugin() *AnalyticsPlugin {
	return &AnalyticsPlugin{}
}

// SetHost is called by GoatFlow to provide Host API access
func (p *AnalyticsPlugin) SetHost(client pb.HostServiceClient) {
	p.host = client
}

// Register returns plugin manifest
func (p *AnalyticsPlugin) Register(ctx context.Context, req *pb.Empty) (*pb.RegisterResponse, error) {
	return &pb.RegisterResponse{
		Name:        "analytics",
		Version:     "1.0.0",
		Description: "Advanced ticket analytics",
	}, nil
}

// Call handles function calls from GoatFlow
func (p *AnalyticsPlugin) Call(ctx context.Context, req *pb.CallRequest) (*pb.CallResponse, error) {
	switch req.Function {
	case "list_reports":
		return p.listReports(ctx, req)
	case "get_report":
		return p.getReport(ctx, req)
	case "run_report":
		return p.runReport(ctx, req)
	case "render_summary":
		return p.renderSummary(ctx, req)
	case "generate_daily_report":
		return p.generateDailyReport(ctx, req)
	default:
		return nil, fmt.Errorf("unknown function: %s", req.Function)
	}
}

// Report represents an analytics report
type Report struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	LastRun     time.Time `json:"last_run,omitempty"`
}

func (p *AnalyticsPlugin) listReports(ctx context.Context, req *pb.CallRequest) (*pb.CallResponse, error) {
	reports := []Report{
		{ID: "ticket-volume", Name: "Ticket Volume", Description: "Daily ticket creation trends"},
		{ID: "resolution-time", Name: "Resolution Time", Description: "Average time to resolve tickets"},
		{ID: "agent-performance", Name: "Agent Performance", Description: "Tickets handled per agent"},
	}
	
	result, _ := json.Marshal(reports)
	return &pb.CallResponse{Result: result}, nil
}

func (p *AnalyticsPlugin) getReport(ctx context.Context, req *pb.CallRequest) (*pb.CallResponse, error) {
	var params struct {
		ID string `json:"id"`
	}
	json.Unmarshal(req.Args, &params)
	
	// Query database for report data
	query := `
		SELECT DATE(create_time) as date, COUNT(*) as count 
		FROM tickets 
		WHERE create_time > DATE_SUB(NOW(), INTERVAL 30 DAY)
		GROUP BY DATE(create_time)
		ORDER BY date
	`
	
	resp, err := p.host.DBQuery(ctx, &pb.DBQueryRequest{
		Query: query,
	})
	if err != nil {
		return nil, err
	}
	
	return &pb.CallResponse{Result: resp.Rows}, nil
}

func (p *AnalyticsPlugin) runReport(ctx context.Context, req *pb.CallRequest) (*pb.CallResponse, error) {
	var params struct {
		ID        string            `json:"id"`
		DateRange map[string]string `json:"date_range"`
	}
	json.Unmarshal(req.Args, &params)
	
	// Log the report run
	p.host.Log(ctx, &pb.LogRequest{
		Level:   "info",
		Message: "Running report",
		Fields:  map[string]string{"report_id": params.ID},
	})
	
	// Execute report logic based on ID
	var data interface{}
	switch params.ID {
	case "ticket-volume":
		data = p.calculateTicketVolume(ctx, params.DateRange)
	case "resolution-time":
		data = p.calculateResolutionTime(ctx, params.DateRange)
	default:
		return nil, fmt.Errorf("unknown report: %s", params.ID)
	}
	
	// Cache results
	cacheKey := fmt.Sprintf("report_%s_%v", params.ID, time.Now().Format("2006-01-02"))
	resultJSON, _ := json.Marshal(data)
	p.host.CacheSet(ctx, &pb.CacheSetRequest{
		Key:   cacheKey,
		Value: resultJSON,
		Ttl:   3600, // 1 hour
	})
	
	return &pb.CallResponse{Result: resultJSON}, nil
}

func (p *AnalyticsPlugin) renderSummary(ctx context.Context, req *pb.CallRequest) (*pb.CallResponse, error) {
	// Get summary stats
	stats := p.getSummaryStats(ctx)
	
	html := fmt.Sprintf(`
		<div class="grid grid-cols-4 gap-4">
			<div class="stat bg-base-200 rounded-lg p-4">
				<div class="stat-title">Today's Tickets</div>
				<div class="stat-value">%d</div>
				<div class="stat-desc">%+d from yesterday</div>
			</div>
			<div class="stat bg-base-200 rounded-lg p-4">
				<div class="stat-title">Avg Resolution</div>
				<div class="stat-value">%.1fh</div>
				<div class="stat-desc">Target: 24h</div>
			</div>
			<div class="stat bg-base-200 rounded-lg p-4">
				<div class="stat-title">Open Tickets</div>
				<div class="stat-value">%d</div>
				<div class="stat-desc">Across all queues</div>
			</div>
			<div class="stat bg-base-200 rounded-lg p-4">
				<div class="stat-title">SLA Compliance</div>
				<div class="stat-value">%.0f%%</div>
				<div class="stat-desc">Last 7 days</div>
			</div>
		</div>
	`, stats.TodayTickets, stats.TicketDelta, stats.AvgResolution, 
	   stats.OpenTickets, stats.SLACompliance)
	
	return &pb.CallResponse{Result: []byte(html)}, nil
}

func (p *AnalyticsPlugin) generateDailyReport(ctx context.Context, req *pb.CallRequest) (*pb.CallResponse, error) {
	p.host.Log(ctx, &pb.LogRequest{
		Level:   "info",
		Message: "Generating daily analytics report",
	})
	
	// Generate comprehensive daily report
	report := p.buildDailyReport(ctx)
	
	// Could send via email, store in DB, etc.
	reportJSON, _ := json.Marshal(report)
	
	return &pb.CallResponse{Result: reportJSON}, nil
}

// Helper methods

type SummaryStats struct {
	TodayTickets  int     `json:"today_tickets"`
	TicketDelta   int     `json:"ticket_delta"`
	AvgResolution float64 `json:"avg_resolution_hours"`
	OpenTickets   int     `json:"open_tickets"`
	SLACompliance float64 `json:"sla_compliance_percent"`
}

func (p *AnalyticsPlugin) getSummaryStats(ctx context.Context) SummaryStats {
	// Query for today's tickets
	todayResp, _ := p.host.DBQuery(ctx, &pb.DBQueryRequest{
		Query: "SELECT COUNT(*) as count FROM tickets WHERE DATE(create_time) = CURDATE()",
	})
	
	// Query for yesterday's tickets
	yesterdayResp, _ := p.host.DBQuery(ctx, &pb.DBQueryRequest{
		Query: "SELECT COUNT(*) as count FROM tickets WHERE DATE(create_time) = DATE_SUB(CURDATE(), INTERVAL 1 DAY)",
	})
	
	// Parse results and calculate stats
	// ... implementation details
	
	return SummaryStats{
		TodayTickets:  42,
		TicketDelta:   5,
		AvgResolution: 18.5,
		OpenTickets:   127,
		SLACompliance: 94.2,
	}
}

func (p *AnalyticsPlugin) calculateTicketVolume(ctx context.Context, dateRange map[string]string) interface{} {
	// Implementation
	return nil
}

func (p *AnalyticsPlugin) calculateResolutionTime(ctx context.Context, dateRange map[string]string) interface{} {
	// Implementation
	return nil
}

func (p *AnalyticsPlugin) buildDailyReport(ctx context.Context) interface{} {
	// Implementation
	return nil
}
```

## Step 4: Build

```bash
./build.sh
```

Or manually:
```bash
go build -o analytics .
```

Output: `analytics` binary (~10MB)

## Step 5: Install

```bash
# Copy binary and manifest
cp analytics manifest.json /path/to/goatflow/plugins/analytics/

# Make executable
chmod +x /path/to/goatflow/plugins/analytics/analytics
```

GoatFlow will start the plugin process automatically.

## Step 6: Test

### Check Plugin Status
```bash
curl http://localhost:8080/api/v1/plugins | jq '.[] | select(.Name=="analytics")'
```

### Test API
```bash
# List reports
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/plugins/analytics/reports

# Run a report
curl -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"date_range": {"start": "2024-01-01", "end": "2024-01-31"}}' \
  http://localhost:8080/api/plugins/analytics/reports/ticket-volume/run
```

## Using Other Languages

### Python

```python
# plugin.py
import grpc
from concurrent import futures
import plugin_pb2
import plugin_pb2_grpc

class AnalyticsPlugin(plugin_pb2_grpc.PluginServiceServicer):
    def __init__(self):
        self.host = None
    
    def SetHost(self, stub):
        self.host = stub
    
    def Register(self, request, context):
        return plugin_pb2.RegisterResponse(
            name="analytics",
            version="1.0.0",
            description="Analytics plugin in Python"
        )
    
    def Call(self, request, context):
        if request.function == "list_reports":
            return self.list_reports(request)
        # ... other handlers
    
    def list_reports(self, request):
        import json
        reports = [
            {"id": "volume", "name": "Ticket Volume"},
        ]
        return plugin_pb2.CallResponse(result=json.dumps(reports).encode())

def serve():
    import os
    socket_path = os.environ.get('GOATFLOW_PLUGIN_SOCKET', '/tmp/goatflow-plugin-analytics.sock')
    
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    plugin_pb2_grpc.add_PluginServiceServicer_to_server(AnalyticsPlugin(), server)
    server.add_insecure_port(f'unix://{socket_path}')
    server.start()
    server.wait_for_termination()

if __name__ == '__main__':
    serve()
```

### Node.js

```javascript
// plugin.js
const grpc = require('@grpc/grpc-js');
const protoLoader = require('@grpc/proto-loader');

const packageDefinition = protoLoader.loadSync('plugin.proto');
const pluginProto = grpc.loadPackageDefinition(packageDefinition).plugin;

let hostClient = null;

const plugin = {
  SetHost(call, callback) {
    hostClient = call.request;
    callback(null, {});
  },
  
  Register(call, callback) {
    callback(null, {
      name: 'analytics',
      version: '1.0.0',
      description: 'Analytics plugin in Node.js'
    });
  },
  
  Call(call, callback) {
    const { function: fn, args } = call.request;
    
    switch (fn) {
      case 'list_reports':
        callback(null, {
          result: JSON.stringify([
            { id: 'volume', name: 'Ticket Volume' }
          ])
        });
        break;
      default:
        callback(new Error(`Unknown function: ${fn}`));
    }
  }
};

const server = new grpc.Server();
server.addService(pluginProto.PluginService.service, plugin);

const socketPath = process.env.GOATFLOW_PLUGIN_SOCKET || '/tmp/goatflow-plugin-analytics.sock';
server.bindAsync(`unix://${socketPath}`, grpc.ServerCredentials.createInsecure(), () => {
  console.log(`Plugin listening on ${socketPath}`);
  server.start();
});
```

## Advanced Patterns

### Concurrent Processing

```go
func (p *AnalyticsPlugin) runBatchReports(ctx context.Context, reportIDs []string) {
	var wg sync.WaitGroup
	results := make(chan ReportResult, len(reportIDs))
	
	for _, id := range reportIDs {
		wg.Add(1)
		go func(reportID string) {
			defer wg.Done()
			result := p.runSingleReport(ctx, reportID)
			results <- result
		}(id)
	}
	
	wg.Wait()
	close(results)
	
	// Collect results
	var allResults []ReportResult
	for r := range results {
		allResults = append(allResults, r)
	}
}
```

### External API Integration

```go
func (p *AnalyticsPlugin) fetchExternalData(ctx context.Context) ([]byte, error) {
	// gRPC plugins can make direct HTTP calls
	resp, err := http.Get("https://api.external-service.com/data")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	return io.ReadAll(resp.Body)
}
```

### Graceful Shutdown

```go
func main() {
	// ... setup ...
	
	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		<-sigCh
		log.Println("Shutting down...")
		server.GracefulStop()
	}()
	
	server.Serve(lis)
}
```

## Debugging

### Run Standalone

```bash
# Set environment and run directly
export GOATFLOW_PLUGIN_SOCKET=/tmp/test-plugin.sock
./analytics
```

### Use Delve Debugger

```bash
dlv exec ./analytics -- 
```

### Add Verbose Logging

```go
func (p *AnalyticsPlugin) Call(ctx context.Context, req *pb.CallRequest) (*pb.CallResponse, error) {
	log.Printf("Call received: function=%s args=%s", req.Function, string(req.Args))
	
	result, err := p.dispatch(ctx, req)
	
	if err != nil {
		log.Printf("Call failed: %v", err)
	} else {
		log.Printf("Call succeeded: %d bytes", len(result.Result))
	}
	
	return result, err
}
```

## Next Steps

- [Host API Reference](./HOST_API.md) - Full API documentation
- [Plugin Author Guide](./AUTHOR_GUIDE.md) - Best practices
- [WASM Tutorial](./WASM_TUTORIAL.md) - Lighter-weight alternative
