package handlers

import (
    "bytes"
    "encoding/json"
    "net/http"

    "github.com/gin-gonic/gin"
    "log/slog"

    "asika/common/db"
    "asika/common/models"
)

// GetQueue gets the merge queue for a repo group
func GetQueue(c *gin.Context) {
    repoGroup := c.Param("repo_group")

    items := make([]models.QueueItem, 0)
    prefix := []byte(repoGroup + "#")

    err := db.ForEach(db.BucketQueueItems, func(key, value []byte) error {
        if !bytes.HasPrefix(key, prefix) {
            return nil
        }

        var item models.QueueItem
        if err := json.Unmarshal(value, &item); err != nil {
            return err
        }

        items = append(items, item)
        return nil
    })

    if err != nil {
        slog.Error("failed to get queue", "error", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "code": 500})
        return
    }

    c.JSON(http.StatusOK, items)
}

// RecheckQueue triggers a manual recheck of the queue
func RecheckQueue(c *gin.Context) {
    repoGroup := c.Param("repo_group")

    slog.Info("manual queue recheck triggered", "repo_group", repoGroup)

    // This would trigger the queue checker
    // For now, just return success
    c.JSON(http.StatusOK, gin.H{"message": "queue recheck triggered"})
}
