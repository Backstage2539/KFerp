package production

import "testing"

func TestWorkstationUsesFrozenSalesSpecLabel(t *testing.T) {
	wo := WorkOrderRow{SalesSpecSnapshotJSON: `{"spec_label":"2.5kg袋装"}`, SpecG: 0}
	task := productionTaskFromJobCard(JobCardRow{}, wo)
	if task.SpecLabel != "2.5kg袋装" {
		t.Fatalf("frozen spec label = %q", task.SpecLabel)
	}
	wo.SalesSpecSnapshotJSON = `{}`
	task = productionTaskFromJobCard(JobCardRow{}, wo)
	if task.SpecLabel != "" {
		t.Fatalf("missing spec must not be guessed: %q", task.SpecLabel)
	}
}

func TestWorkstationUsesRelatedOrdersFromPlan(t *testing.T) {
	wo := WorkOrderRow{RelatedOrders: []ProductionDemandOrder{{OrderID: 83, OrderItemID: 173, CustomerName: "工厂客户", Quantity: 50, SalesUnit: "袋"}}}
	task := productionTaskFromJobCard(JobCardRow{}, wo)
	if len(task.RelatedOrders) != 1 || task.RelatedOrders[0].OrderItemID != 173 || task.RelatedOrders[0].CustomerName != "工厂客户" {
		t.Fatalf("related orders: %+v", task.RelatedOrders)
	}
}

func TestWorkstationShowsOnlyFrozenCurrentOperationNotes(t *testing.T) {
	wo := WorkOrderRow{RelatedOrders: []ProductionDemandOrder{{OrderItemID: 5, OperationNotes: []ProductionOrderOperationNote{{OperationID: 7, SequenceNo: 1, Note: "浅烘"}, {OperationID: 8, SequenceNo: 2, Note: "贴标"}}}}}
	task := productionTaskFromJobCard(JobCardRow{OperationID: 8, SequenceNo: 2}, wo)
	if len(task.RelatedOrders) != 1 || len(task.RelatedOrders[0].OperationNotes) != 1 || task.RelatedOrders[0].OperationNotes[0].Note != "贴标" {
		t.Fatalf("notes mismatched: %+v", task.RelatedOrders)
	}
	if len(wo.RelatedOrders[0].OperationNotes) != 2 {
		t.Fatal("must not mutate frozen work order")
	}
}

func TestWorkstationTaskIncludesFrozenProductAndSpecIdentity(t *testing.T) {
	wo := WorkOrderRow{ID: 1, ProductID: 22, OutputProductID: 22, BomSpecID: 33, BomVariantID: 44}
	for _, task := range []ProductionTask{productionTaskFromJobCard(JobCardRow{}, wo), productionTaskFromWorkOrder(wo)} {
		if task.ProductID != 22 || task.BomSpecID != 33 || task.BomVariantID != 44 {
			t.Fatalf("frozen identity missing: %+v", task)
		}
	}
}
