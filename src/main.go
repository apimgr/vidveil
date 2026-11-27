// SPDX-License-Identifier: MIT
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/apimgr/vidveil/src/app"
	"github.com/apimgr/vidveil/src/config"
)

func main() {
	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		var flagErr config.FlagError
		if errors.As(err, &flagErr) {
			binaryName := filepath.Base(os.Args[0])
			fmt.Fprintf(os.Stderr, "%s: %v\nRun '%s --help' for usage.\n", binaryName, err, binaryName)
			os.Exit(2)
		}
		fmt.Fprintf(os.Stderr, "%s: configuration failed: %v\n", filepath.Base(os.Args[0]), err)
		os.Exit(78)
	}
	cfg.Version = Version
	cfg.CommitID = CommitID
	cfg.BuildEpoch = BuildEpoch
	cfg.OfficialSite = OfficialSite
	if err := app.Run(context.Background(), cfg); err != nil {
		var commandExit app.ExitError
		if errors.As(err, &commandExit) {
			if commandExit.Code != 0 {
				os.Exit(commandExit.Code)
			}
			return
		}
		fmt.Fprintf(os.Stderr, "Server failed: %v\n", err)
		os.Exit(1)
	}
}
