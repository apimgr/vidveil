// SPDX-License-Identifier: MIT
package server

import (
	"net/http"
)

func isTopLevelDocumentNavigation(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	dest := r.Header.Get("Sec-Fetch-Dest")
	mode := r.Header.Get("Sec-Fetch-Mode")
	return (dest == "" || dest == "document") && (mode == "" || mode == "navigate")
}

type versionPurgeWriter struct {
	http.ResponseWriter
	wroteHeader bool
}

func (w *versionPurgeWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *versionPurgeWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	if status >= http.StatusOK && status < http.StatusMultipleChoices {
		w.Header().Set("Clear-Site-Data", `"cache", "storage"`)
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *versionPurgeWriter) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

func (w *versionPurgeWriter) Flush() {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *versionPurgeWriter) FlushError() error {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return http.NewResponseController(w.ResponseWriter).Flush()
}
