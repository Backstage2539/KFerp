package customerfulfillment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http/httptest"
	salesapp "orderapp/internal/application/sales"
	freightdomain "orderapp/internal/domain/freight"
	domain "orderapp/internal/domain/orderresolution"
	freightpg "orderapp/internal/infrastructure/postgres/freight"
	repo "orderapp/internal/infrastructure/postgres/orderresolution"
	salespg "orderapp/internal/infrastructure/postgres/sales"
	resolutionhttp "orderapp/internal/interfaces/http/orderresolution"
	saleshttp "orderapp/internal/interfaces/http/sales"
	"orderapp/internal/interfaces/http/support"
	"testing"
	"time"
)

func TestOrderResolutionClosedOrderDateUsesNormalSave(t *testing.T) {
	for _, mode := range []string{"ship", "customer_stock"} {
		t.Run(mode, func(t *testing.T) {
			pool, schema := seedPR703StockProduct(t)
			ctx := context.Background()
			if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_shipping_policies(customer_id,scene,free_shipping,updated_by)VALUES(501,'sale',true,'fixture')`, schema)); err != nil {
				t.Fatal(err)
			}
			salesRepo := salespg.NewRepository(pool, schema)
			productID, price := int64(910), 30.0
			cmd := salesapp.SaveOrderCommand{FreightProtocol: "deferred_freight_v1", ShipMethod: "pickup", CustomerID: 501, PayStatusID: 1, ShipStatusID: 1, OrderDate: time.Now(), DocumentDate: time.Now(), DeliveryMode: mode, Actor: "sales", Items: []salesapp.OrderItemCommand{{ProductID: &productID, Name: "关闭后修改日期", Units: 4, BomSpecID: 7031, BomVariantID: 7032, SalesUnit: "bag", ManualPrice: &price}}}
			cmd.ShippingProvince, cmd.ShippingCity, cmd.ShippingDistrict = "云南省", "大理白族自治州", "大理市"
			quote := func() {
				if mode != "ship" {
					return
				}
				cmd.FreightProtocol = freightdomain.OrderProtocol
				tx, err := pool.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback(ctx)
				fr := freightpg.NewRepository(pool, schema)
				if cmd.EditID > 0 {
					snapshot, err := fr.SnapshotTx(ctx, tx, cmd.EditID)
					if err != nil || snapshot == nil {
						t.Fatalf("snapshot: %v", err)
					}
					cmd.ExpectedFreightToken = snapshot.Token
				}
				q, err := fr.QuoteOrderTx(ctx, tx, 501, cmd.EditID, cmd.ExpectedFreightToken, freightdomain.Destination{Province: cmd.ShippingProvince, City: cmd.ShippingCity, District: cmd.ShippingDistrict}, []freightpg.WeightItem{{ProductID: 910, BomSpecID: 7031, BomVariantID: 7032, Qty: 4}}, nil)
				if err != nil {
					t.Fatal(err)
				}
				cmd.FreightQuoteToken = q.Token
			}
			if mode == "customer_stock" {
				cmd.DeliveryWarehouseCode = "DEV-E2E-A1"
			}
			quote()
			original, err := salesRepo.SaveOrder(ctx, cmd)
			if err != nil {
				t.Fatal(err)
			}
			r := repo.NewRepository(pool, schema)
			c, err := r.Context(ctx, original.OrderID)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = r.Submit(ctx, original.OrderID, domain.Command{Action: "close", Reason: "customer_cancel", Scope: "all", Timing: "stop_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-07", Version: c.Version, IdempotencyKey: "close-date"}, "sales"); err != nil {
				t.Fatal(err)
			}
			var before, after string
			if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT to_jsonb(i)::text FROM %s.order_items i WHERE order_id=$1`, schema), original.OrderID).Scan(&before); err != nil {
				t.Fatal(err)
			}
			cmd.EditID = original.OrderID
			cmd.Items[0].ItemID = c.Lines[0].ID
			cmd.OrderDate = cmd.OrderDate.AddDate(0, 0, -1)
			quote()
			if _, err = salesRepo.SaveOrder(ctx, cmd); err != nil {
				t.Fatalf("normal date edit after close: %v", err)
			}
			if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT to_jsonb(i)::text FROM %s.order_items i WHERE order_id=$1`, schema), original.OrderID).Scan(&after); err != nil || before != after {
				t.Fatalf("date edit rewrote original goods or closing: %v", err)
			}
		})
	}
}

func TestOrderResolutionHTTPDatabaseLifecycle(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,customer_id,total_amount,grand_total,prepayment_amount,payment_goods_amount)VALUES(78200,'HTTP-RESOLUTION',501,90,90,90,90);INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total)VALUES(78200,78200,1,911,'coffee','1000g',3,'袋',30,90)`, schema))
	if err != nil {
		t.Fatal(err)
	}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("basic_auth_admin", true)
			c.Set("operator_employee", "isolated-api-actor")
			return next(c)
		}
	})
	e.Use(support.AuthorizationMiddleware(nil))
	resolutionhttp.RegisterRoutes(e, repo.NewRepository(pool, schema), nil)
	call := func(method, path string, body any, want int) []byte {
		t.Helper()
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(method, path, bytes.NewReader(raw))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("%s %s status=%d want=%d body=%s", method, path, rec.Code, want, rec.Body.String())
		}
		return rec.Body.Bytes()
	}
	endpoint := "/api/orders/78200/resolution"
	var loaded struct {
		Context        domain.Context    `json:"context"`
		Balances       []repo.Balance    `json:"balances"`
		BalanceHistory []repo.BalanceUse `json:"balance_history"`
	}
	if err = json.Unmarshal(call("GET", endpoint, nil, 200), &loaded); err != nil {
		t.Fatal(err)
	}
	cmd := domain.Command{Action: "close", Reason: "customer_cancel", Scope: "all", Timing: "stop_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-07", Version: loaded.Context.Version, IdempotencyKey: "http-close"}
	call("POST", endpoint+"/preview", cmd, 200)
	var result repo.Result
	if err = json.Unmarshal(call("POST", endpoint+"/submit", cmd, 200), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Applied || result.Status != "pending" || len(result.Steps) != 1 {
		t.Fatalf("HTTP submission not applied: %+v", result)
	}
	call("POST", fmt.Sprintf("%s/%d/steps/%d", endpoint, result.ID, result.Steps[0].ID), repo.StepCommand{Choice: "customer_credit", AmountCents: 9000, PostingDate: "2026-11-07", Note: "客户明确要求留存", IdempotencyKey: "http-retain"}, 200)
	if err = json.Unmarshal(call("GET", endpoint, nil, 200), &loaded); err != nil {
		t.Fatal(err)
	}
	if len(loaded.Balances) != 1 || loaded.Balances[0].AvailableCents != 9000 {
		t.Fatalf("HTTP balance missing: %+v", loaded)
	}
	use := repo.BalanceCommand{SourceEntryID: loaded.Balances[0].ID, ExpectedAvailableCents: 9000, StepCommand: repo.StepCommand{Choice: "refund", AmountCents: 1000, Account: "已选择付款账户", Reference: "isolated-voucher", PostingDate: "2026-11-07", Note: "测试实际退款登记", IdempotencyKey: "http-balance-refund"}}
	call("POST", endpoint+"/balance", use, 200)
	call("POST", endpoint+"/balance", use, 200)
	if err = json.Unmarshal(call("GET", endpoint, nil, 200), &loaded); err != nil {
		t.Fatal(err)
	}
	if len(loaded.Balances) != 1 || loaded.Balances[0].AvailableCents != 8000 || len(loaded.BalanceHistory) != 1 {
		t.Fatalf("HTTP refund reconciliation: %+v", loaded)
	}
	var cash float64
	var audit int
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT payment_goods_amount,(SELECT count(*)FROM %[1]s.audit_logs WHERE entity_type='order' AND entity_id=78200 AND actor='isolated-api-actor')FROM %[1]s.orders WHERE id=78200`, schema)).Scan(&cash, &audit); err != nil || cash != 90 || audit != 3 {
		t.Fatalf("cash=%v audit=%d err=%v", cash, audit, err)
	}
}

func TestOrderResolutionAmendmentCreatesReviewedOrderThroughSalesAPI(t *testing.T) {
	pool, schema := seedPR703StockProduct(t)
	ctx := context.Background()
	salesRepo := salespg.NewRepository(pool, schema)
	productID, price := int64(910), 30.0
	original, err := salesRepo.SaveOrder(ctx, salesapp.SaveOrderCommand{CustomerID: 501, PayStatusID: 1, ShipStatusID: 1, OrderDate: time.Now(), DocumentDate: time.Now(), DeliveryMode: "customer_stock", DeliveryWarehouseCode: "DEV-E2E-A1", Actor: "sales", Items: []salesapp.OrderItemCommand{{ProductID: &productID, Name: "修订原单", Units: 4, BomSpecID: 7031, BomVariantID: 7032, SalesUnit: "bag", ManualPrice: &price}}})
	if err != nil {
		t.Fatal(err)
	}
	resolutionRepo := repo.NewRepository(pool, schema)
	current, err := resolutionRepo.Context(ctx, original.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	result, err := resolutionRepo.Submit(ctx, original.OrderID, domain.Command{Action: "amend", Reason: "entry_error", Scope: "all", Timing: "settle_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-07", Version: current.Version, IdempotencyKey: "amend-api"}, "sales")
	if err != nil {
		t.Fatal(err)
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
	saleshttp.RegisterRoutes(e, saleshttp.Dependencies{Sales: salesapp.NewService(salesRepo)})
	payload := map[string]any{"amendment_id": result.AmendmentID, "customer_id": 501, "order_date": "2026-11-07", "pay_status_id": 1, "ship_status_id": 1, "delivery_mode": "customer_stock", "delivery_warehouse_code": "DEV-E2E-A1", "product_id": []string{"910"}, "bom_spec_id": []string{"7031"}, "bom_variant_id": []string{"7032"}, "item_name": []string{"修订后的数量"}, "qty": []string{"2"}, "sales_unit": []string{"bag"}, "tier_id": []string{"manual"}, "price_selection_mode": []string{"manual"}, "unit_price": []string{"30"}}
	save := func() *httptest.ResponseRecorder {
		raw, _ := json.Marshal(payload)
		request := httptest.NewRequest("POST", "/api/order", bytes.NewReader(raw))
		request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		response := httptest.NewRecorder()
		e.ServeHTTP(response, request)
		return response
	}
	response := save()
	if response.Code != 200 {
		t.Fatalf("amendment save: %d %s", response.Code, response.Body.String())
	}
	var revised int64
	var pending string
	var received float64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT o.id,o.confirmation_status,o.prepayment_amount FROM %[1]s.order_resolution_amendments a JOIN %[1]s.orders o ON o.id=a.new_order_id WHERE a.id=$1 AND o.amended_from_order_id=$2`, schema), result.AmendmentID, original.OrderID).Scan(&revised, &pending, &received); err != nil || pending != "pending" || received != 0 || revised == original.OrderID {
		t.Fatalf("new review/cash/link=%d %s %v err=%v", revised, pending, received, err)
	}
	if duplicate := save(); duplicate.Code == 200 {
		t.Fatal("duplicate amendment created a second order")
	}
	var count int
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*)FROM %s.orders WHERE customer_id=501`, schema)).Scan(&count); err != nil || count != 2 {
		t.Fatalf("failed duplicate left an orphan order: %d %v", count, err)
	}
}
