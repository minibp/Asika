package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"asika/common/events"
	"asika/common/models"
)

// WebhookHandler handles incoming webhooks from all platforms
func WebhookHandler(c *gin.Context) {
	repoGroup := c.Param("repo_group")
	platform := c.Param("platform")

	// Read body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}

	// Get signature from header
	signature := getSignature(c, platform)

	// Get platform client to verify signature
	// TODO: get client from server context or config
	// For now, just parse the event

	var eventType string
	var pr *models.PRRecord

	switch strings.ToLower(platform) {
	case "github":
		eventType, pr = parseGitHubWebhook(c, body, signature)
	case "gitlab":
		eventType, pr = parseGitLabWebhook(c, body, signature)
	case "gitea":
		eventType, pr = parseGiteaWebhook(c, body, signature)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown platform"})
		return
	}

	if eventType == "" {
		c.JSON(http.StatusOK, gin.H{"status": "ignored"})
		return
	}

	// Publish event to bus
	events.PublishPR(events.EventType(eventType), repoGroup, platform, pr, nil)

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func getSignature(c *gin.Context, platform string) string {
	switch strings.ToLower(platform) {
	case "github":
		return c.GetHeader("X-Hub-Signature-256")
	case "gitlab":
		return c.GetHeader("X-Gitlab-Token")
	case "gitea":
		return c.GetHeader("X-Gitea-Signature")
	}
	return ""
}

func parseGitHubWebhook(c *gin.Context, body []byte, signature string) (string, *models.PRRecord) {
	event := c.GetHeader("X-GitHub-Event")
	if event == "" {
		return "", nil
	}

	// TODO: verify signature with github client

	// Parse event
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", nil
	}

	switch event {
	case "pull_request":
		action, _ := payload["action"].(string)
		prData, ok := payload["pull_request"].(map[string]interface{})
		if !ok {
			return "", nil
		}
		pr := parseGitHubPR(prData)
		return "pr_" + action, pr
	case "pull_request_review", "pull_request_review_comment":
		// TODO: check if review is approval
		return "pr_approved", nil
	case "label":
		return "pr_labeled", nil
	}

	return "", nil
}

func parseGitHubPR(data map[string]interface{}) *models.PRRecord {
	pr := &models.PRRecord{}

	if id, ok := data["id"].(float64); ok {
		pr.ID = fmt.Sprintf("%.0f", id)
	}
	if number, ok := data["number"].(float64); ok {
		pr.PRNumber = int(number)
	}
	if title, ok := data["title"].(string); ok {
		pr.Title = title
	}
	if state, ok := data["state"].(string); ok {
		pr.State = state
	}
	if mergeCommit, ok := data["merge_commit_sha"].(string); ok {
		pr.MergeCommitSHA = mergeCommit
	}

	// Get author
	if user, ok := data["user"].(map[string]interface{}); ok {
		if login, ok := user["login"].(string); ok {
			pr.Author = login
		}
	}

	return pr
}

func parseGitLabWebhook(c *gin.Context, body []byte, token string) (string, *models.PRRecord) {
	event := c.GetHeader("X-Gitlab-Event")
	if event == "" {
		return "", nil
	}

	// TODO: verify token

	// Parse event
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", nil
	}

	switch event {
	case "Merge Request Hook":
		// Get action from object_attributes
		attrs, ok := payload["object_attributes"].(map[string]interface{})
		if !ok {
			return "", nil
		}
		pr := parseGitLabMR(attrs)
		action, _ := attrs["action"].(string)
		return "pr_" + action, pr
	}

	return "", nil
}

func parseGitLabMR(data map[string]interface{}) *models.PRRecord {
	pr := &models.PRRecord{}

	if id, ok := data["id"].(float64); ok {
		pr.ID = fmt.Sprintf("%.0f", id)
	}
	if iid, ok := data["iid"].(float64); ok {
		pr.PRNumber = int(iid)
	}
	if title, ok := data["title"].(string); ok {
		pr.Title = title
	}
	if state, ok := data["state"].(string); ok {
		pr.State = state
	}

	// Get author
	if author, ok := data["author"].(map[string]interface{}); ok {
		if username, ok := author["username"].(string); ok {
			pr.Author = username
		}
	}

	return pr
}

func parseGiteaWebhook(c *gin.Context, body []byte, signature string) (string, *models.PRRecord) {
	event := c.GetHeader("X-Gitea-Event")
	if event == "" {
		return "", nil
	}

	// TODO: verify signature

	// Parse event
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", nil
	}

	switch event {
	case "pull_request":
		action, _ := payload["action"].(string)
		prData, ok := payload["pull_request"].(map[string]interface{})
		if !ok {
			return "", nil
		}
		pr := parseGiteaPR(prData)
		return "pr_" + action, pr
	}

	return "", nil
}

func parseGiteaPR(data map[string]interface{}) *models.PRRecord {
	pr := &models.PRRecord{}

	if id, ok := data["id"].(float64); ok {
		pr.ID = fmt.Sprintf("%.0f", id)
	}
	if number, ok := data["number"].(float64); ok {
		pr.PRNumber = int(number)
	}
	if title, ok := data["title"].(string); ok {
		pr.Title = title
	}
	if state, ok := data["state"].(string); ok {
		pr.State = state
	}

	// Get author
	if user, ok := data["user"].(map[string]interface{}); ok {
		if username, ok := user["username"].(string); ok {
			pr.Author = username
		}
	}

	return pr
}
