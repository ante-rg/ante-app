package baseline

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"
)

func TestRates_CoverEveryMonth(t *testing.T) {
	r, err := loadRates()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for c := range r {
		got = append(got, c)
	}
	sort.Strings(got)
	want := "AUD BRL CAD CHF DKK IDR MXN NOK PLN TRY USD"
	if strings.Join(got, " ") != want {
		t.Fatalf("currencies = %v, want %s", got, want)
	}
	for c, months := range r {
		if len(months) != 60 {
			t.Errorf("%s has %d months, want 60", c, len(months))
		}
		for y := 2021; y <= 2026; y++ {
			for m := 1; m <= 12; m++ {
				if y == 2021 && m < 11 || y == 2026 && m > 10 {
					continue
				}
				key := fmt.Sprintf("%04d-%02d", y, m)
				v, ok := months[key]
				if !ok || v <= 0 || math.IsInf(v, 0) || math.IsNaN(v) {
					t.Errorf("%s %s = %v, ok=%v", c, key, v, ok)
				}
			}
		}
	}
}

func TestRates_Rate(t *testing.T) {
	r, err := loadRates()
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := r.rate("EUR", "1999-01"); !ok || v != 1 {
		t.Errorf("EUR = %v, %v", v, ok)
	}
	if v, ok := r.rate("PLN", "2021-11"); !ok || v != 4.646209 {
		t.Errorf("PLN = %v, %v", v, ok)
	}
	if _, ok := r.rate("PLN", "2026-11"); ok {
		t.Error("PLN 2026-11 should be missing")
	}
	for c := range skippedCurrencies {
		if _, ok := r.rate(c, "2024-01"); ok {
			t.Errorf("%s should be missing", c)
		}
	}
}
