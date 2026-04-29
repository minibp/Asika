package commands

import "github.com/spf13/cobra"

// Export subcommand variables
var (
    PrCmd     = &cobra.Command{Use: "pr", Short: "Manage pull requests"}
    QueueCmd  = &cobra.Command{Use: "queue", Short: "Manage merge queue"}
    SyncCmd   = &cobra.Command{Use: "sync", Short: "Manage sync history"}
    ConfigCmd = &cobra.Command{Use: "config", Short: "Manage configuration"}
    WizardCmd = &cobra.Command{Use: "wizard", Short: "Run configuration wizard"}
)
