package commands

import (
    "fmt"

    "github.com/spf13/cobra"
)

// queueCmd represents the queue command
var queueCmd = &cobra.Command{
    Use:   "queue",
    Short: "Manage merge queue",
}

// queueListCmd lists queue items
var queueListCmd = &cobra.Command{
    Use:   "list [repo_group]",
    Short: "List queue items",
    Args:  cobra.ExactArgs(1),
    Run: func(cmd *cobra.Command, args []string) {
        fmt.Println("Queue list")
    },
}

// queueRecheckCmd triggers recheck
var queueRecheckCmd = &cobra.Command{
    Use:   "recheck [repo_group]",
    Short: "Trigger queue recheck",
    Args:  cobra.ExactArgs(1),
    Run: func(cmd *cobra.Command, args []string) {
        fmt.Println("Queue recheck triggered")
    },
}

func init() {
    queueCmd.AddCommand(queueListCmd)
    queueCmd.AddCommand(queueRecheckCmd)
}
