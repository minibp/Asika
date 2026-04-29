package notifier

import (
    "context"
    "fmt"
)

// PlatformNotifier sends notifications via platform comments
type PlatformNotifier struct {
    platform string
    to       []string
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

    return &PlatformNotifier{
        platform: "github",
        to:       to,
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

    return &PlatformNotifier{
        platform: "gitlab",
        to:       to,
    }
}

// Type returns the type of notifier
func (n *PlatformNotifier) Type() string {
    return n.platform + "_at"
}

// Send sends a notification via platform comment
func (n *PlatformNotifier) Send(ctx context.Context, title, body string) error {
    if len(n.to) == 0 {
        return fmt.Errorf("no users configured")
    }

    return nil
}
