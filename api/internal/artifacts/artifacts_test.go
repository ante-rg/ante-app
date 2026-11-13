package artifacts

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/parquet-go/parquet-go"

	"github.com/ante-rg/ante-app/api/internal/fakeartifacts"
)

func asErr(t *testing.T, err error) *Error {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("want *Error, got %v", err)
	}
	return e
}

func TestOpen_CleanFolder(t *testing.T) {
	fo, err := Open(dummy(t))
	if err != nil {
		t.Fatal(err)
	}
	if fo.ModelVersion != "dummy-v0" || len(fo.Card) == 0 {
		t.Fatalf("bad folder %+v", fo)
	}
	ds := fakeartifacts.Generate(1)
	var n [6]int
	count := func(k int) func(int, any) error { return func(int, any) error { n[k]++; return nil } }
	check := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	check(fo.Players(func(r int, v Player) error { return count(0)(r, v) }))
	check(fo.Scores(func(r int, v Score) error { return count(1)(r, v) }))
	check(fo.Activity(func(r int, v Activity) error { return count(2)(r, v) }))
	check(fo.Sessions(func(r int, v Session) error { return count(3)(r, v) }))
	check(fo.RuleFlags(func(r int, v RuleFlag) error { return count(4)(r, v) }))
	check(fo.Sample(func(r int, v Sample) error { return count(5)(r, v) }))
	want := [6]int{len(ds.Players), len(ds.Scores), len(ds.Activity), len(ds.Sessions), len(ds.RuleFlags), len(ds.Sample)}
	if n != want {
		t.Fatalf("row counts %v, want %v", n, want)
	}
}

func TestOpen_TrailingSlashAndRelative(t *testing.T) {
	dir := dummy(t)
	if fo, err := Open(dir + "/"); err != nil || fo.ModelVersion != "dummy-v0" {
		t.Fatalf("trailing slash: %v %v", fo, err)
	}
	cwd, _ := os.Getwd()
	rel, err := filepath.Rel(cwd, dir)
	if err != nil {
		t.Skip("no relative path")
	}
	if fo, err := Open(rel); err != nil || fo.ModelVersion != "dummy-v0" {
		t.Fatalf("relative: %v %v", fo, err)
	}
}

func TestOpen_MissingFolder(t *testing.T) {
	_, err := Open(filepath.Join(t.TempDir(), "nope"))
	if err == nil || !strings.Contains(err.Error(), "folder") {
		t.Fatalf("got %v", err)
	}
}

var cases = map[string]func(t *testing.T) error{
	"missing file": func(t *testing.T) error {
		dir := dummy(t)
		_ = os.Remove(filepath.Join(dir, FileSessions))
		_, err := Open(dir)
		return err
	},
	"card mismatch": func(t *testing.T) error {
		dir := dummy(t)
		_ = os.WriteFile(filepath.Join(dir, FileCard), []byte(`{"model_version":"other"}`), 0o644)
		_, err := Open(dir)
		return err
	},
	"missing column": func(t *testing.T) error {
		dir := dummy(t)
		writeFile(t, dir, FileSessions, []sessionNoChase{{PlayerID: leak}})
		_, err := Open(dir)
		return err
	},
	"extra column": func(t *testing.T) error {
		dir := dummy(t)
		writeFile(t, dir, FileSessions, []sessionExtra{{sessionNoChase{PlayerID: leak}, true, leak}})
		_, err := Open(dir)
		return err
	},
	"wrong type": func(t *testing.T) error {
		dir := dummy(t)
		writeFile(t, dir, FileScores, []scoreStringDay{{PlayerID: leak, Day: leak, Reasons: "[]"}})
		_, err := Open(dir)
		return err
	},
	"int64 out of range": func(t *testing.T) error {
		fo := openWith(t, FilePlayers, playersI64(5, func(p *playerI64) { p.LastDay = ptr(int64(1) << 40) }))
		return fo.Players(func(int, Player) error { return nil })
	},
	"null in required": func(t *testing.T) error {
		rows := make([]sessionOptDuration, 5)
		for k := range rows {
			rows[k] = sessionOptDuration{PlayerID: leak, DurationS: ptr(int32(k))}
		}
		rows[2].DurationS = nil
		fo := openWith(t, FileSessions, rows)
		return fo.Sessions(func(int, Session) error { return nil })
	},
	"null across row groups": func(t *testing.T) error {
		rows := make([]playerI64, 100)
		for k := range rows {
			rows[k] = playerI64{ptr(leak), ptr("P1"), ptr(int64(k)), ptr(int64(1)), ptr(int64(1))}
		}
		rows[72].LastDay = nil
		fo := openWith(t, FilePlayers, rows, parquet.MaxRowsPerRowGroup(50))
		return fo.Players(func(int, Player) error { return nil })
	},
}

func playersI64(n int, edit func(*playerI64)) []playerI64 {
	rows := make([]playerI64, 100)
	for k := range rows {
		rows[k] = playerI64{ptr(leak), ptr("P1"), ptr(int64(k)), ptr(int64(1)), ptr(int64(1))}
	}
	edit(&rows[n-1])
	return rows
}

func openWith[T any](t *testing.T, name string, rows []T, opts ...parquet.WriterOption) *Folder {
	t.Helper()
	dir := dummy(t)
	writeFile(t, dir, name, rows, opts...)
	fo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return fo
}

func TestOpen_MissingFile(t *testing.T) {
	e := asErr(t, cases["missing file"](t))
	if e.File != FileSessions || !strings.Contains(e.Rule, "missing") {
		t.Fatalf("%+v", e)
	}
}

func TestOpen_CardVersionMismatch(t *testing.T) {
	err := cases["card mismatch"](t)
	e := asErr(t, err)
	if e.File != FileCard || !strings.Contains(e.Rule, "dummy-v0") || strings.Contains(err.Error(), "other") {
		t.Fatalf("%+v", e)
	}
}

func TestOpen_MissingColumn(t *testing.T) {
	if e := asErr(t, cases["missing column"](t)); e.File != FileSessions || !strings.Contains(e.Rule, "missing column is_chase") {
		t.Fatalf("%+v", e)
	}
}

func TestOpen_ExtraColumn(t *testing.T) {
	if e := asErr(t, cases["extra column"](t)); e.File != FileSessions || !strings.Contains(e.Rule, "unexpected column foo") {
		t.Fatalf("%+v", e)
	}
}

func TestOpen_WrongType(t *testing.T) {
	if e := asErr(t, cases["wrong type"](t)); e.File != FileScores || e.Column != "day" {
		t.Fatalf("%+v", e)
	}
}

func TestScan_Int64AndOptionalAccepted(t *testing.T) {
	ds := fakeartifacts.Generate(1)
	rows := make([]playerI64, len(ds.Players))
	for k, p := range ds.Players {
		rows[k] = playerI64{ptr(p.ID), ptr(p.Operator), ptr(int64(p.LastDay)), ptr(int64(p.SessionCount)), ptr(int64(p.DepositCount))}
	}
	fo := openWith(t, FilePlayers, rows)
	var got []Player
	if err := fo.Players(func(_ int, v Player) error { got = append(got, v); return nil }); err != nil {
		t.Fatal(err)
	}
	if len(got) != 100 {
		t.Fatalf("got %d rows", len(got))
	}
	for k, p := range ds.Players {
		if got[k] != (Player{p.ID, p.Operator, p.LastDay, p.SessionCount, p.DepositCount}) {
			t.Fatalf("row %d differs: %+v", k+1, got[k])
		}
	}
	if err := fo.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestScan_Int64OutOfRange(t *testing.T) {
	e := asErr(t, cases["int64 out of range"](t))
	if e.File != FilePlayers || e.Row != 5 || e.Column != "last_day" || e.Rule != "out of range" {
		t.Fatalf("%+v", e)
	}
}

func TestScan_NullInRequiredColumn(t *testing.T) {
	e := asErr(t, cases["null in required"](t))
	if e.Row != 3 || e.Column != "duration_s" || e.Rule != "must not be null" {
		t.Fatalf("%+v", e)
	}
}

func TestScan_RowNumbersAcrossRowGroups(t *testing.T) {
	e := asErr(t, cases["null across row groups"](t))
	if e.Row != 73 || e.Column != "last_day" {
		t.Fatalf("%+v", e)
	}
}

func TestScan_FnErrorPassesThrough(t *testing.T) {
	fo, err := Open(dummy(t))
	if err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("stop")
	err = fo.Players(func(row int, _ Player) error {
		if row == 2 {
			return sentinel
		}
		return nil
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("got %v", err)
	}
}

func TestError_NoValues(t *testing.T) {
	for name, build := range cases {
		if err := build(t); err == nil || strings.Contains(err.Error(), leak) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestError_Format(t *testing.T) {
	for _, c := range []struct {
		e    Error
		want string
	}{
		{Error{"a.parquet", 3, "x", "bad"}, "a.parquet: row 3: x: bad"},
		{Error{"a.parquet", 0, "x", "bad"}, "a.parquet: x: bad"},
		{Error{"a.parquet", 0, "", "bad"}, "a.parquet: bad"},
	} {
		if got := c.e.Error(); got != c.want {
			t.Errorf("got %q, want %q", got, c.want)
		}
	}
}
