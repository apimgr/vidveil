// SPDX-License-Identifier: MIT
// AI.md PART 33: CLI Client - Shared Output Helpers
package cmd

import (
	"encoding/csv"
	"encoding/json"
	"os"

	"gopkg.in/yaml.v3"
)

// OutputDataAsJSON writes indented JSON output to stdout.
func OutputDataAsJSON(outputData interface{}) error {
	jsonEncoder := json.NewEncoder(os.Stdout)
	jsonEncoder.SetIndent("", "  ")
	return jsonEncoder.Encode(outputData)
}

// OutputDataAsYAML writes YAML output to stdout.
func OutputDataAsYAML(outputData interface{}) error {
	yamlEncoder := yaml.NewEncoder(os.Stdout)
	defer yamlEncoder.Close()
	return yamlEncoder.Encode(outputData)
}

// OutputDataAsCSV writes a CSV header row followed by data rows to stdout.
func OutputDataAsCSV(headerRow []string, dataRows [][]string) error {
	csvWriter := csv.NewWriter(os.Stdout)
	if err := csvWriter.Write(headerRow); err != nil {
		return err
	}
	for _, row := range dataRows {
		if err := csvWriter.Write(row); err != nil {
			return err
		}
	}
	csvWriter.Flush()
	return csvWriter.Error()
}
