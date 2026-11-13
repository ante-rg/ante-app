package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ante-rg/ante-app/api/internal/fakeartifacts"
)

func main() {
	if err := run(os.Args[1:], os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "fakeartifacts:", err)
		os.Exit(1)
	}
}

func run(args []string, stderr io.Writer) error {
	flags := flag.NewFlagSet("fakeartifacts", flag.ContinueOnError)
	flags.SetOutput(stderr)
	out := flags.String("out", "../artifacts/dummy-v0", "output folder, replaced if it exists")
	seed := flags.Int64("seed", 1, "generator seed")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("unexpected arguments: %v", flags.Args())
	}
	if *out == "" {
		return errors.New("-out must not be empty")
	}
	dir := filepath.Clean(*out)
	if err := checkReplaceable(dir); err != nil {
		return err
	}
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
	ds := fakeartifacts.Generate(*seed)
	if err := fakeartifacts.Write(tmp, filepath.Base(dir), ds); err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("clear %s: %w", dir, err)
	}
	if err := os.Rename(tmp, dir); err != nil {
		return fmt.Errorf("move %s into place: %w", tmp, err)
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
	_, _ = fmt.Fprintln(stderr, "model_card.json written")
	return nil
}

func checkReplaceable(out string) error {
	entries, err := os.ReadDir(out)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect %s: %w", out, err)
	}
	if len(entries) == 0 {
		return nil
	}
	var card struct {
		Synthetic bool `json:"synthetic"`
	}
	b, err := os.ReadFile(filepath.Join(out, "model_card.json"))
	if err == nil {
		err = json.Unmarshal(b, &card)
	}
	if err != nil || !card.Synthetic {
		return fmt.Errorf("refusing to replace %s: not a synthetic artifacts folder (model_card.json with synthetic true)", out)
	}
	return nil
}
