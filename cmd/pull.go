package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:   "pull [workspace name]",
	Short: "Pull the Terraform state file for a workspace",
	Long:  "Pull the Terraform state file for a workspace.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get workspace ID from .workspaces file
		workspaceName := args[0]
		workspaceID, err := getWorkspaceIDByName(workspaceName)
		if err != nil {
			return err
		}

		// Acquire workspace lock
		_, err = acquireWorkspaceLock(workspaceID)
		if err != nil {
			return err
		}

		// Download Terraform state file
		tfcToken, ok := os.LookupEnv("TFC_TOKEN")
		if !ok {
			return fmt.Errorf("TFC_TOKEN environment variable is not set")
		}
		err = downloadTerraformState(workspaceID, tfcToken)
		if err != nil {
			return err
		}

		// Print success message
		fmt.Printf("Successfully pulled Terraform state file for workspace %s (ID %s)\n", workspaceName, workspaceID)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(pullCmd)
}

func downloadTerraformState(workspaceID string, tfcToken string) error {
	// Get download URL for current state version
	downloadURL, err := getCurrentStateDownloadURL(workspaceID, tfcToken)
	if err != nil {
		return err
	}
	// Download state file
	resp, err := http.Get(downloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create output file
	filename := "state.tfstate"
	err = os.MkdirAll(filepath.Dir(filename), 0755)
	if err != nil {
		return err
	}
	out, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write state file to output
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func getCurrentStateDownloadURL(workspaceID string, tfcToken string) (string, error) {
	url := fmt.Sprintf("https://app.terraform.io/api/v2/workspaces/%s/current-state-version", workspaceID)
	req, err := http.NewRequestWithContext(context.Background(), "GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Authorization", "Bearer "+tfcToken)
	req.Header.Add("Content-Type", "application/vnd.api+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to get current state version for workspace %s (status code %d)", workspaceID, resp.StatusCode)
	}

	var stateVersionData map[string]interface{}
	stateVersionBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(stateVersionBody, &stateVersionData)
	if err != nil {
		return "", err
	}

	downloadURL := stateVersionData["data"].(map[string]interface{})["attributes"].(map[string]interface{})["hosted-state-download-url"].(string)

	return downloadURL, nil
}
