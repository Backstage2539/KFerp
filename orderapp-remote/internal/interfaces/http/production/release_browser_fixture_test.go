package production

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	authzapp "orderapp/internal/application/authz"
	bomapp "orderapp/internal/application/bom"
	catalogapp "orderapp/internal/application/catalog"
	manufacturingapp "orderapp/internal/application/manufacturing"
	productionapp "orderapp/internal/application/production"
	stockapp "orderapp/internal/application/stock"
	authzpg "orderapp/internal/infrastructure/postgres/authz"
	bompg "orderapp/internal/infrastructure/postgres/bom"
	catalogpg "orderapp/internal/infrastructure/postgres/catalog"
	manufacturingpg "orderapp/internal/infrastructure/postgres/manufacturing"
	productionpg "orderapp/internal/infrastructure/postgres/production"
	stockpg "orderapp/internal/infrastructure/postgres/stock"
	bomhttp "orderapp/internal/interfaces/http/bom"
	cataloghttp "orderapp/internal/interfaces/http/catalog"
	manufacturinghttp "orderapp/internal/interfaces/http/manufacturing"
	stockhttp "orderapp/internal/interfaces/http/stock"
	supporthttp "orderapp/internal/interfaces/http/support"
)

// This opt-in browser fixture serves the real Vue app and real business handlers.
// All writes are restricted to one disposable local database and test schema.
func seedReleaseRoastingBrowserFixture(t *testing.T) (*pgxpool.Pool, string, *echo.Echo, int64, []int64) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	app, _, workOrderID := createSubmittedFullStockTypedPlan(t, ctx, pool, schema, 87301)
	start := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/start", workOrderID), nil)
	if start.Code != http.StatusOK {
		t.Fatalf("start work order status=%d body=%s", start.Code, start.Body.String())
	}
	seedMaterial := func(id int64, code, name string) {
		t.Helper()
		if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.materials(id,code,name,kind,unit,cost_unit,onhand_g,onhand_units,purchase_price,sale_price) VALUES($1,$2,$3,'bean','kg','kg',5000,0,1,0)`, schema), id, code, name); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.materials SET name=CASE id WHEN 10 THEN '孟连水洗A' WHEN 30 THEN '孟连水洗5T' ELSE name END WHERE id IN(10,30)`, schema)); err != nil {
		t.Fatal(err)
	}
	seedMaterial(40, "ROAST-REFRESH-NEW-B", "巴布亚之光")
	seedMaterial(50, "ROAST-REFRESH-OLD", "已移除旧配料")
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 87302, 30, "ROAST-REFRESH-BATCH-A", "孟连水洗5T", 5_000)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 87303, 40, "ROAST-REFRESH-BATCH-B", "巴布亚之光", 5_000)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 87304, 50, "ROAST-REFRESH-BATCH-OLD", "已移除旧配料", 5_000)

	// V001 is a frozen, incomplete 70% recipe. V002 is the newer complete
	// published version of the same BOM, with a new component identity.
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %[1]s.production_bom_versions SET status='archived' WHERE id=100;
		DELETE FROM %[1]s.production_bom_version_items WHERE version_id=100;
		INSERT INTO %[1]s.production_bom_version_items(version_id,material_id,component_type,ratio_pct)
		VALUES (100,10,'material',50),(100,50,'material',20);
		INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,yield_rate,material_loss_rate,output_qty,output_unit,process_route_id,published_at)
		VALUES (101,100,'V002','published',1,0,1,'kg',32,now()+interval '1 second');
		INSERT INTO %[1]s.production_bom_version_items(version_id,material_id,component_type,ratio_pct)
		VALUES (101,10,'material',70),(101,30,'material',15),(101,40,'material',15);
	`, schema))

	var originalReservationID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.work_order_material_reservations WHERE work_order_id=$1 AND material_id=10 AND status='reserved'`, schema), workOrderID).Scan(&originalReservationID); err != nil {
		t.Fatalf("find original BOM reservation: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_order_material_reservations SET required_g=1500,reserved_g=1500,source_warehouse='wip',source_owner_customer_id=0 WHERE id=$1`, schema), originalReservationID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_order_material_reservation_batches SET reserved_g=1500,reserved_units=0,warehouse='wip',owner_customer_id=0 WHERE reservation_id=$1`, schema), originalReservationID); err != nil {
		t.Fatal(err)
	}
	var staleReservationID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.work_order_material_reservations(work_order_id,running_item_id,material_id,material_name,unit,required_g,reserved_g,component_type,component_id,source_warehouse,source_owner_customer_id,status)
		SELECT $1,running_item_id,50,'已移除旧配料','kg',600,600,'material',50,'wip',0,'reserved'
		FROM %s.work_orders WHERE id=$1 RETURNING id`, schema, schema), workOrderID).Scan(&staleReservationID); err != nil {
		t.Fatalf("seed stale reservation: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.work_order_material_reservation_batches(reservation_id,work_order_id,material_id,component_type,component_id,material_batch_id,batch_code,warehouse,owner_customer_id,reserved_g,status) VALUES($1,$2,50,'material',50,87304,'ROAST-REFRESH-BATCH-OLD','wip',0,600,'reserved')`, schema), staleReservationID, workOrderID); err != nil {
		t.Fatal(err)
	}

	route := `{"operations":[{"seq":1,"operation":"咖啡烘焙+除石","workstation":"智烘"}]}`
	frozen := `[{"material_id":10,"material_name":"在制熟豆","unit":"kg","ratio_pct":50,"source":"bom","component_type":"material","consume_unit":"ratio_pct","output_qty":1,"output_unit":"kg"},{"material_id":50,"material_name":"已移除旧配料","unit":"kg","ratio_pct":20,"source":"bom","component_type":"material","consume_unit":"ratio_pct","output_qty":1,"output_unit":"kg"}]`
	bindings := `{"material:10:0:0":{"node_key":"roast","source_warehouse":"wip"},"material:50:0:0":{"node_key":"roast","source_warehouse":"wip"},"material:20:0:0":{"node_key":"pack","source_warehouse":"packaging"}}`
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET bom_version_id=100,bom_variant_id=0,material_snapshot=$2::jsonb,process_snapshot_json=$3::jsonb,operation_material_bindings=$4::jsonb,planned_g=6000,planned_output_g=4994,planned_inventory_qty=4.994,output_qty=4.994,output_unit='kg',inventory_unit='kg',target_warehouse='wip',status='running' WHERE id=$1`, schema), workOrderID, frozen, route, bindings); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %[1]s.job_cards SET sequence_no=1,operation='咖啡烘焙+除石',status='running',started_at=now()-interval '5 minutes',completed_at=NULL,planned_input_qty=3000,actual_input_qty=0,actual_output_qty=0 WHERE work_order_id=%[2]d;
		INSERT INTO %[1]s.job_cards(work_order_id,sequence_no,operation,status,planned_input_qty) VALUES(%[2]d,1,'咖啡烘焙+除石','pending',3000);
		UPDATE %[1]s.work_order_material_reservations SET required_g=1500,reserved_g=1500 WHERE id=%[3]d;
		UPDATE %[1]s.work_order_material_reservation_batches SET reserved_g=1500 WHERE reservation_id=%[3]d;
	`, schema, workOrderID, originalReservationID))

	var firstCardID, secondCardID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1 AND sequence_no=1 ORDER BY id LIMIT 1`, schema), workOrderID).Scan(&firstCardID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1 AND sequence_no=1 ORDER BY id DESC LIMIT 1`, schema), workOrderID).Scan(&secondCardID); err != nil {
		t.Fatal(err)
	}

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.material_batches SET unit_cost=CASE material_id WHEN 10 THEN 80 WHEN 30 THEN 50 WHEN 40 THEN 60 ELSE 10 END;`, schema))
	return pool, schema, app, workOrderID, []int64{firstCardID, secondCardID}
}

func TestProductionReleaseBrowserFixture(t *testing.T) {
	runProductionReleaseBrowserFixture(t, false, false)
}
func TestProductionReleasePackagingBrowserFixture(t *testing.T) {
	runProductionReleaseBrowserFixture(t, true, false)
}

func TestProductionReleaseBOMReturnBrowserFixture(t *testing.T) {
	runProductionReleaseBrowserFixture(t, false, true)
}

func runProductionReleaseBrowserFixture(t *testing.T, packaging, bomDraft bool) {
	readyName, port, stationName, stationCode := "ORDERAPP_RELEASE_BROWSER_READY", 18661, "隔离智烘", "RELEASE-ROAST"
	if packaging {
		readyName, port, stationName, stationCode = "ORDERAPP_RELEASE_PACK_BROWSER_READY", 18662, "隔离包装台", "RELEASE-PACK"
	}
	if bomDraft {
		readyName, port = "ORDERAPP_RELEASE_BOM_BROWSER_READY", 18663
	}
	ready := os.Getenv(readyName)
	if ready == "" {
		t.Skip("opt-in isolated production release browser fixture")
	}
	cfg, err := pgxpool.ParseConfig(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if err != nil || cfg.ConnConfig.Database != "kferp_release_20261026" || cfg.ConnConfig.Host != "/tmp" {
		t.Fatal("browser fixture only permits the dedicated local release database")
	}
	seed := seedReleaseRoastingBrowserFixture
	if packaging {
		seed = seedReleasePackagingBrowserFixture
	}
	pool, schema, e, woID, cards := seed(t)
	e.Use(supporthttp.OperationLogMiddleware(pool, schema))
	ctx := context.Background()
	if err := authzpg.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.company_departments(id,name,active) VALUES(873,'隔离发布验收',true) ON CONFLICT(id) DO NOTHING;
 INSERT INTO %[1]s.company_employees(id,name,phone,department_id,active,account_type) VALUES(1,'隔离验收员','19900000873',873,true,'internal_employee') ON CONFLICT(id) DO UPDATE SET name=EXCLUDED.name,active=true;
 INSERT INTO %[1]s.employee_roles(employee_id,role_code) VALUES(1,'admin') ON CONFLICT DO NOTHING;`, schema))
	ms := manufacturingapp.NewService(manufacturingpg.NewRepository(pool, schema))
	station, err := ms.SaveManufacturingWorkstation(ctx, manufacturingapp.SaveManufacturingWorkstationCommand{Name: stationName, Code: stationCode, Status: "active", PrimaryEmployeeID: 1, Actor: "隔离验收"})
	if err != nil {
		t.Fatal(err)
	}
	if bomDraft {
		seedReleaseRoastingDraftSpecification(t, ctx, pool, schema, station.ID)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.job_cards SET workstation_id=$1,workstation=$3,assigned_employee_id=0,assigned_to='' WHERE id=ANY($2)`, schema), station.ID, cards, stationName); err != nil {
		t.Fatal(err)
	}
	prod := productionapp.NewService(productionpg.NewRepository(pool, schema))
	if packaging || bomDraft {
		loc, _ := time.LoadLocation("Asia/Shanghai")
		today := time.Now().In(loc).Format("2006-01-02")
		week, days, _ := productionapp.NormalizeProductionRosterWeek(today, loc)
		entries := []productionapp.ProductionAttendanceEntry{}
		for _, day := range days {
			entries = append(entries, productionapp.ProductionAttendanceEntry{EmployeeID: 1, WorkDate: day, Status: "working"})
		}
		if _, err := prod.SaveProductionRoster(ctx, productionapp.SaveProductionRosterCommand{WeekStart: week, Entries: entries, RequestID: "isolated-packaging-roster", Operator: "隔离验收"}, false); err != nil {
			t.Fatal(err)
		}
	}
	stock := stockapp.NewService(stockpg.NewRepository(pool, schema))
	RegisterRoutes(e, Dependencies{Production: prod, Stock: stock})
	bomhttp.RegisterRoutes(e, bomhttp.Dependencies{Bom: bomapp.NewService(bompg.NewRepository(pool, schema))})
	cataloghttp.RegisterRoutes(e, cataloghttp.Dependencies{Catalog: catalogapp.NewService(catalogpg.NewRepository(pool, schema))})
	manufacturinghttp.RegisterRoutes(e, manufacturinghttp.Dependencies{Manufacturing: ms})
	stockhttp.RegisterRoutes(e, stockhttp.Dependencies{Stock: stock})
	auth := authzapp.NewService(authzpg.NewRepository(pool, schema))
	e.GET("/api/auth/me", func(c echo.Context) error {
		a, err := auth.ActorByEmployeeID(c.Request().Context(), 1)
		if err != nil {
			return err
		}
		return c.JSON(200, a)
	})
	e.GET("/fixture/info", func(c echo.Context) error {
		info := map[string]any{"schema": schema, "work_order_id": woID, "job_card_ids": cards, "workstation_id": station.ID, "bom_id": 100}
		if packaging {
			info["expected_source_bag_quantities"] = []int{10, 6, 4, 1}
			info["expected_completion_parts"] = []int{6, 15}
		} else {
			info["expected_input_kg"] = 3
			info["expected_output_kg"] = 2.497
		}
		return c.JSON(200, info)
	})
	e.GET("/fixture/ledger", func(c echo.Context) error {
		out := map[string]json.RawMessage{}
		queries := map[string]string{
			"work_orders":       fmt.Sprintf(`SELECT COALESCE(json_agg(x),'[]') FROM (SELECT id,work_order_no,status,bom_version_id,output_qty,output_unit FROM %s.work_orders ORDER BY id)x`, schema),
			"job_cards":         fmt.Sprintf(`SELECT COALESCE(json_agg(x),'[]') FROM (SELECT id,work_order_id,status,actual_input_qty,actual_output_qty,completed_at FROM %s.job_cards ORDER BY id)x`, schema),
			"stock_entries":     fmt.Sprintf(`SELECT COALESCE(json_agg(x),'[]') FROM (SELECT id,work_order_id,entry_type,purpose,status FROM %s.stock_entries ORDER BY id)x`, schema),
			"stock_entry_items": fmt.Sprintf(`SELECT COALESCE(json_agg(x),'[]') FROM (SELECT stock_entry_id,material_id,item_type,product_id,qty_g,qty_units FROM %s.stock_entry_items ORDER BY id)x`, schema),
			"events":            fmt.Sprintf(`SELECT COALESCE(json_agg(x),'[]') FROM (SELECT id,job_card_id,kind,status,input_qty,output_qty FROM %s.production_operation_events ORDER BY id)x`, schema),
			"audit":             fmt.Sprintf(`SELECT COALESCE(json_agg(x),'[]') FROM (SELECT id,entity_type,entity_id,action,meta FROM %s.audit_logs ORDER BY id)x`, schema),
		}
		for k, q := range queries {
			var data []byte
			if err := pool.QueryRow(c.Request().Context(), q).Scan(&data); err != nil {
				return c.JSON(500, map[string]string{"error": k + ": " + err.Error()})
			}
			out[k] = data
		}
		return c.JSON(200, out)
	})
	target, _ := url.Parse("http://127.0.0.1:18660")
	proxy := httputil.NewSingleHostReverseProxy(target)
	e.GET("/fixture", func(c echo.Context) error {
		return c.HTML(200, `<!doctype html><meta charset="utf-8"><script>localStorage.setItem('auth_token','isolated-release-fixture');location.href='/app/vue-shell?view=workstationView&scope=all';</script>`)
	})
	e.Any("/vue-shell*", func(c echo.Context) error {
		if c.Request().URL.Path == "/vue-shell" {
			c.Request().URL.Path = "/vue-shell/index.html"
		}
		proxy.ServeHTTP(c.Response(), c.Request())
		return nil
	})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/app/") {
			r.URL.Path = strings.TrimPrefix(r.URL.Path, "/app")
		}
		e.ServeHTTP(w, r)
	})
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	go server.Serve(listener)
	defer server.Shutdown(context.Background())
	info, _ := json.Marshal(map[string]any{"url": fmt.Sprintf("http://127.0.0.1:%d/fixture", port), "schema": schema, "work_order_id": woID, "job_card_ids": cards, "workstation_id": station.ID})
	if err := os.WriteFile(ready, info, 0600); err != nil {
		t.Fatal(err)
	}
	t.Log(string(info))
	for {
		time.Sleep(time.Second)
		if _, err := os.Stat(ready); os.IsNotExist(err) {
			return
		}
	}
}

// The old work order intentionally has no variant identity. Its replacement
// draft must nevertheless satisfy today's product-BOM publication contract.
func seedReleaseRoastingDraftSpecification(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string, stationID int64) {
	t.Helper()
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %[1]s.production_boms SET specification_mode='spec_group' WHERE id=100;
		UPDATE %[1]s.production_bom_versions SET status='draft',published_at=NULL WHERE id=101;
		INSERT INTO %[1]s.product_unit_definitions(code,name,unit_type,allow_decimal,active)
		VALUES('kg','千克','weight',true,true) ON CONFLICT(code) DO UPDATE SET active=true,deleted_at=NULL;
		INSERT INTO %[1]s.production_bom_specs(id,bom_id,code,spec_key,name,inventory_unit)
		VALUES(87301,100,'RELEASE-ROAST-KG','roast-kg','烘焙散装 kg','kg');
		INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,logistics_weight_g,process_route_id)
		VALUES(87301,101,87301,'烘焙散装 kg','kg',true,1000,32);
		UPDATE %[1]s.production_bom_version_items SET variant_id=87301 WHERE version_id=101;
		INSERT INTO %[1]s.manufacturing_operations(id,name,status) VALUES(87301,'咖啡烘焙+除石','active');
		INSERT INTO %[1]s.manufacturing_workstation_operations(workstation_id,operation_id) VALUES(%[2]d,87301);
		INSERT INTO %[1]s.manufacturing_workstation_capacities(id,workstation_id,code,name,status,batch_size_qty,batch_size_unit,standard_minutes,hourly_rate)
		VALUES(87301,%[2]d,'RELEASE-ROAST-3KG','隔离烘焙 3kg','active',3,'kg',30,60);
		UPDATE %[1]s.process_route_operations SET operation_id=87301,operation='咖啡烘焙+除石',standard_cost_capacity_id=87301 WHERE route_id=32;
	`, schema, stationID))
	if err := bompg.NewRepository(pool, schema).ValidateProductionBomVersionForPublish(ctx, bomapp.PublishProductionBomVersionCommand{VersionID: 101, Actor: "隔离夹具校验"}); err != nil {
		t.Fatalf("isolated BOM draft must pass the real publication validator: %v", err)
	}
}

func seedReleasePackagingBrowserFixture(t *testing.T) (*pgxpool.Pool, string, *echo.Echo, int64, []int64) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	app, _, sourceWorkOrderID := createSubmittedFullStockTypedPlan(t, ctx, pool, schema, 87325)
	registerProductionWorkstationAPI(app, productionapp.NewService(productionpg.NewRepository(pool, schema)))

	// Model three old packaging cards whose bag quantities live on their work
	// orders while planned_input_qty is zero. The cards share the same frozen
	// product, BOM, route, materials, and target warehouse.
	workOrderIDs := []int64{sourceWorkOrderID}
	for _, entry := range []struct {
		no  string
		qty float64
	}{{"WO-ISOLATED-PACK-2", 6}, {"WO-ISOLATED-PACK-3", 4}, {"WO-ISOLATED-PACK-4", 1}} {
		var id int64
		err := pool.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.work_orders(
				work_order_no,running_item_id,production_plan_id,production_plan_item_id,batch_id,
				output_type,output_product_id,output_material_id,output_name,output_qty,output_unit,
				product_id,bom_spec_id,bom_variant_id,product_name,spec_g,inventory_unit,planned_inventory_qty,
				planned_g,planned_output_g,status,material_snapshot,bom_version_id,operation_template_id,
				process_template_id,process_template_name,process_snapshot_json,operation_summary_json,
				production_config_snapshot_json,customer_product_snapshot_json,customer_id,target_warehouse,
				sales_spec_snapshot_json,operation_material_bindings
			)
			SELECT $2,0,production_plan_id,production_plan_item_id,batch_id,
				output_type,output_product_id,output_material_id,output_name,$3,'袋',
				product_id,bom_spec_id,bom_variant_id,product_name,spec_g,'袋',$3,
				planned_g,planned_output_g,'released',material_snapshot,bom_version_id,operation_template_id,
				process_template_id,process_template_name,process_snapshot_json,operation_summary_json,
				production_config_snapshot_json,customer_product_snapshot_json,customer_id,target_warehouse,
				sales_spec_snapshot_json,operation_material_bindings
			FROM %s.work_orders WHERE id=$1 RETURNING id`, schema, schema), sourceWorkOrderID, entry.no, entry.qty).Scan(&id)
		if err != nil {
			t.Fatalf("clone packaging work order: %v", err)
		}
		if _, err = pool.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.job_cards(work_order_id,sequence_no,operation,workstation,planned_input_qty,actual_input_qty,actual_output_qty,status)
			SELECT $2,sequence_no,operation,workstation,0,0,0,'pending'
			FROM %s.job_cards WHERE work_order_id=$1 ORDER BY id LIMIT 1`, schema, schema), sourceWorkOrderID, id); err != nil {
			t.Fatalf("clone packaging job card: %v", err)
		}
		if _, err = pool.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.work_order_material_reservations(
				work_order_id,running_item_id,material_id,material_name,unit,required_g,required_units,
				reserved_g,reserved_units,consumed_g,consumed_units,returned_g,returned_units,status,
				source_warehouse,source_owner_customer_id,component_type,component_id,
				component_bom_spec_id,component_bom_variant_id,component_spec_g
			)
			SELECT $2,0,material_id,material_name,unit,
				round(required_g*$3::numeric)::bigint,round(required_units*$3::numeric)::bigint,
				round(reserved_g*$3::numeric)::bigint,round(reserved_units*$3::numeric)::bigint,
				0,0,0,0,'reserved',source_warehouse,source_owner_customer_id,
				component_type,component_id,component_bom_spec_id,component_bom_variant_id,component_spec_g
			FROM %s.work_order_material_reservations WHERE work_order_id=$1 AND status='reserved'`, schema, schema), sourceWorkOrderID, id, entry.qty/100); err != nil {
			t.Fatalf("clone packaging material reservations: %v", err)
		}
		workOrderIDs = append(workOrderIDs, id)
	}

	// The original plan reserved the whole 100-bag order. Reduce its legacy
	// reservation and frozen work-order quantity to the first 10-bag source;
	// the other two reservations above represent 8 and 2 bags.
	for _, statement := range []string{
		fmt.Sprintf(`UPDATE %s.work_orders SET status='released',output_qty=10,output_unit='袋',inventory_unit='',planned_inventory_qty=0,target_warehouse='finished_goods' WHERE id=$1`, schema),
		fmt.Sprintf(`UPDATE %s.job_cards SET planned_input_qty=0,actual_input_qty=0,actual_output_qty=0,status='pending' WHERE work_order_id=$1`, schema),
		fmt.Sprintf(`UPDATE %s.work_order_material_reservations SET required_g=round(required_g*0.1)::bigint,required_units=round(required_units*0.1)::bigint,reserved_g=round(reserved_g*0.1)::bigint,reserved_units=round(reserved_units*0.1)::bigint WHERE work_order_id=$1 AND status='reserved'`, schema),
		fmt.Sprintf(`UPDATE %s.work_order_material_reservation_batches SET reserved_g=round(reserved_g*0.1)::bigint,reserved_units=round(reserved_units*0.1)::bigint WHERE work_order_id=$1 AND status='reserved'`, schema),
	} {
		if _, err := pool.Exec(ctx, statement, sourceWorkOrderID); err != nil {
			t.Fatalf("normalize first legacy packaging work order: %v", err)
		}
	}
	for i, qty := range []float64{6, 4, 1} {
		if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET output_qty=$2,inventory_unit='',planned_inventory_qty=0,target_warehouse='finished_goods' WHERE id=$1`, schema), workOrderIDs[i+1], qty); err != nil {
			t.Fatalf("set cloned packaging quantity: %v", err)
		}
	}
	cardIDs := make([]int64, 0, len(workOrderIDs))
	for _, id := range workOrderIDs {
		var cardID int64
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1`, schema), id).Scan(&cardID); err != nil {
			t.Fatalf("load grouped packaging card: %v", err)
		}
		cardIDs = append(cardIDs, cardID)
	}

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.work_orders SET inventory_unit='袋',planned_inventory_qty=output_qty,planned_g=round(output_qty*227)::bigint,planned_output_g=round(output_qty*227)::bigint,sales_spec_count=output_qty; UPDATE %s.work_orders SET work_order_no='WO-ISOLATED-PACK-1' WHERE id=1;`, schema, schema))
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.material_batches SET unit_cost=CASE material_id WHEN 10 THEN 80 WHEN 20 THEN 1 ELSE 0 END;`, schema))
	// Keep each source order and batch identity. Initial stock is sufficient; the
	// separately exercised material-issue API regressions cover source transfers.
	return pool, schema, app, sourceWorkOrderID, cardIDs
}
