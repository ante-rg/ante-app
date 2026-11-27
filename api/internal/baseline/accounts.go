package baseline

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

type account struct {
	ID       string
	Operator string
	Currency string
	Path     string
}

type accountRow struct {
	currency, path string
	netNum         int
	hash           [32]byte
}

func listAccounts(in string, r rates, brands int) (accts []account, skipped int, err error) {
	inBase := filepath.Base(in)
	entries, err := os.ReadDir(in)
	if err != nil {
		return nil, 0, &Error{File: inBase, Rule: "cannot read folder"}
	}
	absIn, err := filepath.Abs(in)
	if err != nil {
		return nil, 0, err
	}
	var rows []accountRow
	seen := map[string]bool{}
	nets := map[int]bool{}
	batches := 0
	for _, e := range entries {
		dir := filepath.Join(absIn, e.Name())
		if !e.IsDir() {
			continue
		}
		data, rerr := os.ReadFile(filepath.Join(dir, "account_currency.csv"))
		if errors.Is(rerr, os.ErrNotExist) {
			continue
		}
		file := e.Name() + "/account_currency.csv"
		if rerr != nil {
			return nil, 0, &Error{File: file, Rule: "cannot read file"}
		}
		batches++
		br, berr := readBatch(dir, file, data, r, seen)
		if berr != nil {
			return nil, 0, berr
		}
		for _, x := range br {
			nets[x.netNum] = true
		}
		rows = append(rows, br...)
	}
	if batches == 0 {
		return nil, 0, &Error{File: inBase, Rule: "no batch folder with account_currency.csv"}
	}
	if len(nets) != brands {
		return nil, 0, &Error{File: inBase, Rule: fmt.Sprintf("want %d brands, found %d", brands, len(nets))}
	}
	codes := make([]int, 0, len(nets))
	for n := range nets {
		codes = append(codes, n)
	}
	sort.Ints(codes)
	code := map[int]string{}
	for i, n := range codes {
		code[n] = fmt.Sprintf("P%d", i+1)
	}

	sort.Slice(rows, func(i, j int) bool { return bytes.Compare(rows[i].hash[:], rows[j].hash[:]) < 0 })
	for i, x := range rows {
		id := fmt.Sprintf("A%05d", i+1)
		if skippedCurrencies[x.currency] {
			skipped++
			continue
		}
		if _, serr := os.Stat(x.path); serr != nil {
			return nil, 0, &Error{File: "account " + id, Rule: "file listed in account_currency.csv is missing"}
		}
		accts = append(accts, account{ID: id, Operator: code[x.netNum], Currency: x.currency, Path: x.path})
	}
	return accts, skipped, nil
}

func readBatch(dir, file string, data []byte, r rates, seen map[string]bool) ([]accountRow, error) {
	cr := csv.NewReader(bytes.NewReader(data))
	cr.FieldsPerRecord = -1
	head, err := cr.Read()
	if err != nil {
		return nil, &Error{File: file, Rule: "malformed csv"}
	}
	col := map[string]int{}
	for i, h := range head {
		col[h] = i
	}
	for _, name := range []string{"file", "net_id", "player_id", "currency"} {
		if _, ok := col[name]; !ok {
			return nil, &Error{File: file, Column: name, Rule: "missing column"}
		}
	}
	var out []accountRow
	for row := 1; ; row++ {
		rec, err := cr.Read()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return nil, &Error{File: file, Row: row, Rule: "malformed csv"}
		}
		get := func(name string) string {
			if i := col[name]; i < len(rec) {
				return rec[i]
			}
			return ""
		}
		bad := func(column, rule string) error { return &Error{File: file, Row: row, Column: column, Rule: rule} }
		net, player, cur, f := get("net_id"), get("player_id"), get("currency"), get("file")
		n, nerr := strconv.Atoi(net)
		switch {
		case nerr != nil || n < 0:
			return nil, bad("net_id", "not a non-negative integer")
		case player == "":
			return nil, bad("player_id", "empty")
		case cur == "":
			return nil, bad("currency", "empty")
		case !filepath.IsLocal(f):
			return nil, bad("file", "must be a relative path inside the batch folder")
		}
		key := net + "/" + player
		if seen[key] {
			return nil, bad("player_id", "duplicate account")
		}
		seen[key] = true
		if _, ok := r[cur]; cur != "EUR" && !ok && !skippedCurrencies[cur] {
			if len(cur) == 3 && isUpperAlpha(cur) {
				return nil, bad("currency", "no EUR rate for currency "+cur)
			}
			return nil, bad("currency", "unknown currency")
		}
		out = append(out, accountRow{currency: cur, path: filepath.Join(dir, f), netNum: n, hash: sha256.Sum256([]byte(key))})
	}
}

func isUpperAlpha(s string) bool {
	for _, c := range s {
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	return true
}
