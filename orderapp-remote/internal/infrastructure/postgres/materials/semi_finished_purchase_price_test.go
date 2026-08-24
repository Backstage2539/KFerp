package materials

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestNormalizeMaterialInputRejectsSemiFinishedPurchasePrice(t *testing.T) {
	_, err := normalizeMaterialInput(materialInput{
		Code: "WIP-ROASTED", Name: "烘焙熟豆", Kind: "bean", Unit: "kg",
		IsSemiFinished: true, PurchasePrice: 288,
	})
	if err == nil || !strings.Contains(err.Error(), "半成品只能通过生产入库") {
		t.Fatalf("normalizeMaterialInput() error = %v, want manufacture-only purchase-price rejection", err)
	}
}

func TestEnsureSchemaEnforcesSemiFinishedPurchasePriceInvariantPostgres(t *testing.T) {
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
	schema := fmt.Sprintf("pr600_semi_price_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })

	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %[1]s.audit_logs(
			id BIGSERIAL PRIMARY KEY, actor TEXT NOT NULL DEFAULT '', entity_type TEXT NOT NULL DEFAULT '',
			entity_id BIGINT, action TEXT NOT NULL DEFAULT '', field TEXT, old_value TEXT, new_value TEXT, meta JSONB
		);
		CREATE TABLE %[1]s.materials(
			id BIGSERIAL PRIMARY KEY, code TEXT NOT NULL UNIQUE, name TEXT NOT NULL,
			kind TEXT NOT NULL DEFAULT 'other', is_semi_finished BOOLEAN NOT NULL DEFAULT false,
			unit TEXT NOT NULL DEFAULT 'kg', cost_unit TEXT NOT NULL DEFAULT 'kg', batch_no TEXT NOT NULL DEFAULT '',
			purchase_price NUMERIC(12,2) NOT NULL DEFAULT 0, sale_price NUMERIC(12,2) NOT NULL DEFAULT 0,
			onhand_g BIGINT NOT NULL DEFAULT 0, onhand_units BIGINT NOT NULL DEFAULT 0,
			min_level_g BIGINT NOT NULL DEFAULT 0, min_level_units BIGINT NOT NULL DEFAULT 0,
			deprecated_at TIMESTAMPTZ NULL, updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		INSERT INTO %[1]s.materials(id,code,name,is_semi_finished,purchase_price)
		VALUES(1,'LEGACY-WIP','历史半成品',true,288);
		CREATE TABLE %[1]s.purchase_orders(
			id BIGSERIAL PRIMARY KEY, material_id BIGINT NOT NULL, status TEXT NOT NULL DEFAULT 'ordered'
		);
		CREATE TABLE %[1]s.purchase_receipts(
			id BIGSERIAL PRIMARY KEY, material_id BIGINT NOT NULL, stock_receipt_id BIGINT NOT NULL DEFAULT 0
		);
		CREATE TABLE %[1]s.stock_entries(
			id BIGINT PRIMARY KEY, purpose TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'draft'
		);
		CREATE TABLE %[1]s.stock_entry_items(
			id BIGSERIAL PRIMARY KEY, stock_entry_id BIGINT NOT NULL, material_id BIGINT NOT NULL DEFAULT 0,
			item_type TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE %[1]s.material_batches(
			id BIGINT PRIMARY KEY, material_id BIGINT NOT NULL, remaining_g BIGINT NOT NULL DEFAULT 0
		);
		INSERT INTO %[1]s.purchase_orders(material_id,status) VALUES(1,'received');
		INSERT INTO %[1]s.purchase_receipts(material_id,stock_receipt_id) VALUES(1,71);
		INSERT INTO %[1]s.stock_entries(id,purpose,status) VALUES(71,'material_receipt','submitted');
		INSERT INTO %[1]s.stock_entry_items(stock_entry_id,material_id,item_type) VALUES(71,1,'material');
		INSERT INTO %[1]s.material_batches(id,material_id,remaining_g) VALUES(81,1,777);
	`, schema)); err != nil {
		t.Fatal(err)
	}

	var constraintOID uint32
	for pass := 1; pass <= 2; pass++ {
		if err := EnsureSchema(ctx, pool, schema); err != nil {
			t.Fatalf("EnsureSchema pass %d: %v", pass, err)
		}
		var oid uint32
		var validated bool
		if err := pool.QueryRow(ctx, fmt.Sprintf(`
			SELECT c.oid,c.convalidated
			FROM pg_constraint c
			WHERE c.conrelid='%s.materials'::regclass
			  AND c.conname='materials_semi_finished_purchase_price_zero'
		`, schema)).Scan(&oid, &validated); err != nil {
			t.Fatalf("load invariant constraint after pass %d: %v", pass, err)
		}
		if !validated {
			t.Fatalf("constraint after pass %d is not validated", pass)
		}
		if constraintOID != 0 && constraintOID != oid {
			t.Fatalf("constraint OID changed across idempotent runs: %d -> %d", constraintOID, oid)
		}
		constraintOID = oid
	}

	var price float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT purchase_price::float8 FROM %s.materials WHERE id=1`, schema)).Scan(&price); err != nil {
		t.Fatal(err)
	}
	if price != 0 {
		t.Fatalf("legacy semi-finished purchase price = %.2f, want 0", price)
	}
	var auditCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT count(*) FROM %s.audit_logs
		WHERE entity_type='material' AND entity_id=1 AND action='normalize'
		  AND field='purchase_price' AND old_value='288.00' AND new_value='0.00'
	`, schema)).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("legacy normalization audit count = %d, want 1", auditCount)
	}
	var historicalRemaining int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT remaining_g FROM %s.material_batches WHERE id=81`, schema)).Scan(&historicalRemaining); err != nil {
		t.Fatal(err)
	}
	if historicalRemaining != 777 {
		t.Fatalf("historical material batch remaining_g = %d, want unchanged 777", historicalRemaining)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.materials SET purchase_price=1 WHERE id=1`, schema)); err == nil {
		t.Fatal("database accepted a non-zero semi-finished purchase price")
	}
}

func TestSemiFinishedPurchasePriceMigrationFailsClosedOnUnfinishedInboundPostgres(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required")
	}

	tests := []struct {
		name    string
		blocker string
	}{
		{
			name: "unfinished purchase order",
			blocker: `INSERT INTO %[1]s.purchase_orders(material_id,status)
				VALUES(1,'ordered')`,
		},
		{
			name: "unfinished purchase receipt",
			blocker: `INSERT INTO %[1]s.purchase_receipts(material_id,stock_receipt_id)
				VALUES(1,0)`,
		},
		{
			name: "unfinished ordinary material receipt",
			blocker: `INSERT INTO %[1]s.stock_entries(id,purpose,status)
				VALUES(91,'material_receipt','draft');
				INSERT INTO %[1]s.stock_entry_items(stock_entry_id,material_id,item_type)
				VALUES(91,1,'material')`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			pool, err := pgxpool.New(ctx, dsn)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(pool.Close)
			schema := fmt.Sprintf("pr600_semi_preflight_%d_%d", os.Getpid(), time.Now().UnixNano())
			if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })

			if _, err := pool.Exec(ctx, fmt.Sprintf(`
				CREATE TABLE %[1]s.audit_logs(
					id BIGSERIAL PRIMARY KEY, actor TEXT NOT NULL DEFAULT '', entity_type TEXT NOT NULL DEFAULT '',
					entity_id BIGINT, action TEXT NOT NULL DEFAULT '', field TEXT, old_value TEXT, new_value TEXT, meta JSONB
				);
				CREATE TABLE %[1]s.materials(
					id BIGINT PRIMARY KEY, code TEXT NOT NULL UNIQUE, name TEXT NOT NULL,
					is_semi_finished BOOLEAN NOT NULL DEFAULT false,
					purchase_price NUMERIC(12,2) NOT NULL DEFAULT 0,
					updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
				);
				CREATE TABLE %[1]s.purchase_orders(
					id BIGSERIAL PRIMARY KEY, material_id BIGINT NOT NULL, status TEXT NOT NULL DEFAULT 'ordered'
				);
				CREATE TABLE %[1]s.purchase_receipts(
					id BIGSERIAL PRIMARY KEY, material_id BIGINT NOT NULL, stock_receipt_id BIGINT NOT NULL DEFAULT 0
				);
				CREATE TABLE %[1]s.stock_entries(
					id BIGINT PRIMARY KEY, purpose TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'draft'
				);
				CREATE TABLE %[1]s.stock_entry_items(
					id BIGSERIAL PRIMARY KEY, stock_entry_id BIGINT NOT NULL, material_id BIGINT NOT NULL DEFAULT 0,
					item_type TEXT NOT NULL DEFAULT ''
				);
				INSERT INTO %[1]s.materials(id,code,name,is_semi_finished,purchase_price)
				VALUES(1,'LEGACY-WIP','历史半成品',true,288);
				`+tc.blocker+`;
			`, schema)); err != nil {
				t.Fatal(err)
			}

			err = ensureSemiFinishedPurchasePriceConstraint(ctx, pool, schema)
			if err == nil || !strings.Contains(err.Error(), "未完成") {
				t.Fatalf("ensureSemiFinishedPurchasePriceConstraint error=%v, want unfinished-inbound blocker", err)
			}

			var price float64
			var auditCount, constraintCount int
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT purchase_price::float8 FROM %s.materials WHERE id=1`, schema)).Scan(&price); err != nil {
				t.Fatal(err)
			}
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.audit_logs`, schema)).Scan(&auditCount); err != nil {
				t.Fatal(err)
			}
			if err := pool.QueryRow(ctx, `
				SELECT count(*) FROM pg_constraint c
				JOIN pg_class t ON t.oid=c.conrelid
				JOIN pg_namespace n ON n.oid=t.relnamespace
				WHERE n.nspname=$1 AND t.relname='materials'
				  AND c.conname='materials_semi_finished_purchase_price_zero'
			`, schema).Scan(&constraintCount); err != nil {
				t.Fatal(err)
			}
			if price != 288 || auditCount != 0 || constraintCount != 0 {
				t.Fatalf("failed migration changed price/audit/constraint = %.2f/%d/%d, want 288/0/0", price, auditCount, constraintCount)
			}
		})
	}
}
