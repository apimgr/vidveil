// SPDX-License-Identifier: MIT
package engine

import (
	"testing"

	"github.com/apimgr/vidveil/src/server/model"
)

// TestResultMatchesIntent_MomDaughterRejectsMalePresence covers the live
// regression found while beta-testing the "mom daughter" query: titles
// containing plain male-relationship words (dad, father, son, brother,
// boyfriend, bf) were leaking through because malePresenceWords only had
// concatenated step-forms (stepdad, stepfather, stepbro) and phrase-anchored
// entries (boyfriend fucks, hubby fucks), never the bare words themselves.
func TestResultMatchesIntent_MomDaughterRejectsMalePresence(t *testing.T) {
	intent := DetectQueryIntent("mom daughter")
	if !intent.IsFemaleOnly {
		t.Fatalf("expected IsFemaleOnly=true for %q", "mom daughter")
	}

	rejectTitles := []string{
		"18 Yoyo - Step Father Fuck Step Daughter While Step Mom",
		"Mom, daughter and son fucked together",
		"Mom Bangs Daughter'S Boyfriend (Tara Holiday)",
		"Daughter fucks mom's BF",
		"Mom and daughter screw dad",
		"S4E3: Step mom Teaches Sex and Shares Cum with Step daughter in Bed with New Step dad",
		"step Mom, step , and step Daughter Love Triangle - step Creampies step Mom, step Brother Creampies step Sister",
		"stepDAUGHTER have sex with stepDADDY next to step MOM with Jane",
	}

	for _, title := range rejectTitles {
		r := model.VideoResult{Title: title}
		if ResultMatchesIntent(r, intent) {
			t.Errorf("expected result to be rejected for female-only query, title=%q", title)
		}
	}

	acceptTitles := []string{
		"Mom daughter webcam lesbian",
		"Mom Daughter Masturbation Pussy Licking",
		"Mom And Daughter Strip And Play On Webcam",
	}

	for _, title := range acceptTitles {
		r := model.VideoResult{Title: title}
		if !ResultMatchesIntent(r, intent) {
			t.Errorf("expected result to be accepted for female-only query, title=%q", title)
		}
	}
}

// TestResultMatchesIntent_ToyWordsStillAllowed ensures the sex-toy exception
// for ambiguous words (cock, dick, bbc) still works after the malePresenceWords
// expansion — "bbc dildo" must not be rejected just because "bbc" is present.
func TestResultMatchesIntent_ToyWordsStillAllowed(t *testing.T) {
	intent := DetectQueryIntent("lesbian")
	r := model.VideoResult{Title: "Lesbian toys with a bbc dildo"}
	if !ResultMatchesIntent(r, intent) {
		t.Errorf("expected toy-word exception to allow result, title=%q", r.Title)
	}
}

// TestDetectQueryIntent_MomSonNotFemaleOnly ensures a mixed-gender family combo
// (mom + son) is never classified as female-only.
func TestDetectQueryIntent_MomSonNotFemaleOnly(t *testing.T) {
	intent := DetectQueryIntent("mom son")
	if intent.IsFemaleOnly {
		t.Fatalf("expected IsFemaleOnly=false for %q", "mom son")
	}
}
