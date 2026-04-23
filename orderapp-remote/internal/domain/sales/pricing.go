package sales

import "math"

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

// RetailPackagePrice converts a 227g retail price to the selected package spec.
// Retail quotes are whole Yuan per package, rounded up after gram conversion.
func RetailPackagePrice(retailPrice227G float64, specG int64) float64 {
	if retailPrice227G <= 0 || specG <= 0 {
		return 0
	}
	return math.Ceil(retailPrice227G * float64(specG) / 227.0)
}

func RetailLinePrice(retailPrice227G float64, specG, units int64) (packagePrice float64, lineTotal float64) {
	packagePrice = RetailPackagePrice(retailPrice227G, specG)
	if units <= 0 {
		return packagePrice, 0
	}
	return packagePrice, packagePrice * float64(units)
}
