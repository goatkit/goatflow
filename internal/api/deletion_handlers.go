package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/deletion"
)

func deletionService(c *gin.Context) *deletion.Service {
	svc, err := deletion.NewService()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database unavailable"})
		return nil
	}
	return svc
}

func handleAdminRecycleBinList(c *gin.Context) {
	svc := deletionService(c)
	if svc == nil {
		return
	}
	deletion.HandleAdminRecycleBinList(svc)(c)
}

func handleAdminRestore(c *gin.Context) {
	svc := deletionService(c)
	if svc == nil {
		return
	}
	deletion.HandleAdminRestore(svc)(c)
}

func handleAdminHardDelete(c *gin.Context) {
	svc := deletionService(c)
	if svc == nil {
		return
	}
	deletion.HandleAdminHardDelete(svc)(c)
}

func handleAdminBatchSoftDelete(c *gin.Context) {
	svc := deletionService(c)
	if svc == nil {
		return
	}
	deletion.HandleAdminBatchSoftDelete(svc)(c)
}

func handleAdminBatchHardDelete(c *gin.Context) {
	svc := deletionService(c)
	if svc == nil {
		return
	}
	deletion.HandleAdminBatchHardDelete(svc)(c)
}

func handleAdminDeletionLog(c *gin.Context) {
	svc := deletionService(c)
	if svc == nil {
		return
	}
	deletion.HandleAdminDeletionLog(svc)(c)
}
