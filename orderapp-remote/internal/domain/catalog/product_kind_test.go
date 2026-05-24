package catalog

import "testing"

func TestNormalizeProductKindPreservesGreenBeanAndDripBag(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty defaults to roasted", input: "", want: ProductKindRoasted},
		{name: "roasted canonical", input: ProductKindRoasted, want: ProductKindRoasted},
		{name: "legacy roasted bean alias", input: "roasted_bean", want: ProductKindRoasted},
		{name: "drip bag canonical", input: ProductKindDripBag, want: ProductKindDripBag},
		{name: "instant coffee canonical", input: ProductKindInstantCoffee, want: ProductKindInstantCoffee},
		{name: "instant coffee Chinese alias", input: "速溶咖啡", want: ProductKindInstantCoffee},
		{name: "instant alias", input: "instant", want: ProductKindInstantCoffee},
		{name: "green bean canonical", input: ProductKindGreenBean, want: ProductKindGreenBean},
		{name: "green alias", input: "green", want: ProductKindGreenBean},
		{name: "raw alias", input: "raw_bean", want: ProductKindGreenBean},
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

func TestProductKindLabelsDistinguishKnownKinds(t *testing.T) {
	cases := map[string]string{
		ProductKindRoasted:       "熟豆",
		ProductKindGreenBean:     "生豆",
		ProductKindDripBag:       "挂耳",
		ProductKindInstantCoffee: "速溶咖啡",
	}
	for kind, want := range cases {
		if got := ProductKindLabel(kind); got != want {
			t.Fatalf("ProductKindLabel(%q) = %q, want %q", kind, got, want)
		}
	}
}
