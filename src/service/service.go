// Package service provides system service management (systemd, launchd, Windows Service)
// This is a re-export package that wraps src/server/service/system/service
package service

import "github.com/apimgr/vidveil/src/server/service/system"

// Re-export main types and functions
type ServiceManager = system.ServiceManager

var (
	NewServiceManager = system.NewServiceManager
)
