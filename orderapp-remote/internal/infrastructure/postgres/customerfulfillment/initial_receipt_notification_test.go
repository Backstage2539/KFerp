package customerfulfillment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"testing"
	"time"

	app "orderapp/internal/application/customerfulfillment"
	messages "orderapp/internal/application/messagecenter"
	salesapp "orderapp/internal/application/sales"
	domain "orderapp/internal/domain/freight"
	pg "orderapp/internal/infrastructure/postgres/messagecenter"
	salespg "orderapp/internal/infrastructure/postgres/sales"
	wechat "orderapp/internal/infrastructure/wechataccount"
)

func TestInitialOrderReceiptNotificationOutbox(t *testing.T) {
	pool, schema := seedPR703StockProduct(t)
	ctx := context.Background()
	if err := pg.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %[1]s.wechat_bindings(app_id text,openid text,mini_user_id bigint,customer_id bigint,active boolean DEFAULT true,bound_at timestamptz DEFAULT now(),PRIMARY KEY(app_id,openid))`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := pg.EnsureExternalSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	repo := pg.NewRepository(pool, schema)
	tpl := wechat.TemplateMapping{ID: "receipt", Fields: map[string]string{"thing1": "customer_name", "time2": "receipt_date", "amount3": "receivable_amount", "character_string4": "receipt_no", "amount6": "received_amount"}}
	cfg := pg.DefaultExternalConfig()
	cfg.OfficialEnabled = true
	cfg.OfficialEvents = []string{"receipt_received"}
	cfg.OfficialSince = time.Now().Add(-time.Minute)
	cfg.OfficialEventSince = map[string]time.Time{"receipt_received": time.Now().Add(-time.Minute)}
	cfg.Templates["receipt_received"] = tpl
	configRaw, _ := json.Marshal(cfg)
	templateRaw, _ := json.Marshal(tpl)
	hash := sha256.Sum256(templateRaw)
	var miniID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.mini_users(openid) VALUES('initial-receipt-test') RETURNING id`, schema)).Scan(&miniID); err != nil {
		t.Fatal(err)
	}
	_, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.message_external_settings SET config=$1 WHERE id=1`, schema), configRaw)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.message_business_test_verifications(event,status_value,template_fingerprint,delivery_id,verified_by,detail_verified) VALUES('receipt_received','货款到账',$1,1,'test',true)`, schema), hex.EncodeToString(hash[:]))
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.wechat_bindings(app_id,openid,mini_user_id,customer_id,active,subscribed,notifications_enabled,notification_consent_at,notification_events) VALUES('test-app','test-openid',$1,501,true,true,true,now()-interval '1 minute','["receipt_received"]')`, schema), miniID)
	if err != nil {
		t.Fatal(err)
	}

	r := NewRepository(pool, schema)
	sales := salespg.NewRepository(pool, schema)
	shipping := int64(2000)
	override := &domain.Override{ManualAmountCents: &shipping}
	quote, err := r.QuoteFulfillmentFees(ctx, app.FulfillmentFeeQuoteCommand{CustomerID: 501, FulfillmentMode: "sale", FreightProtocol: domain.OrderProtocol, EmployeeAdjustment: true, Province: "云南省", Items: []app.FulfillmentFeeItem{{ProductID: 910, BomSpecID: 7031, BomVariantID: 7032, Qty: 2}}, FreightOverride: override})
	if err != nil {
		t.Fatal(err)
	}
	var partial int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.pay_statuses WHERE name='预付款（付款未完成）' LIMIT 1`, schema)).Scan(&partial); err != nil {
		t.Fatal(err)
	}
	price, deposit := 30.0, 20.0
	product := int64(910)
	cmd := salesapp.SaveOrderCommand{Actor: "test", CustomerID: 501, OrderDate: time.Now(), DocumentDate: time.Now(), PayStatusID: partial, ShipStatusID: 1, ShippingProvince: "云南省", FreightProtocol: domain.OrderProtocol, FreightQuoteToken: quote.Freight.Token, FreightOverride: override, ShippingAmount: 20, PrepaymentAmount: &deposit, PaymentMethod: "现金", ReceiptDate: "2026-11-10", CustomerRequestID: "notice-initial-test", CustomerRequestHash: "initial-hash", Items: []salesapp.OrderItemCommand{{ProductID: &product, BomSpecID: 7031, BomVariantID: 7032, Name: "测试", Units: 2, SalesUnit: "袋", ManualPrice: &price}}}
	// The 20 yuan advance is entirely allocated to goods; the remaining
	// receivable comprises 40 yuan goods and 20 yuan shipping.
	cmd.PaymentGoodsAmount = 20
	first, err := sales.SaveOrder(ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	var actualPrepayment, actualGoods, actualShipping float64
	var actualStatus int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT prepayment_amount::float8,payment_goods_amount::float8,payment_shipping_amount::float8,pay_status_id FROM %s.orders WHERE id=$1`, schema), first.OrderID).Scan(&actualPrepayment, &actualGoods, &actualShipping, &actualStatus); err != nil {
		t.Fatal(err)
	}
	if actualPrepayment != 20 || actualGoods != 20 || actualShipping != 0 || actualStatus != partial {
		t.Fatalf("saved payment data=%v/%v/%v status=%d want=20/20/0 status=%d", actualPrepayment, actualGoods, actualShipping, actualStatus, partial)
	}
	retry, err := sales.SaveOrder(ctx, cmd)
	if err != nil || !retry.Replayed || retry.OrderID != first.OrderID {
		t.Fatalf("replay=%+v err=%v", retry, err)
	}
	var pending int
	var amountValue, totalValue string
	err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*),max(payload->>'received_amount'),max(payload->>'receivable_amount') FROM %s.message_business_events WHERE event_type='receipt_received' AND order_id=$1 AND processed_at IS NULL`, schema), first.OrderID).Scan(&pending, &amountValue, &totalValue)
	if err != nil || pending != 1 || amountValue != "20.00" || totalValue != "80.00" {
		t.Fatalf("outbox=%d amount=%s total=%s err=%v", pending, amountValue, totalValue, err)
	}
	worker := pg.Worker{Repo: repo, AppID: "test-app", Authorize: func(ctx context.Context, v pg.Recipient, n messages.OrderNotice) (bool, error) {
		if v.CustomerID != n.CustomerID {
			return false, nil
		}
		_, err := repo.ReceiptDetail(ctx, v.CustomerID, n.ReceiptID)
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return err == nil, err
	}}
	if err = worker.CaptureBusinessCommitted(ctx); err != nil {
		t.Fatal(err)
	}
	delivery, err := repo.ClaimExternal(ctx, "wechat_service_account", "")
	if err != nil {
		t.Fatal(err)
	}
	if delivery.Notice.ReceiptID <= 0 || delivery.Notice.OrderID != 0 || delivery.Notice.ReceivedAmount != "20.00" || delivery.Notice.ReceivableAmount != "80.00" {
		t.Fatalf("wrong notification: %+v", delivery.Notice)
	}
}
