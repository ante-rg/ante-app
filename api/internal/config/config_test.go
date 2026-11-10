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
	c, err := Load(env(map[string]string{"DATABASE_URL": "postgres://x"}))
	if err != nil {
		t.Fatal(err)
	}
	want := Config{DatabaseURL: "postgres://x", ScorerURL: "http://localhost:8000", Addr: ":8080", WebDir: "../web/dist"}
	if c != want {
		t.Fatalf("got %+v, want %+v", c, want)
	}
}
