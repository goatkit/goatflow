package api

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/audit"
)

func TestRecordReadAccessFollowsSetting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	origEnabled, origRecord := auditReadAccessEnabled, recordAuditRequest
	t.Cleanup(func() { auditReadAccessEnabled, recordAuditRequest = origEnabled, origRecord })

	var got []audit.Event
	recordAuditRequest = func(_ *gin.Context, e audit.Event) { got = append(got, e) }

	for _, enabled := range []bool{false, true} {
		got = nil
		auditReadAccessEnabled = func() bool { return enabled }
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/ticket/2026103110000001", nil)
		c.Set("user_id", 7)

		recordTicketView(c, 42, "2026103110000001")
		recordAttachmentDownload(c, 42, 9, 3, "invoice.pdf")

		if !enabled {
			if len(got) != 0 {
				t.Fatalf("recorded %d events with read access off", len(got))
			}
			continue
		}
		if len(got) != 2 {
			t.Fatalf("recorded %d events with read access on, want 2", len(got))
		}
		view, dl := got[0], got[1]
		if view.Action != "access.ticket.view" || view.Category != audit.CategoryAccess || view.TargetID != "42" || view.ActorID != 7 {
			t.Errorf("view event = %+v", view)
		}
		if dl.Action != "access.attachment.download" || dl.TargetType != "attachment" || dl.TargetID != "3" ||
			dl.Details["ticket_id"] != int64(42) || dl.Details["filename"] != "invoice.pdf" {
			t.Errorf("download event = %+v", dl)
		}
	}
}
