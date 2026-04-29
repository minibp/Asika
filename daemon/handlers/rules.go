package handlers

import (
    "encoding/json"
    "net/http"

    "github.com/gin-gonic/gin"
    "log/slog"

    "asika/common/config"
    "asika/common/db"
    "asika/common/models"
)

// GetLabelRules gets the current label rules
func GetLabelRules(c *gin.Context) {
    cfg := config.Current

    // Try to get from DB first (hot-loaded)
    data, err := db.Get(db.BucketConfig, "label_rules")
    if err == nil && data != nil {
        var rules []models.LabelRule
        if err := json.Unmarshal(data, &rules); err == nil {
            c.JSON(http.StatusOK, rules)
            return
        }
    }

    // Fallback to config
    c.JSON(http.StatusOK, cfg.LabelRules)
}

// UpdateLabelRules updates label rules (hot reload)
func UpdateLabelRules(c *gin.Context) {
    var rules []models.LabelRule

    if err := c.ShouldBindJSON(&rules); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request", "code": 400})
        return
    }

    data, err := json.Marshal(rules)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "code": 500})
        return
    }

    // Store in DB for hot reload
    if err := db.Put(db.BucketConfig, "label_rules", data); err != nil {
        slog.Error("failed to store label rules", "error", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "code": 500})
        return
    }

    // Update in-memory config
    config.Current.LabelRules = rules

    c.JSON(http.StatusOK, gin.H{"message": "label rules updated"})
}
