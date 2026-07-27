package production

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	productionapp "orderapp/internal/application/production"
	postgresproduction "orderapp/internal/infrastructure/postgres/production"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func TestProducePlanSummaryAPIIncludesRoastRowsAndMaterials(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.products(
			id,name,default_price,active,spec_label,net_content_qty,net_content_unit,unit_rule_override_json
		) VALUES (1,'曲奇拼配',50,true,'1000g',1000,'g','{"inventory_unit":"kg"}'::jsonb);
		INSERT INTO %s.order_process_statuses(name,sort,active) VALUES ('待处理',10,true)
		ON CONFLICT (name) DO NOTHING;
		INSERT INTO %s.orders(id,order_no,order_date,is_void,process_status_id) VALUES (1,'SO-PLAN-001','2026-04-25',false,(SELECT id FROM %s.order_process_statuses WHERE name='待处理' LIMIT 1));
		INSERT INTO %s.order_items(order_id,line_no,item_name,qty,unit,spec,product_id,unit_price,line_total)
		VALUES (1,1,'曲奇拼配',1,'袋','1000g',1,50,50);
		INSERT INTO %s.product_bom(product_id,yield_rate) VALUES (1,0.8000);
		INSERT INTO %s.materials(id,code,name,kind,unit,onhand_g,onhand_units,purchase_price,sale_price)
		VALUES
			(10,'RAW-A','豆子A','bean','g',0,0,10,0),
			(11,'RAW-B','豆子B','bean','g',0,0,10,0);
		INSERT INTO %s.product_bom_items(product_id,material_id,ratio_pct) VALUES
			(1,10,0.7500),
			(1,11,0.2500);
		INSERT INTO %s.roast_machines(name,capacity_g,allowed_specs,min_roast_g,active)
		VALUES ('样机',2000,'2000',1000,true);
	`, schema, schema, schema, schema, schema, schema, schema, schema, schema))

	e := newProducePlanTestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/produce/unproduced?selected=1-1000&plan=1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/produce/unproduced status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, needle := range []string{`"roast_plans"`, `"materials"`, `"plan_rows"`, `"final_input_g":2000`, `"qty":938`, `"qty":313`, `"bom_summary_error":"product BOM not configured: 曲奇拼配"`} {
		if !strings.Contains(body, needle) {
			t.Fatalf("response missing %s: %s", needle, body)
		}
	}
}

func TestProducePlanSummaryAPIUsesLatestDefaultProductionBomMaterials(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedThreeBeanProductionBomDemand(t, ctx, pool, schema)

	e := newProducePlanTestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/produce/unproduced?selected=556-454&plan=1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/produce/unproduced status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Materials []productionapp.MaterialNeed `json:"materials"`
		PlanRows  []struct {
			BomMaterialLossRate float64 `json:"bom_material_loss_rate"`
			BomSummaryError     string  `json:"bom_summary_error"`
		} `json:"plan_rows"`
		RoastPlans []struct {
			YieldRate   float64 `json:"yield_rate"`
			FinalInputG int64   `json:"final_input_g"`
		} `json:"roast_plans"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v\n%s", err, rec.Body.String())
	}
	assertAPIPlanMaterial(t, payload.Materials, "哥伦比亚EP", 228, "g")
	assertAPIPlanMaterial(t, payload.Materials, "孟连水洗A", 568, "g")
	assertAPIPlanMaterial(t, payload.Materials, "生豆-巴布亚之光-石光", 342, "g")
	if len(payload.RoastPlans) == 0 {
		t.Fatalf("roast_plans empty: %s", rec.Body.String())
	}
	if payload.RoastPlans[0].YieldRate != 1 || payload.RoastPlans[0].FinalInputG != 2000 {
		t.Fatalf("roast plan = %+v, want no-loss BOM yield 1 and machine-rounded final_input_g 2000", payload.RoastPlans[0])
	}
	if len(payload.PlanRows) != 1 || payload.PlanRows[0].BomMaterialLossRate != 0 {
		t.Fatalf("no-loss BOM plan row = %+v, want explicit zero loss", payload.PlanRows)
	}
	if payload.PlanRows[0].BomSummaryError != "" {
		t.Fatalf("valid no-loss BOM must not report a summary error: %+v", payload.PlanRows[0])
	}
	if !strings.Contains(rec.Body.String(), `"bom_material_loss_rate":0`) {
		t.Fatalf("response must preserve explicit no-loss BOM marker: %s", rec.Body.String())
	}
}

func TestProductionPlanAPIDetailKeepsLatestDefaultProductionBomMaterialSnapshot(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedThreeBeanProductionBomDemand(t, ctx, pool, schema)

	e := newProducePlanTestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodPost, "/api/production-plans", strings.NewReader(`{"selected":["556-454"],"source_type":"erp_order"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/production-plans status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var detail productionapp.ProductionPlanDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("unmarshal plan detail: %v\n%s", err, rec.Body.String())
	}
	if len(detail.Items) != 1 {
		t.Fatalf("plan items = %+v, want one item", detail.Items)
	}
	var snapshotRows []struct {
		MaterialName string `json:"material_name"`
	}
	if err := json.Unmarshal([]byte(detail.Items[0].MaterialSnapshot), &snapshotRows); err != nil {
		t.Fatalf("unmarshal material snapshot: %v\n%s", err, detail.Items[0].MaterialSnapshot)
	}
	snapshotNames := map[string]bool{}
	for _, row := range snapshotRows {
		snapshotNames[row.MaterialName] = true
	}
	for _, want := range []string{"哥伦比亚EP", "孟连水洗A", "生豆-巴布亚之光-石光"} {
		if !snapshotNames[want] {
			t.Fatalf("material snapshot missing %s: %+v", want, snapshotRows)
		}
	}
	assertAPIPlanMaterial(t, detail.MaterialSummary, "哥伦比亚EP", 228, "g")
	assertAPIPlanMaterial(t, detail.MaterialSummary, "孟连水洗A", 568, "g")
	assertAPIPlanMaterial(t, detail.MaterialSummary, "生豆-巴布亚之光-石光", 342, "g")
}

func TestParseUnprodSummaryQueryIncludesDemandStatusFilter(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/produce/unproduced?demand_status=in_production&selected=1-454&plan=1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	query := parseUnprodSummaryQuery(c)
	if query.DemandStatus != "in_production" {
		t.Fatalf("DemandStatus = %q, want in_production", query.DemandStatus)
	}
	if !query.Plan || !query.Selected["1-454"] {
		t.Fatalf("query did not preserve plan preview selection: %+v", query)
	}
}

func seedThreeBeanProductionBomDemand(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) {
	t.Helper()
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.products(
			id,name,default_price,active,spec_label,net_content_qty,net_content_unit,unit_rule_override_json
		) VALUES (556,'熟豆-白巧坚果拼配',50,true,'454g',454,'g','{"inventory_unit":"kg"}'::jsonb);
		INSERT INTO %[1]s.order_process_statuses(name,sort,active) VALUES ('待处理',10,true)
		ON CONFLICT (name) DO NOTHING;
		INSERT INTO %[1]s.orders(id,order_no,order_date,is_void,process_status_id)
		VALUES (556,'SO-WHITE-NUT-001','2026-06-26',false,(SELECT id FROM %[1]s.order_process_statuses WHERE name='待处理' LIMIT 1));
		INSERT INTO %[1]s.order_items(order_id,line_no,item_name,qty,unit,spec,product_id,unit_price,line_total)
		VALUES (556,1,'熟豆-白巧坚果拼配',2,'袋','454g',556,50,100);
		INSERT INTO %[1]s.roast_machines(name,capacity_g,allowed_specs,min_roast_g,active)
		VALUES ('测试烘焙机',2000,'2000',1000,true);
		INSERT INTO %[1]s.process_routes(id,name,status,default_equipment,default_minutes)
		VALUES (556,'白巧坚果生产路线','active','测试烘焙机',20);
		INSERT INTO %[1]s.process_route_operations(route_id,seq,operation,workstation,default_equipment,default_minutes,records_loss)
		VALUES (556,1,'烘焙','烘焙工位','测试烘焙机',20,true);
		INSERT INTO %[1]s.materials(id,code,name,kind,unit,onhand_g,onhand_units,purchase_price,sale_price)
		VALUES
			(5561,'RAW-COLOMBIA','哥伦比亚EP','bean','g',0,0,10,0),
			(5562,'RAW-MENGLIAN','孟连水洗A','bean','g',0,0,10,0),
			(5563,'RAW-PAPUA','生豆-巴布亚之光-石光','bean','g',0,0,10,0);
		INSERT INTO %[1]s.production_boms(id,code,name,output_product_id,status,updated_at)
		VALUES (556,'BOM-000556','熟豆-白巧坚果拼配 BOM',556,'active','2026-06-01 00:00:00+00');
		INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,yield_rate,output_qty,output_unit,process_route_id,created_at,published_at)
		VALUES
			(959,556,'V001','archived',0.9000,454,'g',556,'2026-06-01 00:00:00+00','2026-06-01 00:00:00+00'),
			(961,556,'V002','published',0.8000,454,'g',556,'2026-06-02 00:00:00+00','2026-06-02 00:00:00+00');
		INSERT INTO %[1]s.production_bom_version_items(version_id,material_id,component_type,consume_unit,qty_per_unit,ratio_pct)
		VALUES
			(959,5563,'material','ratio_pct',0,100),
			(961,5561,'material','g',114,0),
			(961,5562,'material','g',284,0),
			(961,5563,'material','g',171,0);
		INSERT INTO %[1]s.product_production_bom_bindings(product_id,bom_id,bom_version_id,bound_by)
		VALUES (556,556,961,'test');
		INSERT INTO %[1]s.product_production_configs(product_id,production_bom_id,production_bom_version_id,process_route_id,expected_loss_rate,created_by,updated_by)
		VALUES (556,556,961,556,0,'test','test')
		ON CONFLICT (product_id) DO UPDATE SET
			production_bom_id=excluded.production_bom_id,
			production_bom_version_id=excluded.production_bom_version_id,
			process_route_id=excluded.process_route_id,
			expected_loss_rate=excluded.expected_loss_rate;
	`, schema))
}

func assertAPIPlanMaterial(t *testing.T, rows []productionapp.MaterialNeed, name string, qty int64, unit string) {
	t.Helper()
	for _, row := range rows {
		if row.Name == name {
			if row.Qty != qty || row.Unit != unit {
				t.Fatalf("material %s = %+v, want qty=%d unit=%s", name, row, qty, unit)
			}
			return
		}
	}
	t.Fatalf("material summary missing %s in %+v", name, rows)
}

func TestProducePlanSummaryAPIMarksPlannedDemandAsInProductionAndFiltersIt(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.products(
			id,name,default_price,active,spec_label,net_content_qty,net_content_unit,unit_rule_override_json
		) VALUES (1,'PR491 商品',50,true,'454g',454,'g','{"inventory_unit":"kg"}'::jsonb);
		INSERT INTO %s.order_process_statuses(name,sort,active) VALUES ('待处理',10,true),('生产中',20,true),('生产完成',30,true)
		ON CONFLICT (name) DO NOTHING;
		INSERT INTO %s.orders(id,order_no,order_date,is_void,process_status_id) VALUES (1,'SO-PR491-001','2026-06-13',false,(SELECT id FROM %s.order_process_statuses WHERE name='待处理' LIMIT 1));
		INSERT INTO %s.order_items(order_id,line_no,item_name,qty,unit,spec,product_id,unit_price,line_total)
		VALUES (1,1,'PR491 商品',2,'袋','454g',1,50,100);
		INSERT INTO %s.production_plans(id,plan_no,source_type,status,created_by,created_at)
		VALUES (491,'PP-PR491','erp_order','draft','tester',now());
		INSERT INTO %s.production_plan_items(id,production_plan_id,product_id,product_name,spec_g,planned_g,planned_output_g,gap_g,order_nos,component_snapshot_json,process_route_snapshot_json,production_config_snapshot_json,customer_product_snapshot_json,created_at)
		VALUES (492,491,1,'PR491 商品',454,908,908,908,'SO-PR491-001','[]'::jsonb,'{}'::jsonb,'{}'::jsonb,'[]'::jsonb,now());
	`, schema, schema, schema, schema, schema, schema, schema))

	e := newProducePlanTestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/produce/unproduced?demand_status=in_production", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/produce/unproduced status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, needle := range []string{`"demand_status":"in_production"`, `"demand_status_label":"生产中"`, `"production_plan_no":"PP-PR491"`, `"demand_selectable":false`} {
		if !strings.Contains(body, needle) {
			t.Fatalf("planned demand response missing %s: %s", needle, body)
		}
	}
}

func TestProducePlanSummaryAPILeavesAddOnOrdersSelectableWhenOlderOrdersPlanned(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.products(
			id,name,default_price,active,spec_label,net_content_qty,net_content_unit,unit_rule_override_json
		) VALUES (1,'PR492 加单商品',50,true,'454g',454,'g','{"inventory_unit":"kg"}'::jsonb);
		INSERT INTO %s.order_process_statuses(name,sort,active) VALUES ('待处理',10,true)
		ON CONFLICT (name) DO NOTHING;
		INSERT INTO %s.orders(id,order_no,order_date,is_void,process_status_id) VALUES
			(1,'SO-ADD-OLD','2026-06-13',false,(SELECT id FROM %s.order_process_statuses WHERE name='待处理' LIMIT 1)),
			(2,'SO-ADD-NEW','2026-06-13',false,(SELECT id FROM %s.order_process_statuses WHERE name='待处理' LIMIT 1));
		INSERT INTO %s.order_items(order_id,line_no,item_name,qty,unit,spec,product_id,unit_price,line_total)
		VALUES
			(1,1,'PR492 加单商品',2,'袋','454g',1,50,100),
			(2,1,'PR492 加单商品',3,'袋','454g',1,50,150);
		INSERT INTO %s.production_plans(id,plan_no,source_type,status,created_by,created_at)
		VALUES (4921,'PP-ADD-OLD','erp_order','draft','tester',now());
		INSERT INTO %s.production_plan_items(id,production_plan_id,product_id,product_name,spec_g,planned_g,planned_output_g,gap_g,order_nos,component_snapshot_json,process_route_snapshot_json,production_config_snapshot_json,customer_product_snapshot_json,created_at)
		VALUES (4922,4921,1,'PR492 加单商品',454,908,908,908,'SO-ADD-OLD','[]'::jsonb,'{}'::jsonb,'{}'::jsonb,'[]'::jsonb,now());
	`, schema, schema, schema, schema, schema, schema, schema, schema))

	e := newProducePlanTestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/produce/unproduced?from=2026-06-13&to=2026-06-13&selected=1-454&plan=1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/produce/unproduced status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Rows     []productionapp.UnprodNeedRow `json:"rows"`
		PlanRows []struct {
			productionapp.UnprodNeedRow
			InputG int64 `json:"input_g"`
		} `json:"plan_rows"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
	}
	var oldRow, addOnRow *productionapp.UnprodNeedRow
	for i := range payload.Rows {
		row := &payload.Rows[i]
		switch row.OrderNos {
		case "SO-ADD-OLD":
			oldRow = row
		case "SO-ADD-NEW":
			addOnRow = row
		}
	}
	if oldRow == nil || addOnRow == nil {
		t.Fatalf("rows should split planned and add-on orders, got %+v", payload.Rows)
	}
	if oldRow.DemandStatus != "in_production" || oldRow.DemandSelectable {
		t.Fatalf("old planned row status/selectable = %s/%v, want in_production/false", oldRow.DemandStatus, oldRow.DemandSelectable)
	}
	if addOnRow.DemandStatus != "unplanned" || !addOnRow.DemandSelectable || addOnRow.NeedUnits != 3 || addOnRow.GapG != 1362 {
		t.Fatalf("add-on row = %+v, want unplanned selectable 3 units 1362g gap", addOnRow)
	}
	if len(payload.PlanRows) != 1 || payload.PlanRows[0].OrderNos != "SO-ADD-NEW" || payload.PlanRows[0].NeedG != 1362 {
		t.Fatalf("plan rows should only include add-on demand, got %+v", payload.PlanRows)
	}
}

func TestProducePlanSummaryAPIReturnsExactYieldRateForRoastPlans(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.products(
			id,name,default_price,active,spec_label,net_content_qty,net_content_unit,unit_rule_override_json
		) VALUES (1,'曲奇拼配',50,true,'1000g',1000,'g','{"inventory_unit":"kg"}'::jsonb);
		INSERT INTO %s.order_process_statuses(name,sort,active) VALUES ('待处理',10,true)
		ON CONFLICT (name) DO NOTHING;
		INSERT INTO %s.orders(id,order_no,order_date,is_void,process_status_id) VALUES (1,'SO-PLAN-002','2026-04-25',false,(SELECT id FROM %s.order_process_statuses WHERE name='待处理' LIMIT 1));
		INSERT INTO %s.order_items(order_id,line_no,item_name,qty,unit,spec,product_id,unit_price,line_total)
		VALUES (1,1,'曲奇拼配',1,'袋','1000g',1,50,50);
		INSERT INTO %s.product_bom(product_id,yield_rate) VALUES (1,0.8150);
		INSERT INTO %s.roast_machines(name,capacity_g,allowed_specs,min_roast_g,active)
		VALUES ('样机',2000,'2000',1000,true);
	`, schema, schema, schema, schema, schema, schema, schema))

	e := newProducePlanTestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/produce/unproduced?selected=1-1000&plan=1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/produce/unproduced status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		RoastPlans []struct {
			YieldRate float64 `json:"yield_rate"`
		} `json:"roast_plans"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(payload.RoastPlans) == 0 {
		t.Fatalf("roast_plans empty: %s", rec.Body.String())
	}
	if payload.RoastPlans[0].YieldRate != 0.815 {
		t.Fatalf("roast plan yield_rate = %.4f, want 0.8150", payload.RoastPlans[0].YieldRate)
	}
}

func TestProducePlanSummaryAPIReturnsConfiguredBomMaterialLossRate(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.products(
			id,name,parent_product_id,default_price,active,spec_label,net_content_qty,net_content_unit,unit_rule_override_json
		) VALUES
			(1,'损耗摘要父商品',0,50,true,'1000g',1000,'g','{"inventory_unit":"kg"}'::jsonb),
			(2,'损耗摘要测试商品',1,50,true,'1000g',1000,'g','{"inventory_unit":"kg"}'::jsonb);
		INSERT INTO %[1]s.order_process_statuses(name,sort,active) VALUES ('待处理',10,true)
		ON CONFLICT (name) DO NOTHING;
		INSERT INTO %[1]s.orders(id,order_no,order_date,is_void,process_status_id)
		VALUES (1,'SO-BOM-LOSS-001','2026-07-27',false,(SELECT id FROM %[1]s.order_process_statuses WHERE name='待处理' LIMIT 1));
		INSERT INTO %[1]s.order_items(order_id,line_no,item_name,qty,unit,spec,product_id,unit_price,line_total)
		VALUES (1,1,'损耗摘要测试商品',1,'件','1000g',2,50,50);
		INSERT INTO %[1]s.materials(id,code,name,kind,unit,onhand_g,onhand_units,purchase_price,sale_price)
		VALUES (10,'RAW-BOM-LOSS','损耗测试原料','bean','kg',0,0,10,0);
		INSERT INTO %[1]s.production_boms(id,code,name,output_product_id,status)
		VALUES (1,'BOM-LOSS-001','损耗摘要测试 BOM',1,'active');
		INSERT INTO %[1]s.process_routes(id,name,status)
		VALUES (1,'损耗摘要测试路线','active');
		INSERT INTO %[1]s.production_bom_versions(
			id,bom_id,version_no,status,yield_rate,output_qty,output_unit,material_loss_rate,process_route_id,published_at
		) VALUES (1,1,'V001','published',1,1,'kg',0.2,1,'2026-07-27 00:00:00+00');
		INSERT INTO %[1]s.production_bom_version_items(
			version_id,material_id,component_type,consume_unit,qty_per_unit,ratio_pct,material_loss_rate
		) VALUES (1,10,'material','ratio_pct',0,100,0);
	`, schema))

	e := newProducePlanTestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/produce/unproduced?selected=2-1000&plan=1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/produce/unproduced status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		PlanRows []struct {
			BomMaterialLossRate float64 `json:"bom_material_loss_rate"`
			BomSummaryError     string  `json:"bom_summary_error"`
		} `json:"plan_rows"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(payload.PlanRows) != 1 {
		t.Fatalf("plan_rows = %d, want 1: %s", len(payload.PlanRows), rec.Body.String())
	}
	if payload.PlanRows[0].BomMaterialLossRate != 0.2 {
		t.Fatalf("bom_material_loss_rate = %.4f, want 0.2000: %s", payload.PlanRows[0].BomMaterialLossRate, rec.Body.String())
	}
	if payload.PlanRows[0].BomSummaryError != "" {
		t.Fatalf("inherited valid BOM must not report a summary error: %+v", payload.PlanRows[0])
	}
}

func TestProducePlanSummaryAPIUsesInheritedPublishedBomLossOnceWithoutMachineRounding(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedRumuParentBomLossDemand(t, ctx, pool, schema)

	e := newProducePlanTestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/produce/unproduced?from=2026-07-27&to=2026-07-27&selected=789-454&plan=1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/produce/unproduced status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		PlanRows []struct {
			InputG              int64   `json:"input_g"`
			BomMaterialLossRate float64 `json:"bom_material_loss_rate"`
			BomSummaryError     string  `json:"bom_summary_error"`
		} `json:"plan_rows"`
		Materials []productionapp.MaterialNeed `json:"materials"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v\n%s", err, rec.Body.String())
	}
	if len(payload.PlanRows) != 1 {
		t.Fatalf("plan_rows = %d, want 1: %s", len(payload.PlanRows), rec.Body.String())
	}
	row := payload.PlanRows[0]
	if row.BomSummaryError != "" || math.Abs(row.BomMaterialLossRate-0.18) > 0.000000001 {
		t.Fatalf("resolved inherited BOM summary = %+v, want published V004 loss 18%%", row)
	}
	if row.InputG != 7751 {
		t.Fatalf("plan input_g = %d, want round(14*454/(1-0.18)) = 7751", row.InputG)
	}
	var material *productionapp.MaterialNeed
	for i := range payload.Materials {
		if payload.Materials[i].Name == "如目达摩生豆" {
			material = &payload.Materials[i]
			break
		}
	}
	if material == nil || material.Unit != "g" || material.Qty != 7751 || material.ExactQty != 7751 {
		t.Fatalf("material demand = %+v in %+v, want 7751g with BOM loss applied exactly once", material, payload.Materials)
	}
}

func seedRumuParentBomLossDemand(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) {
	t.Helper()
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.products(
			id,name,parent_product_id,default_price,active,spec_label,net_content_qty,net_content_unit,unit_rule_override_json
		) VALUES
			(644,'如目达摩',0,0,true,'',0,'','{"inventory_unit":"kg"}'::jsonb),
			(789,'如目达摩',644,0,true,'454g',454,'g','{}'::jsonb);
		INSERT INTO %[1]s.order_process_statuses(name,sort,active)
		VALUES ('待处理',10,true) ON CONFLICT (name) DO NOTHING;
		INSERT INTO %[1]s.orders(id,order_no,order_date,is_void,process_status_id)
		VALUES (
			55701,'SO-PR557-RUMU','2026-07-27',false,
			(SELECT id FROM %[1]s.order_process_statuses WHERE name='待处理' LIMIT 1)
		);
		INSERT INTO %[1]s.order_items(
			order_id,line_no,item_name,qty,unit,sales_unit,spec,product_id,unit_price,line_total,price_source_json
		) VALUES (
			55701,1,'如目达摩',14,'454g','454g','454g',789,0,0,
			'{"production_quantity_snapshot":{"sku_id":789,"parent_product_id":644,"spec_label":"454g","sales_unit":"454g","inventory_unit":"kg","inventory_qty_per_sales_unit":0.454,"conversion_source":"published_inventory_conversion"}}'::jsonb
		);
		INSERT INTO %[1]s.materials(id,code,name,kind,unit,onhand_g,onhand_units,purchase_price,sale_price)
		VALUES (55701,'RAW-PR557-RUMU','如目达摩生豆','bean','g',0,0,54,0);
		INSERT INTO %[1]s.process_routes(id,name,status,default_equipment,default_minutes)
		VALUES (55701,'如目达摩标准烘焙','active','智烘',20);
		INSERT INTO %[1]s.process_route_operations(
			route_id,seq,operation,workstation,default_equipment,default_minutes,records_loss
		) VALUES (55701,1,'烘焙','烘焙中心','智烘',20,true);
		INSERT INTO %[1]s.production_boms(id,code,name,output_product_id,status)
		VALUES (55701,'BOM-000644','如目达摩 生产 BOM',644,'active');
		INSERT INTO %[1]s.production_bom_versions(
			id,bom_id,version_no,status,yield_rate,output_qty,output_unit,material_loss_rate,process_route_id,published_at
		) VALUES (55704,55701,'V004','published',0.8,1,'kg',0.18,55701,'2026-07-27 00:00:00+00');
		INSERT INTO %[1]s.production_bom_version_items(
			version_id,material_id,component_type,consume_unit,ratio_pct,material_loss_rate
		) VALUES (55704,55701,'material','ratio_pct',100,0.18);
		INSERT INTO %[1]s.product_production_bom_bindings(product_id,bom_id,bom_version_id,bound_by)
		VALUES (644,55701,55704,'test');
		INSERT INTO %[1]s.product_production_configs(
			product_id,production_bom_id,production_bom_version_id,process_route_id,expected_loss_rate,note,created_by,updated_by
		) VALUES
			(644,55701,55704,55701,0.2,'legacy-backfill','test','test'),
			(789,0,0,0,0.2,'legacy-backfill','test','test')
		ON CONFLICT (product_id) DO UPDATE SET
			expected_loss_rate=excluded.expected_loss_rate,
			note=excluded.note,
			updated_by=excluded.updated_by;
		INSERT INTO %[1]s.roast_machines(name,capacity_g,allowed_specs,min_roast_g,active)
		VALUES ('智烘',4000,'2000,4000',2000,true);
	`, schema))
}

func TestProducePlanTreatsDeclinedStockBatchDecisionAsProductionGap(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.products(
			id,name,default_price,active,spec_label,net_content_qty,net_content_unit,unit_rule_override_json
		) VALUES (1,'曲奇拼配',50,true,'454g',454,'g','{"inventory_unit":"kg"}'::jsonb);
		INSERT INTO %s.order_process_statuses(name,sort,active) VALUES ('待处理',10,true)
		ON CONFLICT (name) DO NOTHING;
		INSERT INTO %s.orders(id,order_no,order_date,is_void,process_status_id) VALUES (1,'SO-DECLINE-STOCK','2026-05-03',false,(SELECT id FROM %s.order_process_statuses WHERE name='待处理' LIMIT 1));
		INSERT INTO %s.order_items(order_id,line_no,item_name,qty,unit,spec,product_id,unit_price,line_total)
		VALUES (1,1,'曲奇拼配',2,'袋','454g',1,50,100);
		INSERT INTO %s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g) VALUES (1,454,'finished_goods',5,0);
		INSERT INTO %s.order_stock_decisions(order_id,decision,operator) VALUES (1,'produce','测试员');
		INSERT INTO %s.product_bom(product_id,yield_rate) VALUES (1,0.8000);
		INSERT INTO %s.roast_machines(name,capacity_g,allowed_specs,min_roast_g,active)
		VALUES ('样机',2000,'2000',1000,true);
	`, schema, schema, schema, schema, schema, schema, schema, schema, schema))

	e := newProducePlanTestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/produce/unproduced?selected=1-454&plan=1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/produce/unproduced status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, needle := range []string{`"order_nos":"SO-DECLINE-STOCK"`, `"need_g":908`, `"inv_g":2270`, `"gap_g":908`, `"plan_rows"`, `"roast_plans"`} {
		if !strings.Contains(body, needle) {
			t.Fatalf("declined stock plan response missing %s: %s", needle, body)
		}
	}
	if strings.Contains(body, "库存充足") {
		t.Fatalf("declined stock plan should not return stock sufficient tip: %s", body)
	}
}

func TestProducePlanExcludesShippedOrdersWithBlankProcessStatus(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.products(
			id,name,default_price,active,spec_label,net_content_qty,net_content_unit,unit_rule_override_json
		) VALUES (1,'曜石2.0',50,true,'454g',454,'g','{"inventory_unit":"kg"}'::jsonb);
		INSERT INTO %s.order_process_statuses(name,sort,active) VALUES ('待处理',10,true)
		ON CONFLICT (name) DO NOTHING;
		INSERT INTO %s.ship_statuses(name) VALUES ('未发货'),('已发货');
		ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS ship_status_id BIGINT;
		INSERT INTO %s.orders(id,order_no,order_date,is_void,process_status_id,ship_status_id) VALUES
			(1,'SO-SHIPPED-BLANK','2026-03-28',false,NULL,(SELECT id FROM %s.ship_statuses WHERE name='已发货' LIMIT 1)),
			(2,'SO-DECLINE-STOCK','2026-05-03',false,NULL,(SELECT id FROM %s.ship_statuses WHERE name='未发货' LIMIT 1));
		INSERT INTO %s.order_items(order_id,line_no,item_name,qty,unit,spec,product_id,unit_price,line_total)
		VALUES
			(1,1,'曜石2.0',80,'袋','454g',1,50,4000),
			(2,1,'曜石2.0',2,'袋','454g',1,50,100);
		INSERT INTO %s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g) VALUES (1,454,'finished_goods',170,0);
		INSERT INTO %s.order_stock_decisions(order_id,decision,operator) VALUES (2,'produce','测试员');
		INSERT INTO %s.product_bom(product_id,yield_rate) VALUES (1,0.8000);
		INSERT INTO %s.roast_machines(name,capacity_g,allowed_specs,min_roast_g,active)
		VALUES ('样机',2000,'2000',1000,true);
	`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema))

	e := newProducePlanTestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/produce/unproduced?selected=1-454&plan=1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/produce/unproduced status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, needle := range []string{`"order_nos":"SO-DECLINE-STOCK"`, `"need_units":2`, `"need_g":908`, `"gap_g":908`} {
		if !strings.Contains(body, needle) {
			t.Fatalf("response missing %s: %s", needle, body)
		}
	}
	if strings.Contains(body, "SO-SHIPPED-BLANK") || strings.Contains(body, `"need_units":82`) {
		t.Fatalf("shipped blank-process order should not inflate production need: %s", body)
	}
}

func TestProducePlanSkipsOrderItemsWithoutProductID(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.products(
			id,name,default_price,active,spec_label,net_content_qty,net_content_unit,unit_rule_override_json
		) VALUES (1,'曲奇拼配',50,true,'454g',454,'g','{"inventory_unit":"kg"}'::jsonb);
		INSERT INTO %s.order_process_statuses(name,sort,active) VALUES ('待处理',10,true)
		ON CONFLICT (name) DO NOTHING;
		INSERT INTO %s.orders(id,order_no,order_date,is_void,process_status_id) VALUES
			(1,'SO-UNLINKED-ITEM','2026-05-10',false,(SELECT id FROM %s.order_process_statuses WHERE name='待处理' LIMIT 1)),
			(2,'SO-LINKED-ITEM','2026-05-10',false,(SELECT id FROM %s.order_process_statuses WHERE name='待处理' LIMIT 1));
		INSERT INTO %s.order_items(order_id,line_no,item_name,qty,unit,spec,product_id,unit_price,line_total)
		VALUES
			(1,1,'历史未绑定商品',3,'袋','100g/袋',NULL,50,150),
			(2,1,'曲奇拼配',2,'袋','454g',1,50,100);
		INSERT INTO %s.product_bom(product_id,yield_rate) VALUES (1,0.8000);
		INSERT INTO %s.roast_machines(name,capacity_g,allowed_specs,min_roast_g,active)
		VALUES ('样机',2000,'2000',1000,true);
	`, schema, schema, schema, schema, schema, schema, schema, schema))

	e := newProducePlanTestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/produce/unproduced?plan=1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/produce/unproduced status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, needle := range []string{`"order_nos":"SO-LINKED-ITEM"`, `"product_id":1`, `"need_units":2`, `"gap_g":908`} {
		if !strings.Contains(body, needle) {
			t.Fatalf("response missing %s: %s", needle, body)
		}
	}
	if strings.Contains(body, "SO-UNLINKED-ITEM") || strings.Contains(body, "历史未绑定商品") {
		t.Fatalf("unlinked historical order item should not enter production plan: %s", body)
	}
}

func TestProducePlanIncludesInProgressOrdersWithRemainingItems(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.products(
			id,name,default_price,active,product_kind,drip_bag_grams,
			spec_label,net_content_qty,net_content_unit,unit_rule_override_json
		) VALUES
			(1,'GoalE2E 熟豆',50,true,'roasted_bean',0,'227g',227,'g','{"inventory_unit":"kg"}'::jsonb),
			(2,'GoalE2E 挂耳',5,true,'drip_bag',10,'10g',10,'g','{"inventory_unit":"kg"}'::jsonb);
		INSERT INTO %s.order_process_statuses(name,sort,active) VALUES ('生产中',20,true)
		ON CONFLICT (name) DO NOTHING;
		INSERT INTO %s.orders(id,order_no,order_date,is_void,process_status_id) VALUES
			(1,'SO-IN-PROGRESS-REMAINING','2026-06-05',false,(SELECT id FROM %s.order_process_statuses WHERE name='生产中' LIMIT 1));
		INSERT INTO %s.order_items(
			order_id,line_no,item_name,qty,unit,spec,product_id,unit_price,line_total,
			product_kind,sales_unit,unit_bag_count,unit_bean_g
		) VALUES
			(1,1,'GoalE2E 熟豆',2,'袋','227g',1,58,116,'roasted','bag',0,0),
			(1,2,'GoalE2E 挂耳',20,'袋','10g/袋',2,5,100,'drip_bag','bag',1,10);
	`, schema, schema, schema, schema, schema))

	e := newProducePlanTestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/produce/unproduced?from=2026-06-05&to=2026-06-05", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/produce/unproduced status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, needle := range []string{`"order_nos":"SO-IN-PROGRESS-REMAINING"`, `"product_id":1`, `"product_id":2`, `"gap_g":454`, `"gap_g":200`} {
		if !strings.Contains(body, needle) {
			t.Fatalf("in-progress remaining plan response missing %s: %s", needle, body)
		}
	}
}

func TestProducePlanSummaryAPIDripBagBoxCreatesDripDemandAndUpstreamShortage(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.products(
			id,name,default_price,active,product_kind,drip_bag_grams,drip_box_bag_count,
			spec_label,net_content_qty,net_content_unit,unit_rule_override_json
		) VALUES
			(1,'蓝山挂耳',8,true,'drip_bag',10,10,'10g',10,'g','{"inventory_unit":"kg"}'::jsonb),
			(2,'蓝山熟豆',50,true,'roasted_bean',10,10,'10g',10,'g','{"inventory_unit":"kg"}'::jsonb);
		INSERT INTO %s.order_process_statuses(name,sort,active) VALUES ('待处理',10,true)
		ON CONFLICT (name) DO NOTHING;
		INSERT INTO %s.orders(id,order_no,order_date,is_void,process_status_id) VALUES
			(1,'SO-DRIP-BOX','2026-05-18',false,(SELECT id FROM %s.order_process_statuses WHERE name='待处理' LIMIT 1));
		INSERT INTO %s.order_items(
			order_id,line_no,item_name,qty,unit,spec,product_id,unit_price,line_total,
			product_kind,sales_unit,unit_bag_count,unit_bean_g
		) VALUES (
			1,1,'蓝山挂耳',2,'盒','10g/袋',1,80,160,
			'drip_bag','box',10,10
		);
		INSERT INTO %s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g) VALUES
			(1,10,'finished_goods',5,0),
			(2,0,'finished_goods',0,40);
		INSERT INTO %s.product_bom(product_id,yield_rate) VALUES (1,1.0000),(2,0.8000);
		INSERT INTO %s.product_bom_items(
			product_id,material_id,ratio_pct,component_type,component_product_id,component_spec_g,consume_unit,qty_per_unit
		) VALUES
			(1,0,0,'finished_product',2,0,'g_per_bag',10),
			(1,21,0,'material',0,0,'unit_per_bag',1),
			(1,22,0,'material',0,0,'unit_per_box',1);
		INSERT INTO %s.materials(id,code,name,kind,unit,onhand_g,onhand_units,purchase_price,sale_price)
		VALUES
			(21,'FILTER','挂耳滤袋','pack','个',0,100,1,0),
			(22,'DRIP-BOX','挂耳盒','pack','个',0,10,1,0);
		INSERT INTO %s.roast_machines(name,capacity_g,allowed_specs,min_roast_g,active)
		VALUES ('样机',1000,'1000',100,true);
		`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema))

	e := newProducePlanTestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/produce/unproduced?selected=1-10&plan=1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/produce/unproduced status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	for _, needle := range []string{
		`"product_name":"蓝山挂耳"`,
		`"need_units":20`,
		`"need_g":200`,
		`"inv_units":5`,
		`"gap_g":150`,
		`"production_kind":"drip_bag"`,
		`"need_bags":20`,
		`"upstream_product_id":2`,
		`"upstream_roast_demand_g":150`,
		`"upstream_shortage_g":110`,
		`"name":"挂耳滤袋"`,
		`"qty":15`,
		`"name":"挂耳盒"`,
		`"qty":2`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("drip plan response missing %s: %s", needle, body)
		}
	}
	if strings.Contains(body, `"product_name":"蓝山熟豆","spec_g":10`) {
		t.Fatalf("drip order should create drip production need, not direct roasted bean row: %s", body)
	}
}

func newProducePlanTestEcho(pool *pgxpool.Pool, schema string) *echo.Echo {
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(1))
			c.Set("operator_employee", "测试员")
			c.Set("actor", "测试员")
			return next(c)
		}
	})
	productionSvc := productionapp.NewService(postgresproduction.NewRepository(pool, schema))
	registerUnprodSummaryPages(e)
	registerUnprodSummaryAPI(e, productionSvc)
	registerProductionPlanAPI(e, productionSvc)
	registerProductionFlowPages(e, productionSvc, nil)
	return e
}
