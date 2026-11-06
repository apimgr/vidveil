// SPDX-License-Identifier: MIT
package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/apimgr/vidveil/src/common/version"
	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/notify"
	paths "github.com/apimgr/vidveil/src/paths"
	"github.com/apimgr/vidveil/src/server/service/scheduler"
)

func TestScheduledBackupFailureUsesDedicatedAdminNotification(t *testing.T) {
	received := make(chan notify.Payload, 1)
	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() { _ = r.Body.Close() }()
		var payload notify.Payload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode webhook payload: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		received <- payload
		w.WriteHeader(http.StatusOK)
	}))
	defer webhook.Close()

	cfg := config.DefaultAppConfig()
	cfg.Server.Contact.Admin.Webhooks = map[string]string{"generic": webhook.URL}
	s := &Server{
		appConfig:        cfg,
		notifyDispatcher: notify.New(&cfg.Server.Contact, paths.ProjectName, version.GetVersion(), cfg.GetPublicURL()),
	}
	task := scheduler.ScheduledTask{ID: "backup_daily", Name: "Daily Backup", NextRun: time.Now().Add(time.Hour)}
	history := scheduler.TaskHistory{Result: "failure", Error: "disk full"}
	s.notifyScheduledTaskFailure(task, history)

	select {
	case payload := <-received:
		if payload.Event != "admin.backup_failed" {
			t.Fatalf("event = %q, want admin.backup_failed", payload.Event)
		}
		if payload.Severity != notify.SeverityCritical {
			t.Fatalf("severity = %q, want critical", payload.Severity)
		}
		if payload.Body == "" {
			t.Fatal("failure details missing from notification")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for scheduler failure notification")
	}
}
