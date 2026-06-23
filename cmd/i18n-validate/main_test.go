// SPDX-License-Identifier: MIT
package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTempLocale writes a locale map as JSON to dir/filename.
func writeTempLocale(t *testing.T, dir, filename string, m map[string]string) {
	t.Helper()
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal locale: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, filename), data, 0o644); err != nil {
		t.Fatalf("write locale %s: %v", filename, err)
	}
}

// makeValidDir creates a temp locale dir with a complete valid set.
func makeValidDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	en := map[string]string{
		"key.one":      "value one",
		"key.two":      "value %d two",
		"key.three":    "three %s end",
	}
	writeTempLocale(t, dir, "en.json", en)
	for _, name := range requiredLocales {
		writeTempLocale(t, dir, name, map[string]string{
			"key.one":   "tr one",
			"key.two":   "tr %d two",
			"key.three": "tr %s end",
		})
	}
	return dir
}

// ---- validate() tests ----

func TestValidate_AllLocalesValid(t *testing.T) {
	dir := makeValidDir(t)
	failures := validate(dir, "en.json", io.Discard)
	if failures != 0 {
		t.Errorf("expected 0 failures, got %d", failures)
	}
}

func TestValidate_CanonicalMissing(t *testing.T) {
	dir := t.TempDir()
	failures := validate(dir, "en.json", io.Discard)
	if failures == 0 {
		t.Error("expected failure for missing canonical file")
	}
}

func TestValidate_NoLocaleFiles(t *testing.T) {
	dir := t.TempDir()
	writeTempLocale(t, dir, "en.json", map[string]string{"k": "v"})
	var buf strings.Builder
	failures := validate(dir, "en.json", &buf)
	if failures == 0 {
		t.Error("expected failure when no locale files besides canonical")
	}
}

func TestValidate_RequiredLocaleMissing(t *testing.T) {
	dir := t.TempDir()
	writeTempLocale(t, dir, "en.json", map[string]string{"k": "v"})
	// Only add one locale (not all required)
	writeTempLocale(t, dir, "de.json", map[string]string{"k": "Wert"})
	failures := validate(dir, "en.json", io.Discard)
	if failures == 0 {
		t.Error("expected failures for missing required locales")
	}
}

func TestValidate_MissingKey(t *testing.T) {
	dir := t.TempDir()
	writeTempLocale(t, dir, "en.json", map[string]string{"key.a": "a", "key.b": "b"})
	for _, name := range requiredLocales {
		// ar.json only has key.a — missing key.b
		writeTempLocale(t, dir, name, map[string]string{"key.a": "translated a"})
	}
	failures := validate(dir, "en.json", io.Discard)
	if failures == 0 {
		t.Error("expected failure for missing key in locale")
	}
}

func TestValidate_ExtraKey(t *testing.T) {
	dir := t.TempDir()
	writeTempLocale(t, dir, "en.json", map[string]string{"key.a": "a"})
	for _, name := range requiredLocales {
		writeTempLocale(t, dir, name, map[string]string{"key.a": "tr a", "key.extra": "extra"})
	}
	failures := validate(dir, "en.json", io.Discard)
	if failures == 0 {
		t.Error("expected failure for extra key in locale")
	}
}

func TestValidate_EmptyValue(t *testing.T) {
	dir := t.TempDir()
	writeTempLocale(t, dir, "en.json", map[string]string{"key.a": "a"})
	for _, name := range requiredLocales {
		writeTempLocale(t, dir, name, map[string]string{"key.a": ""})
	}
	failures := validate(dir, "en.json", io.Discard)
	if failures == 0 {
		t.Error("expected failure for empty value in locale")
	}
}

func TestValidate_InterpolationMismatch(t *testing.T) {
	dir := t.TempDir()
	writeTempLocale(t, dir, "en.json", map[string]string{"key.a": "value %d"})
	for _, name := range requiredLocales {
		writeTempLocale(t, dir, name, map[string]string{"key.a": "translation %s"})
	}
	failures := validate(dir, "en.json", io.Discard)
	if failures == 0 {
		t.Error("expected failure for interpolation mismatch")
	}
}

func TestValidate_UnreadableDir(t *testing.T) {
	failures := validate("/nonexistent/path/xyz", "en.json", io.Discard)
	if failures == 0 {
		t.Error("expected failure for unreadable directory")
	}
}

func TestValidate_BadLocaleJSON(t *testing.T) {
	dir := t.TempDir()
	writeTempLocale(t, dir, "en.json", map[string]string{"key.a": "a"})
	for _, name := range requiredLocales {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(`{invalid`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	failures := validate(dir, "en.json", io.Discard)
	if failures == 0 {
		t.Error("expected failure for invalid locale JSON")
	}
}

// ---- loadLocale() tests ----

func TestLoadLocale_ValidFile(t *testing.T) {
	dir := t.TempDir()
	m := map[string]string{"key.one": "value one", "key.two": "value two"}
	writeTempLocale(t, dir, "test.json", m)
	got, err := loadLocale(filepath.Join(dir, "test.json"))
	if err != nil {
		t.Fatalf("loadLocale error: %v", err)
	}
	if len(got) != len(m) {
		t.Errorf("expected %d keys, got %d", len(m), len(got))
	}
}

func TestLoadLocale_MissingFile(t *testing.T) {
	_, err := loadLocale(filepath.Join(t.TempDir(), "nonexistent.json"))
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestLoadLocale_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte(`{not valid json`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := loadLocale(path)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestLoadLocale_EmptyObject(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(path, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := loadLocale(path)
	if err != nil {
		t.Fatalf("loadLocale error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 keys, got %d", len(got))
	}
}

// ---- equalVarSets() tests ----

func TestEqualVarSets_BothNil(t *testing.T) {
	if !equalVarSets(nil, nil) {
		t.Error("both nil should be equal")
	}
}

func TestEqualVarSets_BothEmpty(t *testing.T) {
	if !equalVarSets([]string{}, []string{}) {
		t.Error("both empty should be equal")
	}
}

func TestEqualVarSets_SameSingleVar(t *testing.T) {
	if !equalVarSets([]string{"%d"}, []string{"%d"}) {
		t.Error("identical single-element slices should be equal")
	}
}

func TestEqualVarSets_SameMultipleVarsReordered(t *testing.T) {
	if !equalVarSets([]string{"%d", "%s"}, []string{"%s", "%d"}) {
		t.Error("same elements in different order should be equal")
	}
}

func TestEqualVarSets_DuplicateVarsMatch(t *testing.T) {
	if !equalVarSets([]string{"%d", "%d"}, []string{"%d", "%d"}) {
		t.Error("identical duplicate vars should be equal")
	}
}

func TestEqualVarSets_LengthMismatch(t *testing.T) {
	if equalVarSets([]string{"%d"}, []string{"%d", "%s"}) {
		t.Error("different lengths should not be equal")
	}
}

func TestEqualVarSets_DifferentVarTypes(t *testing.T) {
	if equalVarSets([]string{"%d"}, []string{"%s"}) {
		t.Error("different var types should not be equal")
	}
}

func TestEqualVarSets_ExtraInSecond(t *testing.T) {
	if equalVarSets([]string{"%d", "%d"}, []string{"%d", "%s"}) {
		t.Error("mismatched var types should not be equal")
	}
}

func TestEqualVarSets_EmptyVsNonEmpty(t *testing.T) {
	if equalVarSets([]string{"%d"}, nil) {
		t.Error("non-empty vs nil should not be equal")
	}
	if equalVarSets(nil, []string{"%d"}) {
		t.Error("nil vs non-empty should not be equal")
	}
}
