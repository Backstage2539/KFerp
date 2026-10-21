package customerfulfillment

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	salesapp "orderapp/internal/application/sales"
	domain "orderapp/internal/domain/freight"
	freightpg "orderapp/internal/infrastructure/postgres/freight"
	salespg "orderapp/internal/infrastructure/postgres/sales"
	"testing"
	"time"
)

func TestLegacyMiniEmployeeFreightPreservesCurrentCharge(t *testing.T) {
	pool, schema := seedPR703StockProduct(t)
	ctx := context.Background()
	sales := salespg.NewRepository(pool, schema)
	fr := freightpg.NewRepository(pool, schema)
	product, price := int64(910), 30.0
	for index, shipping := range []float64{20, 0, 12.34} {
		// First save with no freight policy/template/address/weight configured;
		// then prove a free-shipping policy cannot rewrite the old form's charge.
		if index == 1 {
			if _, err := fr.SavePolicy(ctx, domain.Policy{CustomerID: 501, Scene: "sale", FreeShipping: true}, 0, "test"); err != nil {
				t.Fatal(err)
			}
		}
		t.Run(fmt.Sprint(shipping), func(t *testing.T) {
			cmd := salesapp.SaveOrderCommand{Actor: "employee:703", DraftEmployeeID: 703, LegacyEmployeeFreight: true,
				CustomerID: 501, OrderDate: time.Now(), DocumentDate: time.Now(), PayStatusID: 1, ShipStatusID: 1,
				ShippingAmount: shipping, PaymentGoodsAmount: 25, PaymentShippingAmount: shipping,
				Items: []salesapp.OrderItemCommand{{ProductID: &product, BomSpecID: 7031, BomVariantID: 7032, Name: "旧小程序录单", Units: 2, SalesUnit: "袋", ManualPrice: &price}}}
			result, err := sales.SaveOrder(ctx, cmd)
			if err != nil {
				t.Fatalf("old employee order must save without quote fields: %v", err)
			}
			var total, freight, goodsPaid, freightPaid float64
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT grand_total::float8,shipping_amount::float8,payment_goods_amount::float8,payment_shipping_amount::float8 FROM %s.orders WHERE id=$1`, schema), result.OrderID).Scan(&total, &freight, &goodsPaid, &freightPaid); err != nil {
				t.Fatal(err)
			}
			if total != 60+shipping || freight != shipping || goodsPaid != 25 || freightPaid != shipping {
				t.Fatalf("legacy money changed: total=%v freight=%v goodsPaid=%v freightPaid=%v", total, freight, goodsPaid, freightPaid)
			}
			var audits int
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.audit_logs WHERE entity_type='order' AND entity_id=$1 AND action='create' AND actor=$2`, schema), result.OrderID, cmd.Actor).Scan(&audits); err != nil || audits != 1 {
				t.Fatalf("saved legacy order must be visible in operation logs: count=%d err=%v", audits, err)
			}
			var raw []byte
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT snapshot_json FROM %s.order_freight_snapshots WHERE order_id=$1`, schema), result.OrderID).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			var snapshot domain.Snapshot
			if err := json.Unmarshal(raw, &snapshot); err != nil {
				t.Fatal(err)
			}
			cents := int64(shipping*100 + 0.5)
			if snapshot.Protocol != domain.OrderProtocol || snapshot.ChargeMode != "manual_current" || snapshot.CurrentReceivableCents != cents || snapshot.ManualAmountCents == nil || *snapshot.ManualAmountCents != cents || snapshot.Policy.FreeShipping {
				t.Fatalf("legacy freight must be a fixed current charge, including zero: %+v", snapshot)
			}
			settled, err := domain.Settlement(snapshot, 1800, 0)
			if err != nil || settled.DeltaCents != 0 {
				t.Fatalf("actual courier bill must not charge legacy order again: %+v %v", settled, err)
			}
		})
	}
}

func TestLegacyMiniEmployeeFreightDoesNotBypassQuoteValidation(t *testing.T) {
	pool, schema := seedPR703StockProduct(t)
	ctx := context.Background()
	sales := salespg.NewRepository(pool, schema)
	product, price := int64(910), 30.0
	base := salesapp.SaveOrderCommand{Actor: "employee:703", DraftEmployeeID: 703, LegacyEmployeeFreight: true,
		CustomerID: 501, OrderDate: time.Now(), DocumentDate: time.Now(), PayStatusID: 1, ShipStatusID: 1, ShippingAmount: 20,
		Items: []salesapp.OrderItemCommand{{ProductID: &product, BomSpecID: 7031, BomVariantID: 7032, Name: "协议保护", Units: 2, SalesUnit: "袋", ManualPrice: &price}}}
	for _, tc := range []struct {
		name   string
		change func(*salesapp.SaveOrderCommand)
	}{
		{"not_employee", func(c *salesapp.SaveOrderCommand) { c.DraftEmployeeID = 0 }},
		{"not_legacy", func(c *salesapp.SaveOrderCommand) { c.LegacyEmployeeFreight = false }},
		{"customer", func(c *salesapp.SaveOrderCommand) { c.CustomerSubmission = true }},
		{"unknown_protocol", func(c *salesapp.SaveOrderCommand) { c.FreightProtocol = "unknown" }},
		{"partial_quote", func(c *salesapp.SaveOrderCommand) { c.FreightQuoteToken = "tampered" }},
		{"partial_edit_token", func(c *salesapp.SaveOrderCommand) { c.ExpectedFreightToken = "tampered" }},
		{"override", func(c *salesapp.SaveOrderCommand) { c.FreightOverride = &domain.Override{} }},
		{"negative_amount", func(c *salesapp.SaveOrderCommand) { c.ShippingAmount = -1 }},
		{"fractional_cent", func(c *salesapp.SaveOrderCommand) { c.ShippingAmount = 0.001 }},
		{"excess_amount", func(c *salesapp.SaveOrderCommand) { c.ShippingAmount = float64(domain.MaxCents)/100 + 1 }},
		{"nonfinite_amount", func(c *salesapp.SaveOrderCommand) { c.ShippingAmount = math.Inf(1) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := base
			tc.change(&cmd)
			if _, err := sales.SaveOrder(ctx, cmd); err == nil {
				t.Fatalf("incomplete or untrusted quote accepted: %v", err)
			}
		})
	}
}
