package artifacts

import (
	"math"
	"strings"
	"testing"
)

type pRow struct {
	PlayerID     string `parquet:"player_id"`
	OperatorCode string `parquet:"operator_code"`
	LastDay      int32  `parquet:"last_day"`
	SessionCount int32  `parquet:"session_count"`
	DepositCount int32  `parquet:"deposit_count"`
}

type sRow struct {
	PlayerID string   `parquet:"player_id"`
	Day      int32    `parquet:"day"`
	Score    *float64 `parquet:"score,optional"`
	Reasons  string   `parquet:"reasons"`
}

type aRow struct {
	PlayerID       string   `parquet:"player_id"`
	Day            int32    `parquet:"day"`
	DepositsN      int32    `parquet:"deposits_n"`
	DepositsEUR    float64  `parquet:"deposits_eur"`
	SessionsN      int32    `parquet:"sessions_n"`
	PlayMinutes    float64  `parquet:"play_minutes"`
	NetResultEUR   float64  `parquet:"net_result_eur"`
	ChaseSessionsN int32    `parquet:"chase_sessions_n"`
	NightSessionsN int32    `parquet:"night_sessions_n"`
	EndBalanceEUR  *float64 `parquet:"end_balance_eur,optional"`
}

type aRow32 struct {
	PlayerID       string   `parquet:"player_id"`
	Day            int32    `parquet:"day"`
	DepositsN      int32    `parquet:"deposits_n"`
	DepositsEUR    float64  `parquet:"deposits_eur"`
	SessionsN      int32    `parquet:"sessions_n"`
	PlayMinutes    float32  `parquet:"play_minutes"`
	NetResultEUR   float64  `parquet:"net_result_eur"`
	ChaseSessionsN int32    `parquet:"chase_sessions_n"`
	NightSessionsN int32    `parquet:"night_sessions_n"`
	EndBalanceEUR  *float64 `parquet:"end_balance_eur,optional"`
}

type ssRow struct {
	PlayerID     string  `parquet:"player_id"`
	SessionNo    int32   `parquet:"session_no"`
	StartDay     int32   `parquet:"start_day"`
	DurationS    int32   `parquet:"duration_s"`
	DepositsN    int32   `parquet:"deposits_n"`
	DepositsEUR  float64 `parquet:"deposits_eur"`
	NetResultEUR float64 `parquet:"net_result_eur"`
	IsNight      bool    `parquet:"is_night"`
	IsChase      bool    `parquet:"is_chase"`
}

type fRow struct {
	PlayerID string `parquet:"player_id"`
	Day      int32  `parquet:"day"`
	RuleID   string `parquet:"rule_id"`
	RuleText string `parquet:"rule_text"`
}

type smRow struct {
	PlayerID string `parquet:"player_id"`
	Source   string `parquet:"source"`
	Position int32  `parquet:"position"`
}

func collect[R, T any](t *testing.T, it func(func(int, R) error) error, conv func(R) T) []T {
	t.Helper()
	var out []T
	if err := it(func(_ int, r R) error { out = append(out, conv(r)); return nil }); err != nil {
		t.Fatal(err)
	}
	return out
}

func playerRows(t *testing.T, fo *Folder) []pRow {
	return collect(t, fo.Players, func(p Player) pRow { return pRow(p) })
}

func scoreRows(t *testing.T, fo *Folder) []sRow {
	return collect(t, fo.Scores, func(s Score) sRow { return sRow(s) })
}

func activityRows(t *testing.T, fo *Folder) []aRow {
	return collect(t, fo.Activity, func(a Activity) aRow { return aRow(a) })
}

func sessionRows(t *testing.T, fo *Folder) []ssRow {
	return collect(t, fo.Sessions, func(s Session) ssRow { return ssRow(s) })
}

func check(t *testing.T, edit func(dir string, fo *Folder), want Error) {
	t.Helper()
	dir := dummy(t)
	fo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	edit(dir, fo)
	if fo, err = Open(dir); err != nil {
		t.Fatal(err)
	}
	e := asErr(t, fo.Validate())
	if want.Rule == "" {
		want.Rule = e.Rule
	}
	if *e != want {
		t.Fatalf("got %+v, want %+v", *e, want)
	}
	if strings.Contains(e.Error(), leak) {
		t.Fatalf("error leaks a value: %s", e)
	}
}

func passes(t *testing.T, edit func(dir string, fo *Folder)) {
	t.Helper()
	dir := dummy(t)
	fo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	edit(dir, fo)
	if fo, err = Open(dir); err != nil {
		t.Fatal(err)
	}
	if err := fo.Validate(); err != nil {
		t.Fatalf("want pass, got %v", err)
	}
}

func firstScored(rows []sRow) int {
	for k, r := range rows {
		if r.Score != nil {
			return k
		}
	}
	return -1
}

func TestValidate_Clean(t *testing.T) {
	passes(t, func(string, *Folder) {})
}

func TestValidate_EmptyFlagsAndSample(t *testing.T) {
	passes(t, func(dir string, _ *Folder) {
		writeFile(t, dir, FileFlags, []fRow{})
		writeFile(t, dir, FileSample, []smRow{})
	})
}

func TestValidate_DuplicatePlayer(t *testing.T) {
	check(t, func(dir string, fo *Folder) {
		rows := playerRows(t, fo)
		rows[0].PlayerID = leak
		rows[1] = rows[0]
		writeFile(t, dir, FilePlayers, rows)
	}, Error{File: FilePlayers, Row: 2, Column: "player_id"})
}

func TestValidate_NoPlayers(t *testing.T) {
	check(t, func(dir string, _ *Folder) {
		writeFile(t, dir, FilePlayers, []pRow{})
	}, Error{File: FilePlayers, Rule: "must have at least one row"})
}

func TestValidate_EmptyOperator(t *testing.T) {
	check(t, func(dir string, fo *Folder) {
		rows := playerRows(t, fo)
		rows[2].OperatorCode = ""
		writeFile(t, dir, FilePlayers, rows)
	}, Error{File: FilePlayers, Row: 3, Column: "operator_code"})
}

func TestValidate_NegativeCount(t *testing.T) {
	check(t, func(dir string, fo *Folder) {
		rows := playerRows(t, fo)
		rows[2].SessionCount = -1
		writeFile(t, dir, FilePlayers, rows)
	}, Error{File: FilePlayers, Row: 3, Column: "session_count"})
}

func TestValidate_LastDayTooLarge(t *testing.T) {
	check(t, func(dir string, fo *Folder) {
		rows := playerRows(t, fo)
		rows[4].PlayerID = leak
		rows[4].LastDay = math.MaxInt32
		writeFile(t, dir, FilePlayers, rows)
	}, Error{File: FilePlayers, Row: 5, Column: "last_day"})
}

func TestValidate_ScoreGap(t *testing.T) {
	check(t, func(dir string, fo *Folder) {
		id := playerRows(t, fo)[16].PlayerID
		var rows []sRow
		var mine []int
		for k, r := range scoreRows(t, fo) {
			rows = append(rows, r)
			if r.PlayerID == id {
				mine = append(mine, k)
			}
		}
		drop := mine[len(mine)/2]
		writeFile(t, dir, FileScores, append(rows[:drop:drop], rows[drop+1:]...))
	}, Error{File: FileScores, Rule: "player at players.parquet row 17: missing days"})
}

func TestValidate_PlayerWithoutScores(t *testing.T) {
	check(t, func(dir string, fo *Folder) {
		id := playerRows(t, fo)[0].PlayerID
		var rows []sRow
		for _, r := range scoreRows(t, fo) {
			if r.PlayerID != id {
				rows = append(rows, r)
			}
		}
		writeFile(t, dir, FileScores, rows)
	}, Error{File: FileScores, Rule: "player at players.parquet row 1: missing days"})
}

func TestValidate_ScoreRowErrors(t *testing.T) {
	one := `[{"code":"a","text":"b","weight":1}]`
	for name, tc := range map[string]struct {
		edit func(*sRow)
		col  string
	}{
		"out of range":       {func(r *sRow) { r.Score = ptr(1.5) }, "score"},
		"nan":                {func(r *sRow) { r.Score = ptr(math.NaN()) }, "score"},
		"not json":           {func(r *sRow) { r.Reasons = "not json" }, "reasons"},
		"json null":          {func(r *sRow) { r.Reasons = "null" }, "reasons"},
		"four reasons":       {func(r *sRow) { r.Reasons = "[" + strings.Repeat(one[1:len(one)-1]+",", 3) + one[1:len(one)-1] + "]" }, "reasons"},
		"missing weight":     {func(r *sRow) { r.Reasons = `[{"code":"a","text":"b"}]` }, "reasons"},
		"weight is a string": {func(r *sRow) { r.Reasons = `[{"code":"a","text":"b","weight":"1"}]` }, "reasons"},
	} {
		t.Run(name, func(t *testing.T) {
			fo, err := Open(dummy(t))
			if err != nil {
				t.Fatal(err)
			}
			row := firstScored(scoreRows(t, fo)) + 1
			check(t, func(dir string, fo *Folder) {
				rows := scoreRows(t, fo)
				tc.edit(&rows[row-1])
				writeFile(t, dir, FileScores, rows)
			}, Error{File: FileScores, Row: row, Column: tc.col})
		})
	}
}

func TestValidate_DuplicateScoreDay(t *testing.T) {
	check(t, func(dir string, fo *Folder) {
		rows := scoreRows(t, fo)
		rows[1].Day = rows[0].Day
		writeFile(t, dir, FileScores, rows)
	}, Error{File: FileScores, Row: 2, Column: "day"})
}

func TestValidate_NullScoreWithReasons(t *testing.T) {
	check(t, func(dir string, fo *Folder) {
		rows := scoreRows(t, fo)
		rows[0].Score = nil
		rows[0].Reasons = `[{"code":"a","text":"b","weight":1}]`
		writeFile(t, dir, FileScores, rows)
	}, Error{File: FileScores, Row: 1, Column: "reasons"})
}

func TestValidate_UnknownPlayer(t *testing.T) {
	check(t, func(dir string, fo *Folder) {
		rows := sessionRows(t, fo)
		rows[0].PlayerID = leak
		writeFile(t, dir, FileSessions, rows)
	}, Error{File: FileSessions, Row: 1, Column: "player_id"})
}

func TestValidate_DayAfterLastDay(t *testing.T) {
	check(t, func(dir string, fo *Folder) {
		rows := activityRows(t, fo)
		rows[0].Day = math.MaxInt32
		writeFile(t, dir, FileActivity, rows)
	}, Error{File: FileActivity, Row: 1, Column: "day"})
}

func TestValidate_DuplicateActivityDay(t *testing.T) {
	check(t, func(dir string, fo *Folder) {
		rows := activityRows(t, fo)
		rows[1].PlayerID, rows[1].Day = rows[0].PlayerID, rows[0].Day
		writeFile(t, dir, FileActivity, rows)
	}, Error{File: FileActivity, Row: 2, Column: "day"})
}

func editActivity(t *testing.T, edit func(*aRow)) func(dir string, fo *Folder) {
	return func(dir string, fo *Folder) {
		rows := activityRows(t, fo)
		edit(&rows[0])
		writeFile(t, dir, FileActivity, rows)
	}
}

func TestValidate_NegativeDeposits(t *testing.T) {
	check(t, editActivity(t, func(a *aRow) { a.DepositsEUR = -1 }),
		Error{File: FileActivity, Row: 1, Column: "deposits_eur"})
}

func TestValidate_NegativeNetResultAllowed(t *testing.T) {
	passes(t, editActivity(t, func(a *aRow) { a.NetResultEUR = -50; a.EndBalanceEUR = ptr(-3.0) }))
}

func TestValidate_InfMinutes(t *testing.T) {
	check(t, editActivity(t, func(a *aRow) { a.PlayMinutes = math.Inf(1) }),
		Error{File: FileActivity, Row: 1, Column: "play_minutes"})
}

func TestValidate_NaNFloat32Minutes(t *testing.T) {
	check(t, func(dir string, fo *Folder) {
		rows := collect(t, fo.Activity, func(a Activity) aRow32 {
			return aRow32{a.PlayerID, a.Day, a.DepositsN, a.DepositsEUR, a.SessionsN, float32(a.PlayMinutes),
				a.NetResultEUR, a.ChaseSessionsN, a.NightSessionsN, a.EndBalanceEUR}
		})
		rows[0].PlayMinutes = float32(math.NaN())
		writeFile(t, dir, FileActivity, rows)
	}, Error{File: FileActivity, Row: 1, Column: "play_minutes"})
}

func TestValidate_DuplicateSession(t *testing.T) {
	check(t, func(dir string, fo *Folder) {
		rows := sessionRows(t, fo)
		rows[1].PlayerID, rows[1].SessionNo = rows[0].PlayerID, rows[0].SessionNo
		writeFile(t, dir, FileSessions, rows)
	}, Error{File: FileSessions, Row: 2, Column: "session_no"})
}

func TestValidate_NegativeDuration(t *testing.T) {
	check(t, func(dir string, fo *Folder) {
		rows := sessionRows(t, fo)
		rows[0].DurationS = -1
		writeFile(t, dir, FileSessions, rows)
	}, Error{File: FileSessions, Row: 1, Column: "duration_s"})
}

func TestValidate_DuplicateFlag(t *testing.T) {
	check(t, func(dir string, fo *Folder) {
		id := playerRows(t, fo)[0].PlayerID
		f := fRow{id, 0, "r1", "text"}
		writeFile(t, dir, FileFlags, []fRow{f, f})
	}, Error{File: FileFlags, Row: 2, Column: "rule_id"})
}

func TestValidate_EmptyRuleText(t *testing.T) {
	check(t, func(dir string, fo *Folder) {
		writeFile(t, dir, FileFlags, []fRow{{playerRows(t, fo)[0].PlayerID, 0, "r1", ""}})
	}, Error{File: FileFlags, Row: 1, Column: "rule_text"})
}

func sampleOf(t *testing.T, fo *Folder, src [3]string, pos [3]int32, same bool) []smRow {
	ps := playerRows(t, fo)
	rows := make([]smRow, 3)
	for k := range rows {
		id := ps[k].PlayerID
		if same {
			id = ps[0].PlayerID
		}
		rows[k] = smRow{id, src[k], pos[k]}
	}
	return rows
}

func TestValidate_BadSource(t *testing.T) {
	check(t, func(dir string, fo *Folder) {
		writeFile(t, dir, FileSample, sampleOf(t, fo, [3]string{"model", "oracle", "both"}, [3]int32{1, 2, 3}, false))
	}, Error{File: FileSample, Row: 2, Column: "source"})
}

func TestValidate_DuplicateSamplePlayer(t *testing.T) {
	check(t, func(dir string, fo *Folder) {
		writeFile(t, dir, FileSample, sampleOf(t, fo, [3]string{"model", "rules", "both"}, [3]int32{1, 2, 3}, true))
	}, Error{File: FileSample, Row: 2, Column: "player_id"})
}

func TestValidate_PositionsNotContiguous(t *testing.T) {
	check(t, func(dir string, fo *Folder) {
		writeFile(t, dir, FileSample, sampleOf(t, fo, [3]string{"model", "rules", "both"}, [3]int32{1, 2, 4}, false))
	}, Error{File: FileSample, Column: "position"})
}

func TestValidate_NegativeSessionDeposits(t *testing.T) {
	check(t, func(dir string, fo *Folder) {
		rows := sessionRows(t, fo)
		rows[0].DepositsEUR = -1
		writeFile(t, dir, FileSessions, rows)
	}, Error{File: FileSessions, Row: 1, Column: "deposits_eur"})
}

func TestScan_InvalidUTF8(t *testing.T) {
	check(t, func(dir string, fo *Folder) {
		rows := playerRows(t, fo)
		rows[0].OperatorCode = "\xff" + leak
		writeFile(t, dir, FilePlayers, rows)
	}, Error{File: FilePlayers, Row: 1, Column: "operator_code", Rule: "must be valid UTF-8 without NUL bytes"})
}

func TestScan_NULByte(t *testing.T) {
	check(t, func(dir string, fo *Folder) {
		writeFile(t, dir, FileFlags, []fRow{{playerRows(t, fo)[0].PlayerID, 0, "r1", "a\x00" + leak}})
	}, Error{File: FileFlags, Row: 1, Column: "rule_text", Rule: "must be valid UTF-8 without NUL bytes"})
}

func reasonsFail(t *testing.T, raw, rule string) {
	t.Helper()
	fo, err := Open(dummy(t))
	if err != nil {
		t.Fatal(err)
	}
	row := firstScored(scoreRows(t, fo)) + 1
	check(t, func(dir string, fo *Folder) {
		rows := scoreRows(t, fo)
		rows[row-1].Reasons = raw
		writeFile(t, dir, FileScores, rows)
	}, Error{File: FileScores, Row: row, Column: "reasons", Rule: rule})
}

func TestValidate_ReasonsNULEscape(t *testing.T) {
	reasonsFail(t, `[{"code":"a","text":"x\u0000y","weight":1}]`, "must not contain a NUL character")
}

func TestValidate_ReasonCapitalisedKeys(t *testing.T) {
	reasonsFail(t, `[{"Code":"a","Text":"b","Weight":1}]`, reasonShape)
}

func TestValidate_ReasonWeightString(t *testing.T) {
	reasonsFail(t, `[{"code":"a","text":"b","weight":"1"}]`, reasonShape)
}

func TestValidate_ReasonNotObject(t *testing.T) {
	reasonsFail(t, `["a"]`, "each reason must be an object")
}
