package customerfulfillment

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"strings"
)

func (r *Repository) CustomerWorkspace(ctx context.Context, customerID int64, page string, publicationID int64) (map[string]any, error) {
	if err := r.requirePortalCustomerWithWorkbench(ctx, customerID); err != nil {
		return nil, err
	}
	name, err := r.resolveCustomerName(ctx, customerID)
	if err != nil {
		return nil, err
	}
	codes, err := r.listCustomerCapabilityCodes(ctx, customerID)
	if err != nil {
		return nil, err
	}
	has := func(code string) bool {
		for _, c := range codes {
			if c == code {
				return true
			}
		}
		return false
	}
	out := map[string]any{"customer_id": customerID, "customer_name": name, "capabilities": codes}
	switch page {
	case "context":
		return out, nil
	case "home":
		if has("direct_ship") || has("product_order") || has("processing") || has("mall") {
			var orders, pending, missing int
			var amount float64
			err = r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*),count(*) FILTER(WHERE coalesce(s.name,'') NOT IN ('已发货','已出库','已签收','已收货','已完成')),count(*) FILTER(WHERE btrim(coalesce(o.receiver_name,''))='' OR btrim(coalesce(o.receiver_phone,''))='' OR btrim(coalesce(o.receiver_address,''))=''),coalesce(sum(o.grand_total) FILTER(WHERE coalesce(p.name,'') IN ('未付款','未收款','部分付款','部分收款')),0)::float8 FROM %[1]s.orders o LEFT JOIN %[1]s.ship_statuses s ON s.id=o.ship_status_id LEFT JOIN %[1]s.pay_statuses p ON p.id=o.pay_status_id WHERE o.customer_id=$1 AND coalesce(o.is_void,false)=false AND o.portal_service_code<>''`, r.schema), customerID).Scan(&orders, &pending, &missing, &amount)
			if err != nil {
				return nil, err
			}
			out["orders"] = orders
			out["pending_shipment"] = pending
			out["missing_recipient"] = missing
			if has("settlement") {
				out["pending_settlement_amount"] = amount
			}
		}
		if has("inventory_custody") {
			var count int
			err = r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.warehouses WHERE customer_id=$1 AND active=true`, r.schema), customerID).Scan(&count)
			if err != nil {
				return nil, err
			}
			out["warehouses"] = count
			var units, weight int64
			err = r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT coalesce(sum(fi.onhand_units),0)::bigint,coalesce(sum(fi.onhand_units*fi.spec_g+fi.onhand_loose_g),0)::bigint FROM %[1]s.finished_inventory fi JOIN %[1]s.warehouses w ON w.code=fi.warehouse AND w.active=true AND w.customer_id=$1 WHERE fi.owner_customer_id IN (0,$1)`, r.schema), customerID).Scan(&units, &weight)
			if err != nil {
				return nil, err
			}
			out["finished_goods_units"] = units
			out["finished_goods_kg"] = float64(weight) / 1000
		}
	case "bean_list":
		if !has(page) {
			return nil, fmt.Errorf("价格表能力未开通")
		}
		rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT id,coalesce(config_json->'publication_batch'->>'table_name',nullif(product_type_name,''),list_type),version_no,coalesce(to_char(published_at,'YYYY-MM-DD HH24:MI'),''),list_type FROM %s.bean_list_publications WHERE owner_type='customer' AND owner_key=$1 AND status='published' AND coalesce(nullif(publication_purpose,''),'factory_supply')='factory_supply' ORDER BY published_at DESC NULLS LAST,id DESC`, r.schema), fmt.Sprint(customerID))
		if err != nil {
			return nil, err
		}
		lists, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (map[string]any, error) {
			var id int64
			var name, version, date, kind string
			err := row.Scan(&id, &name, &version, &date, &kind)
			return map[string]any{"id": id, "name": name, "version_no": version, "published_at": date, "list_type": kind}, err
		})
		if err != nil {
			return nil, err
		}
		out["price_lists"] = lists
		if publicationID > 0 {
			found := false
			for _, list := range lists {
				if list["id"] == publicationID {
					found = true
				}
			}
			if !found {
				return nil, fmt.Errorf("价格表不存在或不属于当前客户")
			}
			var content []byte
			err = r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT content_json FROM %s.bean_list_publications WHERE id=$1 AND owner_type='customer' AND owner_key=$2 AND status='published'`, r.schema), publicationID, fmt.Sprint(customerID)).Scan(&content)
			if err != nil {
				return nil, err
			}
			out["rows"], err = customerPricePreviewRows(content)
			if err != nil {
				return nil, err
			}
		}
	case "inventory_custody":
		if !has(page) {
			return nil, fmt.Errorf("库存能力未开通")
		}
		out["custody_balances"], err = r.listCustodyBalances(ctx, customerID)
		if err == nil {
			out["finished_goods"], err = r.listFinishedGoods(ctx, customerID)
		}
	case "processing":
		if !has(page) {
			return nil, fmt.Errorf("代加工能力未开通")
		}
		out["processing_orders"], err = r.listProcessingOrders(ctx, customerID)
	case "settlement":
		if !has(page) {
			return nil, fmt.Errorf("结算能力未开通")
		}
		out["fees"], err = r.listFeeItems(ctx, customerID)
		if err == nil {
			out["settlements"], err = r.listSettlements(ctx, customerID)
		}
	case "direct_ship", "product_order", "mall":
		if !has(page) {
			return nil, fmt.Errorf("录单能力未开通")
		}
	default:
		return nil, fmt.Errorf("未知客户页面")
	}
	return out, err
}

func customerPricePreviewRows(content []byte) ([]map[string]any, error) {
	var snapshot struct {
		PriceRows []map[string]any `json:"price_rows"`
	}
	if err := json.Unmarshal(content, &snapshot); err != nil {
		return nil, fmt.Errorf("价格表内容无法读取")
	}
	result := make([]map[string]any, 0, len(snapshot.PriceRows))
	if len(snapshot.PriceRows) == 0 {
		var legacy customerFulfillmentPublishedContent
		if err := json.Unmarshal(content, &legacy); err != nil {
			return nil, err
		}
		for _, group := range legacy.Groups {
			for _, item := range group.Items {
				var fields map[string]any
				_ = json.Unmarshal(item, &fields)
				name := fields["product_name"]
				if name == nil {
					name = fields["name"]
				}
				for _, kind := range []string{"commercial", "retail", "green", "drip"} {
					for _, tier := range customerFulfillmentPublishedItemTiers(item, kind) {
						option := customerFulfillmentPublishedTierOption(0, kind, tier)
						var max any = ""
						if tier.MaxQty != nil {
							max = *tier.MaxQty
						} else if tier.MaxLb != nil {
							max = *tier.MaxLb
						}
						unit := tier.PriceUnit
						if unit == "" {
							unit = tier.DisplayUnit
						}
						if unit == "" {
							unit = tier.SalesUnit
						}
						switch unit {
						case "lb":
							unit = "磅"
						case "kg":
							unit = "kg"
						case "bag":
							unit = "袋"
						case "box":
							unit = "盒"
						}
						result = append(result, map[string]any{"product_name": name, "spec": fmt.Sprintf("%dg", option.SpecG), "min_qty": option.Min, "max_qty": max, "quantity_unit": unit, "price_unit": unit, "unit_price": option.UnitPrice})
					}
				}
			}
		}
		return result, nil
	}

	for _, row := range snapshot.PriceRows {
		pick := func(keys ...string) any {
			for _, key := range keys {
				if v, ok := row[key]; ok && v != nil && strings.TrimSpace(fmt.Sprint(v)) != "" {
					return v
				}
			}
			return ""
		}
		result = append(result, map[string]any{"product_name": pick("product_name", "product_title", "base_product_name", "name"), "spec": pick("bom_spec_name", "spec_name", "spec", "spec_label", "spec_g"), "min_qty": pick("min_qty", "min_lb"), "max_qty": pick("max_qty", "max_lb"), "quantity_unit": pick("tier_quantity_unit", "inventory_unit"), "price_unit": pick("price_unit", "display_unit", "inventory_unit"), "unit_price": pick("final_unit_price", "unit_price", "price")})
	}
	return result, nil
}
