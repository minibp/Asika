package server

import (
	"log/slog"
	"os"
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
	"asika/daemon/handlers"
	"asika/daemon/platform"
	"asika/daemon/polling"
	"asika/daemon/queue"
	"asika/daemon/syncer"
)

// Bootstrap initializes and starts all daemon subsystems.
// Returns the HTTP server ready to Start().
func Bootstrap(cfg *models.Config) (*Server, error) {
	// 1. Init database
	if err := db.Init(cfg.Database.Path); err != nil {
		return nil, err
	}

	// 2. Init auth
	auth.Init(cfg.Auth.JWTSecret, config.GenerateTokenExpiry(cfg.Auth.TokenExpiry))

	// 3. Create platform clients
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

	// 4. Init event bus
	events.Init()

	// 5. Check merge methods (fatal if fail)
	if err := platforms.CheckMergeMethods(cfg, clients); err != nil {
		slog.Error("FATAL: merge method check failed", "error", err)
		os.Exit(1)
	}

	// 6. Start merge queue
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

	// 7. Start spam detector
	spamDetector := syncer.NewSpamDetectorWithClients(cfg, clients)
	go func() {
		if !cfg.Spam.Enabled {
			return
		}
		window := parseDuration(cfg.Spam.TimeWindow, 10*time.Minute)
		ticker := time.NewTicker(window / 2)
		for range ticker.C {
			spamDetector.Scan()
		}
	}()
	slog.Info("spam detector started", "enabled", cfg.Spam.Enabled)

	// 8. Start poller (if polling mode)
	if cfg.Events.Mode == "polling" {
		poller := polling.NewPoller(cfg, clients)
		go poller.Start()
		slog.Info("poller started")
	}

	// 9. Start event consumer
	eventConsumer := consumer.NewConsumerWithClients(cfg, clients)
	go eventConsumer.Start()
	slog.Info("event consumer started")

	// 10. Start Telegram bot
	startTelegram(cfg, clients, queueMgr, syncr, spamDetector)

	// 11. Start Feishu bot
	startFeishu(cfg, clients, queueMgr, syncr, spamDetector)

	// 12. Create server
	handlers.InitClients(clients)
	srv := NewServer(cfg, clients)

	return srv, nil
}

// startTelegram starts the Telegram interactive bot if configured.
func startTelegram(
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

	cfgMap := map[string]interface{}{
		"token": cfg.Telegram.Token,
		"to":    toStringList(cfg.Telegram.ChatIDs),
	}
	telegramNotifier := notifier.NewTelegramNotifier(cfgMap)

	tgBot := platform.NewTelegramBot(
		bot, cfg, clients, queueMgr, syncr, spamDetector,
		telegramNotifier, cfg.Telegram.AdminIDs,
	)

	go tgBot.Start()
	slog.Info("telegram bot started", "admin_ids", len(cfg.Telegram.AdminIDs))
}

// startFeishu starts the Feishu interactive bot if configured.
func startFeishu(
	cfg *models.Config,
	clients map[platforms.PlatformType]platforms.PlatformClient,
	queueMgr *queue.Manager,
	syncr *syncer.Syncer,
	spamDetector *syncer.SpamDetector,
) {
	if cfg == nil || !cfg.Feishu.Enabled || cfg.Feishu.AppID == "" {
		return
	}

	cfgMap := map[string]interface{}{
		"webhook_url": cfg.Feishu.WebhookURL,
		"app_id":      cfg.Feishu.AppID,
		"app_secret":  cfg.Feishu.AppSecret,
	}
	feishuNotifier := notifier.NewFeishuNotifier(cfgMap)

	fsBot := platform.NewFeishuBot(
		cfg, clients, queueMgr, syncr, spamDetector, feishuNotifier,
	)

	handlers.InitFeishuBot(fsBot)

	go fsBot.Start()
	slog.Info("feishu bot started", "app_id", cfg.Feishu.AppID)
}

func parseDuration(s string, defaultDur time.Duration) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return defaultDur
	}
	return d
}

func toStringList(strs []string) []interface{} {
	result := make([]interface{}, len(strs))
	for i, s := range strs {
		result[i] = s
	}
	return result
}