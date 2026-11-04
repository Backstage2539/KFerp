package production

import (
	"math"
	"testing"
)

func TestFinishedReceiptWeightUnitsAreNotCounts(t *testing.T) {
	for _, tc := range []struct {
		unit  string
		grams float64
	}{
		{"g", 1}, {"kg", 1000}, {"lb", 453.59237}, {"oz", 28.349523125},
	} {
		if isCountReceipt(tc.unit) || math.Abs(receiptWeightQuantity(tc.grams, tc.unit)-1) > 1e-9 {
			t.Errorf("%s is not converted from grams correctly", tc.unit)
		}
	}
}
