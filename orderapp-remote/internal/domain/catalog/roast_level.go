package catalog

import (
	productiondomain "orderapp/internal/domain/production"
	"strings"
)

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

func RoastLevelOptions() []RoastLevelOption {
	return []RoastLevelOption{
		{Label: RoastLevelLight, YieldRate: 0.82},
		{Label: RoastLevelMedium, YieldRate: 0.815},
		{Label: RoastLevelMediumDark, YieldRate: 0.81},
		{Label: RoastLevelDark, YieldRate: 0.80},
	}
}

func NormalizeRoastLevel(v string) string {
	v = strings.TrimSpace(v)
	for _, item := range RoastLevelOptions() {
		if v == item.Label {
			return item.Label
		}
	}
	return ""
}

func YieldRateForRoastLevel(v string) float64 {
	v = NormalizeRoastLevel(v)
	for _, item := range RoastLevelOptions() {
		if v == item.Label {
			return item.YieldRate
		}
	}
	return 0
}

func ResolveYieldRate(roastLevel string, fallback float64) float64 {
	if rate := YieldRateForRoastLevel(roastLevel); rate > 0 {
		return rate
	}
	return productiondomain.NormalizeYieldRate(fallback)
}
