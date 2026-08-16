package costing

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPublishRunPersistsCutoverProductPriceByBOMSpecPostgres(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	schema := fmt.Sprintf("pr600_publish_spec_price_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %[1]s.products(
			id BIGINT PRIMARY KEY, default_price NUMERIC NOT NULL DEFAULT 0,
			retail_price_100g NUMERIC NOT NULL DEFAULT 0,retail_price_200g NUMERIC NOT NULL DEFAULT 0,
			retail_price_227g NUMERIC NOT NULL DEFAULT 0,retail_price_250g NUMERIC NOT NULL DEFAULT 0
		);
		CREATE TABLE %[1]s.product_price_tiers(
			id BIGSERIAL PRIMARY KEY,product_id BIGINT NOT NULL,bom_spec_id BIGINT NOT NULL DEFAULT 0,bom_variant_id BIGINT NOT NULL DEFAULT 0,
			spec_g BIGINT NOT NULL DEFAULT 454,min_qty_units NUMERIC NOT NULL DEFAULT 0,max_qty_units NUMERIC,
			price_per_unit NUMERIC NOT NULL DEFAULT 0,min_qty_lb NUMERIC,max_qty_lb NUMERIC,price_per_lb NUMERIC,
			active BOOLEAN NOT NULL DEFAULT true,product_kind TEXT NOT NULL DEFAULT '',price_basis TEXT NOT NULL DEFAULT '',
			sales_unit TEXT NOT NULL DEFAULT '',unit_bag_count INTEGER NOT NULL DEFAULT 0,price_source_json JSONB NOT NULL DEFAULT '{}'::jsonb
		);
		CREATE TABLE %[1]s.cost_calculation_runs(id BIGINT PRIMARY KEY,status TEXT NOT NULL,published_at TIMESTAMPTZ);
		CREATE TABLE %[1]s.cost_calculation_items(id BIGSERIAL PRIMARY KEY,run_id BIGINT NOT NULL,result_json JSONB NOT NULL);
		CREATE TABLE %[1]s.audit_logs(id BIGSERIAL PRIMARY KEY,actor TEXT,entity_type TEXT,entity_id BIGINT,action TEXT,field TEXT,old_value TEXT,new_value TEXT,meta JSONB);
		CREATE FUNCTION %[1]s.require_bom_spec_price() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			IF NEW.product_id=1 AND NEW.bom_spec_id<=0 THEN
				RAISE EXCEPTION 'bom_spec_id_required';
			END IF;
			RETURN NEW;
		END $$;
		CREATE TRIGGER require_bom_spec_price BEFORE INSERT ON %[1]s.product_price_tiers
			FOR EACH ROW EXECUTE FUNCTION %[1]s.require_bom_spec_price();
		INSERT INTO %[1]s.products(id) VALUES(1);
		INSERT INTO %[1]s.cost_calculation_runs(id,status) VALUES(1,'draft');
		INSERT INTO %[1]s.cost_calculation_items(run_id,result_json) VALUES(
			1,'{"product_id":1,"bom_spec_id":701,"bom_variant_id":702,"is_default_sku":true,"product_kind":"roasted_bean","commercial_wholesale_tiers":[{"spec_g":227,"min_qty":1,"price_per_unit":42}]}'::jsonb
		);
	`, schema)); err != nil {
		t.Fatal(err)
	}

	if err := NewRepository(pool, schema).PublishRun(ctx, "pr600-price", 1); err != nil {
		t.Fatalf("publish cutover BOM-spec price: %v", err)
	}
	var specID, variantID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT bom_spec_id,bom_variant_id FROM %s.product_price_tiers WHERE product_id=1`, schema)).Scan(&specID, &variantID); err != nil {
		t.Fatal(err)
	}
	if specID != 701 || variantID != 702 {
		t.Fatalf("published tier identity=(%d,%d), want (701,702)", specID, variantID)
	}
}
