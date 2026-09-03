// SPDX-License-Identifier: MIT
package cmd

import "strings"

// parseCLILongFlagArgument normalizes a long-form CLI flag and extracts any inline value.
func parseCLILongFlagArgument(flagArgument string) (string, string, bool) {
	if !strings.HasPrefix(flagArgument, "--") {
		return flagArgument, "", false
	}

	flagParts := strings.SplitN(flagArgument, "=", 2)
	if len(flagParts) != 2 {
		return flagArgument, "", false
	}

	return flagParts[0], flagParts[1], true
}

// readCLILongFlagValue reads a long-form CLI flag value from either --flag=value or --flag value syntax.
func readCLILongFlagValue(args []string, currentIndex int) (string, int, bool) {
	_, inlineFlagValue, hasInlineFlagValue := parseCLILongFlagArgument(args[currentIndex])
	if hasInlineFlagValue {
		return inlineFlagValue, currentIndex, true
	}

	if currentIndex+1 >= len(args) {
		return "", currentIndex, false
	}

	return args[currentIndex+1], currentIndex + 1, true
}
