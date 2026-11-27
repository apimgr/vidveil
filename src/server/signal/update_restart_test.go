// SPDX-License-Identifier: MIT
package signal

import "testing"

func TestUpdateRestartRequestWakesShutdownWaiter(t *testing.T) {
	ResetShutdown()
	RequestUpdateRestart()
	select {
	case <-waitForUpdateRestart():
	default:
		t.Fatal("staged update shutdown request was not queued")
	}
}

func TestUpdateRestartSignalClassification(t *testing.T) {
	if !IsUpdateRestart(updateRestartSignal{}) {
		t.Fatal("update restart signal was not classified")
	}
}
