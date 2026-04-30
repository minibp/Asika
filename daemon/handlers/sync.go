package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"log/slog"

	"asika/common/db"
	"asika/common/models"
)

// GetSyncHistory handles GET /api/v1/sync/history (8.5)
func GetSyncHistory(c *gin.Context) {
	var history []models.SyncRecord
	err := db.ForEach(db.BucketSyncHistory, func(key, value []byte) error {
		var record models.SyncRecord
		if err := json.Unmarshal(value, &record); err != nil {
			return nil // Skip invalid entries
		}
		history = append(history, record)
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read sync history"})
		return
	}

	c.JSON(http.StatusOK, history)
}

// RetrySync handles POST /api/v1/sync/retry/:sync_id (8.5)
func RetrySync(c *gin.Context) {
	syncID := c.Param("sync_id")

	// Get sync record from DB
	data, err := db.Get(db.BucketSyncHistory, syncID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "sync record not found"})
		return
	}

	var record models.SyncRecord
	if err := json.Unmarshal(data, &record); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse sync record"})
		return
	}

	// Only retry failed syncs
	if record.Status != "failed" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "only failed syncs can be retried"})
		return
	}

	// TODO: Implement actual retry logic
	// This would involve finding the original PR and calling syncer.SyncOnMerge()
	slog.Info("retry sync triggered", "sync_id", syncID)

	c.JSON(http.StatusOK, gin.H{"message": "retry triggered for sync " + syncID})
}
