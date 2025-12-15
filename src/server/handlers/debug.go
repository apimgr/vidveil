// SPDX-License-Identifier: MIT
package handlers

import (
	"expvar"
	"net/http"
	"net/http/pprof"

	"github.com/go-chi/chi/v5"
)

// DebugVars serves the expvar debug variables
func DebugVars(w http.ResponseWriter, r *http.Request) {
	expvar.Handler().ServeHTTP(w, r)
}

// DebugPprof serves the pprof index
func DebugPprof(w http.ResponseWriter, r *http.Request) {
	pprof.Index(w, r)
}

// DebugPprofCmdline serves the cmdline profile
func DebugPprofCmdline(w http.ResponseWriter, r *http.Request) {
	pprof.Cmdline(w, r)
}

// DebugPprofProfile serves the CPU profile
func DebugPprofProfile(w http.ResponseWriter, r *http.Request) {
	pprof.Profile(w, r)
}

// DebugPprofSymbol serves the symbol lookup
func DebugPprofSymbol(w http.ResponseWriter, r *http.Request) {
	pprof.Symbol(w, r)
}

// DebugPprofTrace serves the execution trace
func DebugPprofTrace(w http.ResponseWriter, r *http.Request) {
	pprof.Trace(w, r)
}

// DebugPprofHandler serves specific pprof handlers by name
func DebugPprofHandler(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	pprof.Handler(name).ServeHTTP(w, r)
}
