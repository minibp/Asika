package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"asika/common/auth"
	"asika/common/config"
	"asika/common/db"
	"asika/common/events"
	"asika/common/platforms"
	"asika/daemon/consumer"
	"asika/daemon/handlers"
	"asika/daemon/polling"
	"asika/daemon/queue"
	"asika/daemon/server"
	"asika/daemon/syncer"
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
		clients[platforms.PlatformGitLab] = platforms.NewGitLabClient(cfg.Tokens.GitLab, "", cfg.Events.WebhookSecret)
	}
	if cfg.Tokens.Gitea != "" {
		clients[platforms.PlatformGitea] = platforms.NewGiteaClient("https://gitea.example.com", cfg.Tokens.Gitea, cfg.Events.WebhookSecret)
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
