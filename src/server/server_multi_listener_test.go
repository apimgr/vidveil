// SPDX-License-Identifier: MIT
// AI.md PART 8: Graceful shutdown must close every active HTTP listener.
package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/apimgr/vidveil/src/config"
)

func TestShutdownClosesEveryHTTPListener(t *testing.T) {
	app := &Server{appConfig: config.DefaultAppConfig(), router: chi.NewRouter()}
	app.router.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	listeners := make([]net.Listener, 2)
	addresses := make([]string, 2)
	for i := range listeners {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		listeners[i] = listener
		addresses[i] = listener.Addr().String()
		t.Cleanup(func() { _ = listener.Close() })
		go func(listener net.Listener) { _ = app.ServeOn(listener) }(listener)
	}

	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}
	for _, address := range addresses {
		waitForServerListener(t, client, "http://"+address+"/health")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := app.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	for _, address := range addresses {
		response, err := client.Get(fmt.Sprintf("http://%s/health", address))
		if err == nil {
			_ = response.Body.Close()
			t.Errorf("listener %s still accepted requests after shutdown", address)
		}
	}
}

func waitForServerListener(t *testing.T, client *http.Client, endpoint string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		response, err := client.Get(endpoint)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode != http.StatusOK {
				t.Fatalf("GET %s status = %d, want 200", endpoint, response.StatusCode)
			}
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("server listener %s did not become ready", endpoint)
}
