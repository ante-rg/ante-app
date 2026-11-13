package artifacts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

type pstate struct {
	row       int
	lastDay   int32
	scoreSeen []bool
	scoreN    int32
	actSeen   []bool
}

type validator struct {
	fo      *Folder
	players map[string]*pstate
	order   []*pstate
}

const maxLastDay = 36500

func bad(file string, row int, column, rule string) error {
	return &Error{File: file, Row: row, Column: column, Rule: rule}
}

func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

func (fo *Folder) Validate() error {
	v := &validator{fo: fo, players: map[string]*pstate{}}
	for _, step := range []func() error{
		v.players1, v.scores, v.activity, v.sessions, v.flags, v.sample,
	} {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}

func (v *validator) players1() error {
	err := v.fo.Players(func(row int, p Player) error {
		switch {
		case p.PlayerID == "":
			return bad(FilePlayers, row, "player_id", "must not be empty")
		case v.players[p.PlayerID] != nil:
			return bad(FilePlayers, row, "player_id", "must be unique")
		case p.OperatorCode == "":
			return bad(FilePlayers, row, "operator_code", "must not be empty")
		case p.LastDay < 0:
			return bad(FilePlayers, row, "last_day", "must be 0 or more")
		case p.LastDay > maxLastDay:
			return bad(FilePlayers, row, "last_day", fmt.Sprintf("must be at most %d", maxLastDay))
		case p.SessionCount < 0:
			return bad(FilePlayers, row, "session_count", "must be 0 or more")
		case p.DepositCount < 0:
			return bad(FilePlayers, row, "deposit_count", "must be 0 or more")
		}
		ps := &pstate{row: row, lastDay: p.LastDay}
		v.players[p.PlayerID] = ps
		v.order = append(v.order, ps)
		return nil
	})
	if err == nil && len(v.order) == 0 {
		return &Error{File: FilePlayers, Rule: "must have at least one row"}
	}
	return err
}

func (v *validator) player(file string, row int, id string, day int32, dayCol string) (*pstate, error) {
	ps := v.players[id]
	if ps == nil {
		return nil, bad(file, row, "player_id", "must exist in players.parquet")
	}
	if dayCol != "" && (day < 0 || day > ps.lastDay) {
		return nil, bad(file, row, dayCol, "must be between 0 and the player's last_day")
	}
	return ps, nil
}

const reasonShape = "each reason needs a string code, a string text and a number weight"

func jsonKind(raw json.RawMessage) byte {
	if t := bytes.TrimSpace(raw); len(t) > 0 {
		return t[0]
	}
	return 0
}

func reasonsOK(raw string, nullScore bool) string {
	var rs []json.RawMessage
	if json.Unmarshal([]byte(raw), &rs) != nil || rs == nil {
		return "must be a JSON array"
	}
	if len(rs) > 3 {
		return "must have at most 3 reasons"
	}
	if nullScore && len(rs) > 0 {
		return "must be [] when score is null"
	}
	for _, r := range rs {
		var m map[string]json.RawMessage
		if json.Unmarshal(r, &m) != nil || m == nil {
			return "each reason must be an object"
		}
		if jsonKind(m["code"]) != '"' || jsonKind(m["text"]) != '"' || !isNumber(m["weight"]) {
			return reasonShape
		}
	}
	var all []map[string]any
	_ = json.Unmarshal([]byte(raw), &all)
	for _, m := range all {
		for _, k := range []string{"code", "text"} {
			if strings.ContainsRune(m[k].(string), 0) {
				return "must not contain a NUL character"
			}
		}
	}
	return ""
}

func isNumber(raw json.RawMessage) bool {
	var f float64
	return jsonKind(raw) != '"' && json.Unmarshal(raw, &f) == nil && jsonKind(raw) != 'n'
}

func (v *validator) scores() error {
	err := v.fo.Scores(func(row int, s Score) error {
		ps, err := v.player(FileScores, row, s.PlayerID, s.Day, "day")
		if err != nil {
			return err
		}
		if ps.scoreSeen == nil {
			ps.scoreSeen = make([]bool, int(ps.lastDay)+1)
		}
		if ps.scoreSeen[s.Day] {
			return bad(FileScores, row, "day", "must be unique per player")
		}
		ps.scoreSeen[s.Day] = true
		ps.scoreN++
		if s.Score != nil && (!finite(*s.Score) || *s.Score < 0 || *s.Score > 1) {
			return bad(FileScores, row, "score", "must be between 0 and 1")
		}
		if rule := reasonsOK(s.Reasons, s.Score == nil); rule != "" {
			return bad(FileScores, row, "reasons", rule)
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, ps := range v.order {
		if int(ps.scoreN) != int(ps.lastDay)+1 {
			return bad(FileScores, 0, "", fmt.Sprintf("player at players.parquet row %d: missing days", ps.row))
		}
	}
	return nil
}

func (v *validator) activity() error {
	return v.fo.Activity(func(row int, a Activity) error {
		ps, err := v.player(FileActivity, row, a.PlayerID, a.Day, "day")
		if err != nil {
			return err
		}
		if ps.actSeen == nil {
			ps.actSeen = make([]bool, int(ps.lastDay)+1)
		}
		if ps.actSeen[a.Day] {
			return bad(FileActivity, row, "day", "must be unique per player")
		}
		ps.actSeen[a.Day] = true
		for _, c := range []struct {
			name string
			ok   bool
		}{
			{"deposits_n", a.DepositsN >= 0},
			{"deposits_eur", finite(a.DepositsEUR) && a.DepositsEUR >= 0},
			{"sessions_n", a.SessionsN >= 0},
			{"play_minutes", finite(a.PlayMinutes) && a.PlayMinutes >= 0},
			{"net_result_eur", finite(a.NetResultEUR)},
			{"chase_sessions_n", a.ChaseSessionsN >= 0},
			{"night_sessions_n", a.NightSessionsN >= 0},
			{"end_balance_eur", a.EndBalanceEUR == nil || finite(*a.EndBalanceEUR)},
		} {
			if !c.ok {
				return bad(FileActivity, row, c.name, "must be a finite number, and 0 or more where it is a count or an amount deposited or played")
			}
		}
		return nil
	})
}

func (v *validator) sessions() error {
	seen := map[struct {
		p *pstate
		n int32
	}]bool{}
	return v.fo.Sessions(func(row int, s Session) error {
		ps, err := v.player(FileSessions, row, s.PlayerID, s.StartDay, "start_day")
		if err != nil {
			return err
		}
		k := struct {
			p *pstate
			n int32
		}{ps, s.SessionNo}
		switch {
		case seen[k]:
			return bad(FileSessions, row, "session_no", "must be unique per player")
		case s.DurationS < 0:
			return bad(FileSessions, row, "duration_s", "must be 0 or more")
		case s.DepositsN < 0:
			return bad(FileSessions, row, "deposits_n", "must be 0 or more")
		case !finite(s.DepositsEUR) || s.DepositsEUR < 0:
			return bad(FileSessions, row, "deposits_eur", "must be a finite number of 0 or more")
		case !finite(s.NetResultEUR):
			return bad(FileSessions, row, "net_result_eur", "must be a finite number")
		}
		seen[k] = true
		return nil
	})
}

func (v *validator) flags() error {
	seen := map[struct {
		p    *pstate
		day  int32
		rule string
	}]bool{}
	return v.fo.RuleFlags(func(row int, r RuleFlag) error {
		ps, err := v.player(FileFlags, row, r.PlayerID, r.Day, "day")
		if err != nil {
			return err
		}
		k := struct {
			p    *pstate
			day  int32
			rule string
		}{ps, r.Day, r.RuleID}
		switch {
		case r.RuleID == "":
			return bad(FileFlags, row, "rule_id", "must not be empty")
		case r.RuleText == "":
			return bad(FileFlags, row, "rule_text", "must not be empty")
		case seen[k]:
			return bad(FileFlags, row, "rule_id", "must be unique per player and day")
		}
		seen[k] = true
		return nil
	})
}

func (v *validator) sample() error {
	seen := map[*pstate]bool{}
	pos := map[int32]bool{}
	err := v.fo.Sample(func(row int, s Sample) error {
		ps, err := v.player(FileSample, row, s.PlayerID, 0, "")
		if err != nil {
			return err
		}
		switch {
		case seen[ps]:
			return bad(FileSample, row, "player_id", "must be unique")
		case s.Source != "model" && s.Source != "rules" && s.Source != "both":
			return bad(FileSample, row, "source", "must be model, rules or both")
		case s.Position < 1 || pos[s.Position]:
			return bad(FileSample, row, "position", "must be 1 or more and unique")
		}
		seen[ps], pos[s.Position] = true, true
		return nil
	})
	if err != nil {
		return err
	}
	for p := int32(1); p <= int32(len(pos)); p++ {
		if !pos[p] {
			return bad(FileSample, 0, "position", "must be exactly 1..n with no gaps")
		}
	}
	return nil
}
