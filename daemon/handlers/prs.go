package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"log/slog"

	"asika/common/config"
	"asika/common/db"
	"asika/common/models"
	"asika/common/platforms"
	"asika/daemon/syncer"
)

// clients is a package-level variable to access platform clients
var clients map[platforms.PlatformType]platforms.PlatformClient

// syncerRef is set by InitSyncer from cmd/asikad/main.go
var syncerRef *syncer.Syncer

// InitClients initializes the platform clients for handlers
func InitClients(c map[platforms.PlatformType]platforms.PlatformClient) {
	clients = c
}

// InitSyncer initializes the syncer for handlers
func InitSyncer(s *syncer.Syncer) {
	syncerRef = s
}

// ListPRs handles GET /api/v1/repos/:repo_group/prs (8.2)
func ListPRs(c *gin.Context) {
	repoGroup := c.Param("repo_group")
	state := c.Query("state")
	platform := c.Query("platform")
	isDraftStr := c.Query("is_draft")
	author := c.Query("author")
	label := c.Query("label")
	createdAfter := c.Query("created_after")
	updatedAfter := c.Query("updated_after")
	pageStr := c.Query("page")
	perPageStr := c.Query("per_page")

	records := make([]models.PRRecord, 0)

	cfg := config.Current()
	group := config.GetRepoGroupByName(cfg, repoGroup)
	if group == nil {
		c.JSON(http.StatusOK, records)
		return
	}

	// Get platform client
	client := getClientForGroup(group, platform)
	if client == nil {
		c.JSON(http.StatusOK, records)
		return
	}

	owner, repo := config.GetOwnerRepoFromGroup(group, platform)
	if owner == "" || repo == "" {
		c.JSON(http.StatusOK, records)
		return
	}

	prs, err := client.ListPRs(c.Request.Context(), owner, repo, state)
	if err != nil {
		slog.Error("failed to list PRs from platform", "error", err, "platform", platform, "repo_group", repoGroup)
		c.JSON(http.StatusOK, gin.H{"error": "platform API error: " + err.Error(), "data": records})
		return
	}

	// Convert to PRRecord format
	for _, pr := range prs {
		records = append(records, models.PRRecord{
			PRNumber: pr.PRNumber,
			Title:    pr.Title,
			Author:   pr.Author,
			State:    pr.State,
			Labels:   pr.Labels,
			Platform: platform,
			IsDraft:  pr.IsDraft,
		})
	}

	// Filter by draft status if requested
	if isDraftStr != "" {
		isDraft := isDraftStr == "true"
		filtered := make([]models.PRRecord, 0)
		for _, pr := range records {
			if pr.IsDraft == isDraft {
				filtered = append(filtered, pr)
			}
		}
		records = filtered
	}

	// Filter by author if requested
	if author != "" {
		filtered := make([]models.PRRecord, 0)
		for _, pr := range records {
			if pr.Author == author {
				filtered = append(filtered, pr)
			}
		}
		records = filtered
	}

	// Filter by label if requested
	if label != "" {
		filtered := make([]models.PRRecord, 0)
		for _, pr := range records {
			hasLabel := false
			for _, l := range pr.Labels {
				if l == label {
					hasLabel = true
					break
				}
			}
			if hasLabel {
				filtered = append(filtered, pr)
			}
		}
		records = filtered
	}

	// Filter by created_after if requested
	if createdAfter != "" {
		t, err := time.Parse(time.RFC3339, createdAfter)
		if err == nil {
			filtered := make([]models.PRRecord, 0)
			for _, pr := range records {
				if pr.CreatedAt.After(t) {
					filtered = append(filtered, pr)
				}
			}
			records = filtered
		}
	}

	// Filter by updated_after if requested
	if updatedAfter != "" {
		t, err := time.Parse(time.RFC3339, updatedAfter)
		if err == nil {
			filtered := make([]models.PRRecord, 0)
			for _, pr := range records {
				if pr.UpdatedAt.After(t) {
					filtered = append(filtered, pr)
				}
			}
			records = filtered
		}
	}

	// Pagination
	page := 1
	perPage := 20
	if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
		page = p
	}
	if pp, err := strconv.Atoi(perPageStr); err == nil && pp > 0 && pp <= 100 {
		perPage = pp
	}

	total := len(records)
	start := (page - 1) * perPage
	if start >= total {
		c.JSON(http.StatusOK, gin.H{"data": []models.PRRecord{}, "total": total, "page": page, "per_page": perPage})
		return
	}
	end := start + perPage
	if end > total {
		end = total
	}
	paged := records[start:end]

	c.JSON(http.StatusOK, gin.H{"data": paged, "total": total, "page": page, "per_page": perPage})
}

// GetPR handles GET /api/v1/repos/:repo_group/prs/:pr_id (8.2)
func GetPR(c *gin.Context) {
	repoGroup := c.Param("repo_group")
	prID := c.Param("pr_id")

	cfg := config.Current()
	group := config.GetRepoGroupByName(cfg, repoGroup)
	if group == nil {
		c.JSON(http.StatusOK, gin.H{"error": "repo group not found"})
		return
	}

// Try to find PR in DB using index or scan
    var found *models.PRRecord
    prNumber, convErr := strconv.Atoi(prID)
    if convErr == nil {
        data, err := db.GetPRByIndex("", repoGroup, prNumber)
        if err == nil && data != nil {
            var pr models.PRRecord
            if json.Unmarshal(data, &pr) == nil && pr.RepoGroup == repoGroup {
                found = &pr
            }
        }
    }
    if found == nil {
        data, err := db.GetPRByIndex(prID, "", 0)
        if err == nil && data != nil {
            var pr models.PRRecord
            if json.Unmarshal(data, &pr) == nil && pr.RepoGroup == repoGroup {
                found = &pr
            }
        }
    }
    if found == nil {
        db.ForEach(db.BucketPRs, func(key, value []byte) error {
            var pr models.PRRecord
            if json.Unmarshal(value, &pr) != nil {
                return nil
            }
            if pr.RepoGroup == repoGroup && (pr.ID == prID || fmt.Sprintf("%d", pr.PRNumber) == prID) {
                found = &pr
            }
            return nil
        })
    }

	if found != nil {
		c.JSON(http.StatusOK, found)
		return
	}

	// Not in DB, try platform APIs
	platforms := map[string]string{
		"github": group.GitHub,
		"gitlab": group.GitLab,
		"gitea":  group.Gitea,
	}

	ctx := c.Request.Context()
	for plat, repoPath := range platforms {
		if repoPath == "" {
			continue
		}
		client := getClientForGroup(group, plat)
		if client == nil {
			continue
		}
		owner, repo := config.GetOwnerRepoFromGroup(group, plat)

		prNumber, convErr := strconv.Atoi(prID)
		if convErr != nil {
			continue
		}

		pr, err := client.GetPR(ctx, owner, repo, prNumber)
		if err != nil {
			continue
		}
		if pr != nil {
			pr.RepoGroup = repoGroup
			pr.Platform = plat
			c.JSON(http.StatusOK, pr)
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"error": "PR not found"})
}

// ApprovePR handles POST /api/v1/repos/:repo_group/prs/:pr_id/approve (8.2)
func ApprovePR(c *gin.Context) {
	repoGroup := c.Param("repo_group")
	prID := c.Param("pr_id")

	cfg := config.Current()
	group := config.GetRepoGroupByName(cfg, repoGroup)
	if group == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo group not found"})
		return
	}

	// Get platform from PR record in DB
	key := repoGroup + "#" + prID
	data, err := db.Get(db.BucketPRs, key)
	if err != nil || data == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "PR not found"})
		return
	}

	var pr models.PRRecord
	if err := json.Unmarshal(data, &pr); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse PR"})
		return
	}

	client := getClientForGroup(group, pr.Platform)
	if client == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no platform client available"})
		return
	}

	owner, repo := config.GetOwnerRepoFromGroup(group, pr.Platform)
	if owner == "" || repo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot resolve repo"})
		return
	}

	if err := client.ApprovePR(c.Request.Context(), owner, repo, pr.PRNumber); err != nil {
		slog.Error("failed to approve PR", "error", err)
		db.AppendAuditLog("error", "PR approve failed", map[string]interface{}{
			"pr_number":  pr.PRNumber,
			"pr_title":    pr.Title,
			"repo_group":  repoGroup,
			"actor":       c.GetString("username"),
			"platform":    pr.Platform,
			"error":       err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to approve PR"})
		return
	}

	db.AppendAuditLog("info", "PR approved", map[string]interface{}{
		"pr_number":  pr.PRNumber,
		"pr_title":    pr.Title,
		"repo_group":  repoGroup,
		"actor":       c.GetString("username"),
		"platform":    pr.Platform,
	})
	c.JSON(http.StatusOK, gin.H{"message": "PR approved"})
}

// ClosePR handles POST /api/v1/repos/:repo_group/prs/:pr_id/close (8.2)
func ClosePR(c *gin.Context) {
	repoGroup := c.Param("repo_group")
	prID := c.Param("pr_id")

	cfg := config.Current()
	group := config.GetRepoGroupByName(cfg, repoGroup)
	if group == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo group not found"})
		return
	}

	key := repoGroup + "#" + prID
	data, err := db.Get(db.BucketPRs, key)
	if err != nil || data == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "PR not found"})
		return
	}

	var pr models.PRRecord
	if err := json.Unmarshal(data, &pr); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse PR"})
		return
	}

	client := getClientForGroup(group, pr.Platform)
	if client == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no platform client available"})
		return
	}

	owner, repo := config.GetOwnerRepoFromGroup(group, pr.Platform)
	if owner == "" || repo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot resolve repo"})
		return
	}

	if err := client.ClosePR(c.Request.Context(), owner, repo, pr.PRNumber); err != nil {
		slog.Error("failed to close PR", "error", err)
		db.AppendAuditLog("error", "PR close failed", map[string]interface{}{
			"pr_number":  pr.PRNumber,
			"pr_title":    pr.Title,
			"repo_group":  repoGroup,
			"actor":       c.GetString("username"),
			"platform":    pr.Platform,
			"error":       err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to close PR"})
		return
	}

	db.AppendAuditLog("info", "PR closed", map[string]interface{}{
		"pr_number":  pr.PRNumber,
		"pr_title":    pr.Title,
		"repo_group":  repoGroup,
		"actor":       c.GetString("username"),
		"platform":    pr.Platform,
	})
	c.JSON(http.StatusOK, gin.H{"message": "PR closed"})
}

// ReopenPR handles POST /api/v1/repos/:repo_group/prs/:pr_id/reopen (8.2)
func ReopenPR(c *gin.Context) {
	repoGroup := c.Param("repo_group")
	prID := c.Param("pr_id")

	cfg := config.Current()
	group := config.GetRepoGroupByName(cfg, repoGroup)
	if group == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo group not found"})
		return
	}

	key := repoGroup + "#" + prID
	data, err := db.Get(db.BucketPRs, key)
	if err != nil || data == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "PR not found"})
		return
	}

	var pr models.PRRecord
	if err := json.Unmarshal(data, &pr); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse PR"})
		return
	}

	client := getClientForGroup(group, pr.Platform)
	if client == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no platform client available"})
		return
	}

	owner, repo := config.GetOwnerRepoFromGroup(group, pr.Platform)
	if owner == "" || repo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot resolve repo"})
		return
	}

	if err := client.ReopenPR(c.Request.Context(), owner, repo, pr.PRNumber); err != nil {
		slog.Error("failed to reopen PR", "error", err)
		db.AppendAuditLog("error", "PR reopen failed", map[string]interface{}{
			"pr_number":  pr.PRNumber,
			"pr_title":    pr.Title,
			"repo_group":  repoGroup,
			"actor":       c.GetString("username"),
			"platform":    pr.Platform,
			"error":       err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reopen PR"})
		return
	}

	db.AppendAuditLog("info", "PR reopened", map[string]interface{}{
		"pr_number":  pr.PRNumber,
		"pr_title":    pr.Title,
		"repo_group":  repoGroup,
		"actor":       c.GetString("username"),
		"platform":    pr.Platform,
	})
	c.JSON(http.StatusOK, gin.H{"message": "PR reopened"})
}

// MarkSpam handles POST /api/v1/repos/:repo_group/prs/:pr_id/spam (8.2)
func MarkSpam(c *gin.Context) {
	repoGroup := c.Param("repo_group")
	prID := c.Param("pr_id")

	cfg := config.Current()
	group := config.GetRepoGroupByName(cfg, repoGroup)
	if group == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo group not found"})
		return
	}

	key := repoGroup + "#" + prID
	data, err := db.Get(db.BucketPRs, key)
	if err != nil || data == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "PR not found"})
		return
	}

	var pr models.PRRecord
	if err := json.Unmarshal(data, &pr); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse PR"})
		return
	}

	client := getClientForGroup(group, pr.Platform)
	if client == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no platform client available"})
		return
	}

	owner, repo := config.GetOwnerRepoFromGroup(group, pr.Platform)
	if owner == "" || repo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot resolve repo"})
		return
	}

	// Close the PR as spam
	if err := client.ClosePR(c.Request.Context(), owner, repo, pr.PRNumber); err != nil {
		slog.Error("failed to mark PR as spam", "error", err)
		db.AppendAuditLog("error", "PR spam marking failed", map[string]interface{}{
			"pr_number":  pr.PRNumber,
			"pr_title":    pr.Title,
			"repo_group":  repoGroup,
			"actor":       c.GetString("username"),
			"platform":    pr.Platform,
			"error":       err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to mark as spam"})
		return
	}

	// Update PR record to mark as spam
	pr.State = "spam"
	pr.SpamFlag = true
updated, _ := json.Marshal(pr)
    db.PutPRWithIndex(key, updated, pr.ID, repoGroup, pr.PRNumber)

	db.AppendAuditLog("warn", "PR marked as spam", map[string]interface{}{
		"pr_number":  pr.PRNumber,
		"pr_title":    pr.Title,
		"repo_group":  repoGroup,
		"actor":       c.GetString("username"),
		"platform":    pr.Platform,
	})

	// TODO: Send notifications to admins

	c.JSON(http.StatusOK, gin.H{"message": "PR marked as spam"})
}

// CommentPR handles POST /api/v1/repos/:repo_group/prs/:pr_id/comment
func CommentPR(c *gin.Context) {
	repoGroup := c.Param("repo_group")
	prID := c.Param("pr_id")

	var req struct {
		Body string `json:"body" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body is required"})
		return
	}

	cfg := config.Current()
	group := config.GetRepoGroupByName(cfg, repoGroup)
	if group == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo group not found"})
		return
	}

	key := repoGroup + "#" + prID
	data, err := db.Get(db.BucketPRs, key)
	if err != nil || data == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "PR not found"})
		return
	}

	var pr models.PRRecord
	if err := json.Unmarshal(data, &pr); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse PR"})
		return
	}

	client := getClientForGroup(group, pr.Platform)
	if client == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no platform client available"})
		return
	}

	owner, repo := config.GetOwnerRepoFromGroup(group, pr.Platform)
	if owner == "" || repo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot resolve repo"})
		return
	}

	if err := client.CommentPR(c.Request.Context(), owner, repo, pr.PRNumber, req.Body); err != nil {
		slog.Error("failed to comment on PR", "error", err)
		db.AppendAuditLog("error", "PR comment failed", map[string]interface{}{
			"pr_number":  pr.PRNumber,
			"pr_title":    pr.Title,
			"repo_group":  repoGroup,
			"actor":       c.GetString("username"),
			"platform":    pr.Platform,
			"error":       err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to comment on PR"})
		return
	}

	db.AppendAuditLog("info", "PR commented", map[string]interface{}{
		"pr_number":  pr.PRNumber,
		"pr_title":    pr.Title,
		"repo_group":  repoGroup,
		"actor":       c.GetString("username"),
		"platform":    pr.Platform,
		"comment":     req.Body[:min(len(req.Body), 50)],
	})
	c.JSON(http.StatusOK, gin.H{"message": "comment added"})
}

// BatchApprovePR handles POST /api/v1/repos/:repo_group/prs/batch/approve
func BatchApprovePR(c *gin.Context) {
	repoGroup := c.Param("repo_group")

	var req struct {
		PRIDs []string `json:"pr_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pr_ids is required"})
		return
	}

	cfg := config.Current()
	group := config.GetRepoGroupByName(cfg, repoGroup)
	if group == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo group not found"})
		return
	}

	results := make(map[string]string)
	for _, prID := range req.PRIDs {
		key := repoGroup + "#" + prID
		data, err := db.Get(db.BucketPRs, key)
		if err != nil || data == nil {
			results[prID] = "not found"
			continue
		}

		var pr models.PRRecord
		if err := json.Unmarshal(data, &pr); err != nil {
			results[prID] = "parse error"
			continue
		}

		client := getClientForGroup(group, pr.Platform)
		if client == nil {
			results[prID] = "no client"
			continue
		}

		owner, repo := config.GetOwnerRepoFromGroup(group, pr.Platform)
		if owner == "" || repo == "" {
			results[prID] = "cannot resolve repo"
			continue
		}

		if err := client.ApprovePR(c.Request.Context(), owner, repo, pr.PRNumber); err != nil {
			results[prID] = "failed: " + err.Error()
			slog.Warn("batch approve failed", "pr_id", prID, "error", err)
		} else {
			results[prID] = "success"
			db.AppendAuditLog("info", "PR approved (batch)", map[string]interface{}{
				"pr_number":  pr.PRNumber,
				"pr_title":    pr.Title,
				"repo_group":  repoGroup,
				"actor":       c.GetString("username"),
				"platform":    pr.Platform,
				"batch":       true,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{"results": results})
}

// BatchClosePR handles POST /api/v1/repos/:repo_group/prs/batch/close
func BatchClosePR(c *gin.Context) {
	repoGroup := c.Param("repo_group")

	var req struct {
		PRIDs []string `json:"pr_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pr_ids is required"})
		return
	}

	cfg := config.Current()
	group := config.GetRepoGroupByName(cfg, repoGroup)
	if group == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo group not found"})
		return
	}

	results := make(map[string]string)
	for _, prID := range req.PRIDs {
		key := repoGroup + "#" + prID
		data, err := db.Get(db.BucketPRs, key)
		if err != nil || data == nil {
			results[prID] = "not found"
			continue
		}

		var pr models.PRRecord
		if err := json.Unmarshal(data, &pr); err != nil {
			results[prID] = "parse error"
			continue
		}

		client := getClientForGroup(group, pr.Platform)
		if client == nil {
			results[prID] = "no client"
			continue
		}

		owner, repo := config.GetOwnerRepoFromGroup(group, pr.Platform)
		if owner == "" || repo == "" {
			results[prID] = "cannot resolve repo"
			continue
		}

		if err := client.ClosePR(c.Request.Context(), owner, repo, pr.PRNumber); err != nil {
			results[prID] = "failed: " + err.Error()
			slog.Warn("batch close failed", "pr_id", prID, "error", err)
		} else {
			results[prID] = "success"
			db.AppendAuditLog("info", "PR closed (batch)", map[string]interface{}{
				"pr_number":  pr.PRNumber,
				"pr_title":    pr.Title,
				"repo_group":  repoGroup,
				"actor":       c.GetString("username"),
				"platform":    pr.Platform,
				"batch":       true,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{"results": results})
}

// BatchLabelPR handles POST /api/v1/repos/:repo_group/prs/batch/label
func BatchLabelPR(c *gin.Context) {
	repoGroup := c.Param("repo_group")

	var req struct {
		PRIDs []string `json:"pr_ids" binding:"required"`
		Label  string   `json:"label" binding:"required"`
		Color  string   `json:"color"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pr_ids and label are required"})
		return
	}

	cfg := config.Current()
	group := config.GetRepoGroupByName(cfg, repoGroup)
	if group == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo group not found"})
		return
	}

	results := make(map[string]string)
	for _, prID := range req.PRIDs {
		key := repoGroup + "#" + prID
		data, err := db.Get(db.BucketPRs, key)
		if err != nil || data == nil {
			results[prID] = "not found"
			continue
		}

		var pr models.PRRecord
		if err := json.Unmarshal(data, &pr); err != nil {
			results[prID] = "parse error"
			continue
		}

		client := getClientForGroup(group, pr.Platform)
		if client == nil {
			results[prID] = "no client"
			continue
		}

		owner, repo := config.GetOwnerRepoFromGroup(group, pr.Platform)
		if owner == "" || repo == "" {
			results[prID] = "cannot resolve repo"
			continue
		}

		if err := client.AddLabel(c.Request.Context(), owner, repo, pr.PRNumber, req.Label, req.Color); err != nil {
			results[prID] = "failed: " + err.Error()
			slog.Warn("batch label failed", "pr_id", prID, "error", err)
		} else {
			results[prID] = "success"
			db.AppendAuditLog("info", "PR labeled (batch)", map[string]interface{}{
				"pr_number":  pr.PRNumber,
				"pr_title":    pr.Title,
				"repo_group":  repoGroup,
				"actor":       c.GetString("username"),
				"platform":    pr.Platform,
				"label":       req.Label,
				"batch":       true,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{"results": results})
}

// GetLogs handles GET /api/v1/logs (8.2)
func GetLogs(c *gin.Context) {
	level := c.Query("level")

	logs := make([]models.AuditLog, 0)
	err := db.ForEach(db.BucketLogs, func(key, value []byte) error {
		var log models.AuditLog
		if err := json.Unmarshal(value, &log); err != nil {
			return nil
		}
		if level != "" && log.Level != level {
			return nil
		}
		logs = append(logs, log)
		return nil
	})
	if err != nil {
		c.JSON(http.StatusOK, logs)
		return
	}

	c.JSON(http.StatusOK, logs)
}

// getClientForGroup returns the platform client for a repo group
func getClientForGroup(group *models.RepoGroup, platform string) platforms.PlatformClient {
	if platform == "" {
		platform = "github"
	}
	if clients == nil {
		return nil
	}
	return clients[platforms.PlatformType(platform)]
}
