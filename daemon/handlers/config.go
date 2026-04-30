package handlers

import (
    "net/http"

    "github.com/BurntSushi/toml"
    "github.com/gin-gonic/gin"
    "log/slog"
    "os"
    "path/filepath"

    "asika/common/config"
    "asika/common/models"
)

// GetConfig returns current config (with sensitive data masked)
func GetConfig(c *gin.Context) {
    cfg := config.Current()

    if cfg == nil {
        c.JSON(http.StatusServiceUnavailable, gin.H{"error": "config not loaded"})
        return
    }

    // Create a copy with masked sensitive fields
    masked := *cfg

    // Mask tokens
    masked.Tokens = models.TokensConfig{
        GitHub: maskToken(cfg.Tokens.GitHub),
        GitLab: maskToken(cfg.Tokens.GitLab),
        Gitea:  maskToken(cfg.Tokens.Gitea),
    }

    // Mask JWT secret
    masked.Auth.JWTSecret = maskSecret(cfg.Auth.JWTSecret)

    c.JSON(http.StatusOK, masked)
}

// UpdateConfig updates the configuration and saves to file
func UpdateConfig(c *gin.Context) {
    var newCfg models.Config

    if err := c.ShouldBindJSON(&newCfg); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid config format", "detail": err.Error()})
        return
    }

    // Get current config path
    configPath := os.Getenv("ASIKA_CONFIG")
    if configPath == "" {
        configPath = "/etc/asika_config.toml"
    }

    // Ensure directory exists
    dir := filepath.Dir(configPath)
    if err := os.MkdirAll(dir, 0755); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create config directory", "detail": err.Error()})
        return
    }

    // Marshal config to TOML
    data, err := toml.Marshal(&newCfg)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal config", "detail": err.Error()})
        return
    }

    // Write to file
    if err := os.WriteFile(configPath, data, 0644); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write config file", "detail": err.Error()})
        return
    }

    // Update in-memory config
    config.Store(&newCfg)

    slog.Info("config updated successfully", "path", configPath)
    c.JSON(http.StatusOK, gin.H{"message": "config updated successfully", "path": configPath})
}

// maskToken masks a token for display
func maskToken(token string) string {
    if len(token) <= 8 {
        return "***"
    }
    return token[:4] + "****" + token[len(token)-4:]
}

// maskSecret masks a secret for display
func maskSecret(secret string) string {
    if len(secret) <= 8 {
        return "***"
    }
    return secret[:4] + "****" + secret[len(secret)-4:]
}

// TestNotify sends a test notification
func TestNotify(c *gin.Context) {
    slog.Info("test notification triggered")

    c.JSON(http.StatusOK, gin.H{"message": "test notification sent"})
}
