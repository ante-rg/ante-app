package ingestkey

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	a, err := New()
	if err != nil {
		t.Fatal(err)
	}
	b, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("two keys are equal")
	}
	if !strings.HasPrefix(a, "ante_") || len(a) != 5+43 {
		t.Fatalf("bad key shape: len %d", len(a))
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(a, "ante_"))
	if err != nil || len(raw) != 32 {
		t.Fatalf("suffix decodes to %d bytes, err %v", len(raw), err)
	}
}

func TestHash(t *testing.T) {
	sum := sha256.Sum256([]byte("ante_x"))
	want := hex.EncodeToString(sum[:])
	if got := Hash("ante_x"); got != want || len(got) != 64 {
		t.Fatalf("Hash = %q, want %q", got, want)
	}
}
