// SPDX-License-Identifier: MIT
package server

import (
	"mime"
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

type buildStampWriter struct {
	http.ResponseWriter
	request     *http.Request
	stamp       string
	secure      bool
	purge       bool
	wroteHeader bool
}

func (w *buildStampWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *buildStampWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	contentType := w.Header().Get("Content-Type")
	mediaType, _, _ := mime.ParseMediaType(contentType)
	if status >= http.StatusOK && status < http.StatusMultipleChoices &&
		mediaType == "text/html" && isTopLevelDocumentNavigation(w.request) {
		if w.purge {
			w.Header().Set("Clear-Site-Data", `"cache", "storage"`)
		}
		http.SetCookie(w.ResponseWriter, &http.Cookie{
			Name:     "vidveil_build",
			Value:    w.stamp,
			Path:     "/",
			MaxAge:   31536000,
			Secure:   w.secure,
			SameSite: http.SameSiteLaxMode,
		})
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *buildStampWriter) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", http.DetectContentType(data))
		}
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

func (w *buildStampWriter) Flush() {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *buildStampWriter) FlushError() error {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return http.NewResponseController(w.ResponseWriter).Flush()
}
