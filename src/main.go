// SPDX-License-Identifier: MIT
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/apimgr/vidveil/src/app"
	"github.com/apimgr/vidveil/src/config"
)

func main() {
	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Server configuration failed: %v\n", err)
		os.Exit(1)
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
