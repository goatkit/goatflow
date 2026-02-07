# WASM Plugin Tutorial

Build your first WebAssembly plugin for GoatFlow using TinyGo.

## Prerequisites

- [TinyGo](https://tinygo.org/getting-started/install/) 0.30+
- [Go](https://golang.org/dl/) 1.21+
- GoatFlow running locally

## Step 1: Scaffold the Plugin

```bash
cd /path/to/goatflow/plugins
gk plugin init ticket-counter --type wasm
cd ticket-counter
```

This creates:
```
ticket-counter/
├── manifest.json
├── main.go
├── build.sh
└── README.md
```

## Step 2: Define the Manifest

Edit `manifest.json`:

```json
{
  "name": "ticket-counter",
  "version": "1.0.0",
  "description": "Displays ticket counts by state",
  "author": "Your Name",
  "license": "MIT",
  
  "routes": [
    {
      "method": "GET",
      "path": "/api/plugins/ticket-counter/counts",
      "handler": "get_counts",
      "middleware": ["auth"],
      "description": "Returns ticket counts by state"
    }
  ],
  
  "widgets": [
    {
      "id": "ticket-counts",
      "title": "Ticket Counts",
      "handler": "render_widget",
      "location": "agent_home",
      "size": "small"
    }
  ],
  
  "permissions": ["db:read", "cache:read", "cache:write"]
}
```

## Step 3: Implement the Plugin

Edit `main.go`:

```go
//go:build tinygo.wasm

package main

import (
	"encoding/json"
	"fmt"
)

// Host API imports - provided by GoatFlow runtime
//
//go:wasmimport host db_query
func hostDBQuery(queryPtr, queryLen, argsPtr, argsLen uint32) uint64

//go:wasmimport host cache_get
func hostCacheGet(keyPtr, keyLen uint32) uint64

//go:wasmimport host cache_set
func hostCacheSet(keyPtr, keyLen, valPtr, valLen, ttl uint32) uint32

//go:wasmimport host log
func hostLog(levelPtr, levelLen, msgPtr, msgLen, fieldsPtr, fieldsLen uint32)

// Memory management
var allocBuf = make([]byte, 0, 64*1024)

//export gk_alloc
func gkAlloc(size uint32) uint32 {
	if cap(allocBuf) < int(size) {
		allocBuf = make([]byte, size)
	} else {
		allocBuf = allocBuf[:size]
	}
	return uint32(uintptr(unsafe.Pointer(&allocBuf[0])))
}

//export gk_register
func gkRegister() uint64 {
	manifest := `{
		"name": "ticket-counter",
		"version": "1.0.0",
		"description": "Displays ticket counts by state"
	}`
	return packStringResult(manifest)
}

// TicketCount represents count for a state
type TicketCount struct {
	State string `json:"state"`
	Count int    `json:"count"`
}

//export get_counts
func getCounts(reqPtr, reqLen uint32) uint64 {
	// Try cache first
	cacheKey := "ticket_counts"
	if cached := cacheGet(cacheKey); cached != "" {
		return packStringResult(cached)
	}
	
	// Query database
	query := `
		SELECT ts.name as state, COUNT(*) as count 
		FROM tickets t 
		JOIN ticket_states ts ON t.state_id = ts.id 
		GROUP BY t.state_id, ts.name
		ORDER BY count DESC
	`
	
	rows, err := dbQuery(query)
	if err != nil {
		logError("Query failed", err)
		return packErrorResult(err.Error())
	}
	
	var counts []TicketCount
	for _, row := range rows {
		counts = append(counts, TicketCount{
			State: row["state"].(string),
			Count: int(row["count"].(float64)),
		})
	}
	
	result, _ := json.Marshal(counts)
	
	// Cache for 60 seconds
	cacheSet(cacheKey, string(result), 60)
	
	return packStringResult(string(result))
}

//export render_widget
func renderWidget(reqPtr, reqLen uint32) uint64 {
	// Get counts (uses cache)
	countsJSON := getCounts(0, 0)
	// ... parse and render HTML
	
	var counts []TicketCount
	json.Unmarshal(unpackResult(countsJSON), &counts)
	
	html := `<div class="grid grid-cols-2 gap-4">`
	for _, c := range counts {
		html += fmt.Sprintf(`
			<div class="stat bg-base-200 rounded-lg p-4">
				<div class="stat-title">%s</div>
				<div class="stat-value text-2xl">%d</div>
			</div>
		`, c.State, c.Count)
	}
	html += `</div>`
	
	return packStringResult(html)
}

// Helper functions

func dbQuery(query string, args ...any) ([]map[string]any, error) {
	argsJSON, _ := json.Marshal(args)
	result := hostDBQuery(
		stringToPtr(query), uint32(len(query)),
		uint32(uintptr(unsafe.Pointer(&argsJSON[0]))), uint32(len(argsJSON)),
	)
	
	data := unpackResult(result)
	var rows []map[string]any
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func cacheGet(key string) string {
	result := hostCacheGet(stringToPtr(key), uint32(len(key)))
	if result == 0 {
		return ""
	}
	return string(unpackResult(result))
}

func cacheSet(key, value string, ttl int) {
	hostCacheSet(
		stringToPtr(key), uint32(len(key)),
		stringToPtr(value), uint32(len(value)),
		uint32(ttl),
	)
}

func logError(msg string, err error) {
	fields, _ := json.Marshal(map[string]string{"error": err.Error()})
	level := "error"
	hostLog(
		stringToPtr(level), uint32(len(level)),
		stringToPtr(msg), uint32(len(msg)),
		uint32(uintptr(unsafe.Pointer(&fields[0]))), uint32(len(fields)),
	)
}

func stringToPtr(s string) uint32 {
	return uint32(uintptr(unsafe.Pointer(unsafe.StringData(s))))
}

func packStringResult(s string) uint64 {
	ptr := stringToPtr(s)
	return uint64(ptr)<<32 | uint64(len(s))
}

func packErrorResult(msg string) uint64 {
	return packStringResult(`{"error":"` + msg + `"}`)
}

func unpackResult(packed uint64) []byte {
	ptr := uint32(packed >> 32)
	len := uint32(packed & 0xFFFFFFFF)
	return unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), len)
}

func main() {}
```

## Step 4: Build

```bash
./build.sh
```

Or manually:
```bash
tinygo build -o ticket-counter.wasm -target wasm -gc=leaking -no-debug main.go
```

Output: `ticket-counter.wasm` (~50KB)

## Step 5: Install

Copy to plugins directory:
```bash
cp ticket-counter.wasm /path/to/goatflow/plugins/
```

With hot reload enabled, the plugin loads automatically. Otherwise restart GoatFlow.

## Step 6: Test

### Test the API
```bash
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/plugins/ticket-counter/counts
```

### View the Widget

Go to Agent Dashboard - your widget should appear.

### Check Logs

Admin → Plugins → View Logs → Filter: ticket-counter

## Using the GoatKit SDK (Recommended)

Instead of raw WASM imports, use the GoatKit SDK:

```go
//go:build tinygo.wasm

package main

import (
	"encoding/json"
	
	"github.com/goatkit/goatkit-sdk/wasm"
)

var host = wasm.GetHost()

//export get_counts
func getCounts(reqPtr, reqLen uint32) uint64 {
	ctx := wasm.Context()
	
	// Cache check
	if cached, found := host.CacheGet(ctx, "counts"); found {
		return wasm.PackResult(cached)
	}
	
	// Query
	rows, err := host.DBQuery(ctx, "SELECT state, COUNT(*) FROM tickets GROUP BY state")
	if err != nil {
		host.Log(ctx, "error", "Query failed", map[string]any{"error": err})
		return wasm.PackError(err)
	}
	
	result, _ := json.Marshal(rows)
	host.CacheSet(ctx, "counts", result, 60)
	
	return wasm.PackResult(result)
}
```

Install SDK:
```bash
go get github.com/goatkit/goatkit-sdk@latest
```

## Debugging Tips

### 1. Check Plugin Loaded

```bash
curl http://localhost:8080/api/v1/plugins | jq '.[] | select(.Name=="ticket-counter")'
```

### 2. Enable Debug Logs

```go
host.Log(ctx, "debug", "Starting query", map[string]any{
    "cache_key": cacheKey,
})
```

### 3. Test Locally

```bash
# Run GoatFlow with hot reload
GOATFLOW_PLUGIN_HOT_RELOAD=true ./goats serve
```

### 4. Check WASM Size

Keep plugins small (<1MB). Large plugins slow down loading.

```bash
ls -lh ticket-counter.wasm
# Should be ~50-200KB for simple plugins
```

## Common Patterns

### Pagination

```go
//export list_items
func listItems(reqPtr, reqLen uint32) uint64 {
	req := wasm.ParseRequest(reqPtr, reqLen)
	
	page := req.QueryInt("page", 1)
	limit := req.QueryInt("limit", 20)
	offset := (page - 1) * limit
	
	rows, _ := host.DBQuery(ctx, 
		"SELECT * FROM items LIMIT ? OFFSET ?", 
		limit, offset)
	
	return wasm.PackJSON(rows)
}
```

### Error Handling

```go
func safeHandler(reqPtr, reqLen uint32) uint64 {
	defer func() {
		if r := recover(); r != nil {
			host.Log(ctx, "error", "Panic recovered", map[string]any{
				"panic": fmt.Sprint(r),
			})
		}
	}()
	
	// ... handler logic
}
```

### Background Refresh

Use scheduled jobs instead of long-running goroutines:

```json
{
  "jobs": [{
    "id": "refresh-cache",
    "schedule": "*/5 * * * *",
    "handler": "refresh_cache"
  }]
}
```

## Limitations

WASM plugins have some limitations vs gRPC:

| Feature | WASM | gRPC |
|---------|------|------|
| Goroutines | Limited | Full |
| Network | Via Host API | Direct |
| File I/O | None | Full |
| External libs | TinyGo-compatible | Any |
| Memory | 256MB max | Unlimited |

For complex plugins needing full Go features, consider [gRPC plugins](./GRPC_TUTORIAL.md).

## Next Steps

- [Host API Reference](./HOST_API.md) - All available functions
- [Plugin Author Guide](./AUTHOR_GUIDE.md) - Best practices
- [gRPC Tutorial](./GRPC_TUTORIAL.md) - Alternative plugin type
