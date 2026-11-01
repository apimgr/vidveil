// SPDX-License-Identifier: MIT
// Package format renders machine values as localized, human-readable text.
package format

import (
	"math"
	"strings"
	"time"

	"github.com/apimgr/vidveil/src/common/i18n"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

const (
	secondsPerMinute = 60
	secondsPerHour   = 60 * secondsPerMinute
	secondsPerDay    = 24 * secondsPerHour
)

// Duration formats a duration using at most two units and localized unit names.
func Duration(d time.Duration, locale string) string {
	seconds := math.Abs(d.Seconds())
	negative := d < 0
	var parts []string
	appendUnit := func(value float64, unit string) {
		if value <= 0 || len(parts) >= 2 {
			return
		}
		parts = append(parts, translatedUnit(locale, unit, value))
	}
	remaining := seconds
	switch {
	case seconds >= secondsPerDay:
		value := math.Floor(remaining / secondsPerDay)
		appendUnit(value, "days")
		remaining -= value * secondsPerDay
		appendUnit(math.Floor(remaining/secondsPerHour), "hours")
	case seconds >= secondsPerHour:
		value := math.Floor(remaining / secondsPerHour)
		appendUnit(value, "hours")
		remaining -= value * secondsPerHour
		appendUnit(math.Floor(remaining/secondsPerMinute), "minutes")
	case seconds >= secondsPerMinute:
		value := math.Floor(remaining / secondsPerMinute)
		appendUnit(value, "minutes")
		remaining -= value * secondsPerMinute
		appendUnit(math.Floor(remaining), "seconds")
	default:
		appendUnit(seconds, "seconds")
	}
	if len(parts) == 0 {
		parts = append(parts, translatedUnit(locale, "seconds", 0))
	}
	result := strings.Join(parts, " ")
	if negative {
		result = "−" + result
	}
	return result
}

// Size formats a byte count using binary units through terabytes.
func Size(bytes uint64, locale string) string {
	value := float64(bytes)
	unit := "bytes"
	units := []string{"bytes", "kilobytes", "megabytes", "gigabytes", "terabytes"}
	for index := 0; value >= 1024 && index < len(units)-1; index++ {
		value /= 1024
		unit = units[index+1]
	}
	value = math.Round(value*10) / 10
	return translatedUnit(locale, unit, value)
}

// Count formats an integer with locale-aware grouping separators.
func Count(count int64, locale string) string {
	tag, err := language.Parse(locale)
	if err != nil {
		tag = language.English
	}
	return message.NewPrinter(tag).Sprintf("%d", count)
}

func translatedUnit(locale, unit string, value float64) string {
	tag, err := language.Parse(locale)
	if err != nil {
		tag = language.English
	}
	printer := message.NewPrinter(tag)
	var formatted string
	if value == math.Trunc(value) {
		formatted = printer.Sprintf("%d", int64(value))
	} else {
		formatted = printer.Sprintf("%.1f", value)
	}
	if value != math.Trunc(value) {
		return i18n.TranslateFormat(locale, "format."+unit+"_other", "count", formatted)
	}
	return i18n.TranslatePluralFormat(locale, "format."+unit, int(value), "count", formatted)
}
