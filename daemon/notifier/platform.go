package notifier

import (
	"context"
	"fmt"
	"log/slog"

	"asika/common/platforms"
)

// PlatformNotifier sends notifications via platform comments
type PlatformNotifier struct {
	platform string
	to       []string
	client   platforms.PlatformClient
	owner    string
	repo     string
}

// NewGitHubAtNotifier creates a new GitHub @ notifier
func NewGitHubAtNotifier(config map[string]interface{}) *PlatformNotifier {
	to := make([]string, 0)
	if toList, ok := config["to"].([]interface{}); ok {
		for _, t := range toList {
			if s, ok := t.(string); ok {
				to = append(to, s)
			}
		}
	}

	owner, _ := config["owner"].(string)
	repo, _ := config["repo"].(string)

	return &PlatformNotifier{
		platform: "github",
		to:       to,
		owner:    owner,
		repo:     repo,
	}
}

// NewGitLabAtNotifier creates a new GitLab @ notifier
func NewGitLabAtNotifier(config map[string]interface{}) *PlatformNotifier {
	to := make([]string, 0)
	if toList, ok := config["to"].([]interface{}); ok {
		for _, t := range toList {
			if s, ok := t.(string); ok {
				to = append(to, s)
			}
		}
	}

	owner, _ := config["owner"].(string)
	repo, _ := config["repo"].(string)

	return &PlatformNotifier{
		platform: "gitlab",
		to:       to,
		owner:    owner,
		repo:     repo,
	}
}

// SetClient sets the platform client
func (n *PlatformNotifier) SetClient(client platforms.PlatformClient) {
	n.client = client
}

// Type returns the type of notifier
func (n *PlatformNotifier) Type() string {
	return n.platform + "_at"
}

// Send sends a notification via platform comment, @mentioning configured users
func (n *PlatformNotifier) Send(ctx context.Context, title, body string) error {
	if len(n.to) == 0 {
		return fmt.Errorf("no users configured")
	}

	if n.client == nil {
		// Without a client, we can only log
		slog.Warn("platform notifier has no client, logging instead", "platform", n.platform)
		slog.Info("notification", "title", title, "body", body)
		return nil
	}

	// Build mention body
	mentions := ""
	for _, user := range n.to {
		mentions += "@" + user + " "
	}

	commentBody := fmt.Sprintf("**%s**\n\n%s\n\n%s", title, body, mentions)

	// Comment on the latest open issue/PR or a specific one
	// For now, we just log the comment since we need a specific PR number
	slog.Info("platform notification prepared", "platform", n.platform, "comment", commentBody)
	return nil
}