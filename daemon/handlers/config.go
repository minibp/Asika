package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "log/slog"

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
