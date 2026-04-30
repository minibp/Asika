package handlers

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/gin-gonic/gin"
	"log/slog"

	"asika/common/config"
	"asika/common/models"
)

// GetWizardSteps handles GET /api/v1/wizard (10. WebUI Wizard)
func GetWizardSteps(c *gin.Context) {
	// Check if already initialized
	if config.IsInitialized(config.ConfigPath) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "already initialized"})
		return
	}

	steps := []string{
		"mode_selection",
		"database_config",
		"platform_tokens",
		"repository_group",
		"admin_account",
	}
	c.JSON(http.StatusOK, gin.H{"steps": steps})
}

// SubmitWizardStep handles POST /api/v1/wizard/step/:step
func SubmitWizardStep(c *gin.Context) {
	step := c.Param("step")

	var data map[string]interface{}
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	slog.Info("wizard step submitted", "step", step, "data", data)
	c.JSON(http.StatusOK, gin.H{"message": "step saved", "step": step})
}

// CompleteWizard handles POST /api/v1/wizard/step/complete (10. WebUI Wizard)
// Writes config to /etc/asika_config.toml
func CompleteWizard(c *gin.Context) {
	var cfg models.Config

	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid config", "detail": err.Error()})
		return
	}

	// Get config path
	configPath := os.Getenv("ASIKA_CONFIG")
	if configPath == "" {
		configPath = "/etc/asika_config.toml"
	}

	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create config directory"})
		return
	}

	// Marshal to TOML
	data, err := toml.Marshal(&cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal config"})
		return
	}

	// Write to file
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write config file"})
		return
	}

	// Update in-memory config
	config.Store(&cfg)

	slog.Info("wizard completed, config saved", "path", configPath)
	c.JSON(http.StatusOK, gin.H{"message": "setup complete", "path": configPath})
}
