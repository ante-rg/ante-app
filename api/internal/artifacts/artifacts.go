package artifacts

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/parquet-go/parquet-go"
)

const (
	FilePlayers  = "players.parquet"
	FileScores   = "daily_scores.parquet"
	FileActivity = "daily_activity.parquet"
	FileSessions = "sessions.parquet"
	FileFlags    = "rule_flags.parquet"
	FileSample   = "review_sample.parquet"
	FileCard     = "model_card.json"
)

type Error struct {
	File   string
	Row    int
	Column string
	Rule   string
}

func (e *Error) Error() string {
	switch {
	case e.Row > 0:
		return fmt.Sprintf("%s: row %d: %s: %s", e.File, e.Row, e.Column, e.Rule)
	case e.Column != "":
		return fmt.Sprintf("%s: %s: %s", e.File, e.Column, e.Rule)
	default:
		return fmt.Sprintf("%s: %s", e.File, e.Rule)
	}
}

type kind int

const (
	kStr kind = iota
	kInt
	kFloat
	kBool
)

type col struct {
	name     string
	kind     kind
	nullable bool
}

func s(n string) col    { return col{name: n, kind: kStr} }
func i(n string) col    { return col{name: n, kind: kInt} }
func f(n string) col    { return col{name: n, kind: kFloat} }
func b(n string) col    { return col{name: n, kind: kBool} }
func (c col) null() col { c.nullable = true; return c }

var (
	colsPlayers  = []col{s("player_id"), s("operator_code"), i("last_day"), i("session_count"), i("deposit_count")}
	colsScores   = []col{s("player_id"), i("day"), f("score").null(), s("reasons")}
	colsActivity = []col{s("player_id"), i("day"), i("deposits_n"), f("deposits_eur"), i("sessions_n"),
		f("play_minutes"), f("net_result_eur"), i("chase_sessions_n"), i("night_sessions_n"),
		f("end_balance_eur").null()}
	colsSessions = []col{s("player_id"), i("session_no"), i("start_day"), i("duration_s"), i("deposits_n"),
		f("deposits_eur"), f("net_result_eur"), b("is_night"), b("is_chase")}
	colsFlags  = []col{s("player_id"), i("day"), s("rule_id"), s("rule_text")}
	colsSample = []col{s("player_id"), s("source"), i("position")}
)

var parquetFiles = []struct {
	name string
	cols []col
}{
	{FilePlayers, colsPlayers}, {FileScores, colsScores}, {FileActivity, colsActivity},
	{FileSessions, colsSessions}, {FileFlags, colsFlags}, {FileSample, colsSample},
}

type Folder struct {
	Dir          string
	ModelVersion string
	Card         []byte
}

func Open(dir string) (*Folder, error) {
	dir = filepath.Clean(dir)
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return nil, fmt.Errorf("artifacts folder %q not found or not a directory", dir)
	}
	fo := &Folder{Dir: dir, ModelVersion: filepath.Base(dir)}
	for _, pf := range parquetFiles {
		if _, err := os.Stat(filepath.Join(dir, pf.name)); err != nil {
			return nil, &Error{File: pf.name, Rule: "file missing"}
		}
	}
	card, err := os.ReadFile(filepath.Join(dir, FileCard))
	if err != nil {
		return nil, &Error{File: FileCard, Rule: "file missing"}
	}
	var m map[string]any
	if json.Unmarshal(card, &m) != nil || m == nil {
		return nil, &Error{File: FileCard, Rule: "must be a JSON object"}
	}
	if v, ok := m["model_version"].(string); !ok || v != fo.ModelVersion {
		return nil, &Error{File: FileCard, Rule: "model_version must equal the folder name " + fo.ModelVersion}
	}
	fo.Card = card
	for _, pf := range parquetFiles {
		if err := fo.checkSchema(pf.name, pf.cols); err != nil {
			return nil, err
		}
	}
	return fo, nil
}

func (fo *Folder) openParquet(name string) (*parquet.File, *os.File, error) {
	fh, err := os.Open(filepath.Join(fo.Dir, name))
	if err != nil {
		return nil, nil, &Error{File: name, Rule: "file missing"}
	}
	st, err := fh.Stat()
	if err != nil {
		_ = fh.Close()
		return nil, nil, &Error{File: name, Rule: "cannot read file"}
	}
	pq, err := parquet.OpenFile(fh, st.Size())
	if err != nil {
		_ = fh.Close()
		return nil, nil, &Error{File: name, Rule: "not a valid parquet file"}
	}
	return pq, fh, nil
}

func (fo *Folder) checkSchema(name string, want []col) error {
	pq, fh, err := fo.openParquet(name)
	if err != nil {
		return err
	}
	defer func() { _ = fh.Close() }()
	got := map[string]parquet.Node{}
	for _, fd := range pq.Schema().Fields() {
		got[fd.Name()] = fd
	}
	for _, c := range want {
		fd, ok := got[c.name]
		if !ok {
			return &Error{File: name, Rule: "missing column " + c.name}
		}
		if !typeOK(fd, c.kind) {
			return &Error{File: name, Column: c.name, Rule: "wrong type"}
		}
		delete(got, c.name)
	}
	for _, fd := range pq.Schema().Fields() {
		if _, extra := got[fd.Name()]; extra {
			return &Error{File: name, Rule: "unexpected column " + fd.Name()}
		}
	}
	return nil
}

func typeOK(n parquet.Node, k kind) bool {
	if !n.Leaf() || n.Repeated() {
		return false
	}
	switch pk := n.Type().Kind(); k {
	case kStr:
		return pk == parquet.ByteArray
	case kInt:
		return pk == parquet.Int32 || pk == parquet.Int64
	case kFloat:
		return pk == parquet.Float || pk == parquet.Double
	default:
		return pk == parquet.Boolean
	}
}
