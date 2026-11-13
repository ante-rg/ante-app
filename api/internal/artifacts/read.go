package artifacts

import (
	"bytes"
	"errors"
	"io"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/parquet-go/parquet-go"
)

type Player struct {
	PlayerID, OperatorCode              string
	LastDay, SessionCount, DepositCount int32
}

type Score struct {
	PlayerID string
	Day      int32
	Score    *float64
	Reasons  string
}

type Activity struct {
	PlayerID                       string
	Day, DepositsN                 int32
	DepositsEUR                    float64
	SessionsN                      int32
	PlayMinutes, NetResultEUR      float64
	ChaseSessionsN, NightSessionsN int32
	EndBalanceEUR                  *float64
}

type Session struct {
	PlayerID                                  string
	SessionNo, StartDay, DurationS, DepositsN int32
	DepositsEUR, NetResultEUR                 float64
	IsNight, IsChase                          bool
}

type RuleFlag struct {
	PlayerID string
	Day      int32
	RuleID   string
	RuleText string
}

type Sample struct {
	PlayerID, Source string
	Position         int32
}

const chunkRows = 1024

type dec struct {
	file string
	cols []col
	vals []parquet.Value
	row  int
	err  *Error
}

func (d *dec) fail(i int, rule string) {
	if d.err == nil {
		d.err = &Error{File: d.file, Row: d.row, Column: d.cols[i].name, Rule: rule}
	}
}

func (d *dec) value(i int) (parquet.Value, bool) {
	v := d.vals[i]
	if v.IsNull() {
		if !d.cols[i].nullable {
			d.fail(i, "must not be null")
		}
		return v, false
	}
	return v, true
}

func (d *dec) str(i int) string {
	if v, ok := d.value(i); ok {
		b := v.ByteArray()
		if !utf8.Valid(b) || bytes.IndexByte(b, 0) >= 0 {
			d.fail(i, "must be valid UTF-8 without NUL bytes")
			return ""
		}
		return string(b)
	}
	return ""
}

func (d *dec) int(i int) int32 {
	v, ok := d.value(i)
	if !ok {
		return 0
	}
	n := v.Int64()
	if v.Kind() == parquet.Int32 {
		n = int64(v.Int32())
	}
	if n < math.MinInt32 || n > math.MaxInt32 {
		d.fail(i, "out of range")
		return 0
	}
	return int32(n)
}

func (d *dec) float(i int) float64 {
	v, ok := d.value(i)
	if !ok {
		return 0
	}
	if v.Kind() == parquet.Float {
		return float64(v.Float())
	}
	return v.Double()
}

func (d *dec) optFloat(i int) *float64 {
	if d.vals[i].IsNull() {
		return nil
	}
	x := d.float(i)
	return &x
}

func (d *dec) bool(i int) bool {
	v, ok := d.value(i)
	return ok && v.Boolean()
}

func scan[T any](fo *Folder, file string, cols []col, build func(*dec) T, fn func(int, T) error) error {
	pq, fh, err := fo.openParquet(file)
	if err != nil {
		return err
	}
	defer func() { _ = fh.Close() }()
	pos := map[string]int{}
	for ci, p := range pq.Schema().Columns() {
		pos[strings.Join(p, ".")] = ci
	}
	idx := make([]int, len(cols))
	for ci, c := range cols {
		p, ok := pos[c.name]
		if !ok {
			return &Error{File: file, Rule: "missing column " + c.name}
		}
		idx[ci] = p
	}
	d := &dec{file: file, cols: cols, vals: make([]parquet.Value, len(cols))}
	byLeaf := make([]parquet.Value, len(pos))
	buf := make([]parquet.Row, chunkRows)
	for _, rg := range pq.RowGroups() {
		if err := scanGroup(rg, buf, byLeaf, idx, d, build, fn); err != nil {
			return err
		}
	}
	return nil
}

func scanGroup[T any](rg parquet.RowGroup, buf []parquet.Row, byLeaf []parquet.Value, idx []int, d *dec,
	build func(*dec) T, fn func(int, T) error,
) (err error) {
	rows := rg.Rows()
	defer func() {
		if cerr := rows.Close(); err == nil && cerr != nil {
			err = &Error{File: d.file, Rule: "cannot read file"}
		}
	}()
	for {
		n, rerr := rows.ReadRows(buf)
		for _, r := range buf[:n] {
			for _, v := range r {
				byLeaf[v.Column()] = v
			}
			for ci, p := range idx {
				d.vals[ci] = byLeaf[p]
			}
			d.row++
			v := build(d)
			if d.err != nil {
				return d.err
			}
			if err := fn(d.row, v); err != nil {
				return err
			}
		}
		if errors.Is(rerr, io.EOF) {
			return nil
		}
		if rerr != nil {
			return &Error{File: d.file, Rule: "cannot read file"}
		}
	}
}

func (fo *Folder) Players(fn func(row int, v Player) error) error {
	return scan(fo, FilePlayers, colsPlayers, func(d *dec) Player {
		return Player{d.str(0), d.str(1), d.int(2), d.int(3), d.int(4)}
	}, fn)
}

func (fo *Folder) Scores(fn func(row int, v Score) error) error {
	return scan(fo, FileScores, colsScores, func(d *dec) Score {
		return Score{d.str(0), d.int(1), d.optFloat(2), d.str(3)}
	}, fn)
}

func (fo *Folder) Activity(fn func(row int, v Activity) error) error {
	return scan(fo, FileActivity, colsActivity, func(d *dec) Activity {
		return Activity{d.str(0), d.int(1), d.int(2), d.float(3), d.int(4), d.float(5), d.float(6),
			d.int(7), d.int(8), d.optFloat(9)}
	}, fn)
}

func (fo *Folder) Sessions(fn func(row int, v Session) error) error {
	return scan(fo, FileSessions, colsSessions, func(d *dec) Session {
		return Session{d.str(0), d.int(1), d.int(2), d.int(3), d.int(4), d.float(5), d.float(6),
			d.bool(7), d.bool(8)}
	}, fn)
}

func (fo *Folder) RuleFlags(fn func(row int, v RuleFlag) error) error {
	return scan(fo, FileFlags, colsFlags, func(d *dec) RuleFlag {
		return RuleFlag{d.str(0), d.int(1), d.str(2), d.str(3)}
	}, fn)
}

func (fo *Folder) Sample(fn func(row int, v Sample) error) error {
	return scan(fo, FileSample, colsSample, func(d *dec) Sample {
		return Sample{d.str(0), d.str(1), d.int(2)}
	}, fn)
}
