// SPDX-License-Identifier: MIT
package secret

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestWrappedPreviousEncryptionKeyAndPendingRecovery(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	oldKey := strings.Repeat("01", 32)
	newKey := strings.Repeat("02", 32)
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := StorePreviousEncryptionKey(ctx, tx, oldKey, newKey, time.Now().UTC().Add(time.Hour)); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	previous, valid, err := LoadPreviousEncryptionKey(ctx, testStore(db), newKey, time.Now().UTC())
	if err != nil || !valid || previous != oldKey {
		t.Fatalf("previous key = (%q, %v, %v)", previous, valid, err)
	}
	recovered, pending, err := LoadPendingEncryptionKey(ctx, testStore(db), oldKey, time.Now().UTC())
	if err != nil || !pending || recovered != newKey {
		t.Fatalf("pending key from old config = (%q, %v, %v)", recovered, pending, err)
	}
	recovered, pending, err = LoadPendingEncryptionKey(ctx, testStore(db), newKey, time.Now().UTC())
	if err != nil || !pending || recovered != newKey {
		t.Fatalf("pending key from new config = (%q, %v, %v)", recovered, pending, err)
	}
}
