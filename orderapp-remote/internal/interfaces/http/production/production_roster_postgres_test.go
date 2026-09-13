package production

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	manufacturingapp "orderapp/internal/application/manufacturing"
	productionapp "orderapp/internal/application/production"
	manufacturingpg "orderapp/internal/infrastructure/postgres/manufacturing"
	productionpg "orderapp/internal/infrastructure/postgres/production"

	"github.com/labstack/echo/v4"
)

func TestPR657ProductionRosterPostgresLifecycle(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.company_departments(id,name,active) VALUES(657,'PR657隔离生产部',true);
		INSERT INTO %s.company_employees(id,name,phone,department_id,active,account_type) VALUES
			(6571,'排班主负责人','pr657-1',657,true,'internal_employee'),
			(6572,'排班替补','pr657-2',657,true,'internal_employee');
	`, schema, schema))
	manufacturingSvc := manufacturingapp.NewService(manufacturingpg.NewRepository(pool, schema))
	station, err := manufacturingSvc.SaveManufacturingWorkstation(ctx, manufacturingapp.SaveManufacturingWorkstationCommand{
		Code: "PR657-WS", Name: "PR657隔离工位", Status: "active", PrimaryEmployeeID: 6571, BackupEmployeeIDs: []int64{6572}, Actor: "PR657测试",
	})
	if err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.work_orders(id,work_order_no,status,product_name,planned_g,planned_output_g,inventory_unit,planned_inventory_qty,work_center)
		VALUES(65701,'WO-PR657','released','排班测试商品',4000,4000,'kg',4,'PR657隔离工位');
		INSERT INTO %s.job_cards(id,work_order_id,sequence_no,operation,workstation_id,workstation,status,planned_input_qty)
		VALUES(65702,65701,1,'排班测试工序',%d,'PR657隔离工位','pending',4000);
	`, schema, schema, station.ID))

	service := productionapp.NewService(productionpg.NewRepository(pool, schema))
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("actor", "PR657测试")
			c.Set("employee_id", int64(6572))
			return next(c)
		}
	})
	registerProductionRosterAPI(e, service)
	registerProductionWorkstationAPI(e, service)

	location, _ := time.LoadLocation("Asia/Shanghai")
	weekStart, days, err := productionapp.NormalizeProductionRosterWeek(time.Now().In(location).Format("2006-01-02"), location)
	if err != nil {
		t.Fatal(err)
	}
	entries := make([]productionapp.ProductionAttendanceEntry, 0, 14)
	for _, day := range days {
		entries = append(entries,
			productionapp.ProductionAttendanceEntry{EmployeeID: 6571, WorkDate: day, Status: "off"},
			productionapp.ProductionAttendanceEntry{EmployeeID: 6572, WorkDate: day, Status: "working"},
		)
	}
	body := productionapp.SaveProductionRosterCommand{WeekStart: weekStart, ExpectedVersion: 0, Entries: entries, RequestID: "pr657-save"}
	post := func(path string, value any) *httptest.ResponseRecorder {
		data, _ := json.Marshal(value)
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(data)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}
	preview := post("/api/production-roster/preview", body)
	if preview.Code != http.StatusOK || !strings.Contains(preview.Body.String(), `"employee_id":6572`) {
		t.Fatalf("preview %d %s", preview.Code, preview.Body.String())
	}
	var persisted int
	pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.production_employee_attendance`, schema)).Scan(&persisted)
	if persisted != 0 {
		t.Fatal("preview wrote attendance")
	}
	saved := post("/api/production-roster/save", body)
	if saved.Code != http.StatusOK || !strings.Contains(saved.Body.String(), `"saved":true`) {
		t.Fatalf("save %d %s", saved.Code, saved.Body.String())
	}
	replay := post("/api/production-roster/save", body)
	if replay.Code != http.StatusOK || !strings.Contains(replay.Body.String(), `"replayed":true`) {
		t.Fatalf("replay %d %s", replay.Code, replay.Body.String())
	}

	todayReq := httptest.NewRequest(http.MethodGet, "/api/production-roster/today", nil)
	todayRec := httptest.NewRecorder()
	e.ServeHTTP(todayRec, todayReq)
	if todayRec.Code != http.StatusOK || !strings.Contains(todayRec.Body.String(), `"workstation":"PR657隔离工位"`) {
		t.Fatalf("today %d %s", todayRec.Code, todayRec.Body.String())
	}
	overviewReq := httptest.NewRequest(http.MethodGet, "/api/production/workstation-overview?scope=mine", nil)
	overviewRec := httptest.NewRecorder()
	e.ServeHTTP(overviewRec, overviewReq)
	if overviewRec.Code != http.StatusOK || !strings.Contains(overviewRec.Body.String(), `"assigned_employee_id":6572`) {
		t.Fatalf("overview %d %s", overviewRec.Code, overviewRec.Body.String())
	}

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.work_orders SET status='running' WHERE id=65701; UPDATE %s.job_cards SET status='running',assigned_employee_id=6571,assigned_to='排班主负责人' WHERE id=65702`, schema, schema))
	handover := post("/api/production-roster/handover", productionapp.HandoverWorkstationCommand{WorkstationID: station.ID, WorkDate: time.Now().In(location).Format("2006-01-02"), EmployeeID: 6572, ExpectedVersion: 1, RequestID: "pr657-handover"})
	if handover.Code != http.StatusOK || !strings.Contains(handover.Body.String(), `65702`) {
		t.Fatalf("handover %d %s", handover.Code, handover.Body.String())
	}
	var assigned int64
	pool.QueryRow(ctx, fmt.Sprintf(`SELECT assigned_employee_id FROM %s.job_cards WHERE id=65702`, schema)).Scan(&assigned)
	if assigned != 6572 {
		t.Fatalf("handover assigned employee=%d", assigned)
	}
	var previousAssignments string
	pool.QueryRow(ctx, fmt.Sprintf(`SELECT previous_assignments_json::text FROM %s.production_workstation_handovers WHERE request_id='pr657-handover'`, schema)).Scan(&previousAssignments)
	if !strings.Contains(previousAssignments, `"job_card_id": 65702`) || !strings.Contains(previousAssignments, `"employee_id": 6571`) {
		t.Fatalf("handover previous assignments=%s", previousAssignments)
	}
	var audits int
	pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.audit_logs WHERE entity_type IN ('production_roster','production_workstation_handover')`, schema)).Scan(&audits)
	if audits != 2 {
		t.Fatalf("audit count=%d", audits)
	}
}

func TestPR658RosterReliefAndManualReplacementAPI(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %s.company_departments(id,name,active) VALUES(658,'替补测试',true);
 INSERT INTO %s.company_employees(id,name,phone,department_id,active,account_type) VALUES
 (6581,'A 主负责人','6581',658,true,'internal_employee'),(6582,'B 替补','6582',658,true,'internal_employee'),(6583,'C 临时员工','6583',658,true,'internal_employee');`, schema, schema))
	m := manufacturingapp.NewService(manufacturingpg.NewRepository(pool, schema))
	stations := []int64{}
	for _, name := range []string{"替补工位一", "替补工位二"} {
		station, err := m.SaveManufacturingWorkstation(ctx, manufacturingapp.SaveManufacturingWorkstationCommand{Name: name, Code: name, Status: "active", PrimaryEmployeeID: 6581, BackupEmployeeIDs: []int64{6582}, Actor: "PR658"})
		if err != nil {
			t.Fatal(err)
		}
		if len(station.BackupEmployeeIDs) != 1 || station.BackupEmployeeIDs[0] != 6582 {
			t.Fatalf("saved backup missing: %+v", station)
		}
		stations = append(stations, station.ID)
	}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error { c.Set("actor", "PR658"); c.Set("employee_id", int64(6583)); return next(c) }
	})
	registerProductionRosterAPI(e, productionapp.NewService(productionpg.NewRepository(pool, schema)))
	post := func(path string, body map[string]any) (int, productionapp.ProductionRosterWeek, string) {
		b, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", path, strings.NewReader(string(b)))
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		e.ServeHTTP(res, req)
		var out productionapp.ProductionRosterWeek
		_ = json.Unmarshal(res.Body.Bytes(), &out)
		return res.Code, out, res.Body.String()
	}
	entries := []map[string]any{}
	for _, day := range []string{"2026-09-14", "2026-09-15"} {
		for _, id := range []int64{6581, 6582, 6583} {
			entries = append(entries, map[string]any{"employee_id": id, "work_date": day, "status": "working"})
		}
	}
	body := map[string]any{"week_start": "2026-09-14", "expected_version": 0, "entries": entries, "overrides": []map[string]any{{"workstation_id": stations[0], "work_date": "2026-09-15", "employee_id": 6583}}, "request_id": "initial"}
	code, _, raw := post("/api/production-roster/save", body)
	if code != 200 {
		t.Fatalf("outside-list working owner rejected: %d %s", code, raw)
	}
	body["expected_version"] = 1
	body["request_id"] = "leave"
	for _, entry := range entries {
		if entry["work_date"] == "2026-09-15" && (entry["employee_id"] == int64(6581) || entry["employee_id"] == int64(6583)) {
			entry["status"] = "off"
		}
	}
	code, out, raw := post("/api/production-roster/preview", body)
	if code != 200 {
		t.Fatalf("leave preview: %d %s", code, raw)
	}
	for _, a := range out.Assignments {
		if a.WorkDate == "2026-09-15" && a.EmployeeID != 6582 {
			t.Fatalf("backup did not relieve: %+v", a)
		}
	}
	if len(out.Overrides) != 0 {
		t.Fatalf("off manual owner not released: %+v", out.Overrides)
	}
	code, _, raw = post("/api/production-roster/save", body)
	if code != 200 {
		t.Fatalf("leave save: %d %s", code, raw)
	}
	body["expected_version"] = 2
	body["request_id"] = "replace"
	body["overrides"] = []any{}
	body["replacement"] = map[string]any{"work_date": "2026-09-14", "from_employee_id": 6581, "to_employee_id": 6583, "workstation_ids": stations, "mark_from_off": true}
	code, out, raw = post("/api/production-roster/save", body)
	if code != 200 {
		t.Fatalf("bulk replace: %d %s", code, raw)
	}
	for _, a := range out.Assignments {
		if a.WorkDate == "2026-09-14" && a.EmployeeID != 6583 {
			t.Fatalf("replacement not applied: %+v", a)
		}
	}
	for _, entry := range out.Entries {
		if entry.EmployeeID == 6581 && entry.WorkDate == "2026-09-14" && entry.Status != "off" {
			t.Fatal("A not set off")
		}
	}
	code, replayed, raw := post("/api/production-roster/save", body)
	if code != 200 || !replayed.Replayed {
		t.Fatalf("replay failed %d %s", code, raw)
	}
	body["request_id"] = "stale"
	code, _, _ = post("/api/production-roster/save", body)
	if code != 409 {
		t.Fatalf("stale version=%d", code)
	}
	body["expected_version"] = 3
	body["request_id"] = "invalid"
	body["replacement"] = map[string]any{"work_date": "2026-09-15", "from_employee_id": 6582, "to_employee_id": 6583, "workstation_ids": stations}
	code, _, _ = post("/api/production-roster/save", body)
	if code == 200 {
		t.Fatal("off replacement accepted")
	}
	var version int
	_ = pool.QueryRow(ctx, fmt.Sprintf(`SELECT version FROM %s.production_roster_weeks WHERE week_start='2026-09-14'`, schema)).Scan(&version)
	if version != 3 {
		t.Fatal("invalid batch partially saved")
	}
}
