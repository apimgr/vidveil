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

func TestRegistryCachesStatsInRegistrationOrder(t *testing.T) {
	registry := NewRegistry()
	first, second := uint64(4), uint64(9)
	calls := 0
	if err := registry.RegisterStat("searches_total", func() uint64 { calls++; return first }); err != nil {
		t.Fatal(err)
	}
	if err := registry.RegisterStat("engines_healthy", func() uint64 { calls++; return second }); err != nil {
		t.Fatal(err)
	}
	if got := registry.Stats(); len(got) != 2 || got[0].Value != 0 || calls != 0 {
		t.Fatalf("initial stats = %#v, calls = %d", got, calls)
	}
	registry.Refresh()
	want := []StatResult{{Key: "searches_total", Value: 4}, {Key: "engines_healthy", Value: 9}}
	if got := registry.Stats(); !reflect.DeepEqual(got, want) || calls != 2 {
		t.Fatalf("stats = %#v, calls = %d; want %#v, 2", got, calls, want)
	}
	first, second = 5, 10
	if got := registry.Stats(); !reflect.DeepEqual(got, want) || calls != 2 {
		t.Fatalf("Stats sampled callbacks instead of returning cached values: %#v, calls = %d", got, calls)
	}
	registry.Refresh()
	want = []StatResult{{Key: "searches_total", Value: 5}, {Key: "engines_healthy", Value: 10}}
	if got := registry.Stats(); !reflect.DeepEqual(got, want) {
		t.Fatalf("refreshed stats = %#v, want %#v", got, want)
	}
}

func TestRegistryValidatesStatKeysAndCallbacks(t *testing.T) {
	registry := NewRegistry()
	for _, key := range []string{"", "../searches", "Searches_total", "engine-healthy"} {
		if err := registry.RegisterStat(key, func() uint64 { return 1 }); err == nil {
			t.Errorf("RegisterStat(%q) succeeded", key)
		}
	}
	if err := registry.RegisterStat("searches_total", nil); err == nil {
		t.Error("RegisterStat accepted a nil callback")
	}
	if err := registry.RegisterStat("searches_total", func() uint64 { return 1 }); err != nil {
		t.Fatal(err)
	}
	if err := registry.RegisterStat("searches_total", func() uint64 { return 2 }); err == nil {
		t.Error("RegisterStat accepted a duplicate key")
	}
}
