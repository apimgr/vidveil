// SPDX-License-Identifier: MIT
package main

import "testing"

func TestBuildEpoch(t *testing.T) {
	original := BuildEpoch
	t.Cleanup(func() { BuildEpoch = original })

	for _, test := range []struct {
		name  string
		value string
		want  int64
	}{
		{name: "valid", value: "1765112713", want: 1765112713},
		{name: "unset", value: "0", want: 0},
		{name: "invalid", value: "invalid", want: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			BuildEpoch = test.value
			if got := buildEpoch(); got != test.want {
				t.Fatalf("buildEpoch() = %d, want %d", got, test.want)
			}
		})
	}
}

func TestBuildDate(t *testing.T) {
	if got := buildDate(0); got != "unknown" {
		t.Fatalf("buildDate(0) = %q, want unknown", got)
	}
	if got := buildDate(-1); got != "unknown" {
		t.Fatalf("buildDate(-1) = %q, want unknown", got)
	}
	if got := buildDate(1765112713); got != "2025-12-07T13:05:13Z" {
		t.Fatalf("buildDate(valid epoch) = %q", got)
	}
}
