package catalog

import catalogdomain "orderapp/internal/domain/catalog"

const (
	RoastLevelLight      = catalogdomain.RoastLevelLight
	RoastLevelMedium     = catalogdomain.RoastLevelMedium
	RoastLevelMediumDark = catalogdomain.RoastLevelMediumDark
	RoastLevelDark       = catalogdomain.RoastLevelDark
)

type RoastLevelOption = catalogdomain.RoastLevelOption

func roastLevelOptions() []RoastLevelOption {
	return catalogdomain.RoastLevelOptions()
}

func NormalizeRoastLevel(v string) string {
	return catalogdomain.NormalizeRoastLevel(v)
}

func yieldRateForRoastLevel(v string) float64 {
	return catalogdomain.YieldRateForRoastLevel(v)
}

func ResolveYieldRate(roastLevel string, fallback float64) float64 {
	return catalogdomain.ResolveYieldRate(roastLevel, fallback)
}
