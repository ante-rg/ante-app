package artifacts

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/parquet-go/parquet-go"

	"github.com/ante-rg/ante-app/api/internal/fakeartifacts"
)

const leak = "LEAK-MARKER"

func dummy(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "dummy-v0")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fakeartifacts.Write(dir, "dummy-v0", fakeartifacts.Generate(1)); err != nil {
		t.Fatal(err)
	}
	return dir
}

func writeFile[T any](t *testing.T, dir, name string, rows []T, opts ...parquet.WriterOption) {
	t.Helper()
	fh, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = fh.Close() }()
	w := parquet.NewGenericWriter[T](fh, opts...)
	if _, err := w.Write(rows); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

type playerI64 struct {
	PlayerID     *string `parquet:"player_id,optional"`
	OperatorCode *string `parquet:"operator_code,optional"`
	LastDay      *int64  `parquet:"last_day,optional"`
	SessionCount *int64  `parquet:"session_count,optional"`
	DepositCount *int64  `parquet:"deposit_count,optional"`
}

type sessionOptDuration struct {
	PlayerID     string  `parquet:"player_id"`
	SessionNo    int32   `parquet:"session_no"`
	StartDay     int32   `parquet:"start_day"`
	DurationS    *int32  `parquet:"duration_s,optional"`
	DepositsN    int32   `parquet:"deposits_n"`
	DepositsEUR  float64 `parquet:"deposits_eur"`
	NetResultEUR float64 `parquet:"net_result_eur"`
	IsNight      bool    `parquet:"is_night"`
	IsChase      bool    `parquet:"is_chase"`
}

type sessionNoChase struct {
	PlayerID     string  `parquet:"player_id"`
	SessionNo    int32   `parquet:"session_no"`
	StartDay     int32   `parquet:"start_day"`
	DurationS    int32   `parquet:"duration_s"`
	DepositsN    int32   `parquet:"deposits_n"`
	DepositsEUR  float64 `parquet:"deposits_eur"`
	NetResultEUR float64 `parquet:"net_result_eur"`
	IsNight      bool    `parquet:"is_night"`
}

type sessionExtra struct {
	sessionNoChase
	IsChase bool   `parquet:"is_chase"`
	Foo     string `parquet:"foo"`
}

type scoreStringDay struct {
	PlayerID string   `parquet:"player_id"`
	Day      string   `parquet:"day"`
	Score    *float64 `parquet:"score,optional"`
	Reasons  string   `parquet:"reasons"`
}

func ptr[T any](v T) *T { return &v }
