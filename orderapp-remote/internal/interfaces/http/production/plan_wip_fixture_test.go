package production

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	stockapp "orderapp/internal/application/stock"
	postgresstock "orderapp/internal/infrastructure/postgres/stock"
)

// moveFixtureStockToWIP uses the actual warehouse posting path before plan submission.
func moveFixtureStockToWIP(t *testing.T, pool *pgxpool.Pool, schema string, item stockapp.StockDocumentItemCommand) {
	t.Helper()
	svc := stockapp.NewService(postgresstock.NewRepository(pool, schema))
	item.ToWarehouse = "wip"
	if _, err := svc.CreateAndSubmitStockDocument(context.Background(), stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialTransfer, Operator: "隔离测试仓库员", Items: []stockapp.StockDocumentItemCommand{item}}); err != nil {
		t.Fatalf("post fixture transfer into WIP: %v", err)
	}
}

func moveFixtureTypedFinishedToWIP(t *testing.T, pool *pgxpool.Pool, schema string) {
	t.Helper()
	moveFixtureStockToWIP(t, pool, schema, stockapp.StockDocumentItemCommand{ItemType: "finished_product", ProductID: 2, SpecG: 1, InventoryUnit: "g", QtyUnits: 10000, BatchCode: "FP-TYPED-ROASTED", FromWarehouse: "finished_goods"})
}
