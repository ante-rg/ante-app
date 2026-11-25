package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-jose/go-jose/v4"
)

const (
	testTeam = "https://t.cloudflareaccess.com"
	testAud  = "aud1"
)

func mint(t *testing.T, key *rsa.PrivateKey, claims map[string]any) string {
	t.Helper()
	opts := (&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "k1")
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, opts)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	jws, err := signer.Sign(payload)
	if err != nil {
		t.Fatal(err)
	}
	tok, err := jws.CompactSerialize()
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func TestAccess_Email(t *testing.T) {
	good, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	v := NewAccessWithKeys(testTeam, testAud, &oidc.StaticKeySet{PublicKeys: []crypto.PublicKey{&good.PublicKey}})
	now := time.Now()
	base := func() map[string]any {
		return map[string]any{
			"iss":   testTeam,
			"aud":   []string{testAud},
			"exp":   now.Add(5 * time.Minute).Unix(),
			"iat":   now.Unix(),
			"email": "rg@example.com",
		}
	}

	tests := []struct {
		name   string
		key    *rsa.PrivateKey
		change func(map[string]any)
		raw    string
		want   string
		fail   bool
	}{
		{name: "valid", key: good, want: "rg@example.com"},
		{name: "aud among several", key: good, change: func(c map[string]any) { c["aud"] = []string{"other", testAud} }, want: "rg@example.com"},
		{name: "wrong key", key: other, fail: true},
		{name: "expired", key: good, change: func(c map[string]any) { c["exp"] = now.Add(-10 * time.Minute).Unix() }, fail: true},
		{name: "wrong aud", key: good, change: func(c map[string]any) { c["aud"] = []string{"other"} }, fail: true},
		{name: "wrong iss", key: good, change: func(c map[string]any) { c["iss"] = "https://evil.cloudflareaccess.com" }, fail: true},
		{name: "no email", key: good, change: func(c map[string]any) { delete(c, "email"); c["common_name"] = "x.access" }, fail: true},
		{name: "malformed", raw: "not-a-jwt", fail: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			raw := tc.raw
			if raw == "" {
				claims := maps.Clone(base())
				if tc.change != nil {
					tc.change(claims)
				}
				raw = mint(t, tc.key, claims)
			}
			got, err := v.Email(context.Background(), raw)
			if tc.fail {
				if err == nil {
					t.Fatalf("want error, got email %q", got)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("got (%q, %v), want (%q, nil)", got, err, tc.want)
			}
		})
	}
}

func TestNewAccess_RemoteKeys(t *testing.T) {
	good, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cdn-cgi/access/certs" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{
			{Key: &good.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"},
		}})
	}))
	t.Cleanup(srv.Close)

	v := NewAccess(context.Background(), srv.URL, testAud)
	now := time.Now()
	tok := mint(t, good, map[string]any{
		"iss":   srv.URL,
		"aud":   []string{testAud},
		"exp":   now.Add(5 * time.Minute).Unix(),
		"iat":   now.Unix(),
		"email": "rg@example.com",
	})
	got, err := v.Email(context.Background(), tok)
	if err != nil || got != "rg@example.com" {
		t.Fatalf("got (%q, %v), want (%q, nil)", got, err, "rg@example.com")
	}
}

func TestEmailContext(t *testing.T) {
	if got, ok := Email(context.Background()); got != "" || ok {
		t.Fatalf("empty ctx: got (%q, %v)", got, ok)
	}
	if got, ok := Email(WithEmail(context.Background(), "a@b")); got != "a@b" || !ok {
		t.Fatalf("got (%q, %v)", got, ok)
	}
}
