package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
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

func init() {
	rootCmd.AddCommand(newWorkspaceCmd)

	newWorkspaceCmd.Flags().StringVar(&workspaceID, "workspace-id", "", "Terraform Cloud workspace ID")
	newWorkspaceCmd.Flags().StringVar(&workspaceName, "workspace-name", "", "Terraform Cloud workspace name")
	newWorkspaceCmd.MarkFlagRequired("workspace-id")
	newWorkspaceCmd.MarkFlagRequired("workspace-name")
}
