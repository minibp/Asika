package commands

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// RootCmd is the root command
var RootCmd = &cobra.Command{
	Use:   "asika",
	Short: "Asika PR Manager CLI",
	Long:  `Asika is a PR management tool with multi-platform support.`,
}

// Version can be set via -ldflags
var Version = "dev"

type cliConfig struct {
	Token  string `json:"token"`
	Server string `json:"server"`
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "asika", "config.json")
}

func loadCLIConfig() cliConfig {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return cliConfig{}
	}
	var cfg cliConfig
	json.Unmarshal(data, &cfg)
	return cfg
}

func saveCLIConfig(cfg cliConfig) {
	dir := filepath.Dir(configPath())
	os.MkdirAll(dir, 0700)
	data, _ := json.Marshal(cfg)
	os.WriteFile(configPath(), data, 0600)
}

// GetToken returns token from flag, env, or saved file
func GetToken(cmd *cobra.Command) string {
	token, _ := cmd.Flags().GetString("token")
	if token != "" {
		return token
	}
	token = os.Getenv("ASIKA_TOKEN")
	if token != "" {
		return token
	}
	return loadCLIConfig().Token
}

// GetServer returns server address from flag, env, or saved config
func GetServer(cmd *cobra.Command) string {
	server, _ := cmd.Flags().GetString("server")
	if server != "" && server != "http://localhost:8080" {
		return server
	}
	if s := os.Getenv("ASIKA_SERVER"); s != "" {
		return s
	}
	if cfg := loadCLIConfig(); cfg.Server != "" {
		return cfg.Server
	}
	return "http://localhost:8080"
}

func init() {
	RootCmd.PersistentFlags().StringP("token", "t", "", "JWT token (or use ASIKA_TOKEN env)")
	RootCmd.PersistentFlags().StringP("server", "s", "http://localhost:8080", "asikad server address")
	RootCmd.PersistentFlags().StringP("output", "o", "table", "Output format: table, json, yaml")
}