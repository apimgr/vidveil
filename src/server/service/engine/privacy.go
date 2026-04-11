// SPDX-License-Identifier: MIT
// Static privacy metadata for upstream search engines.
// Per AI.md PART 1: privacy-first design. Scores reflect the upstream site's
// practices if a user were to visit directly. VidVeil proxies all requests so
// users are never directly exposed to these sites.
package engine

import "github.com/apimgr/vidveil/src/server/model"

// enginePrivacyScores holds static privacy metadata keyed by engine name.
// RequiresJS: engine HTML requires JavaScript to render search results.
// SetsCookies: engine sets persistent tracking/session cookies.
// HasTracking: engine embeds third-party analytics or ad trackers.
var enginePrivacyScores = map[string]model.EnginePrivacyScore{
	// Privacy-friendlier engines
	"hqporner":    {RequiresJS: false, SetsCookies: false, HasTracking: false},
	"eporner":     {RequiresJS: false, SetsCookies: true, HasTracking: false},
	"xhamster":    {RequiresJS: false, SetsCookies: true, HasTracking: true},
	"xvideos":     {RequiresJS: false, SetsCookies: true, HasTracking: false},
	"pornhub":     {RequiresJS: false, SetsCookies: true, HasTracking: true},
	"redtube":     {RequiresJS: false, SetsCookies: true, HasTracking: true},
	"youporn":     {RequiresJS: false, SetsCookies: true, HasTracking: true},
	"tube8":       {RequiresJS: false, SetsCookies: true, HasTracking: true},
	"spankbang":   {RequiresJS: false, SetsCookies: true, HasTracking: true},
	"xnxx":        {RequiresJS: false, SetsCookies: false, HasTracking: false},
	"tnaflix":     {RequiresJS: false, SetsCookies: true, HasTracking: true},
	"empflix":     {RequiresJS: false, SetsCookies: true, HasTracking: true},
	"drtuber":     {RequiresJS: false, SetsCookies: true, HasTracking: false},
	"porntrex":    {RequiresJS: false, SetsCookies: true, HasTracking: true},
	"beeg":        {RequiresJS: true, SetsCookies: true, HasTracking: true},
	"alphaporno":  {RequiresJS: false, SetsCookies: true, HasTracking: false},
	"anyporn":     {RequiresJS: false, SetsCookies: true, HasTracking: false},
	"flyflv":      {RequiresJS: false, SetsCookies: true, HasTracking: false},
	"fourtube":    {RequiresJS: false, SetsCookies: true, HasTracking: true},
	"fux":         {RequiresJS: false, SetsCookies: true, HasTracking: false},
	"gotporn":     {RequiresJS: false, SetsCookies: true, HasTracking: false},
	"hellporno":   {RequiresJS: false, SetsCookies: true, HasTracking: false},
	"lovehomeporn":{RequiresJS: false, SetsCookies: true, HasTracking: false},
	"motherless":  {RequiresJS: false, SetsCookies: true, HasTracking: false},
	"nonktube":    {RequiresJS: false, SetsCookies: true, HasTracking: false},
	"nubilesporn": {RequiresJS: true, SetsCookies: true, HasTracking: true},
	"nuvid":       {RequiresJS: false, SetsCookies: true, HasTracking: false},
	"pornbox":     {RequiresJS: false, SetsCookies: true, HasTracking: false},
	"pornerbros":  {RequiresJS: false, SetsCookies: true, HasTracking: true},
	"pornflip":    {RequiresJS: false, SetsCookies: true, HasTracking: false},
	"pornhat":     {RequiresJS: false, SetsCookies: true, HasTracking: false},
	"pornhd":      {RequiresJS: false, SetsCookies: true, HasTracking: false},
	"pornmd":      {RequiresJS: false, SetsCookies: true, HasTracking: false},
	"pornone":     {RequiresJS: false, SetsCookies: true, HasTracking: false},
}

// defaultPrivacyScore is used for any engine not listed above.
var defaultPrivacyScore = model.EnginePrivacyScore{
	RequiresJS:  false,
	SetsCookies: true,
	HasTracking: true,
}

// getEnginePrivacyScore returns the privacy score for a named engine.
func getEnginePrivacyScore(name string) model.EnginePrivacyScore {
	if score, ok := enginePrivacyScores[name]; ok {
		return score
	}
	return defaultPrivacyScore
}
