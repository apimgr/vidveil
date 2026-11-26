// SPDX-License-Identifier: MIT
// Package main regenerates embedded API specifications from package annotations.
package main

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/apimgr/vidveil/src/graphql"
	"github.com/apimgr/vidveil/src/swagger"
)

func main() {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot locate spec generator source")
	}
	sourceDir := filepath.Dir(sourceFile)
	if len(os.Args) == 2 && os.Args[1] == "--graphql-only" {
		write(filepath.Join(sourceDir, "../../graphql/schema_generated.graphql"), graphql.SchemaSource())
		return
	}
	write(filepath.Join(sourceDir, "../../swagger/openapi_generated.json"), swagger.GenerateSourceSpec())
	write(filepath.Join(sourceDir, "../../graphql/schema_generated.graphql"), graphql.SchemaSource())
}

func write(path, value string) {
	if err := os.WriteFile(path, []byte(value+"\n"), 0o644); err != nil {
		panic(err)
	}
}
