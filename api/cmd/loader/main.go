package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/ante-rg/ante-app/api/internal/artifacts"
	"github.com/ante-rg/ante-app/api/internal/db"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Getenv, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "loader:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, getenv func(string) string, stderr io.Writer) error {
	flags := flag.NewFlagSet("loader", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dir := flags.String("dir", "", "artifacts folder (required); its name is the model version")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("unexpected arguments: %v", flags.Args())
	}
	if *dir == "" {
		return errors.New("-dir is required")
	}
	url := getenv("DATABASE_URL")
	if url == "" {
		return errors.New("DATABASE_URL is required")
	}
	f, err := artifacts.Open(*dir)
	if err != nil {
		return err
	}
	if err := f.Validate(); err != nil {
		return err
	}
	pool, err := db.Open(ctx, url)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		return err
	}
	n, err := load(ctx, pool, f)
	if err != nil {
		return err
	}
	for _, t := range []string{"players", "daily_scores", "daily_activity", "sessions", "rule_flags", "review_sample"} {
		_, _ = fmt.Fprintf(stderr, "%s %d rows\n", t, n[t])
	}
	_, _ = fmt.Fprintf(stderr, "active model: %s\n", f.ModelVersion)
	return nil
}
