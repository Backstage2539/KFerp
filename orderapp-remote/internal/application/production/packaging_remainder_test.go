package production

import "testing"

func TestPackagingRemainderSettlement(t *testing.T) {
	tests := []struct {
		name                   string
		in                     PackagingRemainderInput
		wantUsed, wantEstimate float64
		fail                   bool
	}{
		{"exact remainder", PackagingRemainderInput{AvailableG: 83000, CompletedQty: 180, RemainingTargetQty: 180, StandardGPerPiece: 454, ActualRemainderG: 1280}, 81720, 1280, false},
		{"weighed variation", PackagingRemainderInput{AvailableG: 83000, CompletedQty: 180, RemainingTargetQty: 180, StandardGPerPiece: 454, ActualRemainderG: 1200, DifferenceReason: "装袋差异"}, 81800, 1280, false},
		{"variation requires reason", PackagingRemainderInput{AvailableG: 83000, CompletedQty: 180, RemainingTargetQty: 180, StandardGPerPiece: 454, ActualRemainderG: 1200}, 0, 0, true},
		{"negative remainder", PackagingRemainderInput{AvailableG: 83000, CompletedQty: 180, RemainingTargetQty: 180, StandardGPerPiece: 454, ActualRemainderG: -1}, 0, 0, true},
		{"too much remainder", PackagingRemainderInput{AvailableG: 83000, CompletedQty: 180, RemainingTargetQty: 180, StandardGPerPiece: 454, ActualRemainderG: 83001}, 0, 0, true},
		{"over target", PackagingRemainderInput{AvailableG: 83000, CompletedQty: 181, RemainingTargetQty: 180, StandardGPerPiece: 454}, 0, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CalculatePackagingRemainder(tt.in)
			if (err != nil) != tt.fail {
				t.Fatalf("error = %v, fail=%v", err, tt.fail)
			}
			if err == nil && (got.UsedG != tt.wantUsed || got.EstimatedRemainderG != tt.wantEstimate) {
				t.Fatalf("got %#v", got)
			}
		})
	}
}

func TestPackagingComponentAllocationDoesNotProportionallyConsumePackaging(t *testing.T) {
	balances := []PackagingComponentBalance{{ID: 1, Key: "bean", Quantity: 83000, Cost: 166}, {ID: 2, Key: "bag", Quantity: 200, Cost: 100}}
	alloc, err := AllocatePackagingComponents(balances, map[string]float64{"bean": 81720, "bag": 180})
	if err != nil {
		t.Fatal(err)
	}
	if len(alloc) != 2 || alloc[0].Quantity != 81720 || alloc[1].Quantity != 180 || alloc[1].Cost != 90 {
		t.Fatalf("unexpected allocations: %#v", alloc)
	}
	if _, err = AllocatePackagingComponents(balances, map[string]float64{"bean": 83001}); err == nil {
		t.Fatal("expected insufficient balance")
	}
}

func TestPackagingComponentAllocationAcrossBatchesPreservesCosts(t *testing.T) {
	balances := []PackagingComponentBalance{{ID: 1, Key: "bean", Quantity: 30000, Cost: 60}, {ID: 2, Key: "bean", Quantity: 53000, Cost: 159}, {ID: 3, Key: "bag", Quantity: 200, Cost: 100}}
	alloc, err := AllocatePackagingComponents(balances, map[string]float64{"bean": 81720, "bag": 180})
	if err != nil {
		t.Fatal(err)
	}
	if len(alloc) != 3 || alloc[0].Quantity != 30000 || alloc[0].Cost != 60 || alloc[1].Quantity != 51720 || alloc[1].Cost != 155.16 || alloc[2].Quantity != 180 || alloc[2].Cost != 90 {
		t.Fatalf("incorrect quantities or cost: %#v", alloc)
	}
	for _, bad := range []map[string]float64{{"bean": -1}, {"bean": 1.5}, {"missing": 1}} {
		if _, err := AllocatePackagingComponents(balances, bad); err == nil {
			t.Fatalf("accepted invalid request: %v", bad)
		}
	}
}

func TestPackagingRemainderCannotProduceWithoutMaterial(t *testing.T) {
	_, err := CalculatePackagingRemainder(PackagingRemainderInput{AvailableG: 1000, CompletedQty: 1, RemainingTargetQty: 1, StandardGPerPiece: 454, ActualRemainderG: 1000, DifferenceReason: "盘点"})
	if err == nil {
		t.Fatal("completed package without any material")
	}
}

func TestFrozenPackagingBeanIdentity(t *testing.T) {
	materials := []OperationMaterialUse{{Key: "material:11:0:0", MaterialID: 11, Unit: "kg", TotalQty: 83, OwnerCustomerID: 7}, {Key: "material:12:0:0", MaterialID: 12, Unit: "袋", TotalQty: 180}}
	got, err := ResolvePackagingWeightComponent(materials)
	if err != nil || got.Key != "material:11:0:0" || got.OwnerCustomerID != 7 {
		t.Fatalf("got %#v err=%v", got, err)
	}
	if _, err := ResolvePackagingWeightComponent(materials[1:]); err == nil {
		t.Fatal("missing bean accepted")
	}
	materials = append(materials, OperationMaterialUse{Key: "material:13:0:0", MaterialID: 13, Unit: "g", TotalQty: 100})
	if _, err := ResolvePackagingWeightComponent(materials); err == nil {
		t.Fatal("ambiguous bean accepted")
	}
}
