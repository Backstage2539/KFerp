package production

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	productionapp "orderapp/internal/application/production"
)

func TestRequirePlanMaterialsInWIP(t *testing.T) {
	cases := []struct {
		name      string
		source    productionapp.ProductionPlanComponentSource
		wantError bool
	}{
		{"现货不在WIP", productionapp.ProductionPlanComponentSource{ComponentName: "咖啡袋", RequiredUnits: 10, WIPCoveredUnits: 3, TransferUnits: 7}, true},
		{"现货已经入库", productionapp.ProductionPlanComponentSource{ComponentName: "咖啡袋", RequiredUnits: 10, WIPCoveredUnits: 10}, false},
		{"前道计划产出不要求现货", productionapp.ProductionPlanComponentSource{ComponentName: "熟豆", RequiredG: 0, UpstreamG: 83000, DemandG: 83000}, false},
		{"前道与现货混合但现货未到", productionapp.ProductionPlanComponentSource{ComponentName: "熟豆", RequiredG: 1000, UpstreamG: 83000, DemandG: 84000, WIPCoveredG: 500}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := requirePlanMaterialsInWIP([]productionapp.ProductionPlanComponentSource{tc.source})
			if (err != nil) != tc.wantError {
				t.Fatalf("error = %v, want error = %v", err, tc.wantError)
			}
			if err != nil && !strings.Contains(err.Error(), tc.source.ComponentName) {
				t.Fatalf("missing material name: %v", err)
			}
		})
	}
}

func TestPendingPlanTransfersBlockWorkOrders(t *testing.T) {
	dsn := os.Getenv("ORDERAPP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated ORDERAPP_TEST_DATABASE_URL required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	schema := fmt.Sprintf("test_plan_pending_%d", time.Now().UnixNano())
	if _, err = pool.Exec(ctx, fmt.Sprintf(`CREATE SCHEMA %s; CREATE TABLE %s.stock_entries (id bigint, status text); CREATE TABLE %s.production_plan_material_transfer_links (production_plan_id bigint, stock_entry_id bigint); INSERT INTO %s.stock_entries VALUES (1,'draft'),(2,'submitted'),(3,'cancelled'); INSERT INTO %s.production_plan_material_transfer_links VALUES (42,1),(43,2),(44,3)`, schema, schema, schema, schema, schema)); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	for _, tc := range []struct {
		id      int64
		blocked bool
	}{{42, true}, {43, false}, {44, false}, {45, false}} {
		err = requireNoPendingPlanTransfersTx(ctx, tx, schema, tc.id)
		if (err != nil) != tc.blocked {
			t.Fatalf("plan %d: err=%v, blocked=%v", tc.id, err, tc.blocked)
		}
	}
	legacySchema := fmt.Sprintf("test_plan_legacy_%d", time.Now().UnixNano())
	if _, err = tx.Exec(ctx, "CREATE SCHEMA "+legacySchema); err != nil {
		t.Fatal(err)
	}
	if err = requireNoPendingPlanTransfersTx(ctx, tx, legacySchema, 42); err != nil {
		t.Fatalf("schema without transfer feature cannot have pending transfers: %v", err)
	}
}
