package production

import (
	"context"
	"fmt"
	productionapp "orderapp/internal/application/production"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	"testing"
)

func TestProductionLogProductsHideTemplateRemovedDerivedSKUs(t *testing.T) {
	rows := []postgresinfra.ProductOption{
		{ID: 1, Name: "当前规格", AutoDerivedSKU: true, DerivedSpecStatus: "active"},
		{ID: 2, Name: "历史规格", AutoDerivedSKU: true, DerivedSpecStatus: "template_removed"},
		{ID: 3, Name: "普通商品", AutoDerivedSKU: false, DerivedSpecStatus: "template_removed"},
	}

	out := make([]productionapp.ProductionLogProductOption, 0, len(rows))
	for _, product := range rows {
		if skipTemplateRemovedDerivedProductOption(product) {
			continue
		}
		out = append(out, productionapp.ProductionLogProductOption{ID: product.ID, Name: product.Name})
	}

	if len(out) != 2 {
		t.Fatalf("product options = %d, want 2", len(out))
	}
	for _, product := range out {
		if product.ID == 2 {
			t.Fatalf("template_removed derived SKU should be hidden from production log candidates: %+v", out)
		}
	}
}

func TestListProductionLogsIncludesFinishedBatchCode(t *testing.T) {
	pool, schema := newProductionTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.production_logs (
	id BIGSERIAL PRIMARY KEY,
	running_item_id BIGINT NOT NULL DEFAULT 0,
	batch_id TEXT NOT NULL DEFAULT '',
	product_id BIGINT NOT NULL DEFAULT 0,
	product_name TEXT NOT NULL DEFAULT '',
	spec_g BIGINT NOT NULL DEFAULT 0,
	order_nos TEXT NOT NULL DEFAULT '',
	planned_need_g BIGINT NOT NULL DEFAULT 0,
	input_g BIGINT NOT NULL DEFAULT 0,
	bom_yield_rate NUMERIC(10,4) NOT NULL DEFAULT 0,
	finished_units BIGINT NOT NULL DEFAULT 0,
	finished_loose_g BIGINT NOT NULL DEFAULT 0,
	finished_total_g BIGINT NOT NULL DEFAULT 0,
	actual_yield_rate NUMERIC(10,4) NOT NULL DEFAULT 0,
	started_by TEXT NOT NULL DEFAULT '',
	started_at TIMESTAMPTZ,
	finished_by TEXT NOT NULL DEFAULT '',
	finished_at TIMESTAMPTZ,
	inventory_units_before BIGINT NOT NULL DEFAULT 0,
	inventory_loose_g_before BIGINT NOT NULL DEFAULT 0,
	inventory_units_after BIGINT NOT NULL DEFAULT 0,
	inventory_loose_g_after BIGINT NOT NULL DEFAULT 0,
	material_summary JSONB NOT NULL DEFAULT '[]'::jsonb,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.products (id BIGINT PRIMARY KEY, name TEXT NOT NULL DEFAULT '');
CREATE TABLE %s.stock_batches (
	id BIGSERIAL PRIMARY KEY,
	batch_code TEXT NOT NULL DEFAULT '',
	item_type TEXT NOT NULL DEFAULT '',
	item_id BIGINT NOT NULL DEFAULT 0,
	item_name TEXT NOT NULL DEFAULT '',
	spec_g BIGINT NOT NULL DEFAULT 0,
	source_doc_type TEXT NOT NULL DEFAULT '',
	source_doc_id BIGINT NOT NULL DEFAULT 0,
	source_batch_id TEXT NOT NULL DEFAULT '',
	qty_g BIGINT NOT NULL DEFAULT 0,
	qty_units BIGINT NOT NULL DEFAULT 0,
	remaining_g BIGINT NOT NULL DEFAULT 0,
	remaining_units BIGINT NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO %s.products(id,name) VALUES (9,'红岩拼配');
INSERT INTO %s.production_logs(running_item_id,batch_id,product_id,product_name,spec_g,order_nos,finished_units,finished_total_g,finished_at,material_summary)
VALUES (77,'PB-77',9,'红岩拼配',454,'SO-77',2,908,now(),'[{"material_name":"生豆","batch_code":"MB-77","deduct_g":1000}]'::jsonb);
INSERT INTO %s.stock_batches(batch_code,item_type,item_id,item_name,spec_g,source_doc_type,source_doc_id,source_batch_id,qty_g,qty_units,remaining_g,remaining_units,created_at)
VALUES ('FP-77','finished_product',9,'红岩拼配',454,'production_run',77,'PB-77',908,2,908,2,now());
`, schema, schema, schema, schema, schema, schema))

	res, err := NewRepository(pool, schema).ListProductionLogs(ctx, productionapp.ProductionLogsQuery{RunningItemID: 77, Limit: 20})
	if err != nil {
		t.Fatalf("ListProductionLogs: %v", err)
	}
	if len(res.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(res.Rows))
	}
	if res.Rows[0].FinishedBatchCode != "FP-77" {
		t.Fatalf("finished batch code = %q, want FP-77", res.Rows[0].FinishedBatchCode)
	}
}
