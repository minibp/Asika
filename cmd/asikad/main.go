package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gopkg.in/telebot.v3"

	"asika/common/auth"
	"asika/common/config"
	"asika/common/db"
	"asika/common/events"
	"asika/common/models"
	"asika/common/notifier"
	"asika/common/platforms"
	"asika/daemon/consumer"
	feishubot "asika/daemon/feishu"
	"asika/daemon/handlers"
	"asika/daemon/polling"
	"asika/daemon/queue"
	"asika/daemon/server"
	"asika/daemon/syncer"
	tgbot "asika/daemon/telegram"
)

func main() {
    // Load config
    configPath := os.Getenv("ASIKA_CONFIG")
    if configPath == "" {
        configPath = "/etc/asika_config.toml"
    }

    cfg, err := config.Load(configPath)

    // If config doesn't exist, start server in initialization mode
    if err != nil {
        slog.Warn("config not found, starting in initialization mode", "error", err)
        srv := server.NewServer(nil, nil)
        slog.Info("asikad starting in initialization mode")
        if err := srv.Start(); err != nil {
            slog.Error("server failed", "error", err)
            os.Exit(1)
        }
        return
    }

    // Initialize database
    if err := db.Init(cfg.Database.Path); err != nil {
        slog.Error("failed to initialize database", "error", err)
        os.Exit(1)
    }
    defer db.Close()

    // Initialize auth
    auth.Init(cfg.Auth.JWTSecret, config.GenerateTokenExpiry(cfg.Auth.TokenExpiry))

	// Create platform clients
	clients := make(map[platforms.PlatformType]platforms.PlatformClient)

	if cfg.Tokens.GitHub != "" {
		clients[platforms.PlatformGitHub] = platforms.NewGitHubClient(cfg.Tokens.GitHub, cfg.Events.WebhookSecret)
	}
	if cfg.Tokens.GitLab != "" {
		clients[platforms.PlatformGitLab] = platforms.NewGitLabClient(cfg.Tokens.GitLab, cfg.GitLabBaseURL, cfg.Events.WebhookSecret)
	}
	if cfg.Tokens.Gitea != "" {
		giteaURL := cfg.GiteaBaseURL
		if giteaURL == "" {
			giteaURL = "https://gitea.example.com"
		}
		if gc := platforms.NewGiteaClient(giteaURL, cfg.Tokens.Gitea, cfg.Events.WebhookSecret); gc != nil {
			clients[platforms.PlatformGitea] = gc
		}
	}

    // Initialize event bus
    events.Init()

    // Setup SIGHUP handler for config reload
    setupSIGHUPHandler()

    // Check merge methods
    if err := platforms.CheckMergeMethods(cfg, clients); err != nil {
        platforms.ExitOnCheckFailed(err)
    }

	// Start merge queue manager and periodic checker
	queueMgr := queue.NewManager(cfg, clients)
	handlers.InitQueueMgr(queueMgr)
	syncr := syncer.NewSyncer(cfg, clients)
	handlers.InitSyncer(syncr)
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		for range ticker.C {
			queueMgr.CheckQueue()
		}
	}()
	slog.Info("merge queue checker started")

    // Start spam detector periodic scan
    spamDetector := syncer.NewSpamDetectorWithClients(cfg, clients)
    go func() {
        if !cfg.Spam.Enabled {
            return
        }
        window := parseDurationDefault(cfg.Spam.TimeWindow, 10*time.Minute)
        ticker := time.NewTicker(window / 2)
        for range ticker.C {
            spamDetector.Scan()
        }
    }()
    slog.Info("spam detector started", "enabled", cfg.Spam.Enabled)

    // Start poller if in polling mode
    if cfg.Events.Mode == "polling" {
        poller := polling.NewPoller(cfg, clients)
        go poller.Start()
        slog.Info("poller started")
    }

    // Start event consumer (with clients for wiring)
    eventConsumer := consumer.NewConsumerWithClients(cfg, clients)
    go eventConsumer.Start()
    slog.Info("event consumer started")

    // Start Telegram bot (interactive decisions + notifications)
    startTelegramBot(cfg, clients, queueMgr, syncr, spamDetector)

    // Start Feishu bot (interactive decisions + notifications)
    startFeishuBot(cfg, clients, queueMgr, syncr, spamDetector)

	// Create and start server
	srv := server.NewServer(cfg, clients)

	slog.Info("Asika daemon starting")
	slog.Info("Copyright (R) 2026 The minibp developers. All rights reserved")
	if err := srv.Start(); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

// setupSIGHUPHandler sets up SIGHUP signal handler for hot reload
func setupSIGHUPHandler() {
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGHUP)

    go func() {
        for range sigChan {
            slog.Info("received SIGHUP, reloading config")
            cfg, err := config.Load(config.ConfigPath)
            if err != nil {
                slog.Error("failed to reload config", "error", err)
                continue
            }
            config.Store(cfg)
            slog.Info("config reloaded successfully")
        }
    }()
}

// parseDurationDefault parses a duration with a fallback
func parseDurationDefault(s string, defaultDur time.Duration) time.Duration {
    d, err := time.ParseDuration(s)
    if err != nil {
        return defaultDur
    }
    return d
}

// startTelegramBot starts the Telegram interactive bot if configured.
func startTelegramBot(
	cfg *models.Config,
	clients map[platforms.PlatformType]platforms.PlatformClient,
	queueMgr *queue.Manager,
	syncr *syncer.Syncer,
	spamDetector *syncer.SpamDetector,
) {
	if cfg == nil || !cfg.Telegram.Enabled || cfg.Telegram.Token == "" {
		return
	}

	pref := telebot.Settings{
		Token:  cfg.Telegram.Token,
		Poller: &telebot.LongPoller{Timeout: 10},
	}

	bot, err := telebot.NewBot(pref)
	if err != nil {
		slog.Error("failed to create telegram bot", "error", err)
		return
	}

	// Find or create the TelegramNotifier for notification sending
	var telegramNotifier *notifier.TelegramNotifier
	for _, nc := range cfg.Notify {
		if nc.Type == "telegram" {
			cfgMap := map[string]interface{}{
				"token": cfg.Telegram.Token,
				"to":    toStringList(cfg.Telegram.ChatIDs),
			}
			telegramNotifier = notifier.NewTelegramNotifier(cfgMap)
			if telegramNotifier != nil && telegramNotifier.Bot() == nil {
				// Inject our bot instance
				// We need to create it with the bot we already have
				_ = telegramNotifier
			}
			break
		}
	}

	// If no notifier configured, create one anyway for the interactive bot
	if telegramNotifier == nil {
		cfgMap := map[string]interface{}{
			"token": cfg.Telegram.Token,
			"to":    toStringList(cfg.Telegram.ChatIDs),
		}
		telegramNotifier = notifier.NewTelegramNotifier(cfgMap)
	}

	tgBot := tgbot.NewBot(
		bot,
		cfg,
		clients,
		queueMgr,
		syncr,
		spamDetector,
		telegramNotifier,
		cfg.Telegram.AdminIDs,
	)

	go tgBot.Start()
	slog.Info("telegram bot started", "admin_ids", len(cfg.Telegram.AdminIDs))
}

func toStringList(strings []string) []interface{} {
	result := make([]interface{}, len(strings))
	for i, s := range strings {
		result[i] = s
	}
	return result
}

// startFeishuBot starts the Feishu interactive bot if configured.
func startFeishuBot(
	cfg *models.Config,
	clients map[platforms.PlatformType]platforms.PlatformClient,
	queueMgr *queue.Manager,
	syncr *syncer.Syncer,
	spamDetector *syncer.SpamDetector,
) {
	if cfg == nil || !cfg.Feishu.Enabled || cfg.Feishu.AppID == "" {
		return
	}

	// Create feishu notifier
	cfgMap := map[string]interface{}{
		"webhook_url": cfg.Feishu.WebhookURL,
		"app_id":      cfg.Feishu.AppID,
		"app_secret":  cfg.Feishu.AppSecret,
	}
	feishuNotifier := notifier.NewFeishuNotifier(cfgMap)

	fsBot := feishubot.NewBot(
		cfg,
		clients,
		queueMgr,
		syncr,
		spamDetector,
		feishuNotifier,
	)

	handlers.InitFeishuBot(fsBot)

	go fsBot.Start()
	slog.Info("feishu bot started", "app_id", cfg.Feishu.AppID)
}
