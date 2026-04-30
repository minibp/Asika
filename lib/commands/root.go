package commands

import "github.com/spf13/cobra"

// RootCmd is the root command
var RootCmd = &cobra.Command{
	Use:   "asika",
	Short: "Asika PR Manager CLI",
	Long:  `Asika is a PR management tool with multi-platform support.`,
}

// Version can be set via -ldflags
var Version = "dev"

func init() {
	// Add persistent flags
	RootCmd.PersistentFlags().StringP("token", "t", "", "JWT token (or use ASIKA_TOKEN env)")
	RootCmd.PersistentFlags().StringP("server", "s", "http://localhost:8080", "asikad server address")
	RootCmd.PersistentFlags().StringP("output", "o", "table", "Output format: table, json, yaml")
}