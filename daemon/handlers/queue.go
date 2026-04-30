package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"log/slog"

	"asika/common/config"
	"asika/common/db"
	"asika/common/models"
	"asika/daemon/queue"
)

// queueMgr is a package-level variable to access the queue manager
var queueMgr *queue.Manager

// InitQueueMgr initializes the queue manager for handlers
func InitQueueMgr(mgr *queue.Manager) {
	queueMgr = mgr
}

// GetQueue handles GET /api/v1/queue/:repo_group (8.3)
func GetQueue(c *gin.Context) {
	repoGroup := c.Param("repo_group")

	cfg := config.Current()
	group := config.GetRepoGroupByName(cfg, repoGroup)
	if group == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo group not found"})
		return
	}

	// Get queue items from DB
	var items []models.QueueItem
	err := db.ForEach(db.BucketQueueItems, func(key, value []byte) error {
		var item models.QueueItem
		if err := json.Unmarshal(value, &item); err != nil {
			return nil // Skip invalid entries
		}
		// Filter by repo group
		if strings.HasPrefix(string(key), repoGroup+"#") {
			items = append(items, item)
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read queue"})
		return
	}

	c.JSON(http.StatusOK, items)
}

// RecheckQueue handles POST /api/v1/queue/:repo_group/recheck (8.3)
func RecheckQueue(c *gin.Context) {
	repoGroup := c.Param("repo_group")

	cfg := config.Current()
	group := config.GetRepoGroupByName(cfg, repoGroup)
	if group == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo group not found"})
		return
	}

	if queueMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "queue manager not initialized"})
		return
	}

	// Trigger queue recheck
	go queueMgr.CheckQueue()
	slog.Info("queue recheck triggered", "repo_group", repoGroup)

	c.JSON(http.StatusOK, gin.H{"message": "queue recheck triggered"})
}
