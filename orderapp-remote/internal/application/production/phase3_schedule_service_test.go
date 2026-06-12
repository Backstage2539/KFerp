package production

import (
	"context"
	"testing"
)

type phase3ScheduleRepo struct {
	fakeFlowRepo
	assignment ScheduleAssignmentCommand
	capacity   CapacityCalendarCommand
	query      ScheduleBoardQuery
	mrpQuery   MRPSuggestionQuery
	traceQuery ProductionTraceAnalyticsQuery
}

func (r *phase3ScheduleRepo) SaveScheduleAssignment(ctx context.Context, cmd ScheduleAssignmentCommand) (ScheduleAssignmentResult, error) {
	r.assignment = cmd
	return ScheduleAssignmentResult{
		WorkOrder: WorkOrderRow{
			ID:             cmd.WorkOrderID,
			WorkOrderNo:    "WO-PR480-001",
			Status:         "released",
			PlannedStartAt: cmd.PlannedStartAt,
			PlannedEndAt:   cmd.PlannedEndAt,
			ShiftCode:      cmd.ShiftCode,
			AssignedTo:     cmd.AssignedTo,
			Priority:       cmd.Priority,
			SchedulingNote: cmd.Note,
		},
		JobCard: JobCardRow{
			ID:             cmd.JobCardID,
			WorkOrderID:    cmd.WorkOrderID,
			Status:         "pending",
			Workstation:    cmd.WorkCenter,
			PlannedStartAt: cmd.PlannedStartAt,
			PlannedEndAt:   cmd.PlannedEndAt,
			ShiftCode:      cmd.ShiftCode,
			AssignedTo:     cmd.AssignedTo,
			Priority:       cmd.Priority,
		},
		Conflicts: []ScheduleConflict{{Severity: "warning", Message: "印刷线 早班负载超过可用产能"}},
	}, nil
}

func (r *phase3ScheduleRepo) SaveCapacityCalendar(ctx context.Context, cmd CapacityCalendarCommand) (CapacityCalendarRow, error) {
	r.capacity = cmd
	return CapacityCalendarRow{
		ID:               7,
		WorkCenter:       cmd.WorkCenter,
		WorkDate:         cmd.WorkDate,
		ShiftCode:        cmd.ShiftCode,
		AvailableMinutes: cmd.AvailableMinutes,
		DowntimeMinutes:  cmd.DowntimeMinutes,
		Note:             cmd.Note,
	}, nil
}

func (r *phase3ScheduleRepo) ScheduleBoard(ctx context.Context, query ScheduleBoardQuery) (ScheduleBoardResult, error) {
	r.query = query
	return ScheduleBoardResult{
		WorkOrders: []WorkOrderRow{{ID: 88, WorkOrderNo: "WO-PR480-001", WorkCenter: query.WorkCenter, PlannedStartAt: query.From + " 09:00", PlannedEndAt: query.From + " 11:00"}},
		JobCards:   []JobCardRow{{ID: 91, WorkOrderID: 88, Operation: "印刷", Workstation: query.WorkCenter, PlannedStartAt: query.From + " 09:00", PlannedEndAt: query.From + " 10:00"}},
		Capacity:   []CapacityCalendarRow{{WorkCenter: query.WorkCenter, WorkDate: query.From, ShiftCode: "早班", AvailableMinutes: 480, DowntimeMinutes: 30}},
		Conflicts:  []ScheduleConflict{{Severity: "warning", Message: "印刷线 早班负载超过可用产能"}},
	}, nil
}

func (r *phase3ScheduleRepo) MRPSuggestions(ctx context.Context, query MRPSuggestionQuery) (MRPSuggestionResult, error) {
	r.mrpQuery = query
	return MRPSuggestionResult{
		Rows: []MRPSuggestionRow{{
			MaterialID:             query.MaterialID,
			MaterialName:           "孟连水洗5T批次",
			Unit:                   "g",
			RequiredG:              60000,
			WIPG:                   10000,
			RawG:                   30000,
			ReservedG:              5000,
			AvailableG:             5000,
			WIPTransferSuggestionG: 30000,
			ShortageG:              25000,
			PurchaseSuggestionG:    25000,
			WorkOrderCount:         2,
			SourceWorkOrders:       "WO-PR482-001,WO-PR482-002",
			SuggestionType:         "purchase_suggestion",
		}},
		PurchaseSuggestionG: 25000,
		TransferSuggestionG: 30000,
	}, nil
}

func (r *phase3ScheduleRepo) ProductionTraceAnalytics(ctx context.Context, query ProductionTraceAnalyticsQuery) (ProductionTraceAnalyticsResult, error) {
	r.traceQuery = query
	return ProductionTraceAnalyticsResult{
		TraceLinks: []ProductionTraceLinkRow{{
			WorkOrderID:   88,
			WorkOrderNo:   "WO-PR484-001",
			RunningItemID: 99,
			BatchID:       query.BatchID,
			JobCardID:     91,
			Operation:     "烘焙",
			StockEntryID:  7,
			EntryNo:       "SE-0000000007",
			EntryType:     "finished_receipt",
			MaterialID:    10,
			MaterialName:  "孟连水洗5T批次",
			QtyG:          45400,
		}},
		CostVariance: []ProductionCostVarianceRow{{
			WorkOrderID:  88,
			WorkOrderNo:  "WO-PR484-001",
			ProductName:  "工厂量单商品",
			PlannedCost:  100,
			ActualCost:   118,
			Variance:     18,
			VarianceRate: 0.18,
		}},
		AbnormalLosses: []ProductionAbnormalLossRow{{
			JobCardID:      91,
			WorkOrderID:    88,
			WorkOrderNo:    "WO-PR484-001",
			Operation:      "烘焙",
			ActualLossQty:  1200,
			ActualLossRate: 0.12,
			Severity:       "warning",
		}},
		TotalVariance:     18,
		AbnormalLossCount: 1,
	}, nil
}

func TestServiceOwnsManufacturingPhase3ScheduleCapacity(t *testing.T) {
	repo := &phase3ScheduleRepo{}
	svc := NewService(repo)

	assigned, err := svc.SaveScheduleAssignment(context.Background(), ScheduleAssignmentCommand{
		WorkOrderID:    88,
		JobCardID:      91,
		WorkCenter:     "  印刷线 ",
		PlannedStartAt: " 2026-06-13 09:00 ",
		PlannedEndAt:   "2026-06-13 11:30",
		ShiftCode:      " 早班 ",
		AssignedTo:     "  王师傅 ",
		Priority:       2,
		Note:           "  插单优先 ",
		Operator:       "  planner ",
	})
	if err != nil {
		t.Fatalf("SaveScheduleAssignment() error = %v", err)
	}
	if repo.assignment.WorkCenter != "印刷线" || repo.assignment.Operator != "planner" || repo.assignment.AssignedTo != "王师傅" || repo.assignment.Note != "插单优先" {
		t.Fatalf("assignment was not normalized: %+v", repo.assignment)
	}
	if assigned.WorkOrder.PlannedStartAt != "2026-06-13 09:00" || assigned.JobCard.Workstation != "印刷线" || len(assigned.Conflicts) != 1 {
		t.Fatalf("assignment result = %+v", assigned)
	}

	capacity, err := svc.SaveCapacityCalendar(context.Background(), CapacityCalendarCommand{
		WorkCenter:       "印刷线",
		WorkDate:         "2026-06-13",
		ShiftCode:        "早班",
		AvailableMinutes: 480,
		DowntimeMinutes:  30,
		Note:             "设备保养",
		Operator:         "planner",
	})
	if err != nil {
		t.Fatalf("SaveCapacityCalendar() error = %v", err)
	}
	if capacity.WorkCenter != "印刷线" || capacity.AvailableMinutes != 480 || capacity.DowntimeMinutes != 30 {
		t.Fatalf("capacity result = %+v", capacity)
	}

	board, err := svc.ScheduleBoard(context.Background(), ScheduleBoardQuery{From: "2026-06-13", To: "2026-06-13", WorkCenter: " 印刷线 ", Limit: 0})
	if err != nil {
		t.Fatalf("ScheduleBoard() error = %v", err)
	}
	if repo.query.WorkCenter != "印刷线" || repo.query.Limit != 200 {
		t.Fatalf("schedule query = %+v", repo.query)
	}
	if len(board.WorkOrders) != 1 || len(board.JobCards) != 1 || len(board.Capacity) != 1 || board.Conflicts[0].Severity != "warning" {
		t.Fatalf("schedule board = %+v", board)
	}
}

func TestServiceOwnsManufacturingPhase3MRPSuggestions(t *testing.T) {
	repo := &phase3ScheduleRepo{}
	svc := NewService(repo)

	res, err := svc.MRPSuggestions(context.Background(), MRPSuggestionQuery{
		From:       " 2026-06-13 ",
		To:         "2026-06-14",
		Status:     " released ",
		WorkCenter: " 烘焙机 ",
		MaterialID: 10,
		Limit:      0,
	})
	if err != nil {
		t.Fatalf("MRPSuggestions() error = %v", err)
	}
	if repo.mrpQuery.From != "2026-06-13" || repo.mrpQuery.Status != "released" || repo.mrpQuery.WorkCenter != "烘焙机" || repo.mrpQuery.Limit != 50 {
		t.Fatalf("MRP query was not normalized: %+v", repo.mrpQuery)
	}
	if len(res.Rows) != 1 || res.Rows[0].SuggestionType != "purchase_suggestion" || res.PurchaseSuggestionG != 25000 || res.TransferSuggestionG != 30000 {
		t.Fatalf("MRP suggestions = %+v", res)
	}
}

func TestServiceOwnsManufacturingPhase3TraceAnalytics(t *testing.T) {
	repo := &phase3ScheduleRepo{}
	svc := NewService(repo)
	res, err := svc.ProductionTraceAnalytics(context.Background(), ProductionTraceAnalyticsQuery{
		WorkOrderID: 88,
		BatchID:     " BATCH-WO-88 ",
		Limit:       0,
	})
	if err != nil {
		t.Fatalf("ProductionTraceAnalytics() error = %v", err)
	}
	if repo.traceQuery.BatchID != "BATCH-WO-88" || repo.traceQuery.Limit != 50 || repo.traceQuery.WorkOrderID != 88 {
		t.Fatalf("trace query was not normalized: %+v", repo.traceQuery)
	}
	if len(res.TraceLinks) != 1 || len(res.CostVariance) != 1 || len(res.AbnormalLosses) != 1 || res.TotalVariance != 18 || res.AbnormalLossCount != 1 {
		t.Fatalf("trace analytics = %+v", res)
	}
}

func TestServiceRejectsInvalidManufacturingPhase3ScheduleCommands(t *testing.T) {
	svc := NewService(&phase3ScheduleRepo{})
	if _, err := svc.SaveScheduleAssignment(context.Background(), ScheduleAssignmentCommand{WorkOrderID: 0, Operator: "planner"}); err == nil {
		t.Fatal("SaveScheduleAssignment() error = nil, want work_order_id validation")
	}
	if _, err := svc.SaveScheduleAssignment(context.Background(), ScheduleAssignmentCommand{WorkOrderID: 88, PlannedStartAt: "2026-06-13 12:00", PlannedEndAt: "2026-06-13 09:00", Operator: "planner"}); err == nil {
		t.Fatal("SaveScheduleAssignment() error = nil, want time range validation")
	}
	if _, err := svc.SaveCapacityCalendar(context.Background(), CapacityCalendarCommand{WorkCenter: "", WorkDate: "2026-06-13", AvailableMinutes: 480, Operator: "planner"}); err == nil {
		t.Fatal("SaveCapacityCalendar() error = nil, want work_center validation")
	}
	if _, err := svc.SaveCapacityCalendar(context.Background(), CapacityCalendarCommand{WorkCenter: "印刷线", WorkDate: "2026-06-13", AvailableMinutes: -1, Operator: "planner"}); err == nil {
		t.Fatal("SaveCapacityCalendar() error = nil, want available minutes validation")
	}
	if _, err := svc.MRPSuggestions(context.Background(), MRPSuggestionQuery{From: "2026-06-14", To: "2026-06-13"}); err == nil {
		t.Fatal("MRPSuggestions() error = nil, want date range validation")
	}
	if _, err := svc.ProductionTraceAnalytics(context.Background(), ProductionTraceAnalyticsQuery{WorkOrderID: -1}); err == nil {
		t.Fatal("ProductionTraceAnalytics() error = nil, want work_order_id validation")
	}
}
