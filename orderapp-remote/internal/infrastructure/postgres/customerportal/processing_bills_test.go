package customerportal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	customerportalapp "orderapp/internal/application/customerportal"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCustomerProcessingBillQueriesAreTenantScopedAndOnlyExposePushedBills(t *testing.T) {
	body, err := os.ReadFile("processing_bills.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, marker := range []string{
		"s.customer_id=$1",
		"s.id=$2",
		"s.processing_billing_run_id>0",
		"s.status IN ('confirmed','settled','paid','reversed')",
		"r.status IN ('confirmed','paid','reversed')",
		"processing_billing_work_orders",
		"processing_billing_line_snapshots",
	} {
		if !strings.Contains(text, marker) {
			t.Fatalf("customer bill query missing isolation marker %q", marker)
		}
	}
}

func TestCustomerProcessingBillsOnlyReturnCurrentCustomerConfirmedSnapshots(t *testing.T) {
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
	schema := fmt.Sprintf("customer_processing_bills_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		pool.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	})
	for _, statement := range []string{
		fmt.Sprintf(`CREATE TABLE %s.customer_settlement_batches(
			id BIGINT PRIMARY KEY,customer_id BIGINT NOT NULL,settlement_no TEXT,status TEXT,total_amount NUMERIC,
			confirmed_at TIMESTAMPTZ,paid_at TIMESTAMPTZ,created_at TIMESTAMPTZ,processing_billing_run_id BIGINT
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.processing_billing_runs(
			id BIGINT PRIMARY KEY,customer_id BIGINT NOT NULL,settlement_batch_id BIGINT,status TEXT,currency TEXT
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.processing_billing_work_orders(
			id BIGINT PRIMARY KEY,billing_run_id BIGINT NOT NULL,work_order_id BIGINT,work_order_no TEXT,product_name TEXT,completed_at TIMESTAMPTZ
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.processing_billing_line_snapshots(
			id BIGINT PRIMARY KEY,billing_run_id BIGINT NOT NULL,work_order_id BIGINT,fee_type TEXT,fee_name TEXT,basis TEXT,
			base_quantity NUMERIC,unit_price NUMERIC,amount NUMERIC
		)`, schema),
		fmt.Sprintf(`INSERT INTO %s.processing_billing_runs(id,customer_id,settlement_batch_id,status,currency) VALUES
			(101,19,201,'confirmed','CNY'),(102,20,202,'confirmed','CNY'),(103,19,203,'confirmed','CNY')`, schema),
		fmt.Sprintf(`INSERT INTO %s.customer_settlement_batches(id,customer_id,settlement_no,status,total_amount,confirmed_at,created_at,processing_billing_run_id) VALUES
			(201,19,'CPB-19-201','confirmed',66,now(),now(),101),
			(202,20,'CPB-20-202','confirmed',88,now(),now(),102),
			(203,19,'CPB-19-203','draft',99,NULL,now(),103)`, schema),
		fmt.Sprintf(`INSERT INTO %s.processing_billing_work_orders(id,billing_run_id,work_order_id,work_order_no,product_name,completed_at) VALUES
			(301,101,91,'WO-91','客户甲产品',now()),(302,102,92,'WO-92','客户乙产品',now()),(303,103,93,'WO-93','草稿产品',now())`, schema),
		fmt.Sprintf(`INSERT INTO %s.processing_billing_line_snapshots(id,billing_run_id,work_order_id,fee_type,fee_name,basis,base_quantity,unit_price,amount) VALUES
			(401,101,91,'processing','烘焙费','actual_output_kg',8,2,16),
			(402,101,91,'processing','工厂物料费','factory_material_actual_cost',50,1,50),
			(403,102,92,'processing','客户乙费用','actual_output_kg',8,11,88)`, schema),
	} {
		if _, err := pool.Exec(ctx, statement); err != nil {
			t.Fatalf("seed processing bill schema: %v\n%s", err, statement)
		}
	}

	repo := NewRepository(pool, schema)
	bills, err := repo.ListCustomerProcessingBills(ctx, 19, 100)
	if err != nil || len(bills) != 1 || bills[0].ID != 201 || bills[0].TotalAmount != "66.00" {
		t.Fatalf("ListCustomerProcessingBills()=%+v err=%v", bills, err)
	}
	detail, err := repo.GetCustomerProcessingBill(ctx, 19, 201)
	if err != nil || len(detail.WorkOrders) != 1 || len(detail.Lines) != 2 || detail.Lines[1].Amount != "50.00" {
		t.Fatalf("GetCustomerProcessingBill()=%+v err=%v", detail, err)
	}
	if _, err := repo.GetCustomerProcessingBill(ctx, 19, 202); !errors.Is(err, customerportalapp.ErrCustomerBillNotFound) {
		t.Fatalf("cross-customer detail err=%v, want ErrCustomerBillNotFound", err)
	}
	if _, err := repo.GetCustomerProcessingBill(ctx, 19, 203); !errors.Is(err, customerportalapp.ErrCustomerBillNotFound) {
		t.Fatalf("draft detail err=%v, want ErrCustomerBillNotFound", err)
	}
}
