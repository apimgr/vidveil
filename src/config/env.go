// SPDX-License-Identifier: MIT
// Environment variable overrides per AI.md: all settings can be overridden via
// {PROJECT_NAME}_{SECTION}_{KEY} env vars, e.g. VIDVEIL_SERVER_PORT, VIDVEIL_DATABASE_TYPE.
package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/apimgr/vidveil/src/paths"
)

// EnvPrefix is the project env var prefix derived from the project name
var EnvPrefix = strings.ToUpper(paths.ProjectName) + "_"

// ApplyEnvOverrides walks the config struct and applies {PREFIX}_{PATH} env overrides.
// Two names are checked per field: the full yaml path (VIDVEIL_SERVER_DATABASE_TYPE) and,
// for fields under the server section, an alias without SERVER_ (VIDVEIL_DATABASE_TYPE).
// The full name wins when both are set. Invalid values warn and are ignored per AI.md PART 12.
func ApplyEnvOverrides(cfg *AppConfig) {
	applyEnvToStruct(reflect.ValueOf(cfg).Elem(), nil)
}

// applyEnvToStruct recurses through struct fields following yaml tags
func applyEnvToStruct(v reflect.Value, path []string) {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("yaml")
		// Skip runtime-only fields and untagged fields
		if tag == "" || tag == "-" {
			continue
		}
		// Strip yaml tag options like ",omitempty"
		name := strings.Split(tag, ",")[0]
		if name == "" || name == "-" {
			continue
		}
		fv := v.Field(i)
		fieldPath := append(append([]string{}, path...), name)
		if fv.Kind() == reflect.Struct {
			applyEnvToStruct(fv, fieldPath)
			continue
		}
		if val, ok := lookupEnvForPath(fieldPath); ok {
			setFieldFromEnv(fv, val, fieldPath)
		}
	}
}

// lookupEnvForPath checks the full-path env name, then the SERVER_-less alias
func lookupEnvForPath(path []string) (string, bool) {
	full := EnvPrefix + strings.ToUpper(strings.Join(path, "_"))
	if val, ok := os.LookupEnv(full); ok {
		return val, true
	}
	if len(path) > 1 && path[0] == "server" {
		alias := EnvPrefix + strings.ToUpper(strings.Join(path[1:], "_"))
		if val, ok := os.LookupEnv(alias); ok {
			return val, true
		}
	}
	return "", false
}

// setFieldFromEnv converts and assigns an env string to a config field
func setFieldFromEnv(fv reflect.Value, val string, path []string) {
	if !fv.CanSet() {
		return
	}
	name := EnvPrefix + strings.ToUpper(strings.Join(path, "_"))
	switch fv.Kind() {
	case reflect.String:
		fv.SetString(val)
	case reflect.Bool:
		b, err := ParseBool(val, false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: invalid boolean for %s: %q (ignored)\n", name, val)
			return
		}
		fv.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: invalid integer for %s: %q (ignored)\n", name, val)
			return
		}
		fv.SetInt(n)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: invalid number for %s: %q (ignored)\n", name, val)
			return
		}
		fv.SetFloat(f)
	case reflect.Slice:
		if fv.Type().Elem().Kind() == reflect.String {
			// Comma-separated list; empty value clears the slice
			var items []string
			for _, item := range strings.Split(val, ",") {
				if trimmed := strings.TrimSpace(item); trimmed != "" {
					items = append(items, trimmed)
				}
			}
			fv.Set(reflect.ValueOf(items))
		}
	}
}
