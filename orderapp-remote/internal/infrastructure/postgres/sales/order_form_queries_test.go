package sales

import (
	"context"
	"fmt"
	"strings"
	"testing"

	postgrescore "orderapp/internal/infrastructure/postgres/core"
)

func TestOrderEditItemsQueryUsesPersistedPriceOverrideColumn(t *testing.T) {
	query := orderEditItemsQuery("tenant_test")

	if !strings.Contains(query, "oi.price_overridden") {
		t.Fatalf("order edit items query must read the persisted price_overridden column: %s", query)
	}
	if strings.Contains(query, "oi.price_override,") || strings.Contains(query, "COALESCE(oi.price_override,false)") {
		t.Fatalf("order edit items query still references the nonexistent price_override column: %s", query)
	}
}

func TestOrderEditItemsQueryExecutesAgainstCanonicalPostgresSchema(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()

	if err := postgrescore.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("core EnsureSchema: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS bom_spec_id BIGINT NOT NULL DEFAULT 0;
		ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS bom_variant_id BIGINT NOT NULL DEFAULT 0;
		ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS customer_product_alias_id BIGINT NOT NULL DEFAULT 0;
		ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS customer_product_display_name_snapshot TEXT NOT NULL DEFAULT '';
		ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS customer_item_code_snapshot TEXT NOT NULL DEFAULT '';
		ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS brand_name_snapshot TEXT NOT NULL DEFAULT '';
		ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS product_code_snapshot TEXT NOT NULL DEFAULT '';
		ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS product_name_snapshot TEXT NOT NULL DEFAULT '';
		INSERT INTO %[1]s.products(id,name) VALUES (9001,'测试商品');
		INSERT INTO %[1]s.orders(id,order_no) VALUES (9001,'TEST-ORDER');
		INSERT INTO %[1]s.order_items(order_id,line_no,product_id,item_name,price_overridden)
		VALUES (9001,1,9001,'测试商品',true);
	`, schema)); err != nil {
		t.Fatalf("prepare canonical order item: %v", err)
	}

	rows, err := pool.Query(ctx, orderEditItemsQuery(schema), int64(9001))
	if err != nil {
		t.Fatalf("query canonical order item: %v", err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatalf("canonical order item missing: %v", rows.Err())
	}
	values, err := rows.Values()
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 33 || values[21] != true {
		t.Fatalf("order item values=%#v, want price override at column 22", values)
	}
}

func TestFetchCurrentProductCatalogPublicationTypeIDsUsesActiveFeatureSelection(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %[1]s.business_groups (
			id BIGINT PRIMARY KEY,
			name TEXT NOT NULL,
			code TEXT NOT NULL,
			active BOOLEAN NOT NULL DEFAULT true
		);
		CREATE TABLE %[1]s.business_group_usages (
			id BIGSERIAL PRIMARY KEY,
			group_id BIGINT NOT NULL,
			usage_key TEXT NOT NULL,
			active BOOLEAN NOT NULL DEFAULT true,
			sort_order INTEGER NOT NULL DEFAULT 0
		);
		INSERT INTO %[1]s.business_groups(id,name,code,active) VALUES
			(1532,'咖啡豆','BEAN',true),
			(1533,'咖啡挂耳','drip_bag',true),
			(6,'商品默认分组','default_product_catalog',true);
		INSERT INTO %[1]s.business_group_usages(group_id,usage_key,active,sort_order) VALUES
			(1532,'product_catalog',true,1),
			(1533,'product_catalog',false,2),
			(6,'product_catalog',true,3);
	`, schema)); err != nil {
		t.Fatalf("prepare product catalog feature selection: %v", err)
	}
	repo := NewRepository(pool, schema)
	got, err := repo.fetchCurrentProductCatalogPublicationTypeIDs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != 8000000000001532 {
		t.Fatalf("current publication type ids = %v, want [8000000000001532]", got)
	}
}
