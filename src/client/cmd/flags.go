// SPDX-License-Identifier: MIT
package cmd

import "strings"

// ParseCLILongFlagArgument normalizes a long-form CLI flag and extracts any inline value.
func ParseCLILongFlagArgument(flagArgument string) (string, string, bool) {
	if !strings.HasPrefix(flagArgument, "--") {
		return flagArgument, "", false
	}

	flagParts := strings.SplitN(flagArgument, "=", 2)
	if len(flagParts) != 2 {
		return flagArgument, "", false
	}

	return flagParts[0], flagParts[1], true
}

// ReadCLILongFlagValue reads a long-form CLI flag value from either --flag=value or --flag value syntax.
func ReadCLILongFlagValue(args []string, currentIndex int) (string, int, bool) {
	_, inlineFlagValue, hasInlineFlagValue := ParseCLILongFlagArgument(args[currentIndex])
	if hasInlineFlagValue {
		return inlineFlagValue, currentIndex, true
	}

	if currentIndex+1 >= len(args) {
		return "", currentIndex, false
	}

	return args[currentIndex+1], currentIndex + 1, true
}
