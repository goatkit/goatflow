package api

// safeString extracts string from interface{} safely.
func safeString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
