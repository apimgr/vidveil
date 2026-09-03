// SPDX-License-Identifier: MIT
// AI.md PART 22: deterministic coverage for cumulative update-channel matching.
package maintenance

import "testing"

func TestMatchesBranch(t *testing.T) {
	cases := []struct {
		name   string
		tag    string
		prerel bool
		branch string
		want   bool
	}{
		{"stable release matches stable branch", "1.2.3", false, "stable", true},
		{"stable release matches beta branch", "1.2.3", false, "beta", true},
		{"stable release matches daily branch", "1.2.3", false, "daily", true},
		{"beta prerelease matches beta branch", "1.2.3-beta", true, "beta", true},
		{"beta prerelease matches daily branch", "1.2.3-beta", true, "daily", true},
		{"beta prerelease excluded from stable branch", "1.2.3-beta", true, "stable", false},
		{"daily prerelease matches daily branch", "20260725123045", true, "daily", true},
		{"daily prerelease excluded from beta branch", "20260725123045", true, "beta", false},
		{"daily prerelease excluded from stable branch", "20260725123045", true, "stable", false},
		{"non-daily-length prerelease excluded from daily", "202607251230", true, "daily", false},
		{"dotted 14-char prerelease not treated as daily", "2026.07.251230", true, "daily", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := GitHubRelease{TagName: tc.tag, Prerelease: tc.prerel}
			if got := matchesBranch(r, tc.branch); got != tc.want {
				t.Errorf("matchesBranch(tag=%q prerelease=%v, %q) = %v, want %v",
					tc.tag, tc.prerel, tc.branch, got, tc.want)
			}
		})
	}
}
