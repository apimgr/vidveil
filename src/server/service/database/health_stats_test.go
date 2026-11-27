// SPDX-License-Identifier: MIT
package database

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestRequestStatsRoundTrip(t *testing.T) {
	manager, err := NewSchemaManager(filepath.Join(t.TempDir(), "server.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := manager.Close(); err != nil {
			t.Errorf("close schema manager: %v", err)
		}
	})
	if err := manager.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	store := manager.GetAppDatabase()
	hour := time.Now().UTC().Truncate(time.Hour).Unix() / 3600
	want := RequestStatsSnapshot{Total: 42, Hourly: map[int64]uint64{hour: 3}}
	if err := store.SaveRequestStats(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	got, err := store.LoadRequestStats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Total != want.Total || !reflect.DeepEqual(got.Hourly, want.Hourly) {
		t.Fatalf("request stats = %#v, want %#v", got, want)
	}
}
