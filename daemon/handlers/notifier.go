package handlers

import (
	"context"
	"log/slog"
	"time"

	"asika/common/models"
	"asika/common/notifier"
	"asika/common/platforms"
)

// notifyFunc is an optional external notification sender (set by core).
var notifyFunc func(title, body string)

// SetNotifyFunc sets the external notification function.
func SetNotifyFunc(fn func(title, body string)) {
	notifyFunc = fn
}

var globalNotifiers []notifier.Notifier

// InitNotifiers initializes the notification senders for handlers.
func InitNotifiers(cfg *models.Config, clients map[platforms.PlatformType]platforms.PlatformClient) {
	notifiers := make([]notifier.Notifier, 0, len(cfg.Notify))
	for _, nc := range cfg.Notify {
		n := createNotifierFromNotifyConfig(nc)
		if n != nil {
			notifiers = append(notifiers, n)
		}
	}
	notifier.WirePlatformNotifiers(notifiers, clients)
	globalNotifiers = notifiers
}

func sendNotifications(title, body string) {
	if notifyFunc != nil {
		notifyFunc(title, body)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, n := range globalNotifiers {
		if err := n.Send(ctx, title, body); err != nil {
			slog.Warn("notification send failed", "type", n.Type(), "error", err)
		}
	}
}

func createNotifierFromNotifyConfig(nc models.NotifyConfig) notifier.Notifier {
	switch nc.Type {
	case "smtp":
		return notifier.NewSMTPNotifier(nc.Config)
	case "wecom":
		return notifier.NewWeComNotifier(nc.Config)
	case "github_at":
		return notifier.NewGitHubAtNotifier(nc.Config)
	case "gitlab_at":
		return notifier.NewGitLabAtNotifier(nc.Config)
	case "gitea_at":
		return notifier.NewGiteaAtNotifier(nc.Config)
	case "telegram":
		return notifier.NewTelegramNotifier(nc.Config)
	case "feishu":
		return notifier.NewFeishuNotifier(nc.Config)
	case "discord":
		return notifier.NewDiscordNotifier(nc.Config)
	}
	return nil
}
