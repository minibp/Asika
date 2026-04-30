package commands

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"

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
        server := GetServer(cmd)

        url := fmt.Sprintf("%s/api/v1/sync/history", server)
        req, err := http.NewRequest("GET", url, nil)
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            return
        }

        token := GetToken(cmd)
        if token != "" {
            req.Header.Set("Authorization", "Bearer "+token)
        }

        resp, err := http.DefaultClient.Do(req)
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            return
        }
        defer resp.Body.Close()

        if resp.StatusCode != http.StatusOK {
            body, _ := io.ReadAll(resp.Body)
            fmt.Printf("Error: HTTP %d - %s\n", resp.StatusCode, string(body))
            return
        }

        var history []interface{}
        json.NewDecoder(resp.Body).Decode(&history)
        fmt.Println(history)
    },
}

// syncRetryCmd retries a sync
var syncRetryCmd = &cobra.Command{
    Use:   "retry [sync_id]",
    Short: "Retry a failed sync",
    Args:  cobra.ExactArgs(1),
    Run: func(cmd *cobra.Command, args []string) {
        syncID := args[0]

        server := GetServer(cmd)

        url := fmt.Sprintf("%s/api/v1/sync/retry/%s", server, syncID)
        req, err := http.NewRequest("POST", url, nil)
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            return
        }

        token := GetToken(cmd)
        if token != "" {
            req.Header.Set("Authorization", "Bearer "+token)
        }

        resp, err := http.DefaultClient.Do(req)
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            return
        }
        defer resp.Body.Close()

        if resp.StatusCode != http.StatusOK {
            body, _ := io.ReadAll(resp.Body)
            fmt.Printf("Error: HTTP %d - %s\n", resp.StatusCode, string(body))
            return
        }

        fmt.Println("Sync retry triggered")
    },
}

func init() {
    syncCmd.AddCommand(syncHistoryCmd)
    syncCmd.AddCommand(syncRetryCmd)

    syncHistoryCmd.Flags().String("repo_group", "", "Filter by repo group")
    syncHistoryCmd.Flags().Int("limit", 20, "Max number of records")

    RootCmd.AddCommand(syncCmd)
}
