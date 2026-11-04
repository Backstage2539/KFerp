package sales

import "testing"

func TestRetainedSnapshotSpecMatchesVueHydration(t *testing.T) {
	for _, tc := range []struct {
		name   string
		label  string
		source any
		grams  int64
		want   bool
	}{
		{"kilogram label", "1kg", nil, 1000, true},
		{"fractional kilogram label", "0.5kg", nil, 500, true},
		{"pound label", "1lb", nil, 454, true},
		{"plain pound", "lb", nil, 454, true},
		{"frozen kg", "大包装", map[string]any{"effective_sales_spec": map[string]any{"net_content_qty": 1.0, "net_content_unit": "kg"}}, 1000, true},
		{"frozen half kg", "大包装", map[string]any{"effective_sales_spec": map[string]any{"net_content_qty": 0.5, "net_content_unit": "kg"}}, 500, true},
		{"frozen pound", "大包装", map[string]any{"effective_sales_spec": map[string]any{"net_content_qty": 1.0, "net_content_unit": "lb"}}, 454, true},
		{"frozen milligram", "样品", map[string]any{"effective_sales_spec": map[string]any{"net_content_qty": 1500.0, "net_content_unit": "mg"}}, 1, true},
		{"frozen explicit grams", "样品", map[string]any{"effective_sales_spec": map[string]any{"spec_g": 750.0}}, 750, true},
		{"pre-hydration label", "1kg", nil, 1, true},
		{"conflicting value", "1kg", nil, 123, false},
		{"unverifiable value", "大包装", nil, 1000, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := retainedSnapshotSpecMatches(tc.grams, tc.label, tc.source); got != tc.want {
				t.Fatalf("label=%q grams=%d got %v want %v", tc.label, tc.grams, got, tc.want)
			}
		})
	}
}
