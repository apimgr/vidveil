// SPDX-License-Identifier: MIT
package metric_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/paths"
	"github.com/apimgr/vidveil/src/server/service/database"
	"github.com/apimgr/vidveil/src/server/service/metric"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRefreshOwnerTokenMetricsCountsOnlyActiveTokens(t *testing.T) {
	base := filepath.Join(os.TempDir(), paths.ProjectOrg)
	if err := os.MkdirAll(base, 0755); err != nil {
		t.Fatal(err)
	}
	tempDir, err := os.MkdirTemp(base, paths.ProjectName+"-metrics-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })
	db, err := database.NewAppDatabase(database.DatabaseConfig{Driver: database.DriverSQLite, Path: filepath.Join(tempDir, "metrics.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close metrics database: %v", err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.Write(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `CREATE TABLE api_tokens (kind TEXT, revoked_at INTEGER, expires_at INTEGER)`); err != nil {
			return err
		}
		now := time.Now().Unix()
		_, err := tx.ExecContext(ctx, `INSERT INTO api_tokens (kind, revoked_at, expires_at) VALUES
			('primary', NULL, NULL), ('linked', NULL, ?), ('linked', NULL, ?), ('primary', ?, NULL)`, now+3600, now-3600, now)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := metric.RefreshOwnerTokenMetrics(db); err != nil {
		t.Fatal(err)
	}
	if got := testutil.ToFloat64(metric.OwnerTokensActive.WithLabelValues("primary")); got != 1 {
		t.Errorf("active primary tokens = %v, want 1", got)
	}
	if got := testutil.ToFloat64(metric.OwnerTokensActive.WithLabelValues("linked")); got != 1 {
		t.Errorf("active linked tokens = %v, want 1", got)
	}
}
