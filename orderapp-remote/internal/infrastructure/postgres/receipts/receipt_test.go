package receipts

import "testing"

func TestReceiptValidate(t *testing.T) {
	good := Receipt{CustomerID: 5, SourceType: "combined_payment", SourceID: 7, ActualCents: 12500, ReceivableCents: 15000, Method: "微信", Actor: "tester", Allocations: []byte(`[{"order_id":11,"amount_cents":12000}]`)}
	if err := good.Validate(); err != nil {
		t.Fatal(err)
	}
	cases := []Receipt{good, good, good, good, good, good}
	cases[0].CustomerID = 0
	cases[1].SourceID = 0
	cases[2].ActualCents = 0
	cases[3].ReceivableCents = 0
	cases[4].Actor = " "
	cases[5].Allocations = []byte(`bad`)
	for i, c := range cases {
		if c.Validate() == nil {
			t.Errorf("case %d accepted", i)
		}
	}
}
