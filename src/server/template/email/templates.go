// SPDX-License-Identifier: MIT
// Package emailtemplates provides the built-in notification templates.
package emailtemplates

import (
	"embed"
	"io/fs"
	"path/filepath"
	"sort"
)

// Files contains the default email templates embedded in the server binary.
//
//go:embed *.txt
var Files embed.FS

// Names returns the embedded template names without their file extensions.
func Names() []string {
	entries, err := fs.ReadDir(Files, ".")
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".txt" {
			names = append(names, entry.Name()[:len(entry.Name())-len(filepath.Ext(entry.Name()))])
		}
	}
	sort.Strings(names)
	return names
}
