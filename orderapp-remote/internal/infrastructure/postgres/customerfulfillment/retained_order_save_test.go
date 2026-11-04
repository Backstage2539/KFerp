package customerfulfillment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	salesapp "orderapp/internal/application/sales"
	customerstock "orderapp/internal/infrastructure/postgres/customerstock"
	salespg "orderapp/internal/infrastructure/postgres/sales"
	saleshttp "orderapp/internal/interfaces/http/sales"
)

func TestRetainedOrderHTTPSavePreservesReceivedProgress(t *testing.T) {
	for _, scenario := range []string{"converted", "manual", "automatic", "kilogram"} {
		t.Run(scenario, func(t *testing.T) {
			frozen, automatic := scenario == "converted", scenario == "automatic"
			pool, schema := seedPR703StockProduct(t)
			ctx := context.Background()
			productID, price := int64(910), 30.0
			repo := salespg.NewRepository(pool, schema)
			cmd := salesapp.SaveOrderCommand{CustomerID: 501, PayStatusID: 1, ShipStatusID: 1, OrderDate: time.Now(), DocumentDate: time.Now(), DeliveryMode: "customer_stock", DeliveryWarehouseCode: "DEV-E2E-A1", Actor: "employee:703", ReceiverName: "历史收件人", ReceiverPhone: "13800000000", ReceiverAddress: "历史地址", Items: []salesapp.OrderItemCommand{{ProductID: &productID, Name: "留厂回归", Units: 4, BomSpecID: 7031, BomVariantID: 7032, SalesUnit: "bag", ManualPrice: &price}}}
			saved, err := repo.SaveOrder(ctx, cmd)
			if err != nil {
				t.Fatal(err)
			}
			var initialStatus string
			if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT delivery_status FROM %s.orders WHERE id=$1`, schema), saved.OrderID).Scan(&initialStatus); err != nil || initialStatus != "pending_receipt" {
				t.Fatalf("new retained status=%s err=%v", initialStatus, err)
			}
			var itemID int64
			if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.order_items WHERE order_id=$1`, schema), saved.OrderID).Scan(&itemID); err != nil {
				t.Fatal(err)
			}
			if automatic {
				if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_items SET price_overridden=false,price_source_json=jsonb_set(price_source_json,'{price_selection_mode}','"auto"'::jsonb,true) WHERE id=$1`, schema), itemID); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "kilogram" {
				if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_items SET spec='1kg' WHERE id=$1`, schema), itemID); err != nil {
					t.Fatal(err)
				}
			}
			selection := salesapp.SpecConversionSelection{GroupID: 718, Revision: 1, SourceSpecID: 7039, SourceVariantID: 7040}
			if frozen {
				quote := salesapp.SpecConversionQuote{SpecConversionQuoteRequest: salesapp.SpecConversionQuoteRequest{SpecConversionSelection: selection, CustomerID: 501, ProductID: 910, PriceRowKey: "legacy-row", TargetSpecID: 7031, TargetVariantID: 7032, Quantity: 4}, TargetSpecName: "454g袋", TargetUnit: "袋", UnitPrice: "30.00", LineTotal: "120.00"}
				snapshot, _ := json.Marshal(map[string]any{"price_selection_mode": "converted_tier", "spec_conversion": quote})
				if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_items SET price_source_json=$1 WHERE id=$2`, schema), snapshot, itemID); err != nil {
					t.Fatal(err)
				}
			}
			e := echo.New()
			e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
				return func(c echo.Context) error {
					c.Set("basic_auth_admin", true)
					c.Set("employee_id", int64(703))
					c.Set("actor", "employee:703")
					return next(c)
				}
			})
			saleshttp.RegisterRoutes(e, saleshttp.Dependencies{Sales: salesapp.NewService(repo)})
			for index, status := range []string{"partially_received", "received"} {
				tx, err := pool.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				key := fmt.Sprintf("RETAINED-718-%d", index)
				_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.stock_batches(batch_code,item_type,item_id,item_name,bom_spec_id,bom_variant_id,owner_customer_id,qty_units,remaining_units,quality_status) VALUES($1,'finished_product',910,'留厂回归',7031,7032,501,2,2,'passed')`, schema), key)
				if err == nil {
					_, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.finished_inventory SET onhand_units=onhand_units+2 WHERE product_id=910 AND bom_spec_id=7031 AND warehouse='DEV-E2E-A1'`, schema))
				}
				if err == nil {
					err = customerstock.ReceiveProductionTx(ctx, tx, schema, 718, 910, 7031, 7032, 0, 2, key, key, "DEV-E2E-A1", saved.OrderNo, "employee:703")
				}
				if err != nil {
					tx.Rollback(ctx)
					t.Fatal(err)
				}
				if err = tx.Commit(ctx); err != nil {
					t.Fatal(err)
				}
				payload := map[string]any{"edit_id": saved.OrderID, "customer_id": 501, "order_date": time.Now().AddDate(0, 0, index+1).Format("2006-01-02"), "pay_status_id": 1, "ship_status_id": 1, "delivery_mode": "customer_stock", "delivery_warehouse_code": "DEV-E2E-A1", "receiver_name": "历史收件人", "receiver_phone": "13800000000", "receiver_address": "历史地址", "product_id": []string{"910"}, "item_id": []string{strconv.FormatInt(itemID, 10)}, "bom_spec_id": []string{"7031"}, "bom_variant_id": []string{"7032"}, "item_name": []string{"留厂回归"}, "qty": []string{"4"}, "sales_unit": []string{"bag"}, "tier_id": []string{"manual"}, "price_selection_mode": []string{"manual"}, "unit_price": []string{"30"}}
				// Match buildOrderPayload in the Vue employee order editor, including
				// its empty/default fields and the frozen source read from OrderForm.
				var storedUnit, storedSpec, storedSource string
				if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT unit,spec,price_source_json::text FROM %s.order_items WHERE id=$1`, schema), itemID).Scan(&storedUnit, &storedSpec, &storedSource); err != nil {
					t.Fatal(err)
				}
				var leadingSpec int
				_, _ = fmt.Sscanf(storedSpec, "%d", &leadingSpec)
				payload["unit"] = []string{storedUnit}
				payload["sales_unit"] = []string{""}
				payload["spec"] = []string{strconv.Itoa(leadingSpec)}
				if scenario == "kilogram" {
					payload["spec"] = []string{"1000"}
				}
				payload["product_kind"] = []string{"roasted_bean"}
				payload["unit_bag_count"] = []string{"0"}
				payload["unit_bean_g"] = []string{"0"}
				payload["parent_product_id"] = []string{"910"}
				payload["price_source_json"] = []string{storedSource}
				payload["item_note"] = []string{""}
				payload["discount_type"] = []string{""}
				payload["discount_value"] = []string{""}
				if automatic {
					payload["tier_id"] = []string{"auto"}
					payload["price_selection_mode"] = []string{"auto"}
				}
				if frozen {
					raw, _ := json.Marshal(selection)
					payload["price_selection_mode"] = []string{"converted_tier"}
					delete(payload, "tier_id")
					delete(payload, "unit_price")
					payload["selected_price_row_key"] = []string{"legacy-row"}
					payload["spec_conversion"] = []string{string(raw)}
				}
				var beforeSnapshot string
				if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT to_jsonb(i)::text FROM %s.order_items i WHERE id=$1`, schema), itemID).Scan(&beforeSnapshot); err != nil {
					t.Fatal(err)
				}
				raw, _ := json.Marshal(payload)
				req := httptest.NewRequest("POST", "/api/order", bytes.NewReader(raw))
				req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, req)
				if rec.Code != 200 {
					t.Fatalf("unchanged retained item must allow date save: %d %s", rec.Code, rec.Body.String())
				}
				var afterSnapshot string
				if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT to_jsonb(i)::text FROM %s.order_items i WHERE id=$1`, schema), itemID).Scan(&afterSnapshot); err != nil {
					t.Fatal(err)
				}
				if afterSnapshot != beforeSnapshot {
					t.Fatalf("date-only save changed frozen item snapshot: before %s after %s", beforeSnapshot, afterSnapshot)
				}
				var audits int
				if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.audit_logs WHERE entity_type='order' AND entity_id=$1 AND action='update'`, schema), saved.OrderID).Scan(&audits); err != nil || audits < index+1 {
					t.Fatalf("save audit count=%d err=%v", audits, err)
				}
				var currentItemID int64
				var actualDate string
				if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT i.id,to_char(o.order_date,'YYYY-MM-DD') FROM %s.orders o JOIN %s.order_items i ON i.order_id=o.id WHERE o.id=$1`, schema, schema), saved.OrderID).Scan(&currentItemID, &actualDate); err != nil || currentItemID != itemID || actualDate != payload["order_date"] {
					t.Fatalf("item/date changed unexpectedly: item=%d want=%d date=%s want=%v err=%v", currentItemID, itemID, actualDate, payload["order_date"], err)
				}
				var actual, name string
				var received, physical int64
				var freight float64
				if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT delivery_status,receiver_name,shipping_amount FROM %s.orders WHERE id=$1`, schema), saved.OrderID).Scan(&actual, &name, &freight); err != nil {
					t.Fatal(err)
				}
				if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT received_qty FROM %s.customer_stock_entitlements WHERE source_order_id=$1`, schema), saved.OrderID).Scan(&received); err != nil {
					t.Fatal(err)
				}
				if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units FROM %s.finished_inventory WHERE product_id=910 AND bom_spec_id=7031 AND warehouse='DEV-E2E-A1'`, schema)).Scan(&physical); err != nil {
					t.Fatal(err)
				}
				if actual != status || received != int64((index+1)*2) || physical != received || name != "历史收件人" || freight != 0 {
					t.Fatalf("save changed retained progress: status=%s want=%s received=%d physical=%d recipient=%s freight=%v", actual, status, received, physical, name, freight)
				}
				if index == 1 && scenario == "manual" {
					payload["qty"] = []string{"5"}
					changed, _ := json.Marshal(payload)
					request := httptest.NewRequest("POST", "/api/order", bytes.NewReader(changed))
					request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
					response := httptest.NewRecorder()
					e.ServeHTTP(response, request)
					if response.Code < 400 || !strings.Contains(response.Body.String(), "留厂销售已有关联代发、生产或入库") {
						t.Fatalf("changed quantity must remain protected: %d %s", response.Code, response.Body.String())
					}
					for _, statement := range []string{
						fmt.Sprintf(`UPDATE %s.order_items SET spec='other' WHERE id=$1`, schema),
						fmt.Sprintf(`UPDATE %s.order_items SET unit='other' WHERE id=$1`, schema),
						fmt.Sprintf(`UPDATE %s.order_items SET bom_variant_id=9999 WHERE id=$1`, schema),
						fmt.Sprintf(`DELETE FROM %s.order_items WHERE id=$1`, schema),
					} {
						if _, err := pool.Exec(ctx, statement, itemID); err == nil || !strings.Contains(err.Error(), "留厂销售已有关联代发、生产或入库") {
							t.Fatalf("protected identity/delete must fail: %s: %v", statement, err)
						}
					}
					var qty, receipt, stock, auditCount int64
					if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT i.qty::bigint,e.received_qty,f.onhand_units,(SELECT count(*) FROM %s.audit_logs a WHERE a.entity_type='order' AND a.entity_id=o.id AND a.action='update') FROM %s.orders o JOIN %s.order_items i ON i.order_id=o.id JOIN %s.customer_stock_entitlements e ON e.source_order_item_id=i.id JOIN %s.finished_inventory f ON f.product_id=i.product_id AND f.bom_spec_id=i.bom_spec_id AND f.warehouse=e.warehouse_code WHERE o.id=$1`, schema, schema, schema, schema, schema), saved.OrderID).Scan(&qty, &receipt, &stock, &auditCount); err != nil || qty != 4 || receipt != 4 || stock != 4 || auditCount != int64(audits) {
						t.Fatalf("rejected edit changed data: qty=%d receipt=%d stock=%d audits=%d err=%v", qty, receipt, stock, auditCount, err)
					}
					// Real edits must never be silently discarded by snapshot reuse.
					payload["qty"] = []string{"4"}
					for _, tc := range []struct {
						name  string
						field string
						value []string
					}{
						{"price", "unit_price", []string{"31"}},
						{"discount", "discount_type", []string{"free"}},
						{"note", "item_note", []string{"new note"}},
						{"price source", "price_source_json", []string{`{"modified":true}`}},
						{"conflicting gram spec", "spec", []string{"123"}},
					} {
						prior, existed := payload[tc.field]
						payload[tc.field] = tc.value
						changed, _ = json.Marshal(payload)
						request = httptest.NewRequest("POST", "/api/order", bytes.NewReader(changed))
						request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
						response = httptest.NewRecorder()
						e.ServeHTTP(response, request)
						if response.Code < 400 {
							t.Fatalf("changed %s must remain blocked: %d %s", tc.name, response.Code, response.Body.String())
						}
						if existed {
							payload[tc.field] = prior
						} else {
							delete(payload, tc.field)
						}
					}
					var afterRejected string
					if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT to_jsonb(i)::text FROM %s.order_items i WHERE id=$1`, schema), itemID).Scan(&afterRejected); err != nil || afterRejected != beforeSnapshot {
						t.Fatalf("rejected changes altered item snapshot: err=%v before=%s after=%s", err, beforeSnapshot, afterRejected)
					}

				}
			}
		})
	}
}

func TestOrdinaryOrderEnteringCustomerStockInitializesReceipt(t *testing.T) {
	pool, schema := seedPR703StockProduct(t)
	ctx := context.Background()
	repo := salespg.NewRepository(pool, schema)
	productID, price := int64(910), 30.0
	cmd := salesapp.SaveOrderCommand{CustomerID: 501, PayStatusID: 1, ShipStatusID: 1, OrderDate: time.Now(), DocumentDate: time.Now(), Actor: "employee:703", DraftEmployeeID: 703, LegacyEmployeeFreight: true, DeliveryMode: "ship", Items: []salesapp.OrderItemCommand{{ProductID: &productID, Name: "留厂切换回归", Units: 4, BomSpecID: 7031, BomVariantID: 7032, SalesUnit: "bag", ManualPrice: &price}}}
	saved, err := repo.SaveOrder(ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	cmd.EditID = saved.OrderID
	cmd.DeliveryMode = "customer_stock"
	cmd.DeliveryWarehouseCode = "DEV-E2E-A1"
	if _, err = repo.SaveOrder(ctx, cmd); err != nil {
		t.Fatal(err)
	}
	var status string
	var promised, received int
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT o.delivery_status,e.promised_qty,e.received_qty FROM %s.orders o JOIN %s.customer_stock_entitlements e ON e.source_order_id=o.id WHERE o.id=$1`, schema, schema), saved.OrderID).Scan(&status, &promised, &received); err != nil || status != "pending_receipt" || promised != 4 || received != 0 {
		t.Fatalf("entry status=%s promised=%d received=%d err=%v", status, promised, received, err)
	}
}
