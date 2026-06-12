package production

import (
	"context"
	"net/http"
	"net/http/httptest"
	productionapp "orderapp/internal/application/production"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func (r *workOrderAPIRepo) SaveScheduleAssignment(ctx context.Context, cmd productionapp.ScheduleAssignmentCommand) (productionapp.ScheduleAssignmentResult, error) {
	r.scheduleAssignment = cmd
	return productionapp.ScheduleAssignmentResult{
		WorkOrder: productionapp.WorkOrderRow{ID: cmd.WorkOrderID, WorkOrderNo: "WO-PR480-001", PlannedStartAt: cmd.PlannedStartAt, PlannedEndAt: cmd.PlannedEndAt, ShiftCode: cmd.ShiftCode, AssignedTo: cmd.AssignedTo, Priority: cmd.Priority},
		JobCard:   productionapp.JobCardRow{ID: cmd.JobCardID, WorkOrderID: cmd.WorkOrderID, PlannedStartAt: cmd.PlannedStartAt, PlannedEndAt: cmd.PlannedEndAt, ShiftCode: cmd.ShiftCode, AssignedTo: cmd.AssignedTo, Priority: cmd.Priority},
		Conflicts: []productionapp.ScheduleConflict{{Severity: "warning", Message: "产能冲突"}},
	}, nil
}

func (r *workOrderAPIRepo) SaveCapacityCalendar(ctx context.Context, cmd productionapp.CapacityCalendarCommand) (productionapp.CapacityCalendarRow, error) {
	r.capacityCalendar = cmd
	return productionapp.CapacityCalendarRow{ID: 7, WorkCenter: cmd.WorkCenter, WorkDate: cmd.WorkDate, ShiftCode: cmd.ShiftCode, AvailableMinutes: cmd.AvailableMinutes, DowntimeMinutes: cmd.DowntimeMinutes, Note: cmd.Note}, nil
}

func (r *workOrderAPIRepo) ScheduleBoard(ctx context.Context, query productionapp.ScheduleBoardQuery) (productionapp.ScheduleBoardResult, error) {
	r.scheduleQuery = query
	return productionapp.ScheduleBoardResult{
		WorkOrders: []productionapp.WorkOrderRow{{ID: 88, WorkOrderNo: "WO-PR480-001", WorkCenter: query.WorkCenter, PlannedStartAt: query.From + " 09:00", PlannedEndAt: query.From + " 11:00"}},
		JobCards:   []productionapp.JobCardRow{{ID: 91, WorkOrderID: 88, Operation: "印刷", Workstation: query.WorkCenter}},
		Capacity:   []productionapp.CapacityCalendarRow{{ID: 7, WorkCenter: query.WorkCenter, WorkDate: query.From, ShiftCode: "早班", AvailableMinutes: 480, DowntimeMinutes: 30}},
		Conflicts:  []productionapp.ScheduleConflict{{Severity: "warning", Message: "产能冲突"}},
	}, nil
}

func TestManufacturingPhase3ScheduleCapacityAPIs(t *testing.T) {
	e := echo.New()
	repo := &workOrderAPIRepo{}
	RegisterRoutes(e, Dependencies{Production: productionapp.NewService(repo)})

	req := httptest.NewRequest(http.MethodPost, "/api/production-schedule/assign", strings.NewReader(`{"work_order_id":88,"job_card_id":91,"work_center":"印刷线","planned_start_at":"2026-06-13 09:00","planned_end_at":"2026-06-13 11:30","shift_code":"早班","assigned_to":"王师傅","priority":2,"note":"插单优先"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/production-schedule/assign status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.scheduleAssignment.WorkOrderID != 88 || repo.scheduleAssignment.JobCardID != 91 || repo.scheduleAssignment.Operator == "" {
		t.Fatalf("schedule assignment command = %+v", repo.scheduleAssignment)
	}
	for _, want := range []string{`"work_order"`, `"job_card"`, `"conflicts"`, `"planned_start_at":"2026-06-13 09:00"`, `"shift_code":"早班"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("schedule assignment response missing %s: %s", want, rec.Body.String())
		}
	}

	req = httptest.NewRequest(http.MethodPost, "/api/production-capacity-calendar", strings.NewReader(`{"work_center":"印刷线","work_date":"2026-06-13","shift_code":"早班","available_minutes":480,"downtime_minutes":30,"note":"设备保养"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/production-capacity-calendar status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.capacityCalendar.WorkCenter != "印刷线" || repo.capacityCalendar.AvailableMinutes != 480 {
		t.Fatalf("capacity calendar command = %+v", repo.capacityCalendar)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/production-schedule?from=2026-06-13&to=2026-06-13&work_center=%E5%8D%B0%E5%88%B7%E7%BA%BF&limit=50", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/production-schedule status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.scheduleQuery.From != "2026-06-13" || repo.scheduleQuery.To != "2026-06-13" || repo.scheduleQuery.WorkCenter != "印刷线" || repo.scheduleQuery.Limit != 50 {
		t.Fatalf("schedule query = %+v", repo.scheduleQuery)
	}
	for _, want := range []string{`"work_orders"`, `"job_cards"`, `"capacity"`, `"conflicts"`, `"WO-PR480-001"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("schedule board response missing %s: %s", want, rec.Body.String())
		}
	}
}

func TestManufacturingPhase3MRPSuggestionAPI(t *testing.T) {
	e := echo.New()
	repo := &workOrderAPIRepo{}
	RegisterRoutes(e, Dependencies{Production: productionapp.NewService(repo)})

	req := httptest.NewRequest(http.MethodGet, "/api/mrp/suggestions?from=2026-06-13&to=2026-06-14&status=released&work_center=%E7%83%98%E7%84%99%E6%9C%BA&material_id=10&limit=50", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/mrp/suggestions status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.mrpQuery.From != "2026-06-13" || repo.mrpQuery.To != "2026-06-14" || repo.mrpQuery.Status != "released" || repo.mrpQuery.WorkCenter != "烘焙机" || repo.mrpQuery.MaterialID != 10 || repo.mrpQuery.Limit != 50 {
		t.Fatalf("MRP query = %+v", repo.mrpQuery)
	}
	for _, want := range []string{
		`"rows"`,
		`"material_name":"孟连水洗5T批次"`,
		`"wip_transfer_suggestion_g":30000`,
		`"purchase_suggestion_g":25000`,
		`"suggestion_type":"purchase_suggestion"`,
		`"source_work_orders":"WO-PR482-001,WO-PR482-002"`,
		`"purchase_suggestion_g":25000`,
		`"transfer_suggestion_g":30000`,
	} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("MRP suggestion response missing %s: %s", want, rec.Body.String())
		}
	}
}

func TestManufacturingPhase3ProductionTraceAnalyticsAPI(t *testing.T) {
	e := echo.New()
	repo := &workOrderAPIRepo{}
	RegisterRoutes(e, Dependencies{Production: productionapp.NewService(repo)})

	req := httptest.NewRequest(http.MethodGet, "/api/production-trace/analytics?work_order_id=88&batch_id=BATCH-WO-88&limit=20", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/production-trace/analytics status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.traceQuery.WorkOrderID != 88 || repo.traceQuery.BatchID != "BATCH-WO-88" || repo.traceQuery.Limit != 20 {
		t.Fatalf("trace query = %+v", repo.traceQuery)
	}
	for _, want := range []string{
		`"trace_links"`,
		`"cost_variance"`,
		`"abnormal_losses"`,
		`"entry_no":"SE-0000000007"`,
		`"variance":18`,
		`"abnormal_loss_count":1`,
	} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("trace analytics response missing %s: %s", want, rec.Body.String())
		}
	}
}
