package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"asika/common/version"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Asika version %s\n", version.Version)
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}
