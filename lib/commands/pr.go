package commands

import (
    "encoding/json"
    "fmt"
    "net/http"

    "github.com/spf13/cobra"

    "asika/common/config"
)

// prCmd represents the pr command
var prCmd = &cobra.Command{
    Use:   "pr",
    Short: "Manage pull requests",
}

// prListCmd lists PRs
var prListCmd = &cobra.Command{
    Use:   "list [repo_group]",
    Short: "List pull requests",
    Args:  cobra.ExactArgs(1),
    Run: func(cmd *cobra.Command, args []string) {
        repoGroup := args[0]
        state, _ := cmd.Flags().GetString("state")
        platform, _ := cmd.Flags().GetString("platform")

        url := fmt.Sprintf("%s/api/v1/repos/%s/prs?state=%s&platform=%s", config.Current.Server.Listen, repoGroup, state, platform)
        resp, err := http.Get(url)
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            return
        }
        defer resp.Body.Close()

        if resp.StatusCode != http.StatusOK {
            fmt.Printf("Error: HTTP %d\n", resp.StatusCode)
            return
        }

        var prs []interface{}
        json.NewDecoder(resp.Body).Decode(&prs)
        fmt.Println(prs)
    },
}

// prShowCmd shows a PR
var prShowCmd = &cobra.Command{
    Use:   "show [repo_group] [pr_id]",
    Short: "Show pull request details",
    Args:  cobra.ExactArgs(2),
    Run: func(cmd *cobra.Command, args []string) {
        repoGroup := args[0]
        prID := args[1]

        url := fmt.Sprintf("%s/api/v1/repos/%s/prs/%s", config.Current.Server.Listen, repoGroup, prID)
        resp, err := http.Get(url)
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            return
        }
        defer resp.Body.Close()

        if resp.StatusCode != http.StatusOK {
            fmt.Printf("Error: HTTP %d\n", resp.StatusCode)
            return
        }

        var pr interface{}
        json.NewDecoder(resp.Body).Decode(&pr)
        fmt.Println(pr)
    },
}

// prApproveCmd approves a PR
var prApproveCmd = &cobra.Command{
    Use:   "approve [repo_group] [pr_id]",
    Short: "Approve a pull request",
    Args:  cobra.ExactArgs(2),
    Run: func(cmd *cobra.Command, args []string) {
        fmt.Println("PR approved")
    },
}

// prCloseCmd closes a PR
var prCloseCmd = &cobra.Command{
    Use:   "close [repo_group] [pr_id]",
    Short: "Close a pull request",
    Args:  cobra.ExactArgs(2),
    Run: func(cmd *cobra.Command, args []string) {
        fmt.Println("PR closed")
    },
}

// prReopenCmd reopens a PR
var prReopenCmd = &cobra.Command{
    Use:   "reopen [repo_group] [pr_id]",
    Short: "Reopen a pull request",
    Args:  cobra.ExactArgs(2),
    Run: func(cmd *cobra.Command, args []string) {
        fmt.Println("PR reopened")
    },
}

// prSpamCmd marks/unmarks spam
var prSpamCmd = &cobra.Command{
    Use:   "spam [repo_group] [pr_id]",
    Short: "Mark/unmark spam",
    Args:  cobra.ExactArgs(2),
    Run: func(cmd *cobra.Command, args []string) {
        undo, _ := cmd.Flags().GetBool("undo")
        if undo {
            fmt.Println("Spam mark removed")
        } else {
            fmt.Println("PR marked as spam")
        }
    },
}

// Add subcommands to prCmd
func init() {
    prCmd.AddCommand(prListCmd)
    prCmd.AddCommand(prShowCmd)
    prCmd.AddCommand(prApproveCmd)
    prCmd.AddCommand(prCloseCmd)
    prCmd.AddCommand(prReopenCmd)
    prCmd.AddCommand(prSpamCmd)

    prListCmd.Flags().String("state", "open", "Filter by state")
    prListCmd.Flags().String("platform", "", "Filter by platform")
    prSpamCmd.Flags().Bool("undo", false, "Remove spam mark")
}
