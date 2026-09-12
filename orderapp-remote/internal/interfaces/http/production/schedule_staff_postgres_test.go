package production

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	manufacture "orderapp/internal/application/manufacturing"
	app "orderapp/internal/application/production"
	manuPG "orderapp/internal/infrastructure/postgres/manufacturing"
	productionPG "orderapp/internal/infrastructure/postgres/production"
	"strings"
	"testing"
)

func TestPR655ScheduleStaffPostgresLifecycle(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, fmt.Sprintf(sql, schema), args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO %s.company_departments(id,name) VALUES(100,'隔离测试部门')`)
	exec(`INSERT INTO %s.company_employees(id,name,phone,department_id,active) VALUES(101,'负责人甲','test-pr655-101',100,true),(102,'协作乙','test-pr655-102',100,true),(103,'负责人丙','test-pr655-103',100,true),(104,'停用丁','test-pr655-104',100,false)`)
	ms := manufacture.NewService(manuPG.NewRepository(pool, schema))
	op, err := ms.SaveManufacturingOperation(ctx, manufacture.SaveManufacturingOperationCommand{Name: "隔离包装工序", Code: "TEST-PACK", Status: "active", EligibleEmployeeIDs: []int64{101, 102, 103}, DefaultEmployeeID: 101, DefaultCollaboratorIDs: []int64{102}, Actor: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if !op.StaffingReady || len(op.DefaultCollaboratorIDs) != 1 {
		t.Fatalf("staff configuration lost %+v", op)
	}
	exec(`INSERT INTO %s.work_orders(id,work_order_no,status,product_name,inventory_unit,planned_inventory_qty,planned_g,planned_output_g,sales_spec_count,order_nos) VALUES(201,'WO-TEST-A','released','测试商品','kg',2,2000,2000,2,'SO-TEST-A'),(202,'WO-TEST-B','released','测试商品','kg',2,2000,2000,2,'SO-TEST-B')`)
	exec(`INSERT INTO %s.job_cards(id,work_order_id,operation_id,operation,workstation,sequence_no,status,planned_input_qty) VALUES(301,201,$1,'包装','包装台',1,'pending',2000),(302,202,$1,'包装','另一工位',1,'pending',2000),(303,202,$1,'包装','包装台',1,'completed',2000)`, op.ID)
	svc := app.NewService(productionPG.NewRepository(pool, schema))
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error { c.Set("actor", "test"); return next(c) }
	})
	registerProductionScheduleAPI(e, svc)
	post := func(path string, body any, want int) app.ScheduleBatchResult {
		t.Helper()
		data, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(data)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("%s: got %d want %d %s", path, rec.Code, want, rec.Body.String())
		}
		var out app.ScheduleBatchResult
		json.Unmarshal(rec.Body.Bytes(), &out)
		return out
	}
	item := map[string]any{"work_order_id": 201, "job_card_id": 301, "expected_version": 0, "assigned_employee_id": 101, "collaborator_employee_ids": []int64{102}, "planned_start_at": "2026-09-12T09:00", "planned_end_at": "2026-09-12T11:00", "shift_code": "早班", "note": "保留备注"}
	body := map[string]any{"request_id": "first", "items": []any{item}}
	preview := post("/api/production-schedule/preview", body, 200)
	var before int
	pool.QueryRow(ctx, fmt.Sprintf(`SELECT schedule_version FROM %s.job_cards WHERE id=301`, schema)).Scan(&before)
	if before != 0 || preview.Saved {
		t.Fatal("preview wrote task")
	}
	body["preview_token"] = preview.PreviewToken
	post("/api/production-schedule/batch", body, 200)
	replay := post("/api/production-schedule/batch", body, 200)
	if !replay.Replayed {
		t.Fatal("retry not replayed")
	}
	item2 := map[string]any{"work_order_id": 202, "job_card_id": 302, "expected_version": 0, "assigned_employee_id": 102, "planned_start_at": "2026-09-12T10:00", "planned_end_at": "2026-09-12T12:00"}
	body2 := map[string]any{"request_id": "second", "items": []any{item2}}
	overlap := post("/api/production-schedule/preview", body2, 200)
	if len(overlap.Conflicts) != 1 || overlap.Conflicts[0].EmployeeID != 102 {
		t.Fatalf("helper overlap missing %+v", overlap)
	}
	post("/api/production-schedule/batch", body2, 409)
	body2["preview_token"] = overlap.PreviewToken
	post("/api/production-schedule/batch", body2, 200)
	// A personnel-only patch preserves scheduling fields and is immediately visible through the shared read model.
	change := map[string]any{"request_id": "people-only", "items": []any{map[string]any{"work_order_id": 201, "job_card_id": 301, "expected_version": 1, "assigned_employee_id": 103, "collaborator_employee_ids": []int64{}}}}
	changed := post("/api/production-schedule/batch", change, 200)
	row := changed.Rows[0]
	if row.PlannedStartAt != "2026-09-12 09:00" || row.ShiftCode != "早班" || row.SchedulingNote != "保留备注" || row.AssignedEmployeeID != 103 {
		t.Fatalf("personnel cleared scheduling %+v", row)
	}
	cards, err := svc.ListJobCards(ctx, app.JobCardQuery{WorkOrderID: 201, Limit: 20})
	if err != nil || len(cards) != 1 || cards[0].AssignedEmployeeID != 103 || cards[0].ScheduleVersion != 2 {
		t.Fatalf("shared read model mismatch %+v %v", cards, err)
	}
	stale := map[string]any{"request_id": "stale", "items": []any{item}}
	post("/api/production-schedule/batch", stale, 409)
	// Entire batch rolls back if a later task is historical or staff are invalid.
	atomic := map[string]any{"request_id": "atomic", "items": []any{map[string]any{"work_order_id": 201, "job_card_id": 301, "expected_version": 2, "assigned_employee_id": 101}, map[string]any{"work_order_id": 202, "job_card_id": 303, "expected_version": 0, "assigned_employee_id": 101}}}
	post("/api/production-schedule/batch", atomic, 409)
	invalid := map[string]any{"request_id": "invalid", "items": []any{map[string]any{"work_order_id": 201, "job_card_id": 301, "expected_version": 2, "assigned_employee_id": 104}}}
	post("/api/production-schedule/batch", invalid, 400)
	var lead int64
	pool.QueryRow(ctx, fmt.Sprintf(`SELECT assigned_employee_id FROM %s.job_cards WHERE id=301`, schema)).Scan(&lead)
	if lead != 103 {
		t.Fatal("failed batch partially wrote")
	}
	req := httptest.NewRequest(http.MethodGet, "/api/production-schedule?scope=scheduled&page=1&limit=1&from=2026-09-12&to=2026-09-12", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var board app.ScheduleBoardResult
	if err = json.Unmarshal(rec.Body.Bytes(), &board); err != nil || board.Total != 2 || len(board.Rows) != 1 || board.TotalPages != 2 {
		t.Fatalf("server pagination %+v %s", board, rec.Body.String())
	}
	if len(board.Load) != 2 || board.Load[0].AvailableMinutes != nil {
		t.Fatalf("missing capacity must not look empty %+v", board.Load)
	}
	var audits int
	pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.audit_logs WHERE entity_type='job_card' AND action='schedule'`, schema)).Scan(&audits)
	if audits != 3 {
		t.Fatalf("writes audited %d, expected 3", audits)
	}
	// Conflict confirmation is invalidated when an overlapping task changes after preview.
	conflictChange := map[string]any{"request_id": "changed-conflict", "items": []any{map[string]any{"work_order_id": 201, "job_card_id": 301, "expected_version": 2, "assigned_employee_id": 102}}}
	oldPreview := post("/api/production-schedule/preview", conflictChange, 200)
	post("/api/production-schedule/batch", map[string]any{"request_id": "extend-peer", "items": []any{map[string]any{"work_order_id": 202, "job_card_id": 302, "expected_version": 1, "planned_end_at": "2026-09-12T12:30"}}}, 200)
	conflictChange["preview_token"] = oldPreview.PreviewToken
	post("/api/production-schedule/batch", conflictChange, 409)
	newPreview := post("/api/production-schedule/preview", conflictChange, 200)
	if oldPreview.PreviewToken == newPreview.PreviewToken {
		t.Fatal("peer conflict changes must invalidate confirmation")
	}
	conflictChange["preview_token"] = newPreview.PreviewToken
	post("/api/production-schedule/batch", conflictChange, 200)
	// Candidate validation covers active but unqualified employees as well as duplicates.
	exec(`INSERT INTO %s.company_employees(id,name,phone,department_id,active) VALUES(105,'无资格员工','test-pr655-105',100,true)`)
	post("/api/production-schedule/batch", map[string]any{"request_id": "unqualified", "items": []any{map[string]any{"work_order_id": 201, "job_card_id": 301, "expected_version": 3, "assigned_employee_id": 105}}}, 400)
	post("/api/production-schedule/batch", map[string]any{"request_id": "duplicate", "items": []any{map[string]any{"work_order_id": 201, "job_card_id": 301, "expected_version": 3, "collaborator_employee_ids": []int64{102}}}}, 400)
	_, err = ms.SaveManufacturingOperation(ctx, manufacture.SaveManufacturingOperationCommand{ID: op.ID, Name: op.Name, Code: op.Code, Status: "active", EligibleEmployeeIDs: []int64{101, 102, 103}, DefaultEmployeeID: 103, Actor: "test"})
	if err != nil {
		t.Fatal(err)
	}
	pool.QueryRow(ctx, fmt.Sprintf(`SELECT assigned_employee_id FROM %s.job_cards WHERE id=301`, schema)).Scan(&lead)
	if lead != 102 {
		t.Fatal("operation defaults changed a saved assignment")
	}
	// Missing station can be filled compatibly without changing task size or creating batches.
	exec(`INSERT INTO %s.manufacturing_workstations(id,code,name,status) VALUES(501,'QA-NEW','补充工位','active')`)
	exec(`INSERT INTO %s.manufacturing_workstation_operations(workstation_id,operation_id) VALUES(501,$1)`, op.ID)
	exec(`INSERT INTO %s.manufacturing_workstation_capacities(id,workstation_id,code,name,status,batch_size_qty,batch_size_unit) VALUES(501,501,'QA-2KG','2kg 批次','active',2,'kg')`)
	exec(`INSERT INTO %s.job_cards(id,work_order_id,operation_id,operation,sequence_no,status,planned_input_qty) VALUES(304,202,$1,'包装',2,'pending',1000)`, op.ID)
	filled := post("/api/production-schedule/batch", map[string]any{"request_id": "fill-station", "items": []any{map[string]any{"work_order_id": 202, "job_card_id": 304, "expected_version": 0, "assigned_employee_id": 101, "workstation_capacity_id": 501}}}, 200)
	if filled.Rows[0].WorkstationID != 501 || filled.Rows[0].PlannedInputQty != 1000 {
		t.Fatal("station filling changed batch")
	}
	// Compatibility single-person assignment is a patch too: original times and shift survive.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/production-schedule/assign", strings.NewReader(`{"work_order_id":201,"job_card_id":301,"expected_version":3,"assigned_employee_id":103,"request_id":"single-compat"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	e.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	cards, err = svc.ListJobCards(ctx, app.JobCardQuery{WorkOrderID: 201, Limit: 20})
	if err != nil || cards[0].PlannedStartAt != "2026-09-12 09:00" || cards[0].ShiftCode != "早班" {
		t.Fatalf("single patch lost time %+v %v", cards, err)
	}
	// Simultaneous retries serialize to one write and one replay.
	v := int64(1)
	leadID := int64(103)
	cmd := app.ScheduleBatchCommand{RequestID: "concurrent", Operator: "test", Items: []app.ScheduleTaskPatch{{WorkOrderID: 202, JobCardID: 304, ExpectedVersion: &v, AssignedEmployeeID: &leadID}}}
	type answer struct {
		result app.ScheduleBatchResult
		err    error
	}
	done := make(chan answer, 2)
	for i := 0; i < 2; i++ {
		go func() { result, err := svc.ScheduleBatch(ctx, cmd, false); done <- answer{result, err} }()
	}
	replayCount := 0
	for i := 0; i < 2; i++ {
		got := <-done
		if got.err != nil {
			t.Fatal(got.err)
		}
		if got.result.Replayed {
			replayCount++
		}
	}
	if replayCount != 1 {
		t.Fatalf("concurrent retries produced %d replays", replayCount)
	}

}
