package production

import "testing"

func TestDeferredPackagingUsagePreservesRemainder(t *testing.T) {
	tests := []struct {
		total               float64
		before, qty, target float64
		available, want     float64
		fail                bool
	}{
		{22700, 0, 100, 100, 23700, 22700, false},
		{1001, 0, 1, 3, 1001, 334, false},
		{1001, 1, 1, 3, 667, 333, false},
		{1001, 2, 1, 3, 334, 334, false},
		{22700, 0, 100, 100, 22699, 0, true},
	}
	for _, tt := range tests {
		got, err := DeferredPackagingUsage(tt.total, tt.before, tt.qty, tt.target, tt.available)
		if (err != nil) != tt.fail || got != tt.want {
			t.Fatalf("%+v: got %v, error %v", tt, got, err)
		}
	}
}
