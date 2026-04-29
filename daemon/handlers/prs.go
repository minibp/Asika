package handlers

import (
    "bytes"
    "encoding/json"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "log/slog"

    "asika/common/config"
    "asika/common/db"
    "asika/common/models"
)

// ListPRs lists PRs in a repo group
func ListPRs(c *gin.Context) {
    repoGroup := c.Param("repo_group")
    state := c.DefaultQuery("state", "open")
    platform := c.DefaultQuery("platform", "")

    cfg := config.Current()
    group := config.GetRepoGroupByName(cfg, repoGroup)
    if group == nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "repo group not found", "code": 404})
        return
    }

    // Get PRs from DB
    prs := make([]models.PRRecord, 0)
    prefix := []byte(repoGroup + "#")

    err := db.ForEach(db.BucketPRs, func(key, value []byte) error {
        if !bytes.HasPrefix(key, prefix) {
            return nil
        }

        var pr models.PRRecord
        if err := json.Unmarshal(value, &pr); err != nil {
            return err
        }

        if state != "" && pr.State != state {
            return nil
        }
        if platform != "" && pr.Platform != platform {
            return nil
        }

        prs = append(prs, pr)
        return nil
    })

    if err != nil {
        slog.Error("failed to list PRs", "error", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "code": 500})
        return
    }

    c.JSON(http.StatusOK, prs)
}

// GetPR gets a single PR
func GetPR(c *gin.Context) {
    repoGroup := c.Param("repo_group")
    prID := c.Param("pr_id")

    key := repoGroup + "#" + prID

    data, err := db.Get(db.BucketPRs, key)
    if err != nil || data == nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "PR not found", "code": 404})
        return
    }

    var pr models.PRRecord
    if err := json.Unmarshal(data, &pr); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "code": 500})
        return
    }

    c.JSON(http.StatusOK, pr)
}

// ApprovePR approves a PR
func ApprovePR(c *gin.Context) {
    repoGroup := c.Param("repo_group")
    prID := c.Param("pr_id")

    // Get PR from DB to find platform and number
    key := repoGroup + "#" + prID
    data, err := db.Get(db.BucketPRs, key)
    if err != nil || data == nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "PR not found", "code": 404})
        return
    }

    var pr models.PRRecord
    if err := json.Unmarshal(data, &pr); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "code": 500})
        return
    }

    // This is a simplified version - need to get actual platform client
    // For now, just log and return success
    slog.Info("approving PR", "pr", prID, "platform", pr.Platform)

    c.JSON(http.StatusOK, gin.H{"message": "PR approved"})
}

// ClosePR closes a PR
func ClosePR(c *gin.Context) {
    repoGroup := c.Param("repo_group")
    prID := c.Param("pr_id")

    key := repoGroup + "#" + prID
    data, err := db.Get(db.BucketPRs, key)
    if err != nil || data == nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "PR not found", "code": 404})
        return
    }

    var pr models.PRRecord
    if err := json.Unmarshal(data, &pr); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "code": 500})
        return
    }

    pr.State = "closed"
    pr.UpdatedAt = time.Now()

    updated, err := json.Marshal(pr)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "code": 500})
        return
    }

    if err := db.Put(db.BucketPRs, key, updated); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "code": 500})
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "PR closed"})
}

// ReopenPR reopens a PR
func ReopenPR(c *gin.Context) {
    repoGroup := c.Param("repo_group")
    prID := c.Param("pr_id")

    key := repoGroup + "#" + prID
    data, err := db.Get(db.BucketPRs, key)
    if err != nil || data == nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "PR not found", "code": 404})
        return
    }

    var pr models.PRRecord
    if err := json.Unmarshal(data, &pr); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "code": 500})
        return
    }

    pr.State = "open"
    pr.UpdatedAt = time.Now()

    updated, err := json.Marshal(pr)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "code": 500})
        return
    }

    if err := db.Put(db.BucketPRs, key, updated); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "code": 500})
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "PR reopened"})
}

// MarkSpam marks a PR as spam
func MarkSpam(c *gin.Context) {
    repoGroup := c.Param("repo_group")
    prID := c.Param("pr_id")

    key := repoGroup + "#" + prID
    data, err := db.Get(db.BucketPRs, key)
    if err != nil || data == nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "PR not found", "code": 404})
        return
    }

    var pr models.PRRecord
    if err := json.Unmarshal(data, &pr); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "code": 500})
        return
    }

    pr.SpamFlag = true
    pr.State = "closed"
    pr.UpdatedAt = time.Now()

    updated, err := json.Marshal(pr)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "code": 500})
        return
    }

    if err := db.Put(db.BucketPRs, key, updated); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "code": 500})
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "PR marked as spam"})
}

// UnmarkSpam removes spam flag from a PR
func UnmarkSpam(c *gin.Context) {
    repoGroup := c.Param("repo_group")
    prID := c.Param("pr_id")

    key := repoGroup + "#" + prID
    data, err := db.Get(db.BucketPRs, key)
    if err != nil || data == nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "PR not found", "code": 404})
        return
    }

    var pr models.PRRecord
    if err := json.Unmarshal(data, &pr); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "code": 500})
        return
    }

    pr.SpamFlag = false
    pr.State = "open"
    pr.UpdatedAt = time.Now()

    updated, err := json.Marshal(pr)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "code": 500})
        return
    }

    if err := db.Put(db.BucketPRs, key, updated); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "code": 500})
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "spam flag removed"})
}
