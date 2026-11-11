// Package config reads process configuration from the environment.
package config

import "errors"

type Config struct {
	DatabaseURL string
	ScorerURL   string
	Addr        string
	WebDir      string
}

func Load(getenv func(string) string) (Config, error) {
	c := Config{
		DatabaseURL: getenv("DATABASE_URL"),
		ScorerURL:   orDefault(getenv("SCORER_URL"), "http://localhost:8000"),
		Addr:        orDefault(getenv("ADDR"), ":8080"),
		WebDir:      orDefault(getenv("WEB_DIR"), "../web/dist"),
	}
	if c.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	return c, nil
}

func orDefault(v, d string) string {
	if v == "" {
		return d
	}
	return v
}
