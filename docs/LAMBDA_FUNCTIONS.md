# Lambda Functions in Dynamic Modules

## Overview

Lambda functions bring programmable business logic directly into YAML module configurations using JavaScript. This revolutionary feature allows customers to customize data display, formatting, calculations, and database queries without touching Go code.

## Architecture

```
YAML Config (lambda: |...)  →  goja JavaScript Engine  →  Safe Execution Context
                              ↓
                         Database Access (read-only)
                              ↓  
                         Computed Field Result
```

## Key Features

### 🔒 **Security First**
- **Isolated contexts**: Each lambda runs in its own goja context, isolated from the host system
- **Read-only database**: Only SELECT queries allowed
- **Timeout protection**: 5-second execution limit prevents runaway scripts
- **SQL injection prevention**: Parameterized queries only

### ⚡ **High Performance**
- **goja Engine**: Pure-Go JavaScript interpreter - no external JS runtime to deploy
- **Resource limits**: Prevents performance degradation
- **Concurrent execution**: Each module runs in isolated context

### 🎯 **Easy to Use**
- **Familiar syntax**: Standard JavaScript (ES6+)
- **Built-in utilities**: Date formatting, database access
- **Rich context**: Access to item data, database queries
- **Error handling**: Graceful degradation on failures

## Basic Usage

### Simple Data Formatting

```yaml
computed_fields:
  - name: priority_status
    label: Priority
    show_in_list: true
    show_in_form: false
    lambda: |
      if (item.priority >= 5) {
        return '<span class="text-red-600 font-bold">High</span>';
      } else if (item.priority >= 3) {
        return '<span class="text-yellow-600">Medium</span>';
      } else {
        return '<span class="text-green-600">Low</span>';
      }
```

### Date Formatting

```yaml
computed_fields:
  - name: created_formatted
    label: Created
    show_in_list: true
    show_in_form: false
    lambda: |
      if (!item.create_time) {
        return '<span class="text-gray-400">Unknown</span>';
      }
      
      // Use built-in formatDate utility
      var formatted = formatDate(item.create_time);
      var date = new Date(item.create_time);
      var now = new Date();
      var diffDays = Math.floor((now - date) / (1000 * 60 * 60 * 24));
      
      var relative = '';
      if (diffDays === 0) {
        relative = 'Today';
      } else if (diffDays === 1) {
        relative = 'Yesterday';  
      } else if (diffDays < 7) {
        relative = diffDays + ' days ago';
      } else {
        relative = formatted;
      }
      
      return '<span title="' + formatted + '">' + relative + '</span>';
```

### Database Queries

```yaml
computed_fields:
  - name: ticket_count
    label: Active Tickets
    show_in_list: true
    show_in_form: false
    lambda: |
      try {
        var result = db.queryRow(
          "SELECT COUNT(*) as count FROM ticket WHERE priority_id = $1 AND state_id IN (1,2,3)", 
          item.id.toString()
        );
        
        if (result && result.count !== undefined) {
          var count = parseInt(result.count);
          if (count === 0) {
            return '<span class="text-gray-400">No tickets</span>';
          } else {
            return '<span class="text-blue-600 font-semibold">' + count + ' tickets</span>';
          }
        }
        
        return '<span class="text-gray-400">-</span>';
      } catch (error) {
        return '<span class="text-red-400">Error: ' + error + '</span>';
      }
```

## Advanced Examples

### Complex HTML Generation

```yaml
computed_fields:
  - name: progress_bar
    label: Progress
    show_in_list: true
    show_in_form: false
    lambda: |
      var progress = Math.min(Math.max(item.completion || 0, 0), 100);
      var color = progress < 30 ? 'bg-red-500' : progress < 70 ? 'bg-yellow-500' : 'bg-green-500';
      
      return '<div class="w-full bg-gray-200 rounded-full h-3">' +
             '<div class="' + color + ' h-3 rounded-full transition-all duration-300" ' +
             'style="width: ' + progress + '%"></div>' +
             '<span class="text-xs text-gray-600 ml-2">' + progress + '%</span>' +
             '</div>';
```

### Multi-row Database Queries

```yaml
computed_fields:
  - name: recent_activity
    label: Recent Activity
    show_in_list: true
    show_in_form: false
    lambda: |
      try {
        var activities = db.query(
          "SELECT action, created_at FROM activity_log WHERE user_id = $1 ORDER BY created_at DESC LIMIT 3",
          item.id.toString()
        );
        
        if (activities && activities.length > 0) {
          var html = '<ul class="text-sm space-y-1">';
          for (var i = 0; i < activities.length; i++) {
            var activity = activities[i];
            html += '<li class="text-gray-600">' + activity.action + '</li>';
          }
          html += '</ul>';
          return html;
        }
        
        return '<span class="text-gray-400">No activity</span>';
      } catch (error) {
        return '<span class="text-red-400">Query error</span>';
      }
```

### Conditional Logic with Multiple Fields

```yaml
computed_fields:
  - name: user_status_badge
    label: Status
    show_in_list: true
    show_in_form: false
    lambda: |
      var status = '';
      var color = '';
      
      // Check if user is active
      if (item.valid_id !== 1) {
        status = 'Inactive';
        color = 'bg-gray-100 text-gray-800';
      }
      // Check last login
      else if (!item.last_login || item.last_login === null) {
        status = 'Never logged in';
        color = 'bg-yellow-100 text-yellow-800';
      } 
      else {
        var lastLogin = new Date(item.last_login);
        var daysSinceLogin = Math.floor((new Date() - lastLogin) / (1000 * 60 * 60 * 24));
        
        if (daysSinceLogin > 90) {
          status = 'Inactive (' + daysSinceLogin + ' days)';
          color = 'bg-red-100 text-red-800';
        } else if (daysSinceLogin > 30) {
          status = 'Less active';
          color = 'bg-yellow-100 text-yellow-800';
        } else {
          status = 'Active';
          color = 'bg-green-100 text-green-800';
        }
      }
      
      return '<span class="px-2 py-1 text-xs rounded-full ' + color + '">' + status + '</span>';
```

## Built-in Utilities

### formatDate(dateString)
Formats a date string to user-friendly format.

```javascript
var formatted = formatDate(item.create_time);
// Returns: "Aug 23, 2025 4:30 PM"
```

### Database Access

#### db.queryRow(query, ...args)
Executes a query that returns a single row.

```javascript
var result = db.queryRow("SELECT COUNT(*) as count FROM table WHERE id = $1", item.id.toString());
// Returns: { count: 5 }
```

#### db.query(query, ...args) 
Executes a query that returns multiple rows.

```javascript
var rows = db.query("SELECT name FROM groups WHERE user_id = $1", item.id.toString());
// Returns: [{ name: "admin" }, { name: "users" }]
```

## Configuration Options

### Lambda Configuration (Optional)

```yaml
lambda_config:
  timeout_ms: 3000      # Execution timeout (default: 5000ms)
  memory_limit_mb: 16   # Memory limit (default: 32MB)
```

### Field Configuration

```yaml
computed_fields:
  - name: field_name        # Unique field identifier
    label: Display Name     # Label shown in UI
    show_in_list: true     # Show in table listings
    show_in_form: false    # Show in forms (usually false for computed fields)
    lambda: |              # JavaScript code
      return "Hello, World!";
```

## Security Guidelines

### Allowed Operations
- ✅ **Data formatting**: HTML generation, text manipulation
- ✅ **Mathematical calculations**: Numbers, dates, percentages
- ✅ **Conditional logic**: if/else, switch statements
- ✅ **String operations**: Concatenation, templating
- ✅ **Date/time operations**: Formatting, calculations
- ✅ **Array operations**: Iteration, filtering, mapping
- ✅ **Database SELECT queries**: Read-only data access

### Blocked Operations
- ❌ **File system access**: No file read/write operations
- ❌ **Network requests**: No HTTP calls or external APIs
- ❌ **System commands**: No shell/OS access
- ❌ **Database modifications**: No INSERT, UPDATE, DELETE
- ❌ **Infinite loops**: Execution timeout prevents runaway code
- ❌ **Memory abuse**: Memory limits prevent excessive allocation

### Best Practices

1. **Always handle errors**: Use try/catch blocks for database queries
2. **Validate data**: Check if fields exist before using them
3. **Use parameterized queries**: Always use $1, $2, etc. for query parameters
4. **Keep it simple**: Complex logic should be in the backend
5. **Test thoroughly**: Verify lambda functions work with various data
6. **Optimize performance**: Avoid expensive operations in tight loops

## Error Handling

### Graceful Degradation

```javascript
lambda: |
  try {
    var result = db.queryRow("SELECT COUNT(*) FROM table WHERE id = $1", item.id.toString());
    return result.count || 0;
  } catch (error) {
    console.log("Database error:", error);
    return '<span class="text-red-400">Error loading data</span>';
  }
```

### Field Validation

```javascript
lambda: |
  // Always check if fields exist
  if (!item.priority || item.priority === null || item.priority === undefined) {
    return '<span class="text-gray-400">No priority</span>';
  }
  
  var priority = parseInt(item.priority);
  if (isNaN(priority)) {
    return '<span class="text-red-400">Invalid priority</span>';
  }
  
  return priority >= 5 ? 'High' : 'Low';
```

## Performance Considerations

### Execution Limits
- **Timeout**: 5 seconds maximum execution time
- **Memory**: 32MB memory limit per execution
- **Concurrent**: Multiple lambda functions execute in parallel
- **Caching**: Consider caching expensive computations in the database

### Optimization Tips

```javascript
// Good: Simple, fast operations
lambda: |
  return item.status === 1 ? 'Active' : 'Inactive';

// Avoid: Complex loops in lambda functions
lambda: |
  // Instead of complex calculations here...
  // Consider pre-computing in database and just formatting the result
  var expensiveCalculation = 0;
  for (var i = 0; i < 10000; i++) {
    expensiveCalculation += Math.sqrt(i);
  }
  return expensiveCalculation;
```

## Debugging

### Common Issues

1. **Field access**: Use `item.field_name`, not `item['field_name']`
2. **Type conversion**: Always convert to string for database parameters
3. **SQL syntax**: Use PostgreSQL syntax for queries
4. **HTML escaping**: Be careful with quotes in generated HTML

### Debug Output

```javascript
lambda: |
  // Log values for debugging
  console.log("Item data:", JSON.stringify(item));
  console.log("Field value:", item.priority);
  
  return item.priority || 'No priority';
```

## Migration from Source-based Fields

### Old Way (Source-based)
```yaml
computed_fields:
  - name: full_name
    label: Name
    source: "CONCAT first_name, last_name"
```

### New Way (Lambda-based)
```yaml
computed_fields:
  - name: full_name
    label: Name
    lambda: |
      var firstName = item.first_name || '';
      var lastName = item.last_name || '';
      var fullName = (firstName + ' ' + lastName).trim();
      
      if (fullName === '') {
        return item.login || 'Unknown User';
      }
      
      return fullName;
```

## Real-world Use Cases

### E-commerce: Order Status
```javascript
lambda: |
  var status = item.status;
  var shippedDate = item.shipped_date;
  var deliveredDate = item.delivered_date;
  
  if (deliveredDate) {
    return '<span class="text-green-600">✓ Delivered</span>';
  } else if (shippedDate) {
    var daysSinceShipped = Math.floor((new Date() - new Date(shippedDate)) / (1000 * 60 * 60 * 24));
    return '<span class="text-blue-600">📦 Shipped (' + daysSinceShipped + ' days ago)</span>';
  } else if (status === 'processing') {
    return '<span class="text-yellow-600">⏳ Processing</span>';
  } else {
    return '<span class="text-gray-600">📝 Pending</span>';
  }
```

### Support Tickets: SLA Status
```javascript
lambda: |
  try {
    var createdDate = new Date(item.created_date);
    var now = new Date();
    var hoursOpen = Math.floor((now - createdDate) / (1000 * 60 * 60));
    
    // Get SLA hours from database
    var slaResult = db.queryRow("SELECT response_hours FROM sla WHERE priority_id = $1", item.priority_id.toString());
    var slaHours = slaResult.response_hours || 24;
    
    var remainingHours = slaHours - hoursOpen;
    
    if (remainingHours <= 0) {
      return '<span class="text-red-600 font-bold">⚠️ SLA Breached</span>';
    } else if (remainingHours <= 2) {
      return '<span class="text-orange-600">🔥 ' + remainingHours + 'h remaining</span>';
    } else {
      return '<span class="text-green-600">✓ ' + remainingHours + 'h remaining</span>';
    }
  } catch (error) {
    return '<span class="text-gray-400">SLA Unknown</span>';
  }
```

### User Management: Login Activity
```javascript
lambda: |
  if (!item.last_login_date) {
    return '<span class="text-gray-400">Never</span>';
  }
  
  var lastLogin = new Date(item.last_login_date);
  var now = new Date();
  var diffMs = now - lastLogin;
  var diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));
  var diffHours = Math.floor(diffMs / (1000 * 60 * 60));
  var diffMins = Math.floor(diffMs / (1000 * 60));
  
  var relativeTime = '';
  var color = '';
  
  if (diffMins < 5) {
    relativeTime = 'Just now';
    color = 'text-green-600';
  } else if (diffMins < 60) {
    relativeTime = diffMins + ' minutes ago';
    color = 'text-green-600';
  } else if (diffHours < 24) {
    relativeTime = diffHours + ' hours ago';
    color = 'text-blue-600';
  } else if (diffDays < 7) {
    relativeTime = diffDays + ' days ago';
    color = 'text-yellow-600';
  } else {
    relativeTime = diffDays + ' days ago';
    color = 'text-red-600';
  }
  
  return '<span class="' + color + '" title="' + formatDate(item.last_login_date) + '">' + relativeTime + '</span>';
```

## Conclusion

Lambda functions transform static YAML configurations into programmable, intelligent interfaces. They provide the perfect balance of power and safety, allowing customers to implement custom business logic without compromising security or performance.

**Key Benefits:**
- 🎯 **Customer customization** without code changes
- 🔒 **Enterprise security** with goja sandboxing
- ⚡ **High performance** JavaScript execution
- 🛠️ **Developer friendly** with familiar syntax
- 🔄 **Hot reload** for instant updates
- 📊 **Rich data access** with database queries

Lambda functions represent the future of configurable business applications - bringing the power of programming directly to the configuration layer.