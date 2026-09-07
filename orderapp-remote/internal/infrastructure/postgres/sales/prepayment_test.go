package sales

import (
	"context"
	"fmt"
	"testing"
)

func TestPrepaymentSettlementValidationAndDocumentInvalidation(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer pool.Close()
	defer pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	_, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE %[1]s.orders(id BIGINT,grand_total NUMERIC,prepayment_amount NUMERIC);INSERT INTO %[1]s.orders VALUES(1,100,30);CREATE TABLE %[1]s.pay_statuses(id BIGINT,name TEXT);INSERT INTO %[1]s.pay_statuses VALUES(1,'未付款'),(2,'已付款'),(9,'预付款（付款未完成）');CREATE TABLE %[1]s.sales_order_documents(order_id BIGINT,is_latest BOOLEAN);INSERT INTO %[1]s.sales_order_documents VALUES(1,true),(2,true),(1,false);CREATE TABLE %[1]s.combined_sales_order_images(order_ids JSONB,is_latest BOOLEAN);INSERT INTO %[1]s.combined_sales_order_images VALUES('[1,2]',true),('[2,3]',true);`, schema))
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	for _, status := range []int64{2, 9} {
		if err := validateStoredPrepaymentTx(ctx, tx, schema, 1, status, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := validateStoredPrepaymentTx(ctx, tx, schema, 1, 1, nil); err == nil {
		t.Fatal("deposit must not become unpaid")
	}
	total := 20.0
	if err := validateStoredPrepaymentTx(ctx, tx, schema, 1, 9, &total); err == nil {
		t.Fatal("reduced total must not lose deposit")
	}
	for i := 0; i < 2; i++ {
		if err := invalidatePrepaymentDocumentsTx(ctx, tx, schema, 1); err != nil {
			t.Fatal(err)
		}
	}
	var latest, count, combined int
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FILTER(WHERE is_latest),count(*) FROM %s.sales_order_documents`, schema)).Scan(&latest, &count); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FILTER(WHERE is_latest) FROM %s.combined_sales_order_images`, schema)).Scan(&combined); err != nil {
		t.Fatal(err)
	}
	if latest != 1 || count != 3 || combined != 1 {
		t.Fatalf("invalidated unrelated data or lost history %d/%d/%d", latest, count, combined)
	}
}
