package main

import (
	"strings"
	"testing"
)

func TestHandleMaintenanceSecretArgumentValidation(t *testing.T) {
	for _, arg := range []string{"", "rotate", "generate installation_secret", "rotate installation_secret extra"} {
		err := handleMaintenanceSecret(arg, t.TempDir(), t.TempDir())
		if err == nil || !strings.Contains(err.Error(), "Usage:") {
			t.Errorf("arg %q: got %v, want usage error", arg, err)
		}
	}
}

func TestHandleMaintenanceSecretRejectsAutomaticSecrets(t *testing.T) {
	for _, name := range []string{"cookie_signing_key", "csrf_token_secret", "unknown"} {
		err := handleMaintenanceSecret("rotate "+name, t.TempDir(), t.TempDir())
		if err == nil || !strings.Contains(err.Error(), "unsupported secret name") {
			t.Errorf("name %q: got %v, want rejection", name, err)
		}
	}
}
