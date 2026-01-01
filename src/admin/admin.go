// Package admin provides admin panel functionality
// This is a re-export package that wraps src/server/service/admin
package admin

import "github.com/apimgr/vidveil/src/server/service/admin"

// Re-export main types and functions from server/service/admin
type Service = admin.Service

var (
	New = admin.New
)
