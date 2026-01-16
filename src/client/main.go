// SPDX-License-Identifier: MIT
// AI.md PART 36: CLI Client
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/apimgr/vidveil/src/client/cmd"
	"github.com/apimgr/vidveil/src/client/paths"
)

// Build-time variables (set via -ldflags)
var (
	ProjectName = "vidveil"
	Version     = "dev"
	CommitID    = "unknown"
	BuildDate   = "unknown"
)

func main() {
	// Per AI.md PART 36: Ensure directories exist on every startup
	// Creates config, data, cache, log dirs with 0700 permissions
	if err := paths.EnsureClientDirs(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to create directories: %v\n", err)
	}

	// Set build info for commands
	cmd.ProjectName = ProjectName
	cmd.Version = Version
	cmd.CommitID = CommitID
	cmd.BuildDate = BuildDate
	cmd.BinaryName = filepath.Base(os.Args[0])

	if err := cmd.ExecuteCLI(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
