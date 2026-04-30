package commands

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"

    "github.com/BurntSushi/toml"
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

// configSetCmd sets config by sending to server
var configSetCmd = &cobra.Command{
    Use:   "set",
    Short: "Set configuration (reads from file or stdin, sends to server)",
    Run: func(cmd *cobra.Command, args []string) {
        server, _ := cmd.Flags().GetString("server")
        if server == "" {
            server = "http://localhost:8080"
        }

        token, _ := cmd.Flags().GetString("token")
        if token == "" {
            token = os.Getenv("ASIKA_TOKEN")
        }

        // Read config from file or stdin
        var inputData []byte
        filePath, _ := cmd.Flags().GetString("file")
        if filePath != "" {
            data, err := os.ReadFile(filePath)
            if err != nil {
                fmt.Printf("Error reading file: %v\n", err)
                return
            }
            inputData = data
        } else {
            // Read from stdin
            data, err := io.ReadAll(os.Stdin)
            if err != nil {
                fmt.Printf("Error reading stdin: %v\n", err)
                return
            }
            if len(data) == 0 {
                fmt.Println("Error: no input provided. Use --file or pipe TOML content to stdin")
                return
            }
            inputData = data
        }

        // Parse TOML to validate
        var cfg map[string]interface{}
        if err := toml.Unmarshal(inputData, &cfg); err != nil {
            fmt.Printf("Error: invalid TOML format: %v\n", err)
            return
        }

        // Convert to JSON for API
        jsonData, err := json.Marshal(cfg)
        if err != nil {
            fmt.Printf("Error: failed to convert to JSON: %v\n", err)
            return
        }

        // Send to server
        url := fmt.Sprintf("%s/api/v1/config", server)
        req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            return
        }
        req.Header.Set("Content-Type", "application/json")
        if token != "" {
            req.Header.Set("Authorization", "Bearer "+token)
        }

        resp, err := http.DefaultClient.Do(req)
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            return
        }
        defer resp.Body.Close()

        body, _ := io.ReadAll(resp.Body)
        if resp.StatusCode != http.StatusOK {
            fmt.Printf("Error: HTTP %d - %s\n", resp.StatusCode, string(body))
            return
        }

        fmt.Printf("Config updated successfully: %s\n", string(body))
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
    configCmd.AddCommand(configSetCmd)
    configCmd.AddCommand(configReloadCmd)

    configSetCmd.Flags().String("file", "", "Path to TOML config file (if not provided, reads from stdin)")
}
