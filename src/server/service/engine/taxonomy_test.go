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

// TestResultMatchesIntent_LesbianTeenNotZeroedByBoilerplateNoise is a
// regression test for the live bug where `GET /search?q=lesbian%20teen`
// returned near-empty results in production. Root cause: malePresenceWords
// and olderIndicatorWords were matched against the ENTIRE scraped blob
// (title+description+tags+performer), and generic words like "guy"/"guys"/
// "men"/"mom" appear constantly in unrelated scraped ad/nav/tag boilerplate,
// silently rejecting nearly every real result. Realistic titles here don't
// contain the literal word "teen" (as real upstream results often don't),
// and Description/Tags carry the kind of unrelated noise seen in production
// scrapes — none of this should cause a female-only teen query to reject
// an on-topic result.
func TestResultMatchesIntent_LesbianTeenNotZeroedByBoilerplateNoise(t *testing.T) {
	intent := DetectQueryIntent("lesbian teen")
	if !intent.IsFemaleOnly {
		t.Fatalf("expected IsFemaleOnly=true for %q", "lesbian teen")
	}

	results := []model.VideoResult{
		{
			Title:       "18yo Girlfriends Get Frisky On Webcam",
			Description: "More hot girl-on-girl scenes with your favorite guys' picks and top rated moms",
			Tags:        []string{"amateur", "webcam", "guys favorite", "related-men-content"},
			Performer:   "Riley",
		},
		{
			Title:       "Young Girl On Girl Fun In College Dorm",
			Description: "Recommended for men who like young lesbians - see more amateur content",
			Tags:        []string{"college", "petite", "amateur"},
		},
	}

	for _, r := range results {
		if !ResultMatchesIntent(r, intent) {
			t.Errorf("expected result to be accepted (boilerplate noise should not reject), title=%q", r.Title)
		}
	}
}

// TestResultMatchesIntent_GenericWordsNoLongerInMalePresenceWords locks in
// the trimmed malePresenceWords list — ultra-generic single words must not
// be present, since they matched innocuous boilerplate far too often.
func TestResultMatchesIntent_GenericWordsNoLongerInMalePresenceWords(t *testing.T) {
	removed := []string{"male", "man", "men", "guy", "guys", "husband", "dude"}
	for _, w := range malePresenceWords {
		for _, r := range removed {
			if w == r {
				t.Errorf("malePresenceWords still contains overly generic word %q", r)
			}
		}
	}
}

// TestResultMatchesIntent_StepbrotherCompoundWord is a regression test for
// the live bug where `GET /search?q=Teen%20lesbian` returned titles like
// "Invite Stepbrother". Root cause: malePresenceWords/familyMaleWords had
// the abbreviation "stepbro" but not the full compound word "stepbrother",
// and containsWholeWord's word-boundary matching means neither "stepbro"
// nor bare "brother" matches inside "stepbrother".
func TestResultMatchesIntent_StepbrotherCompoundWord(t *testing.T) {
	intent := DetectQueryIntent("teen lesbian")
	if !intent.IsFemaleOnly {
		t.Fatalf("expected IsFemaleOnly=true for %q", "teen lesbian")
	}

	rejectTitles := []string{
		"Invite Stepbrother",
		"Teen Fucks Her Stepbrother After School",
	}

	for _, title := range rejectTitles {
		r := model.VideoResult{Title: title}
		if ResultMatchesIntent(r, intent) {
			t.Errorf("expected result to be rejected for female-only query, title=%q", title)
		}
	}
}

// TestResultMatchesIntent_GrandparentCompoundWords is a regression test for
// the same word-boundary gap as the stepbrother bug: malePresenceWords/
// familyMaleWords had "grandpa" but not "grandfather"/"granddad", and
// containsWholeWord's word-boundary matching means neither "father" nor
// "dad" matches inside "grandfather"/"granddad".
func TestResultMatchesIntent_GrandparentCompoundWords(t *testing.T) {
	intent := DetectQueryIntent("teen lesbian")
	if !intent.IsFemaleOnly {
		t.Fatalf("expected IsFemaleOnly=true for %q", "teen lesbian")
	}

	rejectTitles := []string{
		"Grandfather and Teen",
		"Granddad Fucks Stepdaughter",
	}

	for _, title := range rejectTitles {
		r := model.VideoResult{Title: title}
		if ResultMatchesIntent(r, intent) {
			t.Errorf("expected result to be rejected for female-only query, title=%q", title)
		}
	}
}

// TestDetectQueryIntent_GrandparentFamilyCombo covers the family-combo
// inference gap: familyFemaleWords lacked "granddaughter" and
// familyMaleWords lacked "grandson", so a "grandma granddaughter" query
// wasn't inferred female-only the way "mom daughter" already was.
func TestDetectQueryIntent_GrandparentFamilyCombo(t *testing.T) {
	intent := DetectQueryIntent("grandma granddaughter")
	if !intent.IsFemaleOnly {
		t.Fatalf("expected IsFemaleOnly=true for %q", "grandma granddaughter")
	}

	mixedIntent := DetectQueryIntent("grandma grandson")
	if mixedIntent.IsFemaleOnly {
		t.Fatalf("expected IsFemaleOnly=false for %q (male family term present)", "grandma grandson")
	}
}

// TestResultMatchesIntent_DescriptionAndTagsOutOfScope ensures the intent
// filter only inspects Title/Performer — Description and Tags must not be
// able to trigger a false-positive rejection even if they still contained a
// disqualifying word (defense in depth alongside the trimmed word list).
func TestResultMatchesIntent_DescriptionAndTagsOutOfScope(t *testing.T) {
	intent := DetectQueryIntent("lesbian teen")
	r := model.VideoResult{
		Title:       "Two Teen Girls Kissing",
		Description: "his dad and stepbro talk about this on their podcast",
		Tags:        []string{"stepbro", "milf", "grandpa"},
	}
	if !ResultMatchesIntent(r, intent) {
		t.Errorf("expected result to be accepted — Description/Tags noise must be out of scope, title=%q", r.Title)
	}
}
