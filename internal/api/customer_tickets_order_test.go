package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/repository"
	"github.com/goatkit/goatflow/internal/service"
)

// The customer ticket list must not splice ?order= into SQL: an injected
// "desc LIMIT 0" used to empty the list (and any other SQL could be run).
func TestCustomerTicketListOrderIsNotInjectable(t *testing.T) {
	db, err := database.GetDB()
	require.NoError(t, err)
	setupTemplateRenderer(t)
	gin.SetMode(gin.TestMode)

	login := fmt.Sprintf("order-inj-%d", time.Now().UnixNano())
	title := fmt.Sprintf("ORDERINJ%d", time.Now().UnixNano())
	svc := service.NewTicketService(repository.NewTicketRepository(db))
	created, err := svc.Create(context.Background(), service.CreateTicketInput{
		Title: title, QueueID: 1, PriorityID: 3, UserID: 1, CustomerUserID: login,
	})
	require.NoError(t, err)
	t.Cleanup(func() { deleteNumberingTicket(db, int64(created.ID)) })

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_id", uint(1))
		c.Set("username", login)
		c.Set("user_role", "Customer")
		c.Next()
	})
	r.GET("/customer/tickets", handleCustomerTickets(db))

	for _, order := range []string{"desc", "asc", "desc LIMIT 0", "asc; DELETE FROM ticket", "nonsense"} {
		t.Run(order, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/customer/tickets?order="+url.QueryEscape(order), nil)
			req.Header.Set("Accept", "text/html")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			require.Contains(t, w.Body.String(), title, "ticket must stay listed whatever ?order= says")
		})
	}
}
