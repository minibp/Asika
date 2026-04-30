package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"log/slog"

	"asika/common/config"
	"asika/common/models"
)

// WebhookHandler handles POST /webhook/:repo_group/:platform (8. Webhook)
func WebhookHandler(c *gin.Context) {
	repoGroup := c.Param("repo_group")
	platform := c.Param("platform")

	// Get config
	cfg := config.Current()
	if cfg == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "config not loaded"})
		return
	}

	// Get repo group
	group := config.GetRepoGroupByName(cfg, repoGroup)
	if group == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo group not found"})
		return
	}

	// Read body
	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}

	// Verify signature (simplified - would use platform client)
	slog.Info("webhook received",
		"repo_group", repoGroup,
		"platform", platform,
		"body_size", len(body),
	)

	// In real implementation:
	// 1. Verify webhook signature using platform client
	// 2. Parse event based on platform
	// 3. Convert to internal event
	// 4. Publish to event bus

	c.JSON(http.StatusOK, gin.H{
		"message": "webhook received",
		"repo_group": repoGroup,
		"platform":  platform,
	})
}

// GetOwnerRepoFromGroup is a helper to get owner/repo for a platform
func GetOwnerRepoFromGroup(group *models.RepoGroup, platform string) (owner, repo string) {
	var repoPath string
	switch platform {
	case "github":
		repoPath = group.GitHub
	case "gitlab":
		repoPath = group.GitLab
	case "gitea":
		repoPath = group.Gitea
	}
	if repoPath == "" {
		return "", ""
	}
	// Simplified - would parse owner/repo
	return "owner", repoPath
}
