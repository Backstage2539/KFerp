package production

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	productionapp "orderapp/internal/application/production"
	postgresproduction "orderapp/internal/infrastructure/postgres/production"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func TestProductionPlanDraftCancelIsConcurrentAndIdempotent(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedProductionPlanLifecycleData(t, ctx, pool, schema)

	repo := postgresproduction.NewRepository(pool, schema)
	plan, err := repo.CreateProductionPlan(ctx, productionapp.CreateProductionPlanCommand{
		Selected: map[string]bool{"1-227": true},
		Operator: "计划员",
	})
	if err != nil {
		t.Fatalf("CreateProductionPlan: %v", err)
	}

	app := newProductionFlowTestEcho(pool, schema)
	results := make(chan int, 2)
	start := make(chan struct{})
	for range 2 {
		go func() {
			<-start
			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/production-plans/%d/cancel", plan.ID), nil)
			rec := httptest.NewRecorder()
			app.ServeHTTP(rec, req)
			results <- rec.Code
		}()
	}
	close(start)
	for range 2 {
		if code := <-results; code != http.StatusOK {
			t.Fatalf("concurrent cancel status = %d, want 200", code)
		}
	}

	assertProductionFlowCount(t, pool, schema, "production_plans", fmt.Sprintf("id=%d AND status='cancelled' AND cancelled_at IS NOT NULL", plan.ID), 1)
	assertProductionFlowCount(t, pool, schema, "work_orders", fmt.Sprintf("production_plan_id=%d", plan.ID), 0)
	assertProductionFlowCount(t, pool, schema, "audit_logs", fmt.Sprintf("entity_type='production_plan' AND entity_id=%d AND action='cancel'", plan.ID), 1)
}

func TestProductionPlanSubmitAndDraftCancelAreSerialized(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedProductionPlanLifecycleData(t, ctx, pool, schema)

	repo := postgresproduction.NewRepository(pool, schema)
	plan, err := repo.CreateProductionPlan(ctx, productionapp.CreateProductionPlanCommand{
		Selected: map[string]bool{"1-227": true},
		Operator: "计划员",
	})
	if err != nil {
		t.Fatalf("CreateProductionPlan: %v", err)
	}

	app := newProductionFlowTestEcho(pool, schema)
	type transitionResult struct {
		action string
		code   int
		body   string
	}
	results := make(chan transitionResult, 2)
	start := make(chan struct{})
	for action, path := range map[string]string{
		"submit": fmt.Sprintf("/api/production-plans/%d/submit", plan.ID),
		"cancel": fmt.Sprintf("/api/production-plans/%d/cancel", plan.ID),
	} {
		go func(action, path string) {
			<-start
			req := httptest.NewRequest(http.MethodPost, path, nil)
			rec := httptest.NewRecorder()
			app.ServeHTTP(rec, req)
			results <- transitionResult{action: action, code: rec.Code, body: rec.Body.String()}
		}(action, path)
	}
	close(start)

	got := []transitionResult{<-results, <-results}
	successes := 0
	rejections := 0
	for _, result := range got {
		switch result.code {
		case http.StatusOK:
			successes++
		case http.StatusBadRequest:
			if !strings.Contains(result.body, "must be draft") {
				t.Fatalf("%s rejected for unexpected reason: %s", result.action, result.body)
			}
			rejections++
		default:
			t.Fatalf("%s status=%d body=%s", result.action, result.code, result.body)
		}
	}
	if successes != 1 || rejections != 1 {
		t.Fatalf("submit/cancel outcomes=%+v, want one success and one draft-state rejection", got)
	}

	var status string
	var workOrderCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT pp.status,COUNT(wo.id)::int
		FROM %s.production_plans pp
		LEFT JOIN %s.work_orders wo ON wo.production_plan_id=pp.id
		WHERE pp.id=$1
		GROUP BY pp.id
	`, schema, schema), plan.ID).Scan(&status, &workOrderCount); err != nil {
		t.Fatalf("load final production plan state: %v", err)
	}
	switch status {
	case "cancelled":
		if workOrderCount != 0 {
			t.Fatalf("cancelled plan has %d work orders, want 0", workOrderCount)
		}
		assertProductionFlowCount(t, pool, schema, "audit_logs", fmt.Sprintf("entity_type='production_plan' AND entity_id=%d AND action='cancel'", plan.ID), 1)
		assertProductionFlowCount(t, pool, schema, "audit_logs", fmt.Sprintf("entity_type='production_plan' AND entity_id=%d AND action='submit'", plan.ID), 0)
	case "submitted":
		if workOrderCount != 1 {
			t.Fatalf("submitted plan has %d work orders, want 1", workOrderCount)
		}
		assertProductionFlowCount(t, pool, schema, "audit_logs", fmt.Sprintf("entity_type='production_plan' AND entity_id=%d AND action='cancel'", plan.ID), 0)
		assertProductionFlowCount(t, pool, schema, "audit_logs", fmt.Sprintf("entity_type='production_plan' AND entity_id=%d AND action='submit'", plan.ID), 1)
	default:
		t.Fatalf("final production plan status = %q, want cancelled or submitted", status)
	}
}

func TestProductionPlanAPIConcurrentCreatePlansDemandOnlyOnce(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.products(
			id,name,parent_product_id,default_price,active,spec_label,net_content_qty,net_content_unit,unit_rule_override_json
		) VALUES
			(1700,'并发计划父商品',0,0,true,'',0,'','{"inventory_unit":"kg"}'::jsonb),
			(1701,'并发计划商品',1700,0,true,'454g',454,'g','{}'::jsonb);
		INSERT INTO %[1]s.order_process_statuses(name,sort,active)
		VALUES ('待处理',10,true)
		ON CONFLICT (name) DO NOTHING;
		INSERT INTO %[1]s.orders(id,order_no,order_date,is_void,process_status_id)
		VALUES (
			1701,'SO-PR553-CONCURRENT','2026-07-25',false,
			(SELECT id FROM %[1]s.order_process_statuses WHERE name='待处理' LIMIT 1)
		);
		INSERT INTO %[1]s.order_items(
			order_id,line_no,item_name,qty,unit,sales_unit,spec,product_id,unit_price,line_total,price_source_json
		) VALUES (
			1701,1,'并发计划商品',4,'454g','454g','不可作为换算来源',1701,0,0,
			'{"production_quantity_snapshot":{"sku_id":1701,"parent_product_id":1700,"spec_label":"454g","sales_unit":"454g","inventory_unit":"kg","inventory_qty_per_sales_unit":0.454,"conversion_source":"published_inventory_conversion"}}'::jsonb
		);
		INSERT INTO %[1]s.materials(id,code,name,kind,unit,onhand_g,onhand_units,purchase_price,sale_price)
		VALUES (1701,'RAW-CONCURRENT','并发计划原料','bean','kg',0,0,50,0);
		INSERT INTO %[1]s.process_routes(id,name,status,default_equipment,default_minutes)
		VALUES (1701,'并发计划路线','active','测试设备',10);
		INSERT INTO %[1]s.process_route_operations(
			route_id,seq,operation,workstation,default_equipment,default_minutes,records_loss
		) VALUES (1701,1,'测试工序','测试工位','测试设备',10,false);
		INSERT INTO %[1]s.production_boms(id,code,name,output_product_id,status)
		VALUES (1701,'PBOM-CONCURRENT','并发计划父商品 BOM',1700,'active');
		INSERT INTO %[1]s.production_bom_versions(
			id,bom_id,version_no,status,yield_rate,output_qty,output_unit,process_route_id,published_at
		) VALUES (1701,1701,'V001','published',1,1,'kg',1701,now());
		INSERT INTO %[1]s.production_bom_version_items(
			version_id,material_id,component_type,consume_unit,ratio_pct
		) VALUES (1701,1701,'material','ratio_pct',100);
		INSERT INTO %[1]s.product_production_bom_bindings(product_id,bom_id,bom_version_id,bound_by)
		VALUES (1700,1701,1701,'test');
	`, schema))

	blocker, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatalf("begin order lock blocker: %v", err)
	}
	defer func() { _ = blocker.Rollback(ctx) }()
	if _, err := blocker.Exec(ctx, fmt.Sprintf(
		`SELECT id FROM %s.orders WHERE order_no='SO-PR553-CONCURRENT' FOR UPDATE`,
		schema,
	)); err != nil {
		t.Fatalf("lock source order: %v", err)
	}

	app := newProductionFlowTestEcho(pool, schema)
	type createResult struct {
		code int
		body string
	}
	results := make(chan createResult, 2)
	start := make(chan struct{})
	for range 2 {
		go func() {
			<-start
			req := httptest.NewRequest(
				http.MethodPost,
				"/api/production-plans",
				strings.NewReader(`{"from":"2026-07-25","to":"2026-07-25","source_type":"erp_order","selected":["1701-454"]}`),
			)
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			app.ServeHTTP(rec, req)
			results <- createResult{code: rec.Code, body: rec.Body.String()}
		}()
	}
	close(start)

	waitForProductionPlanOrderLockWaiters(t, ctx, pool, schema, 2)
	if err := blocker.Commit(ctx); err != nil {
		t.Fatalf("release source order lock: %v", err)
	}

	got := []createResult{<-results, <-results}
	successes := 0
	rejections := 0
	for _, result := range got {
		switch result.code {
		case http.StatusOK:
			successes++
		case http.StatusBadRequest:
			if !strings.Contains(result.body, "selected production items required") {
				t.Fatalf("second create rejected for unexpected reason: %s", result.body)
			}
			rejections++
		default:
			t.Fatalf("concurrent create status=%d body=%s", result.code, result.body)
		}
	}
	if successes != 1 || rejections != 1 {
		t.Fatalf("concurrent create outcomes=%+v, want one success and one no-demand rejection", got)
	}
	assertProductionFlowCount(t, pool, schema, "production_plans", "1=1", 1)
	assertProductionFlowCount(t, pool, schema, "production_plan_items", "1=1", 1)
}

func waitForProductionPlanOrderLockWaiters(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	schema string,
	want int,
) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var got int
		if err := pool.QueryRow(ctx, `
			SELECT COUNT(*)::int
			FROM pg_stat_activity
			WHERE datname=current_database()
			  AND wait_event_type='Lock'
			  AND query LIKE $1
		`, "%"+schema+".orders WHERE order_no = ANY($1) FOR UPDATE%").Scan(&got); err != nil {
			t.Fatalf("query production-plan lock waiters: %v", err)
		}
		if got >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d concurrent production-plan order locks", want)
}
