package urlutil

import "testing"

func TestDefaultFetchRemoteImageConfig(t *testing.T) {
	cfg := DefaultFetchRemoteImageConfig()
	if cfg.MaxSize != 10*1024*1024 || cfg.Timeout.Seconds() != 30 {
		t.Fatalf("unexpected defaults: %#v", cfg)
	}
	if len(cfg.AllowedTypes) != 5 || len(cfg.AllowedSchemes) != 1 || cfg.AllowedSchemes[0] != "https" {
		t.Fatalf("unexpected allow lists: %#v", cfg)
	}
}

func TestValidateRemoteURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "public IP", url: "https://8.8.8.8/logo.png"},
		{name: "http blocked", url: "http://8.8.8.8/logo.png", wantErr: true},
		{name: "private IPv4", url: "https://10.0.0.1/logo.png", wantErr: true},
		{name: "carrier grade NAT", url: "https://100.64.0.1/logo.png", wantErr: true},
		{name: "documentation IPv4", url: "https://192.0.2.1/logo.png", wantErr: true},
		{name: "loopback IPv4", url: "https://127.0.0.1/logo.png", wantErr: true},
		{name: "private IPv6", url: "https://[fd00::1]/logo.png", wantErr: true},
		{name: "documentation IPv6", url: "https://[2001:db8::1]/logo.png", wantErr: true},
		{name: "localhost", url: "https://localhost/logo.png", wantErr: true},
		{name: "internal suffix", url: "https://service.internal/logo.png", wantErr: true},
		{name: "credentials", url: "https://user:pass@example.com/logo.png", wantErr: true},
		{name: "missing host", url: "https:///logo.png", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRemoteURL(tt.url, DefaultFetchRemoteImageConfig())
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateRemoteURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateRemoteURL_CustomScheme(t *testing.T) {
	cfg := DefaultFetchRemoteImageConfig()
	cfg.AllowedSchemes = []string{"https", "http"}
	if err := ValidateRemoteURL("http://8.8.8.8/logo.png", cfg); err != nil {
		t.Fatalf("explicitly allowed scheme rejected: %v", err)
	}
}
