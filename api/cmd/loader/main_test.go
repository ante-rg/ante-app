package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ante-rg/ante-app/api/internal/artifacts"
	"github.com/ante-rg/ante-app/api/internal/fakeartifacts"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestRun_RequiresDir(t *testing.T) {
	err := run(context.Background(), nil, env(map[string]string{"DATABASE_URL": "x"}), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "-dir") {
		t.Fatalf("want -dir error, got %v", err)
	}
}

func TestRun_RequiresDatabaseURL(t *testing.T) {
	err := run(context.Background(), []string{"-dir", "x"}, env(nil), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("want DATABASE_URL error, got %v", err)
	}
}

func TestRun_InvalidFolderStopsBeforeDB(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "dummy-v0")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fakeartifacts.Write(dir, "dummy-v0", fakeartifacts.Generate(1)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, artifacts.FileSessions), []byte("junk"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := run(context.Background(), []string{"-dir", dir}, env(map[string]string{"DATABASE_URL": "postgres://invalid"}), &bytes.Buffer{})
	var ae *artifacts.Error
	if !errors.As(err, &ae) {
		t.Fatalf("want *artifacts.Error, got %v", err)
	}
}

func TestRun_HelpIsNotAnError(t *testing.T) {
	if err := run(context.Background(), []string{"-h"}, env(nil), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
}

func TestRun_RejectsPositionalArgs(t *testing.T) {
	if err := run(context.Background(), []string{"-dir", "x", "extra"}, env(nil), &bytes.Buffer{}); err == nil {
		t.Fatal("want error")
	}
}

func TestDBErr_NoDetail(t *testing.T) {
	err := dbErr("players", &pgconn.PgError{
		Code: "23505", ConstraintName: "players_pkey",
		Detail: "Key (player_id)=(LEAK-MARKER) already exists.", Message: "duplicate key LEAK-MARKER",
	})
	if got, want := err.Error(), "load players: players_pkey (23505)"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDBErr_NoConstraint(t *testing.T) {
	for _, c := range []struct {
		pe   pgconn.PgError
		want string
	}{
		{pgconn.PgError{Code: "23502", ColumnName: "player_id"}, "load players: player_id (23502)"},
		{pgconn.PgError{Code: "22P02"}, "load players: 22P02"},
	} {
		c.pe.Detail, c.pe.Message = "LEAK-MARKER", "LEAK-MARKER"
		if got := dbErr("players", &c.pe).Error(); got != c.want {
			t.Fatalf("got %q want %q", got, c.want)
		}
	}
}
