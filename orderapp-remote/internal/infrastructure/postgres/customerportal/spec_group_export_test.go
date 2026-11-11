package customerportal

import (
	app "orderapp/internal/application/customerportal"
	"testing"
)

func TestBeanListSnapshotFlagsGroupPricingForPNG(t *testing.T) {
	cases := []struct {
		content string
		want    bool
	}{
		{`{"price_rows":[{"spec_pricing_group_id":1}]}`, true},
		{`{"price_rows":[{"pricing_rule_id":1}]}`, false},
	}
	for _, tc := range cases {
		row := app.BeanListSummary{ID: 1}
		if err := parseBeanListDisplaySummary([]byte(`{}`), []byte(tc.content), &row); err != nil {
			t.Fatal(err)
		}
		if row.SpecPricingGroup != tc.want {
			t.Fatalf("content=%s got=%v", tc.content, row.SpecPricingGroup)
		}
	}
}
