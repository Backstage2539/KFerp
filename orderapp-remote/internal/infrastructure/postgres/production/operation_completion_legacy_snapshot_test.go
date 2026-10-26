package production

import "testing"

func TestIsEmptyLegacyExecutionSnapshot(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  bool
	}{
		{name: "missing", value: nil, want: true},
		{name: "blank", value: "  ", want: true},
		{name: "null placeholder", value: "null", want: true},
		{name: "empty placeholder", value: "{}", want: true},
		{name: "zero config placeholder", value: `{"version":0,"receipt_enabled":false}`, want: true},
		{name: "active version", value: `{"version":1}`, want: false},
		{name: "partial warehouse config", value: `{"receipt_warehouse":"finished_goods"}`, want: false},
		{name: "unknown field", value: `{"legacy_flag":true}`, want: false},
		{name: "malformed config", value: "{", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isEmptyLegacyExecutionSnapshot(tt.value); got != tt.want {
				t.Fatalf("isEmptyLegacyExecutionSnapshot(%v)=%v, want %v", tt.value, got, tt.want)
			}
		})
	}
}
