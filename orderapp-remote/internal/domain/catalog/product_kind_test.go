package catalog

import "testing"

func TestNormalizeProductKindDefaultsAndAcceptsGreenBean(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty defaults to roasted", input: "", want: ProductKindRoasted},
		{name: "roasted stays roasted", input: "roasted", want: ProductKindRoasted},
		{name: "green bean canonical", input: "green_bean", want: ProductKindGreenBean},
		{name: "green alias", input: "green", want: ProductKindGreenBean},
		{name: "Chinese alias", input: "生豆", want: ProductKindGreenBean},
		{name: "unknown defaults to roasted", input: "unexpected", want: ProductKindRoasted},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeProductKind(tc.input); got != tc.want {
				t.Fatalf("NormalizeProductKind(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestProductKindLabelsDistinguishGreenAndRoasted(t *testing.T) {
	if ProductKindLabel(ProductKindRoasted) != "熟豆" {
		t.Fatalf("roasted label = %q", ProductKindLabel(ProductKindRoasted))
	}
	if ProductKindLabel(ProductKindGreenBean) != "生豆" {
		t.Fatalf("green label = %q", ProductKindLabel(ProductKindGreenBean))
	}
}
