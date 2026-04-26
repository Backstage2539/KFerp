package production

import "strings"

const (
	RoastLevelLight      = "浅烘"
	RoastLevelMedium     = "中烘"
	RoastLevelMediumDark = "中深烘"
	RoastLevelDark       = "深烘"
)

type RoastLevelOption struct {
	Label     string
	YieldRate float64
}

func roastLevelOptions() []RoastLevelOption {
	return []RoastLevelOption{
		{Label: RoastLevelLight, YieldRate: 0.82},
		{Label: RoastLevelMedium, YieldRate: 0.815},
		{Label: RoastLevelMediumDark, YieldRate: 0.81},
		{Label: RoastLevelDark, YieldRate: 0.80},
	}
}

func normalizeRoastLevel(v string) string {
	v = strings.TrimSpace(v)
	for _, item := range roastLevelOptions() {
		if v == item.Label {
			return item.Label
		}
	}
	return ""
}

func yieldRateForRoastLevel(v string) float64 {
	v = normalizeRoastLevel(v)
	for _, item := range roastLevelOptions() {
		if v == item.Label {
			return item.YieldRate
		}
	}
	return 0
}

func resolveYieldRate(roastLevel string, fallback float64) float64 {
	if rate := yieldRateForRoastLevel(roastLevel); rate > 0 {
		return rate
	}
	return normalizeYieldRate(fallback)
}
