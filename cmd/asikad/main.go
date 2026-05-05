package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"gopkg.in/telebot.v3"

	"asika/common/auth"
	"asika/common/config"
	"asika/common/db"
	"asika/common/version"
	"asika/common/events"
	"asika/common/models"
	"asika/common/notifier"
	"asika/common/platforms"
	"asika/common/utils"
	"asika/daemon/consumer"
	"asika/daemon/handlers"
	"asika/daemon/platform"
	"asika/daemon/polling"
	"asika/daemon/queue"
	"asika/daemon/server"
	"asika/daemon/syncer"
)

func main() {
    desktopMode := flag.Bool("desktop", false, "Run in desktop foreground mode (open browser to WebUI)")
    versionFlag := flag.Bool("version", false, "Print version and exit")
    flag.Parse()

    if *versionFlag {
        fmt.Printf("Asika daemon version %s\n", version.Version)
        return
    }

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

        if *desktopMode {
            go func() {
                time.Sleep(500 * time.Millisecond)
                slog.Info("opening browser", "url", "http://localhost:8080")
                if err := utils.OpenBrowser("http://localhost:8080"); err != nil {
                    slog.Warn("failed to open browser", "error", err)
                }
            }()
        }

        if err := srv.Start(); err != nil {
            slog.Error("server failed", "error", err)
            os.Exit(1)
        }
        return
    }

     // Initialize database with retry (handles stale lock from ungraceful shutdown)
     if err := dbInitWithRetry(cfg.Database.Path, 5); err != nil {
         slog.Error("failed to initialize database", "error", err)
         os.Exit(1)
     }
     slog.Info("database initialized", "path", cfg.Database.Path)
     defer func() {
         slog.Info("closing database")
         db.Close()
         slog.Info("database closed")
     }()

    // Migrate old DB records (repo group name changes, e.g. "main" -> "default")
    migrateRepoGroupNames(cfg)

    // Migrate PR states: closed PRs with MergeCommitSHA should be "merged"
    migratePRStates(cfg)

	// Create platform clients (needed for PR fetch)
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

    // Initialize auth
    auth.Init(cfg.Auth.JWTSecret, config.GenerateTokenExpiry(cfg.Auth.TokenExpiry))

    // Initialize event bus (must be before PR fetch to avoid panic)
    events.Init()

      // Initial PR fetch: fetch all PRs from platforms after events init
      var poller *polling.Poller
      if cfg.Events.Mode == "polling" {
          poller = polling.NewPoller(cfg, clients)
          poller.PollOnce() // Synchronous initial fetch
          go poller.Start()
          slog.Info("background poller started")
      } else {
          poller = polling.NewPoller(cfg, clients)
          poller.PollOnce()
      }

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
 		defer ticker.Stop()
 		for {
 			select {
 			case <-ticker.C:
 				queueMgr.CheckQueue()
 			case <-queueMgr.StopChan():
 				slog.Info("merge queue checker stopped")
 				return
 			}
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
        defer ticker.Stop()
         for {
            select {
            case <-ticker.C:
                spamDetector.Scan()
            case <-spamDetector.StopChan():
                slog.Info("spam detector stopped")
                return
            }
         }
    }()
    slog.Info("spam detector started", "enabled", cfg.Spam.Enabled)

    // Start event consumer (with clients for wiring)
    eventConsumer := consumer.NewConsumerWithClients(cfg, clients)
    go eventConsumer.Start()
    slog.Info("event consumer started")

     // Start Telegram bot (interactive decisions + notifications)
     tgBot := startTelegramBot(cfg, clients, queueMgr, syncr, spamDetector)

     // Start Feishu bot (interactive decisions + notifications)
     fsBot := startFeishuBot(cfg, clients, queueMgr, syncr, spamDetector)

	// Create and start server
	srv := server.NewServer(cfg, clients)

	// Setup shutdown handler after server is created
	sigChan := setupShutdownHandler()

	slog.Info("Asika daemon starting")
	slog.Info("Copyright (R) 2026 The minibp developers. All rights reserved")

	if *desktopMode {
		listenAddr := ":8080"
		if cfg.Server.Listen != "" {
			listenAddr = cfg.Server.Listen
		}
		url := serverURL(listenAddr)
		go func() {
			time.Sleep(500 * time.Millisecond)
			slog.Info("opening browser", "url", url)
			if err := utils.OpenBrowser(url); err != nil {
				slog.Warn("failed to open browser", "error", err)
			}
		}()
	}

 	// Start server in background so we can handle shutdown signals
 	go func() {
 		if err := srv.Start(); err != nil {
 			if err == http.ErrServerClosed {
 				slog.Info("http server closed")
 			} else {
 				slog.Error("server failed", "error", err)
 				os.Exit(1)
 			}
 		}
 	}()

	// Wait for shutdown signal
	sig := <-sigChan
	slog.Info("received signal, shutting down", "signal", sig)

	// 1. Stop background workers (stop writing to DB)
	queueMgr.Stop()
	spamDetector.Stop()
	eventConsumer.Stop()
	poller.Stop()

	// 2. Stop platform bots
	if tgBot != nil {
		tgBot.Stop()
	}
	if fsBot != nil {
		fsBot.Stop()
	}

	// 3. Stop HTTP server (finish in-flight requests)
	if err := srv.Stop(); err != nil {
		slog.Error("server stop error", "error", err)
	}

	// 4. Close DB (release locks)
	db.Close()

	slog.Info("shutdown complete")
	os.Exit(0)
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

// setupShutdownHandler handles SIGINT/SIGTERM for graceful shutdown
func setupShutdownHandler() chan os.Signal {
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    return sigChan
}

// dbInitWithRetry initializes DB with retries for lock conflicts
func dbInitWithRetry(dbPath string, maxRetries int) error {
    for i := 0; i < maxRetries; i++ {
        err := db.Init(dbPath)
        if err == nil {
            return nil
        }
        if i < maxRetries-1 {
            slog.Warn("db init failed, retrying", "attempt", i+1, "max", maxRetries, "error", err)
            time.Sleep(2 * time.Second)
        }
    }
    return db.Init(dbPath)
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
// Returns the bot instance (or nil if not configured), so it can be stopped on shutdown.
func startTelegramBot(
	cfg *models.Config,
	clients map[platforms.PlatformType]platforms.PlatformClient,
	queueMgr *queue.Manager,
	syncr *syncer.Syncer,
	spamDetector *syncer.SpamDetector,
) *platform.TelegramBot {
	if cfg == nil || !cfg.Telegram.Enabled || cfg.Telegram.Token == "" {
		return nil
	}

	pref := telebot.Settings{
		Token:  cfg.Telegram.Token,
		Poller: &telebot.LongPoller{Timeout: 10},
	}

	bot, err := telebot.NewBot(pref)
	if err != nil {
		slog.Error("failed to create telegram bot", "error", err)
		return nil
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

	tgBot := platform.NewTelegramBot(
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
	return tgBot
}

func toStringList(strings []string) []interface{} {
	result := make([]interface{}, len(strings))
	for i, s := range strings {
		result[i] = s
	}
	return result
}

// startFeishuBot starts the Feishu interactive bot if configured.
// Returns the bot instance (or nil if not configured), so it can be stopped on shutdown.
func startFeishuBot(
	cfg *models.Config,
	clients map[platforms.PlatformType]platforms.PlatformClient,
	queueMgr *queue.Manager,
	syncr *syncer.Syncer,
	spamDetector *syncer.SpamDetector,
) *platform.FeishuBot {
	if cfg == nil || !cfg.Feishu.Enabled || cfg.Feishu.AppID == "" {
		return nil
	}

	// Create feishu notifier
	cfgMap := map[string]interface{}{
		"webhook_url": cfg.Feishu.WebhookURL,
		"app_id":      cfg.Feishu.AppID,
		"app_secret":  cfg.Feishu.AppSecret,
	}
	feishuNotifier := notifier.NewFeishuNotifier(cfgMap)

	fsBot := platform.NewFeishuBot(
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
	return fsBot
}

// serverURL builds a browser URL from a listen address.
func serverURL(listen string) string {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return "http://localhost:8080"
	}
	if host == "" || host == "0.0.0.0" {
		host = "localhost"
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return fmt.Sprintf("http://%s:%s", host, port)
}

// migrateRepoGroupNames updates old DB records when repo group name changed (e.g. "main" -> "default")
func migrateRepoGroupNames(cfg *models.Config) {
	if len(cfg.RepoGroups) == 0 {
		return
	}
	currentName := cfg.RepoGroups[0].Name

	// Collect known repo group names from config
	validNames := make(map[string]bool)
	for _, rg := range cfg.RepoGroups {
		validNames[rg.Name] = true
	}

	// Migrate PR records
	var prKeysToDelete []string
	var prsToReinsert []struct {
		key     string
		value   []byte
		pr      models.PRRecord
		newKey  string
	}
	_ = db.ForEach(db.BucketPRs, func(key, value []byte) error {
		var pr models.PRRecord
		if json.Unmarshal(value, &pr) != nil {
			return nil
		}
		if !validNames[pr.RepoGroup] {
			pr.RepoGroup = currentName
			newKey := currentName + "#" + pr.ID
			updated, _ := json.Marshal(pr)
			prsToReinsert = append(prsToReinsert, struct {
				key     string
				value   []byte
				pr      models.PRRecord
				newKey  string
			}{string(key), updated, pr, newKey})
			prKeysToDelete = append(prKeysToDelete, string(key))
			slog.Info("migrating PR record", "old_key", string(key), "new_key", newKey, "pr_id", pr.ID)
		}
		return nil
	})
	for _, item := range prsToReinsert {
		db.PutPRWithIndex(item.newKey, item.value, item.pr.ID, item.pr.RepoGroup, item.pr.PRNumber)
	}
	for _, k := range prKeysToDelete {
		db.Delete(db.BucketPRs, k)
	}

	// Migrate queue items
	var qiKeysToDelete []string
	var qisToReinsert []struct {
		key    string
		value  []byte
		newKey string
	}
	_ = db.ForEach(db.BucketQueueItems, func(key, value []byte) error {
		var item models.QueueItem
		if json.Unmarshal(value, &item) != nil {
			return nil
		}
		if !validNames[item.RepoGroup] {
			item.RepoGroup = currentName
			newKey := currentName + "#" + item.PRID
			updated, _ := json.Marshal(item)
			qisToReinsert = append(qisToReinsert, struct {
				key    string
				value  []byte
				newKey string
			}{string(key), updated, newKey})
			qiKeysToDelete = append(qiKeysToDelete, string(key))
			slog.Info("migrating queue item", "old_key", string(key), "new_key", newKey, "pr_id", item.PRID)
		}
		return nil
	})
	for _, item := range qisToReinsert {
		db.Put(db.BucketQueueItems, item.newKey, item.value)
	}
	for _, k := range qiKeysToDelete {
		db.Delete(db.BucketQueueItems, k)
	}

	if len(prsToReinsert)+len(qisToReinsert) > 0 {
		slog.Info("repo group migration complete", "prs_migrated", len(prsToReinsert), "queue_items_migrated", len(qisToReinsert))
	}
}

// migratePRStates fixes historical PR records: closed PRs with MergedAt set should be "merged"
func migratePRStates(cfg *models.Config) {
	var keysToUpdate []struct {
		key   string
		value []byte
	}
	_ = db.ForEach(db.BucketPRs, func(key, value []byte) error {
		var pr models.PRRecord
		if json.Unmarshal(value, &pr) != nil {
			return nil
		}
		if pr.State == "closed" && !pr.MergedAt.IsZero() {
			pr.State = "merged"
			data, _ := json.Marshal(pr)
			keysToUpdate = append(keysToUpdate, struct {
				key   string
				value []byte
			}{string(key), data})
		}
		return nil
	})
	for _, item := range keysToUpdate {
		db.Put(db.BucketPRs, item.key, item.value)
	}
	if len(keysToUpdate) > 0 {
		slog.Info("PR state migration complete", "merged_fixed", len(keysToUpdate))
	}
}

// syncPRStates refreshes PR state, merge_commit_sha, etc. from platform for open/closed PRs in local DB
func syncPRStates(cfg *models.Config, clients map[platforms.PlatformType]platforms.PlatformClient) {
	if len(clients) == 0 {
		return
	}

	type prUpdate struct {
		key    string
		data   []byte
		pr     models.PRRecord
	}

	ctx := context.Background()
	var updates []prUpdate

	slog.Info("syncing PR states from platform...")

	_ = db.ForEach(db.BucketPRs, func(key, value []byte) error {
		var pr models.PRRecord
		if json.Unmarshal(value, &pr) != nil {
			return nil
		}
		if pr.PRNumber == 0 || pr.Platform == "" {
			return nil
		}
		if pr.State == "merged" || pr.State == "closed" {
			return nil
		}

		group := config.GetRepoGroupByName(cfg, pr.RepoGroup)
		if group == nil {
			return nil
		}

		platType := platforms.PlatformType(pr.Platform)
		client, ok := clients[platType]
		if !ok {
			return nil
		}

		owner, repo := config.GetOwnerRepoFromGroup(group, pr.Platform)
		updated, err := client.GetPR(ctx, owner, repo, pr.PRNumber)
		if err != nil || updated == nil {
			slog.Warn("failed to sync PR state", "pr", pr.PRNumber, "platform", pr.Platform, "error", err)
			return nil
		}

		if updated.State != pr.State {
			slog.Info("updating PR state", "pr", pr.PRNumber, "old", pr.State, "new", updated.State)
			pr.State = updated.State
		}
		if updated.MergeCommitSHA != "" && pr.MergeCommitSHA != updated.MergeCommitSHA {
			pr.MergeCommitSHA = updated.MergeCommitSHA
		}
		pr.IsApproved = updated.IsApproved
		pr.IsDraft = updated.IsDraft
		pr.HasConflict = updated.HasConflict
		pr.HTMLURL = updated.HTMLURL
		pr.Labels = updated.Labels
		pr.UpdatedAt = updated.UpdatedAt

		data, _ := json.Marshal(pr)
		updates = append(updates, prUpdate{string(key), data, pr})
		return nil
	})

	for _, u := range updates {
		db.PutPRWithIndex(u.key, u.data, u.pr.ID, u.pr.RepoGroup, u.pr.PRNumber)
	}

	if len(updates) > 0 {
		slog.Info("PR state sync complete", "updated", len(updates))
	} else {
		slog.Info("PR state sync complete (no changes)")
	}
}
