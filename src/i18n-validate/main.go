// SPDX-License-Identifier: MIT
// i18n-validate: validates all locale JSON files against the canonical en.json.
// Run via: go run ./src/i18n-validate/main.go
// Or via: make i18n-validate
//
// Checks performed per AI.md FINAL CHECKPOINT:
//   - All locale files have identical key sets to en.json (no missing, no extra keys)
//   - No empty string values in any locale file
//   - All interpolation variables (%d, %s, %v, %f, etc.) match en.json for each key
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	localeDir = "src/common/i18n/locales"
	canonical = "en.json"
)

// requiredLocales lists the locale files that MUST be present per AI.md PART 30.
var requiredLocales = []string{"ar.json", "de.json", "es.json", "fr.json", "ja.json", "zh.json"}

// interpolationRe matches printf-style format verbs used in i18n strings.
var interpolationRe = regexp.MustCompile(`%[+\-# 0-9*.]*[diouxXeEfFgGtTsqpvw%]`)

func main() {
	failures := validate(localeDir, canonical, os.Stderr)
	if failures > 0 {
		fmt.Fprintf(os.Stderr, "\ni18n validation FAILED: %d error(s)\n", failures)
		os.Exit(1)
	}
	fmt.Printf("i18n validation OK: all locale files match %s\n", canonical)
}

// validate validates all locale JSON files in dir against the canonical locale file.
// Errors are written to w. Returns the number of validation failures.
func validate(dir, canon string, w io.Writer) int {
	enPath := filepath.Join(dir, canon)
	enKeys, err := loadLocale(enPath)
	if err != nil {
		fmt.Fprintf(w, "error: cannot load %s: %v\n", enPath, err)
		return 1
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(w, "error: cannot read locale dir %s: %v\n", dir, err)
		return 1
	}

	var locales []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") && e.Name() != canon {
			locales = append(locales, e.Name())
		}
	}
	sort.Strings(locales)

	if len(locales) == 0 {
		fmt.Fprintf(w, "error: no locale files found in %s\n", dir)
		return 1
	}

	localeSet := make(map[string]bool, len(locales))
	for _, l := range locales {
		localeSet[l] = true
	}
	failures := 0
	for _, r := range requiredLocales {
		if !localeSet[r] {
			fmt.Fprintf(w, "error: required locale file missing: %s\n", r)
			failures++
		}
	}

	for _, locale := range locales {
		path := filepath.Join(dir, locale)
		keys, err := loadLocale(path)
		if err != nil {
			fmt.Fprintf(w, "[%s] error: cannot load: %v\n", locale, err)
			failures++
			continue
		}

		// Check for missing keys
		for k := range enKeys {
			if _, ok := keys[k]; !ok {
				fmt.Fprintf(w, "[%s] MISSING key: %q\n", locale, k)
				failures++
			}
		}

		// Check for extra keys
		for k := range keys {
			if _, ok := enKeys[k]; !ok {
				fmt.Fprintf(w, "[%s] EXTRA key (not in %s): %q\n", locale, canon, k)
				failures++
			}
		}

		// Check for empty values and interpolation variable parity
		for k, v := range keys {
			if strings.TrimSpace(v) == "" {
				fmt.Fprintf(w, "[%s] EMPTY value for key: %q\n", locale, k)
				failures++
				continue
			}
			enVal, inEN := enKeys[k]
			if !inEN {
				continue
			}
			enVars := interpolationRe.FindAllString(enVal, -1)
			locVars := interpolationRe.FindAllString(v, -1)
			if !equalVarSets(enVars, locVars) {
				fmt.Fprintf(w, "[%s] INTERPOLATION MISMATCH key %q: en=%v locale=%v\n",
					locale, k, enVars, locVars)
				failures++
			}
		}
	}

	return failures
}

// loadLocale reads a JSON locale file and returns a key→value map.
func loadLocale(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return m, nil
}

// equalVarSets returns true when two slices contain the same multiset of format verbs.
func equalVarSets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	count := make(map[string]int, len(a))
	for _, v := range a {
		count[v]++
	}
	for _, v := range b {
		count[v]--
		if count[v] < 0 {
			return false
		}
	}
	return true
}
