package sales

import (
	"context"
	"fmt"
	salesapp "orderapp/internal/application/sales"
	salesdomain "orderapp/internal/domain/sales"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestDeliveryMethodDisplayNameHidesInternalCodes(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
	}{
		{in: "sf_small", want: "顺丰发货"},
		{in: "sf_large", want: "顺丰大件"},
		{in: "sf_express", want: "顺丰标快"},
		{in: "顺丰冷运", want: "顺丰冷运"},
	} {
		if got := deliveryMethodDisplayName(tc.in); got != tc.want {
			t.Fatalf("deliveryMethodDisplayName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestGenerateDeliveryNoteDocumentCleansFileWhenDocumentInsertFails(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	prepareSalesSchemaPrerequisites(t, ctx, pool, schema)
	prepareDeliveryNoteGenerationOrderColumns(t, ctx, pool, schema)
	assetDir := t.TempDir()
	repo := NewRepository(pool, schema, WithSalesOrderAssetDir(assetDir))
	repo.deliveryNoteRenderer = fakeDeliveryNoteRenderer{}
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	seedDeliveryNoteGenerationOrder(t, ctx, pool, schema)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.delivery_note_documents ADD CONSTRAINT delivery_note_document_test_reject_latest CHECK (is_latest = false)`, schema)); err != nil {
		t.Fatalf("add reject latest constraint: %v", err)
	}

	if _, err := repo.GenerateDeliveryNoteDocument(ctx, salesapp.GenerateDeliveryNoteDocumentCommand{Actor: "测试员", OrderID: 1}); err == nil {
		t.Fatalf("GenerateDeliveryNoteDocument should fail when document insert is rejected")
	}
	assertSalesPostgresAssetDirEmpty(t, assetDir)
}

type fakeDeliveryNoteRenderer struct{}

func (fakeDeliveryNoteRenderer) Render(snapshot salesdomain.DeliveryNoteSnapshot) ([]byte, error) {
	if err := snapshot.Validate(); err != nil {
		return nil, err
	}
	return []byte("%PDF-delivery-test"), nil
}

func prepareDeliveryNoteGenerationOrderColumns(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) {
	t.Helper()
	for _, stmt := range []string{
		fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS ship_status_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS ship_method TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS receiver_name TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS receiver_phone TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS receiver_address TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS source_warehouse TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS notes TEXT NOT NULL DEFAULT ''`, schema),
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("prepare delivery note order columns: %v", err)
		}
	}
}

func seedDeliveryNoteGenerationOrder(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) {
	t.Helper()
	var shipStatusID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.ship_statuses WHERE name='已发货' ORDER BY id LIMIT 1`, schema)).Scan(&shipStatusID); err != nil {
		t.Fatalf("query shipped status: %v", err)
	}
	stmts := []string{
		fmt.Sprintf(`INSERT INTO %s.company_profile(id, company_name, company_address) VALUES(1, '棵凡咖啡', '云南省普洱市孟连县')`, schema),
		fmt.Sprintf(`INSERT INTO %s.customers(id, name, company_name, company_address, company_phone, contact, phone, address) VALUES(1, '某某咖啡馆', '某某咖啡贸易公司', '上海市徐汇区', '021-12345678', '张三', '13800000000', '上海市徐汇区')`, schema),
		fmt.Sprintf(`INSERT INTO %s.products(id, name) VALUES(1, '橘皮乌龙')`, schema),
		fmt.Sprintf(`INSERT INTO %s.orders(id, order_no, order_date, customer_id, ship_status_id, ship_method, ship_tracking_no, receiver_name, receiver_phone, receiver_address, source_warehouse, notes, total_amount, shipping_amount, discount_amount, grand_total)
			VALUES(1, 'SO-20260513-DN01', '2026-05-13', 1, %d, '顺丰', 'SF123456789', '张三', '13800000000', '上海市徐汇区', 'finished_goods', '随货附出库单', 134, 0, 0, 134)`, schema, shipStatusID),
		fmt.Sprintf(`INSERT INTO %s.order_items(order_id, line_no, product_id, item_name, item_note, spec, qty, unit, unit_price, line_total)
			VALUES(1, 1, 1, '橘皮乌龙', '发货备注', '300g', 2, '件', 67, 134)`, schema),
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("seed delivery note order: %v", err)
		}
	}
}
