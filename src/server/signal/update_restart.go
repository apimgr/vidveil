// SPDX-License-Identifier: MIT
package signal

import "os"

type updateRestartSignal struct{}

func (updateRestartSignal) Signal()        {}
func (updateRestartSignal) String() string { return "staged update restart" }

var updateRestartRequests = make(chan struct{}, 1)

// RequestUpdateRestart asks the normal shutdown path to stop the server so a
// supervisor can restart it and apply a staged update with elevated rights.
func RequestUpdateRestart() {
	select {
	case updateRestartRequests <- struct{}{}:
	default:
	}
}

// IsUpdateRestart reports whether a shutdown notification requests exit 75.
func IsUpdateRestart(sig os.Signal) bool {
	_, ok := sig.(updateRestartSignal)
	return ok
}

func waitForUpdateRestart() <-chan struct{} { return updateRestartRequests }

func resetUpdateRestart() {
	select {
	case <-updateRestartRequests:
	default:
	}
}

var _ os.Signal = updateRestartSignal{}
