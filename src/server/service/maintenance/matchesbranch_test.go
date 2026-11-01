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
		{"stable release matches devel branch", "1.2.3", false, "devel", true},
		{"beta prerelease matches beta branch", "1.2.3-beta", true, "beta", true},
		{"beta prerelease matches devel branch", "1.2.3-beta", true, "devel", true},
		{"beta prerelease excluded from stable branch", "1.2.3-beta", true, "stable", false},
		{"devel release matches devel branch", "devel", true, "devel", true},
		{"devel release excluded from beta branch", "devel", true, "beta", false},
		{"devel release excluded from stable branch", "devel", true, "stable", false},
		{"timestamp prerelease excluded from devel", "20260725123045", true, "devel", false},
		{"dotted timestamp prerelease excluded from devel", "2026.07.251230", true, "devel", false},
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
