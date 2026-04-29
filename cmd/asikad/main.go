package main

import (
    "log/slog"
    "os"
    "os/signal"
    "syscall"

    "asika/common/auth"
    "asika/common/config"
    "asika/common/db"
    "asika/common/events"
    "asika/common/platforms"
    "asika/daemon/consumer"
    "asika/daemon/polling"
    "asika/daemon/server"
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
        // Start server with nil config - wizard routes will be available
        srv := server.NewServer(nil)
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
        clients[platforms.PlatformGitHub] = platforms.NewGitHubClient(cfg.Tokens.GitHub)
    }
    if cfg.Tokens.GitLab != "" {
        clients[platforms.PlatformGitLab] = platforms.NewGitLabClient(cfg.Tokens.GitLab)
    }
    if cfg.Tokens.Gitea != "" {
        clients[platforms.PlatformGitea] = platforms.NewGiteaClient("https://gitea.example.com", cfg.Tokens.Gitea)
    }

    // Initialize event bus
    events.Init()

    // Setup SIGHUP handler for config reload
    setupSIGHUPHandler()

    // Check merge methods
    if err := platforms.CheckMergeMethods(cfg, clients); err != nil {
        platforms.ExitOnCheckFailed(err)
    }

    // Start poller if in polling mode
    if cfg.Events.Mode == "polling" {
        poller := polling.NewPoller(cfg, clients)
        go poller.Start()
        slog.Info("poller started")
    }

    // Start event consumer
    eventConsumer := consumer.NewConsumer()
    go eventConsumer.Start()
    slog.Info("event consumer started")

    // Create and start server
    srv := server.NewServer(cfg)

    slog.Info("asikad starting")
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
