// SPDX-License-Identifier: MIT
package format

import (
	"testing"
	"time"
)

func TestDuration(t *testing.T) {
	tests := []struct {
		name   string
		value  time.Duration
		locale string
		want   string
	}{
		{name: "seconds", value: 45 * time.Second, locale: "en", want: "45 seconds"},
		{name: "fractional seconds", value: 500 * time.Millisecond, locale: "en", want: "0.5 seconds"},
		{name: "minutes and seconds", value: 65 * time.Second, locale: "en", want: "1 minute 5 seconds"},
		{name: "hours", value: time.Hour, locale: "en", want: "1 hour"},
		{name: "days and hours", value: 26 * time.Hour, locale: "en", want: "1 day 2 hours"},
		{name: "localized singular", value: time.Second, locale: "es", want: "1 segundo"},
		{name: "arabic dual", value: 2 * time.Second, locale: "ar", want: "٢ ثانيتان"},
		{name: "zero", value: 0, locale: "en", want: "0 seconds"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := Duration(test.value, test.locale); got != test.want {
				t.Errorf("Duration(%s, %q) = %q, want %q", test.value, test.locale, got, test.want)
			}
		})
	}
}

func TestSize(t *testing.T) {
	tests := []struct {
		bytes  uint64
		locale string
		want   string
	}{
		{bytes: 1, locale: "en", want: "1 byte"},
		{bytes: 512, locale: "en", want: "512 bytes"},
		{bytes: 1024, locale: "en", want: "1 kilobyte"},
		{bytes: 1536, locale: "en", want: "1.5 kilobytes"},
		{bytes: 5 * 1024 * 1024, locale: "en", want: "5 megabytes"},
		{bytes: 1024, locale: "fr", want: "1 kilooctet"},
	}
	for _, test := range tests {
		if got := Size(test.bytes, test.locale); got != test.want {
			t.Errorf("Size(%d, %q) = %q, want %q", test.bytes, test.locale, got, test.want)
		}
	}
}

func TestCount(t *testing.T) {
	if got := Count(12847, "en"); got != "12,847" {
		t.Errorf("Count(12847, en) = %q, want %q", got, "12,847")
	}
	if got := Count(12847, "de"); got != "12.847" {
		t.Errorf("Count(12847, de) = %q, want %q", got, "12.847")
	}
}
