package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/ante-rg/ante-app/api/internal/db"
	"github.com/ante-rg/ante-app/api/internal/ingestkey"
	"github.com/ante-rg/ante-app/api/internal/testdb"
)

func TestRun(t *testing.T) {
	ctx := context.Background()
	url := testdb.Start(t)
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO operators (code, name) VALUES ('P3', 'P3')`); err != nil {
		t.Fatal(err)
	}
	env := func(string) string { return url }
	issue := func(op string) (string, string, error) {
		var out, errb bytes.Buffer
		err := run([]string{"-operator", op}, env, &out, &errb)
		return strings.TrimSpace(out.String()), errb.String(), err
	}
	hashOf := func() string {
		var h string
		if err := pool.QueryRow(ctx, `SELECT api_key_hash FROM operators WHERE code='P3'`).Scan(&h); err != nil {
			t.Fatal(err)
		}
		return h
	}

	key, msg, err := issue("P3")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(key, "ante_") || strings.Contains(key, "\n") {
		t.Fatalf("stdout is not one key line")
	}
	if hashOf() != ingestkey.Hash(key) {
		t.Fatal("stored hash does not match key")
	}
	if !strings.Contains(msg, "Key for P3 saved. It is shown only once; re-run to rotate.") {
		t.Fatalf("stderr = %q", msg)
	}
	key2, _, err := issue("P3")
	if err != nil {
		t.Fatal(err)
	}
	if key2 == key || hashOf() != ingestkey.Hash(key2) {
		t.Fatal("rotation did not replace the key")
	}
	if _, _, err := issue("P9"); err == nil || !strings.Contains(err.Error(), "unknown operator P9 (load artifacts first)") {
		t.Fatalf("P9 err = %v", err)
	}
	var out, errb bytes.Buffer
	err = run([]string{"-operator", "P3"}, func(string) string { return "" }, &out, &errb)
	if err == nil || err.Error() != "DATABASE_URL is required" {
		t.Fatalf("no url err = %v", err)
	}
}
