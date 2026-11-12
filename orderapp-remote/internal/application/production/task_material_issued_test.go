package production

import "testing"

func TestTaskReadinessCountsOnlyItsOwnPostedInputs(t *testing.T) {
	tasks := []ProductionTask{{JobCardID: 201, WorkOrderID: 99, SequenceNo: 1, PlannedInputQty: 50, Status: "paused", StatusLabel: "已暂停"}, {JobCardID: 202, WorkOrderID: 99, SequenceNo: 1, PlannedInputQty: 50, Status: "pending"}}
	cards := []JobCardRow{{ID: 201, WorkOrderID: 99, SequenceNo: 1, PlannedInputQty: 50, Status: "paused"}, {ID: 202, WorkOrderID: 99, SequenceNo: 1, PlannedInputQty: 50, Status: "pending"}}
	coverage := ProductionWIPStatus{DataComplete: true, StartedInputsKnown: true, Materials: []WIPReservationRow{{MaterialID: 30, ItemType: "material", ItemID: 30, Unit: "kg", RequiredG: 100000, AvailableG: 30000}}, StartedInputs: []TaskMaterialIssued{{JobCardID: 201, ItemType: "material", ItemID: 30, Warehouse: "wip", Unit: "g", IssuedQty: 50000, InProcessQty: 20000}}}
	applyTaskBatchAndMaterialReadiness(tasks, cards, map[int64]ProductionWIPStatus{99: coverage})
	a, b := tasks[0].MaterialReadiness[0], tasks[1].MaterialReadiness[0]
	if a.IssuedG != 50000 || a.InProcessG != 20000 || a.RemainingRequiredG != 0 || a.ShortageG != 0 || tasks[0].StatusLabel != "已暂停" {
		t.Fatalf("already issued task incorrectly blocked: %+v / %+v", tasks[0], a)
	}
	if b.IssuedG != 0 || b.WIPAvailableG != 30000 || b.ShortageG != 20000 {
		t.Fatalf("unstarted task must use only remaining WIP: %+v", b)
	}
}

func TestTaskReadinessPartialIssueAndUnknownLegacy(t *testing.T) {
	tasks := []ProductionTask{{JobCardID: 201, WorkOrderID: 99, SequenceNo: 1, PlannedInputQty: 10, Status: "running", StatusLabel: "执行中"}}
	cards := []JobCardRow{{ID: 201, WorkOrderID: 99, SequenceNo: 1, PlannedInputQty: 10, Status: "running"}}
	coverage := ProductionWIPStatus{DataComplete: true, StartedInputsKnown: true, Materials: []WIPReservationRow{{MaterialID: 30, ItemType: "material", ItemID: 30, Unit: "kg", RequiredG: 10000, AvailableG: 1500}}, StartedInputs: []TaskMaterialIssued{{JobCardID: 201, ItemType: "material", ItemID: 30, Warehouse: "wip", Unit: "kg", IssuedQty: 7.25, InProcessQty: 2}}}
	applyTaskBatchAndMaterialReadiness(tasks, cards, map[int64]ProductionWIPStatus{99: coverage})
	m := tasks[0].MaterialReadiness[0]
	if m.IssuedG != 7250 || m.RemainingRequiredG != 2750 || m.ShortageG != 1250 || tasks[0].StatusLabel != "执行中" {
		t.Fatalf("partial issue mismatch: %+v %+v", tasks[0], m)
	}
	coverage.StartedInputsKnown = false
	coverage.StartedInputs = nil
	coverage.Materials[0].ConsumedG = 7250
	tasks[0] = ProductionTask{JobCardID: 201, WorkOrderID: 99, SequenceNo: 1, PlannedInputQty: 10, Status: "running", StatusLabel: "执行中"}
	applyTaskBatchAndMaterialReadiness(tasks, cards, map[int64]ProductionWIPStatus{99: coverage})
	if tasks[0].MaterialReadinessState != "unknown" || tasks[0].StatusLabel != "执行中" {
		t.Fatalf("ambiguous historical issue must be unknown, not guessed: %+v", tasks[0])
	}
}

func TestTaskReadinessDuplicateRowsSharePostedBudget(t *testing.T) {
	tasks := []ProductionTask{{JobCardID: 201, WorkOrderID: 99, SequenceNo: 1, PlannedInputQty: 10, Status: "running"}}
	cards := []JobCardRow{{ID: 201, WorkOrderID: 99, SequenceNo: 1, PlannedInputQty: 10, Status: "running"}}
	item := WIPReservationRow{MaterialID: 30, ItemType: "material", ItemID: 30, Unit: "kg", RequiredG: 10000}
	coverage := ProductionWIPStatus{DataComplete: true, StartedInputsKnown: true, Materials: []WIPReservationRow{item, item}, StartedInputs: []TaskMaterialIssued{{JobCardID: 201, ItemType: "material", ItemID: 30, Warehouse: "wip", Unit: "kg", IssuedQty: 22, InProcessQty: 22}}}
	applyTaskBatchAndMaterialReadiness(tasks, cards, map[int64]ProductionWIPStatus{99: coverage})
	a, b := tasks[0].MaterialReadiness[0], tasks[0].MaterialReadiness[1]
	if a.IssuedG != 10000 || b.IssuedG != 12000 || a.InProcessG != 10000 || b.InProcessG != 12000 || a.ShortageG != 0 || b.ShortageG != 0 {
		t.Fatalf("duplicate rows improperly allocated posted inputs: %+v %+v", a, b)
	}
}

func TestTaskReadinessMissingPostedComponentsIsUnknown(t *testing.T) {
	tasks := []ProductionTask{{JobCardID: 201, WorkOrderID: 99, SequenceNo: 1, PlannedInputQty: 10, Status: "running", StatusLabel: "执行中", MaterialConsumptionPolicy: "bom_start_v1"}}
	cards := []JobCardRow{{ID: 201, WorkOrderID: 99, SequenceNo: 1, PlannedInputQty: 10, Status: "running"}}
	coverage := ProductionWIPStatus{DataComplete: true, StartedInputsKnown: true, Materials: []WIPReservationRow{{MaterialID: 30, ItemType: "material", ItemID: 30, Unit: "kg", RequiredG: 10000}}}
	applyTaskBatchAndMaterialReadiness(tasks, cards, map[int64]ProductionWIPStatus{99: coverage})
	if tasks[0].MaterialReadinessState != "unknown" || tasks[0].StatusLabel != "执行中" {
		t.Fatalf("missing task input must not be guessed: %+v", tasks[0])
	}
}

func TestTaskReadinessSharedSubgramWIPDoesNotRoundUpEachCard(t *testing.T) {
	tasks := []ProductionTask{{JobCardID: 201, WorkOrderID: 99, SequenceNo: 1, PlannedInputQty: 1, Status: "running"}, {JobCardID: 202, WorkOrderID: 99, SequenceNo: 1, PlannedInputQty: 1, Status: "running"}}
	cards := []JobCardRow{{ID: 201, WorkOrderID: 99, SequenceNo: 1, PlannedInputQty: 1, Status: "running"}, {ID: 202, WorkOrderID: 99, SequenceNo: 1, PlannedInputQty: 1, Status: "running"}}
	coverage := ProductionWIPStatus{DataComplete: true, StartedInputsKnown: true, Materials: []WIPReservationRow{{MaterialID: 30, ItemType: "material", ItemID: 30, Unit: "kg", RequiredG: 2, AvailableG: 1}}, StartedInputs: []TaskMaterialIssued{{JobCardID: 201, ItemType: "material", ItemID: 30, Warehouse: "wip", Unit: "g", IssuedQty: 0.5}, {JobCardID: 202, ItemType: "material", ItemID: 30, Warehouse: "wip", Unit: "g", IssuedQty: 0.5}}}
	applyTaskBatchAndMaterialReadiness(tasks, cards, map[int64]ProductionWIPStatus{99: coverage})
	a, b := tasks[0].MaterialReadiness[0], tasks[1].MaterialReadiness[0]
	if a.WIPAvailableG != 0.5 || b.WIPAvailableG != 0.5 || a.ShortageG != 0 || b.ShortageG != 0 {
		t.Fatalf("subgram allocation rounded prematurely: %+v %+v", a, b)
	}
}

func TestTaskReadinessMatchesFrozenProductAlias(t *testing.T) {
	tasks := []ProductionTask{{JobCardID: 201, WorkOrderID: 99, SequenceNo: 1, PlannedInputQty: 1, Status: "running"}}
	cards := []JobCardRow{{ID: 201, WorkOrderID: 99, SequenceNo: 1, PlannedInputQty: 1, Status: "running"}}
	coverage := ProductionWIPStatus{DataComplete: true, StartedInputsKnown: true, Materials: []WIPReservationRow{{ItemType: "finished_product", ItemID: 8, Unit: "kg", RequiredG: 1000}}, StartedInputs: []TaskMaterialIssued{{JobCardID: 201, ItemType: "product", ItemID: 8, Warehouse: "wip", Unit: "g", IssuedQty: 1000}}}
	applyTaskBatchAndMaterialReadiness(tasks, cards, map[int64]ProductionWIPStatus{99: coverage})
	if tasks[0].MaterialReadiness[0].ShortageG != 0 || tasks[0].MaterialReadiness[0].IssuedG != 1000 {
		t.Fatalf("product alias not recognized: %+v", tasks[0])
	}
}

// Reproduce the observed eight packaging jobs after a shared roasting lot has
// already been consumed. The 0.270 kg still in WIP is not the amount issued.
func TestTaskReadinessEightPostedPackagingOrders(t *testing.T) {
	amounts := []float64{50, 30, 52, 25, 26, 25, 0.23, 1}
	tasks := make([]ProductionTask, 0, len(amounts))
	cards := make([]JobCardRow, 0, len(amounts))
	coverage := map[int64]ProductionWIPStatus{}
	for i, kg := range amounts {
		id := int64(100 + i)
		status, label := "running", "执行中"
		if i == 0 {
			status, label = "paused", "已暂停"
		}
		tasks = append(tasks, ProductionTask{JobCardID: id, WorkOrderID: id, SequenceNo: 1, PlannedInputQty: kg, Status: status, StatusLabel: label, MaterialConsumptionPolicy: "bom_start_v1"})
		cards = append(cards, JobCardRow{ID: id, WorkOrderID: id, SequenceNo: 1, PlannedInputQty: kg, Status: status})
		coverage[id] = ProductionWIPStatus{DataComplete: true, StartedInputsKnown: true, Materials: []WIPReservationRow{{ItemType: "material", ItemID: 121, Unit: "kg", RequiredG: int64(kg * 1000), AvailableG: 270}}, StartedInputs: []TaskMaterialIssued{{JobCardID: id, ItemType: "material", ItemID: 121, Warehouse: "wip", Unit: "kg", IssuedQty: kg, InProcessQty: kg}}}
	}
	applyTaskBatchAndMaterialReadiness(tasks, cards, coverage)
	for i, task := range tasks {
		if task.MaterialReadinessState != "ready" || task.IsBlocked || len(task.MaterialReadiness) != 1 {
			t.Fatalf("order %d not ready: %+v", i, task)
		}
		row := task.MaterialReadiness[0]
		if row.IssuedG != amounts[i]*1000 || row.RemainingRequiredG != 0 || row.WIPAvailableG != 0 || row.ShortageG != 0 {
			t.Fatalf("order %d has incorrect coverage: %+v", i, row)
		}
		if (i == 0 && task.StatusLabel != "已暂停") || (i > 0 && task.StatusLabel != "执行中") {
			t.Fatalf("order %d status overwritten: %+v", i, task)
		}
	}
}

func TestTaskReadinessDoesNotBorrowDifferentOwnerOrWarehouse(t *testing.T) {
	for _, tc := range []struct {
		name      string
		owner     int64
		warehouse string
		spec      int64
	}{
		{"owner", 7, "wip", 2}, {"warehouse", 1, "raw", 2}, {"spec", 1, "wip", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task := []ProductionTask{{JobCardID: 201, WorkOrderID: 99, SequenceNo: 1, PlannedInputQty: 1, Status: "running"}}
			cards := []JobCardRow{{ID: 201, WorkOrderID: 99, SequenceNo: 1, PlannedInputQty: 1, Status: "running"}}
			coverage := ProductionWIPStatus{DataComplete: true, StartedInputsKnown: true, Materials: []WIPReservationRow{{ItemType: "product", ItemID: 8, BomSpecID: 2, OwnerCustomerID: 1, SourceWarehouse: "wip", Unit: "kg", RequiredG: 1000}}, StartedInputs: []TaskMaterialIssued{{JobCardID: 201, ItemType: "product", ItemID: 8, BomSpecID: tc.spec, OwnerCustomerID: tc.owner, Warehouse: tc.warehouse, Unit: "kg", IssuedQty: 1}}}
			applyTaskBatchAndMaterialReadiness(task, cards, map[int64]ProductionWIPStatus{99: coverage})
			row := task[0].MaterialReadiness[0]
			if row.IssuedG != 0 || row.ShortageG != 1000 {
				t.Fatalf("borrowed incompatible input: %+v", row)
			}
		})
	}
}
