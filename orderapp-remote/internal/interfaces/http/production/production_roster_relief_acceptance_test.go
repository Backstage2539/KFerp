package production

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	manufacturingapp "orderapp/internal/application/manufacturing"
	productionapp "orderapp/internal/application/production"
	authzpg "orderapp/internal/infrastructure/postgres/authz"
	manufacturingpg "orderapp/internal/infrastructure/postgres/manufacturing"
	productionpg "orderapp/internal/infrastructure/postgres/production"
)

// Only uses the disposable schema created by newProductionFlowTestDB. This same
// fixture is used by API regressions and opt-in real-browser acceptance.
func seedRosterReliefFixture(t *testing.T) (*pgxpool.Pool, string, []int64, *productionapp.Service) {
	t.Helper()
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %s.company_departments(id,name,active) VALUES(658,'隔离排班验收',true);
 INSERT INTO %s.company_employees(id,name,phone,department_id,active,account_type) VALUES
 (6581,'验收 A 主负责人','19900006581',658,true,'internal_employee'),
 (6582,'验收 B 第一替补','19900006582',658,true,'internal_employee'),
 (6583,'验收 C 临时员工','19900006583',658,true,'internal_employee'),
 (6584,'验收 D 第二替补','19900006584',658,true,'internal_employee');`, schema, schema))
	hash := sha256.Sum256([]byte("orderapp-mobile-auth:Fixture658!"))
	for _, id := range []int64{6581, 6582, 6583, 6584} {
		_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.employee_login_passwords(employee_id,password_hash,login_disabled,must_reset_password) VALUES($1,$2,false,false)`, schema), id, hex.EncodeToString(hash[:]))
		if err != nil {
			t.Fatal(err)
		}
	}
	m := manufacturingapp.NewService(manufacturingpg.NewRepository(pool, schema))
	ids := []int64{}
	for _, name := range []string{"隔离智烘", "隔离包装台", "隔离备料台"} {
		s, err := m.SaveManufacturingWorkstation(ctx, manufacturingapp.SaveManufacturingWorkstationCommand{Name: name, Code: name, Status: "active", PrimaryEmployeeID: 6581, BackupEmployeeIDs: []int64{6582, 6584}, Actor: "PR658隔离数据"})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, s.ID)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
 INSERT INTO %s.materials(id,code,name,kind,unit,cost_unit,onhand_g,onhand_units,purchase_price,sale_price) VALUES(658,'PR658-BEAN','隔离验收生豆','bean','kg','kg',4000,0,50,0);
 INSERT INTO %s.produce_running_items(id,batch_id,product_id,product_name,spec_g,need_g,order_nos,status,started_by,input_g) VALUES(658,'PR658-BATCH',1,'隔离榛巧拼配',0,4000,'SO-PR658','running','隔离数据',4000);
 INSERT INTO %s.work_orders(id,work_order_no,running_item_id,batch_id,product_id,product_name,inventory_unit,planned_inventory_qty,planned_g,planned_output_g,order_nos,status,work_center) VALUES(658,'WO-PR658-ISOLATED',658,'PR658-BATCH',1,'隔离榛巧拼配','kg',4,4000,4000,'SO-PR658','running','隔离智烘');
 INSERT INTO %s.job_cards(id,work_order_id,sequence_no,operation,workstation_id,workstation,status,planned_input_qty) VALUES(6581,658,1,'咖啡烘焙+除石',%d,'隔离智烘','pending',2000),(6582,658,1,'咖啡烘焙+除石',%d,'隔离智烘','pending',2000);
 INSERT INTO %s.work_order_material_reservations(id,work_order_id,running_item_id,material_id,material_name,unit,required_g,reserved_g,consumed_g,status,component_type) VALUES(658,658,658,658,'隔离验收生豆','g',4000,4000,0,'reserved','material');
 `, schema, schema, schema, schema, ids[0], ids[0], schema))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 658, 658, "MB-PR658", "隔离验收生豆", 4000)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %s.work_order_material_reservation_batches(reservation_id,work_order_id,material_id,component_type,component_id,material_batch_id,batch_code,warehouse,reserved_g,consumed_g,status) VALUES(658,658,658,'material',658,658,'MB-PR658','wip',4000,0,'reserved');`, schema))
	return pool, schema, ids, productionapp.NewService(productionpg.NewRepository(pool, schema))
}

func TestPR658PreviewScopeAndOutsideOwnerExecution(t *testing.T) {
	pool, schema, stations, svc := seedRosterReliefFixture(t)
	ctx := context.Background()
	loc, _ := time.LoadLocation("Asia/Shanghai")
	today := time.Now().In(loc).Format("2006-01-02")
	week, days, _ := productionapp.NormalizeProductionRosterWeek(today, loc)
	entries := []productionapp.ProductionAttendanceEntry{}
	for _, day := range days {
		for _, id := range []int64{6581, 6582, 6583, 6584} {
			entries = append(entries, productionapp.ProductionAttendanceEntry{EmployeeID: id, WorkDate: day, Status: "working"})
		}
	}
	cmd := productionapp.SaveProductionRosterCommand{WeekStart: week, Entries: entries, RequestID: "initial", Operator: "PR658"}
	saved, err := svc.SaveProductionRoster(ctx, cmd, false)
	if err != nil {
		t.Fatal(err)
	}
	cmd.ExpectedVersion = saved.Version
	cmd.RequestID = "partial"
	cmd.Replacement = &productionapp.ProductionRosterReplacement{WorkDate: today, FromEmployeeID: 6581, ToEmployeeID: 6583, WorkstationIDs: stations[:1]}
	preview, err := svc.SaveProductionRoster(ctx, cmd, true)
	if err != nil {
		t.Fatal(err)
	}
	if preview.AffectedTaskCount != 2 || len(preview.Changes) != 1 || len(preview.Changes[0].TaskIDs) != 2 {
		t.Fatalf("incorrect affected scope: %+v", preview.Changes)
	}
	cmd.ExpectedPreviewFingerprint = preview.PreviewFingerprint
	// A concurrent task replacement with the same count must invalidate the preview.
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.job_cards SET id=6589 WHERE id=6582`, schema))
	if _, err = svc.SaveProductionRoster(ctx, cmd, false); err == nil || !strings.Contains(err.Error(), "重新核对") {
		t.Fatalf("same-count changed scope accepted: %v", err)
	}
	cmd.ExpectedPreviewFingerprint = ""
	preview, err = svc.SaveProductionRoster(ctx, cmd, true)
	if err != nil {
		t.Fatal(err)
	}
	cmd.ExpectedPreviewFingerprint = preview.PreviewFingerprint
	saved, err = svc.SaveProductionRoster(ctx, cmd, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range saved.Assignments {
		if a.WorkDate == today {
			want := int64(6581)
			if a.WorkstationID == stations[0] {
				want = 6583
			}
			if a.EmployeeID != want {
				t.Fatalf("partial replacement changed extra station %+v", a)
			}
		}
	}
	for _, e := range saved.Entries {
		if e.EmployeeID == 6581 && e.WorkDate == today && e.Status != "working" {
			t.Fatal("default replacement changed A attendance")
		}
	}
	// Test the actual API start as the outside-list employee, then shared 'mine' ownership.
	actorID := int64(6583)
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error { c.Set("actor", "PR658"); c.Set("employee_id", actorID); return next(c) }
	})
	registerWorkOrderAPI(e, svc)
	registerProductionRosterAPI(e, svc)
	registerProductionWorkstationAPI(e, svc)
	request := func(method, path string, body any) *httptest.ResponseRecorder {
		data, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, strings.NewReader(string(data)))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		e.ServeHTTP(w, r)
		return w
	}
	beforeStart := request("GET", "/api/production/workstation-overview?scope=mine", nil)
	var overview productionapp.ProductionWorkstationOverview
	_ = json.Unmarshal(beforeStart.Body.Bytes(), &overview)
	if beforeStart.Code != 200 || len(overview.Tasks) != 2 || !overview.Tasks[0].CanStart {
		t.Fatalf("assigned roster owner still blocked before start: %s", beforeStart.Body.String())
	}
	started := request("POST", "/api/job-cards/6581/start", map[string]any{})
	if started.Code != 200 {
		t.Fatalf("outside owner start %d %s", started.Code, started.Body.String())
	}
	var actual int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT assigned_employee_id FROM %s.job_cards WHERE id=6581`, schema)).Scan(&actual); err != nil || actual != 6583 {
		t.Fatalf("start snapshot %d %v", actual, err)
	}
	mine := request("GET", "/api/production/workstation-overview?scope=mine", nil)
	if mine.Code != 200 || !strings.Contains(mine.Body.String(), "隔离智烘") {
		t.Fatalf("mine %d %s", mine.Code, mine.Body.String())
	}
	// Leave releases this outside-list override, while execution snapshots wait for handover.
	cmd = productionapp.SaveProductionRosterCommand{WeekStart: week, Entries: saved.Entries, Overrides: saved.Overrides, ExpectedVersion: saved.Version, RequestID: "outside-leave", Operator: "PR658"}
	for i := range cmd.Entries {
		if cmd.Entries[i].WorkDate == today && (cmd.Entries[i].EmployeeID == 6581 || cmd.Entries[i].EmployeeID == 6583) {
			cmd.Entries[i].Status = "off"
		}
	}
	preview, err = svc.SaveProductionRoster(ctx, cmd, true)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, change := range preview.Changes {
		if change.WorkstationID == stations[0] && change.WorkDate == today {
			found = true
			if change.HandoverCount != 1 || change.ToEmployeeID != 6582 || change.TaskCount != 1 {
				t.Fatalf("handover preview %+v", change)
			}
		}
	}
	if !found {
		t.Fatal("missing relief")
	}
	cmd.ExpectedPreviewFingerprint = preview.PreviewFingerprint
	saved, err = svc.SaveProductionRoster(ctx, cmd, false)
	if err != nil {
		t.Fatal(err)
	}
	pool.QueryRow(ctx, fmt.Sprintf(`SELECT assigned_employee_id FROM %s.job_cards WHERE id=6581`, schema)).Scan(&actual)
	if actual != 6583 {
		t.Fatal("save rewrote actual operator before handover")
	}
	forged := request("POST", "/api/production-roster/handover", productionapp.HandoverWorkstationCommand{WorkstationID: stations[0], WorkDate: today, EmployeeID: 6582, ExpectedVersion: saved.Version, RequestID: "forged-handover"})
	if forged.Code != 403 {
		t.Fatalf("another employee confirmed handover: %d", forged.Code)
	}
	actorID = 6582
	handover := request("POST", "/api/production-roster/handover", productionapp.HandoverWorkstationCommand{WorkstationID: stations[0], WorkDate: today, EmployeeID: 6582, ExpectedVersion: saved.Version, RequestID: "handover"})
	if handover.Code != 200 {
		t.Fatalf("handover %d %s", handover.Code, handover.Body.String())
	}
	pool.QueryRow(ctx, fmt.Sprintf(`SELECT assigned_employee_id FROM %s.job_cards WHERE id=6581`, schema)).Scan(&actual)
	if actual != 6582 {
		t.Fatal("handover not applied")
	}
	// Follow the saved relief list to override all originally affected A stations.
	current, err := svc.ProductionRosterWeek(ctx, productionapp.ProductionRosterQuery{WeekStart: week})
	if err != nil {
		t.Fatal(err)
	}
	if len(current.RecentChanges) == 0 {
		t.Fatal("saved relief list missing after reload")
	}
	var audit string
	pool.QueryRow(ctx, fmt.Sprintf(`SELECT meta::text FROM %s.audit_logs WHERE entity_type='production_roster' ORDER BY id DESC LIMIT 1`, schema)).Scan(&audit)
	if !strings.Contains(audit, "6581") || !strings.Contains(audit, "released_overrides") {
		t.Fatalf("relief audit missing scope %s", audit)
	}
}

// Run with ORDERAPP_ROSTER_ACCEPTANCE_READY=/tmp/unique-file. Start the normal
// local application with the printed DB_SCHEMA. Removing the ready file stops
// the fixture and drops its entire isolated schema.
func TestPR658BrowserFixture(t *testing.T) {
	ready := os.Getenv("ORDERAPP_ROSTER_ACCEPTANCE_READY")
	if ready == "" {
		t.Skip("opt-in browser fixture")
	}
	pool, schema, _, _ := seedRosterReliefFixture(t)
	ctx := context.Background()
	// Replace the production API suite's deliberately minimal empty portal table;
	// the normal application will create its full portal schema on startup.
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`ALTER TABLE %s.customer_processing_production_demands RENAME TO fixture_minimal_demands`, schema))
	if err := authzpg.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %s.employee_roles(employee_id,role_code) VALUES(6581,'admin'),(6582,'production'),(6583,'production'),(6584,'production')`, schema))
	if err := os.WriteFile(ready, []byte(schema), 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("isolated schema ready: %s", schema)
	for {
		time.Sleep(time.Second)
		if _, err := os.Stat(ready); os.IsNotExist(err) {
			return
		}
	}
}
