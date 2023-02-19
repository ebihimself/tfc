package cmd

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Upload the Terraform state for a workspace by name",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		workspaceName := args[0]

		workspaceID, err := getWorkspaceIDByName(workspaceName)
		if err != nil {
			return err
		}

		// Load Terraform state file
		stateFile, err := os.Open("state.tfstate")
		if err != nil {
			return err
		}
		defer stateFile.Close()

		stateBytes, err := ioutil.ReadAll(stateFile)
		if err != nil {
			return err
		}

		// Prepare request payload
		stateB64 := base64.StdEncoding.EncodeToString(stateBytes)
		payload := map[string]interface{}{
			"data": map[string]interface{}{
				"type": "state-versions",
				"attributes": map[string]interface{}{
					"md5":     fmt.Sprintf("%x", md5sum(stateBytes)),
					"state":   stateB64,
					"serial":  getSerialNumber(stateBytes),
					"lineage": getLineage(stateBytes),
				},
			},
		}

		payloadJSON, err := json.Marshal(payload)
		if err != nil {
			return err
		}

		// Send HTTP request to upload Terraform state file
		tfcToken, ok := os.LookupEnv("TFC_TOKEN")
		if !ok {
			return fmt.Errorf("TFC_TOKEN environment variable is not set")
		}
		if err := uploadTerraformState(workspaceID, tfcToken, payloadJSON); err != nil {
			return err
		}

		fmt.Println("Terraform state uploaded successfully.")

		return nil
	},
}

func uploadTerraformState(workspaceID, tfcToken string, payload []byte) error {
	// Send HTTP request to upload Terraform state file
	apiURL := fmt.Sprintf("https://app.terraform.io/api/v2/workspaces/%s/state-versions", workspaceID)
	req, err := http.NewRequest("POST", apiURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tfcToken)
	req.Header.Set("Content-Type", "application/vnd.api+json")
	req.Body = ioutil.NopCloser(bytes.NewReader(payload))
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("HTTP request failed with status code %d", resp.StatusCode)
	}

	releaseWorkspaceLock(workspaceID)

	return nil
}

func init() {
	rootCmd.AddCommand(pushCmd)
}

func md5sum(data []byte) []byte {
	h := md5.New()
	h.Write(data)
	return h.Sum(nil)
}

func getSerialNumber(data []byte) int {
	var obj map[string]interface{}
	err := json.Unmarshal(data, &obj)
	if err != nil {
		return 0
	}
	return int(obj["serial"].(float64))
}

func getLineage(data []byte) string {
	var obj map[string]interface{}
	err := json.Unmarshal(data, &obj)
	if err != nil {
		return ""
	}
	return obj["lineage"].(string)
}
