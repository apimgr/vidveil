// SPDX-License-Identifier: MIT
// AI.md PART 36: CLI Client
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/apimgr/vidveil/src/client/cmd"
)

// Build-time variables (set via -ldflags)
var (
	ProjectName = "vidveil"
	Version     = "dev"
	CommitID    = "unknown"
	BuildDate   = "unknown"
)

func main() {
	// Set build info for commands
	cmd.ProjectName = ProjectName
	cmd.Version = Version
	cmd.CommitID = CommitID
	cmd.BuildDate = BuildDate
	cmd.BinaryName = filepath.Base(os.Args[0])

	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
