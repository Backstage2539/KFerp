package postgres

import "testing"

func TestSerialSequenceRepairValue(t *testing.T) {
	tests := []struct {
		name      string
		maxID     int64
		lastValue int64
		wantValue int64
		wantFix   bool
	}{
		{name: "sequence behind rows", maxID: 14, lastValue: 7, wantValue: 15, wantFix: true},
		{name: "sequence equal to maximum row", maxID: 14, lastValue: 14, wantValue: 15, wantFix: true},
		{name: "sequence already ahead", maxID: 14, lastValue: 15, wantValue: 0, wantFix: false},
		{name: "empty table", maxID: 0, lastValue: 1, wantValue: 0, wantFix: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotFix := serialSequenceRepairValue(tt.maxID, tt.lastValue)
			if gotValue != tt.wantValue || gotFix != tt.wantFix {
				t.Fatalf("serialSequenceRepairValue(%d, %d) = (%d, %v), want (%d, %v)", tt.maxID, tt.lastValue, gotValue, gotFix, tt.wantValue, tt.wantFix)
			}
		})
	}
}
