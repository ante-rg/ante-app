package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ante-rg/ante-app/api/internal/baseline"
	"github.com/ante-rg/ante-app/api/internal/fakeartifacts"
)

const (
	maxDefaultDay = 120
	deriveSeed    = 1
)

func main() {
	if err := run(os.Args[1:], os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "baseline:", err)
		os.Exit(1)
	}
}

func run(args []string, stderr io.Writer) error {
	flags := flag.NewFlagSet("baseline", flag.ContinueOnError)
	flags.SetOutput(stderr)
	in := flags.String("in", "", "folder with the v4 batch folders (required)")
	out := flags.String("out", "../artifacts/v4-baseline-v0", "output folder, must not exist or be empty")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("unexpected arguments: %v", flags.Args())
	}
	if *in == "" {
		return errors.New("-in is required")
	}
	if *out == "" {
		return errors.New("-out must not be empty")
	}
	dir := filepath.Clean(*out)
	if err := checkEmpty(dir); err != nil {
		return err
	}
	players, sessions, act, skipped, err := baseline.Build(*in)
	if err != nil {
		return err
	}
	var lastDay int32
	for _, p := range players {
		lastDay = max(lastDay, p.LastDay)
	}
	ds := fakeartifacts.Derive(players, sessions, act, min(maxDefaultDay, lastDay), deriveSeed)

	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return fmt.Errorf("create parent of %s: %w", dir, err)
	}
	tmp, err := os.MkdirTemp(filepath.Dir(dir), filepath.Base(dir)+".tmp-")
	if err != nil {
		return fmt.Errorf("create temp folder: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	if err := os.Chmod(tmp, 0o755); err != nil {
		return fmt.Errorf("chmod %s: %w", tmp, err)
	}
	if err := fakeartifacts.Write(tmp, ds, baseline.Card(filepath.Base(dir), ds, skipped)); err != nil {
		return err
	}
	if err := moveIntoPlace(tmp, dir); err != nil {
		return err
	}
	for _, l := range []struct {
		name string
		n    int
	}{
		{"players", len(ds.Players)}, {"daily_scores", len(ds.Scores)}, {"daily_activity", len(ds.Activity)},
		{"sessions", len(ds.Sessions)}, {"rule_flags", len(ds.RuleFlags)}, {"review_sample", len(ds.Sample)},
	} {
		_, _ = fmt.Fprintf(stderr, "%s.parquet %d rows\n", l.name, l.n)
	}
	_, _ = fmt.Fprintf(stderr, "skipped accounts (no EUR rate): %d\n", skipped)
	_, _ = fmt.Fprintln(stderr, "model_card.json written")
	return nil
}

func moveIntoPlace(tmp, dir string) error {
	if err := os.Remove(dir); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("move %s into place: %s is not empty, delete it first: %w", tmp, dir, err)
	}
	if err := os.Rename(tmp, dir); err != nil {
		return fmt.Errorf("move %s into place: %s may not be empty, delete it first: %w", tmp, dir, err)
	}
	return nil
}

func checkEmpty(out string) error {
	entries, err := os.ReadDir(out)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect %s: %w", out, err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("%s is not empty; delete it first", out)
	}
	return nil
}
