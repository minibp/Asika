package commands

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"

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
        repoGroup := args[0]

        server := GetServer(cmd)

        url := fmt.Sprintf("%s/api/v1/queue/%s", server, repoGroup)
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

        var queue []interface{}
        json.NewDecoder(resp.Body).Decode(&queue)
        fmt.Println(queue)
    },
}

// queueRecheckCmd triggers recheck
var queueRecheckCmd = &cobra.Command{
    Use:   "recheck [repo_group]",
    Short: "Trigger queue recheck",
    Args:  cobra.ExactArgs(1),
    Run: func(cmd *cobra.Command, args []string) {
        repoGroup := args[0]

        server := GetServer(cmd)

        url := fmt.Sprintf("%s/api/v1/queue/%s/recheck", server, repoGroup)
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

        fmt.Println("Queue recheck triggered")
    },
}

func init() {
    queueCmd.AddCommand(queueListCmd)
    queueCmd.AddCommand(queueRecheckCmd)

    RootCmd.AddCommand(queueCmd)
}
