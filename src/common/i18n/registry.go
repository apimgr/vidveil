// SPDX-License-Identifier: MIT
package i18n

// LocaleInfo describes one complete, embedded locale.
type LocaleInfo struct {
	Code             string
	NativeName       string
	Direction        string
	PluralCategories []string
}

// SupportedLanguageRegistry is the single source for locale labels and
// language metadata exposed by the server UI.
var SupportedLanguageRegistry = []LocaleInfo{
	{Code: "ar", NativeName: "العربية", Direction: "rtl", PluralCategories: []string{"zero", "one", "two", "few", "many", "other"}},
	{Code: "de", NativeName: "Deutsch", Direction: "ltr", PluralCategories: []string{"one", "other"}},
	{Code: "en", NativeName: "English", Direction: "ltr", PluralCategories: []string{"one", "other"}},
	{Code: "es", NativeName: "Español", Direction: "ltr", PluralCategories: []string{"one", "other"}},
	{Code: "fr", NativeName: "Français", Direction: "ltr", PluralCategories: []string{"one", "other"}},
	{Code: "it", NativeName: "Italiano", Direction: "ltr", PluralCategories: []string{"one", "many", "other"}},
	{Code: "ja", NativeName: "日本語", Direction: "ltr", PluralCategories: []string{"other"}},
	{Code: "zh", NativeName: "中文", Direction: "ltr", PluralCategories: []string{"other"}},
}

// LocaleInfoFor returns the registry metadata for a supported locale.
func LocaleInfoFor(code string) (LocaleInfo, bool) {
	for _, locale := range SupportedLanguageRegistry {
		if locale.Code == code {
			return locale, true
		}
	}
	return LocaleInfo{}, false
}
