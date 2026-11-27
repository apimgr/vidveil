// SPDX-License-Identifier: MIT
package dbstore

import (
	"context"
	"database/sql"
)

// RowScanner exposes one query result without exposing its connection pool.
type RowScanner interface {
	Scan(...any) error
}

// Reader exposes only query and health-probe capabilities from the database store.
type Reader interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) RowScanner
	PingContext(context.Context) error
}
