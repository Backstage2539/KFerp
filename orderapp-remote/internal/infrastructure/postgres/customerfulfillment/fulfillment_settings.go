package customerfulfillment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	freightdomain "orderapp/internal/domain/freight"
	freightpg "orderapp/internal/infrastructure/postgres/freight"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	app "orderapp/internal/application/customerfulfillment"
	postgresinfra "orderapp/internal/infrastructure/postgres"
)

func ensureFulfillmentSettingsSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %[1]s.customer_fulfillment_settings (
 customer_id BIGINT PRIMARY KEY REFERENCES %[1]s.customers(id) ON DELETE CASCADE,
 revision BIGINT NOT NULL DEFAULT 1, modes_json JSONB NOT NULL DEFAULT '[]',
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),updated_by TEXT NOT NULL DEFAULT '');
CREATE TABLE IF NOT EXISTS %[1]s.shipping_fee_templates (
 id BIGSERIAL PRIMARY KEY,name TEXT NOT NULL,active BOOLEAN NOT NULL DEFAULT true,revision BIGINT NOT NULL DEFAULT 1,
 packaging_weight_g BIGINT NOT NULL DEFAULT 0 CHECK(packaging_weight_g>=0),rules_json JSONB NOT NULL DEFAULT '[]',
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),updated_by TEXT NOT NULL DEFAULT '');
CREATE TABLE IF NOT EXISTS %[1]s.fulfillment_order_fee_snapshots (
 order_id BIGINT PRIMARY KEY REFERENCES %[1]s.orders(id),customer_id BIGINT NOT NULL REFERENCES %[1]s.customers(id),
 request_id BIGINT NOT NULL DEFAULT 0,mode TEXT NOT NULL,quote_json JSONB NOT NULL,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),updated_by TEXT NOT NULL DEFAULT '');
CREATE INDEX IF NOT EXISTS fulfillment_order_fee_customer_idx ON %[1]s.fulfillment_order_fee_snapshots(customer_id,order_id);
CREATE OR REPLACE FUNCTION %[1]s.guard_fulfillment_shipping_fee_snapshot() RETURNS trigger LANGUAGE plpgsql AS $shipping_guard$
DECLARE expected_cents BIGINT;
BEGIN
 IF NEW.shipping_amount IS NOT DISTINCT FROM OLD.shipping_amount THEN RETURN NEW; END IF;
 SELECT (quote_json->>'shipping_fee_cents')::bigint INTO expected_cents FROM %[1]s.fulfillment_order_fee_snapshots WHERE order_id=NEW.id;
 IF FOUND AND round(COALESCE(NEW.shipping_amount,0)*100)::bigint<>expected_cents THEN
  RAISE EXCEPTION '请通过代发费用调整修改运费并填写原因' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $shipping_guard$;
DROP TRIGGER IF EXISTS guard_fulfillment_shipping_fee_snapshot ON %[1]s.orders;
CREATE TRIGGER guard_fulfillment_shipping_fee_snapshot BEFORE UPDATE OF shipping_amount ON %[1]s.orders FOR EACH ROW EXECUTE FUNCTION %[1]s.guard_fulfillment_shipping_fee_snapshot();
`, schema))
	return err
}

func defaultFulfillmentSettings(id int64) app.FulfillmentSettings {
	return app.FulfillmentSettings{CustomerID: id, Modes: []app.FulfillmentModeSettings{{Mode: "stock"}, {Mode: "on_demand"}}}
}
func (r *Repository) loadFulfillmentSettings(ctx context.Context, q miniDirectShipQuerier, id int64, lock bool) (app.FulfillmentSettings, error) {
	s := defaultFulfillmentSettings(id)
	if id <= 0 {
		return s, fmt.Errorf("请选择客户")
	}
	var raw []byte
	clause := ""
	if lock {
		clause = " FOR SHARE"
	}
	err := q.QueryRow(ctx, fmt.Sprintf(`SELECT revision,modes_json FROM %s.customer_fulfillment_settings WHERE customer_id=$1%s`, r.schema, clause), id).Scan(&s.Revision, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		err = nil
	} else if err != nil {
		return s, err
	} else if err = json.Unmarshal(raw, &s.Modes); err != nil {
		return s, err
	}
	for i := range s.Modes {
		p, e := freightpg.NewRepository(r.pool, r.schema).LoadPolicy(ctx, q, id, s.Modes[i].Mode, lock)
		if e != nil {
			return s, e
		}
		if p.Revision > 0 {
			s.Modes[i].ShippingTemplateID = p.TemplateID
			s.Modes[i].ShippingFree = p.FreeShipping
		}
	}
	return s, nil
}
func (r *Repository) LoadFulfillmentSettings(ctx context.Context, id int64) (app.FulfillmentSettings, error) {
	return r.loadFulfillmentSettings(ctx, r.pool, id, false)
}
func (r *Repository) ValidateCustomerFulfillmentModeTx(ctx context.Context, tx pgx.Tx, id int64, mode string) (string, error) {
	if mode == "" {
		return "", nil
	}
	s, err := r.loadFulfillmentSettings(ctx, tx, id, true)
	if err != nil {
		return "", err
	}
	for _, m := range s.Modes {
		if m.Mode == mode && m.Enabled {
			if mode == "stock" {
				if err = r.validateFulfillmentWarehouse(ctx, tx, id, m.WarehouseCode); err != nil {
					return "", err
				}
				return m.WarehouseCode, nil
			}
			if mode == "on_demand" {
				return "", nil
			}
		}
	}
	return "", fmt.Errorf("该客户未启用所选代发场景")
}
func (r *Repository) validateFulfillmentWarehouse(ctx context.Context, q miniDirectShipQuerier, id int64, code string) error {
	var ok bool
	err := q.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.warehouses WHERE code=$1 AND customer_id=$2 AND active=true AND kind IN ('finished','customer_finished','customer_processing','customer'))`, r.schema), strings.TrimSpace(code), id).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("库存代发必须绑定该客户启用的成品仓")
	}
	return nil
}
func (r *Repository) SaveFulfillmentSettings(ctx context.Context, c app.SaveFulfillmentSettingsCommand) (app.FulfillmentSettings, error) {
	if err := app.ValidateFulfillmentSettings(c.Settings); err != nil {
		return app.FulfillmentSettings{}, err
	}
	if strings.TrimSpace(c.Actor) == "" {
		return app.FulfillmentSettings{}, fmt.Errorf("操作人不能为空")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.FulfillmentSettings{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, fmt.Sprintf("fulfillment-settings:%d", c.Settings.CustomerID)); err != nil {
		return app.FulfillmentSettings{}, err
	}
	var customerExists bool
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.customers WHERE id=$1 AND active=true)`, r.schema), c.Settings.CustomerID).Scan(&customerExists); err != nil {
		return app.FulfillmentSettings{}, err
	}
	if !customerExists {
		return app.FulfillmentSettings{}, fmt.Errorf("客户不存在或已停用")
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`SELECT customer_id FROM %s.customer_fulfillment_settings WHERE customer_id=$1 FOR UPDATE`, r.schema), c.Settings.CustomerID); err != nil {
		return app.FulfillmentSettings{}, err
	}
	old, err := r.loadFulfillmentSettings(ctx, tx, c.Settings.CustomerID, true)
	if err != nil {
		return old, err
	}
	if old.Revision != c.ExpectedRevision {
		return old, fmt.Errorf("履约设置已变化，请刷新后重试")
	}
	for _, m := range c.Settings.Modes {
		if !m.Enabled {
			continue
		}
		if m.Mode == "stock" {
			if err = r.validateFulfillmentWarehouse(ctx, tx, c.Settings.CustomerID, m.WarehouseCode); err != nil {
				return old, err
			}
		}
		policy, e := freightpg.NewRepository(r.pool, r.schema).LoadPolicy(ctx, tx, c.Settings.CustomerID, m.Mode, true)
		if e != nil {
			return old, e
		}
		if m.ShippingTemplateID == 0 && policy.FreeShipping {
			continue
		}
		tm, e := r.loadShippingFeeTemplate(ctx, tx, m.ShippingTemplateID, true)
		if e != nil || !tm.Active {
			return old, fmt.Errorf("运费模板未配置或已停用")
		}
	}
	for _, m := range c.Settings.Modes {
		if m.ShippingTemplateID > 0 {
			_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_shipping_policies(customer_id,scene,template_id,updated_by)VALUES($1,$2,$3,$4) ON CONFLICT(customer_id,scene) DO UPDATE SET template_id=EXCLUDED.template_id,revision=customer_shipping_policies.revision+1,updated_by=EXCLUDED.updated_by,updated_at=now()`, r.schema), c.Settings.CustomerID, m.Mode, m.ShippingTemplateID, c.Actor)
			if err != nil {
				return old, err
			}
		}
	}
	c.Settings.Revision = old.Revision + 1
	raw, _ := json.Marshal(c.Settings.Modes)
	_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_fulfillment_settings(customer_id,revision,modes_json,updated_by) VALUES($1,$2,$3,$4) ON CONFLICT(customer_id) DO UPDATE SET revision=EXCLUDED.revision,modes_json=EXCLUDED.modes_json,updated_by=EXCLUDED.updated_by,updated_at=now()`, r.schema), c.Settings.CustomerID, c.Settings.Revision, raw, c.Actor)
	if err != nil {
		return old, err
	}
	if err = r.auditFulfillmentConfig(ctx, tx, c.Actor, "customer_fulfillment_settings", c.Settings.CustomerID, "save", old, c.Settings); err != nil {
		return old, err
	}
	if err = tx.Commit(ctx); err != nil {
		return old, err
	}
	return c.Settings, nil
}
func (r *Repository) ListShippingFeeTemplates(ctx context.Context) ([]app.ShippingFeeTemplate, error) {
	return freightpg.NewRepository(r.pool, r.schema).Templates(ctx)
}
func (r *Repository) loadShippingFeeTemplate(ctx context.Context, q miniDirectShipQuerier, id int64, lock bool) (app.ShippingFeeTemplate, error) {
	return freightpg.NewRepository(r.pool, r.schema).LoadTemplate(ctx, q, id, lock)
}
func (r *Repository) SaveShippingFeeTemplate(ctx context.Context, c app.SaveShippingFeeTemplateCommand) (app.ShippingFeeTemplate, error) {
	return freightpg.NewRepository(r.pool, r.schema).SaveTemplate(ctx, c.Template, c.ExpectedRevision, c.Actor)
}
func (r *Repository) auditFulfillmentConfig(ctx context.Context, tx pgx.Tx, actor, entity string, id int64, action string, old, new any) error {
	before, _ := json.Marshal(old)
	after, _ := json.Marshal(new)
	return postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, entity, &id, action, postgresinfra.StrPtr("configuration"), postgresinfra.StrPtr(string(before)), postgresinfra.StrPtr(string(after)), postgresinfra.AuditMeta{"requirement": "PR-703"})
}

func (r *Repository) QuoteFulfillmentFees(ctx context.Context, c app.FulfillmentFeeQuoteCommand) (app.FulfillmentFeeQuote, error) {
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return app.FulfillmentFeeQuote{}, e
	}
	defer tx.Rollback(ctx)
	q, e := r.QuoteFulfillmentFeesTx(ctx, tx, c)
	if e != nil {
		return q, e
	}
	e = tx.Commit(ctx)
	return q, e
}
func (r *Repository) QuoteFulfillmentFeesTx(ctx context.Context, tx pgx.Tx, c app.FulfillmentFeeQuoteCommand) (app.FulfillmentFeeQuote, error) {
	var out app.FulfillmentFeeQuote
	if _, e := r.ValidateCustomerFulfillmentModeTx(ctx, tx, c.CustomerID, strings.TrimSuffix(c.FulfillmentMode, "sale")); e != nil {
		return out, e
	}
	s, e := r.loadFulfillmentSettings(ctx, tx, c.CustomerID, true)
	if e != nil {
		return out, e
	}
	var mode app.FulfillmentModeSettings
	for _, m := range s.Modes {
		if m.Mode == c.FulfillmentMode {
			mode = m
		}
	}
	if c.FulfillmentMode != "sale" && (!mode.Enabled || !mode.FeeConfigured) {
		return out, fmt.Errorf("请配置当前场景的代发服务费")
	}
	qty, grams := int64(0), int64(0)
	policy, e := freightpg.NewRepository(r.pool, r.schema).LoadPolicy(ctx, tx, c.CustomerID, c.FulfillmentMode, true)
	if e != nil {
		return out, e
	}
	policy, e = freightdomain.ApplyOverride(policy, c.FreightOverride, c.EmployeeAdjustment)
	if e != nil {
		return out, e
	}
	for _, item := range c.Items {
		if item.ProductID <= 0 || item.Qty <= 0 || item.Qty > 1000000 {
			return out, fmt.Errorf("商品数量无效")
		}
		g, e := r.fulfillmentItemNetWeightG(ctx, tx, c.CustomerID, item)
		if e != nil && !policy.FreeShipping {
			return out, e
		}
		if g <= 0 && policy.FreeShipping {
			grams = -1000000001
		} else if grams >= 0 {
			grams += g * item.Qty
		}
		qty += item.Qty
	}
	if grams < 0 {
		grams = 0
	}
	f, e := freightpg.NewRepository(r.pool, r.schema).QuoteWithOverrideTx(ctx, tx, c.CustomerID, c.FulfillmentMode, freightdomain.Destination{Province: c.Province, City: c.City, District: c.District}, grams, c.FreightOverride, c.EmployeeAdjustment)
	if e != nil {
		return out, e
	}
	if c.ServiceOverrideCents != nil || c.ShippingOverrideCents != nil {
		if !c.EmployeeAdjustment || strings.TrimSpace(c.AdjustmentReason) == "" {
			return out, fmt.Errorf("员工调整费用必须填写原因")
		}
	}
	if qty <= 0 || qty > 1000000 || mode.PerPackageFeeCents > freightdomain.MaxCents/qty {
		return out, fmt.Errorf("服务费超出范围")
	}
	service := mode.FixedFeeCents + mode.PerPackageFeeCents*qty
	out = app.FulfillmentFeeQuote{CustomerID: c.CustomerID, FulfillmentMode: c.FulfillmentMode, SettingsRevision: s.Revision, ShippingTemplateID: f.TemplateID, ShippingTemplateRevision: f.TemplateRevision, Province: c.Province, City: c.City, District: c.District, PackageQty: qty, NetWeightG: grams, PackagingWeightG: f.PackagingWeightG, ChargeableWeightG: f.Estimate.WeightG, FixedFeeCents: mode.FixedFeeCents, PerPackageFeeCents: mode.PerPackageFeeCents, CalculatedServiceFeeCents: service, ServiceFeeCents: service, CalculatedShippingFeeCents: f.EstimatedTotalCents, Freight: &f}
	if c.ServiceOverrideCents != nil {
		out.ServiceFeeCents = *c.ServiceOverrideCents
	}
	if out.ServiceFeeCents < 0 || out.ServiceFeeCents > freightdomain.MaxCents {
		return out, fmt.Errorf("服务费金额无效")
	}
	if c.ShippingOverrideCents != nil {
		return out, fmt.Errorf("预估运费请通过运费政策调整；实际运费通过账单出账")
	}
	out.AdjustmentReason = strings.TrimSpace(c.AdjustmentReason)
	out.FeeQuoteToken = app.FulfillmentFeeQuoteToken(out)
	return out, nil
}
func (r *Repository) fulfillmentItemNetWeightG(ctx context.Context, q miniDirectShipQuerier, id int64, i app.FulfillmentFeeItem) (int64, error) {
	return freightpg.NewRepository(r.pool, r.schema).ItemNetWeightG(ctx, q, id, freightpg.WeightItem{ProductID: i.ProductID, BomSpecID: i.BomSpecID, BomVariantID: i.BomVariantID, Qty: i.Qty})
}
func (r *Repository) SaveFulfillmentOrderFeesTx(ctx context.Context, tx pgx.Tx, customerID, orderID, requestID int64, mode string, q app.FulfillmentFeeQuote, actor string) error {
	if mode == "" {
		return nil
	}
	if q.CustomerID != customerID || q.FulfillmentMode != mode || q.FeeQuoteToken == "" || q.FeeQuoteToken != app.FulfillmentFeeQuoteToken(q) {
		return fmt.Errorf("代发费用报价无效，请重新预览")
	}
	var orderCustomer int64
	var void bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT customer_id,COALESCE(is_void,false) FROM %s.orders WHERE id=$1 FOR UPDATE`, r.schema), orderID).Scan(&orderCustomer, &void); err != nil {
		return err
	}
	if orderCustomer != customerID || void {
		return fmt.Errorf("订单不存在、已作废或不属于当前客户")
	}
	var existing []byte
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT quote_json FROM %s.fulfillment_order_fee_snapshots WHERE order_id=$1`, r.schema), orderID).Scan(&existing)
	if err == nil {
		var old app.FulfillmentFeeQuote
		_ = json.Unmarshal(existing, &old)
		if old.FeeQuoteToken == q.FeeQuoteToken {
			return nil
		}
		return fmt.Errorf("订单已计费，请通过费用调整并填写原因")
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if q.Freight != nil {
		if err = freightpg.NewRepository(r.pool, r.schema).SaveSnapshotTx(ctx, tx, orderID, *q.Freight, actor); err != nil {
			return err
		}
	}
	raw, _ := json.Marshal(q)
	if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.fulfillment_order_fee_snapshots(order_id,customer_id,request_id,mode,quote_json,updated_by) VALUES($1,$2,$3,$4,$5,$6)`, r.schema), orderID, customerID, requestID, mode, raw, actor); err != nil {
		return err
	}
	if mode != "sale" {
		if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_fee_items(customer_id,source_type,source_id,fee_type,amount,note) VALUES($1,'order',$2,'direct_ship_service',$3,'代发服务费')`, r.schema), customerID, orderID, float64(q.ServiceFeeCents)/100); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET shipping_amount=$2,grand_total=COALESCE(total_amount,0)-COALESCE(discount_amount,0)+$2 WHERE id=$1`, r.schema), orderID, float64(q.ShippingFeeCents)/100); err != nil {
		return err
	}
	return r.auditFulfillmentConfig(ctx, tx, actor, "fulfillment_order_fees", orderID, "calculate", nil, q)
}
func (r *Repository) AdjustFulfillmentOrderFees(ctx context.Context, c app.AdjustFulfillmentOrderFeesCommand) (app.FulfillmentFeeQuote, error) {
	var q app.FulfillmentFeeQuote
	if c.CustomerID <= 0 || c.OrderID <= 0 || c.ExpectedQuoteToken == "" || strings.TrimSpace(c.Actor) == "" || strings.TrimSpace(c.Reason) == "" || c.ServiceFeeCents < 0 || c.ShippingFeeCents < 0 || c.ServiceFeeCents > 999999999999 || c.ShippingFeeCents > 999999999999 {
		return q, fmt.Errorf("请填写有效费用、当前报价版本和调整原因")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return q, err
	}
	defer tx.Rollback(ctx)
	var raw []byte
	var void bool
	var settlementID int64
	var status string
	// Every fee writer acquires order, snapshot, then fee locks in this order.
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(is_void,false) FROM %s.orders WHERE id=$1 AND customer_id=$2 FOR UPDATE`, r.schema), c.OrderID, c.CustomerID).Scan(&void); err != nil {
		return q, fmt.Errorf("订单费用不存在或不属于当前客户")
	}
	if void {
		return q, fmt.Errorf("订单已作废或费用已变化，请刷新")
	}
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT quote_json FROM %s.fulfillment_order_fee_snapshots WHERE order_id=$1 AND customer_id=$2 FOR UPDATE`, r.schema), c.OrderID, c.CustomerID).Scan(&raw); err != nil {
		return q, fmt.Errorf("订单费用不存在或不属于当前客户")
	}
	if err = json.Unmarshal(raw, &q); err != nil {
		return q, err
	}
	if void || q.FeeQuoteToken != c.ExpectedQuoteToken {
		return q, fmt.Errorf("订单已作废或费用已变化，请刷新")
	}
	old := q
	if q.Freight != nil && c.ShippingFeeCents != 0 {
		return q, fmt.Errorf("预估运费不计入订单，请通过实际运费账单调整")
	}
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT settlement_batch_id,status FROM %s.customer_fee_items WHERE customer_id=$1 AND source_type='order' AND source_id=$2 AND fee_type='direct_ship_service' FOR UPDATE`, r.schema), c.CustomerID, c.OrderID).Scan(&settlementID, &status); err != nil {
		return q, err
	}
	if settlementID > 0 || status != "unsettled" {
		return q, fmt.Errorf("费用已结算，请先处理结算冲销")
	}
	q.ServiceFeeCents = c.ServiceFeeCents
	q.ShippingFeeCents = c.ShippingFeeCents
	q.AdjustmentReason = strings.TrimSpace(c.Reason)
	q.FeeQuoteToken = app.FulfillmentFeeQuoteToken(q)
	raw, _ = json.Marshal(q)
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.fulfillment_order_fee_snapshots SET quote_json=$3,updated_by=$4,updated_at=now() WHERE order_id=$1 AND customer_id=$2`, r.schema), c.OrderID, c.CustomerID, raw, c.Actor); err != nil {
		return q, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_fee_items SET amount=$3,note=$4 WHERE customer_id=$1 AND source_type='order' AND source_id=$2 AND fee_type='direct_ship_service'`, r.schema), c.CustomerID, c.OrderID, float64(c.ServiceFeeCents)/100, "代发服务费 · "+c.Reason); err != nil {
		return q, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET shipping_amount=$2,grand_total=COALESCE(total_amount,0)-COALESCE(discount_amount,0)+$2 WHERE id=$1`, r.schema), c.OrderID, float64(c.ShippingFeeCents)/100); err != nil {
		return q, err
	}
	if err = r.auditFulfillmentConfig(ctx, tx, c.Actor, "fulfillment_order_fees", c.OrderID, "adjust", old, q); err != nil {
		return q, err
	}
	if err = tx.Commit(ctx); err != nil {
		return q, err
	}
	return q, nil
}

func (r *Repository) GetFulfillmentOrderFees(ctx context.Context, customerID, orderID int64) (app.FulfillmentFeeQuote, error) {
	var q app.FulfillmentFeeQuote
	var raw []byte
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT quote_json FROM %s.fulfillment_order_fee_snapshots WHERE customer_id=$1 AND order_id=$2`, r.schema), customerID, orderID).Scan(&raw)
	if err != nil {
		return q, fmt.Errorf("该订单暂无代发场景费用快照")
	}
	err = json.Unmarshal(raw, &q)
	return q, err
}

func (r *Repository) ReverseFulfillmentOrderFeesTx(ctx context.Context, tx pgx.Tx, customerID, orderID int64, actor string) error {
	var lockedOrderID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.orders WHERE id=$1 AND customer_id=$2 FOR UPDATE`, r.schema), orderID, customerID).Scan(&lockedOrderID); err != nil {
		return fmt.Errorf("订单不存在或不属于当前客户")
	}
	var raw []byte
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT quote_json FROM %s.fulfillment_order_fee_snapshots WHERE order_id=$1 AND customer_id=$2 FOR UPDATE`, r.schema), orderID, customerID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var old app.FulfillmentFeeQuote
	if err = json.Unmarshal(raw, &old); err != nil {
		return err
	}
	var settlementID int64
	var status string
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT settlement_batch_id,status FROM %s.customer_fee_items WHERE customer_id=$1 AND source_type='order' AND source_id=$2 AND fee_type='direct_ship_service' FOR UPDATE`, r.schema), customerID, orderID).Scan(&settlementID, &status)
	if err != nil {
		return err
	}
	if status == "reversed" {
		return nil
	}
	if settlementID > 0 || status != "unsettled" {
		return fmt.Errorf("代发费用已结算，请先冲销结算后取消订单")
	}
	q := old
	q.ServiceFeeCents = 0
	q.ShippingFeeCents = 0
	q.AdjustmentReason = "订单取消，未结费用撤销"
	q.FeeQuoteToken = app.FulfillmentFeeQuoteToken(q)
	raw, _ = json.Marshal(q)
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_fee_items SET amount=0,status='reversed',note='订单取消，代发服务费已撤销' WHERE customer_id=$1 AND source_type='order' AND source_id=$2 AND fee_type='direct_ship_service'`, r.schema), customerID, orderID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.fulfillment_order_fee_snapshots SET quote_json=$3,updated_at=now(),updated_by=$4 WHERE customer_id=$1 AND order_id=$2`, r.schema), customerID, orderID, raw, actor); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET shipping_amount=0,grand_total=COALESCE(total_amount,0)-COALESCE(discount_amount,0) WHERE id=$1 AND customer_id=$2`, r.schema), orderID, customerID); err != nil {
		return err
	}
	return r.auditFulfillmentConfig(ctx, tx, actor, "fulfillment_order_fees", orderID, "reverse", old, q)
}
