package orderresolution

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Legacy deductions aggregate by product and specification rather than order
// line. Never assign an ambiguous shipment to a convenient line or price.
func shipmentQuantities(items, stock, allocations []map[string]json.RawMessage) (map[int64]float64, string) {
	out := map[int64]float64{}
	for _, d := range stock {
		matched := matchingOrderItems(items, d)
		// Historical batches may have been explicitly allocated to a newer
		// order of the same specification. Require the exact recorded line,
		// batch and frozen allocation; do not substitute the current variant.
		if len(matched) == 0 && num(d, "bom_spec_id") > 0 && num(d, "batch_id") > 0 && num(d, "deducted_g") == 0 && num(d, "spec_g") == 0 {
			for _, item := range items {
				if num(item, "product_id") != num(d, "product_id") || num(item, "bom_spec_id") != num(d, "bom_spec_id") {
					continue
				}
				for _, a := range allocations {
					if num(a, "order_id") == num(d, "order_id") && num(a, "order_item_id") == num(item, "id") && num(a, "product_id") == num(d, "product_id") && num(a, "bom_spec_id") == num(d, "bom_spec_id") && num(a, "bom_variant_id") == num(d, "bom_variant_id") && num(a, "batch_id") == num(d, "batch_id") && str(a, "batch_code") == str(d, "batch_code") && str(a, "warehouse") != "" && num(a, "spec_g") == 0 && num(a, "allocated_g") == 0 && num(a, "allocated_units") >= num(d, "deducted_units") {
						matched = append(matched, item)
						break
					}
				}
			}
		}
		if len(matched) != 1 {
			return out, "发货记录无法唯一对应订单商品，请先核对商品、规格和批次后再处理"
		}
		item := matched[0]
		qty := num(d, "deducted_units")
		if qty == 0 && num(d, "spec_g") > 0 {
			qty = num(d, "deducted_g") / num(d, "spec_g")
		}
		if qty <= 0 {
			return out, "发货记录缺少有效数量，请先核对原出库记录"
		}
		id := int64(num(item, "id"))
		out[id] += qty
		if out[id]+num(item, "resolution_closed_qty") > num(item, "qty")+1e-9 {
			return out, fmt.Sprintf("%s 的发货与关闭数量超过原单，请先核对", str(item, "item_name"))
		}
	}
	return out, ""
}

func matchingOrderItems(items []map[string]json.RawMessage, d map[string]json.RawMessage) []map[string]json.RawMessage {
	var matched []map[string]json.RawMessage
	for _, item := range items {
		if num(item, "product_id") != num(d, "product_id") || num(item, "bom_spec_id") != num(d, "bom_spec_id") {
			continue
		}
		if num(d, "bom_spec_id") == 0 {
			digits := strings.Map(func(r rune) rune {
				if r >= '0' && r <= '9' {
					return r
				}
				return -1
			}, str(item, "spec"))
			grams, _ := strconv.ParseInt(digits, 10, 64)
			if float64(grams) != num(d, "spec_g") {
				continue
			}
		}
		if num(item, "bom_variant_id") > 0 && num(d, "bom_variant_id") > 0 && num(item, "bom_variant_id") != num(d, "bom_variant_id") {
			continue
		}
		matched = append(matched, item)
	}
	return matched
}
