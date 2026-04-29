package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var Version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Asika version %s\n", Version)
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}
