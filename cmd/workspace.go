package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

type workspace struct {
	Name        string `json:"name"`
	WorkspaceID string `json:"workspace_id"`
}

var workspaceID string
var workspaceName string

var newWorkspaceCmd = &cobra.Command{
	Use:   "new",
	Short: "Add a new workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		if workspaceID == "" {
			return fmt.Errorf("workspace ID is required")
		}
		if workspaceName == "" {
			return fmt.Errorf("workspace name is required")
		}

		// Load existing workspaces
		workspaces, err := loadWorkspaces()
		if err != nil {
			return err
		}

		// Check if workspace with the same name already exists
		for _, w := range workspaces {
			if w.Name == workspaceName {
				return fmt.Errorf("workspace with the name %s already exists", workspaceName)
			}
		}

		// Add the new workspace
		workspaces = append(workspaces, workspace{
			Name:        workspaceName,
			WorkspaceID: workspaceID,
		})

		// Save the updated workspaces
		if err := saveWorkspaces(workspaces); err != nil {
			return err
		}

		fmt.Printf("Workspace %s added successfully.\n", workspaceName)

		return nil
	},
}

func getWorkspaceIDByName(workspaceName string) (string, error) {
	workspaces, err := loadWorkspaces()
	if err != nil {
		return "", err
	}

	var workspaceID string
	found := false
	for _, workspace := range workspaces {
		if fmt.Sprint(workspace.Name) == workspaceName {
			workspaceID = workspace.WorkspaceID
			found = true
			break
		}
	}

	if !found {
		return "", fmt.Errorf("workspace %s not found", workspaceName)
	}

	return workspaceID, nil
}

func loadWorkspaces() ([]workspace, error) {
	workspaceFile := getWorkspaceFile()
	data, err := ioutil.ReadFile(workspaceFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var workspaces []workspace
	err = json.Unmarshal(data, &workspaces)
	if err != nil {
		return nil, err
	}
	return workspaces, nil
}

func saveWorkspaces(workspaces []workspace) error {
	workspaceFile := getWorkspaceFile()
	data, err := json.MarshalIndent(workspaces, "", "  ")
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(workspaceFile, data, 0644)
	if err != nil {
		return err
	}
	return nil
}

func getWorkspaceFile() string {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return filepath.Join(dir, ".workspaces")
}

func acquireWorkspaceLock(workspaceID string) (bool, error) {
	tfcToken, ok := os.LookupEnv("TFC_TOKEN")
	if !ok {
		return false, fmt.Errorf("TFC_TOKEN environment variable is not set")
	}

	lockURL := fmt.Sprintf("https://app.terraform.io/api/v2/workspaces/%s/actions/lock", workspaceID)
	lockRequest, err := http.NewRequestWithContext(context.Background(), "POST", lockURL, nil)
	if err != nil {
		return false, err
	}
	lockRequest.Header.Add("Authorization", "Bearer "+tfcToken)
	lockRequest.Header.Set("Content-Type", "application/vnd.api+json")

	client := &http.Client{}
	lockResponse, err := client.Do(lockRequest)
	if err != nil {
		return false, err
	}
	defer lockResponse.Body.Close()

	if lockResponse.StatusCode != 200 {
		return false, fmt.Errorf("failed to acquire lock on workspace %s (status code %d)", workspaceID, lockResponse.StatusCode)
	}

	return true, nil
}

func releaseWorkspaceLock(workspaceID string) error {
	tfcToken, ok := os.LookupEnv("TFC_TOKEN")
	if !ok {
		return fmt.Errorf("TFC_TOKEN environment variable is not set")
	}

	unlockURL := fmt.Sprintf("https://app.terraform.io/api/v2/workspaces/%s/actions/unlock", workspaceID)
	unlockRequest, err := http.NewRequestWithContext(context.Background(), "POST", unlockURL, nil)
	if err != nil {
		return err
	}
	unlockRequest.Header.Add("Authorization", "Bearer "+tfcToken)
	unlockRequest.Header.Add("Content-Type", "application/vnd.api+json")

	unlockRequestData := map[string]interface{}{
		"data": map[string]interface{}{
			"type": "actions",
			"attributes": map[string]interface{}{
				"reason": "Lock released by Terraform CLI",
			},
			"relationships": map[string]interface{}{
				"lock": map[string]interface{}{
					"data": map[string]string{
						"type": "workspace-lock",
					},
				},
			},
		},
	}

	unlockRequestBody, err := json.Marshal(unlockRequestData)
	if err != nil {
		return err
	}

	unlockRequest.Body = ioutil.NopCloser(bytes.NewReader(unlockRequestBody))

	client := &http.Client{}
	unlockResponse, err := client.Do(unlockRequest)
	if err != nil {
		return err
	}
	defer unlockResponse.Body.Close()

	if unlockResponse.StatusCode != 200 {
		return fmt.Errorf("failed to release lock on workspace %s (status code %d)", workspaceID, unlockResponse.StatusCode)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(newWorkspaceCmd)

	newWorkspaceCmd.Flags().StringVar(&workspaceID, "workspace-id", "", "Terraform Cloud workspace ID")
	newWorkspaceCmd.Flags().StringVar(&workspaceName, "workspace-name", "", "Terraform Cloud workspace name")
	newWorkspaceCmd.MarkFlagRequired("workspace-id")
	newWorkspaceCmd.MarkFlagRequired("workspace-name")
}
