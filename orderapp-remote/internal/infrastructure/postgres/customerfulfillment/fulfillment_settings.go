package customerfulfillment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	app "orderapp/internal/application/customerfulfillment"
	conversion "orderapp/internal/domain/specconversion"
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
		return s, nil
	}
	if err != nil {
		return s, err
	}
	err = json.Unmarshal(raw, &s.Modes)
	return s, err
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
		tm, e := r.loadShippingFeeTemplate(ctx, tx, m.ShippingTemplateID, true)
		if e != nil || !tm.Active {
			return old, fmt.Errorf("运费模板未配置或已停用")
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
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT id,name,active,revision,packaging_weight_g,rules_json FROM %s.shipping_fee_templates ORDER BY active DESC,name,id`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []app.ShippingFeeTemplate{}
	for rows.Next() {
		var t app.ShippingFeeTemplate
		var raw []byte
		if err = rows.Scan(&t.ID, &t.Name, &t.Active, &t.Revision, &t.PackagingWeightG, &raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &t.Rules); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
func (r *Repository) loadShippingFeeTemplate(ctx context.Context, q miniDirectShipQuerier, id int64, lock bool) (app.ShippingFeeTemplate, error) {
	var t app.ShippingFeeTemplate
	var raw []byte
	clause := ""
	if lock {
		clause = " FOR SHARE"
	}
	err := q.QueryRow(ctx, fmt.Sprintf(`SELECT id,name,active,revision,packaging_weight_g,rules_json FROM %s.shipping_fee_templates WHERE id=$1%s`, r.schema, clause), id).Scan(&t.ID, &t.Name, &t.Active, &t.Revision, &t.PackagingWeightG, &raw)
	if err != nil {
		return t, err
	}
	err = json.Unmarshal(raw, &t.Rules)
	return t, err
}
func (r *Repository) SaveShippingFeeTemplate(ctx context.Context, c app.SaveShippingFeeTemplateCommand) (app.ShippingFeeTemplate, error) {
	if err := app.ValidateShippingFeeTemplate(c.Template); err != nil {
		return app.ShippingFeeTemplate{}, err
	}
	if strings.TrimSpace(c.Actor) == "" {
		return app.ShippingFeeTemplate{}, fmt.Errorf("操作人不能为空")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.ShippingFeeTemplate{}, err
	}
	defer tx.Rollback(ctx)
	old := app.ShippingFeeTemplate{}
	t := c.Template
	raw, _ := json.Marshal(t.Rules)
	if t.ID > 0 {
		var oldRaw []byte
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT id,name,active,revision,packaging_weight_g,rules_json FROM %s.shipping_fee_templates WHERE id=$1 FOR UPDATE`, r.schema), t.ID).Scan(&old.ID, &old.Name, &old.Active, &old.Revision, &old.PackagingWeightG, &oldRaw)
		if err != nil {
			return t, fmt.Errorf("运费模板不存在")
		}
		_ = json.Unmarshal(oldRaw, &old.Rules)
		if old.Revision != c.ExpectedRevision {
			return t, fmt.Errorf("运费模板已变化，请刷新后重试")
		}
		err = tx.QueryRow(ctx, fmt.Sprintf(`UPDATE %s.shipping_fee_templates SET name=$2,active=$3,revision=revision+1,packaging_weight_g=$4,rules_json=$5,updated_by=$6,updated_at=now() WHERE id=$1 RETURNING revision`, r.schema), t.ID, t.Name, t.Active, t.PackagingWeightG, raw, c.Actor).Scan(&t.Revision)
	} else {
		if c.ExpectedRevision != 0 {
			return t, fmt.Errorf("运费模板版本无效")
		}
		err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.shipping_fee_templates(name,active,packaging_weight_g,rules_json,updated_by) VALUES($1,$2,$3,$4,$5) RETURNING id,revision`, r.schema), strings.TrimSpace(t.Name), t.Active, t.PackagingWeightG, raw, c.Actor).Scan(&t.ID, &t.Revision)
	}
	if err != nil {
		return t, err
	}
	if err = r.auditFulfillmentConfig(ctx, tx, c.Actor, "shipping_fee_template", t.ID, "save", old, t); err != nil {
		return t, err
	}
	if err = tx.Commit(ctx); err != nil {
		return t, err
	}
	return t, nil
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
	var q app.FulfillmentFeeQuote
	if _, err := r.ValidateCustomerFulfillmentModeTx(ctx, tx, c.CustomerID, c.FulfillmentMode); err != nil {
		return q, err
	}
	s, err := r.loadFulfillmentSettings(ctx, tx, c.CustomerID, true)
	if err != nil {
		return q, err
	}
	var templateID int64
	for _, m := range s.Modes {
		if m.Mode == c.FulfillmentMode {
			templateID = m.ShippingTemplateID
		}
	}
	tm, err := r.loadShippingFeeTemplate(ctx, tx, templateID, true)
	if err != nil {
		return q, fmt.Errorf("该场景尚未配置有效运费模板")
	}
	qty, grams := int64(0), int64(0)
	for _, item := range c.Items {
		if item.ProductID <= 0 || item.Qty <= 0 || item.Qty > 1000000 {
			return q, fmt.Errorf("商品数量无效")
		}
		unitG, e := r.fulfillmentItemNetWeightG(ctx, tx, c.CustomerID, item)
		if e != nil {
			return q, e
		}
		qty += item.Qty
		if unitG <= 0 || unitG > 1000000000/item.Qty {
			return q, fmt.Errorf("商品净重无效或超出可处理范围")
		}
		grams += unitG * item.Qty
	}
	return app.CalculateFulfillmentFees(s, tm, c, qty, grams)
}
func weightInGrams(qty float64, unit string) int64 {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "g", "克":
	case "kg", "千克", "公斤":
		qty *= 1000
	case "lb", "lbs", "磅":
		qty *= 454
	default:
		return 0
	}
	if qty <= 0 || math.IsNaN(qty) || math.IsInf(qty, 0) || qty > 1000000000 {
		return 0
	}
	return int64(math.Ceil(qty))
}
func (r *Repository) fulfillmentItemNetWeightG(ctx context.Context, q miniDirectShipQuerier, customerID int64, item app.FulfillmentFeeItem) (int64, error) {
	if item.BomSpecID > 0 {
		if grams, err := r.structuredBOMSpecWeightG(ctx, q, customerID, item); err != nil || grams > 0 {
			return grams, err
		}
		// A BOM name is presentation, never a source of shipping weight. Use the frozen published specification.
		rows, err := q.Query(ctx, fmt.Sprintf(`SELECT p.content_json FROM %[1]s.customer_order_price_table_bindings b JOIN %[1]s.bean_list_publications p ON p.id=b.publication_id WHERE b.customer_id=$1 AND b.usage_code='direct_ship' AND p.status='published' AND p.deleted_at IS NULL`, r.schema), customerID)
		if err != nil {
			return 0, err
		}
		defer rows.Close()
		found := int64(0)
		for rows.Next() {
			var raw []byte
			if err = rows.Scan(&raw); err != nil {
				return 0, err
			}
			var content struct {
				Rows []map[string]any `json:"price_rows"`
			}
			if err = json.Unmarshal(raw, &content); err != nil {
				return 0, err
			}
			for _, row := range content.Rows {
				spec, _ := row["effective_sales_spec"].(map[string]any)
				id := payloadInt64(row, "bom_spec_id")
				if id == 0 {
					id = payloadInt64(spec, "bom_spec_id")
				}
				product := payloadInt64(row, "parent_product_id")
				if product == 0 {
					product = payloadInt64(row, "product_id")
				}
				if product == 0 {
					product = payloadInt64(spec, "parent_product_id")
					if product == 0 {
						product = payloadInt64(spec, "product_id")
					}
				}
				if id != item.BomSpecID || product != item.ProductID {
					continue
				}
				n := floatFromAny(spec["net_content_qty"])
				u := payloadString(spec, "net_content_unit")
				if n <= 0 {
					n = floatFromAny(row["net_content_qty"])
					u = payloadString(row, "net_content_unit")
				}
				if n <= 0 {
					sku, _ := row["sku_snapshot"].(map[string]any)
					n = floatFromAny(sku["net_content_qty"])
					u = payloadString(sku, "net_content_unit")
				}
				g := weightInGrams(n, u)
				if g > 0 {
					if found > 0 && found != g {
						return 0, fmt.Errorf("已发布规格净重存在冲突，请核对价格表")
					}
					found = g
				}
			}
		}
		if err = rows.Err(); err != nil {
			return 0, err
		}
		if found > 0 {
			return found, nil
		}
		return 0, fmt.Errorf("商品规格未配置已发布净重，无法计算运费")
	}
	var qty float64
	var unit string
	err := q.QueryRow(ctx, fmt.Sprintf(`SELECT net_content_qty::float8,net_content_unit FROM %s.products WHERE id=$1 AND active=true AND COALESCE(customer_id,0) IN (0,$2)`, r.schema), item.ProductID, customerID).Scan(&qty, &unit)
	if err != nil {
		return 0, fmt.Errorf("商品不存在或不可用")
	}
	g := weightInGrams(qty, unit)
	if g <= 0 {
		return 0, fmt.Errorf("商品未配置净重，无法计算运费")
	}
	return g, nil
}

func (r *Repository) structuredBOMSpecWeightG(ctx context.Context, q miniDirectShipQuerier, customerID int64, item app.FulfillmentFeeItem) (int64, error) {
	var identity conversion.Identity
	var currentVariant int64
	err := q.QueryRow(ctx, fmt.Sprintf(`SELECT spec.id,COALESCE(spec.source_spec_template_id,0),COALESCE(spec.source_spec_template_key,''),variant.inventory_unit,variant.id
 FROM %[1]s.production_bom_output_bindings binding JOIN %[1]s.production_bom_versions version ON version.id=binding.bom_version_id AND version.status='published'
 JOIN %[1]s.production_bom_specs spec ON spec.bom_id=binding.bom_id AND spec.id=$2
 JOIN %[1]s.production_bom_version_variants variant ON variant.version_id=version.id AND variant.bom_spec_id=spec.id
 WHERE binding.output_type='product' AND binding.output_id=$1 AND binding.is_default=true AND ($3::bigint=0 OR variant.id=$3)`, r.schema), item.ProductID, item.BomSpecID, item.BomVariantID).Scan(&identity.SpecID, &identity.TemplateID, &identity.TemplateKey, &identity.InventoryUnit, &currentVariant)
	if err != nil {
		return 0, fmt.Errorf("商品规格不存在或未发布，无法计算运费")
	}
	if grams := weightInGrams(1, identity.InventoryUnit); grams > 0 {
		return grams, nil
	}
	if relationExists(ctx, q, r.schema+".spec_conversion_groups") {
		rows, e := q.Query(ctx, fmt.Sprintf(`SELECT id,name,base_unit,revision,members_json FROM %s.spec_conversion_groups WHERE active=true ORDER BY id`, r.schema))
		if e != nil {
			return 0, e
		}
		found := int64(0)
		for rows.Next() {
			var g conversion.Group
			var raw []byte
			if e = rows.Scan(&g.ID, &g.Name, &g.BaseUnit, &g.Revision, &raw); e != nil {
				rows.Close()
				return 0, e
			}
			if e = json.Unmarshal(raw, &g.Members); e != nil {
				rows.Close()
				return 0, e
			}
			member, e := g.Match(identity)
			if e != nil {
				continue
			}
			qty, e := strconv.ParseFloat(member.BaseQuantity, 64)
			if e != nil {
				rows.Close()
				return 0, fmt.Errorf("规格重量换算配置无效")
			}
			grams := weightInGrams(qty, g.BaseUnit)
			if grams > 0 {
				if found > 0 && found != grams {
					rows.Close()
					return 0, fmt.Errorf("规格存在冲突的净重换算关系")
				}
				found = grams
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return 0, err
		}
		if found > 0 {
			return found, nil
		}
	}
	// A retained sale freezes the exact specification weight independently of a current dropship price list.
	rows, err := q.Query(ctx, fmt.Sprintf(`SELECT item.price_source_json FROM %[1]s.order_items item JOIN %[1]s.orders sale ON sale.id=item.order_id WHERE sale.customer_id=$1 AND COALESCE(sale.is_void,false)=false AND COALESCE(to_jsonb(sale)->>'delivery_mode','')='customer_stock' AND item.product_id=$2 AND item.bom_spec_id=$3 ORDER BY sale.id DESC,item.id DESC`, r.schema), customerID, item.ProductID, item.BomSpecID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return 0, err
		}
		var source map[string]any
		if json.Unmarshal(raw, &source) != nil {
			continue
		}
		spec, _ := source["effective_sales_spec"].(map[string]any)
		if grams := weightInGrams(floatFromAny(spec["net_content_qty"]), payloadString(spec, "net_content_unit")); grams > 0 {
			return grams, nil
		}
		frozen, _ := source["production_quantity_snapshot"].(map[string]any)
		if grams := weightInGrams(floatFromAny(frozen["inventory_qty_per_sales_unit"]), payloadString(frozen, "inventory_unit")); grams > 0 {
			return grams, nil
		}
	}
	return 0, rows.Err()
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
	raw, _ := json.Marshal(q)
	if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.fulfillment_order_fee_snapshots(order_id,customer_id,request_id,mode,quote_json,updated_by) VALUES($1,$2,$3,$4,$5,$6)`, r.schema), orderID, customerID, requestID, mode, raw, actor); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_fee_items(customer_id,source_type,source_id,fee_type,amount,note) VALUES($1,'order',$2,'direct_ship_service',$3,'代发服务费')`, r.schema), customerID, orderID, float64(q.ServiceFeeCents)/100); err != nil {
		return err
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
