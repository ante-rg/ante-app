package config

import (
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestLoad_MissingDatabaseURL(t *testing.T) {
	_, err := Load(env(nil))
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("want error naming DATABASE_URL, got %v", err)
	}
}

func TestLoad_Defaults(t *testing.T) {
	c, err := Load(env(map[string]string{"DATABASE_URL": "postgres://x", "AUTH_DEV_USER": "dev@localhost"}))
	if err != nil {
		t.Fatal(err)
	}
	want := Config{DatabaseURL: "postgres://x", ScorerURL: "http://localhost:8000", Addr: ":8080", WebDir: "../web/dist", AuthDevUser: "dev@localhost"}
	if c != want {
		t.Fatalf("got %+v, want %+v", c, want)
	}
}

func TestLoad_Auth(t *testing.T) {
	const dom = "https://t.cloudflareaccess.com"
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string // empty: no error expected
	}{
		{"none", nil, "CF_ACCESS_TEAM_DOMAIN and CF_ACCESS_AUD, or AUTH_DEV_USER"},
		{"dev and CF domain", map[string]string{"AUTH_DEV_USER": "d", "CF_ACCESS_TEAM_DOMAIN": dom}, "AUTH_DEV_USER cannot"},
		{"dev and CF aud", map[string]string{"AUTH_DEV_USER": "d", "CF_ACCESS_AUD": "abc"}, "AUTH_DEV_USER cannot"},
		{"domain only", map[string]string{"CF_ACCESS_TEAM_DOMAIN": dom}, "CF_ACCESS_AUD"},
		{"aud only", map[string]string{"CF_ACCESS_AUD": "abc"}, "CF_ACCESS_TEAM_DOMAIN"},
		{"http domain", map[string]string{"CF_ACCESS_TEAM_DOMAIN": "http://t.cloudflareaccess.com", "CF_ACCESS_AUD": "abc"}, "https://"},
		{"https only", map[string]string{"CF_ACCESS_TEAM_DOMAIN": "https://", "CF_ACCESS_AUD": "abc"}, "https://"},
		{"no host", map[string]string{"CF_ACCESS_TEAM_DOMAIN": "https:///x", "CF_ACCESS_AUD": "abc"}, "https://"},
		{"CF ok, trailing slash", map[string]string{"CF_ACCESS_TEAM_DOMAIN": dom + "/", "CF_ACCESS_AUD": "abc"}, ""},
		{"dev ok", map[string]string{"AUTH_DEV_USER": "dev@localhost"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := map[string]string{"DATABASE_URL": "postgres://x"}
			for k, v := range tt.env {
				m[k] = v
			}
			c, err := Load(env(m))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("want error containing %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if c.AuthDevUser != tt.env["AUTH_DEV_USER"] {
				t.Fatalf("AuthDevUser = %q", c.AuthDevUser)
			}
			if tt.env["AUTH_DEV_USER"] != "" && (c.CFAccessTeamDomain != "" || c.CFAccessAUD != "") {
				t.Fatalf("CF fields set in dev mode: %q %q", c.CFAccessTeamDomain, c.CFAccessAUD)
			}
			if tt.env["CF_ACCESS_AUD"] != "" && (c.CFAccessTeamDomain != dom || c.CFAccessAUD != "abc") {
				t.Fatalf("got %q %q", c.CFAccessTeamDomain, c.CFAccessAUD)
			}
		})
	}
}
