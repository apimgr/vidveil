// SPDX-License-Identifier: MIT
package maintenance

import "testing"

func TestValidStagedUpdateVersion(t *testing.T) {
	for _, version := range []string{"1.2.3", "202612011200-beta", "devel", "v1.2.3"} {
		if !validStagedUpdateVersion(version) {
			t.Errorf("version %q was rejected", version)
		}
	}
	for _, version := range []string{"", "../1.2.3", "1.2.3\\payload", "1.2.3\x00", "1.2.3?token"} {
		if validStagedUpdateVersion(version) {
			t.Errorf("unsafe version %q was accepted", version)
		}
	}
}
