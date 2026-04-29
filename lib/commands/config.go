package commands

import (
    "fmt"

    "github.com/spf13/cobra"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
    Use:   "config",
    Short: "Manage configuration",
}

// configShowCmd shows config
var configShowCmd = &cobra.Command{
    Use:   "show",
    Short: "Show current config (masked)",
    Run: func(cmd *cobra.Command, args []string) {
        fmt.Println("Config shown")
    },
}

// configReloadCmd triggers config reload
var configReloadCmd = &cobra.Command{
    Use:   "reload",
    Short: "Trigger config hot reload",
    Run: func(cmd *cobra.Command, args []string) {
        fmt.Println("Config reload triggered")
    },
}

func init() {
    configCmd.AddCommand(configShowCmd)
    configCmd.AddCommand(configReloadCmd)
}
