// SPDX-License-Identifier: MIT
package cmd

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/client/api"
)

func TestCLIMinimumVersionError(t *testing.T) {
	tests := []struct {
		name               string
		current            string
		minimum            string
		wantError          bool
		wantMessageVersion string
	}{
		{name: "minimum unset", current: "1.0.0", wantError: false},
		{name: "at minimum", current: "1.2.0", minimum: "1.2.0", wantError: false},
		{name: "above minimum", current: "1.3.0", minimum: "1.2.0", wantError: false},
		{name: "below minimum", current: "1.1.9", minimum: "v1.2.0", wantError: true, wantMessageVersion: "1.2.0"},
		{name: "development build below minimum", current: "dev", minimum: "1.2.0", wantError: true, wantMessageVersion: "1.2.0"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := cliMinimumVersionError(test.current, test.minimum)
			if (err != nil) != test.wantError {
				t.Fatalf("cliMinimumVersionError() = %v, want error %t", err, test.wantError)
			}
			if err != nil && (!strings.Contains(err.Error(), test.wantMessageVersion) || !strings.Contains(err.Error(), "--update yes")) {
				t.Fatalf("minimum-version message does not include required version and upgrade command: %v", err)
			}
		})
	}
}

func TestShouldAutoUpdateCLI(t *testing.T) {
	if !shouldAutoUpdateCLI(true, true, false) {
		t.Fatal("non-interactive opted-in update should install automatically")
	}
	if shouldAutoUpdateCLI(true, false, false) {
		t.Fatal("non-interactive update should not install without opt-in")
	}
	if shouldAutoUpdateCLI(true, true, true) {
		t.Fatal("interactive update should prompt instead of silently installing")
	}
	if shouldAutoUpdateCLI(false, true, false) {
		t.Fatal("current CLI should not update")
	}
}

func TestUpdatePermissionDetection(t *testing.T) {
	wrappedPermissionError := errors.Join(errors.New("move failed"), os.ErrPermission)
	if !isCLIBinaryPermissionError(wrappedPermissionError) {
		t.Fatal("wrapped access-denied error should be recognized as a permission failure")
	}
}

func TestPromptForCLIUpdate(t *testing.T) {
	accepted, err := promptForCLIUpdate(strings.NewReader("yes\n"), &strings.Builder{})
	if err != nil || !accepted {
		t.Fatalf("yes response = (%t, %v), want accepted", accepted, err)
	}
	accepted, err = promptForCLIUpdate(strings.NewReader("no\n"), &strings.Builder{})
	if err != nil || accepted {
		t.Fatalf("no response = (%t, %v), want declined", accepted, err)
	}
}

func TestCheckCLIUpdateBeforeRequestsBlocksBelowMinimum(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/autodiscover" || r.URL.Query().Get("channel") != "stable" {
			t.Errorf("unexpected bootstrap request: %s", r.URL.String())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"cli_min_version":"2.0.0","cli_versions":{}}`))
	}))
	defer server.Close()

	previousClient, previousConfig := apiClient, cliConfig
	apiClient = api.NewAPIClient(server.URL, "", 5, "v1")
	cliConfig = &CLIConfig{Server: CLIServerConfig{Address: server.URL}, Update: CLIUpdateConfig{Channel: "stable"}}
	t.Cleanup(func() { apiClient, cliConfig = previousClient, previousConfig })

	err := checkCLIUpdateBeforeRequests(false)
	if err == nil || !strings.Contains(err.Error(), "requires 2.0.0") {
		t.Fatalf("checkCLIUpdateBeforeRequests() = %v, want minimum-version error", err)
	}
}
