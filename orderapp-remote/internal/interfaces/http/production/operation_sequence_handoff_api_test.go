package production

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	productionapp "orderapp/internal/application/production"
	postgresproduction "orderapp/internal/infrastructure/postgres/production"
	"strings"
	"testing"
)

func TestPR723OperationSequenceIgnoresPlannedQuantity(t *testing.T) {
	for _, tc := range []struct {
		name, unit    string
		first, second float64
	}{
		{"kg below planned input", "kg", 8.6, 2.6},
		{"grams below planned input", "g", 8600, 2600},
		{"count below planned input", "件", 8, 2},
		{"zero yield does not become a quantity gate", "kg", 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool, schema := newProductionFlowTestDB(t)
			ctx := context.Background()
			mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
    INSERT INTO %[1]s.produce_running_items(id,batch_id,product_id,product_name,spec_g,need_g,order_nos,status,started_by,input_g) VALUES(723,'PR723',1,'交接测试',0,11627,'PR723','running','测试',13000);
    INSERT INTO %[1]s.work_orders(id,work_order_no,running_item_id,batch_id,product_id,product_name,inventory_unit,planned_inventory_qty,planned_g,planned_output_g,status) VALUES(723,'PR723',723,'PR723',1,'交接测试','%[2]s',9.534,11627,9534,'running');
    INSERT INTO %[1]s.job_cards(id,work_order_id,sequence_no,operation,workstation,status,assigned_to,planned_input_qty,actual_output_qty) VALUES
    (7231,723,1,'烘焙','布勒','completed','测试员',10000,%[3]f),
    (7232,723,1,'烘焙','智烘','completed','测试员',3000,%[4]f),
    (7233,723,2,'色选','色选机','pending','测试员',11627,0),
    (7234,723,3,'包装','包装台','pending','测试员',11627,0);
   `, schema, tc.unit, tc.first, tc.second))
			app := newProductionFlowTestEcho(pool, schema)
			registerProductionWorkstationAPI(app, productionapp.NewService(postgresproduction.NewRepository(pool, schema)))
			started := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/job-cards/7233/start", nil)
			if started.Code != 200 {
				t.Fatalf("completed predecessor must permit start regardless of planned quantity: %d %s", started.Code, started.Body.String())
			}
			assertProductionFlowCount(t, pool, schema, "job_cards", "id=7233 AND status='running' AND planned_input_qty=11627", 1)
			assertProductionFlowCount(t, pool, schema, "audit_logs", "entity_type='job_card' AND entity_id=7233 AND action='start'", 1)
			// Even abundant reported output cannot let a later operation bypass an unfinished step.
			mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.job_cards SET actual_output_qty=99999 WHERE id=7233`, schema))
			blocked := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/job-cards/7234/start", nil)
			if blocked.Code != 400 || !strings.Contains(blocked.Body.String(), "前序工序未完成") {
				t.Fatalf("must enforce operation order: %d %s", blocked.Code, blocked.Body.String())
			}
			assertProductionFlowCount(t, pool, schema, "job_cards", "id=7234 AND status='pending'", 1)
			done := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/job-cards/7233/complete", map[string]any{"actual_input_qty": tc.first + tc.second, "actual_output_qty": tc.first + tc.second, "metrics_json": map[string]any{"inventory_unit": tc.unit, "quantity_basis": "inventory_unit"}})
			if done.Code != 200 {
				t.Fatalf("complete actual handoff: %d %s", done.Code, done.Body.String())
			}
			next := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/job-cards/7234/start", nil)
			if next.Code != 200 {
				t.Fatalf("next step after actual completion: %d %s", next.Code, next.Body.String())
			}
		})
	}
}

func TestPR723OperationSequenceBlocksEveryUnfinishedEarlierStep(t *testing.T) {
	for _, action := range []string{"start", "resume", "complete"} {
		t.Run(action, func(t *testing.T) {
			pool, schema := newProductionFlowTestDB(t)
			ctx := context.Background()
			status := "pending"
			if action == "resume" {
				status = "paused"
			}
			if action == "complete" {
				status = "running"
			}
			mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
    INSERT INTO %[1]s.produce_running_items(id,batch_id,product_id,product_name,spec_g,need_g,order_nos,status,started_by,input_g) VALUES(723,'PR723',1,'顺序测试',0,100,'PR723','running','测试',100);
    INSERT INTO %[1]s.work_orders(id,work_order_no,running_item_id,batch_id,product_id,product_name,inventory_unit,planned_g,planned_output_g,status) VALUES(723,'PR723',723,'PR723',1,'顺序测试','kg',100,100,'running');
    INSERT INTO %[1]s.job_cards(id,work_order_id,sequence_no,operation,workstation,status,assigned_to,planned_input_qty,actual_output_qty) VALUES
    (7231,723,1,'前处理','前处理','pending','测试员',100,99999),
    (7232,723,2,'中间处理','中间处理','completed','测试员',100,99999),
    (7233,723,3,'末处理','末处理','%[2]s','测试员',100,0);
   `, schema, status))
			app := newProductionFlowTestEcho(pool, schema)
			res := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/job-cards/7233/"+action, map[string]any{"actual_input_qty": 1, "actual_output_qty": 1})
			if res.Code != 400 || !strings.Contains(res.Body.String(), "前序工序未完成") {
				t.Fatalf("%s bypassed earlier operation: %d %s", action, res.Code, res.Body.String())
			}
			assertProductionFlowCount(t, pool, schema, "audit_logs", "entity_type='job_card' AND entity_id=7233", 0)
			assertProductionFlowCount(t, pool, schema, "job_cards", fmt.Sprintf("id=7233 AND status='%s'", status), 1)
			// Cancelling a task with no output does not impose a quantity requirement.
			mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.job_cards SET status='cancelled',actual_output_qty=0 WHERE id=7231`, schema))
			res = serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/job-cards/7233/"+action, map[string]any{"actual_input_qty": 1, "actual_output_qty": 1})
			if res.Code != 200 {
				t.Fatalf("cancelled earlier task should not block: %d %s", res.Code, res.Body.String())
			}
		})
	}
}

func TestPR723WorkstationReflectsPredecessorStateAndActualOutput(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
  INSERT INTO %[1]s.work_orders(id,work_order_no,product_id,product_name,inventory_unit,planned_g,planned_output_g,status) VALUES(723,'PR723',1,'交接测试','kg',11627,9534,'running');
  INSERT INTO %[1]s.job_cards(id,work_order_id,sequence_no,operation,workstation,status,assigned_to,planned_input_qty,actual_output_qty) VALUES
  (7231,723,1,'烘焙','布勒','completed','测试员',10000,8.6),
  (7232,723,1,'烘焙','智烘','pending','测试员',3000,2.6),
  (7233,723,2,'色选','色选机','pending','测试员',11627,0);
 `, schema))
	app := newProductionFlowTestEcho(pool, schema)
	registerProductionWorkstationAPI(app, productionapp.NewService(postgresproduction.NewRepository(pool, schema)))
	for _, completed := range []bool{false, true} {
		if completed {
			mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.job_cards SET status='completed' WHERE id=7232`, schema))
		}
		res := serveMultilevelProductionJSON(t, app, http.MethodGet, "/api/production/workstation-overview?scope=all", nil)
		if res.Code != 200 {
			t.Fatalf("overview: %d %s", res.Code, res.Body.String())
		}
		var body struct {
			Tasks []map[string]any `json:"tasks"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		var task map[string]any
		for _, row := range body.Tasks {
			if row["job_card_id"] == float64(7233) {
				task = row
			}
		}
		if task == nil {
			t.Fatalf("missing task: %s", res.Body.String())
		}
		if task["can_start"] != completed || task["is_blocked"] == completed {
			t.Fatalf("predecessor completion=%v must control UI readiness: %+v", completed, task)
		}
		if completed {
			if task["predecessor_output_qty"] != 11.2 || task["material_readiness_state"] != "ready" {
				t.Fatalf("actual kg output and readiness: %+v", task)
			}
		} else if !strings.Contains(fmt.Sprint(task["blocking_reason"]), "前序工序未完成") {
			t.Fatalf("missing predecessor reason: %+v", task)
		}
	}
}

func TestPR723CancelledPredecessorDoesNotBecomeRawMaterialGate(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
 INSERT INTO %[1]s.produce_running_items(id,batch_id,product_id,product_name,spec_g,need_g,order_nos,status,started_by,input_g) VALUES(723,'PR723',1,'取消交接测试',0,100,'PR723','running','测试',100);
 INSERT INTO %[1]s.work_orders(id,work_order_no,running_item_id,batch_id,product_id,product_name,inventory_unit,planned_g,planned_output_g,status) VALUES(723,'PR723',723,'PR723',1,'取消交接测试','kg',100,100,'running');
 INSERT INTO %[1]s.job_cards(id,work_order_id,sequence_no,operation,workstation,status,assigned_to,planned_input_qty) VALUES
 (7231,723,1,'前处理','前处理','cancelled','测试员',100),
 (7233,723,2,'后处理','后处理','pending','测试员',100);
 `, schema))
	app := newProductionFlowTestEcho(pool, schema)
	res := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/job-cards/7233/start", nil)
	if res.Code != 200 {
		t.Fatalf("cancelled predecessor must not turn later operation into raw WIP task: %d %s", res.Code, res.Body.String())
	}
}
