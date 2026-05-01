package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var queueCmd = &cobra.Command{
	Use:   "queue",
	Short: "Manage merge queue",
}

var queueListCmd = &cobra.Command{
	Use:   "list [repo_group]",
	Short: "List queue items",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		url := fmt.Sprintf("%s/api/v1/queue/%s",
			GetServer(cmd), args[0],
		)
		resp := doRequest("GET", url, cmd)
		if resp == nil {
			return
		}
		handleResponse(resp, "No items in queue")
	},
}

var queueRecheckCmd = &cobra.Command{
	Use:   "recheck [repo_group]",
	Short: "Trigger queue recheck",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		url := fmt.Sprintf("%s/api/v1/queue/%s/recheck",
			GetServer(cmd), args[0],
		)
		resp := doRequest("POST", url, cmd)
		if resp == nil {
			return
		}
		handleWriteResponse(resp, "Queue recheck triggered")
	},
}

func init() {
	queueCmd.AddCommand(queueListCmd)
	queueCmd.AddCommand(queueRecheckCmd)
	RootCmd.AddCommand(queueCmd)
}
