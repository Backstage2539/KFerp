package production

import (
	app "orderapp/internal/application/production"
	"testing"
)

func TestWIPExpectedMatchesCumulativePackagingRounding(t *testing.T) {
	cases := []struct {
		name                           string
		material                       app.OperationMaterialUse
		isBean                         bool
		before, remaining, total, want float64
	}{
		{"weight tail one gram", app.OperationMaterialUse{Unit: "kg", TotalQty: 0.003}, true, 1, 1, 2, 0.001},
		{"weight first two grams", app.OperationMaterialUse{Unit: "kg", TotalQty: 0.003}, true, 0, 1, 2, 0.002},
		{"count tail follows cumulative ceiling", app.OperationMaterialUse{Unit: "pic", TotalQty: 3}, false, 1, 1, 2, 1},
		{"count first follows cumulative ceiling", app.OperationMaterialUse{Unit: "pic", TotalQty: 3}, false, 0, 1, 2, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := wipExpectedSlice(tc.material, tc.isBean, tc.before, tc.remaining, tc.total)
			if err != nil || got != tc.want {
				t.Fatalf("got %v, %v; want %v", got, err, tc.want)
			}
		})
	}
}
