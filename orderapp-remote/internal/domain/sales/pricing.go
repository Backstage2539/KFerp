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

type RetailSpecPrices struct {
	Price100G float64
	Price200G float64
	Price227G float64
	Price250G float64
}

// RetailPackagePrice converts a 227g retail price to the selected package spec.
// Retail quotes are whole Yuan per package, rounded up after gram conversion.
func RetailPackagePrice(retailPrice227G float64, specG int64) float64 {
	if retailPrice227G <= 0 || specG <= 0 {
		return 0
	}
	return math.Ceil(retailPrice227G * float64(specG) / 227.0)
}

func RetailPackagePriceForSpec(prices RetailSpecPrices, specG int64) float64 {
	switch specG {
	case 100:
		if prices.Price100G > 0 {
			return prices.Price100G
		}
	case 200:
		if prices.Price200G > 0 {
			return prices.Price200G
		}
	case 227:
		if prices.Price227G > 0 {
			return prices.Price227G
		}
	case 250:
		if prices.Price250G > 0 {
			return prices.Price250G
		}
	}
	return RetailPackagePrice(prices.Price227G, specG)
}

func RetailLinePrice(retailPrice227G float64, specG, units int64) (packagePrice float64, lineTotal float64) {
	packagePrice = RetailPackagePrice(retailPrice227G, specG)
	if units <= 0 {
		return packagePrice, 0
	}
	return packagePrice, packagePrice * float64(units)
}

func RetailLinePriceForSpec(prices RetailSpecPrices, specG, units int64) (packagePrice float64, lineTotal float64) {
	packagePrice = RetailPackagePriceForSpec(prices, specG)
	if units <= 0 {
		return packagePrice, 0
	}
	return packagePrice, packagePrice * float64(units)
}
