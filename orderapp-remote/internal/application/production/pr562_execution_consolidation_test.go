package production

import (
	"math"
	"strings"
	"testing"
)

func TestPR562JobCardRequirementComesFromFrozenRouteSnapshot(t *testing.T) {
	card := JobCardRow{
		SequenceNo: 2,
		Operation:  "包装",
		ProcessSnapshotJSON: `{
			"route_name":"标准包装路线",
			"operations":[
				{"seq":1,"operation":"烘焙","quality_checklist_json":"[\"颜色\"]"},
				{"seq":2,"operation":"包装","quality_checklist_json":"[\"封口完整\",\"标签位置\"]"}
			]
		}`,
	}
	if got := jobCardProcessRequirement(card); got != "质检项：封口完整、标签位置" {
		t.Fatalf("process requirement = %q", got)
	}

	historical := JobCardRow{SequenceNo: 1, Operation: "历史工序", ProcessSnapshotJSON: `{}`}
	if got := jobCardProcessRequirement(historical); got != "按冻结工艺路线执行" {
		t.Fatalf("historical process requirement = %q", got)
	}

	duplicateNames := JobCardRow{
		SequenceNo:  2,
		OperationID: 22,
		Operation:   "质检",
		ProcessSnapshotJSON: `{
			"operations":[
				{"seq":1,"operation_id":11,"operation":"质检","process_requirement":"检查烘焙色"},
				{"seq":2,"operation_id":22,"operation":"质检","process_requirement":"检查封口"}
			]
		}`,
	}
	if got := jobCardProcessRequirement(duplicateNames); got != "检查封口" {
		t.Fatalf("sequence must win when operation names repeat, got %q", got)
	}

	damaged := JobCardRow{SequenceNo: 1, Operation: "包装", ProcessSnapshotJSON: `{broken`}
	if got := jobCardProcessRequirement(damaged); got != "按冻结工艺路线执行" {
		t.Fatalf("damaged historical snapshot requirement = %q", got)
	}
}

func TestPR562ExecutionHubStartIsACommandAndOtherActionsNavigate(t *testing.T) {
	actions := buildWorkOrderContextActions(
		WorkOrderRow{ID: 88, WorkOrderNo: "WO-PR562-001"},
		[]JobCardRow{{ID: 91, WorkOrderID: 88}},
		ProductionExecutionReadiness{CanStart: true, CanComplete: false},
	)
	if len(actions) == 0 {
		t.Fatal("context actions are empty")
	}
	start := actions[0]
	if start.Key != "startProduction" || start.ActionType != "command" || start.Endpoint != "/api/produce/work-orders/88/start" || start.View != "" || start.Disabled {
		t.Fatalf("start action = %+v", start)
	}
	for _, action := range actions[1:] {
		if action.ActionType != "navigate" || strings.TrimSpace(action.View) == "" || action.Endpoint != "" {
			t.Fatalf("navigation action = %+v", action)
		}
	}
}

func TestPR562ProductionActionsFollowServerStateOnly(t *testing.T) {
	cases := []struct {
		status string
		want   []string
	}{
		{status: "pending", want: []string{"start"}},
		{status: "ready", want: []string{"start"}},
		{status: "running", want: []string{"pause", "complete", "report_exception", "material_call"}},
		{status: "paused", want: []string{"resume", "complete"}},
		{status: "completed", want: nil},
		{status: "cancelled", want: nil},
	}
	for _, tc := range cases {
		task := ProductionTask{JobCardID: 91, RunningItemID: 99, Status: tc.status}
		got := productionAvailableActions(task)
		if strings.Join(got, ",") != strings.Join(tc.want, ",") {
			t.Fatalf("status %s actions = %v, want %v", tc.status, got, tc.want)
		}
	}
}

func TestPR562WorkstationTaskCarriesFrozenRequirementActualsAndInventoryConversion(t *testing.T) {
	task := productionTaskFromJobCard(
		JobCardRow{
			ID: 91, WorkOrderID: 88, Status: "running", Operation: "包装",
			ProcessRequirement: "封口完整并核对标签",
			PlannedInputQty:    6.356, ActualInputQty: 6.4, ActualOutputQty: 6.356,
			ActualMinutes: 42, ActualLossQty: 0.044, ActualLossRate: 0.006875,
			RecordsLoss: true, LossReason: "包装取样", ExceptionReason: "无异常",
		},
		WorkOrderRow{
			ID: 88, RunningItemID: 99, WorkOrderNo: "WO-PR562-001",
			InventoryQtyPerSalesUnit: 0.454, InventoryUnit: "Kg",
		},
	)
	if task.ProcessRequirement != "封口完整并核对标签" ||
		task.PlannedInputQty != 6.356 ||
		task.PlannedInputInventoryQty != 6.356 ||
		task.ActualMinutes != 42 ||
		task.ActualInputQty != 6.4 ||
		task.ActualOutputQty != 6.356 ||
		task.InventoryQtyPerSalesUnit != 0.454 ||
		task.InventoryUnit != "Kg" {
		t.Fatalf("workstation task projection = %+v", task)
	}
}

func TestPR562ReleasedWorkOrderReadinessDoesNotTreatPendingRouteAsPriorOperationBlock(t *testing.T) {
	workOrder := WorkOrderRow{ID: 88, Status: "released"}
	cards := []JobCardRow{
		{ID: 91, WorkOrderID: 88, SequenceNo: 1, Status: "pending", Workstation: "烘焙工位"},
		{ID: 92, WorkOrderID: 88, SequenceNo: 2, Status: "pending", Workstation: "包装工位"},
	}
	wip := ProductionWIPStatus{DataComplete: true, Status: "ok"}

	readiness := buildWorkOrderExecutionReadiness(workOrder, wip, cards, nil)
	if !readiness.CanStart {
		t.Fatalf("released multi-operation work order must be startable, got %+v", readiness)
	}
	if blockingReasonCodesContain(readiness.BlockingReasons, "prior_operation_incomplete") {
		t.Fatalf("ordinary pending route must not be reported as a prior-operation exception: %+v", readiness.BlockingReasons)
	}

	cards[1].ExceptionReason = "前序工序数据待复核"
	readiness = buildWorkOrderExecutionReadiness(workOrder, wip, cards, nil)
	if !readiness.CanStart {
		t.Fatalf("historical prior-operation note must remain non-blocking before work-order start: %+v", readiness)
	}
	var priorReason *ProductionBlockingReason
	for i := range readiness.BlockingReasons {
		if readiness.BlockingReasons[i].Code == "prior_operation_incomplete" {
			priorReason = &readiness.BlockingReasons[i]
			break
		}
	}
	if priorReason == nil || priorReason.Severity == "blocked" {
		t.Fatalf("historical prior-operation note must remain visible as non-blocking context: %+v", readiness.BlockingReasons)
	}
}

func TestPR562WorkOrderCompletionReadinessRequiresEveryJobCardFinished(t *testing.T) {
	wip := ProductionWIPStatus{DataComplete: true, Status: "ok"}
	workOrder := WorkOrderRow{ID: 88, Status: "running"}

	running := []JobCardRow{{
		ID: 91, WorkOrderID: 88, SequenceNo: 1, Status: "running", Workstation: "烘焙工位",
	}}
	if readiness := buildWorkOrderExecutionReadiness(workOrder, wip, running, nil); readiness.CanComplete {
		t.Fatalf("running job card must prevent work-order completion: %+v", readiness)
	}

	workOrder.Status = "partially_completed"
	partiallyFinished := []JobCardRow{
		{ID: 91, WorkOrderID: 88, SequenceNo: 1, Status: "completed", Workstation: "烘焙工位"},
		{ID: 92, WorkOrderID: 88, SequenceNo: 2, Status: "pending", Workstation: "包装工位"},
	}
	if readiness := buildWorkOrderExecutionReadiness(workOrder, wip, partiallyFinished, nil); readiness.CanComplete {
		t.Fatalf("pending job card must prevent work-order completion: %+v", readiness)
	}

	partiallyFinished[1].Status = "completed"
	if readiness := buildWorkOrderExecutionReadiness(workOrder, wip, partiallyFinished, nil); !readiness.CanComplete {
		t.Fatalf("all completed job cards must allow finished receipt: %+v", readiness)
	}

	partiallyFinished[1].Status = "cancelled"
	if readiness := buildWorkOrderExecutionReadiness(workOrder, wip, partiallyFinished, nil); !readiness.CanComplete {
		t.Fatalf("completed or cancelled job cards must allow finished receipt: %+v", readiness)
	}
}

func TestPR562WorkstationReadinessLinkUsesRegisteredViewKey(t *testing.T) {
	readiness := buildWorkOrderExecutionReadiness(
		WorkOrderRow{ID: 88, Status: "running", WorkCenter: "烘焙工位"},
		ProductionWIPStatus{DataComplete: true, Status: "ok"},
		[]JobCardRow{{ID: 91, WorkOrderID: 88, Status: "running", Workstation: "烘焙工位"}},
		nil,
	)
	for _, reason := range readiness.BlockingReasons {
		if reason.Code != "job_cards_incomplete" {
			continue
		}
		if len(reason.RelatedLinks) != 1 || reason.RelatedLinks[0].View != "workstationView" {
			t.Fatalf("workstation readiness link = %+v, want registered workstationView", reason.RelatedLinks)
		}
		return
	}
	t.Fatalf("job_cards_incomplete reason missing: %+v", readiness.BlockingReasons)
}

func TestPR562PlannedInputInventoryQuantityUsesFrozenWorkOrderRatioAndCardShare(t *testing.T) {
	workOrder := WorkOrderRow{
		ID: 88, PlannedG: 6356, PlannedInventoryQty: 6.356,
		InventoryQtyPerSalesUnit: 0.454, InventoryUnit: "Kg",
	}
	card := JobCardRow{
		ID: 91, WorkOrderID: 88, Status: "pending", Workstation: "烘焙工位",
		PlannedInputQty: 3178,
	}
	task := productionTaskFromJobCard(card, workOrder)
	if math.Abs(task.PlannedInputInventoryQty-3.178) > 0.000000001 {
		t.Fatalf("planned input inventory quantity = %.9f, want 3.178 Kg", task.PlannedInputInventoryQty)
	}
	if task.PlannedInputQty != 3178 {
		t.Fatalf("raw legacy planned input must remain unchanged, got %.3f", task.PlannedInputQty)
	}

	legacy := productionTaskFromJobCard(card, WorkOrderRow{ID: 88, PlannedG: 6356})
	if legacy.PlannedInputInventoryQty != card.PlannedInputQty {
		t.Fatalf("legacy task without authoritative inventory conversion must retain raw quantity, got %.3f", legacy.PlannedInputInventoryQty)
	}
	if legacy.InventoryUnit != "g" {
		t.Fatalf("legacy weight task must remain executable with gram display semantics, got unit %q", legacy.InventoryUnit)
	}

	legacyCount := productionTaskFromJobCard(
		JobCardRow{ID: 92, WorkOrderID: 89, Status: "pending", Workstation: "包装工位", PlannedInputQty: 12},
		WorkOrderRow{ID: 89, PlannedUnits: 12},
	)
	if legacyCount.InventoryUnit != "件" || legacyCount.PlannedInputInventoryQty != 12 {
		t.Fatalf("legacy count task must remain executable with count semantics: %+v", legacyCount)
	}
}

func TestPR562RunningTaskIgnoresHistoricalExceptionWhilePausedTaskRemainsBlocked(t *testing.T) {
	if got := productionBlockingReason("running", "历史异常已处理", "烘焙工位", "操作员"); got != "" {
		t.Fatalf("running task must not stay blocked by historical exception, got %q", got)
	}
	if got := productionBlockingReason("paused", "设备检查", "烘焙工位", "操作员"); got != "设备检查" {
		t.Fatalf("paused task must preserve current blocking reason, got %q", got)
	}
	if got := productionBlockingReason("paused", "", "烘焙工位", "操作员"); got != "已暂停" {
		t.Fatalf("paused task without reason must remain blocked, got %q", got)
	}
}
