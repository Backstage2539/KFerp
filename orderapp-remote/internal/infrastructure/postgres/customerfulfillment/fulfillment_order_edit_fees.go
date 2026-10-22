package customerfulfillment

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"

	app "orderapp/internal/application/customerfulfillment"
	domain "orderapp/internal/domain/freight"
	freightpg "orderapp/internal/infrastructure/postgres/freight"

	"github.com/jackc/pgx/v5"
)

// The caller takes LockOrderFreight before locking the order. Both preview and
// save use the original policy/settings; opening an order never upgrades prices.
func (r *Repository) loadFulfillmentOrderEditFeesTx(ctx context.Context, tx pgx.Tx, customerID, orderID int64) (app.FulfillmentFeeQuote, error) {
	var old app.FulfillmentFeeQuote
	var raw []byte
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT quote_json FROM %s.fulfillment_order_fee_snapshots WHERE order_id=$1 AND customer_id=$2 FOR UPDATE`, r.schema), orderID, customerID).Scan(&raw); err != nil {
		return old, fmt.Errorf("原订单代发费用快照不可用，请核对后重试")
	}
	if err := json.Unmarshal(raw, &old); err != nil {
		return old, err
	}
	if old.CustomerID != customerID || (old.FulfillmentMode != "stock" && old.FulfillmentMode != "on_demand") || old.Freight == nil || old.Freight.Protocol != domain.Protocol || old.Freight.CurrentReceivableCents != 0 || old.ShippingFeeCents != 0 || old.FeeQuoteToken != app.FulfillmentFeeQuoteToken(old) {
		return old, fmt.Errorf("原订单代发费用快照无效，不能自动覆盖")
	}
	var freightRaw []byte
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT snapshot_json FROM %s.order_freight_snapshots WHERE order_id=$1 AND customer_id=$2 FOR UPDATE`, r.schema), orderID, customerID).Scan(&freightRaw); err != nil {
		return old, fmt.Errorf("原订单运费快照不可用，请核对后重试")
	}
	var freight domain.Snapshot
	if err := json.Unmarshal(freightRaw, &freight); err != nil {
		return old, err
	}
	if freight.Token != old.Freight.Token || freight.Token != domain.Token(freight) {
		return old, fmt.Errorf("原订单费用与运费版本不一致，请重新打开")
	}
	return old, nil
}

func fulfillmentEditItemKey(product, spec, variant int64) string {
	return fmt.Sprintf("%d:%d:%d", product, spec, variant)
}

func fulfillmentEditSameItems(items []app.FulfillmentFeeItem, basis []domain.WeightBasis) bool {
	a, b := map[string]int64{}, map[string]int64{}
	for _, i := range items {
		a[fulfillmentEditItemKey(i.ProductID, i.BomSpecID, i.BomVariantID)] += i.Qty
	}
	for _, i := range basis {
		b[fulfillmentEditItemKey(i.ProductID, i.BomSpecID, i.BomVariantID)] += i.Qty
	}
	return len(a) > 0 && reflect.DeepEqual(a, b)
}

func fulfillmentEditSameDestination(a, b domain.Destination) bool {
	if normalized, err := domain.ResolveDestination(a); err == nil {
		a = normalized
	}
	if normalized, err := domain.ResolveDestination(b); err == nil {
		b = normalized
	}
	return a == b
}

func (r *Repository) previewFulfillmentOrderEditFeesTx(ctx context.Context, tx pgx.Tx, c app.FulfillmentFeeQuoteCommand) (app.FulfillmentFeeQuote, error) {
	old, err := r.loadFulfillmentOrderEditFeesTx(ctx, tx, c.CustomerID, c.OrderID)
	if err != nil {
		return old, err
	}
	if c.FulfillmentMode != old.FulfillmentMode || c.ExpectedFreightToken != old.Freight.Token {
		return old, fmt.Errorf("代发场景或运费版本已变化，请重新打开")
	}
	if c.ShippingOverrideCents != nil || c.FreightOverride != nil {
		return old, fmt.Errorf("编辑沿用原运费模板及政策，实际运费通过账单出账")
	}
	qty := int64(0)
	for _, i := range c.Items {
		if i.ProductID <= 0 || i.Qty <= 0 || i.Qty > 1000000 || qty > 1000000-i.Qty {
			return old, fmt.Errorf("商品数量无效")
		}
		qty += i.Qty
	}
	if qty <= 0 || old.FixedFeeCents < 0 || old.PerPackageFeeCents < 0 || old.FixedFeeCents > domain.MaxCents || old.PerPackageFeeCents > (domain.MaxCents-old.FixedFeeCents)/qty {
		return old, fmt.Errorf("代发服务费计费设置或数量无效")
	}
	destination := domain.Destination{Province: strings.TrimSpace(c.Province), City: strings.TrimSpace(c.City), District: strings.TrimSpace(c.District)}
	freightChanged := !fulfillmentEditSameItems(c.Items, old.Freight.ItemWeights) || !fulfillmentEditSameDestination(destination, old.Freight.Destination)
	q := old
	if freightChanged {
		fr := freightpg.NewRepository(r.pool, r.schema)
		if err = fr.EditableTx(ctx, tx, c.OrderID); err != nil {
			return old, err
		}
		var raw []byte
		var template domain.Template
		if old.Freight.TemplateID > 0 {
			if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT snapshot_json FROM %s.shipping_fee_template_versions WHERE template_id=$1 AND revision=$2`, r.schema), old.Freight.TemplateID, old.Freight.TemplateRevision).Scan(&raw); err != nil {
				return old, fmt.Errorf("原订单运费模板版本不可用，不能改用最新模板")
			}
			if err = json.Unmarshal(raw, &template); err != nil {
				return old, err
			}
		}
		grams, basis, err := r.fulfillmentOrderEditWeightTotals(ctx, tx, c.CustomerID, c.Items, old.Freight)
		if err != nil {
			return old, err
		}
		freight, err := domain.Quote(old.Freight.Policy, template, destination, grams)
		if err != nil {
			return old, err
		}
		freight.AdjustmentReason = old.Freight.AdjustmentReason
		freight = freightpg.AttachWeightBasis(freight, basis)
		q.Freight = &freight
		q.Province, q.City, q.District = destination.Province, destination.City, destination.District
		q.PackageQty, q.NetWeightG, q.PackagingWeightG, q.ChargeableWeightG = qty, grams, freight.PackagingWeightG, freight.Estimate.WeightG
		q.CalculatedShippingFeeCents = freight.EstimatedTotalCents
	}
	q.CalculatedServiceFeeCents = old.FixedFeeCents + old.PerPackageFeeCents*qty
	if qty != old.PackageQty {
		q.ServiceFeeCents = q.CalculatedServiceFeeCents
		q.AdjustmentReason = ""
	}
	if c.ServiceOverrideCents != nil && *c.ServiceOverrideCents != q.ServiceFeeCents {
		if !c.EmployeeAdjustment || strings.TrimSpace(c.AdjustmentReason) == "" || *c.ServiceOverrideCents < 0 || *c.ServiceOverrideCents > domain.MaxCents {
			return old, fmt.Errorf("员工调整服务费必须填写有效金额和原因")
		}
		q.ServiceFeeCents, q.AdjustmentReason = *c.ServiceOverrideCents, strings.TrimSpace(c.AdjustmentReason)
	}
	if q.ServiceFeeCents != old.ServiceFeeCents {
		if err = r.fulfillmentEditServiceFeeWritable(ctx, tx, c.CustomerID, c.OrderID, old.ServiceFeeCents); err != nil {
			return old, err
		}
	}
	q.FeeQuoteToken = app.FulfillmentFeeQuoteToken(q)
	return q, nil
}

// Existing lines use their frozen logistics weight, including when quantity
// changes. Only newly added product/spec/variant identities consult current BOMs.
func (r *Repository) fulfillmentOrderEditWeightTotals(ctx context.Context, tx pgx.Tx, customerID int64, items []app.FulfillmentFeeItem, old *domain.Snapshot) (int64, []domain.WeightBasis, error) {
	basis := make([]domain.WeightBasis, 0, len(items))
	total, unknown := float64(0), false
	for _, i := range items {
		var saved *domain.WeightBasis
		for _, b := range old.ItemWeights {
			if i.ProductID == b.ProductID && i.BomSpecID == b.BomSpecID && i.BomVariantID == b.BomVariantID {
				copy := b
				copy.Qty = i.Qty
				saved = &copy
				break
			}
		}
		if saved == nil {
			_, rows, err := freightpg.NewRepository(r.pool, r.schema).WeightTotals(ctx, tx, customerID, []freightpg.WeightItem{{ProductID: i.ProductID, BomSpecID: i.BomSpecID, BomVariantID: i.BomVariantID, Qty: i.Qty}}, old.Policy.FreeShipping)
			if err != nil {
				return 0, nil, err
			}
			if len(rows) != 1 {
				return 0, nil, fmt.Errorf("商品物流重量无效")
			}
			saved = &rows[0]
		}
		basis = append(basis, *saved)
		if saved.LogisticsUnitWeightG == nil {
			if !old.Policy.FreeShipping {
				return 0, nil, fmt.Errorf("原商品物流重量缺失，请先核对")
			}
			unknown = true
		} else {
			total += *saved.LogisticsUnitWeightG * float64(i.Qty)
		}
	}
	if math.IsNaN(total) || math.IsInf(total, 0) || total < 0 || total > 1e9 {
		return 0, nil, fmt.Errorf("商品物流重量超出可处理范围")
	}
	if unknown {
		return 0, basis, nil
	}
	return int64(math.Ceil(math.Round(total*1e6) / 1e6)), basis, nil
}

func (r *Repository) fulfillmentEditServiceFeeWritable(ctx context.Context, tx pgx.Tx, customerID, orderID, expectedCents int64) error {
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT round(amount*100)::bigint,COALESCE(settlement_batch_id,0),status FROM %s.customer_fee_items WHERE customer_id=$1 AND source_type='order' AND source_id=$2 AND fee_type='direct_ship_service' FOR UPDATE`, r.schema), customerID, orderID)
	if err != nil {
		return err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var cents, settlement int64
		var status string
		if err := rows.Scan(&cents, &settlement, &status); err != nil {
			return err
		}
		count++
		if cents != expectedCents || settlement > 0 || status != "unsettled" {
			return fmt.Errorf("代发服务费已结算或已变化，请先通过财务流程核对处理")
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("原代发服务费记录不可用，请先核对账单")
	}
	return nil
}

// Called only with a quote rederived from trusted inputs in this same transaction.
// It updates the original source records, never adds a second receivable or edits
// payment allocations. The outer edit transaction owns merchandise and receipts.
func (r *Repository) saveFulfillmentOrderEditFeesTx(ctx context.Context, tx pgx.Tx, customerID, orderID int64, expectedFeeToken, expectedFreightToken string, q app.FulfillmentFeeQuote, actor string) error {
	old, err := r.loadFulfillmentOrderEditFeesTx(ctx, tx, customerID, orderID)
	if err != nil {
		return err
	}
	if expectedFeeToken != old.FeeQuoteToken || expectedFreightToken != old.Freight.Token || strings.TrimSpace(actor) == "" {
		return fmt.Errorf("代发费用已变化，请重新打开后保存")
	}
	if q.CustomerID != customerID || q.FulfillmentMode != old.FulfillmentMode || q.FeeQuoteToken != app.FulfillmentFeeQuoteToken(q) || q.Freight == nil || q.Freight.Token != domain.Token(*q.Freight) || q.Freight.Protocol != domain.Protocol || q.Freight.CurrentReceivableCents != 0 || q.ShippingFeeCents != 0 || q.ServiceFeeCents < 0 || q.ServiceFeeCents > domain.MaxCents || q.Freight.Policy != old.Freight.Policy || q.Freight.TemplateID != old.Freight.TemplateID || q.Freight.TemplateRevision != old.Freight.TemplateRevision || q.FixedFeeCents != old.FixedFeeCents || q.PerPackageFeeCents != old.PerPackageFeeCents || q.SettingsRevision != old.SettingsRevision {
		return fmt.Errorf("代发费用报价无效，不能改变原计费政策")
	}
	if q.FeeQuoteToken == old.FeeQuoteToken {
		return nil
	}
	fr := freightpg.NewRepository(r.pool, r.schema)
	if q.Freight.Token != old.Freight.Token {
		if err = fr.EditableTx(ctx, tx, orderID); err != nil {
			return err
		}
	}
	if q.ServiceFeeCents != old.ServiceFeeCents {
		if err = r.fulfillmentEditServiceFeeWritable(ctx, tx, customerID, orderID, old.ServiceFeeCents); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_fee_items SET amount=$3,note=$4 WHERE customer_id=$1 AND source_type='order' AND source_id=$2 AND fee_type='direct_ship_service'`, r.schema), customerID, orderID, float64(q.ServiceFeeCents)/100, "代发服务费 · 订单编辑 "+q.AdjustmentReason); err != nil {
			return err
		}
	}
	if q.Freight.Token != old.Freight.Token {
		raw, _ := json.Marshal(q.Freight)
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_freight_snapshots SET snapshot_json=$2,created_by=$3 WHERE order_id=$1`, r.schema), orderID, raw, actor); err != nil {
			return err
		}
		if err = fr.Audit(ctx, tx, actor, "order_freight", orderID, "adjust", old.Freight, q.Freight); err != nil {
			return err
		}
	}
	raw, _ := json.Marshal(q)
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.fulfillment_order_fee_snapshots SET quote_json=$3,updated_by=$4,updated_at=now() WHERE order_id=$1 AND customer_id=$2`, r.schema), orderID, customerID, raw, actor); err != nil {
		return err
	}
	return r.auditFulfillmentConfig(ctx, tx, actor, "fulfillment_order_fees", orderID, "edit", old, q)
}
