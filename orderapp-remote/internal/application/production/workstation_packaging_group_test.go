package production

import "testing"

func TestPackagingGroupKeysAcceptEquivalentLegacyZeroSplitCards(t *testing.T) {
	quantities := []float64{10, 6, 4, 1}
	tasks := make([]ProductionTask, 0, len(quantities))
	orders := make(map[int64]WorkOrderRow, len(quantities))
	cards := make([]JobCardRow, 0, len(quantities))
	for i, qty := range quantities {
		id := int64(i + 1)
		tasks = append(tasks, ProductionTask{
			JobCardID: id, WorkOrderID: id, SequenceNo: 1, Operation: "包装", Workstation: "包装台",
			Status: "pending", PlannedOutputInventoryQty: qty, InventoryUnit: "袋", SpecG: 454,
			TargetWarehouse: "finished_goods", PlannedInputQty: 0, MaterialDataComplete: true,
			MaterialReadiness: []ProductionTaskMaterialReadiness{
				{MaterialID: 127, MaterialName: "小菠萝-半成品", Unit: "g", RequiredG: int64(qty * 454), WIPAvailableG: int64(qty * 454), SourceWarehouse: "wip"},
				{MaterialID: 100, MaterialName: "454g银色袋", Unit: "pic", RequiredUnits: int64(qty), SourceWarehouse: "packaging"},
			},
		})
		orders[id] = WorkOrderRow{
			ID: id, OutputProductID: 57, ProductName: "小菠萝", OutputName: "小菠萝",
			SpecG: 454, CustomerID: int64(100 + i), BomVariantID: int64(48 + i),
			TargetWarehouse:     "finished_goods",
			ProcessSnapshotJSON: `{"operations":[{"seq":1,"operation":"包装","execution_json":""}]}`,
		}
		cards = append(cards, JobCardRow{ID: id, WorkOrderID: id, SequenceNo: 1, Status: "pending", PlannedInputQty: 0})
	}

	applyPackagingGroupKeys(tasks, orders, cards)
	key := tasks[0].PackagingGroupKey
	if key == "" {
		t.Fatal("eligible legacy packaging cards should receive a server-generated group key")
	}
	for _, task := range tasks {
		if task.PackagingGroupKey != key {
			t.Fatalf("equivalent orders must group despite customer/BOM variant differences: %+v", task)
		}
		if !task.AutomaticCompletion {
			t.Fatalf("unique zero-input legacy packaging card should be safely eligible: %+v", task)
		}
	}

	// Ambiguous split history must fail closed instead of assuming one card per order.
	cards = append(cards, JobCardRow{ID: 99, WorkOrderID: 1, SequenceNo: 1, Status: "pending", PlannedInputQty: 0})
	applyPackagingGroupKeys(tasks, orders, cards)
	if tasks[0].AutomaticCompletion {
		t.Fatal("legacy order with multiple same-sequence cards must not be auto-frozen")
	}
}

func TestPackagingGroupKeysSeparateDifferentOwnerOrSourceWarehouse(t *testing.T) {
	makeTask := func(id int64, owner int64, source string) (ProductionTask, WorkOrderRow, JobCardRow) {
		task := ProductionTask{JobCardID: id, WorkOrderID: id, SequenceNo: 1, Operation: "包装", Workstation: "包装台", Status: "pending", PlannedOutputInventoryQty: 1, InventoryUnit: "袋", SpecG: 454, TargetWarehouse: "finished_goods", MaterialDataComplete: true, MaterialReadiness: []ProductionTaskMaterialReadiness{{MaterialID: 100, OwnerCustomerID: owner, SourceWarehouse: source, Unit: "pic", RequiredUnits: 1}}}
		order := WorkOrderRow{ID: id, OutputProductID: 57, ProductName: "小菠萝", SpecG: 454, TargetWarehouse: "finished_goods"}
		card := JobCardRow{ID: id, WorkOrderID: id, SequenceNo: 1, Status: "pending"}
		return task, order, card
	}
	tasks := []ProductionTask{}
	orders := map[int64]WorkOrderRow{}
	cards := []JobCardRow{}
	for i, identity := range []struct {
		owner  int64
		source string
	}{{0, "wip"}, {0, "packaging"}, {8, "wip"}} {
		id := int64(i + 1)
		task, order, card := makeTask(id, identity.owner, identity.source)
		tasks = append(tasks, task)
		orders[id] = order
		cards = append(cards, card)
	}
	applyPackagingGroupKeys(tasks, orders, cards)
	if tasks[0].PackagingGroupKey == tasks[1].PackagingGroupKey || tasks[0].PackagingGroupKey == tasks[2].PackagingGroupKey {
		t.Fatal("different inventory source warehouses and owners must remain separate groups")
	}
}

func TestPackagingGroupKeysSeparateDifferentFrozenMaterialSpec(t *testing.T) {
	makeTask := func(id int64, spec int64) (ProductionTask, WorkOrderRow, JobCardRow) {
		task := ProductionTask{JobCardID: id, WorkOrderID: id, SequenceNo: 1, Operation: "包装", Workstation: "包装台", Status: "pending", PlannedOutputInventoryQty: 1, InventoryUnit: "袋", SpecG: 454, TargetWarehouse: "finished_goods", MaterialDataComplete: true, MaterialReadiness: []ProductionTaskMaterialReadiness{{MaterialID: 100, BomSpecID: spec, SpecG: spec, Unit: "pic", RequiredUnits: 1}}}
		order := WorkOrderRow{ID: id, OutputProductID: 57, ProductName: "小菠萝", SpecG: 454, TargetWarehouse: "finished_goods"}
		card := JobCardRow{ID: id, WorkOrderID: id, SequenceNo: 1, Status: "pending"}
		return task, order, card
	}
	tasks := []ProductionTask{}
	orders := map[int64]WorkOrderRow{}
	cards := []JobCardRow{}
	for i, spec := range []int64{227, 454} {
		id := int64(i + 1)
		task, order, card := makeTask(id, spec)
		tasks = append(tasks, task)
		orders[id] = order
		cards = append(cards, card)
	}
	applyPackagingGroupKeys(tasks, orders, cards)
	if tasks[0].PackagingGroupKey == tasks[1].PackagingGroupKey {
		t.Fatal("different frozen material specifications must remain separate groups")
	}
}

func TestPackagingGroupKeysDoNotCombineMultipleCardsFromOneWorkOrder(t *testing.T) {
	tasks := []ProductionTask{
		{JobCardID: 1, WorkOrderID: 1, SequenceNo: 1, Operation: "包装", Workstation: "包装台", Status: "pending", PlannedOutputInventoryQty: 1, InventoryUnit: "袋", SpecG: 454, TargetWarehouse: "finished_goods", MaterialDataComplete: true, MaterialReadiness: []ProductionTaskMaterialReadiness{{MaterialID: 100, Unit: "pic", RequiredUnits: 1}}},
		{JobCardID: 2, WorkOrderID: 1, SequenceNo: 1, Operation: "包装", Workstation: "包装台", Status: "pending", PlannedOutputInventoryQty: 1, InventoryUnit: "袋", SpecG: 454, TargetWarehouse: "finished_goods", MaterialDataComplete: true, MaterialReadiness: []ProductionTaskMaterialReadiness{{MaterialID: 100, Unit: "pic", RequiredUnits: 1}}},
	}
	orders := map[int64]WorkOrderRow{1: {ID: 1, OutputProductID: 57, ProductName: "小菠萝", SpecG: 454, TargetWarehouse: "finished_goods"}}
	cards := []JobCardRow{{ID: 1, WorkOrderID: 1, SequenceNo: 1}, {ID: 2, WorkOrderID: 1, SequenceNo: 1}}
	applyPackagingGroupKeys(tasks, orders, cards)
	for _, task := range tasks {
		if task.PackagingGroupKey != "" {
			t.Fatalf("multiple cards from one work order must remain separate: %+v", task)
		}
	}
}

func TestPackagingGroupKeyStillAllowsOnePackagingCardAcrossMultiOperationWorkOrder(t *testing.T) {
	task := ProductionTask{JobCardID: 3, WorkOrderID: 1, SequenceNo: 2, Operation: "包装", Workstation: "包装台", Status: "pending", PlannedOutputInventoryQty: 1, InventoryUnit: "袋", SpecG: 454, TargetWarehouse: "finished_goods", MaterialDataComplete: true, MaterialReadiness: []ProductionTaskMaterialReadiness{{MaterialID: 100, Unit: "pic", RequiredUnits: 1}}}
	order := WorkOrderRow{ID: 1, OutputProductID: 57, ProductName: "小菠萝", SpecG: 454, TargetWarehouse: "finished_goods"}
	cards := []JobCardRow{{ID: 2, WorkOrderID: 1, SequenceNo: 1}, {ID: 3, WorkOrderID: 1, SequenceNo: 2}}
	tasks := []ProductionTask{task}
	applyPackagingGroupKeys(tasks, map[int64]WorkOrderRow{1: order}, cards)
	if tasks[0].PackagingGroupKey == "" {
		t.Fatal("one packaging card in a multi-operation work order should be eligible for grouping")
	}
}
