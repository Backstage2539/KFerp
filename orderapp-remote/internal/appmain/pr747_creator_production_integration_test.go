package appmain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"math"
	"net/http/httptest"
	bomapp "orderapp/internal/application/bom"
	catalogapp "orderapp/internal/application/catalog"
	materialsapp "orderapp/internal/application/materials"
	creatorapp "orderapp/internal/application/productcreator"
	productionapp "orderapp/internal/application/production"
	stockapp "orderapp/internal/application/stock"
	postgresbom "orderapp/internal/infrastructure/postgres/bom"
	postgrescatalog "orderapp/internal/infrastructure/postgres/catalog"
	postgresmaterials "orderapp/internal/infrastructure/postgres/materials"
	postgrescreator "orderapp/internal/infrastructure/postgres/productcreator"
	postgresproduction "orderapp/internal/infrastructure/postgres/production"
	postgresstock "orderapp/internal/infrastructure/postgres/stock"
	productionhttp "orderapp/internal/interfaces/http/production"
	"os"
	"sort"
	"strings"
	"testing"
	"time"
)

// These are read-only exports of the existing published workflows and referenced
// configuration. Tests import them into a new schema, never into business data.
func pr747CreatorFixture(t *testing.T) (*pgxpool.Pool, string, *creatorapp.Service) {
	t.Helper()
	dsn := os.Getenv("ORDERAPP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated PostgreSQL required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("%#v", err)
	}
	t.Cleanup(pool.Close)
	schema := fmt.Sprintf("test_pr747_creator_%d", time.Now().UnixNano())
	_, err = pool.Exec(ctx, "CREATE SCHEMA "+schema)
	if err != nil {
		t.Fatalf("%#v", err)
	}
	t.Cleanup(func() { pool.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") })
	if err = ensureAppSchema(ctx, pool, schema); err != nil {
		t.Fatalf("%#v", err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.product_unit_definitions(code,name,unit_type,allow_decimal,active) VALUES('袋','袋','other',false,true),('盒','盒','other',false,true),('pic','个','other',false,true) ON CONFLICT(code) DO NOTHING`, schema)); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("internal/appmain/testdata/pr747_creator_reference_config.json")
	if err != nil {
		t.Fatalf("%#v", err)
	}
	config := map[string][]map[string]any{}
	if err = json.Unmarshal(raw, &config); err != nil {
		t.Fatalf("%#v", err)
	}
	order := []string{"materials", "manufacturing_operations", "manufacturing_workstations", "manufacturing_workstation_capacities", "manufacturing_workstation_operations", "manufacturing_workstation_capacity_operations", "process_routes", "process_route_operations", "production_bom_spec_templates", "production_bom_spec_template_versions", "production_bom_spec_template_variants", "production_bom_spec_template_variant_items"}
	for _, table := range order {
		for _, row := range config[table] {
			cols := []string{}
			for key := range row {
				cols = append(cols, key)
			}
			sort.Strings(cols)
			data, _ := json.Marshal(row)
			names := strings.Join(cols, ",")
			sql := fmt.Sprintf("INSERT INTO %s.%s(%s) SELECT %s FROM json_populate_record(NULL::%s.%s,$1::json)", schema, table, names, names, schema, table)
			if _, err = pool.Exec(ctx, sql, string(data)); err != nil {
				t.Fatalf("import %s: %v", table, err)
			}
		}
		var serial *string
		if err = pool.QueryRow(ctx, "SELECT pg_get_serial_sequence($1,'id')", schema+"."+table).Scan(&serial); err == nil && serial != nil {
			if _, err = pool.Exec(ctx, fmt.Sprintf("SELECT setval($1,COALESCE((SELECT MAX(id) FROM %s.%s),1),true)", schema, table), *serial); err != nil {
				t.Fatalf("%#v", err)
			}
		}
	}
	raw, err = os.ReadFile("internal/appmain/testdata/pr747_creator_templates.json")
	if err != nil {
		t.Fatalf("%#v", err)
	}
	var templates []struct {
		ID       int64
		Name     string
		Revision int64
		Version  int64
		Workflow json.RawMessage
	}
	if err = json.Unmarshal(raw, &templates); err != nil {
		t.Fatalf("%#v", err)
	}
	for _, v := range templates {
		if _, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.business_templates(id,name,status,revision,published_version,draft_graph) VALUES($1,$2,'published',$3,$4,$5)`, schema), v.ID, v.Name, v.Revision, v.Version, string(v.Workflow)); err != nil {
			t.Fatalf("%#v", err)
		}
		if _, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.business_template_versions(template_id,version,name,workflow) VALUES($1,$2,$3,$4)`, schema), v.ID, v.Version, v.Name, string(v.Workflow)); err != nil {
			t.Fatalf("%#v", err)
		}
	}
	catalog := catalogapp.NewService(postgrescatalog.NewRepository(pool, schema))
	materials := materialsapp.NewService(postgresmaterials.NewRepository(pool, schema))
	bom := bomapp.NewService(postgresbom.NewRepository(pool, schema))
	creator := creatorapp.NewService(postgrescreator.NewRepository(pool, schema))
	exec := postgrescreator.NewBusinessExecutor(schema, pool, catalog, materials, bom, nil, nil)
	creator.UseConfigurationExecutor(exec)
	creator.UseRunStepExecutor(exec)
	return pool, schema, creator
}

const pr747Raw = "499eff60-931f-4de2-bca6-1e818ae90d1d"
const pr747Roast = "c6bfe285-75ab-4958-a2cf-95e2ae1c459b"
const pr747Roasted = "1d4390f1-7f5b-4f9c-b911-5cedc5f2f902"
const pr747Grind = "9cd5dbac-ad61-42bb-a9c1-49308b33057d"
const pr747Powder = "c7458ba8-280d-4fcf-ac6f-7413faba4077"
const pr747Drip = "c394c95c-0e92-4d78-b324-1142d954a0f7"
const pr747Bag = "05256ed7-a1b3-4b23-aed3-eed90875ed7b"
const pr747Box = "2ecd4680-8742-4fcd-bd1a-551d0affc7df"

func pr747Create(t *testing.T, svc *creatorapp.Service, template int64, name string, overrides map[string]map[string]any) creatorapp.Run {
	t.Helper()
	ctx := context.Background()
	run, err := svc.StartRun(ctx, template, "PR-748验收")
	if err != nil {
		t.Fatalf("%#v", err)
	}
	inputs := map[string]map[string]any{
		pr747Raw:     {"rows": []any{map[string]any{"row_id": "raw", "action": "create", "name": name + "生豆", "unit": "kg", "kind": "bean", "supply_mode": "purchase", "owner_type": "factory"}}},
		pr747Roast:   {"components": []any{map[string]any{"row_id": "raw-use", "source_node_id": pr747Raw, "source_row_id": "raw", "unit": "ratio_pct", "quantity": 100}}},
		pr747Roasted: {"action": "create", "kind": "bean", "unit": "kg", "supply_mode": "manufacture", "owner_type": "factory"},
	}
	if template == 1 {
		inputs[pr747Grind] = map[string]any{"main_input_source_node_id": pr747Roasted, "main_input_source_row_id": "output"}
	} else {
		inputs[pr747Grind] = map[string]any{"components": []any{map[string]any{"row_id": "roasted-use", "source_node_id": pr747Roasted, "source_row_id": "output", "unit": "kg", "quantity": 1}}}
		inputs[pr747Powder] = map[string]any{"action": "create", "kind": "bean", "unit": "kg", "supply_mode": "manufacture", "owner_type": "factory"}
		inputs[pr747Box] = map[string]any{"main_input_source_node_id": pr747Bag, "main_input_source_row_id": "spec-1"}
		inputs[pr747Drip] = map[string]any{"main_input_source_node_id": pr747Powder, "main_input_source_row_id": "output"}
	}
	for k, v := range overrides {
		inputs[k] = v
	}
	run, err = svc.SaveRunDraft(ctx, run.ID, run.Revision, inputs, map[string]string{"bfceff1c-bc42-4284-acb6-d68554a78950": name}, "PR-748验收")
	if err != nil {
		t.Fatalf("%#v", err)
	}
	run, err = svc.PreviewRun(ctx, run.ID, run.Revision, "PR-748验收")
	if err != nil {
		t.Fatalf("%#v", err)
	}
	if !run.Preview.Valid {
		t.Fatalf("creator preview: %+v", run.Preview.Issues)
	}
	run, err = svc.CommitConfiguration(ctx, run.ID, run.Revision, "pr747-create-"+name, "PR-748验收")
	if err != nil {
		t.Fatalf("%#v", err)
	}
	return run
}
func TestPR748ExistingCreatorTemplatesProductionChain(t *testing.T) {
	for _, id := range []int64{1, 2} {
		t.Run(fmt.Sprint(id), func(t *testing.T) {
			pool, schema, svc := pr747CreatorFixture(t)
			ctx := context.Background()
			var before string
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT md5(string_agg(workflow::text,',' ORDER BY id)) FROM %s.business_template_versions`, schema)).Scan(&before); err != nil {
				t.Fatalf("%#v", err)
			}
			pr747Create(t, svc, id, fmt.Sprintf("PR748-%d", id), nil)
			var count int
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.production_boms`, schema)).Scan(&count); err != nil {
				t.Fatalf("%#v", err)
			}
			want := 2
			if id == 2 {
				want = 4
			}
			if count != want {
				t.Fatalf("created %d BOMs want %d", count, want)
			}
			var after string
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT md5(string_agg(workflow::text,',' ORDER BY id)) FROM %s.business_template_versions`, schema)).Scan(&after); err != nil {
				t.Fatalf("%#v", err)
			}
			pr747ProductionChain(t, pool, schema, id)
			if before != after {
				t.Fatal("template changed")
			}
		})
	}
}

func pr747ProductionChain(t *testing.T, pool *pgxpool.Pool, schema string, template int64, options ...pr747ProductionOptions) {
	t.Helper()
	opt := pr747ProductionOptions{}
	if len(options) > 0 {
		opt = options[0]
	}
	ctx := context.Background()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, fmt.Sprintf(q, schema), args...); err != nil {
			t.Fatal(err)
		}
	}
	// Only purchased inputs are stocked. Every manufactured layer must come from
	// the previous layer's actual receipt, not synthetic inventory.
	exec(`INSERT INTO %[1]s.material_batches(batch_code,material_id,material_name,received_g,qty_g,remaining_g,qty_units,remaining_units,unit_cost,status,quality_status)
 SELECT 'PR748-OPEN-'||id,id,name,CASE WHEN unit='kg' THEN 1000000 ELSE 0 END,CASE WHEN unit='kg' THEN 1000000 ELSE 0 END,CASE WHEN unit='kg' THEN 1000000 ELSE 0 END,CASE WHEN unit='kg' THEN 0 ELSE 10000 END,CASE WHEN unit='kg' THEN 0 ELSE 10000 END,CASE WHEN unit='kg' THEN 30 ELSE 1 END,'active','pass' FROM %[1]s.materials WHERE NOT is_semi_finished OR id=$1`, opt.ReuseMaterialID)
	exec(`INSERT INTO %[1]s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_g,qty_units) SELECT id,batch_code,material_id,'wip',remaining_g,remaining_units FROM %[1]s.material_batches`)
	exec(`UPDATE %[1]s.materials SET onhand_g=b.g,onhand_units=b.n FROM (SELECT material_id,SUM(remaining_g) g,SUM(remaining_units) n FROM %[1]s.material_batches GROUP BY material_id) b WHERE materials.id=b.material_id`)

	exec(`INSERT INTO %[1]s.order_process_statuses(name,sort,active) SELECT x.name,x.sort,true FROM (VALUES('待处理',10),('生产中',20)) x(name,sort) WHERE NOT EXISTS(SELECT 1 FROM %[1]s.order_process_statuses s WHERE s.name=x.name)`)
	if opt.ReuseProductID > 0 {
		exec(`INSERT INTO %[1]s.finished_inventory(product_id,bom_spec_id,bom_variant_id,spec_g,warehouse,onhand_units) VALUES($1,$2,$3,0,'wip',1000)`, opt.ReuseProductID, opt.ReuseSpecID, opt.ReuseVariantID)
		exec(`INSERT INTO %[1]s.stock_batches(batch_code,item_type,item_id,bom_spec_id,bom_variant_id,item_name,spec_g,source_doc_type,source_doc_id,qty_units,remaining_units,unit_cost,quality_status) VALUES('PR748-EXISTING-BAGS','finished_product',$1,$2,$3,'已有袋装商品',0,'opening',747,1000,1000,2,'pass')`, opt.ReuseProductID, opt.ReuseSpecID, opt.ReuseVariantID)
		exec(`INSERT INTO %[1]s.stock_ledger_entries(item_type,item_id,bom_spec_id,bom_variant_id,item_name,warehouse,source_doc_type,source_doc_id,source_batch_code,qty_before_units,qty_change_units,qty_after_units) VALUES('finished_product',$1,$2,$3,'已有袋装商品','wip','opening',747,'PR748-EXISTING-BAGS',0,1000,1000)`, opt.ReuseProductID, opt.ReuseSpecID, opt.ReuseVariantID)
	}
	var product, spec, variant int64
	var name, unit, label string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT p.id,s.id,v.id,p.name,v.inventory_unit,s.name FROM %[1]s.products p JOIN %[1]s.production_boms b ON b.output_product_id=p.id JOIN %[1]s.production_bom_versions bv ON bv.bom_id=b.id AND bv.status='published' JOIN %[1]s.production_bom_version_variants v ON v.version_id=bv.id JOIN %[1]s.production_bom_specs s ON s.id=v.bom_spec_id ORDER BY p.id DESC,v.is_default DESC,v.id LIMIT 1`, schema)).Scan(&product, &spec, &variant, &name, &unit, &label); err != nil {
		t.Fatal(err)
	}
	exec(`INSERT INTO %[1]s.orders(id,order_no,order_date,is_void,process_status_id) VALUES(747,'SO-PR748','2026-11-01',false,(SELECT id FROM %[1]s.order_process_statuses WHERE name='待处理'))`)
	exec(`INSERT INTO %s.order_items(order_id,line_no,item_name,qty,unit,spec,product_id,bom_spec_id,bom_variant_id,unit_price,line_total) VALUES(747,1,$1,10,$2,$3,$4,$5,$6,1,10)`, name, unit, label, product, spec, variant)
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(747))
			c.Set("actor", "PR-748验收")
			c.Set("operator_employee", "PR-748验收")
			return next(c)
		}
	})
	productionhttp.RegisterRoutes(e, productionhttp.Dependencies{Production: productionapp.NewService(postgresproduction.NewRepository(pool, schema))})
	call := func(method, path string, body any, target any) {
		t.Helper()
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(method, path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		e.ServeHTTP(res, req)
		if res.Code != 200 {
			for _, table := range []string{"stock_batches", "work_orders", "work_order_material_reservations", "work_order_material_reservation_batches", "stock_entry_items"} {
				var data string
				_ = pool.QueryRow(ctx, fmt.Sprintf("SELECT COALESCE(jsonb_agg(to_jsonb(x))::text,'[]') FROM %s.%s x", schema, table)).Scan(&data)
				t.Logf("%s: %s", table, data)
			}
			t.Fatalf("%s %s: %d %s", method, path, res.Code, res.Body.String())
		}
		if target != nil {
			if err := json.Unmarshal(res.Body.Bytes(), target); err != nil {
				t.Fatal(err)
			}
		}
	}
	var plan productionapp.ProductionPlanDetail
	if opt.Direct {
		call("POST", "/api/produce/start", map[string]any{"from": "2026-11-01", "to": "2026-11-01", "selected": []string{fmt.Sprintf("product:%d:bom_spec:%d", product, spec)}}, nil)
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT production_plan_id FROM %s.work_orders ORDER BY id DESC LIMIT 1`, schema)).Scan(&plan.ID); err != nil {
			t.Fatal(err)
		}
		call("GET", fmt.Sprintf("/api/production-plans/%d", plan.ID), nil, &plan)
	} else {
		call("POST", "/api/production-plans", map[string]any{"from": "2026-11-01", "to": "2026-11-01", "selected": []string{fmt.Sprintf("product:%d:bom_spec:%d", product, spec)}}, &plan)
		want := 2
		if template == 2 {
			want = 4
		}
		if opt.ExpectedLayers > 0 {
			want = opt.ExpectedLayers
		}
		if len(plan.Items) != want {
			t.Fatalf("plan layers=%d want %d: %+v", len(plan.Items), want, plan.Items)
		}
		groups := map[int64][]productionapp.ProductionPlanComponentSource{}
		for _, s := range plan.ComponentSources {
			if len(s.Options) > 0 {
				s.SourceWarehouse = s.Options[0].Warehouse
				s.SourceOwnerCustomerID = s.Options[0].OwnerCustomerID
				groups[s.ProductionPlanItemID] = append(groups[s.ProductionPlanItemID], s)
			}
		}
		for id, sources := range groups {
			call("PUT", fmt.Sprintf("/api/production-plans/%d/items/%d/component-sources", plan.ID, id), map[string]any{"sources": sources}, nil)
		}
		splits := []productionapp.ProductionPlanOperationSplit{}
		for _, item := range plan.Items {
			rows, err := pool.Query(ctx, fmt.Sprintf(`SELECT o.seq,o.operation_id,o.operation,o.standard_cost_capacity_id,c.batch_size_unit FROM %[1]s.process_route_operations o JOIN %[1]s.manufacturing_workstation_capacities c ON c.id=o.standard_cost_capacity_id WHERE o.route_id=$1 ORDER BY o.seq`, schema), item.ProcessRouteID)
			if err != nil {
				t.Fatal(err)
			}
			for rows.Next() {
				var s productionapp.ProductionPlanOperationSplit
				var u string
				if err = rows.Scan(&s.OperationSeq, &s.OperationID, &s.Operation, &s.WorkstationCapacityID, &u); err != nil {
					t.Fatal(err)
				}
				s.ProductionPlanItemID = item.ID
				if f := productionapp.OperationUnitFactor(u); f > 0 {
					s.PlannedQty = float64(item.PlannedG) / f
				} else {
					s.PlannedQty = item.PlannedInventoryQty
				}
				splits = append(splits, s)
			}
			rows.Close()
		}
		call("POST", fmt.Sprintf("/api/production-plans/%d/operation-splits", plan.ID), map[string]any{"items": splits}, nil)
		call("POST", fmt.Sprintf("/api/production-plans/%d/submit", plan.ID), nil, nil)
	}
	want := 2
	if template == 2 {
		want = 4
	}
	if opt.ExpectedLayers > 0 {
		want = opt.ExpectedLayers
	}
	var consumed int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_entries WHERE purpose='material_consumption_for_manufacture'`, schema)).Scan(&consumed); err != nil {
		t.Fatal(err)
	}
	if consumed != 0 {
		t.Fatal("plan consumed inputs")
	}
	rows, err := pool.Query(ctx, fmt.Sprintf(`WITH RECURSIVE depths AS(SELECT id,0 d FROM %[1]s.work_orders WHERE production_plan_id=$1 UNION ALL SELECT dep.depends_on_work_order_id,d.d+1 FROM depths d JOIN %[1]s.work_order_dependencies dep ON dep.work_order_id=d.id) SELECT id,MAX(d) FROM depths GROUP BY id ORDER BY MAX(d) DESC,id`, schema), plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	works := []int64{}
	for rows.Next() {
		var id int64
		var depth int
		if err = rows.Scan(&id, &depth); err != nil {
			t.Fatal(err)
		}
		works = append(works, id)
	}
	rows.Close()
	for _, work := range works {
		var picking productionapp.StockDocumentPreview
		call("POST", fmt.Sprintf("/api/produce/work-orders/%d/stock-document-preview", work), map[string]any{"action": "issue"}, &picking)
		for _, i := range picking.Document.Items {
			if i.FromWarehouse == i.ToWarehouse || i.QtyG+i.QtyUnits == 0 {
				continue
			}
			_, err := stockapp.NewService(postgresstock.NewRepository(pool, schema)).CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialTransferForManufacture, WorkOrderID: work, Operator: "PR-748验收", Items: []stockapp.StockDocumentItemCommand{{ItemType: i.ItemType, MaterialID: i.MaterialID, ProductID: i.ProductID, BomSpecID: i.BomSpecID, BomVariantID: i.BomVariantID, SpecG: i.SpecG, InventoryUnit: i.InventoryUnit, OwnerCustomerID: i.OwnerCustomerID, QtyG: i.QtyG, QtyUnits: i.QtyUnits, BatchCode: i.BatchCode, FromWarehouse: i.FromWarehouse, ToWarehouse: i.ToWarehouse}}})
			if err != nil {
				t.Fatal(err)
			}
		}

		cards, err := pool.Query(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1 ORDER BY sequence_no,id`, schema), work)
		if err != nil {
			t.Fatal(err)
		}
		ids := []int64{}
		for cards.Next() {
			var id int64
			if err = cards.Scan(&id); err != nil {
				t.Fatal(err)
			}
			ids = append(ids, id)
		}
		cards.Close()
		for _, id := range ids {
			var p productionapp.OperationCompletionPreview
			call("GET", fmt.Sprintf("/api/job-cards/%d/start-preview", id), nil, &p)
			if p.BlockedReason != "" {
				t.Fatalf("start blocked: %+v", p)
			}
			t.Logf("start card %d op %s input %g %s materials %+v", id, p.OperationName, p.InputQty, p.InputUnit, p.Materials)
			wrongVariant := false
			for _, m := range p.Materials {
				if m.ProductID == opt.ReuseProductID && m.ProductID > 0 {
					wrongVariant = true
					if m.AvailableQty < m.Qty || len(m.EligibleBatches) == 0 {
						t.Fatalf("reused product stock missing from preview: %+v", m)
					}
				}
			}
			if wrongVariant {
				if opt.Direct {
					exec(`UPDATE %s.work_order_material_reservations SET status='released' WHERE work_order_id=$1 AND component_type='product'`, work)
					var incomplete productionapp.OperationCompletionPreview
					call("GET", fmt.Sprintf("/api/job-cards/%d/start-preview", id), nil, &incomplete)
					if incomplete.BlockedCode != "bom_material_reservation_missing" {
						t.Fatalf("missing product component did not block start: %+v", incomplete)
					}
					exec(`UPDATE %s.work_order_material_reservations SET status='reserved' WHERE work_order_id=$1 AND component_type='product'`, work)
				}
				exec(`ALTER TABLE %s.stock_batches DISABLE TRIGGER USER`)
				exec(`UPDATE %s.stock_batches SET bom_variant_id=$2 WHERE item_type='finished_product' AND item_id=$1`, opt.ReuseProductID, opt.ReuseVariantID+100000)
				call("GET", fmt.Sprintf("/api/job-cards/%d/start-preview", id), nil, &p)
				raw, _ := json.Marshal(map[string]any{"actual_input_qty": p.InputQty, "input_unit": p.InputUnit, "materials": p.Materials, "expected_version": p.Version, "request_id": "pr747-wrong-variant"})
				request := httptest.NewRequest("POST", fmt.Sprintf("/api/job-cards/%d/start", id), bytes.NewReader(raw))
				request.Header.Set("Content-Type", "application/json")
				result := httptest.NewRecorder()
				e.ServeHTTP(result, request)
				if result.Code == 200 {
					t.Fatal("wrong product variant was consumed")
				}
				var count int
				if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %[1]s.production_started_inputs WHERE job_card_id=$1`, schema), id).Scan(&count); err != nil || count != 0 {
					t.Fatalf("wrong variant partially wrote input: %d %v", count, err)
				}
				exec(`UPDATE %s.stock_batches SET bom_variant_id=$2 WHERE item_type='finished_product' AND item_id=$1`, opt.ReuseProductID, opt.ReuseVariantID)
				exec(`ALTER TABLE %s.stock_batches ENABLE TRIGGER USER`)
				call("GET", fmt.Sprintf("/api/job-cards/%d/start-preview", id), nil, &p)
			}
			call("POST", fmt.Sprintf("/api/job-cards/%d/start", id), map[string]any{"actual_input_qty": p.InputQty, "input_unit": p.InputUnit, "materials": p.Materials, "expected_version": p.Version, "request_id": fmt.Sprintf("pr747-start-%d", id)}, nil)
			if wrongVariant {
				call("GET", fmt.Sprintf("/api/job-cards/%d/start-preview", id), nil, &p)
				call("POST", fmt.Sprintf("/api/job-cards/%d/start-reversal", id), map[string]any{"expected_version": p.Version, "request_id": "pr747-bag-reversal", "reason": "误开工，袋装未投入加工", "unprocessed_confirmed": true}, nil)
				var remaining int64
				if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT remaining_units FROM %s.stock_batches WHERE item_type='finished_product' AND item_id=$1 AND batch_code='PR748-EXISTING-BAGS'`, schema), opt.ReuseProductID).Scan(&remaining); err != nil || remaining != 1000 {
					t.Fatalf("product return failed original batch: %d %v", remaining, err)
				}
				call("GET", fmt.Sprintf("/api/job-cards/%d/start-preview", id), nil, &p)
				call("POST", fmt.Sprintf("/api/job-cards/%d/start", id), map[string]any{"actual_input_qty": p.InputQty, "input_unit": p.InputUnit, "materials": p.Materials, "expected_version": p.Version, "request_id": "pr747-bag-restart"}, nil)
			}
			call("GET", fmt.Sprintf("/api/job-cards/%d/completion-preview", id), nil, &p)
			call("POST", fmt.Sprintf("/api/job-cards/%d/complete", id), map[string]any{"actual_input_qty": p.InputQty, "actual_output_qty": p.PlannedQty, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": fmt.Sprintf("pr747-complete-%d", id), "finish_operation": true, "actual_minutes": 10, "difference_reason": "PR748按计划逐层生产验收"}, nil)
		}
	}
	var qty, cost float64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(qty_units),0)::float8,COALESCE(SUM(qty_units*unit_cost),0)::float8 FROM %s.stock_batches WHERE item_type='finished_product' AND item_id=$1 AND bom_spec_id=$2`, schema), product, spec).Scan(&qty, &cost); err != nil {
		t.Fatal(err)
	}
	if math.Abs(qty-10) > 1e-8 || cost <= 0 {
		t.Fatalf("final receipt qty/cost=%v/%v", qty, cost)
	}
	var expectedCost float64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE((SELECT SUM(c.cost-COALESCE((SELECT SUM(r.cost) FROM %[1]s.production_started_input_returns r WHERE r.component_id=c.id),0)) FROM %[1]s.production_started_input_components c LEFT JOIN %[1]s.materials m ON c.component_type='material' AND m.id=c.component_id WHERE (c.component_type='material' AND (NOT m.is_semi_finished OR c.component_id=$1)) OR (c.component_type='product' AND c.component_id=$2)),0)::float8+COALESCE((SELECT SUM(operation_cost) FROM %[1]s.production_operation_events WHERE status='posted'),0)::float8`, schema), opt.ReuseMaterialID, opt.ReuseProductID).Scan(&expectedCost); err != nil {
		t.Fatal(err)
	}
	if math.Abs(cost-expectedCost) > 0.00001 {
		t.Fatalf("full chain cost is not conserved: final=%v purchased/existing inputs plus operations=%v", cost, expectedCost)
	}
	var residual float64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(remaining_qty+remaining_cost),0)::float8 FROM %s.production_started_inputs`, schema)).Scan(&residual); err != nil {
		t.Fatal(err)
	}
	if residual != 0 {
		t.Fatalf("unsettled inputs=%v", residual)
	}
	var starts, loggedStarts, completions, loggedCompletions int
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT
 (SELECT COUNT(*) FROM %[1]s.production_operation_events WHERE kind='start' AND status='posted'),
 (SELECT COUNT(*) FROM %[1]s.audit_logs WHERE entity_type='job_card' AND action='start_consumption'),
 (SELECT COUNT(*) FROM %[1]s.production_operation_events WHERE kind='complete' AND status='posted'),
 (SELECT COUNT(*) FROM %[1]s.audit_logs WHERE entity_type='job_card' AND action='complete')`, schema)).Scan(&starts, &loggedStarts, &completions, &loggedCompletions); err != nil {
		t.Fatal(err)
	}
	if starts == 0 || starts != loggedStarts || completions == 0 || completions != loggedCompletions {
		t.Fatalf("production events missing audit records: start %d/%d, complete %d/%d", starts, loggedStarts, completions, loggedCompletions)
	}
	t.Logf("PR748 template %d: %d BOMs, %d work orders, final receipt %g %s, cost %.6f", template, want, len(works), qty, unit, cost)
}

type pr747ProductionOptions struct {
	ReuseMaterialID, ReuseProductID, ReuseSpecID, ReuseVariantID int64
	ExpectedLayers                                               int
	Direct                                                       bool
}

func TestPR748ExistingTemplateReuseProduction(t *testing.T) {
	for _, kind := range []string{"roasted", "powder", "bag"} {
		t.Run(kind, func(t *testing.T) {
			pool, schema, svc := pr747CreatorFixture(t)
			ctx := context.Background()
			template := int64(2)
			if kind == "roasted" {
				template = 1
			}
			pr747Create(t, svc, template, "PR748-existing", nil)
			opt := pr747ProductionOptions{}
			overrides := map[string]map[string]any{}
			if kind == "bag" {
				if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT p.id,s.id,v.id FROM %[1]s.products p JOIN %[1]s.production_boms b ON b.output_product_id=p.id JOIN %[1]s.production_bom_versions bv ON bv.bom_id=b.id AND bv.status='published' JOIN %[1]s.production_bom_version_variants v ON v.version_id=bv.id JOIN %[1]s.production_bom_specs s ON s.id=v.bom_spec_id WHERE p.name LIKE '挂耳-%%' AND s.spec_key='spec-1'`, schema)).Scan(&opt.ReuseProductID, &opt.ReuseSpecID, &opt.ReuseVariantID); err != nil {
					t.Fatal(err)
				}
				overrides[pr747Bag] = map[string]any{"action": "reuse", "product_id": opt.ReuseProductID}
				overrides[pr747Box] = map[string]any{"main_input_source_node_id": pr747Bag, "main_input_source_row_id": fmt.Sprint(opt.ReuseSpecID)}
				opt.ExpectedLayers = 1
			} else {
				prefix := "PR748-existing_"
				node := pr747Roasted
				opt.ExpectedLayers = 1
				if kind == "powder" {
					prefix = "咖啡粉_PR748-existing"
					node = pr747Powder
					opt.ExpectedLayers = 2
				}
				if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.materials WHERE name LIKE $1 AND is_semi_finished`, schema), prefix+"%%").Scan(&opt.ReuseMaterialID); err != nil {
					t.Fatal(err)
				}
				overrides[node] = map[string]any{"action": "reuse", "material_id": opt.ReuseMaterialID}
			}
			var before int
			pool.QueryRow(ctx, fmt.Sprintf("SELECT count(*) FROM %s.production_boms", schema)).Scan(&before)
			run := pr747Create(t, svc, template, "PR748-reuse-"+kind, overrides)
			var after int
			pool.QueryRow(ctx, fmt.Sprintf("SELECT count(*) FROM %s.production_boms", schema)).Scan(&after)
			if after-before != opt.ExpectedLayers {
				t.Fatalf("reuse created %d BOMs instead of %d", after-before, opt.ExpectedLayers)
			}
			steps := run.BusinessResults["steps"].(map[string]any)
			if steps[pr747Roast].(map[string]any)["status"] != "skipped" {
				t.Fatal("reuse did not skip roasting")
			}
			pr747ProductionChain(t, pool, schema, template, opt)
			var upstreamConsumed int
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %[1]s.production_started_input_components c JOIN %[1]s.materials m ON c.component_type='material' AND m.id=c.component_id WHERE m.name LIKE '%%生豆'`, schema)).Scan(&upstreamConsumed); err != nil {
				t.Fatal(err)
			}
			if upstreamConsumed != 0 {
				t.Fatal("reused inventory consumed upstream raw material")
			}
		})
	}
}

func TestPR748CreatorRejectsIncompleteRouteWithoutRequiringInventory(t *testing.T) {
	pool, schema, svc := pr747CreatorFixture(t)
	ctx := context.Background()
	source := pr747Create(t, svc, 1, "PR748-zero-stock", nil)
	var stock int
	if err := pool.QueryRow(ctx, fmt.Sprintf("SELECT count(*) FROM %s.material_batches", schema)).Scan(&stock); err != nil {
		t.Fatal(err)
	}
	if stock != 0 {
		t.Fatal("creation must not require inventory")
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf("DELETE FROM %s.process_route_operations WHERE route_id=5", schema)); err != nil {
		t.Fatal(err)
	}
	run, err := svc.StartRun(ctx, 1, "PR-748验收")
	if err != nil {
		t.Fatal(err)
	}
	run, err = svc.SaveRunDraft(ctx, run.ID, run.Revision, source.Inputs, map[string]string{"bfceff1c-bc42-4284-acb6-d68554a78950": "PR748-broken-route"}, "PR-748验收")
	if err != nil {
		t.Fatal(err)
	}
	run, err = svc.PreviewRun(ctx, run.ID, run.Revision, "PR-748验收")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, i := range run.Preview.Issues {
		if i.Code == "process_route_invalid" {
			found = true
		}
	}
	if run.Preview.Valid || !found {
		t.Fatalf("empty route accepted: %+v", run.Preview.Issues)
	}
}

func TestPR748DirectCreateFromReusedRoastProduction(t *testing.T) {
	pool, schema, svc := pr747CreatorFixture(t)
	ctx := context.Background()
	pr747Create(t, svc, 1, "PR748-direct-source", nil)
	var id int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.materials WHERE is_semi_finished AND name LIKE 'PR748-direct-source%%'`, schema)).Scan(&id); err != nil {
		t.Fatal(err)
	}
	pr747Create(t, svc, 1, "PR748-direct", map[string]map[string]any{pr747Roasted: {"action": "reuse", "material_id": id}})
	pr747ProductionChain(t, pool, schema, 1, pr747ProductionOptions{ReuseMaterialID: id, ExpectedLayers: 1, Direct: true})
}

func TestPR748DirectCreateFromReusedBagProduction(t *testing.T) {
	pool, schema, svc := pr747CreatorFixture(t)
	ctx := context.Background()
	pr747Create(t, svc, 2, "PR748-direct-bag-source", nil)
	opt := pr747ProductionOptions{ExpectedLayers: 1, Direct: true}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT p.id,s.id,v.id FROM %[1]s.products p JOIN %[1]s.production_boms b ON b.output_product_id=p.id JOIN %[1]s.production_bom_versions bv ON bv.bom_id=b.id AND bv.status='published' JOIN %[1]s.production_bom_version_variants v ON v.version_id=bv.id JOIN %[1]s.production_bom_specs s ON s.id=v.bom_spec_id WHERE p.name LIKE '挂耳-%%' AND s.spec_key='spec-1'`, schema)).Scan(&opt.ReuseProductID, &opt.ReuseSpecID, &opt.ReuseVariantID); err != nil {
		t.Fatal(err)
	}
	pr747Create(t, svc, 2, "PR748-direct-bag", map[string]map[string]any{
		pr747Bag: {"action": "reuse", "product_id": opt.ReuseProductID},
		pr747Box: {"main_input_source_node_id": pr747Bag, "main_input_source_row_id": fmt.Sprint(opt.ReuseSpecID)},
	})
	pr747ProductionChain(t, pool, schema, 2, opt)
}
