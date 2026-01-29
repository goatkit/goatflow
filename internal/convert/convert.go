// Package convert provides type conversion utilities.
// This package has no dependencies on other internal packages to avoid circular imports.
package convert

import "strconv"

// ToInt converts various types to int with a fallback value.
// Handles all integer, unsigned, float, and string types.
func ToInt(v interface{}, fallback int) int {
	switch val := v.(type) {
	case int:
		return val
	case int8:
		return int(val)
	case int16:
		return int(val)
	case int32:
		return int(val)
	case int64:
		return int(val)
	case uint:
		return int(val)
	case uint8:
		return int(val)
	case uint16:
		return int(val)
	case uint32:
		return int(val)
	case uint64:
		return int(val)
	case float32:
		return int(val)
	case float64:
		return int(val)
	case string:
		if n, err := strconv.Atoi(val); err == nil {
			return n
		}
	}
	return fallback
}

// ToUint converts various types to uint with a fallback value.
// Handles all integer, unsigned, float, and string types.
// Returns fallback for negative values.
func ToUint(v interface{}, fallback uint) uint {
	switch val := v.(type) {
	case int:
		if val < 0 {
			return fallback
		}
		return uint(val)
	case int8:
		if val < 0 {
			return fallback
		}
		return uint(val)
	case int16:
		if val < 0 {
			return fallback
		}
		return uint(val)
	case int32:
		if val < 0 {
			return fallback
		}
		return uint(val)
	case int64:
		if val < 0 {
			return fallback
		}
		return uint(val)
	case uint:
		return val
	case uint8:
		return uint(val)
	case uint16:
		return uint(val)
	case uint32:
		return uint(val)
	case uint64:
		return uint(val)
	case float32:
		if val < 0 {
			return fallback
		}
		return uint(val)
	case float64:
		if val < 0 {
			return fallback
		}
		return uint(val)
	case string:
		if n, err := strconv.ParseUint(val, 10, 64); err == nil {
			return uint(n)
		}
	}
	return fallback
}

// ToString converts various types to string.
// Handles string, int, int64, uint, uint64, float64, and bool types.
func ToString(v interface{}, fallback string) string {
	switch val := v.(type) {
	case string:
		return val
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	case uint:
		return strconv.FormatUint(uint64(val), 10)
	case uint64:
		return strconv.FormatUint(val, 10)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(val)
	}
	return fallback
}
