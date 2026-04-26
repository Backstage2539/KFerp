package appmain

import salesdomain "orderapp/internal/domain/sales"

// applyRoundToInt: "抹除小数点" => round down to integer (truncate decimal part).
func applyRoundToInt(total float64, enabled bool) (grand float64, rounding float64) {
	return salesdomain.ApplyRoundToInt(total, enabled)
}
