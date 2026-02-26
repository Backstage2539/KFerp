package main

import "testing"

func TestValidateCreateProduceBatchRequest(t *testing.T) {
	ok := CreateProduceBatchRequest{OrderIDs: []int64{1}, BatchID: "B001", Operator: "jj", IdempotencyKey: "k1"}
	if err := validateCreateProduceBatchRequest(ok); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	cases := []CreateProduceBatchRequest{
		{BatchID: "B001", Operator: "jj", IdempotencyKey: "k1"},
		{OrderIDs: []int64{1}, Operator: "jj", IdempotencyKey: "k1"},
		{OrderIDs: []int64{1}, BatchID: "B001", IdempotencyKey: "k1"},
		{OrderIDs: []int64{1}, BatchID: "B001", Operator: "jj"},
	}
	for i, c := range cases {
		if err := validateCreateProduceBatchRequest(c); err == nil {
			t.Fatalf("case %d expected error", i)
		}
	}
}
