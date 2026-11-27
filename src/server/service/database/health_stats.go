// SPDX-License-Identifier: MIT
// AI.md PART 13: Persistent aggregate health counters.
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/apimgr/vidveil/src/common/dbstore"
)

// RequestStatsSnapshot contains the durable lifetime counter and the recent
// hourly buckets used to restore the rolling 24-hour aggregate.
type RequestStatsSnapshot struct {
	Total  uint64
	Hourly map[int64]uint64
}

// LoadRequestStats reads only the cached aggregate rows through the read pool.
func (d *AppDatabase) LoadRequestStats(ctx context.Context) (RequestStatsSnapshot, error) {
	snapshot := RequestStatsSnapshot{Hourly: make(map[int64]uint64)}
	err := d.Read(ctx, func(ctx context.Context, db dbstore.Reader) (callbackErr error) {
		var total int64
		err := db.QueryRowContext(ctx, `SELECT value FROM health_stats WHERE name = 'requests_total'`).Scan(&total)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if total > 0 {
			snapshot.Total = uint64(total)
		}
		rows, err := db.QueryContext(ctx, `SELECT bucket_hour, request_count FROM health_request_buckets WHERE bucket_hour >= ? ORDER BY bucket_hour`, time.Now().UTC().Truncate(time.Hour).Add(-23*time.Hour).Unix()/3600)
		if err != nil {
			return err
		}
		defer func() { callbackErr = errors.Join(callbackErr, rows.Close()) }()
		for rows.Next() {
			var hour, count int64
			if err := rows.Scan(&hour, &count); err != nil {
				return err
			}
			if count > 0 {
				snapshot.Hourly[hour] = uint64(count)
			}
		}
		return rows.Err()
	})
	if err != nil {
		return RequestStatsSnapshot{}, fmt.Errorf("load request health stats: %w", err)
	}
	return snapshot, nil
}

// SaveRequestStats atomically stores the lifetime total and recent hour
// buckets. Expired buckets are deleted in a bounded batch.
func (d *AppDatabase) SaveRequestStats(ctx context.Context, snapshot RequestStatsSnapshot) error {
	return d.Write(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO health_stats(name,value,updated_at) VALUES('requests_total',?,?) ON CONFLICT(name) DO UPDATE SET value=excluded.value,updated_at=excluded.updated_at`, snapshot.Total, time.Now().UTC().Unix()); err != nil {
			return err
		}
		for hour, count := range snapshot.Hourly {
			if _, err := tx.ExecContext(ctx, `INSERT INTO health_request_buckets(bucket_hour,request_count) VALUES(?,?) ON CONFLICT(bucket_hour) DO UPDATE SET request_count=excluded.request_count`, hour, count); err != nil {
				return err
			}
		}
		cutoff := time.Now().UTC().Truncate(time.Hour).Unix()/3600 - 23
		_, err := tx.ExecContext(ctx, `DELETE FROM health_request_buckets WHERE bucket_hour IN (SELECT bucket_hour FROM health_request_buckets WHERE bucket_hour < ? ORDER BY bucket_hour LIMIT 500)`, cutoff)
		return err
	})
}
