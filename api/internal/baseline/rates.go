package baseline

import (
	_ "embed"
	"fmt"
	"math"
	"strconv"
	"strings"
)

//go:embed ecb_rates.csv
var ecbRatesCSV string

type rates map[string]map[string]float64

var skippedCurrencies = map[string]bool{"RUB": true, "KZT": true, "UAH": true}

func loadRates() (rates, error) {
	r := rates{}
	for i, line := range strings.Split(ecbRatesCSV, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || line == "currency,month,rate" {
			continue
		}
		f := strings.Split(line, ",")
		if len(f) != 3 {
			return nil, fmt.Errorf("ecb_rates.csv line %d: want 3 fields, got %d", i+1, len(f))
		}
		v, err := strconv.ParseFloat(f[2], 64)
		if err != nil || v <= 0 || math.IsInf(v, 0) || math.IsNaN(v) {
			return nil, fmt.Errorf("ecb_rates.csv line %d: bad rate %q", i+1, f[2])
		}
		if r[f[0]] == nil {
			r[f[0]] = map[string]float64{}
		}
		if _, dup := r[f[0]][f[1]]; dup {
			return nil, fmt.Errorf("ecb_rates.csv line %d: duplicate %s %s", i+1, f[0], f[1])
		}
		r[f[0]][f[1]] = v
	}
	return r, nil
}

func (r rates) rate(currency, month string) (float64, bool) {
	if currency == "EUR" {
		return 1, true
	}
	v, ok := r[currency][month]
	return v, ok
}
