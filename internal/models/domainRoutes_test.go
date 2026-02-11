package models

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetDomainRoutes(t *testing.T) {
	tests := []struct {
		name      string
		writeCfg  bool
		cfg       string
		wantNames []string
		wantErr   bool
	}{
		{
			name:      "missing config returns empty",
			writeCfg:  false,
			wantNames: nil,
		},
		{
			name:     "valid config parses",
			writeCfg: true,
			cfg: `domains:
  - name: example.com
    paths:
      - location: /
        target: http://127.0.0.1
        upgrade: false
    subdomains:
      - name: api
        paths:
          - location: /v1
            target: http://127.0.0.2
            upgrade: false
`,
			wantNames: []string{"example.com"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			cfgDir := filepath.Join(tmp, "routy")
			t.Setenv("ROUTY_DATA_DIR", cfgDir)
			if err := os.MkdirAll(cfgDir, 0750); err != nil {
				t.Fatalf("mkdir: %v", err)
			}

			if tt.writeCfg {
				if err := os.WriteFile(filepath.Join(cfgDir, configFilename), []byte(tt.cfg), 0600); err != nil {
					t.Fatalf("write cfg: %v", err)
				}
			}

			routes, err := GetDomainRoutes()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(routes.Domains) != len(tt.wantNames) {
				t.Fatalf("domain count = %d, want %d", len(routes.Domains), len(tt.wantNames))
			}
			for i, name := range tt.wantNames {
				if routes.Domains[i].Name != name {
					t.Fatalf("domain[%d] = %q, want %q", i, routes.Domains[i].Name, name)
				}
			}
		})
	}
}
