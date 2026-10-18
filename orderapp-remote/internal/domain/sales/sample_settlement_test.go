package sales

import "testing"

func TestFreeSampleSettlementEligibility(t *testing.T) {
	cases := []struct {
		name                          string
		sample                        bool
		goods, shipping, other, total float64
		want                          bool
	}{
		{"free", true, 0, 0, 0, 0, true}, {"ordinary", false, 0, 0, 0, 0, false},
		{"freight", true, 0, 8, 0, 8, false}, {"paid", true, 10, 0, 0, 10, false},
		{"other fee", true, 0, 0, 1, 1, false}, {"rounding", true, 0.01, 0, 0, 0, false},
		{"discounted", true, 10, 0, 0, 0, false}, {"negative", true, -1, 0, 0, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsFreeSampleSettlement(c.sample, c.goods, c.shipping, c.other, c.total); got != c.want {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
}
