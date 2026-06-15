package production

import (
	"context"
	"fmt"
	productionapp "orderapp/internal/application/production"
	"testing"
)

func TestProductionTraceAnalyticsOmitsEmptyTraceRows(t *testing.T) {
	pool, schema := newProductionTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.work_orders (
	id BIGSERIAL PRIMARY KEY,
	work_order_no TEXT NOT NULL DEFAULT '',
	running_item_id BIGINT NOT NULL DEFAULT 0,
	batch_id TEXT NOT NULL DEFAULT '',
	product_name TEXT NOT NULL DEFAULT '',
	planned_g BIGINT NOT NULL DEFAULT 0,
	actual_cost NUMERIC(12,4) NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	completed_at TIMESTAMPTZ
);
CREATE TABLE %s.job_cards (
	id BIGSERIAL PRIMARY KEY,
	work_order_id BIGINT NOT NULL DEFAULT 0,
	sequence_no INT NOT NULL DEFAULT 1,
	operation TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'pending',
	planned_operation_cost NUMERIC(14,4) NOT NULL DEFAULT 0,
	actual_operation_cost NUMERIC(14,4) NOT NULL DEFAULT 0,
	actual_input_qty NUMERIC(14,4) NOT NULL DEFAULT 0,
	actual_output_qty NUMERIC(14,4) NOT NULL DEFAULT 0,
	actual_loss_qty NUMERIC(14,4) NOT NULL DEFAULT 0,
	actual_loss_rate NUMERIC(10,4) NOT NULL DEFAULT 0,
	loss_reason TEXT NOT NULL DEFAULT '',
	exception_reason TEXT NOT NULL DEFAULT '',
	completed_at TIMESTAMPTZ
);
CREATE TABLE %s.stock_entries (
	id BIGSERIAL PRIMARY KEY,
	entry_no TEXT NOT NULL DEFAULT '',
	entry_type TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'submitted',
	work_order_id BIGINT NOT NULL DEFAULT 0,
	job_card_id BIGINT NOT NULL DEFAULT 0,
	running_item_id BIGINT NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.stock_entry_items (
	id BIGSERIAL PRIMARY KEY,
	stock_entry_id BIGINT NOT NULL DEFAULT 0,
	material_id BIGINT NOT NULL DEFAULT 0,
	item_name TEXT NOT NULL DEFAULT '',
	batch_code TEXT NOT NULL DEFAULT '',
	qty_g BIGINT NOT NULL DEFAULT 0
);
CREATE TABLE %s.production_batch_costs (
	id BIGSERIAL PRIMARY KEY,
	running_item_id BIGINT NOT NULL DEFAULT 0,
	batch_id TEXT NOT NULL DEFAULT '',
	total_cost NUMERIC(12,4) NOT NULL DEFAULT 0,
	unit_cost_per_kg NUMERIC(12,4) NOT NULL DEFAULT 0
);
INSERT INTO %s.work_orders(id,work_order_no,running_item_id,batch_id,product_name,planned_g,created_at)
VALUES (88,'WO-EMPTY',0,'BATCH-EMPTY','空链路',1000,now()), (89,'WO-TRACE',99,'BATCH-TRACE','有链路',1000,now());
INSERT INTO %s.job_cards(id,work_order_id,sequence_no,operation,status) VALUES (12,88,1,'烘焙','released'), (13,89,1,'包装','completed');
INSERT INTO %s.stock_entries(id,entry_no,entry_type,work_order_id,job_card_id,running_item_id,created_at)
VALUES (9,'SE-9','finished_receipt',89,13,99,now());
INSERT INTO %s.stock_entry_items(stock_entry_id,material_id,item_name,batch_code,qty_g)
VALUES (9,0,'熟豆-红岩拼配','FP-9',1000);
`, schema, schema, schema, schema, schema, schema, schema, schema, schema))

	res, err := NewRepository(pool, schema).ProductionTraceAnalytics(ctx, productionapp.ProductionTraceAnalyticsQuery{Limit: 20})
	if err != nil {
		t.Fatalf("ProductionTraceAnalytics: %v", err)
	}
	if len(res.TraceLinks) != 1 {
		t.Fatalf("trace links = %+v, want one stock-entry-backed row", res.TraceLinks)
	}
	if res.TraceLinks[0].WorkOrderNo != "WO-TRACE" || res.TraceLinks[0].EntryNo != "SE-9" || res.TraceLinks[0].BatchCode != "FP-9" {
		t.Fatalf("trace link = %+v, want WO-TRACE / SE-9 / FP-9", res.TraceLinks[0])
	}
}
