package customerfulfillment

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	stockapp "orderapp/internal/application/stock"
	domain "orderapp/internal/domain/orderresolution"
	customerstockpg "orderapp/internal/infrastructure/postgres/customerstock"
	resolutionpg "orderapp/internal/infrastructure/postgres/orderresolution"
	migration "orderapp/internal/infrastructure/postgres/productspecmigration"
	stockpg "orderapp/internal/infrastructure/postgres/stock"
)

func ownershipReturnFixture(t *testing.T) (*pgxpool.Pool, string) {
	pool, schema := seedPR703StockProduct(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`
 INSERT INTO %[1]s.orders(id,order_no,customer_id,delivery_mode,delivery_warehouse_code,total_amount,grand_total)VALUES(78510,'OWNERSHIP-ORIGINAL',501,'customer_stock','DEV-E2E-A1',150,150),(78511,'OWNERSHIP-OTHER',501,'customer_stock','DEV-E2E-A1',60,60);
 INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,bom_spec_id,bom_variant_id,qty,unit,unit_price,line_total)VALUES(78510,78510,1,910,'咖啡',7031,7032,5,'袋',30,150),(78511,78511,1,910,'咖啡',7031,7032,2,'袋',30,60);
 INSERT INTO %[1]s.stock_batches(id,batch_code,item_type,item_id,item_name,bom_spec_id,bom_variant_id,spec_g,owner_customer_id,source_doc_type,source_doc_id,qty_units,remaining_units,quality_status,unit_cost)VALUES(78510,'OWNERSHIP-SHARED-BATCH','finished_product',910,'咖啡',7031,7032,0,501,'production_run',78510,5,5,'passed',12.5);
 UPDATE %[1]s.finished_inventory SET onhand_units=5 WHERE product_id=910 AND bom_spec_id=7031 AND warehouse='DEV-E2E-A1';
 INSERT INTO %[1]s.stock_ledger_entries(item_type,item_id,item_name,bom_spec_id,bom_variant_id,spec_g,owner_customer_id,warehouse,source_doc_type,source_doc_id,source_batch_code,qty_before_units,qty_change_units,qty_after_units)VALUES('finished_product',910,'咖啡',7031,7032,0,501,'DEV-E2E-A1','production_run',78510,'OWNERSHIP-SHARED-BATCH',0,5,5);
 `, schema))
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = customerstockpg.ReceiveProductionTx(ctx, tx, schema, 78510, 910, 7031, 7032, 0, 2, "OTHER-RECEIPT", "OWNERSHIP-SHARED-BATCH", "DEV-E2E-A1", "OWNERSHIP-OTHER", "warehouse"); err != nil {
		t.Fatal(err)
	}
	if err = customerstockpg.ReceiveProductionTx(ctx, tx, schema, 78510, 910, 7031, 7032, 0, 3, "ORIGINAL-RECEIPT", "OWNERSHIP-SHARED-BATCH", "DEV-E2E-A1", "OWNERSHIP-ORIGINAL", "warehouse"); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return pool, schema
}

func TestOrderResolutionOwnershipReturnRequiresActualReceiptAndAvailableBatch(t *testing.T) {
	pool, schema := ownershipReturnFixture(t)
	r := resolutionpg.NewRepository(pool, schema)
	current, err := r.Context(context.Background(), 78510)
	if err != nil {
		t.Fatal(err)
	}
	if len(current.ReturnSources) != 1 {
		t.Fatalf("missing original ownership receipt: %+v", current.ReturnSources)
	}
	source := current.ReturnSources[0]
	if source.Kind != "ownership" || source.ItemID != 78510 || source.BatchID != 78510 || source.AvailableQuantity != 3 || source.BatchAvailableQuantity != 5 || source.EntitlementID <= 0 {
		t.Fatalf("pending entitlement or other sale became returnable: %+v", source)
	}
	ordinary, err := stockpg.NewRepository(pool, schema).ListTransferOptions(context.Background(), stockapp.TransferOptionsQuery{Warehouse: "DEV-E2E-A1", ItemType: "finished_product", ItemID: 910, BomSpecID: 7031, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range ordinary.Rows {
		if row.BatchID == 78510 && row.UnavailableReason == "" {
			t.Fatal("ordinary transfer bypassed retained-sale ownership protection")
		}
	}
	for _, d := range source.Destinations {
		if d.Ownership != "company" || d.OwnerCustomerID != 0 {
			t.Fatalf("unexpected ownership destination: %+v", d)
		}
	}
}

func TestOrderResolutionOwnershipSourceRejectsMissingOrForeignFacts(t *testing.T) {
	for _, tc := range []struct{ name, sql string }{
		{"missing receipt ledger", `DELETE FROM %s.stock_ledger_entries WHERE source_batch_code='OWNERSHIP-SHARED-BATCH'`},
		{"another customer's batch", `UPDATE %s.stock_batches SET owner_customer_id=502 WHERE id=78510`},
		{"physical summary disagrees", `UPDATE %s.finished_inventory SET onhand_units=1 WHERE product_id=910 AND bom_spec_id=7031 AND warehouse='DEV-E2E-A1'`},
		{"quality hold", `UPDATE %s.stock_batches SET quality_status='hold' WHERE id=78510`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool, schema := ownershipReturnFixture(t)
			ctx := context.Background()
			if _, err := pool.Exec(ctx, fmt.Sprintf(tc.sql, schema)); err != nil {
				t.Fatal(err)
			}
			current, err := resolutionpg.NewRepository(pool, schema).Context(ctx, 78510)
			if err != nil {
				t.Fatal(err)
			}
			if len(current.ReturnSources) != 0 {
				t.Fatalf("unsafe ownership source offered: %+v", current.ReturnSources)
			}
		})
	}
}

func TestOrderResolutionOwnershipReturnMovesGoodsWithoutRewritingReceipt(t *testing.T) {
	ownershipReturnPosting(t, false)
}

func TestOrderResolutionOwnershipReturnHistoricalIdentity(t *testing.T) {
	ownershipReturnPosting(t, true)
}

func ownershipReturnPosting(t *testing.T, historical bool) {
	pool, schema := ownershipReturnFixture(t)
	ctx := context.Background()
	if historical {
		if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.production_bom_versions SET status='archived' WHERE id=7030;INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,output_qty,output_unit)VALUES(7033,7030,'V2','published',1,'袋');INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,logistics_weight_g)VALUES(7034,7033,7031,'454g袋','袋',true,454);UPDATE %[1]s.production_bom_output_bindings SET bom_version_id=7033 WHERE output_id=910`, schema)); err != nil {
			t.Fatal(err)
		}
		if err := migration.EnsureAuthorityProjection(ctx, pool, schema); err != nil {
			t.Fatal(err)
		}
	}
	r := resolutionpg.NewRepository(pool, schema)
	current, err := r.Context(ctx, 78510)
	if err != nil {
		t.Fatal(err)
	}
	if len(current.ReturnSources) != 1 {
		t.Fatal("original receipt missing")
	}
	source := current.ReturnSources[0]
	cmd := returnCommand(current.Version)
	cmd.Scope = "selected"
	cmd.Lines = []domain.Selection{{ItemID: 78510, Quantity: 2}}
	cmd.ReturnSettlement = "credit_returned_goods"
	cmd.ReturnSelections = []domain.ReturnSelection{{SourceKind: "ownership", SourceID: source.ID, BatchID: source.BatchID, Quantity: 2, Ownership: "company", Warehouse: "finished_goods", Quality: "quarantine"}}
	cmd.Dispositions = []domain.Disposition{{Kind: "customer_stock", ID: source.EntitlementID, Choice: "ownership_return"}}
	cmd.IdempotencyKey = "ownership-two"
	result, err := r.Submit(ctx, 78510, cmd, "sales")
	if err != nil {
		t.Fatal(err)
	}
	stockID := returnStepID(t, result, "return_stock")
	confirm := resolutionpg.StepCommand{Choice: "receive", Note: "明确收回原客户货权与两袋实物", Reference: "OWNERSHIP-RETURN-2", PostingDate: "2026-11-08", IdempotencyKey: "ownership-stock"}
	result, err = r.CompleteStep(ctx, 78510, result.ID, stockID, confirm, "warehouse")
	if err != nil {
		t.Fatal(err)
	}
	if result.Applied {
		t.Fatal("physical ownership return also confirmed finance")
	}
	if _, err = r.CompleteStep(ctx, 78510, result.ID, stockID, confirm, "warehouse"); err != nil {
		t.Fatal(err)
	}
	var sourceRemaining, customerInventory, companyInventory, targetOwner, targetUnits, received, returned, otherReceived, otherReturned, credit int64
	err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT
 (SELECT remaining_units FROM %[1]s.stock_batches WHERE id=78510),
 (SELECT onhand_units FROM %[1]s.finished_inventory WHERE product_id=910 AND bom_spec_id=7031 AND warehouse='DEV-E2E-A1'),
 (SELECT onhand_units FROM %[1]s.finished_inventory WHERE product_id=910 AND bom_spec_id=7031 AND warehouse='finished_goods'),
 (SELECT owner_customer_id FROM %[1]s.stock_batches WHERE source_doc_type='order_resolution_return'),
 (SELECT remaining_units FROM %[1]s.stock_batches WHERE source_doc_type='order_resolution_return'),
 (SELECT received_qty FROM %[1]s.customer_stock_entitlements WHERE source_order_id=78510),
 (SELECT COALESCE((to_jsonb(e)->>'returned_qty')::bigint,0) FROM %[1]s.customer_stock_entitlements e WHERE source_order_id=78510),
 (SELECT received_qty FROM %[1]s.customer_stock_entitlements WHERE source_order_id=78511),
 (SELECT COALESCE((to_jsonb(e)->>'returned_qty')::bigint,0) FROM %[1]s.customer_stock_entitlements e WHERE source_order_id=78511),
 (SELECT resolution_credit_cents FROM %[1]s.orders WHERE id=78510)`, schema)).Scan(&sourceRemaining, &customerInventory, &companyInventory, &targetOwner, &targetUnits, &received, &returned, &otherReceived, &otherReturned, &credit)
	if err != nil || sourceRemaining != 3 || customerInventory != 3 || companyInventory != 2 || targetOwner != 0 || targetUnits != 2 || received != 3 || returned != 2 || otherReceived != 2 || otherReturned != 0 || credit != 0 {
		t.Fatalf("ownership or original history incorrect: source=%d customer=%d company=%d target=%d/%d received=%d returned=%d other=%d/%d credit=%d error=%v", sourceRemaining, customerInventory, companyInventory, targetOwner, targetUnits, received, returned, otherReceived, otherReturned, credit, err)
	}
	assertMiniDirectShipCount(t, pool, schema, "customer_stock_entitlement_receipts", "stock_batch_id=78510", 2)
	assertMiniDirectShipCount(t, pool, schema, "stock_ledger_entries", "source_doc_type='order_resolution_ownership_return_out'", 1)
	result, err = r.CompleteStep(ctx, 78510, result.ID, returnStepID(t, result, "return_finance"), resolutionpg.StepCommand{Choice: "credit_returned_goods", Note: "按原销售两袋冲减商品应收", IdempotencyKey: "ownership-credit"}, "finance")
	if err != nil {
		t.Fatal(err)
	}
	current, err = r.Context(ctx, 78510)
	if err != nil || current.CreditCents != 6000 || current.Lines[0].Returned != 2 || current.Lines[0].Delivered != 3 {
		t.Fatalf("ownership and receivable mismatch: %+v %v", current, err)
	}
	supplies, err := customerstockpg.ListSupply(ctx, pool, schema, 501, "DEV-E2E-A1", false)
	if err != nil || len(supplies) != 1 || supplies[0].OrderID != 78510 || supplies[0].PendingQty != 2 {
		t.Fatalf("returned goods regenerated a production promise: %+v %v", supplies, err)
	}
	totals, err := customerstockpg.ListPurchaseTotals(ctx, pool, schema, 501, "DEV-E2E-A1")
	if err != nil || len(totals) != 1 || totals[0].PurchasedQty != 5 || totals[0].ReceivedQty != 3 {
		t.Fatalf("net retained purchase totals mismatch: %+v %v", totals, err)
	}
	sources, err := NewRepository(pool, schema).customerInventoryOrderSources(ctx, 501, []string{"DEV-E2E-A1"})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(sources)
	if !strings.Contains(string(raw), `"returned_qty":2`) {
		t.Fatalf("original receipt and returned ownership not separately visible: %s", raw)
	}
	raw, _ = json.Marshal(totals)
	if !strings.Contains(string(raw), `"ReturnedQty":2`) {
		t.Fatalf("net purchase totals lack the return explanation: %s", raw)
	}
	for _, sql := range []string{
		`UPDATE %s.customer_stock_entitlements SET returned_qty=0 WHERE source_order_id=78510`,
		`DELETE FROM %[1]s.customer_stock_entitlement_receipts WHERE entitlement_id IN(SELECT id FROM %[1]s.customer_stock_entitlements WHERE source_order_id=78510)`,
		`UPDATE %[1]s.customer_stock_entitlement_receipts SET quantity=1 WHERE entitlement_id IN(SELECT id FROM %[1]s.customer_stock_entitlements WHERE source_order_id=78510)`,
		`DELETE FROM %s.stock_ledger_entries WHERE source_doc_type='order_resolution_ownership_return_out'`,
	} {
		if _, err = pool.Exec(ctx, fmt.Sprintf(sql, schema)); err == nil {
			t.Fatalf("posted ownership history could be rewritten: %s", sql)
		}
	}
}

func TestOrderResolutionOwnershipCounterRequiresPostedReturn(t *testing.T) {
	pool, schema := ownershipReturnFixture(t)
	if _, err := pool.Exec(context.Background(), fmt.Sprintf(`UPDATE %s.customer_stock_entitlements SET returned_qty=1 WHERE source_order_id=78510`, schema)); err == nil {
		t.Fatal("unbacked ownership counter accepted")
	}
}

func ownershipReturnCommand(c domain.Context) domain.Command {
	cmd := returnCommand(c.Version)
	source := c.ReturnSources[0]
	cmd.Scope = "selected"
	cmd.Lines = []domain.Selection{{ItemID: 78510, Quantity: 2}}
	cmd.ReturnSelections = []domain.ReturnSelection{{SourceKind: "ownership", SourceID: source.ID, BatchID: source.BatchID, Quantity: 2, Ownership: "company", Warehouse: "finished_goods", Quality: "quarantine"}}
	cmd.Dispositions = []domain.Disposition{{Kind: "customer_stock", ID: source.EntitlementID, Choice: "ownership_return"}}
	return cmd
}
