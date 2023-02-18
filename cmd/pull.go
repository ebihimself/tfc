package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Download the Terraform state for a workspace by name",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		workspaceName := args[0]

		// Load existing workspaces
		workspaces, err := loadWorkspaces()
		if err != nil {
			return err
		}

		// Find workspace by name
		var workspaceID string
		for _, w := range workspaces {
			if w.Name == workspaceName {
				workspaceID = w.WorkspaceID
				break
			}
		}
		if workspaceID == "" {
			return fmt.Errorf("workspace with the name %s not found", workspaceName)
		}

		// Download the Terraform state for the specified workspace
		tfcToken, ok := os.LookupEnv("TFC_TOKEN")
		if !ok {
			return fmt.Errorf("TFC_TOKEN environment variable is not set")
		}
		if err := downloadTerraformState(workspaceID, tfcToken); err != nil {
			return err
		}

		fmt.Println("Terraform state downloaded successfully.")

		return nil
	},
}

func downloadTerraformState(workspaceID, tfcToken string) error {
	// Get the hosted state download URL from the Terraform API
	apiURL := fmt.Sprintf("https://app.terraform.io/api/v2/workspaces/%s/current-state-version", workspaceID)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tfcToken)
	req.Header.Set("Content-Type", "application/vnd.api+json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP request failed with status code %d", resp.StatusCode)
	}

	// Parse the hosted state download URL from the Terraform API response
	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return err
	}
	url, ok := data["data"].(map[string]interface{})["attributes"].(map[string]interface{})["hosted-state-download-url"].(string)
	if !ok {
		return fmt.Errorf("Failed to parse hosted state download URL from API response")
	}

	// Download the Terraform state file
	resp, err = http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	file, err := os.Create("state.tfstate")
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func init() {
	rootCmd.AddCommand(pullCmd)
}
