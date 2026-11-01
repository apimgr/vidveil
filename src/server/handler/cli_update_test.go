// SPDX-License-Identifier: MIT
package handler

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type cliUpdateRoundTripper func(*http.Request) (*http.Response, error)

func (roundTripper cliUpdateRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTripper(request)
}

func TestFetchCLIReleaseMetadataGitHub404MeansNoUpdates(t *testing.T) {
	previousClient := cliReleaseHTTPClient
	cliReleaseHTTPClient = &http.Client{Transport: cliUpdateRoundTripper(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(strings.NewReader("not found")),
			Header:     make(http.Header),
		}, nil
	})}
	t.Cleanup(func() { cliReleaseHTTPClient = previousClient })

	metadata, err := fetchCLIReleaseMetadata(context.Background(), "stable")
	if err != nil {
		t.Fatalf("fetchCLIReleaseMetadata() error = %v, want no update", err)
	}
	if len(metadata.Versions) != 0 || len(metadata.Assets) != 0 {
		t.Fatalf("404 metadata = %+v, want no releases", metadata)
	}
}

func TestCLIReleaseVersionUsesDevelReleaseIdentity(t *testing.T) {
	tests := []struct {
		name    string
		release cliRelease
		want    string
		wantErr bool
	}{
		{
			name:    "canonical devel release name",
			release: cliRelease{TagName: "devel", Name: "Devel Build a1b2c3d"},
			want:    "a1b2c3d",
		},
		{
			name:    "canonical devel release body fallback",
			release: cliRelease{TagName: "devel", Body: "Devel build: a1b2c3d"},
			want:    "a1b2c3d",
		},
		{
			name:    "legacy devel release name",
			release: cliRelease{TagName: "devel", Name: "Daily Build a1b2c3d"},
			want:    "a1b2c3d",
		},
		{
			name:    "legacy devel release body fallback",
			release: cliRelease{TagName: "devel", Body: "Daily build: a1b2c3d"},
			want:    "a1b2c3d",
		},
		{
			name:    "stable tag",
			release: cliRelease{TagName: "v1.2.3", Name: "Release 1.2.3"},
			want:    "1.2.3",
		},
		{
			name:    "missing devel identity",
			release: cliRelease{TagName: "devel"},
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := cliReleaseVersion(test.release)
			if (err != nil) != test.wantErr {
				t.Fatalf("cliReleaseVersion() error = %v, wantErr %t", err, test.wantErr)
			}
			if got != test.want {
				t.Errorf("cliReleaseVersion() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestCLIUpdateChannelNormalizesAliases(t *testing.T) {
	tests := map[string]string{
		"":            "stable",
		"stable":      "stable",
		"beta":        "beta",
		"devel":       "devel",
		"daily":       "devel",
		"rolling":     "devel",
		"dev":         "devel",
		"development": "devel",
		"invalid":     "",
	}
	for input, want := range tests {
		if got := cliUpdateChannel(input); got != want {
			t.Errorf("cliUpdateChannel(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestCLIReleaseMatchesCanonicalDevelChannel(t *testing.T) {
	if !cliReleaseMatchesChannel(cliRelease{TagName: "devel"}, "devel") {
		t.Fatal("canonical devel release was not selected for devel channel")
	}
	if cliReleaseMatchesChannel(cliRelease{TagName: "20251205143022"}, "devel") {
		t.Fatal("timestamp release was incorrectly selected for devel channel")
	}
}
