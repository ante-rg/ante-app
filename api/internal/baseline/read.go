package baseline

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/parquet-go/parquet-go"
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

const chunkRows = 1024

type kind int

const (
	kStr kind = iota
	kInt
	kFloat
)

type col struct {
	name string
	kind kind
	opt  bool
}

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

func (d *dec) null(i int) bool {
	if !d.vals[i].IsNull() {
		return false
	}
	if !d.cols[i].opt {
		d.fail(i, "must not be null")
	}
	return true
}

func (d *dec) str(i int) string {
	if d.null(i) {
		return ""
	}
	b := d.vals[i].ByteArray()
	if !utf8.Valid(b) || bytes.IndexByte(b, 0) >= 0 {
		d.fail(i, "must be valid UTF-8 without NUL bytes")
		return ""
	}
	return string(b)
}

func (d *dec) int(i int) int64 {
	if d.null(i) {
		return 0
	}
	v := d.vals[i]
	if v.Kind() == parquet.Int32 {
		return int64(v.Int32())
	}
	return v.Int64()
}

func (d *dec) float(i int) (float64, bool) {
	if d.null(i) {
		return 0, false
	}
	v := d.vals[i]
	x := v.Double()
	if v.Kind() == parquet.Float {
		x = float64(v.Float())
	}
	if math.IsNaN(x) || math.IsInf(x, 0) {
		d.fail(i, "must be finite")
		return 0, false
	}
	return x, true
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
	default:
		return pk == parquet.Float || pk == parquet.Double
	}
}

func scan(dir, file string, cols []col, fn func(d *dec) error) error {
	fh, err := os.Open(filepath.Join(dir, file))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &Error{File: file, Rule: "missing in " + dir}
		}
		return &Error{File: file, Rule: "cannot open in " + dir}
	}
	defer func() { _ = fh.Close() }()
	st, err := fh.Stat()
	if err != nil {
		return &Error{File: file, Rule: "cannot read file"}
	}
	pq, err := parquet.OpenFile(fh, st.Size())
	if err != nil {
		return &Error{File: file, Rule: "not a valid parquet file"}
	}
	pos := map[string]int{}
	for ci, p := range pq.Schema().Columns() {
		pos[strings.Join(p, ".")] = ci
	}
	fields := map[string]parquet.Node{}
	for _, fd := range pq.Schema().Fields() {
		fields[fd.Name()] = fd
	}
	idx := make([]int, len(cols))
	for ci, c := range cols {
		p, ok := pos[c.name]
		if !ok {
			return &Error{File: file, Rule: "missing column " + c.name}
		}
		if !typeOK(fields[c.name], c.kind) {
			return &Error{File: file, Column: c.name, Rule: "wrong type"}
		}
		idx[ci] = p
	}
	d := &dec{file: file, cols: cols, vals: make([]parquet.Value, len(cols))}
	byLeaf := make([]parquet.Value, len(pos))
	buf := make([]parquet.Row, chunkRows)
	for _, rg := range pq.RowGroups() {
		if err := scanGroup(rg, buf, byLeaf, idx, d, fn); err != nil {
			return err
		}
	}
	return nil
}

func scanGroup(rg parquet.RowGroup, buf []parquet.Row, byLeaf []parquet.Value, idx []int, d *dec,
	fn func(d *dec) error,
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
			if err := fn(d); err != nil {
				return err
			}
			if d.err != nil {
				return d.err
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
