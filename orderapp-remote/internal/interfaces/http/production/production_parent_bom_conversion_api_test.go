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

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func TestProductionPlanAPIInheritsParentBOMAndFreezesSalesSpecConversion(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.products(
			id,name,parent_product_id,default_price,active,spec_label,net_content_qty,net_content_unit,unit_rule_override_json
		) VALUES
			(644,'如目达摩',0,0,true,'',0,'','{"inventory_unit":"kg"}'::jsonb),
			(645,'如目达摩新归属',0,0,true,'',0,'','{"inventory_unit":"lb"}'::jsonb),
			(789,'如目达摩',644,0,true,'454g',454,'g','{}'::jsonb);

		INSERT INTO %[1]s.order_process_statuses(name,sort,active) VALUES
			('待处理',10,true),
			('生产中',20,true)
		ON CONFLICT (name) DO NOTHING;
		INSERT INTO %[1]s.orders(id,order_no,order_date,is_void,process_status_id)
		VALUES (
			55301,'SO-20260725-0001','2026-07-25',false,
			(SELECT id FROM %[1]s.order_process_statuses WHERE name='待处理' LIMIT 1)
		);
		INSERT INTO %[1]s.order_items(
			order_id,line_no,item_name,qty,unit,sales_unit,spec,product_id,unit_price,line_total,price_source_json
		) VALUES (
			55301,1,'如目达摩',4,'454g','454g','禁止从本字段解析',789,0,0,
			'{"production_quantity_snapshot":{"sku_id":789,"parent_product_id":644,"spec_label":"454g","sales_unit":"454g","inventory_unit":"kg","inventory_qty_per_sales_unit":0.454,"conversion_source":"published_inventory_conversion"}}'::jsonb
		);
		-- The order snapshot was saved while SKU 789 belonged to parent 644.
		-- Re-parenting the live SKU later must not redirect this order to the
		-- new parent's unit rules or BOM.
		UPDATE %[1]s.products SET parent_product_id=645 WHERE id=789;

		INSERT INTO %[1]s.materials(id,code,name,kind,unit,cost_unit,onhand_g,onhand_units,purchase_price,sale_price)
		VALUES (9001,'RAW-PR553','如目达摩生豆','bean','kg','kg',0,0,54,0);
		INSERT INTO %[1]s.process_routes(id,name,status,default_equipment,default_minutes)
		VALUES (55301,'如目达摩标准烘焙','active','标准烘焙机',20);
		INSERT INTO %[1]s.process_route_operations(
			route_id,seq,operation,workstation,default_equipment,default_minutes,records_loss
		) VALUES (55301,1,'烘焙','烘焙中心','标准烘焙机',20,true);
		INSERT INTO %[1]s.production_boms(id,code,name,output_product_id,status)
		VALUES (55301,'PBOM-PR553','如目达摩父商品 BOM',644,'active');
		INSERT INTO %[1]s.production_bom_versions(
			id,bom_id,version_no,status,yield_rate,output_qty,output_unit,process_route_id,published_at,created_at
		) VALUES
			(55301,55301,'V001','published',1,1,'kg',55301,'2026-07-24 10:00:00+08','2026-07-24 10:00:00+08'),
			(55302,55301,'V002','draft',0.5,1,'kg',55301,NULL,'2026-07-25 10:00:00+08');
		INSERT INTO %[1]s.production_bom_version_items(
			version_id,material_id,component_type,consume_unit,ratio_pct
		) VALUES
			(55301,9001,'material','ratio_pct',100),
			(55302,9001,'material','ratio_pct',100);
		INSERT INTO %[1]s.product_production_bom_bindings(product_id,bom_id,bom_version_id,bound_by)
		VALUES (644,55301,55301,'test');
	`, schema))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 9001, 9001, "MB-PR553", "如目达摩生豆", 3000)

	app := newProductionFlowTestEcho(pool, schema)
	summaryReq := httptest.NewRequest(http.MethodGet, "/api/produce/unproduced?from=2026-07-25&to=2026-07-25", nil)
	summaryRec := httptest.NewRecorder()
	app.ServeHTTP(summaryRec, summaryReq)
	if summaryRec.Code != http.StatusOK {
		t.Fatalf("GET production demand status=%d body=%s", summaryRec.Code, summaryRec.Body.String())
	}
	var summary productionapp.PlanSummaryData
	if err := json.Unmarshal(summaryRec.Body.Bytes(), &summary); err != nil {
		t.Fatalf("decode production demand: %v\n%s", err, summaryRec.Body.String())
	}
	if len(summary.Rows) != 1 {
		t.Fatalf("production demand rows=%d, want 1: %s", len(summary.Rows), summaryRec.Body.String())
	}
	demand := summary.Rows[0]
	assertProductionFloat(t, "demand sales spec count", demand.GapSalesSpecCount, 4)
	assertProductionFloat(t, "demand inventory conversion", demand.InventoryQtyPerSalesUnit, 0.454)
	assertProductionFloat(t, "demand planned inventory qty", demand.GapInventoryQty, 1.816)
	if demand.ProductID != 789 || demand.ParentProductID != 644 || demand.SpecG != 454 || demand.InventoryUnit != "kg" {
		t.Fatalf("production demand identity/conversion=%+v", demand)
	}

	createReq := httptest.NewRequest(
		http.MethodPost,
		"/api/production-plans",
		strings.NewReader(`{"from":"2026-07-25","to":"2026-07-25","source_type":"erp_order","selected":["789-454"]}`),
	)
	createReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	createRec := httptest.NewRecorder()
	app.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusOK {
		t.Fatalf("POST production plan status=%d body=%s", createRec.Code, createRec.Body.String())
	}
	var plan productionapp.ProductionPlanDetail
	if err := json.Unmarshal(createRec.Body.Bytes(), &plan); err != nil {
		t.Fatalf("decode production plan: %v\n%s", err, createRec.Body.String())
	}
	if len(plan.Items) != 1 {
		t.Fatalf("production plan items=%d, want 1: %s", len(plan.Items), createRec.Body.String())
	}
	item := plan.Items[0]
	if item.ProductID != 789 || item.ParentProductID != 644 || item.BomSourceProductID != 644 ||
		!item.BomInherited || item.BomSource != "parent" || item.BomVersionID != 55301 {
		t.Fatalf("production plan BOM inheritance=%+v", item)
	}
	if item.PlannedG != 1816 || item.PlannedOutputG != 1816 || item.SpecG != 454 {
		t.Fatalf("production plan legacy gram projection=%+v", item)
	}
	assertProductionFloat(t, "plan sales spec count", item.SalesSpecCount, 4)
	assertProductionFloat(t, "plan inventory conversion", item.InventoryQtyPerSalesUnit, 0.454)
	assertProductionFloat(t, "plan planned inventory qty", item.PlannedInventoryQty, 1.816)
	if len(plan.MaterialSummary) != 1 {
		t.Fatalf("material summary=%+v", plan.MaterialSummary)
	}
	if plan.MaterialSummary[0].Name != "如目达摩生豆" || plan.MaterialSummary[0].Unit != "kg" {
		t.Fatalf("material summary identity=%+v", plan.MaterialSummary[0])
	}
	assertProductionFloat(t, "material exact kg", plan.MaterialSummary[0].ExactQty, 1.816)
	seedProductionParentBomPlanOperationSplits(t, ctx, pool, schema, plan.ID)

	submitReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", plan.ID), nil)
	submitRec := httptest.NewRecorder()
	app.ServeHTTP(submitRec, submitReq)
	if submitRec.Code != http.StatusOK {
		t.Fatalf("POST submit production plan status=%d body=%s", submitRec.Code, submitRec.Body.String())
	}
	var submitted productionapp.ProductionPlanSubmitResult
	if err := json.Unmarshal(submitRec.Body.Bytes(), &submitted); err != nil {
		t.Fatalf("decode submitted production plan: %v\n%s", err, submitRec.Body.String())
	}
	if len(submitted.WorkOrders) != 1 {
		t.Fatalf("submitted work orders=%d, want 1: %s", len(submitted.WorkOrders), submitRec.Body.String())
	}
	workOrder := submitted.WorkOrders[0]
	if workOrder.ParentProductID != 644 || workOrder.BomSourceProductID != 644 || !workOrder.BomInherited {
		t.Fatalf("work order frozen BOM source=%+v", workOrder)
	}
	assertProductionFloat(t, "work order sales spec count", workOrder.SalesSpecCount, 4)
	assertProductionFloat(t, "work order planned inventory qty", workOrder.PlannedInventoryQty, 1.816)

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %[1]s.products
		SET net_content_qty=100,net_content_unit='g',unit_rule_override_json='{"inventory_unit":"lb"}'::jsonb
		WHERE id IN (644,789);
		UPDATE %[1]s.production_bom_versions
		SET yield_rate=0.25,output_unit='lb',status='archived'
		WHERE id=55301;
		UPDATE %[1]s.production_bom_versions SET status='published',published_at=now() WHERE id=55302;
		UPDATE %[1]s.product_production_bom_bindings SET bom_version_id=55302 WHERE product_id=644;
	`, schema))

	detailReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/produce/work-orders/%d", workOrder.ID), nil)
	detailRec := httptest.NewRecorder()
	app.ServeHTTP(detailRec, detailReq)
	if detailRec.Code != http.StatusOK {
		t.Fatalf("GET work order detail status=%d body=%s", detailRec.Code, detailRec.Body.String())
	}
	var detail productionapp.WorkOrderDetail
	if err := json.Unmarshal(detailRec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode work order detail: %v\n%s", err, detailRec.Body.String())
	}
	frozen := detail.WorkOrder
	if frozen.BomVersionID != 55301 || frozen.InventoryUnit != "kg" || frozen.PlannedG != 1816 ||
		frozen.PlannedOutputG != 1816 || frozen.SpecG != 454 {
		t.Fatalf("work order changed after master data mutation: %+v", frozen)
	}
	assertProductionFloat(t, "frozen work order yield", frozen.YieldRate, 1)
	assertProductionFloat(t, "frozen work order inventory conversion", frozen.InventoryQtyPerSalesUnit, 0.454)
	assertProductionFloat(t, "frozen work order planned inventory qty", frozen.PlannedInventoryQty, 1.816)

	startReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/start", workOrder.ID), nil)
	startRec := httptest.NewRecorder()
	app.ServeHTTP(startRec, startReq)
	if startRec.Code != http.StatusOK {
		t.Fatalf("POST start work order status=%d body=%s", startRec.Code, startRec.Body.String())
	}
	var startedYield float64
	var reservedG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT ri.bom_yield_rate::float8,COALESCE(SUM(res.reserved_g),0)::bigint
		FROM %[1]s.produce_running_items ri
		LEFT JOIN %[1]s.work_order_material_reservations res ON res.running_item_id=ri.id
		WHERE ri.id=(SELECT running_item_id FROM %[1]s.work_orders WHERE id=$1)
		GROUP BY ri.id,ri.bom_yield_rate
	`, schema), workOrder.ID).Scan(&startedYield, &reservedG); err != nil {
		t.Fatalf("query started frozen work order: %v", err)
	}
	assertProductionFloat(t, "started frozen work order yield", startedYield, 1)
	if reservedG != 1816 {
		t.Fatalf("reserved material grams=%d, want 1816", reservedG)
	}

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.products(
			id,name,parent_product_id,default_price,active,spec_label,net_content_qty,net_content_unit,unit_rule_override_json
		) VALUES
			(646,'计件父商品',0,0,true,'',0,'','{"inventory_unit":"件"}'::jsonb),
			(790,'计件商品',646,0,true,'1件',1,'件','{}'::jsonb);
		INSERT INTO %[1]s.orders(id,order_no,order_date,is_void,process_status_id)
		VALUES (
			55302,'SO-PR553-COUNT','2026-07-26',false,
			(SELECT id FROM %[1]s.order_process_statuses WHERE name='待处理' LIMIT 1)
		);
		INSERT INTO %[1]s.order_items(
			order_id,line_no,item_name,qty,unit,sales_unit,spec,product_id,unit_price,line_total,price_source_json
		) VALUES (
			55302,1,'计件商品',2,'件','件','不可解析',790,0,0,
			'{"production_quantity_snapshot":{"sku_id":790,"parent_product_id":646,"spec_label":"1件","sales_unit":"件","inventory_unit":"件","inventory_qty_per_sales_unit":1,"conversion_source":"published_inventory_conversion"}}'::jsonb
		);
		INSERT INTO %[1]s.materials(id,code,name,kind,unit,cost_unit,onhand_g,onhand_units,purchase_price,sale_price)
		VALUES (9002,'COUNT-PR553','计件组件','packaging','个','个',0,0,1,0);
		INSERT INTO %[1]s.process_routes(id,name,status,default_equipment,default_minutes)
		VALUES (55302,'计件路线','active','计件工位',5);
		INSERT INTO %[1]s.process_route_operations(
			route_id,seq,operation,workstation,default_equipment,default_minutes,records_loss
		) VALUES (55302,1,'组装','计件工位','计件工位',5,false);
		INSERT INTO %[1]s.production_boms(id,code,name,output_product_id,status)
		VALUES (55302,'PBOM-COUNT','计件父商品 BOM',646,'active');
		INSERT INTO %[1]s.production_bom_versions(
			id,bom_id,version_no,status,yield_rate,output_qty,output_unit,process_route_id,published_at
		) VALUES (55303,55302,'V001','published',1,1,'件',55302,now());
		INSERT INTO %[1]s.production_bom_version_items(
			version_id,material_id,component_type,consume_unit,qty_per_unit
		) VALUES (55303,9002,'material','unit',1);
		INSERT INTO %[1]s.product_production_bom_bindings(product_id,bom_id,bom_version_id,bound_by)
		VALUES (646,55302,55303,'test');
	`, schema))
	countReq := httptest.NewRequest(
		http.MethodPost,
		"/api/production-plans",
		strings.NewReader(`{"from":"2026-07-26","to":"2026-07-26","source_type":"erp_order","selected":["790-0"]}`),
	)
	countReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	countRec := httptest.NewRecorder()
	app.ServeHTTP(countRec, countReq)
	if countRec.Code != http.StatusBadRequest ||
		!strings.Contains(countRec.Body.String(), "requires a weight inventory unit") ||
		!strings.Contains(countRec.Body.String(), "SO-PR553-COUNT") {
		t.Fatalf("count-based formal plan must reject without writing an empty material plan, status=%d body=%s", countRec.Code, countRec.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_plans", "1=1", 1)
}

func TestProductionPlanAPIAppliesInheritedPublishedBomLossOnce(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedRumuParentBomLossDemand(t, ctx, pool, schema)

	app := newProductionFlowTestEcho(pool, schema)
	createReq := httptest.NewRequest(
		http.MethodPost,
		"/api/production-plans",
		strings.NewReader(`{"from":"2026-07-27","to":"2026-07-27","source_type":"erp_order","selected":["789-454"]}`),
	)
	createReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	createRec := httptest.NewRecorder()
	app.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusOK {
		t.Fatalf("POST production plan status=%d body=%s", createRec.Code, createRec.Body.String())
	}
	var plan productionapp.ProductionPlanDetail
	if err := json.Unmarshal(createRec.Body.Bytes(), &plan); err != nil {
		t.Fatalf("decode production plan: %v\n%s", err, createRec.Body.String())
	}
	if len(plan.Items) != 1 {
		t.Fatalf("production plan items=%d, want 1: %s", len(plan.Items), createRec.Body.String())
	}
	item := plan.Items[0]
	if item.BomVersionID != 55704 || item.BomSourceProductID != 644 || !item.BomInherited {
		t.Fatalf("production plan did not freeze inherited published V004: %+v", item)
	}
	if item.PlannedOutputG != 6356 || item.PlannedG != 7752 {
		t.Fatalf("production plan quantities=%+v, want output 6356g and 18%% gross-input loss demand 7752g", item)
	}
	if len(plan.MaterialSummary) != 1 {
		t.Fatalf("material summary=%+v, want one row", plan.MaterialSummary)
	}
	material := plan.MaterialSummary[0]
	if material.Name != "如目达摩生豆" || material.Unit != "kg" || material.Qty != 8 || math.Abs(material.ExactQty-7.752) > 0.000000001 {
		t.Fatalf("material summary=%+v, want loss applied once rather than old yield plus line loss", material)
	}
}

func TestProductionPlanAPIKeepsSameSKUWithDifferentFrozenParentsIsolated(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.products(
			id,name,parent_product_id,default_price,active,spec_label,net_content_qty,net_content_unit,unit_rule_override_json
		) VALUES
			(644,'如目达摩旧父商品',0,0,true,'',0,'','{"inventory_unit":"kg"}'::jsonb),
			(645,'如目达摩新父商品',0,0,true,'',0,'','{"inventory_unit":"kg"}'::jsonb),
			(789,'如目达摩',645,0,true,'454g',454,'g','{}'::jsonb);
		INSERT INTO %[1]s.order_process_statuses(name,sort,active)
		VALUES ('待处理',10,true) ON CONFLICT (name) DO NOTHING;
		INSERT INTO %[1]s.orders(id,order_no,order_date,is_void,process_status_id) VALUES
			(55311,'SO-SNAPSHOT-A','2026-07-27',false,(SELECT id FROM %[1]s.order_process_statuses WHERE name='待处理' LIMIT 1)),
			(55312,'SO-SNAPSHOT-B','2026-07-27',false,(SELECT id FROM %[1]s.order_process_statuses WHERE name='待处理' LIMIT 1));
		INSERT INTO %[1]s.order_items(
			order_id,line_no,item_name,qty,unit,sales_unit,spec,product_id,unit_price,line_total,price_source_json
		) VALUES
			(55311,1,'如目达摩',1,'454g','454g','454g',789,0,0,
			 '{"production_quantity_snapshot":{"sku_id":789,"parent_product_id":644,"spec_label":"454g","sales_unit":"454g","inventory_unit":"kg","inventory_qty_per_sales_unit":0.454,"conversion_source":"published"}}'::jsonb),
			(55312,1,'如目达摩',1,'454g','454g','454g',789,0,0,
			 '{"production_quantity_snapshot":{"sku_id":789,"parent_product_id":645,"spec_label":"454g","sales_unit":"454g","inventory_unit":"kg","inventory_qty_per_sales_unit":0.454,"conversion_source":"published"}}'::jsonb);

		INSERT INTO %[1]s.materials(id,code,name,kind,unit,cost_unit,onhand_g,onhand_units,purchase_price,sale_price) VALUES
			(9011,'RAW-A','旧父商品原料','bean','kg','kg',0,0,54,0),
			(9012,'RAW-B','新父商品原料','bean','kg','kg',0,0,54,0);
		INSERT INTO %[1]s.process_routes(id,name,status,default_equipment,default_minutes) VALUES
			(55311,'旧父商品路线','active','标准烘焙机',20),
			(55312,'新父商品路线','active','标准烘焙机',20);
		INSERT INTO %[1]s.process_route_operations(
			route_id,seq,operation,workstation,default_equipment,default_minutes,records_loss
		) VALUES
			(55311,1,'烘焙','烘焙中心','标准烘焙机',20,true),
			(55312,1,'烘焙','烘焙中心','标准烘焙机',20,true);
		INSERT INTO %[1]s.production_boms(id,code,name,output_product_id,status) VALUES
			(55311,'PBOM-SNAPSHOT-A','旧父商品 BOM',644,'active'),
			(55312,'PBOM-SNAPSHOT-B','新父商品 BOM',645,'active');
		INSERT INTO %[1]s.production_bom_versions(
			id,bom_id,version_no,status,yield_rate,output_qty,output_unit,material_loss_rate,process_route_id,published_at
		) VALUES
			(55311,55311,'V001','published',1,1,'kg',0.1,55311,now()),
			(55312,55312,'V001','published',1,1,'kg',0.2,55312,now());
		INSERT INTO %[1]s.production_bom_version_items(
			version_id,material_id,component_type,consume_unit,ratio_pct,material_loss_rate
		) VALUES
			(55311,9011,'material','ratio_pct',100,0.1),
			(55312,9012,'material','ratio_pct',100,0.2);
		INSERT INTO %[1]s.product_production_bom_bindings(product_id,bom_id,bom_version_id,bound_by) VALUES
			(644,55311,55311,'test'),
			(645,55312,55312,'test');
	`, schema))

	app := newProductionFlowTestEcho(pool, schema)
	summaryReq := httptest.NewRequest(http.MethodGet, "/api/produce/unproduced?from=2026-07-27&to=2026-07-27", nil)
	summaryRec := httptest.NewRecorder()
	app.ServeHTTP(summaryRec, summaryReq)
	if summaryRec.Code != http.StatusOK {
		t.Fatalf("GET split snapshot demand status=%d body=%s", summaryRec.Code, summaryRec.Body.String())
	}
	var summary productionapp.PlanSummaryData
	if err := json.Unmarshal(summaryRec.Body.Bytes(), &summary); err != nil {
		t.Fatalf("decode split snapshot demand: %v\n%s", err, summaryRec.Body.String())
	}
	if len(summary.Rows) != 2 {
		t.Fatalf("same SKU snapshot rows=%d, want 2: %s", len(summary.Rows), summaryRec.Body.String())
	}
	parents := map[int64]bool{}
	for _, row := range summary.Rows {
		parents[row.ParentProductID] = true
	}
	if !parents[644] || !parents[645] {
		t.Fatalf("same SKU frozen parents were not isolated: %+v", summary.Rows)
	}

	previewReq := httptest.NewRequest(http.MethodGet, "/api/produce/unproduced?from=2026-07-27&to=2026-07-27&plan=1&selected=789-454", nil)
	previewRec := httptest.NewRecorder()
	app.ServeHTTP(previewRec, previewReq)
	if previewRec.Code != http.StatusOK {
		t.Fatalf("GET split snapshot preview status=%d body=%s", previewRec.Code, previewRec.Body.String())
	}
	var preview productionapp.PlanSummaryData
	if err := json.Unmarshal(previewRec.Body.Bytes(), &preview); err != nil {
		t.Fatalf("decode split snapshot preview: %v\n%s", err, previewRec.Body.String())
	}
	if len(preview.PlanRows) != 2 {
		t.Fatalf("same SKU preview rows=%d, want 2: %s", len(preview.PlanRows), previewRec.Body.String())
	}
	losses := map[float64]bool{}
	for _, row := range preview.PlanRows {
		losses[row.BomMaterialLossRate] = true
	}
	if !losses[0.1] || !losses[0.2] {
		t.Fatalf("same SKU preview BOM losses were mixed: %+v", preview.PlanRows)
	}
	materialQty := map[string]float64{}
	for _, material := range preview.Materials {
		materialQty[material.Name] = material.ExactQty
	}
	if math.Abs(materialQty["旧父商品原料"]-0.505) > 0.000000001 ||
		math.Abs(materialQty["新父商品原料"]-0.568) > 0.000000001 {
		t.Fatalf("same SKU preview materials were mixed: %+v", preview.Materials)
	}

	createReq := httptest.NewRequest(
		http.MethodPost,
		"/api/production-plans",
		strings.NewReader(`{"from":"2026-07-27","to":"2026-07-27","source_type":"erp_order","selected":["789-454"]}`),
	)
	createReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	createRec := httptest.NewRecorder()
	app.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusOK {
		t.Fatalf("POST split snapshot plan status=%d body=%s", createRec.Code, createRec.Body.String())
	}
	var plan productionapp.ProductionPlanDetail
	if err := json.Unmarshal(createRec.Body.Bytes(), &plan); err != nil {
		t.Fatalf("decode split snapshot plan: %v\n%s", err, createRec.Body.String())
	}
	if len(plan.Items) != 2 {
		t.Fatalf("same SKU plan items=%d, want 2 isolated items: %s", len(plan.Items), createRec.Body.String())
	}
	for _, item := range plan.Items {
		if item.ParentProductID != item.BomSourceProductID || !item.BomInherited {
			t.Fatalf("plan item did not preserve its frozen parent/BOM: %+v", item)
		}
	}
	seedProductionParentBomPlanOperationSplits(t, ctx, pool, schema, plan.ID)

	submitReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", plan.ID), nil)
	submitRec := httptest.NewRecorder()
	app.ServeHTTP(submitRec, submitReq)
	if submitRec.Code != http.StatusOK {
		t.Fatalf("POST split snapshot submit status=%d body=%s", submitRec.Code, submitRec.Body.String())
	}
	var submitted productionapp.ProductionPlanSubmitResult
	if err := json.Unmarshal(submitRec.Body.Bytes(), &submitted); err != nil {
		t.Fatalf("decode split snapshot work orders: %v\n%s", err, submitRec.Body.String())
	}
	if len(submitted.WorkOrders) != 2 {
		t.Fatalf("same SKU work orders=%d, want 2 isolated work orders: %s", len(submitted.WorkOrders), submitRec.Body.String())
	}
	workOrderParents := map[int64]bool{}
	for _, workOrder := range submitted.WorkOrders {
		workOrderParents[workOrder.ParentProductID] = true
		if workOrder.ParentProductID != workOrder.BomSourceProductID || !workOrder.BomInherited {
			t.Fatalf("work order did not preserve its frozen parent/BOM: %+v", workOrder)
		}
	}
	if !workOrderParents[644] || !workOrderParents[645] {
		t.Fatalf("work order frozen parents were merged: %+v", submitted.WorkOrders)
	}
}

func seedProductionParentBomPlanOperationSplits(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string, planID int64) {
	t.Helper()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_plan_operation_splits(
			production_plan_id,production_plan_item_id,operation_seq,operation,
			batch_size_qty,batch_size_unit,standard_minutes,planned_batch_count,
			planned_qty,planned_qty_g,planned_minutes
		)
		SELECT item.production_plan_id,item.id,operation.seq,operation.operation,
		       item.planned_g::numeric,'g',operation.default_minutes,1,
		       item.planned_g::numeric,item.planned_g,operation.default_minutes
		FROM %s.production_plan_items item
		JOIN %s.process_route_operations operation ON operation.route_id=item.process_route_id
		WHERE item.production_plan_id=$1
		ORDER BY item.id,operation.seq
	`, schema, schema, schema), planID); err != nil {
		t.Fatalf("seed parent BOM operation splits: %v", err)
	}
}

func assertProductionFloat(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.0000001 {
		t.Fatalf("%s=%v, want %v", label, got, want)
	}
}
