package customerfulfillment

import (
	"context"
	"fmt"
	app "orderapp/internal/application/customerfulfillment"
	salesapp "orderapp/internal/application/sales"
	domain "orderapp/internal/domain/orderresolution"
	customerstock "orderapp/internal/infrastructure/postgres/customerstock"
	resolutionpg "orderapp/internal/infrastructure/postgres/orderresolution"
	salespg "orderapp/internal/infrastructure/postgres/sales"
	"testing"
	"time"
)

func TestOrderResolutionReplacementExplicitlyRetainsFullyConvertedSource(t *testing.T) {
	pool, schema := seedPR703StockProduct(t)
	ctx := context.Background()
	sr := salespg.NewRepository(pool, schema)
	product, price := int64(910), 30.0
	item := salesapp.OrderItemCommand{ProductID: &product, Name: "混合独立行", Units: 3, BomSpecID: 7031, BomVariantID: 7032, SalesUnit: "bag", ManualPrice: &price}
	sale := func(items []salesapp.OrderItemCommand) salesapp.SaveOrderResult {
		result, err := sr.SaveOrder(ctx, salesapp.SaveOrderCommand{CustomerID: 501, PayStatusID: 1, ShipStatusID: 1, OrderDate: time.Now(), DocumentDate: time.Now(), DeliveryMode: "customer_stock", DeliveryWarehouseCode: "DEV-E2E-A1", Actor: "employee:703", Items: items})
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	source, target := sale([]salesapp.OrderItemCommand{item, item}), sale([]salesapp.OrderItemCommand{item})
	configurePR703StockFees(t, pool, schema, 7031)
	service := app.NewService(NewRepository(pool, schema))
	service.UseSalesOrderService(salesapp.NewService(sr))
	ship := app.MiniDirectShipCommand{FreightProtocol: "deferred_freight_v1", CustomerID: 501, FulfillmentMode: "stock", IdempotencyKey: "mixed-full-ship", RecipientName: "隔离样本", RecipientPhone: "13800000000", Province: "云南省", City: "大理市", DetailAddress: "测试路1号", Actor: "employee:703", Items: []app.MiniDirectShipItemCommand{{ProductID: 910, BomSpecID: 7031, BomVariantID: 7032, Qty: 6}}}
	preview, err := service.PreviewMiniDirectShip(ctx, ship)
	if err != nil {
		t.Fatal(err)
	}
	ship.PriceQuoteToken, ship.FeeQuoteToken = preview.PriceQuoteToken, preview.FeeQuoteToken
	request, err := service.SubmitMiniDirectShip(ctx, ship)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.stock_batches(batch_code,item_type,item_id,item_name,spec_g,bom_spec_id,bom_variant_id,owner_customer_id,qty_units,remaining_units,quality_status) VALUES('FULL-CONVERTED','finished_product',910,'混合独立行',0,7031,7032,501,3,3,'passed')`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.finished_inventory SET onhand_units=3 WHERE product_id=910 AND bom_spec_id=7031 AND warehouse='DEV-E2E-A1'`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.stock_ledger_entries(item_type,item_id,item_name,owner_customer_id,spec_g,bom_spec_id,bom_variant_id,warehouse,source_doc_type,source_doc_id,source_batch_code,qty_change_units,qty_after_units) VALUES('finished_product',910,'混合独立行',501,0,7031,7032,'DEV-E2E-A1','production_run',703,'FULL-CONVERTED',3,3)`, schema)); err != nil {
		t.Fatal(err)
	}
	if err = customerstock.ReceiveProductionTx(ctx, tx, schema, 703, 910, 7031, 7032, 0, 3, "FULL-CONVERTED", "FULL-CONVERTED", "DEV-E2E-A1", source.OrderNo, "employee:703"); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	r := resolutionpg.NewRepository(pool, schema)
	current, err := r.Context(ctx, source.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	var targetID int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.customer_stock_entitlements WHERE source_order_id=$1`, schema), target.OrderID).Scan(&targetID); err != nil {
		t.Fatal(err)
	}
	cmd := domain.Command{Action: "close", Reason: "customer_cancel", Scope: "all", Timing: "settle_first", ExternalStatus: "unknown", PostingMode: "open_period", PostingDate: "2026-11-08", Version: current.Version, IdempotencyKey: "mixed-full-close"}
	var physicalID int64
	for _, d := range current.Dependencies {
		if d.Kind == "customer_stock" {
			cmd.Dispositions = append(cmd.Dispositions, domain.Disposition{Kind: d.Kind, ID: d.ID, Choice: "keep_sale"})
		}
		if d.Kind == "direct_ship" {
			if len(d.SupplyReservations) != 2 {
				t.Fatalf("fully converted source omitted from explicit choices: %+v", d)
			}
			choice := domain.Disposition{Kind: d.Kind, ID: d.ID, Choice: "replace_supply"}
			for _, s := range d.SupplyReservations {
				selected := domain.SupplyReplacement{ReservationID: s.ID, Quantity: s.Quantity}
				if s.Quantity > 0 {
					selected.TargetID = targetID
				}
				if s.ConvertedQuantity > 0 {
					physicalID = s.ID
					selected.PhysicalChoice = "keep_reserved"
					selected.PhysicalVersion = s.PhysicalVersion
				}
				choice.Supplies = append(choice.Supplies, selected)
			}
			cmd.Dispositions = append(cmd.Dispositions, choice)
		}
	}
	if physicalID == 0 {
		t.Fatal("missing fully converted source")
	}
	var physicalBefore string
	snapshot := func() string {
		var raw string
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT jsonb_build_object('allocations',(SELECT jsonb_agg(to_jsonb(x) ORDER BY id) FROM %[1]s.order_stock_batch_allocations x),'direct',(SELECT jsonb_agg(to_jsonb(x) ORDER BY id) FROM %[1]s.customer_direct_ship_request_allocations x),'reservation',(SELECT to_jsonb(x) FROM %[1]s.customer_stock_supply_reservations x WHERE id=$1))::text`, schema), physicalID).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		return raw
	}
	physicalBefore = snapshot()
	if _, err = domain.Preview(current, cmd); err != nil {
		t.Fatal(err)
	}
	result, err := r.Submit(ctx, source.OrderID, cmd, "sales")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Steps) != 2 {
		t.Fatalf("unexpected steps: %+v", result.Steps)
	}
	var stockStep, externalStep int64
	for _, step := range result.Steps {
		if step.Kind == "direct_ship" {
			stockStep = step.ID
		}
		if step.Kind == "external" {
			externalStep = step.ID
		}
	}
	result, err = r.CompleteStep(ctx, source.OrderID, result.ID, stockStep, resolutionpg.StepCommand{Choice: "replace_supply", Note: "保留已入库三袋，明确替换另一笔三袋待入库预订", IdempotencyKey: "mixed-full-confirm"}, "warehouse")
	if err != nil {
		t.Fatal(err)
	}
	if result.Applied || result.Status != "pending" || snapshot() != physicalBefore {
		t.Fatal("physical retention was not atomic or changed historical records")
	}
	// The replacement can arrive while a separate external-accounting check is pending.
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.stock_batches(batch_code,item_type,item_id,item_name,spec_g,bom_spec_id,bom_variant_id,owner_customer_id,qty_units,remaining_units,quality_status) VALUES('FULL-TARGET','finished_product',910,'替代货源',0,7031,7032,501,3,3,'passed')`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.finished_inventory SET onhand_units=6 WHERE product_id=910 AND bom_spec_id=7031 AND warehouse='DEV-E2E-A1'`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.stock_ledger_entries(item_type,item_id,item_name,owner_customer_id,spec_g,bom_spec_id,bom_variant_id,warehouse,source_doc_type,source_doc_id,source_batch_code,qty_change_units,qty_after_units) VALUES('finished_product',910,'替代货源',501,0,7031,7032,'DEV-E2E-A1','production_run',704,'FULL-TARGET',3,6)`, schema)); err != nil {
		t.Fatal(err)
	}
	if err = customerstock.ReceiveProductionTx(ctx, tx, schema, 704, 910, 7031, 7032, 0, 3, "FULL-TARGET", "FULL-TARGET", "DEV-E2E-A1", target.OrderNo, "employee:703"); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	result, err = r.CompleteStep(ctx, source.OrderID, result.ID, externalStep, resolutionpg.StepCommand{Choice: "none", Note: "已核实未在外部入账", IdempotencyKey: "mixed-full-external"}, "finance")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || result.Status != "completed" {
		t.Fatalf("completed warehouse disposition stranded later finance confirmation: %+v", result)
	}
	current, err = r.Context(ctx, source.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	var delivered, closed float64
	for _, l := range current.Lines {
		delivered += l.Delivered
		closed += l.Closed
	}
	if delivered != 3 || closed != 3 || current.CreditCents != 9000 {
		t.Fatalf("wrong mixed financial/quantity outcome: %+v", current)
	}
	_ = request
}
