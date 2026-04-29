package commands

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"

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
        server, _ := cmd.Flags().GetString("server")
        if server == "" {
            server = "http://localhost:8080"
        }

        url := fmt.Sprintf("%s/api/v1/config", server)
        req, err := http.NewRequest("GET", url, nil)
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            return
        }

        token, _ := cmd.Flags().GetString("token")
        if token == "" {
            token = os.Getenv("ASIKA_TOKEN")
        }
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

        var config interface{}
        json.NewDecoder(resp.Body).Decode(&config)
        fmt.Println(config)
    },
}

// configReloadCmd triggers config reload
var configReloadCmd = &cobra.Command{
    Use:   "reload",
    Short: "Trigger config hot reload",
    Run: func(cmd *cobra.Command, args []string) {
        server, _ := cmd.Flags().GetString("server")
        if server == "" {
            server = "http://localhost:8080"
        }

        url := fmt.Sprintf("%s/api/v1/config", server)
        req, err := http.NewRequest("PUT", url, nil)
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            return
        }

        token, _ := cmd.Flags().GetString("token")
        if token == "" {
            token = os.Getenv("ASIKA_TOKEN")
        }
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

        fmt.Println("Config reload triggered")
    },
}

func init() {
    configCmd.AddCommand(configShowCmd)
    configCmd.AddCommand(configReloadCmd)
}
