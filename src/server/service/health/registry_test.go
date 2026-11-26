// SPDX-License-Identifier: MIT
package health

import (
	"reflect"
	"testing"
)

func TestRegistryCachesChecksInRegistrationOrder(t *testing.T) {
	registry := NewRegistry()
	called := 0
	if err := registry.Register("geoip_country", false, func() string {
		called++
		return StatusOK
	}); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register("blocklist_ads", true, func() string {
		called++
		return StatusError
	}); err != nil {
		t.Fatal(err)
	}
	if got := registry.Snapshot(); len(got) != 2 || got[0].Status != "" || called != 0 {
		t.Fatalf("initial snapshot = %#v, callbacks = %d", got, called)
	}
	registry.Refresh()
	got := registry.Snapshot()
	want := []CheckResult{
		{Key: "geoip_country", Status: StatusOK},
		{Key: "blocklist_ads", Status: StatusError, Critical: true},
	}
	if !reflect.DeepEqual(got, want) || called != 2 {
		t.Fatalf("snapshot = %#v, calls = %d; want %#v, 2", got, called, want)
	}
	_ = registry.Snapshot()
	if called != 2 {
		t.Fatalf("snapshot ran health callback; calls = %d", called)
	}
}

func TestRegistryValidatesKeysAndCallback(t *testing.T) {
	registry := NewRegistry()
	for _, key := range []string{"", "../country", "GeoIP_country", "data-file"} {
		if err := registry.Register(key, false, func() string { return StatusOK }); err == nil {
			t.Errorf("Register(%q) succeeded", key)
		}
	}
	if err := registry.Register("geoip_country", false, nil); err == nil {
		t.Error("Register accepted a nil callback")
	}
}

func TestRegistryConvertsInvalidStatusToError(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register("data_index", false, func() string { return "private details" }); err != nil {
		t.Fatal(err)
	}
	registry.Refresh()
	if got := registry.Snapshot()[0].Status; got != StatusError {
		t.Fatalf("status = %q, want %q", got, StatusError)
	}
}
