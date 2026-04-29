package commands

import (
    "fmt"

    "github.com/spf13/cobra"
)

// wizardCmd runs the init wizard
var wizardCmd = &cobra.Command{
    Use:   "wizard",
    Short: "Run configuration wizard",
    Run: func(cmd *cobra.Command, args []string) {
        fmt.Println("Wizard started")
    },
}
