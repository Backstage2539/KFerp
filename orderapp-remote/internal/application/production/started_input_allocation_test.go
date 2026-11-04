package production

import (
	"math"
	"testing"
)

func TestAllocateStartedInputCostPartialAndFinal(t *testing.T) {
	inputs := []StartedInputBalance{{ID: 1, RemainingQty: 10, RemainingCost: 17.25}, {ID: 2, RemainingQty: 5, RemainingCost: 9}}
	got, err := AllocateStartedInputCost(inputs, 12)
	if err != nil || len(got) != 2 || got[0].Qty != 10 || got[0].Cost != 17.25 || got[1].Qty != 2 || got[1].Cost != 3.6 {
		t.Fatalf("allocation = %+v, %v", got, err)
	}
	got, err = AllocateStartedInputCost([]StartedInputBalance{{ID: 2, RemainingQty: 3, RemainingCost: 5.4}}, 3)
	if err != nil || len(got) != 1 || got[0].Cost != 5.4 {
		t.Fatalf("final allocation = %+v, %v", got, err)
	}
}

func TestAllocateStartedInputCostRejectsInvalidOrInsufficient(t *testing.T) {
	for _, qty := range []float64{-1, math.NaN(), math.Inf(1), 2} {
		if _, err := AllocateStartedInputCost([]StartedInputBalance{{ID: 1, RemainingQty: 1, RemainingCost: 2}}, qty); err == nil {
			t.Fatalf("accepted invalid quantity %v", qty)
		}
	}
	if _, err := AllocateStartedInputCost([]StartedInputBalance{{ID: 1, RemainingQty: 1, RemainingCost: -2}}, 1); err == nil {
		t.Fatal("accepted negative cost")
	}
}

func TestAllocateStartedInputCostPreservesSmallestLedgerQuantity(t *testing.T) {
	got, err := AllocateStartedInputCost([]StartedInputBalance{{ID: 1, RemainingQty: 0.00000001, RemainingCost: 0.01}}, 0.00000001)
	if err != nil || len(got) != 1 || got[0].Qty != 0.00000001 || got[0].Cost != 0.01 {
		t.Fatalf("smallest ledger quantity was lost: %+v, %v", got, err)
	}
	if _, err := AllocateStartedInputCost([]StartedInputBalance{{ID: 1, RemainingQty: 0.00000001, RemainingCost: 0.01}}, 0.00000002); err == nil {
		t.Fatal("accepted a request exceeding the available ledger quantity")
	}
}

func TestAllocateStartedInputCostRejectsSubprecisionAndDuplicateBalances(t *testing.T) {
	balances := []StartedInputBalance{{ID: 1, RemainingQty: 1, RemainingCost: 1}}
	if _, err := AllocateStartedInputCost(balances, 0.000000004); err == nil {
		t.Fatal("subprecision quantity must not silently allocate nothing")
	}
	if _, err := AllocateStartedInputCost([]StartedInputBalance{{ID: 1, RemainingQty: 1, RemainingCost: 1}, {ID: 1, RemainingQty: 1, RemainingCost: 1}}, 2); err == nil {
		t.Fatal("duplicate input balance must not be allocated twice")
	}
}
