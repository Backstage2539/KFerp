package production

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	productionapp "orderapp/internal/application/production"
	postgresbom "orderapp/internal/infrastructure/postgres/bom"
	postgrescompany "orderapp/internal/infrastructure/postgres/company"
	postgresinventory "orderapp/internal/infrastructure/postgres/inventory"
	postgresmaterials "orderapp/internal/infrastructure/postgres/materials"
	postgresproduction "orderapp/internal/infrastructure/postgres/production"
	postgressales "orderapp/internal/infrastructure/postgres/sales"
	postgresstock "orderapp/internal/infrastructure/postgres/stock"
	supporthttp "orderapp/internal/interfaces/http/support"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type materialSummaryItemForTest struct {
	MaterialName string `json:"material_name"`
	DeductG      int64  `json:"deduct_g"`
	DeductUnits  int64  `json:"deduct_units"`
}

func TestProduceStartHandlerPersistsInputG(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.products(id,name,default_price,active) VALUES (1,'橘皮乌龙',50,true);
		INSERT INTO %s.order_process_statuses(name,sort,active) VALUES
			('待处理',10,true),
			('生产中',20,true)
		ON CONFLICT (name) DO NOTHING;
		INSERT INTO %s.orders(id,order_no,order_date,is_void,process_status_id) VALUES
			(1,'SO-TEST-START','2026-04-25',false,(SELECT id FROM %s.order_process_statuses WHERE name='待处理' LIMIT 1));
		INSERT INTO %s.order_items(order_id,line_no,item_name,qty,unit,spec,product_id,unit_price,line_total)
		VALUES (1,1,'橘皮乌龙',2,'袋','227g',1,50,100);
		INSERT INTO %s.product_bom(product_id,yield_rate) VALUES (1,0.8200);
		INSERT INTO %s.materials(id,code,name,kind,unit,onhand_g,onhand_units,purchase_price,sale_price)
			VALUES (10,'RAW-001','卡蒂姆水洗','bean','g',1000,0,54,0);
		INSERT INTO %s.product_bom_items(product_id,material_id,ratio_pct) VALUES (1,10,100.0000);
	`, schema, schema, schema, schema, schema, schema, schema, schema))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 10, 10, "MB-RAW-001", "卡蒂姆水洗", 1000)

	app := newProductionFlowTestEcho(pool, schema)
	form := url.Values{
		"selected":      {"1-227"},
		"input_g_1_227": {"600"},
	}
	rec := serveProductionFlowForm(t, app, http.MethodPost, "/produce/start", form)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("POST /produce/start status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != "../produce/running?ok=1" {
		t.Fatalf("POST /produce/start Location = %q, want %q", got, "../produce/running?ok=1")
	}

	var inputG, plannedUnits, plannedLooseG int64
	var bomYieldRate float64
	var status, startedBy string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT input_g, bom_yield_rate, planned_units, planned_loose_g, status, started_by
		FROM %s.produce_running_items
		WHERE product_id=1 AND spec_g=227
		ORDER BY id DESC
		LIMIT 1
	`, schema)).Scan(&inputG, &bomYieldRate, &plannedUnits, &plannedLooseG, &status, &startedBy); err != nil {
		t.Fatalf("query running item: %v", err)
	}
	if inputG != 600 {
		t.Fatalf("running item input_g = %d, want 600", inputG)
	}
	if math.Abs(bomYieldRate-0.82) > 0.0001 {
		t.Fatalf("running item bom_yield_rate = %.4f, want 0.8200", bomYieldRate)
	}
	if plannedUnits != 2 || plannedLooseG != 38 {
		t.Fatalf("planned inventory = %d units + %dg, want 2 units + 38g", plannedUnits, plannedLooseG)
	}
	if status != "running" {
		t.Fatalf("running item status = %q, want running", status)
	}
	if startedBy != "测试员" {
		t.Fatalf("running item started_by = %q, want 测试员", startedBy)
	}

	var processStatusName string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(s.name,'')
		FROM %s.orders o
		LEFT JOIN %s.order_process_statuses s ON s.id=o.process_status_id
		WHERE o.id=1
	`, schema, schema)).Scan(&processStatusName); err != nil {
		t.Fatalf("query order status: %v", err)
	}
	if processStatusName != "生产中" {
		t.Fatalf("order process_status = %q, want 生产中", processStatusName)
	}
}

func TestProduceStartAPIMergesSameProductSpecsAndKeepsAllOrderNos(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.products(id,name,default_price,active) VALUES (1,'Uraga乌拉嘎',50,true);
		INSERT INTO %s.order_process_statuses(name,sort,active) VALUES
			('待处理',10,true),
			('生产中',20,true)
		ON CONFLICT (name) DO NOTHING;
		INSERT INTO %s.orders(id,order_no,order_date,is_void,process_status_id) VALUES
			(1,'SO-MERGE-454','2026-05-01',false,(SELECT id FROM %s.order_process_statuses WHERE name='待处理' LIMIT 1)),
			(2,'SO-MERGE-227','2026-05-01',false,(SELECT id FROM %s.order_process_statuses WHERE name='待处理' LIMIT 1));
		INSERT INTO %s.order_items(order_id,line_no,item_name,qty,unit,spec,product_id,unit_price,line_total)
		VALUES
			(1,1,'Uraga乌拉嘎',24,'袋','454g',1,50,1200),
			(2,1,'Uraga乌拉嘎',2,'袋','227g',1,50,100);
		INSERT INTO %s.product_bom(product_id,yield_rate) VALUES (1,0.8200);
		INSERT INTO %s.materials(id,code,name,kind,unit,onhand_g,onhand_units,purchase_price,sale_price)
			VALUES (10,'RAW-URAGA','乌拉嘎生豆','bean','g',30000,0,54,0);
		INSERT INTO %s.product_bom_items(product_id,material_id,ratio_pct) VALUES (1,10,100.0000);
		INSERT INTO %s.material_batches(id,batch_code,material_id,material_name,received_g,remaining_g,unit_cost,status,quality_status)
			VALUES (10,'MB-URAGA',10,'乌拉嘎生豆',30000,30000,54,'active','pass');
		INSERT INTO %s.material_batch_locations(material_batch_id,material_id,warehouse,qty_g)
			VALUES (10,10,'wip',30000);
	`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema))

	app := newProductionFlowTestEcho(pool, schema)
	body := bytes.NewReader([]byte(`{"selected":["1-454","1-227"],"input_by_key":{"1-454":16000,"1-227":600}}`))
	req := httptest.NewRequest(http.MethodPost, "/api/produce/start", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/produce/start status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var runningCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*)::int FROM %s.produce_running_items WHERE product_id=1 AND status='running'`, schema)).Scan(&runningCount); err != nil {
		t.Fatalf("query running count: %v", err)
	}
	if runningCount != 1 {
		t.Fatalf("running item count = %d, want 1 merged product-level item", runningCount)
	}

	var runningItemID, needG, inputG int64
	var orderNos string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, need_g, input_g, order_nos
		FROM %s.produce_running_items
		WHERE product_id=1 AND status='running'
	`, schema)).Scan(&runningItemID, &needG, &inputG, &orderNos); err != nil {
		t.Fatalf("query running item: %v", err)
	}
	if needG != 11350 || inputG != 16600 {
		t.Fatalf("running item need/input = %d/%d, want 11350/16600", needG, inputG)
	}
	for _, want := range []string{"SO-MERGE-454", "SO-MERGE-227"} {
		if !strings.Contains(orderNos, want) {
			t.Fatalf("running order_nos = %q, missing %s", orderNos, want)
		}
	}

	var outputCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*)::int FROM %s.produce_running_outputs WHERE running_item_id=$1`, schema), runningItemID).Scan(&outputCount); err != nil {
		t.Fatalf("query running outputs: %v", err)
	}
	if outputCount != 2 {
		t.Fatalf("running output count = %d, want 2", outputCount)
	}
}

func TestProduceFinishAPIMultiSpecRunCompletesAllLinkedOrders(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.products(id,name,default_price,active) VALUES (1,'Uraga乌拉嘎',50,true);
		INSERT INTO %s.order_process_statuses(name,sort,active) VALUES
			('生产中',20,true),
			('生产完成',35,true)
		ON CONFLICT (name) DO NOTHING;
		INSERT INTO %s.orders(id,order_no,order_date,is_void,process_status_id) VALUES
			(1,'SO-MERGE-454','2026-05-01',false,(SELECT id FROM %s.order_process_statuses WHERE name='生产中' LIMIT 1)),
			(2,'SO-MERGE-227','2026-05-01',false,(SELECT id FROM %s.order_process_statuses WHERE name='生产中' LIMIT 1));
		INSERT INTO %s.product_bom(product_id,yield_rate) VALUES (1,0.8200);
		INSERT INTO %s.materials(id,code,name,kind,unit,onhand_g,onhand_units,purchase_price,sale_price)
			VALUES
			(10,'RAW-URAGA','乌拉嘎生豆','bean','g',30000,0,54,0),
			(11,'BAG-454','454g豆袋','pack','个',0,30,1,0),
			(12,'BAG-227','227g豆袋','pack','个',0,10,1,0);
		INSERT INTO %s.product_bom_items(product_id,material_id,ratio_pct) VALUES (1,10,100.0000);
		INSERT INTO %s.packaging_spec_material_map(spec_g,material_id) VALUES (454,11),(227,12);
		INSERT INTO %s.material_batches(id,batch_code,material_id,material_name,received_g,remaining_g,unit_cost,status,quality_status)
			VALUES (10,'MB-URAGA',10,'乌拉嘎生豆',30000,30000,54,'active','pass');
		INSERT INTO %s.material_batch_locations(material_batch_id,material_id,warehouse,qty_g)
			VALUES (10,10,'wip',30000);
		INSERT INTO %s.produce_running_items(
			id,batch_id,product_id,product_name,spec_g,need_g,order_nos,status,
			started_by,started_at,input_g,bom_yield_rate,planned_units,planned_loose_g
		) VALUES (
			1,'BATCH-MERGE-001',1,'Uraga乌拉嘎',0,11350,'SO-MERGE-454,SO-MERGE-227','running',
			'测试员',now(),16600,0.8200,0,0
		);
		INSERT INTO %s.produce_running_outputs(
			running_item_id,product_id,product_name,spec_g,need_g,order_nos,planned_units,planned_loose_g
		) VALUES
			(1,1,'Uraga乌拉嘎',454,10896,'SO-MERGE-454',24,0),
			(1,1,'Uraga乌拉嘎',227,454,'SO-MERGE-227',2,0);
		INSERT INTO %s.work_orders(work_order_no,running_item_id,batch_id,product_id,product_name,spec_g,planned_g,status)
		VALUES ('WO-0000000001',1,'BATCH-MERGE-001',1,'Uraga乌拉嘎',0,16600,'running');
	`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema))

	app := newProductionFlowTestEcho(pool, schema)
	body := bytes.NewReader([]byte(`{
		"id":1,
		"warehouse":"finished_goods",
		"consumed_input_g":16600,
		"outputs":[
			{"spec_g":454,"finished_units":24,"finished_loose_g":0},
			{"spec_g":227,"finished_units":2,"finished_loose_g":0}
		]
	}`))
	req := httptest.NewRequest(http.MethodPost, "/api/produce/running/finish", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/produce/running/finish status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	for _, tc := range []struct {
		specG int64
		units int64
	}{
		{specG: 454, units: 24},
		{specG: 227, units: 2},
	} {
		var units, looseG int64
		if err := pool.QueryRow(ctx, fmt.Sprintf(`
			SELECT onhand_units,onhand_loose_g
			FROM %s.finished_inventory
			WHERE product_id=1 AND spec_g=$1 AND warehouse='finished_goods'
		`, schema), tc.specG).Scan(&units, &looseG); err != nil {
			t.Fatalf("query finished inventory spec %d: %v", tc.specG, err)
		}
		if units != tc.units || looseG != 0 {
			t.Fatalf("inventory spec %d = %d units + %dg, want %d units + 0g", tc.specG, units, looseG, tc.units)
		}
	}

	rows, err := pool.Query(ctx, fmt.Sprintf(`
		SELECT o.order_no,COALESCE(s.name,'')
		FROM %s.orders o
		LEFT JOIN %s.order_process_statuses s ON s.id=o.process_status_id
		WHERE o.id IN (1,2)
		ORDER BY o.id
	`, schema, schema))
	if err != nil {
		t.Fatalf("query order statuses: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var orderNo, status string
		if err := rows.Scan(&orderNo, &status); err != nil {
			t.Fatalf("scan order status: %v", err)
		}
		if status != "生产完成" {
			t.Fatalf("order %s status = %q, want 生产完成", orderNo, status)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestProduceFinishHandlerWritesProductionLog(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.products(id,name,default_price,active) VALUES (1,'橘皮乌龙',50,true);
		INSERT INTO %s.order_process_statuses(name,sort,active) VALUES
			('生产中',20,true)
		ON CONFLICT (name) DO NOTHING;
		INSERT INTO %s.orders(id,order_no,order_date,is_void,process_status_id) VALUES
			(1,'SO-TEST-FINISH','2026-04-25',false,(SELECT id FROM %s.order_process_statuses WHERE name='生产中' LIMIT 1));
		INSERT INTO %s.product_bom(product_id,yield_rate) VALUES (1,0.8200);
		INSERT INTO %s.materials(id,code,name,kind,unit,onhand_g,onhand_units,purchase_price,sale_price)
			VALUES
			(10,'RAW-001','卡蒂姆水洗','bean','g',1000,0,54,0),
			(11,'BAG-227','227g豆袋','pack','个',0,10,1,0);
		INSERT INTO %s.product_bom_items(product_id,material_id,ratio_pct) VALUES (1,10,100.0000);
		INSERT INTO %s.packaging_spec_material_map(spec_g,material_id) VALUES (227,11);
		INSERT INTO %s.finished_inventory(product_id,spec_g,onhand_units,onhand_loose_g) VALUES (1,227,1,0);
		INSERT INTO %s.produce_running_items(
			id,batch_id,product_id,product_name,spec_g,need_g,order_nos,status,
			started_by,started_at,input_g,bom_yield_rate,planned_units,planned_loose_g
		) VALUES (
			1,'BATCH-TEST-001',1,'橘皮乌龙',227,454,'SO-TEST-FINISH','running',
			'测试员',now(),600,0.8200,2,0
		);
		`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 10, 10, "MB-RAW-001", "卡蒂姆水洗", 1000)

	app := newProductionFlowTestEcho(pool, schema)
	form := url.Values{
		"id":               {"1"},
		"finished_units":   {"2"},
		"finished_loose_g": {"10"},
	}
	rec := serveProductionFlowForm(t, app, http.MethodPost, "/produce/running/finish", form)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("POST /produce/running/finish status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != "../../produce/running?ok=1" {
		t.Fatalf("POST /produce/running/finish Location = %q, want %q", got, "../../produce/running?ok=1")
	}

	var finishedUnits, finishedLooseG, finishedTotalG, inputG int64
	var actualYield, bomYield float64
	var finishedBy, materialSummary string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT finished_units, finished_loose_g, finished_total_g, input_g,
		       actual_yield_rate, bom_yield_rate, finished_by, material_summary::text
		FROM %s.production_logs
		WHERE running_item_id=1
	`, schema)).Scan(&finishedUnits, &finishedLooseG, &finishedTotalG, &inputG, &actualYield, &bomYield, &finishedBy, &materialSummary); err != nil {
		t.Fatalf("query production log: %v", err)
	}
	if finishedUnits != 2 || finishedLooseG != 10 || finishedTotalG != 464 {
		t.Fatalf("production log output = %d units + %dg => %dg, want 2 units + 10g => 464g", finishedUnits, finishedLooseG, finishedTotalG)
	}
	if inputG != 600 {
		t.Fatalf("production log input_g = %d, want 600", inputG)
	}
	if math.Abs(actualYield-0.7733) > 0.0001 {
		t.Fatalf("production log actual_yield_rate = %.4f, want 0.7733", actualYield)
	}
	if math.Abs(bomYield-0.82) > 0.0001 {
		t.Fatalf("production log bom_yield_rate = %.4f, want 0.8200", bomYield)
	}
	if finishedBy != "测试员" {
		t.Fatalf("production log finished_by = %q, want 测试员", finishedBy)
	}
	var items []materialSummaryItemForTest
	if err := json.Unmarshal([]byte(materialSummary), &items); err != nil {
		t.Fatalf("unmarshal material_summary: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("material summary items = %d, want 2", len(items))
	}
	byName := map[string]materialSummaryItemForTest{}
	for _, item := range items {
		byName[item.MaterialName] = item
	}
	rawItem, ok := byName["卡蒂姆水洗"]
	if !ok || rawItem.DeductG != 600 {
		t.Fatalf("raw material summary = %+v, want deduct_g=600", rawItem)
	}
	bagItem, ok := byName["227g豆袋"]
	if !ok || bagItem.DeductUnits != 2 {
		t.Fatalf("bag material summary = %+v, want deduct_units=2", bagItem)
	}

	var runningStatus string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.produce_running_items WHERE id=1`, schema)).Scan(&runningStatus); err != nil {
		t.Fatalf("query running status: %v", err)
	}
	if runningStatus != "done" {
		t.Fatalf("running item status = %q, want done", runningStatus)
	}

	var invUnits, invLooseG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units,onhand_loose_g FROM %s.finished_inventory WHERE product_id=1 AND spec_g=227`, schema)).Scan(&invUnits, &invLooseG); err != nil {
		t.Fatalf("query finished inventory: %v", err)
	}
	if invUnits != 3 || invLooseG != 10 {
		t.Fatalf("finished inventory = %d units + %dg, want 3 units + 10g", invUnits, invLooseG)
	}

	var rawOnhandG, bagOnhandUnits int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_g FROM %s.materials WHERE id=10`, schema)).Scan(&rawOnhandG); err != nil {
		t.Fatalf("query raw material: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units FROM %s.materials WHERE id=11`, schema)).Scan(&bagOnhandUnits); err != nil {
		t.Fatalf("query bag material: %v", err)
	}
	if rawOnhandG != 400 {
		t.Fatalf("raw material onhand_g = %d, want 400", rawOnhandG)
	}
	if bagOnhandUnits != 8 {
		t.Fatalf("bag material onhand_units = %d, want 8", bagOnhandUnits)
	}

	var processStatusName string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(s.name,'')
		FROM %s.orders o
		LEFT JOIN %s.order_process_statuses s ON s.id=o.process_status_id
		WHERE o.id=1
	`, schema, schema)).Scan(&processStatusName); err != nil {
		t.Fatalf("query order status: %v", err)
	}
	if processStatusName != "生产完成" {
		t.Fatalf("order process_status = %q, want 生产完成", processStatusName)
	}
}

func TestProduceFinishAPIUsesEditedInputForFullCompletion(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.products(id,name,default_price,active) VALUES (1,'橘皮乌龙',50,true);
		INSERT INTO %s.order_process_statuses(name,sort,active) VALUES
			('生产中',20,true)
		ON CONFLICT (name) DO NOTHING;
		INSERT INTO %s.orders(id,order_no,order_date,is_void,process_status_id) VALUES
			(1,'SO-EDITED-INPUT','2026-05-02',false,(SELECT id FROM %s.order_process_statuses WHERE name='生产中' LIMIT 1));
		INSERT INTO %s.product_bom(product_id,yield_rate) VALUES (1,0.8200);
		INSERT INTO %s.materials(id,code,name,kind,unit,onhand_g,onhand_units,purchase_price,sale_price)
			VALUES
			(10,'RAW-001','卡蒂姆水洗','bean','g',1000,0,54,0),
			(11,'BAG-227','227g豆袋','pack','个',0,10,1,0);
		INSERT INTO %s.product_bom_items(product_id,material_id,ratio_pct) VALUES (1,10,100.0000);
		INSERT INTO %s.packaging_spec_material_map(spec_g,material_id) VALUES (227,11);
		INSERT INTO %s.produce_running_items(
			id,batch_id,product_id,product_name,spec_g,need_g,order_nos,status,
			started_by,started_at,input_g,bom_yield_rate,planned_units,planned_loose_g
		) VALUES (
			1,'BATCH-EDITED-INPUT',1,'橘皮乌龙',227,454,'SO-EDITED-INPUT','running',
			'测试员',now(),600,0.8200,2,38
		);
	`, schema, schema, schema, schema, schema, schema, schema, schema, schema))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 10, 10, "MB-RAW-001", "卡蒂姆水洗", 1000)

	app := newProductionFlowTestEcho(pool, schema)
	body := bytes.NewBufferString(`{"id":1,"finished_units":2,"finished_loose_g":10,"consumed_input_g":700}`)
	req := httptest.NewRequest(http.MethodPost, "/api/produce/running/finish", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/produce/running/finish status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	var inputG, rawDeductG, rawOnhandG int64
	var actualYield float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT input_g, actual_yield_rate
		FROM %s.production_logs
		WHERE running_item_id=1
	`, schema)).Scan(&inputG, &actualYield); err != nil {
		t.Fatalf("query production log: %v", err)
	}
	if inputG != 700 {
		t.Fatalf("production log input_g = %d, want edited 700", inputG)
	}
	if math.Abs(actualYield-0.6629) > 0.0001 {
		t.Fatalf("actual_yield_rate = %.4f, want 0.6629", actualYield)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(deduct_g),0)::bigint
		FROM %s.material_consumption_logs
		WHERE running_item_id=1 AND material_id=10
	`, schema)).Scan(&rawDeductG); err != nil {
		t.Fatalf("query raw material consumption: %v", err)
	}
	if rawDeductG != 700 {
		t.Fatalf("raw material deduct_g = %d, want 700", rawDeductG)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_g FROM %s.materials WHERE id=10`, schema)).Scan(&rawOnhandG); err != nil {
		t.Fatalf("query raw material onhand: %v", err)
	}
	if rawOnhandG != 300 {
		t.Fatalf("raw material onhand_g = %d, want 300", rawOnhandG)
	}
}

func TestProduceFinishHandlerWritesStockLedgerAndFinishedBatch(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.products(id,name,default_price,active) VALUES (1,'橘皮乌龙',50,true);
		INSERT INTO %s.product_bom(product_id,yield_rate) VALUES (1,0.8200);
		INSERT INTO %s.materials(id,code,name,kind,unit,onhand_g,onhand_units,purchase_price,sale_price)
			VALUES
			(10,'RAW-001','卡蒂姆水洗','bean','g',1000,0,54,0),
			(11,'BAG-227','227g豆袋','pack','个',0,10,1,0);
		INSERT INTO %s.product_bom_items(product_id,material_id,ratio_pct) VALUES (1,10,100.0000);
		INSERT INTO %s.packaging_spec_material_map(spec_g,material_id) VALUES (227,11);
		INSERT INTO %s.finished_inventory(product_id,spec_g,onhand_units,onhand_loose_g) VALUES (1,227,1,20);
		INSERT INTO %s.produce_running_items(
			id,batch_id,product_id,product_name,spec_g,need_g,order_nos,status,
			started_by,started_at,input_g,bom_yield_rate,planned_units,planned_loose_g
		) VALUES (
			42,'PLAN-BATCH-001',1,'橘皮乌龙',227,454,'SO-STOCK-LEDGER','running',
			'测试员',now(),600,0.8200,2,0
		);
	`, schema, schema, schema, schema, schema, schema, schema))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 10, 10, "MB-RAW-001", "卡蒂姆水洗", 1000)

	app := newProductionFlowTestEcho(pool, schema)
	form := url.Values{
		"id":               {"42"},
		"finished_units":   {"2"},
		"finished_loose_g": {"10"},
	}
	rec := serveProductionFlowForm(t, app, http.MethodPost, "/produce/running/finish", form)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("POST /produce/running/finish status = %d, want %d body=%s", rec.Code, http.StatusSeeOther, rec.Body.String())
	}

	var batchCode string
	var batchQtyG, batchQtyUnits int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT batch_code, qty_g, qty_units
		FROM %s.stock_batches
		WHERE source_doc_type='production_run' AND source_doc_id=42
	`, schema)).Scan(&batchCode, &batchQtyG, &batchQtyUnits); err != nil {
		t.Fatalf("query stock batch: %v", err)
	}
	if batchCode != "FP-0000000042" {
		t.Fatalf("batch_code = %q, want FP-0000000042", batchCode)
	}
	if batchQtyG != 464 || batchQtyUnits != 2 {
		t.Fatalf("batch qty = %dg/%d units, want 464g/2 units", batchQtyG, batchQtyUnits)
	}

	rows, err := pool.Query(ctx, fmt.Sprintf(`
		SELECT item_type, item_name, source_batch_code, qty_change_g, qty_after_g, qty_change_units, qty_after_units
		FROM %s.stock_ledger_entries
		WHERE source_doc_type='production_run' AND source_doc_id=42
		ORDER BY item_type, item_name
	`, schema))
	if err != nil {
		t.Fatalf("query stock ledger: %v", err)
	}
	defer rows.Close()

	type ledgerRow struct {
		itemType        string
		itemName        string
		sourceBatchCode string
		changeG         int64
		afterG          int64
		changeUnits     int64
		afterUnits      int64
	}
	var got []ledgerRow
	for rows.Next() {
		var row ledgerRow
		if err := rows.Scan(&row.itemType, &row.itemName, &row.sourceBatchCode, &row.changeG, &row.afterG, &row.changeUnits, &row.afterUnits); err != nil {
			t.Fatalf("scan stock ledger: %v", err)
		}
		got = append(got, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate stock ledger: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("stock ledger rows = %d, want 3: %+v", len(got), got)
	}

	byName := map[string]ledgerRow{}
	for _, row := range got {
		byName[row.itemName] = row
	}
	finished := byName["橘皮乌龙"]
	if finished.itemType != "finished_product" || finished.sourceBatchCode != "FP-0000000042" || finished.changeG != 464 || finished.afterG != 711 || finished.changeUnits != 2 || finished.afterUnits != 3 {
		t.Fatalf("finished stock ledger row = %+v, want finished_product +464g to 711g and +2 units to 3", finished)
	}
	raw := byName["卡蒂姆水洗"]
	if raw.itemType != "material" || raw.changeG != -600 || raw.afterG != 400 {
		t.Fatalf("raw stock ledger row = %+v, want material -600g to 400g", raw)
	}
	bag := byName["227g豆袋"]
	if bag.itemType != "material" || bag.changeUnits != -2 || bag.afterUnits != 8 {
		t.Fatalf("bag stock ledger row = %+v, want material -2 units to 8", bag)
	}
}

func TestProduceStartFreezesMaterialSnapshotForFinishAndWorkOrder(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.products(id,name,default_price,active) VALUES (1,'橘皮乌龙',50,true);
		INSERT INTO %s.order_process_statuses(name,sort,active) VALUES
			('待处理',10,true),
			('生产中',20,true)
		ON CONFLICT (name) DO NOTHING;
		INSERT INTO %s.orders(id,order_no,order_date,is_void,process_status_id) VALUES
			(1,'SO-SNAPSHOT','2026-04-25',false,(SELECT id FROM %s.order_process_statuses WHERE name='待处理' LIMIT 1));
		INSERT INTO %s.order_items(order_id,line_no,item_name,qty,unit,spec,product_id,unit_price,line_total)
		VALUES (1,1,'橘皮乌龙',2,'袋','227g',1,50,100);
		INSERT INTO %s.product_bom(product_id,yield_rate) VALUES (1,0.8200);
		INSERT INTO %s.materials(id,code,name,kind,unit,onhand_g,onhand_units,purchase_price,sale_price)
			VALUES
			(10,'RAW-A','孟连水洗5T批次','bean','g',1000,0,54,0),
			(12,'RAW-B','后改B批次','bean','g',1000,0,60,0);
		INSERT INTO %s.product_bom_items(product_id,material_id,ratio_pct) VALUES (1,10,100.0000);
	`, schema, schema, schema, schema, schema, schema, schema, schema))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 10, 10, "MB-SNAPSHOT-A", "孟连水洗5T批次", 1000)

	app := newProductionFlowTestEcho(pool, schema)
	startBody := bytes.NewBufferString(`{"selected":["1-227"],"input_by_key":{"1-227":600}}`)
	startReq := httptest.NewRequest(http.MethodPost, "/api/produce/start", startBody)
	startReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	startRec := httptest.NewRecorder()
	app.ServeHTTP(startRec, startReq)
	if startRec.Code != http.StatusOK {
		t.Fatalf("POST /api/produce/start status=%d body=%s", startRec.Code, startRec.Body.String())
	}

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		DELETE FROM %s.product_bom_items WHERE product_id=1;
		INSERT INTO %s.product_bom_items(product_id,material_id,ratio_pct) VALUES (1,12,100.0000);
	`, schema, schema))

	var runningItemID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.produce_running_items WHERE product_id=1 AND spec_g=227 ORDER BY id DESC LIMIT 1`, schema)).Scan(&runningItemID); err != nil {
		t.Fatalf("query running item id: %v", err)
	}
	finishBody := bytes.NewBufferString(fmt.Sprintf(`{"id":%d,"finished_units":2,"finished_loose_g":10}`, runningItemID))
	finishReq := httptest.NewRequest(http.MethodPost, "/api/produce/running/finish", finishBody)
	finishReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	finishRec := httptest.NewRecorder()
	app.ServeHTTP(finishRec, finishReq)
	if finishRec.Code != http.StatusOK {
		t.Fatalf("POST /api/produce/running/finish status=%d body=%s", finishRec.Code, finishRec.Body.String())
	}

	var consumedMaterial string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(material_name,'')
		FROM %s.material_consumption_logs
		WHERE running_item_id=$1
		ORDER BY id
		LIMIT 1
	`, schema), runningItemID).Scan(&consumedMaterial); err != nil {
		t.Fatalf("query material consumption: %v", err)
	}
	if consumedMaterial != "孟连水洗5T批次" {
		t.Fatalf("consumed material = %q, want frozen 孟连水洗5T批次", consumedMaterial)
	}

	workOrders, err := postgresproduction.NewRepository(pool, schema).ListWorkOrders(ctx, productionapp.WorkOrderQuery{Limit: 10})
	if err != nil {
		t.Fatalf("ListWorkOrders: %v", err)
	}
	if len(workOrders) == 0 {
		t.Fatal("expected a work order")
	}
	if !strings.Contains(workOrders[0].MaterialSummary, "孟连水洗5T批次") || strings.Contains(workOrders[0].MaterialSummary, "后改B批次") {
		t.Fatalf("work order material summary = %q, want frozen material A only", workOrders[0].MaterialSummary)
	}
}

func TestProductionSourceStoresAndUsesMaterialSnapshot(t *testing.T) {
	schemaSrc, err := os.ReadFile("internal/infrastructure/postgres/production/schema.go")
	if err != nil {
		t.Fatal(err)
	}
	repoSrc, err := os.ReadFile("internal/infrastructure/postgres/production/repository.go")
	if err != nil {
		t.Fatal(err)
	}
	workOrderSrc, err := os.ReadFile("internal/infrastructure/postgres/production/work_order.go")
	if err != nil {
		t.Fatal(err)
	}
	consumptionSrc, err := os.ReadFile("internal/infrastructure/postgres/production/material_consumption.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		name string
		src  string
		want string
	}{
		{"schema running snapshot", string(schemaSrc), "produce_running_items ADD COLUMN IF NOT EXISTS material_snapshot JSONB NOT NULL DEFAULT '[]'::jsonb"},
		{"schema work order snapshot", string(schemaSrc), "work_orders ADD COLUMN IF NOT EXISTS material_snapshot JSONB NOT NULL DEFAULT '[]'::jsonb"},
		{"start builds snapshot", string(repoSrc), "buildMaterialSnapshotForRunningItemTx"},
		{"work order persists snapshot", string(workOrderSrc), "material_snapshot"},
		{"finish consumes snapshot", string(consumptionSrc), "materialSnapshotNeedsTx"},
	} {
		if !strings.Contains(check.src, check.want) {
			t.Fatalf("%s missing %q", check.name, check.want)
		}
	}
}

func TestProduceStartAPIUsesSubmittedInputG(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.products(id,name,default_price,active) VALUES (1,'曲奇拼配',50,true);
		INSERT INTO %s.order_process_statuses(name,sort,active) VALUES
			('待处理',10,true),
			('生产中',20,true)
		ON CONFLICT (name) DO NOTHING;
		INSERT INTO %s.orders(id,order_no,order_date,is_void,process_status_id) VALUES
			(1,'SO-API-START','2026-04-25',false,(SELECT id FROM %s.order_process_statuses WHERE name='待处理' LIMIT 1));
		INSERT INTO %s.order_items(order_id,line_no,item_name,qty,unit,spec,product_id,unit_price,line_total)
		VALUES (1,1,'曲奇拼配',1,'袋','1000g',1,50,50);
		INSERT INTO %s.product_bom(product_id,yield_rate) VALUES (1,0.8000);
		INSERT INTO %s.materials(id,code,name,kind,unit,onhand_g,onhand_units,purchase_price,sale_price)
			VALUES (10,'RAW-COOKIE','曲奇拼配生豆','bean','g',3000,0,54,0);
		INSERT INTO %s.product_bom_items(product_id,material_id,ratio_pct) VALUES (1,10,100.0000);
		`, schema, schema, schema, schema, schema, schema, schema, schema))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 10, 10, "MB-COOKIE", "曲奇拼配生豆", 3000)

	app := newProductionFlowTestEcho(pool, schema)
	body := bytes.NewBufferString(`{"selected":["1-1000"],"input_by_key":{"1-1000":2000}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/produce/start", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/produce/start status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	var inputG, plannedUnits, plannedLooseG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT input_g, planned_units, planned_loose_g
		FROM %s.produce_running_items
		WHERE product_id=1 AND spec_g=1000
		ORDER BY id DESC
		LIMIT 1
	`, schema)).Scan(&inputG, &plannedUnits, &plannedLooseG); err != nil {
		t.Fatalf("query running item: %v", err)
	}
	if inputG != 2000 {
		t.Fatalf("running item input_g = %d, want 2000", inputG)
	}
	if plannedUnits != 1 || plannedLooseG != 600 {
		t.Fatalf("running item plan = %d units + %dg, want 1 unit + 600g", plannedUnits, plannedLooseG)
	}
}

func TestProduceRunningAPIContract(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.produce_running_items(
			id,batch_id,product_id,product_name,spec_g,need_g,order_nos,status,
			started_by,started_at,input_g,bom_yield_rate,planned_units,planned_loose_g
		) VALUES (
			11,'BATCH-API-RUN',1,'橘皮乌龙',227,454,'SO-API-RUN','running',
			'测试员',now(),600,0.8200,2,38
		);
	`, schema))

	app := newProductionFlowTestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/produce/running", nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/produce/running status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	var got struct {
		Rows []struct {
			ID           int64   `json:"id"`
			BatchID      string  `json:"batch_id"`
			ProductName  string  `json:"product_name"`
			InputG       int64   `json:"input_g"`
			BomYieldRate float64 `json:"bom_yield_rate"`
		} `json:"rows"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode running API response: %v\n%s", err, rec.Body.String())
	}
	if len(got.Rows) != 1 {
		t.Fatalf("GET /api/produce/running rows = %d, want 1", len(got.Rows))
	}
	row := got.Rows[0]
	if row.ID != 11 || row.BatchID != "BATCH-API-RUN" || row.ProductName != "橘皮乌龙" || row.InputG != 600 || math.Abs(row.BomYieldRate-0.82) > 0.0001 {
		t.Fatalf("running API row = %+v", row)
	}
}

func TestProduceRunningVueRouteContract(t *testing.T) {
	appVue, err := os.ReadFile("frontend-vue-shell/src/App.vue")
	if err != nil {
		t.Fatal(err)
	}
	appContent := string(appVue)
	for _, want := range []string{
		"ProduceRunningView",
		"produceRunning: ProduceRunningView",
	} {
		if !strings.Contains(appContent, want) {
			t.Fatalf("App.vue missing running production Vue wiring %q", want)
		}
	}
	menuIA, err := os.ReadFile("frontend-vue-shell/src/lib/menu-ia.js")
	if err != nil {
		t.Fatal(err)
	}
	menuContent := string(menuIA)
	for _, want := range []string{"produceRunning", "生产中"} {
		if !strings.Contains(menuContent, want) {
			t.Fatalf("menu-ia.js missing running production menu wiring %q", want)
		}
	}

	view, err := os.ReadFile("frontend-vue-shell/src/views/ProduceRunningView.vue")
	if err != nil {
		t.Fatal(err)
	}
	viewContent := string(view)
	for _, want := range []string{"fetchRunningProduction", "finishRunningProduction", "cancelRunningProduction", "生产中", "完成生产", "取消生产"} {
		if !strings.Contains(viewContent, want) {
			t.Fatalf("ProduceRunningView.vue missing %q", want)
		}
	}

	api, err := os.ReadFile("frontend-vue-shell/src/api/production.js")
	if err != nil {
		t.Fatal(err)
	}
	apiContent := string(api)
	for _, want := range []string{"/api/produce/running", "/api/produce/running/finish", "/api/produce/running/cancel"} {
		if !strings.Contains(apiContent, want) {
			t.Fatalf("production api client missing %q", want)
		}
	}
}

func TestListRunningItemsBackfillsMissingInputAndPlan(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.produce_running_items(
			id,batch_id,product_id,product_name,spec_g,need_g,order_nos,status,
			started_by,started_at,input_g,bom_yield_rate,planned_units,planned_loose_g
		) VALUES (
			1,'BATCH-RUN-001',1,'橘皮乌龙',454,454,'SO-TEST-RUN','running',
			'测试员',now(),0,0.8000,0,0
		);
	`, schema))

	rows, err := postgresproduction.NewRepository(pool, schema).ListRunning(ctx)
	if err != nil {
		t.Fatalf("listRunningItems: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("listRunningItems rows = %d, want 1", len(rows))
	}
	if rows[0].InputG != 568 {
		t.Fatalf("listRunningItems input_g = %d, want 568", rows[0].InputG)
	}
	if rows[0].PlanUnits != 1 || rows[0].PlanLooseG != 0 {
		t.Fatalf("listRunningItems plan = %d units + %dg, want 1 unit + 0g", rows[0].PlanUnits, rows[0].PlanLooseG)
	}
}

func TestListRunningItemsRecomputesLegacyPlanFromInput(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.produce_running_items(
			id,batch_id,product_id,product_name,spec_g,need_g,order_nos,status,
			started_by,started_at,input_g,bom_yield_rate,planned_units,planned_loose_g
		) VALUES (
			1,'BATCH-RUN-LEGACY',1,'曲奇拼配',1000,1000,'SO-TEST-RUN','running',
			'测试员',now(),2000,0.8000,1,0
		);
	`, schema))

	rows, err := postgresproduction.NewRepository(pool, schema).ListRunning(ctx)
	if err != nil {
		t.Fatalf("listRunningItems: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("listRunningItems rows = %d, want 1", len(rows))
	}
	if rows[0].PlanUnits != 1 || rows[0].PlanLooseG != 600 {
		t.Fatalf("legacy running plan = %d units + %dg, want 1 unit + 600g", rows[0].PlanUnits, rows[0].PlanLooseG)
	}
}

func TestProduceRunningPageRedirectsToVueShellWithQueryError(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	app := newProductionFlowTestEcho(pool, schema)

	rec := serveProductionFlowForm(t, app, http.MethodGet, "/produce/running?err=input_g+must+be+greater+than+0", nil)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("GET /produce/running status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "vue-shell?view=produceRunning") || !strings.Contains(loc, "err=input_g+must+be+greater+than+0") {
		t.Fatalf("GET /produce/running Location = %q", loc)
	}
}

func newProductionFlowTestDB(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()

	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required for production flow API tests")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	schema := fmt.Sprintf("test_prod_flow_%d", time.Now().UnixNano())
	mustExecProductionFlowTestSQL(t, ctx, pool, "CREATE SCHEMA "+schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, productionFlowTestBaseDDL(schema))
	if err := postgrescompany.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("company EnsureSchema: %v", err)
	}
	if err := supporthttp.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("support EnsureSchema: %v", err)
	}
	if err := postgresmaterials.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("materials EnsureSchema: %v", err)
	}
	if err := postgresbom.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("bom EnsureSchema: %v", err)
	}
	if err := postgresstock.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("stock EnsureSchema: %v", err)
	}
	if err := postgresinventory.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("inventory EnsureSchema: %v", err)
	}
	if err := postgresproduction.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("production EnsureSchema: %v", err)
	}
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("sales EnsureSchema: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	})
	return pool, schema
}

func newProductionFlowTestEcho(pool *pgxpool.Pool, schema string) *echo.Echo {
	e := echo.New()
	e.Renderer = supporthttp.NewTemplateRenderer(template.Must(template.New("").Funcs(supporthttp.TemplateFuncMap()).ParseGlob("templates/*.html")))
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(1))
			c.Set("operator_employee", "测试员")
			c.Set("actor", "测试员")
			return next(c)
		}
	})
	productionSvc := productionapp.NewService(postgresproduction.NewRepository(pool, schema))
	registerUnprodSummaryAPI(e, productionSvc)
	registerProductionFlowPages(e, productionSvc)
	return e
}

func serveProductionFlowForm(t *testing.T, e *echo.Echo, method, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	body := ""
	if form != nil {
		body = form.Encode()
	}
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if form != nil {
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func mustExecProductionFlowTestSQL(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sql string) {
	t.Helper()
	if _, err := pool.Exec(ctx, sql); err != nil {
		t.Fatalf("exec sql: %v\n%s", err, sql)
	}
}

func seedProductionFlowWIPBatch(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string, batchID, materialID int64, batchCode, materialName string, qtyG int64) {
	t.Helper()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.material_batches(id,batch_code,material_id,material_name,received_g,qty_g,remaining_g,unit_cost,status,quality_status)
		VALUES($1,$2,$3,$4,$5,$5,$5,0,'active','pass')
		ON CONFLICT (id) DO UPDATE SET
			batch_code=excluded.batch_code,
			material_id=excluded.material_id,
			material_name=excluded.material_name,
			received_g=excluded.received_g,
			qty_g=excluded.qty_g,
			remaining_g=excluded.remaining_g,
			status='active',
			quality_status='pass';
	`, schema), batchID, batchCode, materialID, materialName, qtyG); err != nil {
		t.Fatalf("seed WIP material batch: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_g)
		VALUES($1,$2,$3,'wip',$4)
		ON CONFLICT (material_batch_id, warehouse) DO UPDATE SET
			batch_code=excluded.batch_code,
			material_id=excluded.material_id,
			qty_g=excluded.qty_g;
	`, schema), batchID, batchCode, materialID, qtyG); err != nil {
		t.Fatalf("seed WIP material location: %v", err)
	}
}

func productionFlowTestBaseDDL(schema string) string {
	return fmt.Sprintf(`
		CREATE TABLE %s.products (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			roast_level TEXT NOT NULL DEFAULT '',
			default_price NUMERIC NOT NULL DEFAULT 0,
			active BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TABLE %s.product_price_tiers (
			id BIGSERIAL PRIMARY KEY,
			product_id BIGINT REFERENCES %s.products(id) ON DELETE CASCADE,
			min_qty_lb NUMERIC,
			max_qty_lb NUMERIC,
			price_per_lb NUMERIC(12,2),
			active BOOLEAN NOT NULL DEFAULT true
		);
		CREATE TABLE %s.customers (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			company_name TEXT NOT NULL DEFAULT '',
			company_address TEXT NOT NULL DEFAULT '',
			company_phone TEXT NOT NULL DEFAULT '',
			contact TEXT NOT NULL DEFAULT '',
			phone TEXT NOT NULL DEFAULT '',
			address TEXT NOT NULL DEFAULT '',
			active BOOLEAN NOT NULL DEFAULT true
		);
		CREATE TABLE %s.order_process_statuses (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			sort INTEGER NOT NULL DEFAULT 0,
			active BOOLEAN NOT NULL DEFAULT true
		);
		CREATE TABLE %s.ship_statuses (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL
		);
		CREATE TABLE %s.orders (
			id BIGSERIAL PRIMARY KEY,
			order_no TEXT,
			order_date DATE,
			is_void BOOLEAN NOT NULL DEFAULT false,
			process_status_id INTEGER REFERENCES %s.order_process_statuses(id),
			ship_status_id BIGINT REFERENCES %s.ship_statuses(id),
			ship_tracking_no TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE %s.order_items (
			id BIGSERIAL PRIMARY KEY,
			order_id BIGINT REFERENCES %s.orders(id) ON DELETE CASCADE,
			line_no INTEGER,
			item_name TEXT,
			item_note TEXT NOT NULL DEFAULT '',
			qty NUMERIC,
			unit TEXT,
			spec TEXT,
			product_id BIGINT REFERENCES %s.products(id),
			unit_price NUMERIC,
			line_total NUMERIC,
			price_overridden BOOLEAN NOT NULL DEFAULT false
		);
		CREATE TABLE %s.customer_processing_production_demands (
			id BIGSERIAL PRIMARY KEY,
			request_no TEXT NOT NULL DEFAULT '',
			customer_id BIGINT NOT NULL DEFAULT 0,
			product_id BIGINT NOT NULL DEFAULT 0,
			product_name TEXT NOT NULL DEFAULT '',
			spec_g BIGINT NOT NULL DEFAULT 0,
			target_qty BIGINT NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'planned',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
	`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema)
}
