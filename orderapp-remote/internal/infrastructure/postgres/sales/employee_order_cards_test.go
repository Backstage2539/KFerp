package sales

import (
	"context"
	"fmt"
	"testing"

	salesapp "orderapp/internal/application/sales"
	postgrescore "orderapp/internal/infrastructure/postgres/core"
)

func TestEmployeeOrderCardsScopeSearchAndCursorPostgres(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer func() { _, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE"); pool.Close() }()
	if err := postgrescore.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	_, err := pool.Exec(ctx, fmt.Sprintf(`
ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS confirmation_pending jsonb, ADD COLUMN IF NOT EXISTS confirmation_status text DEFAULT 'accepted', ADD COLUMN IF NOT EXISTS prepayment_amount numeric DEFAULT 0;
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS product_kind text;
CREATE TABLE IF NOT EXISTS %[1]s.order_shipping_trackings (id bigserial primary key, order_id bigint,tracking_no text);
INSERT INTO %[1]s.customers(id,name) VALUES(500,'Alpha'),(501,'Bravo');
INSERT INTO %[1]s.orders(id,order_no,document_date,customer_id,responsible_party_type,responsible_party_id,grand_total,is_void) VALUES
(81,'single-A','2026-11-10',500,'employee',7,80,false),
(82,'single-B','2026-11-10',500,'employee',7,82,false),
(83,'old','2025-08-01',500,'employee',99,10,false),
(84,'void','2026-11-11',500,'employee',7,50,true),
(85,'other','2026-11-11',500,'employee',99,30,false),
(86,'null-date',null,500,'employee',7,60,false),
(87,'lost','2026-11-12',500,'employee',7,40,false),
(88,'older-null',null,500,'employee',7,40,false);
UPDATE %[1]s.orders SET confirmation_status='pending',confirmation_pending='{"order":{"responsible_party_id":7,"order_no":"proposed","document_date":"2026-11-11","customer_id":501,"grand_total":50,"prepayment_amount":20},"items":[{"order_id":83,"qty":1,"product_id":1,"product_kind":"roasted"}]}' WHERE id=83;
UPDATE %[1]s.orders SET confirmation_pending='{"order":{"responsible_party_id":99},"items":[]}' WHERE id=87;
UPDATE %[1]s.orders SET confirmation_pending='null'::jsonb WHERE id=81;
UPDATE %[1]s.orders SET confirmation_pending='{"items":null}'::jsonb WHERE id=86;
INSERT INTO %[1]s.order_items(order_id,qty,product_id,product_kind) VALUES(81,1,1,'roasted');
INSERT INTO %[1]s.order_shipping_trackings(order_id,tracking_no) VALUES (81,'TRACK-ME');
`, schema))
	if err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	q := salesapp.EmployeeOrderCardsQuery{Scope: "mine", EmployeeID: 7, Limit: 2}
	first, err := repo.ListEmployeeOrderCards(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Rows) != 2 || first.Rows[0].ID != 83 || first.Rows[1].ID != 82 || first.Next == nil {
		t.Fatalf("first %+v", first)
	}
	card := first.Rows[0]
	if card.Customer != "Bravo" || card.OrderNo != "proposed" || card.PaidAmount != "20.00" || card.UnpaidAmount != "30.00" || card.FulfillmentProgress.Label != "待确认" || card.ProcessStatus != "待计划" {
		t.Fatalf("current card %+v", card)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf("INSERT INTO %s.orders(id,document_date,responsible_party_type,responsible_party_id) VALUES(89,'2026-11-13','employee',7)", schema)); err != nil {
		t.Fatal(err)
	}
	q.After = first.Next
	second, err := repo.ListEmployeeOrderCards(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Rows) != 2 || second.Rows[0].ID != 81 || second.Rows[0].ProcessStatus != "待计划" || second.Rows[1].ID != 88 || second.Next == nil || second.Next.Date != "" {
		t.Fatalf("second %+v", second)
	}
	q.After = second.Next
	last, err := repo.ListEmployeeOrderCards(ctx, q)
	if err != nil || len(last.Rows) != 1 || last.Rows[0].ID != 86 || last.Next != nil {
		t.Fatalf("last %+v err %v", last, err)
	}
	q.After = nil
	for keyword, id := range map[string]int64{"Bravo": 83, "TRACK-ME": 81, "lost": 0} {
		q.Q = keyword
		found, err := repo.ListEmployeeOrderCards(ctx, q)
		if err != nil || id == 0 && len(found.Rows) != 0 || id != 0 && (len(found.Rows) != 1 || found.Rows[0].ID != id) {
			t.Fatalf("search %q got %+v err %v", keyword, found, err)
		}
	}
	q.Q = ""
	q.EmployeeID = 0
	closed, err := repo.ListEmployeeOrderCards(ctx, q)
	if err != nil || len(closed.Rows) != 0 {
		t.Fatalf("missing employee scope %+v err %v", closed, err)
	}
}
