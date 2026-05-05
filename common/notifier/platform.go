package notifier

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

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

// NewGiteaAtNotifier creates a new Gitea @ notifier
func NewGiteaAtNotifier(config map[string]interface{}) *PlatformNotifier {
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
		platform: "gitea",
		to:       to,
		owner:    owner,
		repo:     repo,
	}
}

// SetClient sets the platform client
func (n *PlatformNotifier) SetClient(client platforms.PlatformClient) {
	n.client = client
}

// WirePlatformNotifiers injects platform clients into platform_at notifiers based on notifier type.
func WirePlatformNotifiers(notifiers []Notifier, clients map[platforms.PlatformType]platforms.PlatformClient) {
	for _, n := range notifiers {
		pn, ok := n.(*PlatformNotifier)
		if !ok {
			continue
		}
		var pt platforms.PlatformType
		switch pn.platform {
		case "github":
			pt = platforms.PlatformGitHub
		case "gitlab":
			pt = platforms.PlatformGitLab
		case "gitea":
			pt = platforms.PlatformGitea
		}
		if client, exists := clients[pt]; exists {
			pn.SetClient(client)
		}
	}
}

// Type returns the type of notifier
func (n *PlatformNotifier) Type() string {
	return n.platform + "_at"
}

// Send sends a notification via platform comment, @mentioning configured users
func (n *PlatformNotifier) Send(ctx context.Context, title, body string) error {
	if n.client == nil {
		return fmt.Errorf("platform notifier has no client")
	}

	if len(n.to) == 0 {
		return fmt.Errorf("no users configured")
	}

	// Build mention body
	mentions := ""
	for _, user := range n.to {
		mentions += "@" + user + " "
	}

	commentBody := fmt.Sprintf("**%s**\n\n%s\n\n%s", title, body, strings.TrimSpace(mentions))

	// Post comment on the repo's issue tracker or a general PR
	// Try to find an open PR to comment on
	prs, err := n.client.ListPRs(ctx, n.owner, n.repo, "open")
	if err != nil {
		slog.Warn("failed to list PRs for notification", "error", err)
		// Fall back to just logging
		slog.Info("platform notification", "platform", n.platform, "owner", n.owner, "repo", n.repo, "comment", commentBody)
		return nil
	}

	if len(prs) > 0 {
		// Comment on the first open PR
		if err := n.client.CommentPR(ctx, n.owner, n.repo, prs[0].PRNumber, commentBody); err != nil {
			slog.Error("failed to post platform notification comment", "error", err)
			return err
		}
		slog.Info("platform notification comment posted", "platform", n.platform, "pr", prs[0].PRNumber)
	} else {
		// No open PRs, just log
		slog.Info("platform notification (no open PRs)", "platform", n.platform, "comment", commentBody)
	}

	return nil
}
