// SPDX-License-Identifier: MIT
// AI.md PART 32: CLI Client
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/apimgr/vidveil/src/client/cmd"
)

// Build-time variables (set via -ldflags per AI.md PART 25)
// BuildEpoch is the single captured time source (Unix seconds, UTC) - BuildDate
// is NOT embedded via ldflags, it is derived from BuildEpoch at process start.
var (
	ProjectName  = "vidveil"
	Version      = "dev"
	CommitID     = "unknown"
	BuildEpoch   = "0"
	BuildDate    = "unknown"
	OfficialSite = ""
)

// buildEpoch parses BuildEpoch into a Unix timestamp, returning 0 on any
// parse failure (e.g. the default "0" or a dev build with no ldflags set).
func buildEpoch() int64 {
	n, err := strconv.ParseInt(BuildEpoch, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func main() {
	// Derive BuildDate from the embedded BuildEpoch per AI.md PART 25.
	if n := buildEpoch(); n > 0 {
		BuildDate = time.Unix(n, 0).UTC().Format("2006-01-02T15:04:05Z")
	}
	// Set build info for commands
	cmd.ProjectName = ProjectName
	cmd.Version = Version
	cmd.CommitID = CommitID
	cmd.BuildDate = BuildDate
	cmd.OfficialSite = OfficialSite
	cmd.BinaryName = filepath.Base(os.Args[0])

	if err := cmd.ExecuteCLI(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
