package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ante-rg/ante-app/api/internal/db"
	"github.com/ante-rg/ante-app/api/internal/ingestkey"
	"github.com/ante-rg/ante-app/api/internal/store"
)

func main() {
	if err := run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "apikey:", err)
		os.Exit(1)
	}
}

func run(args []string, getenv func(string) string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("apikey", flag.ContinueOnError)
	flags.SetOutput(stderr)
	operator := flags.String("operator", "", "brand code, e.g. P3 (required)")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("unexpected arguments: %v", flags.Args())
	}
	if *operator == "" {
		return errors.New("-operator is required")
	}
	url := getenv("DATABASE_URL")
	if url == "" {
		return errors.New("DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := db.Open(ctx, url)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		return err
	}
	key, err := ingestkey.New()
	if err != nil {
		return err
	}
	n, err := store.New(pool).SetOperatorKeyHash(ctx, store.SetOperatorKeyHashParams{Hash: pgtype.Text{String: ingestkey.Hash(key), Valid: true}, Code: *operator})
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("unknown operator %s (load artifacts first)", *operator)
	}
	_, _ = fmt.Fprintln(stdout, key)
	_, _ = fmt.Fprintf(stderr, "Key for %s saved. It is shown only once; re-run to rotate.\n", *operator)
	return nil
}
