package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
)

var userCmd = &cobra.Command{
	Use:   "user",
	Short: "Manage users",
}

var userListCmd = &cobra.Command{
	Use:   "list",
	Short: "List users",
	Run: func(cmd *cobra.Command, args []string) {
		url := fmt.Sprintf("%s/api/v1/users", GetServer(cmd))
		resp := doRequest("GET", url, cmd)
		if resp == nil {
			return
		}
		_ = handleResponse(resp, "No users found")
	},
}

var userAddCmd = &cobra.Command{
	Use:   "add <username>",
	Short: "Add a user",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		password, _ := cmd.Flags().GetString("password")
		role, _ := cmd.Flags().GetString("role")

		if password == "" {
			fmt.Println("Error: --password is required")
			return
		}
		if role == "" {
			role = "viewer"
		}

		body, _ := json.Marshal(map[string]string{
			"username": args[0],
			"password": password,
			"role":     role,
		})

		url := fmt.Sprintf("%s/api/v1/users", GetServer(cmd))
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		if token := GetToken(cmd); token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		handleWriteResponse(resp, "User added successfully")
	},
}

var userDeleteCmd = &cobra.Command{
	Use:   "delete <username>",
	Short: "Delete a user",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		url := fmt.Sprintf("%s/api/v1/users/%s", GetServer(cmd), args[0])
		req, err := http.NewRequest("DELETE", url, nil)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		if token := GetToken(cmd); token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		handleWriteResponse(resp, "User deleted successfully")
	},
}

func init() {
	userCmd.AddCommand(userListCmd)
	userCmd.AddCommand(userAddCmd)
	userCmd.AddCommand(userDeleteCmd)

	userAddCmd.Flags().StringP("password", "p", "", "User password (required)")
	userAddCmd.Flags().StringP("role", "r", "viewer", "Role: admin, operator, viewer")

	RootCmd.AddCommand(userCmd)
}
