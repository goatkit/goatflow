# Zinc Search Integration

## Overview

GoatFlow uses Zinc as its search engine, providing Elasticsearch-compatible APIs for full-text search across tickets, knowledge base articles, and other content. Zinc is lightweight, requires no JVM, and offers excellent performance for ticketing system needs.

## Architecture

```
┌────────────────────────────────────────────┐
│             GoatFlow Application           │
│                                            │
│  ┌──────────────────────────────────────┐  │
│  │         Search Service               │  │
│  │  - Index tickets on create/update    │  │
│  │  - Execute search queries            │  │
│  │  - Manage search indices             │  │
│  └─────────────┬────────────────────────┘  │
│                │                           │
│  ┌─────────────▼────────────────────────┐  │
│  │      Zinc Client (ES Compatible)     │  │
│  │  - HTTP/REST API                     │  │
│  │  - Bulk indexing                     │  │
│  │  - Query DSL                         │  │
│  └─────────────┬────────────────────────┘  │
└────────────────┼───────────────────────────┘
                 │
    ┌────────────▼─────────────┐
    │      Zinc Server         │
    │  - Full-text search      │
    │  - Faceted search        │
    │  - Aggregations          │
    │  - Auto-complete         │
    └──────────────────────────┘
```

## Configuration

### Docker Compose Setup

```yaml
zinc:
  image: public.ecr.aws/zinclabs/zinc:0.4.9
  container_name: goatflow-zinc
  ports:
    - "4080:4080"
  environment:
    ZINC_FIRST_ADMIN_USER: ${ZINC_USER}
    ZINC_FIRST_ADMIN_PASSWORD: ${ZINC_PASSWORD}
    ZINC_DATA_PATH: /data
  volumes:
    - zinc-data:/data
  networks:
    - goatflow-network
  restart: unless-stopped
```

### Environment Variables

```bash
# .env
ZINC_URL=http://zinc:4080
ZINC_USER=your_zinc_admin_user
ZINC_PASSWORD=your_secure_zinc_password
ZINC_INDEX_PREFIX=goatflow_
```

## Search Service Implementation

### Client Configuration

```go
package search

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
)

type ZincClient struct {
    baseURL  string
    username string
    password string
    client   *http.Client
}

func NewZincClient(baseURL, username, password string) *ZincClient {
    return &ZincClient{
        baseURL:  baseURL,
        username: username,
        password: password,
        client: &http.Client{
            Timeout: 10 * time.Second,
        },
    }
}

func (z *ZincClient) request(method, path string, body interface{}) (*http.Response, error) {
    var bodyReader io.Reader
    if body != nil {
        jsonBody, err := json.Marshal(body)
        if err != nil {
            return nil, err
        }
        bodyReader = bytes.NewReader(jsonBody)
    }

    req, err := http.NewRequest(method, z.baseURL+path, bodyReader)
    if err != nil {
        return nil, err
    }

    req.SetBasicAuth(z.username, z.password)
    req.Header.Set("Content-Type", "application/json")

    return z.client.Do(req)
}
```

### Index Management

```go
// CreateIndex creates a new search index
func (z *ZincClient) CreateIndex(indexName string) error {
    mapping := map[string]interface{}{
        "name": indexName,
        "mappings": map[string]interface{}{
            "properties": map[string]interface{}{
                "ticket_number": map[string]interface{}{
                    "type":     "keyword",
                    "index":    true,
                    "store":    true,
                    "sortable": true,
                },
                "title": map[string]interface{}{
                    "type":           "text",
                    "index":          true,
                    "store":          true,
                    "highlightable":  true,
                    "analyzer":       "standard",
                },
                "description": map[string]interface{}{
                    "type":           "text",
                    "index":          true,
                    "store":          true,
                    "highlightable":  true,
                    "analyzer":       "standard",
                },
                "status": map[string]interface{}{
                    "type":       "keyword",
                    "index":      true,
                    "store":      true,
                    "sortable":   true,
                    "aggregatable": true,
                },
                "priority": map[string]interface{}{
                    "type":       "numeric",
                    "index":      true,
                    "store":      true,
                    "sortable":   true,
                    "aggregatable": true,
                },
                "customer_email": map[string]interface{}{
                    "type":  "keyword",
                    "index": true,
                    "store": true,
                },
                "agent_name": map[string]interface{}{
                    "type":       "keyword",
                    "index":      true,
                    "store":      true,
                    "aggregatable": true,
                },
                "queue": map[string]interface{}{
                    "type":       "keyword",
                    "index":      true,
                    "store":      true,
                    "aggregatable": true,
                },
                "tags": map[string]interface{}{
                    "type":  "keyword",
                    "index": true,
                    "store": true,
                },
                "created_at": map[string]interface{}{
                    "type":     "time",
                    "index":    true,
                    "store":    true,
                    "sortable": true,
                    "format":   "2006-01-02T15:04:05Z07:00",
                },
                "updated_at": map[string]interface{}{
                    "type":     "time",
                    "index":    true,
                    "store":    true,
                    "sortable": true,
                    "format":   "2006-01-02T15:04:05Z07:00",
                },
            },
        },
    }

    resp, err := z.request("POST", "/api/index", mapping)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("failed to create index: %s", resp.Status)
    }

    return nil
}
```

### Indexing Documents

```go
// IndexTicket indexes a ticket document
func (z *ZincClient) IndexTicket(ticket *models.Ticket) error {
    doc := map[string]interface{}{
        "ticket_number":  ticket.Number,
        "title":         ticket.Title,
        "description":   ticket.Description,
        "status":        ticket.Status,
        "priority":      ticket.Priority,
        "customer_email": ticket.Customer.Email,
        "agent_name":    ticket.Agent.Name,
        "queue":         ticket.Queue.Name,
        "tags":          ticket.Tags,
        "created_at":    ticket.CreatedAt,
        "updated_at":    ticket.UpdatedAt,
    }

    path := fmt.Sprintf("/api/%s/_doc/%s", "goatflow_tickets", ticket.ID)
    resp, err := z.request("PUT", path, doc)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
        return fmt.Errorf("failed to index ticket: %s", resp.Status)
    }

    return nil
}

// BulkIndex indexes multiple documents
func (z *ZincClient) BulkIndex(indexName string, documents []map[string]interface{}) error {
    bulkData := map[string]interface{}{
        "index": indexName,
        "records": documents,
    }

    resp, err := z.request("POST", "/api/_bulkv2", bulkData)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("bulk index failed: %s", resp.Status)
    }

    return nil
}
```

### Search Queries

```go
// SearchTickets performs a search query
func (z *ZincClient) SearchTickets(query string, filters map[string]interface{}) (*SearchResult, error) {
    searchQuery := map[string]interface{}{
        "search_type": "match",
        "query": map[string]interface{}{
            "term": query,
            "field": "_all",
        },
        "from": 0,
        "max_results": 20,
        "_source": true,
    }

    // Add filters
    if len(filters) > 0 {
        must := []map[string]interface{}{}
        
        // Text search
        if query != "" {
            must = append(must, map[string]interface{}{
                "match": map[string]interface{}{
                    "_all": query,
                },
            })
        }

        // Status filter
        if status, ok := filters["status"]; ok {
            must = append(must, map[string]interface{}{
                "term": map[string]interface{}{
                    "status": status,
                },
            })
        }

        // Date range filter
        if from, ok := filters["date_from"]; ok {
            must = append(must, map[string]interface{}{
                "range": map[string]interface{}{
                    "created_at": map[string]interface{}{
                        "gte": from,
                    },
                },
            })
        }

        searchQuery = map[string]interface{}{
            "search_type": "querystring",
            "query": map[string]interface{}{
                "bool": map[string]interface{}{
                    "must": must,
                },
            },
            "from": 0,
            "max_results": 20,
            "_source": true,
            "highlight": map[string]interface{}{
                "fields": map[string]interface{}{
                    "title": map[string]interface{}{},
                    "description": map[string]interface{}{},
                },
            },
        }
    }

    resp, err := z.request("POST", "/api/goatflow_tickets/_search", searchQuery)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var result SearchResult
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }

    return &result, nil
}
```

### Aggregations

```go
// GetTicketStats returns aggregated statistics
func (z *ZincClient) GetTicketStats() (*TicketStats, error) {
    query := map[string]interface{}{
        "search_type": "match_all",
        "query": map[string]interface{}{},
        "aggs": map[string]interface{}{
            "status_count": map[string]interface{}{
                "terms": map[string]interface{}{
                    "field": "status",
                },
            },
            "priority_count": map[string]interface{}{
                "terms": map[string]interface{}{
                    "field": "priority",
                },
            },
            "queue_count": map[string]interface{}{
                "terms": map[string]interface{}{
                    "field": "queue",
                },
            },
            "daily_tickets": map[string]interface{}{
                "date_histogram": map[string]interface{}{
                    "field":    "created_at",
                    "interval": "day",
                },
            },
        },
    }

    resp, err := z.request("POST", "/api/goatflow_tickets/_search", query)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var result map[string]interface{}
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }

    // Parse aggregations
    stats := &TicketStats{}
    if aggs, ok := result["aggregations"].(map[string]interface{}); ok {
        // Process aggregation results
        stats.ParseAggregations(aggs)
    }

    return stats, nil
}
```

### Auto-complete

```go
// Autocomplete provides search suggestions
func (z *ZincClient) Autocomplete(prefix string, field string) ([]string, error) {
    query := map[string]interface{}{
        "search_type": "prefix",
        "query": map[string]interface{}{
            "term": prefix,
            "field": field,
        },
        "max_results": 10,
        "_source": []string{field},
    }

    resp, err := z.request("POST", "/api/goatflow_tickets/_search", query)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var result map[string]interface{}
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }

    suggestions := []string{}
    if hits, ok := result["hits"].(map[string]interface{}); ok {
        if hitsList, ok := hits["hits"].([]interface{}); ok {
            for _, hit := range hitsList {
                if h, ok := hit.(map[string]interface{}); ok {
                    if source, ok := h["_source"].(map[string]interface{}); ok {
                        if value, ok := source[field].(string); ok {
                            suggestions = append(suggestions, value)
                        }
                    }
                }
            }
        }
    }

    return suggestions, nil
}
```

## Integration with HTMX

### Search Form

```html
<!-- Search with live results -->
<div class="search-container">
    <input type="search" 
           name="q"
           placeholder="Search tickets..."
           hx-post="/api/search/tickets"
           hx-trigger="keyup changed delay:500ms, search"
           hx-target="#search-results"
           hx-indicator="#search-spinner">
    
    <div id="search-spinner" class="htmx-indicator">
        Searching...
    </div>
    
    <div id="search-results">
        <!-- Results will be loaded here -->
    </div>
</div>
```

### Search Handler

```go
func handleSearchTickets(searchService *SearchService) gin.HandlerFunc {
    return func(c *gin.Context) {
        query := c.PostForm("q")
        
        // Parse filters
        filters := map[string]interface{}{
            "status": c.PostForm("status"),
            "priority": c.PostForm("priority"),
        }
        
        // Perform search
        results, err := searchService.SearchTickets(query, filters)
        if err != nil {
            c.String(http.StatusInternalServerError, "Search failed")
            return
        }
        
        // Render results as HTML
        c.HTML(http.StatusOK, "search_results.html", gin.H{
            "results": results,
            "query": query,
        })
    }
}
```

## Monitoring and Maintenance

### Health Check

```go
func (z *ZincClient) HealthCheck() error {
    resp, err := z.request("GET", "/health", nil)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("zinc unhealthy: %s", resp.Status)
    }
    
    return nil
}
```

### Index Statistics

```go
func (z *ZincClient) GetIndexStats(indexName string) (*IndexStats, error) {
    resp, err := z.request("GET", fmt.Sprintf("/api/index/%s", indexName), nil)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var stats IndexStats
    if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
        return nil, err
    }
    
    return &stats, nil
}
```

### Reindexing

```go
func (z *ZincClient) ReindexAllTickets(tickets []*models.Ticket) error {
    // Delete existing index
    if err := z.DeleteIndex("goatflow_tickets"); err != nil {
        return err
    }
    
    // Create new index with updated mappings
    if err := z.CreateIndex("goatflow_tickets"); err != nil {
        return err
    }
    
    // Bulk index all tickets
    documents := make([]map[string]interface{}, len(tickets))
    for i, ticket := range tickets {
        documents[i] = ticketToDocument(ticket)
    }
    
    return z.BulkIndex("goatflow_tickets", documents)
}
```

## Performance Optimization

### Batch Processing

```go
// Process tickets in batches
func indexTicketsBatch(client *ZincClient, tickets []*models.Ticket) error {
    batchSize := 100
    for i := 0; i < len(tickets); i += batchSize {
        end := i + batchSize
        if end > len(tickets) {
            end = len(tickets)
        }
        
        batch := tickets[i:end]
        documents := make([]map[string]interface{}, len(batch))
        for j, ticket := range batch {
            documents[j] = ticketToDocument(ticket)
        }
        
        if err := client.BulkIndex("goatflow_tickets", documents); err != nil {
            return fmt.Errorf("batch %d failed: %w", i/batchSize, err)
        }
    }
    
    return nil
}
```

### Caching Search Results

```go
type SearchCache struct {
    cache *cache.Cache
}

func (sc *SearchCache) GetOrSearch(key string, searchFunc func() (*SearchResult, error)) (*SearchResult, error) {
    // Check cache
    if cached, found := sc.cache.Get(key); found {
        return cached.(*SearchResult), nil
    }
    
    // Perform search
    result, err := searchFunc()
    if err != nil {
        return nil, err
    }
    
    // Cache result
    sc.cache.Set(key, result, 5*time.Minute)
    
    return result, nil
}
```

## Elasticsearch Compatibility

Zinc provides Elasticsearch-compatible APIs, allowing use of existing Elasticsearch clients:

```go
// Using official Elasticsearch Go client
import "github.com/elastic/go-elasticsearch/v8"

cfg := elasticsearch.Config{
    Addresses: []string{os.Getenv("ZINC_URL")},
    Username:  os.Getenv("ZINC_USER"),
    Password:  os.Getenv("ZINC_PASSWORD"),
}

es, err := elasticsearch.NewClient(cfg)
if err != nil {
    log.Fatal(err)
}

// Use standard Elasticsearch APIs
res, err := es.Search(
    es.Search.WithIndex("goatflow_tickets"),
    es.Search.WithBody(strings.NewReader(query)),
)
```

## Best Practices

1. **Index Design**: Create separate indices for different data types (tickets, KB articles, users)
2. **Mapping**: Define explicit mappings for better search performance
3. **Batch Operations**: Use bulk API for indexing multiple documents
4. **Caching**: Cache frequently accessed search results
5. **Monitoring**: Regular health checks and index statistics monitoring
6. **Security**: Use strong passwords and restrict network access
7. **Backup**: Regular backups of Zinc data directory