// Package config reads process configuration from the environment.
package config

import (
	"errors"
	"net/url"
	"strings"
)

type Config struct {
	DatabaseURL string
	ScorerURL   string
	Addr        string
	WebDir      string

	AuthDevUser        string
	CFAccessTeamDomain string
	CFAccessAUD        string
}

func Load(getenv func(string) string) (Config, error) {
	c := Config{
		DatabaseURL: getenv("DATABASE_URL"),
		ScorerURL:   orDefault(getenv("SCORER_URL"), "http://localhost:8000"),
		Addr:        orDefault(getenv("ADDR"), ":8080"),
		WebDir:      orDefault(getenv("WEB_DIR"), "../web/dist"),

		AuthDevUser:        getenv("AUTH_DEV_USER"),
		CFAccessTeamDomain: strings.TrimSuffix(getenv("CF_ACCESS_TEAM_DOMAIN"), "/"),
		CFAccessAUD:        getenv("CF_ACCESS_AUD"),
	}
	if c.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if err := c.checkAuth(); err != nil {
		return Config{}, err
	}
	return c, nil
}

func (c Config) checkAuth() error {
	cf := c.CFAccessTeamDomain != "" || c.CFAccessAUD != ""
	switch {
	case c.AuthDevUser != "" && cf:
		return errors.New("AUTH_DEV_USER cannot be combined with CF_ACCESS_TEAM_DOMAIN or CF_ACCESS_AUD")
	case c.AuthDevUser == "" && !cf:
		return errors.New("set CF_ACCESS_TEAM_DOMAIN and CF_ACCESS_AUD, or AUTH_DEV_USER for local development")
	case cf && (c.CFAccessTeamDomain == "" || c.CFAccessAUD == ""):
		return errors.New("CF_ACCESS_TEAM_DOMAIN and CF_ACCESS_AUD must both be set")
	}
	if cf {
		u, err := url.Parse(c.CFAccessTeamDomain)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			return errors.New("CF_ACCESS_TEAM_DOMAIN must be https://<team>.cloudflareaccess.com")
		}
	}
	return nil
}

func orDefault(v, d string) string {
	if v == "" {
		return d
	}
	return v
}
