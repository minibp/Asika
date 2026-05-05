package core

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"asika/common/models"
	"asika/common/notifier"
	"asika/common/version"
)

func startUpdateCheck(cfg *models.Config) {
	if !cfg.Updates.Check {
		return
	}

	interval := parseDuration(cfg.Updates.Interval, 24*time.Hour)
	go func() {
		ticker := time.NewTicker(interval)
		for range ticker.C {
			checkAndNotify(cfg)
		}
	}()
	slog.Info("update checker started", "interval", interval)
}

func checkAndNotify(cfg *models.Config) {
	type releaseResponse struct {
		TagName string `json:"tag_name"`
	}

	resp, err := http.Get("https://api.github.com/repos/AsikaProject/asika/releases/latest")
	if err != nil {
		slog.Warn("update check failed", "error", err)
		return
	}
	defer resp.Body.Close()

	var release releaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		slog.Warn("update check: failed to decode response", "error", err)
		return
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	currentVersion := version.Version

	if latestVersion == "" || latestVersion == currentVersion {
		return
	}

	slog.Info("new version available", "current", currentVersion, "latest", latestVersion)

	if cfg.Updates.NotifyOnNew {
		title := "Asika Update Available"
		body := "A new version of Asika (" + latestVersion + ") is available.\nRun `asika self-update` to upgrade."
		for _, nc := range cfg.Notify {
			n := createNotifierFromConfig(nc)
			if n != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				if err := n.Send(ctx, title, body); err != nil {
					slog.Warn("update notification failed", "type", nc.Type, "error", err)
				}
				cancel()
			}
		}
	}
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
