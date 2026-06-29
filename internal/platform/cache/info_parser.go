package cache

import (
	"strconv"
	"strings"
)

// parseRedisInfo parses Redis INFO output into a map.
//
//nolint:unused
func parseRedisInfo(info string) map[string]interface{} {
	result := make(map[string]interface{})
	lines := strings.Split(info, "\r\n")

	var currentSection string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			// Extract section name
			if strings.HasPrefix(line, "# ") {
				currentSection = strings.ToLower(strings.TrimPrefix(line, "# "))
				result[currentSection] = make(map[string]interface{})
			}
			continue
		}

		// Parse key:value pairs
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		// Try to parse value as number
		var parsedValue interface{}
		if intVal, err := strconv.ParseInt(value, 10, 64); err == nil {
			parsedValue = intVal
		} else if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
			parsedValue = floatVal
		} else {
			parsedValue = value
		}

		// Add to appropriate section
		if currentSection != "" {
			if section, ok := result[currentSection].(map[string]interface{}); ok {
				section[key] = parsedValue
			}
		} else {
			result[key] = parsedValue
		}
	}

	return result
}
