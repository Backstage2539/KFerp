package production

import "testing"

func TestOperationHandoffIsPerWorkOrderAndDoesNotAllocatePlannedQuota(t *testing.T) {
	cards := []JobCardRow{
		{ID: 1, WorkOrderID: 1, SequenceNo: 1, Status: "completed", ActualOutputQty: 8.6},
		{ID: 2, WorkOrderID: 1, SequenceNo: 1, Status: "completed", ActualOutputQty: 2.6},
		{ID: 3, WorkOrderID: 1, SequenceNo: 1, Status: "cancelled", ActualOutputQty: 999},
		{ID: 4, WorkOrderID: 2, SequenceNo: 1, Status: "pending", ActualOutputQty: 999},
	}
	tasks := []ProductionTask{
		{JobCardID: 5, WorkOrderID: 1, SequenceNo: 2, Status: "pending", PlannedInputQty: 10000, AvailableActions: []string{"start"}},
		{JobCardID: 6, WorkOrderID: 1, SequenceNo: 2, Status: "pending", PlannedInputQty: 9000, AvailableActions: []string{"start"}},
		{JobCardID: 7, WorkOrderID: 2, SequenceNo: 2, Status: "pending", PlannedInputQty: 1, AvailableActions: []string{"start"}},
	}
	applyTaskOperationHandoff(tasks, cards)
	for _, task := range tasks[:2] {
		if task.IsBlocked || !task.CanStart || task.PredecessorOutputQty != 11.2 || !task.PredecessorCompleted {
			t.Fatalf("completed predecessor should be shared information, not a per-batch quota: %+v", task)
		}
	}
	if !tasks[2].IsBlocked || tasks[2].CanStart {
		t.Fatalf("another work order must remain blocked: %+v", tasks[2])
	}
}

func TestOperationHandoffPreservesExistingQualityBlock(t *testing.T) {
	tasks := []ProductionTask{{JobCardID: 2, WorkOrderID: 1, SequenceNo: 2, Status: "pending", IsBlocked: true, BlockingReason: "质检冻结", NextHandler: "质检"}}
	applyTaskOperationHandoff(tasks, []JobCardRow{{ID: 1, WorkOrderID: 1, SequenceNo: 1, Status: "completed", ActualOutputQty: 2}})
	if tasks[0].CanStart || tasks[0].BlockingReason != "质检冻结" || tasks[0].NextHandler != "质检" {
		t.Fatalf("handoff must not clear another block: %+v", tasks[0])
	}
}
