package orderweight

import (
	"math"
	"strings"
)

type Component struct {
	ID          int64   `json:"id"`
	Unit        string  `json:"unit"`
	ConsumeUnit string  `json:"consume_unit"`
	Quantity    float64 `json:"quantity"`
	RatioPct    float64 `json:"ratio_pct,omitempty"`
}

// MaterialReference measures the recipe's direct net inputs, never production loss.
// Piece components are deliberately not expanded into their own recipes.
func MaterialReference(components []Component, factor, netOutputG float64, outputUnits ...string) *float64 {
	if !validPositive(factor) {
		return nil
	}
	outputUnit := ""
	if len(outputUnits) > 0 {
		outputUnit = strings.ToLower(strings.TrimSpace(outputUnits[0]))
	}
	total, known := 0.0, false
	for _, c := range components {
		unit := strings.ToLower(strings.TrimSpace(c.ConsumeUnit))
		scale := gramsPerUnit(unit)
		var grams float64
		switch {
		case scale > 0:
			if !validPositive(c.Quantity) {
				return nil
			}
			grams = c.Quantity * scale * factor
		case unit == "g_per_bag" || unit == "unit_per_bag" || unit == "unit_per_box":
			scale = gramsPerUnit(c.Unit)
			if unit == "g_per_bag" {
				scale = 1
			}
			if scale == 0 {
				continue
			}
			if unit == "unit_per_box" {
				if outputUnit != "盒" && outputUnit != "box" {
					return nil
				}
			} else if outputUnit != "袋" && outputUnit != "bag" {
				return nil
			}
			if !validPositive(c.Quantity) {
				return nil
			}
			grams = c.Quantity * scale * factor
		case unit == "unit" || unit == "fixed_qty":
			scale = gramsPerUnit(c.Unit)
			if unit == "g_per_bag" {
				scale = 1
			}
			if scale == 0 {
				continue
			}
			if !validPositive(c.Quantity) {
				return nil
			}
			grams = c.Quantity * scale * factor
		case unit == "ratio_pct" && gramsPerUnit(c.Unit) > 0:
			if !validPositive(netOutputG) || !validPositive(c.RatioPct) {
				return nil
			}
			grams = netOutputG * c.RatioPct / 100
		default:
			continue
		}
		if !validPositive(grams) || grams > 1e9 {
			return nil
		}
		total += grams
		known = true
	}
	if !known || !validPositive(total) || total > 1e9 {
		return nil
	}
	total = round(total)
	if total <= 0 {
		return nil
	}
	return &total
}

func validPositive(v float64) bool { return v > 0 && !math.IsNaN(v) && !math.IsInf(v, 0) }
func round(v float64) float64      { return math.Round(v*1e6) / 1e6 }

func gramsPerUnit(unit string) float64 {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "kg", "公斤":
		return 1000
	case "g", "克":
		return 1
	}
	return 0
}
