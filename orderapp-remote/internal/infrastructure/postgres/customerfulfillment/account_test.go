package customerfulfillment

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	app "orderapp/internal/application/customerfulfillment"
	"os"
	"testing"
	"time"
)

func TestCustomerAccountHistorySummaryFeesAndIsolation(t *testing.T) {
	dsn := os.Getenv("ORDERAPP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("test database required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	schema := fmt.Sprintf("test_account_%d", time.Now().UnixNano())
	_, err = pool.Exec(ctx, fmt.Sprintf(`CREATE SCHEMA %[1]s;
 CREATE TABLE %[1]s.customers(id bigint,name text);INSERT INTO %[1]s.customers VALUES(1,'客户一'),(2,'客户二');
 CREATE TABLE %[1]s.pay_statuses(id bigint,name text);INSERT INTO %[1]s.pay_statuses VALUES(1,'未付款'),(2,'预付款（付款未完成）'),(3,'已付款');
 CREATE TABLE %[1]s.ship_statuses(id bigint,name text);INSERT INTO %[1]s.ship_statuses VALUES(1,'未发货');
 CREATE TABLE %[1]s.orders(id bigint,customer_id bigint,order_no text,order_date date,receiver_name text,receiver_phone text,receiver_address text,ship_status_id bigint,pay_status_id bigint,ship_tracking_no text,portal_service_code text,is_void boolean,grand_total numeric,shipping_amount numeric,discount_amount numeric,prepayment_amount numeric);
 INSERT INTO %[1]s.orders SELECT i,1,'SO-'||i,'2026-09-09','张三','13800000000','测试地址',1,CASE WHEN i=1 THEN 2 WHEN i=2 THEN 3 ELSE 1 END,'','',false,100,10,5,CASE WHEN i=1 THEN 30 ELSE 0 END FROM generate_series(1,205)i;
 INSERT INTO %[1]s.orders SELECT 206,2,'FOREIGN','2026-09-09','','','',1,1,'','',false,999,0,0,0;
 INSERT INTO %[1]s.orders SELECT 207,1,'VOID','2026-09-09','','','',1,1,'','',true,999,0,0,0;
 INSERT INTO %[1]s.orders SELECT 208,1,'OLD','2026-08-09','','','',1,1,'','',false,100,0,0,0;
 CREATE TABLE %[1]s.order_items(id bigint,order_id bigint,item_name text,spec text,qty numeric,unit text,unit_price numeric,line_total numeric);INSERT INTO %[1]s.order_items VALUES(1,1,'测试豆','3kg',1,'袋',95,95);
 CREATE TABLE %[1]s.customer_settlement_batches(id bigint,customer_id bigint,settlement_no text,period_from date,period_to date,status text,total_amount numeric,confirmed_at timestamptz,paid_at timestamptz,created_at timestamptz);
 INSERT INTO %[1]s.customer_settlement_batches VALUES(1,1,'DRAFT','2026-09-01','2026-09-30','draft',10,null,null,now()),(2,1,'PAID','2026-09-01','2026-09-30','paid',20,now(),now(),now()),(3,2,'SECRET','2026-09-01','2026-09-30','confirmed',900,now(),null,now());
 CREATE TABLE %[1]s.customer_fee_items(id bigint,customer_id bigint,source_type text,source_id bigint,fee_type text,amount numeric,currency text,occurred_at timestamptz,settlement_batch_id bigint,status text);
 INSERT INTO %[1]s.customer_fee_items VALUES(1,1,'order',1,'shipping',10,'CNY','2026-09-09',1,'settled'),(2,1,'import',206,'adjustment',20,'CNY','2026-09-09',2,'settled'),(3,2,'order',206,'shipping',900,'CNY','2026-09-09',3,'unsettled');`, schema))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	repo := NewRepository(pool, schema)
	q, _ := app.NormalizeAccountQuery(app.AccountQuery{CustomerID: 1, Period: "week", Anchor: "2026-09-09", Limit: 20})
	d, err := repo.CustomerAccount(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Rows) != 205 || d.Summary.TotalCents != 2050000 || d.Summary.PaidCents != 13000 || d.Summary.DueCents != 2037000 {
		t.Fatalf("summary %+v rows %d", d.Summary, len(d.Rows))
	}
	if len(d.Fees) != 2 || len(d.Settlements) != 1 || d.Settlements[0].SettlementNo != "PAID" {
		t.Fatalf("fees/settlements: %+v %+v", d.Fees, d.Settlements)
	}
	for _, f := range d.Fees {
		if f.ID == 1 && f.PaymentStatus != "unpaid" {
			t.Fatal("draft marked paid")
		}
		if f.ID == 2 && f.OrderID != 0 {
			t.Fatal("guessed import order link")
		}
	}
	d.Paginate(2, 20)
	if len(d.Rows) != 20 || d.Total != 205 || d.TotalPages != 11 || d.Summary.TotalCents != 2050000 {
		t.Fatalf("pagination %+v", d)
	}
	q.OrderID = 206
	d, err = repo.CustomerAccount(ctx, q)
	if err != nil || len(d.Rows) != 0 {
		t.Fatalf("foreign order %+v %v", d.Rows, err)
	}
	q.OrderID = 1
	d, err = repo.CustomerAccount(ctx, q)
	if err != nil || len(d.Rows) != 1 || len(d.Rows[0].Items) != 1 {
		t.Fatalf("detail %+v %v", d.Rows, err)
	}
}
