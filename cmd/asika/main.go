package main

import (
    "os"

    "github.com/spf13/cobra"

    "asika/lib/commands"
)

func main() {
    rootCmd := &cobra.Command{
        Use:   "asika",
        Short: "Asika PR Manager CLI",
        Long:  `Asika is a PR management tool with multi-platform support.`,
    }

    // Add subcommands
    rootCmd.AddCommand(commands.PrCmd)
    rootCmd.AddCommand(commands.QueueCmd)
    rootCmd.AddCommand(commands.SyncCmd)
    rootCmd.AddCommand(commands.ConfigCmd)
    rootCmd.AddCommand(commands.WizardCmd)

    // Add flags
    rootCmd.PersistentFlags().StringP("token", "t", "", "JWT token (or use ASIKA_TOKEN env)")
    rootCmd.PersistentFlags().StringP("server", "s", "http://localhost:8080", "asikad server address")
    rootCmd.PersistentFlags().StringP("output", "o", "table", "Output format: table, json, yaml")

    if err := rootCmd.Execute(); err != nil {
        os.Exit(1)
    }
}
