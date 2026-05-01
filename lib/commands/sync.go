package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Manage sync history",
}

var syncHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "Show sync history",
	Run: func(cmd *cobra.Command, args []string) {
		url := fmt.Sprintf("%s/api/v1/sync/history", GetServer(cmd))
		resp := doRequest("GET", url, cmd)
		if resp == nil {
			return
		}
		handleResponse(resp, "No sync history")
	},
}

var syncRetryCmd = &cobra.Command{
	Use:   "retry [sync_id]",
	Short: "Retry a failed sync",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		url := fmt.Sprintf("%s/api/v1/sync/retry/%s",
			GetServer(cmd), args[0],
		)
		resp := doRequest("POST", url, cmd)
		if resp == nil {
			return
		}
		handleWriteResponse(resp, "Sync retry triggered")
	},
}

func init() {
	syncCmd.AddCommand(syncHistoryCmd)
	syncCmd.AddCommand(syncRetryCmd)

	syncHistoryCmd.Flags().String("repo_group", "", "Filter by repo group")
	syncHistoryCmd.Flags().Int("limit", 20, "Max number of records")

	RootCmd.AddCommand(syncCmd)
}
