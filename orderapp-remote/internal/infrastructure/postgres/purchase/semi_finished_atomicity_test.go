package purchase

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	materialsapp "orderapp/internal/application/materials"
	purchaseapp "orderapp/internal/application/purchase"
	stockapp "orderapp/internal/application/stock"
	postgresmaterials "orderapp/internal/infrastructure/postgres/materials"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type controlledMaterialReceipt struct {
	pool            *pgxpool.Pool
	schema          string
	holdAfterCommit bool
	committed       chan struct{}
	release         chan struct{}
	commitOnce      sync.Once
}

func (s *controlledMaterialReceipt) ReceiveMaterial(ctx context.Context, cmd stockapp.MaterialReceiptCommand) (stockapp.MaterialReceiptResult, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return stockapp.MaterialReceiptResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var isSemiFinished bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(is_semi_finished,false)
		FROM %s.materials WHERE id=$1 FOR UPDATE
	`, s.schema), cmd.MaterialID).Scan(&isSemiFinished); err != nil {
		return stockapp.MaterialReceiptResult{}, err
	}
	if isSemiFinished {
		return stockapp.MaterialReceiptResult{}, fmt.Errorf("半成品只能通过生产入库，不能采购或采购收货")
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.materials
		SET onhand_g=onhand_g+$2,updated_at=now()
		WHERE id=$1
	`, s.schema), cmd.MaterialID, cmd.QtyG); err != nil {
		return stockapp.MaterialReceiptResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return stockapp.MaterialReceiptResult{}, err
	}
	s.commitOnce.Do(func() { close(s.committed) })
	if s.holdAfterCommit {
		select {
		case <-ctx.Done():
			return stockapp.MaterialReceiptResult{}, ctx.Err()
		case <-s.release:
		}
	}
	return stockapp.MaterialReceiptResult{ReceiptID: 701, BatchID: 702, BatchCode: "MB-0000000701"}, nil
}

func TestPurchaseReceiptAndSemiFinishedToggleAreSerializedPostgres(t *testing.T) {
	t.Run("receipt commits completely before toggle", func(t *testing.T) {
		pool, schema := setupSemiFinishedAtomicityDB(t)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		stock := &controlledMaterialReceipt{
			pool: pool, schema: schema, holdAfterCommit: true,
			committed: make(chan struct{}), release: make(chan struct{}),
		}
		var releaseOnce sync.Once
		releaseStock := func() { releaseOnce.Do(func() { close(stock.release) }) }
		defer releaseStock()

		purchaseSvc := purchaseapp.NewService(NewRepository(pool, schema), stock)
		materialsRepo := postgresmaterials.NewRepository(pool, schema)
		receiptResult := make(chan error, 1)
		go func() {
			_, err := purchaseSvc.CreatePurchaseReceipt(ctx, purchaseapp.CreatePurchaseReceiptCommand{
				SupplierID: 1, SupplierName: "测试供应商", MaterialID: 1,
				QtyG: 1000, UnitCost: 288, Operator: "buyer",
			})
			receiptResult <- err
		}()

		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-stock.committed:
		}

		toggleResult := make(chan error, 1)
		go func() {
			_, err := materialsRepo.Update(ctx, materialsapp.UpdateCommand{
				Actor: "materials-editor", ID: 1,
				Input: materialsapp.MaterialInput{
					Code: "MAT-1", Name: "待切换半成品", Kind: "bean",
					IsSemiFinished: true, IsSemiFinishedSet: true,
					Unit: "kg", CostUnit: "kg", PurchasePrice: 0, OnhandG: 1000,
				},
			})
			toggleResult <- err
		}()

		select {
		case err := <-toggleResult:
			releaseStock()
			<-receiptResult
			t.Fatalf("semi-finished toggle bypassed the in-flight purchase receipt lock: %v", err)
		case <-time.After(200 * time.Millisecond):
		}

		releaseStock()
		if err := <-receiptResult; err != nil {
			t.Fatalf("CreatePurchaseReceipt: %v", err)
		}
		if err := <-toggleResult; err != nil {
			t.Fatalf("toggle after complete receipt: %v", err)
		}

		assertSemiFinishedAtomicityState(t, pool, schema, true, 0, 1000, 1)
	})

	t.Run("toggle first rejects receipt without stock writes", func(t *testing.T) {
		pool, schema := setupSemiFinishedAtomicityDB(t)
		ctx := context.Background()
		materialsRepo := postgresmaterials.NewRepository(pool, schema)
		if _, err := materialsRepo.Update(ctx, materialsapp.UpdateCommand{
			Actor: "materials-editor", ID: 1,
			Input: materialsapp.MaterialInput{
				Code: "MAT-1", Name: "待切换半成品", Kind: "bean",
				IsSemiFinished: true, IsSemiFinishedSet: true,
				Unit: "kg", CostUnit: "kg", PurchasePrice: 0,
			},
		}); err != nil {
			t.Fatalf("toggle semi-finished: %v", err)
		}

		stock := &controlledMaterialReceipt{
			pool: pool, schema: schema, committed: make(chan struct{}), release: make(chan struct{}),
		}
		purchaseSvc := purchaseapp.NewService(NewRepository(pool, schema), stock)
		_, err := purchaseSvc.CreatePurchaseReceipt(ctx, purchaseapp.CreatePurchaseReceiptCommand{
			SupplierID: 1, SupplierName: "测试供应商", MaterialID: 1,
			QtyG: 1000, UnitCost: 288, Operator: "buyer",
		})
		if err == nil || !strings.Contains(err.Error(), "半成品只能通过生产入库") {
			t.Fatalf("CreatePurchaseReceipt error=%v, want manufacture-only rejection", err)
		}
		assertSemiFinishedAtomicityState(t, pool, schema, true, 0, 0, 0)
	})
}

func setupSemiFinishedAtomicityDB(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
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
	schema := fmt.Sprintf("pr600_semi_atomicity_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		pool.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	})
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %s.audit_logs(
			id BIGSERIAL PRIMARY KEY, actor TEXT NOT NULL DEFAULT '', entity_type TEXT NOT NULL DEFAULT '',
			entity_id BIGINT, action TEXT NOT NULL DEFAULT '', field TEXT, old_value TEXT, new_value TEXT,
			meta JSONB, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := postgresmaterials.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.purchase_suppliers(id,name) VALUES(1,'测试供应商');
		INSERT INTO %s.materials(id,code,name,kind,unit,cost_unit,purchase_price)
		VALUES(1,'MAT-1','待切换半成品','bean','kg','kg',288)
	`, schema, schema)); err != nil {
		t.Fatal(err)
	}
	return pool, schema
}

func assertSemiFinishedAtomicityState(t *testing.T, pool *pgxpool.Pool, schema string, wantSemi bool, wantPrice float64, wantOnhand int64, wantReceipts int) {
	t.Helper()
	ctx := context.Background()
	var semi bool
	var price float64
	var onhand int64
	var receipts int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT is_semi_finished,purchase_price::float8,onhand_g
		FROM %s.materials WHERE id=1
	`, schema)).Scan(&semi, &price, &onhand); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.purchase_receipts`, schema)).Scan(&receipts); err != nil {
		t.Fatal(err)
	}
	if semi != wantSemi || price != wantPrice || onhand != wantOnhand || receipts != wantReceipts {
		t.Fatalf("semi/price/onhand/receipts=%t/%.2f/%d/%d, want %t/%.2f/%d/%d", semi, price, onhand, receipts, wantSemi, wantPrice, wantOnhand, wantReceipts)
	}
}
