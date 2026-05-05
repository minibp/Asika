package core

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"asika/common/config"
)

// SetupConfigReload sets up SIGHUP signal handler for hot config reload.
func SetupConfigReload() {
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
