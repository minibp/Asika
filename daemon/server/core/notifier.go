package core

import (
	"context"
	"log/slog"
	"time"

	"asika/common/models"
	"asika/common/notifier"
	"asika/common/platforms"
	"asika/daemon/handlers"
)

var globalNotifiers []notifier.Notifier

// InitNotifiers creates and wires all configured notifiers with platform clients.
func InitNotifiers(cfg *models.Config, clients map[platforms.PlatformType]platforms.PlatformClient) {
	notifiers := make([]notifier.Notifier, 0, len(cfg.Notify))
	for _, nc := range cfg.Notify {
		n := createNotifierFromConfig(nc)
		if n != nil {
			notifiers = append(notifiers, n)
		}
	}
	notifier.WirePlatformNotifiers(notifiers, clients)
	globalNotifiers = notifiers
	handlers.SetNotifyFunc(SendNotificationSync)
	slog.Info("notifiers initialized", "count", len(globalNotifiers))
}

// SendNotification sends a notification through all configured notifiers.
func SendNotification(ctx context.Context, title, body string) {
	for _, n := range globalNotifiers {
		sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		if err := n.Send(sendCtx, title, body); err != nil {
			slog.Warn("notification send failed", "type", n.Type(), "error", err)
		}
		cancel()
	}
}

// SendNotificationSync sends notifications synchronously with a timeout.
func SendNotificationSync(title, body string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	SendNotification(ctx, title, body)
}

func createNotifierFromConfig(nc models.NotifyConfig) notifier.Notifier {
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
