// SPDX-License-Identifier: MIT
package main

import (
	"runtime"
	"strconv"
	"time"

	"github.com/apimgr/vidveil/src/common/version"
	svcmetrics "github.com/apimgr/vidveil/src/server/service/metric"
)

var (
	Version      = "dev"
	CommitID     = "unknown"
	BuildEpoch   = "0"
	BuildDate    = "unknown"
	OfficialSite = ""
)

func buildEpoch() int64 {
	n, err := strconv.ParseInt(BuildEpoch, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func buildDate(epoch int64) string {
	if epoch <= 0 {
		return "unknown"
	}
	return time.Unix(epoch, 0).UTC().Format("2006-01-02T15:04:05Z")
}

func init() {
	BuildDate = buildDate(buildEpoch())
	version.Version = Version
	version.CommitID = CommitID
	version.BuildTime = BuildDate
	version.OfficialSite = OfficialSite
	svcmetrics.InitMetricsAppInfo(Version, CommitID, BuildDate, runtime.Version())
}
