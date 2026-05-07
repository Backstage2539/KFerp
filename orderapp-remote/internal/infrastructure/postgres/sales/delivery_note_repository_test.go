package sales

import "testing"

func TestDeliveryMethodDisplayNameHidesInternalCodes(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
	}{
		{in: "sf_small", want: "顺丰发货"},
		{in: "sf_large", want: "顺丰大件"},
		{in: "sf_express", want: "顺丰标快"},
		{in: "顺丰冷运", want: "顺丰冷运"},
	} {
		if got := deliveryMethodDisplayName(tc.in); got != tc.want {
			t.Fatalf("deliveryMethodDisplayName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
