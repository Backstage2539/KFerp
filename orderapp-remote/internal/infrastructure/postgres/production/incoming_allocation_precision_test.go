package production

import (
	"math"
	"testing"
)

func TestIncomingAllocationPreservesSubgramPrecision(t *testing.T) {
	tests := []struct {
		name          string
		totalGrams    float64
		unit          string
		weights, want []float64
	}{
		{"actual single batch", 59961.751, "kg", []float64{1}, []float64{59.961751}},
		{"actual split", 59961.751, "kg", []float64{60, 40}, []float64{35.9770506, 23.9847004}},
		{"integer gram legacy split", 93001, "kg", []float64{60, 40}, []float64{55.801, 37.2}},
		{"fractional grams", 59961.751, "g", []float64{1, 1}, []float64{29980.8755, 29980.8755}},
		{"one hundredth microgram", 1.00000001, "g", []float64{1}, []float64{1.00000001}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := incomingQuantitiesFromGrams(tt.totalGrams, tt.unit, tt.weights)
			if err != nil {
				t.Fatal(err)
			}
			for i := range tt.want {
				if math.Abs(got[i]-tt.want[i]) > 1e-10 {
					t.Fatalf("got %v want %v", got, tt.want)
				}
			}
		})
	}
}

func TestIncomingAllocationRejectsUnrepresentablePrecision(t *testing.T) {
	if _, err := incomingQuantitiesFromGrams(1.000000001, "g", []float64{1}); err == nil {
		t.Fatal("sub-precision weight must not be rounded or invented")
	}
	if _, err := incomingQuantitiesFromGrams(1, "bag", []float64{1}); err == nil {
		t.Fatal("incompatible unit must be rejected")
	}
}
