package catalog

import "testing"

func TestYieldRateForRoastLevel(t *testing.T) {
	tests := []struct {
		name       string
		roastLevel string
		want       float64
	}{
		{name: "light", roastLevel: RoastLevelLight, want: 0.82},
		{name: "medium", roastLevel: RoastLevelMedium, want: 0.815},
		{name: "medium dark", roastLevel: RoastLevelMediumDark, want: 0.81},
		{name: "dark", roastLevel: RoastLevelDark, want: 0.80},
		{name: "trimmed", roastLevel: "  中烘  ", want: 0.815},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := yieldRateForRoastLevel(tc.roastLevel); got != tc.want {
				t.Fatalf("yieldRateForRoastLevel(%q) = %.4f, want %.4f", tc.roastLevel, got, tc.want)
			}
		})
	}
}

func TestResolveYieldRateFallsBack(t *testing.T) {
	if got := ResolveYieldRate("", 0.823); got != 0.823 {
		t.Fatalf("ResolveYieldRate fallback = %.4f, want 0.8230", got)
	}
	if got := ResolveYieldRate("未知", 0); got != 0.8 {
		t.Fatalf("ResolveYieldRate default = %.4f, want 0.8000", got)
	}
}
