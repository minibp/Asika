package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"asika/common/config"
	"asika/common/events"
	"asika/common/models"
	"asika/common/platforms"
)

// WebhookHandler handles POST /webhook/:repo_group/:platform
func WebhookHandler(c *gin.Context) {
	repoGroup := c.Param("repo_group")
	platform := c.Param("platform")

	cfg := config.Current()
	if cfg == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "config not loaded"})
		return
	}

	group := config.GetRepoGroupByName(cfg, repoGroup)
	if group == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo group not found"})
		return
	}

	pt := platforms.PlatformType(platform)
	client := clients[pt]
	if client == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported platform: " + platform})
		return
	}

	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}

	if !verifyWebhookSignature(platform, client, body, c) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid webhook signature"})
		return
	}

	eventType, pr, err := parseWebhookEvent(platform, body, repoGroup)
	if err != nil {
		slog.Error("failed to parse webhook", "error", err, "platform", platform)
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse webhook: " + err.Error()})
		return
	}

	if eventType != "" && pr != nil {
		events.PublishPR(events.EventType(eventType), repoGroup, platform, pr, nil)
		slog.Info("webhook event published", "type", eventType, "repo_group", repoGroup, "platform", platform, "pr", pr.PRNumber)
	}

	c.JSON(http.StatusOK, gin.H{"message": "webhook received", "event": eventType})
}

// verifyWebhookSignature verifies the webhook signature based on platform
func verifyWebhookSignature(platform string, client platforms.PlatformClient, body []byte, c *gin.Context) bool {
	switch platform {
	case "github":
		sig := c.GetHeader("X-Hub-Signature-256")
		if sig == "" {
			sig = c.GetHeader("X-Hub-Signature")
		}
		return client.VerifyWebhookSignature(body, sig)
	case "gitlab":
		token := c.GetHeader("X-Gitlab-Token")
		return client.VerifyWebhookSignature(body, token)
	case "gitea":
		sig := c.GetHeader("X-Gitea-Signature")
		return client.VerifyWebhookSignature(body, sig)
	}
	return false
}

// parseWebhookEvent parses the webhook body and returns event type and PR record
func parseWebhookEvent(platform string, body []byte, repoGroup string) (string, *models.PRRecord, error) {
	switch platform {
	case "github":
		return parseGitHubWebhook(body, repoGroup)
	case "gitlab":
		return parseGitLabWebhook(body, repoGroup)
	case "gitea":
		return parseGiteaWebhook(body, repoGroup)
	}
	return "", nil, nil
}

// parseGitHubWebhook parses GitHub webhook payload
func parseGitHubWebhook(body []byte, repoGroup string) (string, *models.PRRecord, error) {
	var payload struct {
		Action      string `json:"action"`
		PullRequest struct {
			Number  int    `json:"number"`
			Title   string `json:"title"`
			State   string `json:"state"`
			Merged  bool   `json:"merged"`
			HTMLURL string `json:"html_url"`
			User    struct {
				Login string `json:"login"`
			} `json:"user"`
			Head struct {
				Sha string `json:"sha"`
			} `json:"head"`
			Base struct {
				Ref string `json:"ref"`
			} `json:"base"`
			Labels []struct {
				Name string `json:"name"`
			} `json:"labels"`
		} `json:"pull_request"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return "", nil, err
	}

	pr := &models.PRRecord{
		Platform:  "github",
		PRNumber:  payload.PullRequest.Number,
		Title:     payload.PullRequest.Title,
		Author:    payload.PullRequest.User.Login,
		State:     payload.PullRequest.State,
		RepoGroup: repoGroup,
	}

	if payload.PullRequest.Merged {
		pr.State = "merged"
	}

	for _, lbl := range payload.PullRequest.Labels {
		pr.Labels = append(pr.Labels, lbl.Name)
	}

	eventType := ""
	switch payload.Action {
	case "opened":
		eventType = string(events.EventPROpened)
	case "closed":
		if pr.State == "merged" {
			eventType = string(events.EventPRMerged)
		} else {
			eventType = string(events.EventPRClosed)
		}
	case "reopened":
		eventType = string(events.EventPRReopened)
	case "labeled":
		eventType = string(events.EventPRLabeled)
	case "approved":
		eventType = string(events.EventPRApproved)
	}

	return eventType, pr, nil
}

// parseGitLabWebhook parses GitLab webhook payload
func parseGitLabWebhook(body []byte, repoGroup string) (string, *models.PRRecord, error) {
	var payload struct {
		ObjectKind string `json:"object_kind"`
		EventName  string `json:"event_name"`
		ObjectAttributes struct {
			IID    int    `json:"iid"`
			Title  string `json:"title"`
			State  string `json:"state"`
			Action string `json:"action"`
			Merged bool   `json:"merged"`
		} `json:"object_attributes"`
		User struct {
			Username string `json:"username"`
		} `json:"user"`
		Labels []struct {
			Title string `json:"title"`
		} `json:"labels"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return "", nil, err
	}

	pr := &models.PRRecord{
		Platform:  "gitlab",
		PRNumber:  payload.ObjectAttributes.IID,
		Title:     payload.ObjectAttributes.Title,
		Author:    payload.User.Username,
		State:     payload.ObjectAttributes.State,
		RepoGroup: repoGroup,
	}

	if payload.ObjectAttributes.Merged {
		pr.State = "merged"
	}

	for _, lbl := range payload.Labels {
		pr.Labels = append(pr.Labels, lbl.Title)
	}

	eventType := ""
	switch payload.ObjectAttributes.State {
	case "opened", "reopened":
		if payload.ObjectAttributes.State == "reopened" {
			eventType = string(events.EventPRReopened)
		} else {
			eventType = string(events.EventPROpened)
		}
	case "closed":
		eventType = string(events.EventPRClosed)
	case "merged":
		eventType = string(events.EventPRMerged)
	}

	return eventType, pr, nil
}

// parseGiteaWebhook parses Gitea/Forgejo webhook payload
func parseGiteaWebhook(body []byte, repoGroup string) (string, *models.PRRecord, error) {
	var payload struct {
		Action     string `json:"action"`
		Number     int    `json:"number"`
		PullRequest struct {
			Title  string `json:"title"`
			State  string `json:"state"`
			Merged bool   `json:"merged"`
			Poster struct {
				Login string `json:"login"`
			} `json:"poster"`
		} `json:"pull_request"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
		Sender struct {
			Login string `json:"login"`
		} `json:"sender"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return "", nil, err
	}

	author := payload.PullRequest.Poster.Login
	if author == "" {
		author = payload.Sender.Login
	}

	pr := &models.PRRecord{
		Platform:  "gitea",
		PRNumber:  payload.Number,
		Title:     payload.PullRequest.Title,
		Author:    author,
		State:     payload.PullRequest.State,
		RepoGroup: repoGroup,
	}

	if payload.PullRequest.Merged {
		pr.State = "merged"
	}

	eventType := ""
	switch payload.Action {
	case "opened":
		eventType = string(events.EventPROpened)
	case "closed":
		if pr.State == "merged" {
			eventType = string(events.EventPRMerged)
		} else {
			eventType = string(events.EventPRClosed)
		}
	case "reopened":
		eventType = string(events.EventPRReopened)
	case "labeled":
		eventType = string(events.EventPRLabeled)
	case "approved":
		eventType = string(events.EventPRApproved)
	}

	return eventType, pr, nil
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
	idx := strings.LastIndex(repoPath, "/")
	if idx < 0 {
		return "", repoPath
	}
	return repoPath[:idx], repoPath[idx+1:]
}
