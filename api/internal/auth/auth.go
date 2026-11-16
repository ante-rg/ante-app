package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

type Verifier interface {
	Email(ctx context.Context, rawToken string) (string, error)
}

type access struct{ v *oidc.IDTokenVerifier }

const jwksTimeout = 10 * time.Second

func NewAccess(ctx context.Context, teamDomain, aud string) Verifier {
	return NewAccessWithKeys(teamDomain, aud, oidc.NewRemoteKeySet(oidc.ClientContext(ctx, &http.Client{Timeout: jwksTimeout}), teamDomain+"/cdn-cgi/access/certs"))
}

func NewAccessWithKeys(teamDomain, aud string, keys oidc.KeySet) Verifier {
	return access{oidc.NewVerifier(teamDomain, keys, &oidc.Config{ClientID: aud})}
}

func (a access) Email(ctx context.Context, rawToken string) (string, error) {
	tok, err := a.v.Verify(ctx, rawToken)
	if err != nil {
		return "", fmt.Errorf("verify access token: %w", err)
	}
	var c struct {
		Email string `json:"email"`
	}
	if err := tok.Claims(&c); err != nil {
		return "", fmt.Errorf("read access token claims: %w", err)
	}
	if c.Email == "" {
		return "", errors.New("token has no email claim")
	}
	return c.Email, nil
}

type emailKey struct{}

func WithEmail(ctx context.Context, email string) context.Context {
	return context.WithValue(ctx, emailKey{}, email)
}

func Email(ctx context.Context) (string, bool) {
	e, _ := ctx.Value(emailKey{}).(string)
	return e, e != ""
}
