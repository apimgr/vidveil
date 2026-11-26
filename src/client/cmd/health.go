// SPDX-License-Identifier: MIT
// AI.md PART 32: Client health command
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
)

// RunHealthCommand verifies that the configured server health endpoint responds successfully.
func RunHealthCommand(args []string) error {
	for _, argument := range args {
		if argument == "--help" || argument == "-h" {
			fmt.Printf("Check server health\n\nUsage:\n  %s health\n", BinaryName)
			return nil
		}
	}
	if apiClient == nil {
		return fmt.Errorf("server is not configured")
	}
	healthy, err := apiClient.Health()
	if err != nil {
		return err
	}
	if !healthy {
		return fmt.Errorf("server health check failed")
	}
	if cliConfig != nil && cliConfig.Output.Format == "json" {
		return json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "healthy"})
	}
	fmt.Println("Server is healthy")
	return nil
}
