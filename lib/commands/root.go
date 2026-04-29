package commands

import "github.com/spf13/cobra"

// RootCmd is the root command
var RootCmd = &cobra.Command{
    Use:   "asika",
    Short: "Asika PR Manager CLI",
    Long:  `Asika is a PR management tool with multi-platform support.`,
}

// Export subcommand variables
var (
    PrCmd     = &cobra.Command{Use: "pr", Short: "Manage pull requests"}
    QueueCmd  = &cobra.Command{Use: "queue", Short: "Manage merge queue"}
    SyncCmd   = &cobra.Command{Use: "sync", Short: "Manage sync history"}
    ConfigCmd = &cobra.Command{Use: "config", Short: "Manage configuration"}
    WizardCmd = &cobra.Command{Use: "wizard", Short: "Run configuration wizard"}
)

func init() {
    RootCmd.AddCommand(PrCmd)
    RootCmd.AddCommand(QueueCmd)
    RootCmd.AddCommand(SyncCmd)
    RootCmd.AddCommand(ConfigCmd)
    RootCmd.AddCommand(WizardCmd)

    // Add persistent flags
    RootCmd.PersistentFlags().StringP("token", "t", "", "JWT token (or use ASIKA_TOKEN env)")
    RootCmd.PersistentFlags().StringP("server", "s", "http://localhost:8080", "asikad server address")
    RootCmd.PersistentFlags().StringP("output", "o", "table", "Output format: table, json, yaml")
}
