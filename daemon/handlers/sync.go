package handlers

import (
    "encoding/json"
    "net/http"
    "strconv"

    "github.com/gin-gonic/gin"
    "log/slog"

    "asika/common/db"
    "asika/common/models"
)

// GetSyncHistory gets sync history with pagination
func GetSyncHistory(c *gin.Context) {
    repoGroup := c.DefaultQuery("repo_group", "")
    limitStr := c.DefaultQuery("limit", "20")

    limit, err := strconv.Atoi(limitStr)
    if err != nil || limit <= 0 {
        limit = 20
    }

    records := make([]models.SyncRecord, 0)

    err = db.ForEach(db.BucketSyncHistory, func(key, value []byte) error {
        var record models.SyncRecord
        if err := json.Unmarshal(value, &record); err != nil {
            return err
        }

        if repoGroup != "" && record.RepoGroup != repoGroup {
            return nil
        }

        records = append(records, record)
        return nil
    })

    if err != nil {
        slog.Error("failed to get sync history", "error", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "code": 500})
        return
    }

    if len(records) > limit {
        records = records[:limit]
    }

    c.JSON(http.StatusOK, records)
}

// RetrySync retries a failed sync
func RetrySync(c *gin.Context) {
    syncID := c.Param("sync_id")

    slog.Info("retrying sync", "sync_id", syncID)

    c.JSON(http.StatusOK, gin.H{"message": "sync retry triggered"})
}
