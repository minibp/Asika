package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"log/slog"

	"asika/common/config"
	"asika/common/db"
	"asika/common/models"
	"asika/common/platforms"
)

// clients is a package-level variable to access platform clients
var clients map[platforms.PlatformType]platforms.PlatformClient

// InitClients initializes the platform clients for handlers
func InitClients(c map[platforms.PlatformType]platforms.PlatformClient) {
	clients = c
}

// ListPRs handles GET /api/v1/repos/:repo_group/prs (8.2)
func ListPRs(c *gin.Context) {
	repoGroup := c.Param("repo_group")
	state := c.Query("state")
	platform := c.Query("platform")

	cfg := config.Current()
	group := config.GetRepoGroupByName(cfg, repoGroup)
	if group == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo group not found"})
		return
	}

	// Get platform client
	client := getClientForGroup(group, platform)
	if client == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no platform configured"})
		return
	}

	owner, repo := config.GetOwnerRepoFromGroup(group, platform)
	if owner == "" || repo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot resolve repo for platform"})
		return
	}

	prs, err := client.ListPRs(c.Request.Context(), owner, repo, state)
	if err != nil {
		slog.Error("failed to list PRs", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list PRs"})
		return
	}

	// Convert to PRRecord format
	var records []models.PRRecord
	for _, pr := range prs {
		records = append(records, models.PRRecord{
			PRNumber: pr.PRNumber,
			Title:    pr.Title,
			Author:   pr.Author,
			State:    pr.State,
			Labels:   pr.Labels,
			Platform: platform,
		})
	}

	c.JSON(http.StatusOK, records)
}

// GetPR handles GET /api/v1/repos/:repo_group/prs/:pr_id (8.2)
func GetPR(c *gin.Context) {
	repoGroup := c.Param("repo_group")
	prID := c.Param("pr_id")

	cfg := config.Current()
	group := config.GetRepoGroupByName(cfg, repoGroup)
	if group == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo group not found"})
		return
	}

	// Try to find PR in DB first
	key := repoGroup + "#" + prID
	data, err := db.Get(db.BucketPRs, key)
	if err == nil {
		var pr models.PRRecord
		if json.Unmarshal(data, &pr) == nil {
			c.JSON(http.StatusOK, pr)
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "PR not found"})
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
	if err != nil {
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to approve PR"})
		return
	}

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
	if err != nil {
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to close PR"})
		return
	}

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
	if err != nil {
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reopen PR"})
		return
	}

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
	if err != nil {
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to mark as spam"})
		return
	}

	// Update PR record to mark as spam
	pr.State = "spam"
	pr.SpamFlag = true
	updated, _ := json.Marshal(pr)
	db.Put(db.BucketPRs, key, updated)

	// TODO: Send notifications to admins

	c.JSON(http.StatusOK, gin.H{"message": "PR marked as spam"})
}

// GetLogs handles GET /api/v1/logs (8.2)
func GetLogs(c *gin.Context) {
	level := c.Query("level")

	var logs []models.AuditLog
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read logs"})
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
