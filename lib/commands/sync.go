package commands

import (
    "fmt"

    "github.com/spf13/cobra"
)

// syncCmd represents the sync command
var syncCmd = &cobra.Command{
    Use:   "sync",
    Short: "Manage sync history",
}

// syncHistoryCmd shows sync history
var syncHistoryCmd = &cobra.Command{
    Use:   "history",
    Short: "Show sync history",
    Run: func(cmd *cobra.Command, args []string) {
        fmt.Println("Sync history")
    },
}

// syncRetryCmd retries a sync
var syncRetryCmd = &cobra.Command{
    Use:   "retry [sync_id]",
    Short: "Retry a failed sync",
    Args:  cobra.ExactArgs(1),
    Run: func(cmd *cobra.Command, args []string) {
        fmt.Println("Sync retry triggered")
    },
}

func init() {
    syncCmd.AddCommand(syncHistoryCmd)
    syncCmd.AddCommand(syncRetryCmd)

    syncHistoryCmd.Flags().String("repo_group", "", "Filter by repo group")
    syncHistoryCmd.Flags().Int("limit", 20, "Max number of records")
}
