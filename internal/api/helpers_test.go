package api

import (
	"math"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// A user_id the target type cannot hold must fall back instead of wrapping
// around into some other user's id.
func TestGetUserIDFromCtx_RejectsOutOfRangeIDs(t *testing.T) {
	ctxWith := func(v any) *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set("user_id", v)
		return c
	}

	tests := []struct {
		name     string
		value    any
		wantInt  int
		wantUint uint
	}{
		{"uint64 above MaxInt", uint64(math.MaxUint64), -7, 9},
		{"uint above MaxInt", uint(math.MaxUint), -7, 9},
		{"float beyond int range", float64(1e300), -7, 9},
		{"negative int for uint", -5, -5, 9},
		{"negative int64 for uint", int64(-5), -5, 9},
		{"non-numeric string", "abc", -7, 9},
		{"in-range uint64", uint64(42), 42, 42},
		{"numeric string", "42", 42, 42},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantInt, GetUserIDFromCtx(ctxWith(tt.value), -7))
			assert.Equal(t, tt.wantUint, GetUserIDFromCtxUint(ctxWith(tt.value), 9))
		})
	}
}
