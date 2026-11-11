package production

import "testing"

func TestSemiFinishedStockBudget(t *testing.T) {
	cases := []struct {
		name, policy              string
		semi                      bool
		available, selected, want float64
		fails                     bool
	}{
		{"new plan does not automatically use stock", "manual", true, 15, 0, 0, false},
		{"chosen quantity reduces manufacturing", "manual", true, 15, 10, 10, false},
		{"legacy draft retains automatic use", "", true, 15, 0, 15, false},
		{"other materials retain automatic use", "manual", false, 15, 0, 15, false},
		{"selection exceeding available rejected", "manual", true, 10, 15, 0, true},
		{"negative selection rejected", "manual", true, 15, -1, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := semiFinishedStockBudget(tc.policy, tc.semi, tc.available, tc.selected)
			if (err != nil) != tc.fails || got != tc.want {
				t.Fatalf("got %v, %v; want %v, error %v", got, err, tc.want, tc.fails)
			}
		})
	}
}
