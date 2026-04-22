package sales

// ApplyRoundToInt applies the order "round down to integer" rule and returns
// the payable grand total plus the rounding adjustment.
func ApplyRoundToInt(total float64, enabled bool) (grand float64, rounding float64) {
	if !enabled {
		return total, 0
	}
	grand = float64(int64(total))
	rounding = grand - total
	return grand, rounding
}
