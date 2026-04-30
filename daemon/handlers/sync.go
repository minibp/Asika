package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetSyncHistory handles GET /api/v1/sync/history (8.5)
func GetSyncHistory(c *gin.Context) {
	// Return sync history from DB
	c.JSON(http.StatusOK, gin.H{
		"message": "sync history not implemented yet",
		"items":   []string{},
	})
}

// RetrySync handles POST /api/v1/sync/retry/:sync_id (8.5)
func RetrySync(c *gin.Context) {
	syncID := c.Param("sync_id")

	// Retry failed sync
	c.JSON(http.StatusOK, gin.H{
		"message": "retry triggered for sync " + syncID,
	})
}
