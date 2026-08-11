package production

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	productionapp "orderapp/internal/application/production"
	stockapp "orderapp/internal/application/stock"
	postgresproduction "orderapp/internal/infrastructure/postgres/production"
	postgresstock "orderapp/internal/infrastructure/postgres/stock"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func TestProductWorkOrderCompleteWithoutStockDocumentCreatesOneAtomicReceipt(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	app, workOrderID, runningItemID := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 15_000)

	complete := serveMultilevelProductionJSON(t, app, http.MethodPost,
		fmt.Sprintf("/api/produce/work-orders/%d/complete", workOrderID),
		map[string]any{
			"finished_units": 100, "consumed_input_g": 22_700,
			"warehouse": "finished_goods", "note": "无预建单据完工",
		})
	if complete.Code != http.StatusOK {
		t.Fatalf("complete product work order status=%d body=%s", complete.Code, complete.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "stock_entries", fmt.Sprintf(
		"work_order_id=%d AND running_item_id=%d AND entry_type='finished_receipt' AND status='submitted'", workOrderID, runningItemID,
	), 1)
	assertProductionFlowCount(t, pool, schema, "audit_logs", fmt.Sprintf(
		"entity_type='stock_entry' AND action='submit' AND entity_id IN (SELECT id FROM %s.stock_entries WHERE work_order_id=%d)", schema, workOrderID,
	), 1)
	assertProductionFlowCount(t, pool, schema, "audit_logs", fmt.Sprintf(
		"entity_type='work_order' AND entity_id=%d AND action='complete'", workOrderID,
	), 1)
	assertProductionFlowCount(t, pool, schema, "audit_logs", fmt.Sprintf(
		"entity_type='produce_running' AND entity_id=%d AND action='finish'", runningItemID,
	), 1)
}

func TestProductWorkOrderCompleteWithoutStockDocumentRollsBackReceiptAndFinalAuditFailures(t *testing.T) {
	tests := []struct {
		name       string
		triggerSQL func(schema string) string
		wantError  string
	}{
		{
			name: "finished receipt insert failure",
			triggerSQL: func(schema string) string {
				return fmt.Sprintf(`
				CREATE FUNCTION %[1]s.reject_finished_receipt() RETURNS trigger AS $body$
				BEGIN
					IF NEW.entry_type='finished_receipt' THEN
						RAISE EXCEPTION 'forced finished receipt failure';
					END IF;
					RETURN NEW;
				END
				$body$ LANGUAGE plpgsql;
				CREATE TRIGGER reject_finished_receipt BEFORE INSERT ON %[1]s.stock_entries
				FOR EACH ROW EXECUTE FUNCTION %[1]s.reject_finished_receipt();
			`, schema)
			},
			wantError: "forced finished receipt failure",
		},
		{
			name: "final running audit failure",
			triggerSQL: func(schema string) string {
				return fmt.Sprintf(`
				CREATE FUNCTION %[1]s.reject_final_running_finish_audit() RETURNS trigger AS $body$
				BEGIN
					IF NEW.entity_type='produce_running' AND NEW.action='finish' THEN
						RAISE EXCEPTION 'forced final running finish audit failure';
					END IF;
					RETURN NEW;
				END
				$body$ LANGUAGE plpgsql;
				CREATE TRIGGER reject_final_running_finish_audit BEFORE INSERT ON %[1]s.audit_logs
				FOR EACH ROW EXECUTE FUNCTION %[1]s.reject_final_running_finish_audit();
			`, schema)
			},
			wantError: "forced final running finish audit failure",
		},
	}
	for index, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, schema := newProductionFlowTestDB(t)
			ctx := context.Background()
			app, workOrderID, runningItemID := startProductCompletionAtomicityFixture(t, ctx, pool, schema, int64(16_000+index*100))
			before := loadProductCompletionAtomicityState(t, ctx, pool, schema, workOrderID, runningItemID)
			mustExecProductionFlowTestSQL(t, ctx, pool, tt.triggerSQL(schema))

			complete := serveMultilevelProductionJSON(t, app, http.MethodPost,
				fmt.Sprintf("/api/produce/work-orders/%d/complete", workOrderID),
				map[string]any{
					"finished_units": 100, "consumed_input_g": 22_700,
					"warehouse": "finished_goods", "note": "强制失败回滚",
				})
			if complete.Code != http.StatusBadRequest || !strings.Contains(complete.Body.String(), tt.wantError) {
				t.Fatalf("completion failure status=%d body=%s, want 400 containing %q", complete.Code, complete.Body.String(), tt.wantError)
			}
			after := loadProductCompletionAtomicityState(t, ctx, pool, schema, workOrderID, runningItemID)
			if after != before {
				t.Fatalf("completion failure left partial state\nbefore=%+v\nafter=%+v", before, after)
			}
		})
	}
}

func TestProduceRunningPartialFinishAuditFailureRollsBackAllChanges(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	app, workOrderID, runningItemID := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 17_000)
	before := loadProductCompletionAtomicityState(t, ctx, pool, schema, workOrderID, runningItemID)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		CREATE FUNCTION %[1]s.reject_partial_finish_audit() RETURNS trigger AS $body$
		BEGIN
			IF NEW.entity_type='produce_running' AND NEW.action='partial_finish' THEN
				RAISE EXCEPTION 'forced partial finish audit failure';
			END IF;
			RETURN NEW;
		END
		$body$ LANGUAGE plpgsql;
		CREATE TRIGGER reject_partial_finish_audit BEFORE INSERT ON %[1]s.audit_logs
		FOR EACH ROW EXECUTE FUNCTION %[1]s.reject_partial_finish_audit();
	`, schema))

	finish := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/produce/running/finish", map[string]any{
		"id": runningItemID, "finished_units": 50, "consumed_input_g": 11_350,
		"warehouse": "finished_goods", "partial": true,
	})
	if finish.Code != http.StatusBadRequest || !strings.Contains(finish.Body.String(), "forced partial finish audit failure") {
		t.Fatalf("partial finish audit failure status=%d body=%s", finish.Code, finish.Body.String())
	}
	after := loadProductCompletionAtomicityState(t, ctx, pool, schema, workOrderID, runningItemID)
	if after != before {
		t.Fatalf("partial finish audit failure left partial state\nbefore=%+v\nafter=%+v", before, after)
	}
	assertProductionFlowCount(t, pool, schema, "audit_logs", fmt.Sprintf(
		"entity_type='produce_running' AND entity_id=%d AND action='partial_finish'", runningItemID,
	), 0)
}

func TestActiveTypedWorkOrderCancelPersistsNoteInBothAtomicAudits(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	app, workOrderID, runningItemID := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 18_000)
	note := "客户调整，停止本批生产"
	cancel := serveMultilevelProductionJSON(t, app, http.MethodPost,
		fmt.Sprintf("/api/produce/work-orders/%d/cancel", workOrderID), map[string]any{"note": note})
	if cancel.Code != http.StatusOK {
		t.Fatalf("cancel active typed work order status=%d body=%s", cancel.Code, cancel.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "audit_logs", fmt.Sprintf(
		"action='cancel' AND meta->>'note'='%s' AND ((entity_type='work_order' AND entity_id=%d) OR (entity_type='produce_running' AND entity_id=%d))",
		note, workOrderID, runningItemID,
	), 2)
}

func TestActiveTypedWorkOrderCancelFinalRunningAuditFailureRollsBackReservationsAndPriorAudit(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	app, workOrderID, runningItemID := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 19_000)
	before := loadTypedCancelAtomicityState(t, ctx, pool, schema, workOrderID, runningItemID)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		CREATE FUNCTION %[1]s.reject_final_running_cancel_audit() RETURNS trigger AS $body$
		BEGIN
			IF NEW.entity_type='produce_running' AND NEW.action='cancel' THEN
				RAISE EXCEPTION 'forced final running cancel audit failure';
			END IF;
			RETURN NEW;
		END
		$body$ LANGUAGE plpgsql;
		CREATE TRIGGER reject_final_running_cancel_audit BEFORE INSERT ON %[1]s.audit_logs
		FOR EACH ROW EXECUTE FUNCTION %[1]s.reject_final_running_cancel_audit();
	`, schema))
	cancel := serveMultilevelProductionJSON(t, app, http.MethodPost,
		fmt.Sprintf("/api/produce/work-orders/%d/cancel", workOrderID), map[string]any{"note": "强制最终审计失败"})
	if cancel.Code != http.StatusBadRequest || !strings.Contains(cancel.Body.String(), "forced final running cancel audit failure") {
		t.Fatalf("cancel final audit failure status=%d body=%s", cancel.Code, cancel.Body.String())
	}
	after := loadTypedCancelAtomicityState(t, ctx, pool, schema, workOrderID, runningItemID)
	if after != before {
		t.Fatalf("cancel final audit failure left partial state\nbefore=%+v\nafter=%+v", before, after)
	}
}

type typedCancelAtomicityState struct {
	WorkOrderStatus, RunningStatus   string
	Reservations, ReservationBatches string
	CancelAuditCount                 int64
}

func loadTypedCancelAtomicityState(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string, workOrderID, runningItemID int64) typedCancelAtomicityState {
	t.Helper()
	var state typedCancelAtomicityState
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT wo.status,run.status,
		       COALESCE((SELECT jsonb_agg(jsonb_build_object(
		         'id',id,'status',status,'reserved_g',reserved_g,'reserved_units',reserved_units,
		         'consumed_g',consumed_g,'consumed_units',consumed_units,'returned_g',returned_g,'returned_units',returned_units
		       ) ORDER BY id)::text FROM %[1]s.work_order_material_reservations WHERE work_order_id=$1),'[]'),
		       COALESCE((SELECT jsonb_agg(jsonb_build_object(
		         'id',id,'status',status,'reserved_g',reserved_g,'reserved_units',reserved_units,
		         'consumed_g',consumed_g,'consumed_units',consumed_units,'returned_g',returned_g,'returned_units',returned_units
		       ) ORDER BY id)::text FROM %[1]s.work_order_material_reservation_batches WHERE work_order_id=$1),'[]'),
		       (SELECT COUNT(*)::bigint FROM %[1]s.audit_logs WHERE action='cancel' AND
		         ((entity_type='work_order' AND entity_id=$1) OR (entity_type='produce_running' AND entity_id=$2)))
		FROM %[1]s.work_orders wo JOIN %[1]s.produce_running_items run ON run.id=wo.running_item_id
		WHERE wo.id=$1 AND run.id=$2
	`, schema), workOrderID, runningItemID).Scan(
		&state.WorkOrderStatus, &state.RunningStatus, &state.Reservations, &state.ReservationBatches, &state.CancelAuditCount,
	); err != nil {
		t.Fatalf("load typed cancel atomicity state: %v", err)
	}
	return state
}

type productCompletionAtomicityState struct {
	WorkOrderStatus, RunningStatus                 string
	FinishedInventoryG, FinishedInventoryUnits     int64
	ProducedBatchCount, FinishedReceiptCount       int64
	MaterialLocationG, MaterialLocationUnits       int64
	ReservationConsumedG, ReservationConsumedUnits int64
	ReservationReturnedG, ReservationReturnedUnits int64
	CompletionAuditCount                           int64
}

func loadProductCompletionAtomicityState(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string, workOrderID, runningItemID int64) productCompletionAtomicityState {
	t.Helper()
	var state productCompletionAtomicityState
	query := fmt.Sprintf(`
		SELECT wo.status,run.status,
		       COALESCE((SELECT SUM(onhand_units*spec_g+onhand_loose_g)::bigint FROM %[1]s.finished_inventory WHERE product_id=1),0),
		       COALESCE((SELECT SUM(onhand_units)::bigint FROM %[1]s.finished_inventory WHERE product_id=1),0),
		       (SELECT COUNT(*)::bigint FROM %[1]s.stock_batches WHERE source_doc_type='production_run' AND source_doc_id=$2),
		       (SELECT COUNT(*)::bigint FROM %[1]s.stock_entries WHERE work_order_id=$1 AND entry_type='finished_receipt'),
		       COALESCE((SELECT SUM(qty_g)::bigint FROM %[1]s.material_batch_locations WHERE warehouse='wip'),0),
		       COALESCE((SELECT SUM(qty_units)::bigint FROM %[1]s.material_batch_locations WHERE warehouse='wip'),0),
		       COALESCE((SELECT SUM(consumed_g)::bigint FROM %[1]s.work_order_material_reservations WHERE work_order_id=$1),0),
		       COALESCE((SELECT SUM(consumed_units)::bigint FROM %[1]s.work_order_material_reservations WHERE work_order_id=$1),0),
		       COALESCE((SELECT SUM(returned_g)::bigint FROM %[1]s.work_order_material_reservations WHERE work_order_id=$1),0),
		       COALESCE((SELECT SUM(returned_units)::bigint FROM %[1]s.work_order_material_reservations WHERE work_order_id=$1),0),
		       (SELECT COUNT(*)::bigint FROM %[1]s.audit_logs
		        WHERE (entity_type='work_order' AND entity_id=$1 AND action='complete')
		           OR (entity_type='produce_running' AND entity_id=$2 AND action='finish')
		           OR (entity_type='stock_entry' AND action='submit' AND entity_id IN
		               (SELECT id FROM %[1]s.stock_entries WHERE work_order_id=$1)))
		FROM %[1]s.work_orders wo
		JOIN %[1]s.produce_running_items run ON run.id=wo.running_item_id
		WHERE wo.id=$1 AND run.id=$2
	`, schema)
	if err := pool.QueryRow(ctx, query, workOrderID, runningItemID).Scan(
		&state.WorkOrderStatus, &state.RunningStatus,
		&state.FinishedInventoryG, &state.FinishedInventoryUnits,
		&state.ProducedBatchCount, &state.FinishedReceiptCount,
		&state.MaterialLocationG, &state.MaterialLocationUnits,
		&state.ReservationConsumedG, &state.ReservationConsumedUnits,
		&state.ReservationReturnedG, &state.ReservationReturnedUnits,
		&state.CompletionAuditCount,
	); err != nil {
		t.Fatalf("load product completion atomicity state: %v", err)
	}
	return state
}

func startProductCompletionAtomicityFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string, batchID int64) (*echo.Echo, int64, int64) {
	t.Helper()
	app, _, workOrderID := createSubmittedFullStockTypedPlan(t, ctx, pool, schema, batchID)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %s.materials SET onhand_g=22700 WHERE id=10;
		UPDATE %s.materials SET onhand_units=100 WHERE id=20;
	`, schema, schema))
	start := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/start", workOrderID), nil)
	if start.Code != http.StatusOK {
		t.Fatalf("start product work order status=%d body=%s", start.Code, start.Body.String())
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %s.job_cards
		SET status='completed',started_at=COALESCE(started_at,now()),completed_at=now(),
		    actual_input_qty=22700,actual_output_qty=22700
		WHERE work_order_id=%d;
	`, schema, workOrderID))
	var runningItemID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT running_item_id FROM %s.work_orders WHERE id=$1`, schema), workOrderID).Scan(&runningItemID); err != nil {
		t.Fatalf("load running item id: %v", err)
	}
	return app, workOrderID, runningItemID
}

func TestTypedMaterialStockDocumentFinishUsesFrozenOutputWarehouseAndCompleteFlow(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	const runningItemID int64 = 9801
	const workOrderID int64 = 9802
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.materials(id,code,name,kind,unit,onhand_g,onhand_units,purchase_price,sale_price) VALUES
			(40,'MATERIAL-OUTPUT','在制熟豆','bean','kg',0,0,0,0),
			(41,'MATERIAL-INPUT','生产生豆','bean','kg',1250,0,50,0);
		INSERT INTO %s.produce_running_items(
			id,batch_id,product_id,product_name,spec_g,need_g,status,started_by,started_at,
			input_g,planned_units,planned_loose_g,material_snapshot,
			output_type,output_product_id,output_material_id,output_name,output_qty,output_unit,target_warehouse
		) VALUES(
			%d,'RUN-TYPED-STOCK',0,'在制熟豆',0,1000,'running','test',now(),
			1250,0,1000,
			'[{
			  "material_id":41,"material_name":"生产生豆","unit":"kg","source":"bom",
			  "component_type":"material","consume_unit":"kg","qty_per_unit":1.25,
			  "output_qty":1,"output_unit":"kg"
			}]'::jsonb,
			'material',0,40,'在制熟豆',1,'kg','wip'
		);
		INSERT INTO %s.work_orders(
			id,work_order_no,running_item_id,batch_id,product_id,product_name,spec_g,planned_g,planned_output_g,status,
			material_snapshot,output_type,output_product_id,output_material_id,output_name,output_qty,output_unit,target_warehouse
		) SELECT %d,'WO-TYPED-STOCK',id,batch_id,0,'在制熟豆',0,1250,1000,'running',material_snapshot,
		         'material',0,40,'在制熟豆',1,'kg','wip'
		  FROM %s.produce_running_items WHERE id=%d;
		INSERT INTO %s.job_cards(work_order_id,sequence_no,operation,status,started_at,completed_at)
		VALUES(%d,1,'烘焙','completed',now(),now());
	`, schema, schema, runningItemID, schema, workOrderID, schema, runningItemID, schema, workOrderID))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 9841, 41, "MB-TYPED-STOCK-INPUT", "生产生豆", 1_250)

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
	stockSvc := stockapp.NewService(postgresstock.NewRepository(pool, schema))
	registerWorkOrderAPI(e, productionSvc)
	registerStockEntryAPI(e, productionSvc, stockSvc)

	preview := serveMultilevelProductionJSON(t, e, http.MethodPost,
		fmt.Sprintf("/api/produce/work-orders/%d/stock-document-preview", workOrderID),
		map[string]any{"action": "finish"})
	if preview.Code != http.StatusOK {
		t.Fatalf("typed finish preview status=%d body=%s", preview.Code, preview.Body.String())
	}
	var previewPayload productionapp.StockDocumentPreview
	if err := json.Unmarshal(preview.Body.Bytes(), &previewPayload); err != nil {
		t.Fatal(err)
	}
	if len(previewPayload.Document.Items) != 1 {
		t.Fatalf("typed finish preview=%+v", previewPayload.Document)
	}
	previewItem := previewPayload.Document.Items[0]
	if previewItem.ItemType != "material" || previewItem.MaterialID != 40 || previewItem.ProductID != 0 ||
		previewItem.InventoryUnit != "kg" || previewItem.QtyG != 1000 || previewItem.QtyUnits != 0 ||
		previewItem.ToWarehouse != "wip" {
		t.Fatalf("typed finish preview item=%+v", previewItem)
	}

	tampered := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/stock-documents", map[string]any{
		"purpose": "manufacture", "work_order_id": workOrderID,
		"items": []map[string]any{{
			"material_id": 40, "item_type": "material", "item_name": "在制熟豆", "inventory_unit": "kg",
			"to_warehouse": "finished_goods", "qty_g": 1000,
		}},
	})
	if tampered.Code != http.StatusBadRequest || (!strings.Contains(tampered.Body.String(), "target warehouse") && !strings.Contains(tampered.Body.String(), "目标仓库")) {
		t.Fatalf("tampered material output stock document status=%d body=%s", tampered.Code, tampered.Body.String())
	}

	create := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/stock-documents", map[string]any{
		"purpose": "manufacture", "work_order_id": workOrderID, "return_source": "work_order",
		"items": []map[string]any{{
			"material_id": 40, "item_type": "material", "item_name": "在制熟豆", "inventory_unit": "kg",
			"to_warehouse": "wip", "qty_g": 1000,
		}},
	})
	if create.Code != http.StatusOK {
		t.Fatalf("create typed material stock document status=%d body=%s", create.Code, create.Body.String())
	}
	var draft stockapp.StockDocumentDetail
	if err := json.Unmarshal(create.Body.Bytes(), &draft); err != nil {
		t.Fatal(err)
	}
	if draft.ID <= 0 || draft.Status != "draft" {
		t.Fatalf("typed material stock draft=%+v", draft)
	}

	submit := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/stock-documents/%d/submit", draft.ID), nil)
	if submit.Code != http.StatusOK {
		t.Fatalf("submit typed material stock document status=%d body=%s", submit.Code, submit.Body.String())
	}
	var submitted stockapp.StockDocumentDetail
	if err := json.Unmarshal(submit.Body.Bytes(), &submitted); err != nil {
		t.Fatal(err)
	}
	if submitted.ID != draft.ID || submitted.Status != "submitted" || len(submitted.Items) != 1 ||
		submitted.Items[0].ItemType != "material" || submitted.Items[0].MaterialID != 40 ||
		submitted.Items[0].ToWarehouse != "wip" || submitted.Items[0].QtyG != 1000 || submitted.Items[0].BatchCode == "" {
		t.Fatalf("submitted typed material stock document=%+v", submitted)
	}

	assertProductionFlowCount(t, pool, schema, "stock_entries", fmt.Sprintf("id=%d AND work_order_id=%d AND purpose='manufacture' AND status='submitted'", draft.ID, workOrderID), 1)
	assertProductionFlowCount(t, pool, schema, "stock_entries", fmt.Sprintf("work_order_id=%d", workOrderID), 1)
	assertProductionFlowCount(t, pool, schema, "work_orders", fmt.Sprintf("id=%d AND status='completed'", workOrderID), 1)
	assertProductionFlowCount(t, pool, schema, "material_batches", "material_id=40 AND qty_g=1000 AND remaining_g=1000", 1)
	assertProductionFlowCount(t, pool, schema, "material_batch_locations", "material_id=40 AND warehouse='wip' AND qty_g=1000", 1)
	assertProductionFlowCount(t, pool, schema, "stock_ledger_entries", fmt.Sprintf("source_doc_type='production_run' AND source_doc_id=%d AND item_type='material' AND item_id=40 AND warehouse='wip' AND qty_change_g=1000", runningItemID), 1)
	assertProductionFlowCount(t, pool, schema, "audit_logs", fmt.Sprintf("entity_type='stock_entry' AND entity_id=%d AND action='submit'", draft.ID), 1)
	assertProductionFlowCount(t, pool, schema, "audit_logs", fmt.Sprintf("entity_type='work_order' AND entity_id=%d AND action='complete'", workOrderID), 1)
}

func TestTypedProductStockDocumentFinishUnlocksDownstreamThroughCompleteFlow(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	seedTypedProductComponentFlow(t, ctx, pool, schema)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 9930, 30, "MB-STOCKOPS-PRODUCT-GREEN", "生产生豆", 20_000)

	app := newProductionFlowTestEcho(pool, schema)
	productionSvc := productionapp.NewService(postgresproduction.NewRepository(pool, schema))
	stockSvc := stockapp.NewService(postgresstock.NewRepository(pool, schema))
	registerStockEntryAPI(app, productionSvc, stockSvc)
	createPlan := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans", map[string]any{
		"from": "2026-08-01", "to": "2026-08-31",
		"selected": []string{"1-227"}, "input_by_key": map[string]int64{"1-227": 22_700},
	})
	if createPlan.Code != http.StatusOK {
		t.Fatalf("create typed product stock-operations plan status=%d body=%s", createPlan.Code, createPlan.Body.String())
	}
	var planID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_plans ORDER BY id DESC LIMIT 1`, schema)).Scan(&planID); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.production_plan_operation_splits(
			production_plan_id,production_plan_item_id,operation_seq,operation,
			batch_size_qty,batch_size_unit,standard_minutes,planned_batch_count,
			planned_qty,planned_qty_g,planned_minutes
		)
		SELECT production_plan_id,id,1,
		       CASE WHEN output_product_id=2 THEN '烘焙' ELSE '包装' END,
		       planned_output_g,'g',15,1,planned_output_g,planned_output_g,15
		FROM %s.production_plan_items WHERE production_plan_id=%d;
	`, schema, schema, planID))
	submitPlan := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", planID), nil)
	if submitPlan.Code != http.StatusOK {
		t.Fatalf("submit typed product stock-operations plan status=%d body=%s", submitPlan.Code, submitPlan.Body.String())
	}
	var rootWorkOrderID, upstreamWorkOrderID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id FROM %s.work_orders WHERE production_plan_id=$1 AND output_product_id=1
	`, schema), planID).Scan(&rootWorkOrderID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id FROM %s.work_orders WHERE production_plan_id=$1 AND output_product_id=2
	`, schema), planID).Scan(&upstreamWorkOrderID); err != nil {
		t.Fatal(err)
	}
	start := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/start", upstreamWorkOrderID), nil)
	if start.Code != http.StatusOK {
		t.Fatalf("start typed product stock-operations upstream status=%d body=%s", start.Code, start.Body.String())
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %s.job_cards
		SET status='completed',started_at=COALESCE(started_at,now()),completed_at=now(),actual_input_qty=12700,actual_output_qty=12700
		WHERE work_order_id=%d;
	`, schema, upstreamWorkOrderID))
	preview := serveMultilevelProductionJSON(t, app, http.MethodPost,
		fmt.Sprintf("/api/produce/work-orders/%d/stock-document-preview", upstreamWorkOrderID),
		map[string]any{"action": "finish"})
	if preview.Code != http.StatusOK {
		t.Fatalf("preview typed product stock document status=%d body=%s", preview.Code, preview.Body.String())
	}
	var previewPayload productionapp.StockDocumentPreview
	if err := json.Unmarshal(preview.Body.Bytes(), &previewPayload); err != nil {
		t.Fatal(err)
	}
	if len(previewPayload.Document.Items) != 1 {
		t.Fatalf("typed product preview=%+v", previewPayload.Document)
	}
	item := previewPayload.Document.Items[0]
	if item.ItemType != "finished_product" || item.ProductID != 2 || item.SpecG != 1 || item.ToWarehouse != "finished_goods" || item.QtyUnits != 12_700 {
		t.Fatalf("typed product preview item=%+v", item)
	}
	createDraft := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/stock-documents", map[string]any{
		"purpose": "manufacture", "work_order_id": upstreamWorkOrderID, "note": "StockOperations typed product completion",
		"items": []map[string]any{{
			"product_id": item.ProductID, "item_type": item.ItemType, "item_name": item.ItemName,
			"spec_g": item.SpecG, "to_warehouse": item.ToWarehouse,
			"qty_g": item.QtyG, "qty_units": item.QtyUnits,
		}},
	})
	if createDraft.Code != http.StatusOK {
		t.Fatalf("create typed product stock document status=%d body=%s", createDraft.Code, createDraft.Body.String())
	}
	var draft stockapp.StockDocumentDetail
	if err := json.Unmarshal(createDraft.Body.Bytes(), &draft); err != nil {
		t.Fatal(err)
	}
	submitDraft := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/stock-documents/%d/submit", draft.ID), nil)
	if submitDraft.Code != http.StatusOK {
		t.Fatalf("submit typed product stock document status=%d body=%s", submitDraft.Code, submitDraft.Body.String())
	}
	var submitted stockapp.StockDocumentDetail
	if err := json.Unmarshal(submitDraft.Body.Bytes(), &submitted); err != nil {
		t.Fatal(err)
	}
	if submitted.ID != draft.ID || submitted.Status != "submitted" || len(submitted.Items) != 1 || submitted.Items[0].BatchCode == "" {
		t.Fatalf("submitted typed product stock document=%+v", submitted)
	}
	assertProductionFlowCount(t, pool, schema, "stock_entries", fmt.Sprintf("work_order_id=%d", upstreamWorkOrderID), 1)
	assertProductionFlowCount(t, pool, schema, "work_orders", fmt.Sprintf("id=%d AND status='completed'", upstreamWorkOrderID), 1)
	assertProductionFlowCount(t, pool, schema, "production_batch_costs", fmt.Sprintf("running_item_id=(SELECT running_item_id FROM %s.work_orders WHERE id=%d)", schema, upstreamWorkOrderID), 1)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservations", fmt.Sprintf(
		"work_order_id=%d AND component_type='product' AND component_id=2 AND required_g=22700 AND reserved_g=22700 AND status='reserved'", rootWorkOrderID,
	), 1)
	downstreamStart := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/start", rootWorkOrderID), nil)
	if downstreamStart.Code != http.StatusOK {
		t.Fatalf("start downstream after StockOperations product completion status=%d body=%s", downstreamStart.Code, downstreamStart.Body.String())
	}
}
