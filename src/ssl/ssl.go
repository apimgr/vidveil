// Package ssl provides SSL/TLS certificate management
// This is a re-export package that wraps src/server/service/ssl
package ssl

import "github.com/apimgr/vidveil/src/server/service/ssl"

// Re-export main types and functions from server/service/ssl
type Manager = ssl.Manager

var (
	New = ssl.New
)
