package catalog

import "testing"

func TestNormalizeProductKindPreservesGreenBeanAndDripBag(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty defaults to roasted bean", input: "", want: ProductKindRoastedBean},
		{name: "roasted bean canonical", input: ProductKindRoastedBean, want: ProductKindRoastedBean},
		{name: "legacy roasted alias", input: "roasted", want: ProductKindRoastedBean},
		{name: "drip bag canonical", input: ProductKindDripBag, want: ProductKindDripBag},
		{name: "green bean canonical", input: ProductKindGreenBean, want: ProductKindGreenBean},
		{name: "green alias", input: "green", want: ProductKindGreenBean},
		{name: "raw alias", input: "raw_bean", want: ProductKindGreenBean},
		{name: "Chinese alias", input: "生豆", want: ProductKindGreenBean},
		{name: "unknown defaults to roasted bean", input: "unexpected", want: ProductKindRoastedBean},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeProductKind(tc.input); got != tc.want {
				t.Fatalf("NormalizeProductKind(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
