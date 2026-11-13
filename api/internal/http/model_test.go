package http

import (
	"strings"
	"testing"
)

func TestDayFor(t *testing.T) {
	d := int32(999)
	cases := []struct {
		name    string
		m       activeModel
		in      *int
		want    int32
		wantErr bool
	}{
		{"nil uses card default", activeModel{MaxDay: 7, Card: card{DefaultDay: new(int32(3))}}, nil, 3, false},
		{"nil clamps card default", activeModel{MaxDay: 7, Card: card{DefaultDay: &d}}, nil, 7, false},
		{"nil without card default is max", activeModel{MaxDay: 7}, nil, 7, false},
		{"negative", activeModel{MaxDay: 7}, new(-1), 0, true},
		{"above max", activeModel{MaxDay: 7}, new(8), 0, true},
		{"max ok", activeModel{MaxDay: 7}, new(7), 7, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := c.m.dayFor(c.in)
			if c.wantErr {
				if err == nil || !strings.Contains(err.Error(), "as_of_day") {
					t.Fatalf("err = %v", err)
				}
				return
			}
			if err != nil || got != c.want {
				t.Fatalf("got %d, %v; want %d", got, err, c.want)
			}
		})
	}
}
