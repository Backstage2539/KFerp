package main

import (
	"reflect"
	"testing"
)

func TestProduceBatchListItem_HasStatusAndCreatedAtFields(t *testing.T) {
	typ := reflect.TypeOf(ProduceBatchListItem{})
	if _, ok := typ.FieldByName("Status"); !ok {
		t.Fatalf("expected field Status")
	}
	if _, ok := typ.FieldByName("CreatedAt"); !ok {
		t.Fatalf("expected field CreatedAt")
	}
}

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
