package finance

import (
	"context"
	"fmt"
	"math"
	"strings"

	appfinance "orderapp/internal/application/finance"
	domain "orderapp/internal/domain/finance"
)

// orderSalesCostDetails reports only quantities which can be traced to an
// actual batch. Reservations are not consumption, and a missing or ambiguous
// link is represented as a separate pending row.
func (r Repository) orderSalesCostDetails(ctx context.Context, filter appfinance.ReportFilter) ([]appfinance.SourceDetail, error) {
	out := []appfinance.SourceDetail{}
	var haveItems, haveDeductions, haveStock, haveAllocations, haveRunning, haveCosts, haveEvents, haveWorkOrders, haveReturns bool
	err := r.query().QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL,to_regclass($2) IS NOT NULL,to_regclass($3) IS NOT NULL,to_regclass($4) IS NOT NULL,to_regclass($5) IS NOT NULL,to_regclass($6) IS NOT NULL,to_regclass($7) IS NOT NULL,to_regclass($8) IS NOT NULL,to_regclass($9) IS NOT NULL`,
		r.schema+".order_items", r.schema+".order_stock_deductions", r.schema+".stock_batches", r.schema+".produce_batch_allocations", r.schema+".produce_running_items", r.schema+".production_batch_costs", r.schema+".production_operation_events", r.schema+".work_orders", r.schema+".order_resolution_stock_returns").Scan(&haveItems, &haveDeductions, &haveStock, &haveAllocations, &haveRunning, &haveCosts, &haveEvents, &haveWorkOrders, &haveReturns)
	if err != nil {
		return out, err
	}
	// A revenue-bearing order with missing lines has no reliable way to
	// distinguish merchandise from service-only charges. Do not silently
	// report its cost as a confirmed zero.
	missingItems := "TRUE"
	if haveItems {
		missingItems = fmt.Sprintf(`NOT EXISTS (SELECT 1 FROM %s.order_items i WHERE i.order_id=o.id)`, r.schema)
	}
	missingQuery := fmt.Sprintf(`SELECT o.id,to_char(o.order_date,'YYYY-MM-DD'),COALESCE(o.order_no,''),COALESCE(c.name,'')
 FROM %[1]s.orders o LEFT JOIN %[1]s.customers c ON c.id=o.customer_id
 WHERE (COALESCE(o.is_void,false)=false OR COALESCE(to_jsonb(o)->>'resolution_status','')='cancelled')
 AND o.order_date >= $1::date AND o.order_date < $1::date+interval '1 month'
 AND ($2::bigint=0 OR o.customer_id=$2) AND (%[2]s) <> 0 AND %[3]s ORDER BY o.id`, r.schema, financeOrderRevenueSQL("o"), missingItems)
	missingRows, err := r.query().Query(ctx, missingQuery, filter.Month+"-01", filter.CustomerID)
	if err != nil {
		return nil, err
	}
	for missingRows.Next() {
		var id int64
		var date, order, client string
		if err = missingRows.Scan(&id, &date, &order, &client); err != nil {
			missingRows.Close()
			return nil, err
		}
		out = append(out, appfinance.SourceDetail{Section: "main_cost", SourceType: "order_sales_cost_pending", SourceID: id, Date: date, Name: order, Category: "订单销售成本", Counterparty: client, CostUnavailable: true, PendingReason: "订单缺少商品明细，无法确认成本", Link: fmt.Sprintf("/app/vue-shell?view=orders&order_id=%d", id)})
	}
	err = missingRows.Err()
	missingRows.Close()
	if err != nil || !haveItems {
		return out, err
	}
	returnQty := "0::numeric"
	if haveReturns {
		returnQty = fmt.Sprintf(`COALESCE((SELECT sum(x.quantity) FROM %s.order_resolution_stock_returns x WHERE x.order_item_id=oi.id AND x.status='posted'),0)`, r.schema)
	}
	query := fmt.Sprintf(`SELECT o.id,oi.id,COALESCE(oi.product_id,0),COALESCE(oi.bom_spec_id,0),COALESCE(oi.bom_variant_id,0),
 (CASE WHEN lower(trim(COALESCE(oi.unit,''))) IN ('kg','公斤','千克') THEN 1000
 WHEN lower(trim(COALESCE(oi.unit,''))) IN ('g','克') THEN 1
 WHEN COALESCE(to_jsonb(oi)->'weight_snapshot_json','{}'::jsonb) <> '{}'::jsonb
 AND NOT COALESCE((to_jsonb(oi)->'weight_snapshot_json'->>'historical_unknown')::boolean,false)
 THEN COALESCE(NULLIF(to_jsonb(oi)->>'material_unit_weight_g','')::numeric,0) ELSE 0 END)::float8,
 GREATEST(0,COALESCE(oi.qty,0)-COALESCE(NULLIF(to_jsonb(oi)->>'resolution_closed_qty','')::numeric,0)+%s)::float8,
 COALESCE(o.order_no,''),to_char(o.order_date,'YYYY-MM-DD'),COALESCE(oi.item_name,''),COALESCE(oi.unit,''),COALESCE(c.name,''),o.customer_id,
 count(*) OVER(PARTITION BY o.id,oi.product_id,oi.bom_spec_id,oi.bom_variant_id)::int
 FROM %[2]s.order_items oi JOIN %[2]s.orders o ON o.id=oi.order_id LEFT JOIN %[2]s.customers c ON c.id=o.customer_id
 WHERE (COALESCE(o.is_void,false)=false OR COALESCE(to_jsonb(o)->>'resolution_status','')='cancelled')
 AND o.order_date >= $1::date AND o.order_date < $1::date+interval '1 month' AND ($2::bigint=0 OR o.customer_id=$2)
 ORDER BY o.order_date,o.id,oi.id`, returnQty, r.schema)
	rows, err := r.query().Query(ctx, query, filter.Month+"-01", filter.CustomerID)
	if err != nil {
		return nil, err
	}
	type item struct {
		orderID, id, product, spec, variant, customer int64
		qty, grams                                    float64
		order, date, name, unit, client               string
		duplicates                                    int
	}
	var items []item
	for rows.Next() {
		var i item
		if err = rows.Scan(&i.orderID, &i.id, &i.product, &i.spec, &i.variant, &i.grams, &i.qty, &i.order, &i.date, &i.name, &i.unit, &i.client, &i.customer, &i.duplicates); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, i)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for _, i := range items {
		if i.qty <= 0 {
			continue
		}
		parts := []salesCostPart{}
		ambiguous := i.duplicates > 1 || i.product <= 0
		unknownLineage := false
		if !ambiguous && haveDeductions && haveStock {
			lineage := "''"
			joins := ""
			if haveRunning {
				lineage = "COALESCE(ri.batch_id,'')"
				joins = fmt.Sprintf("LEFT JOIN %[1]s.produce_running_items ri ON b.source_doc_type='production_run' AND ri.id=b.source_doc_id", r.schema)
			}
			if haveRunning && haveEvents && haveWorkOrders {
				lineage = "COALESCE(ri.batch_id,ri2.batch_id,'')"
				joins += fmt.Sprintf(" LEFT JOIN %[1]s.production_operation_events ev ON b.source_doc_type='production_operation_event' AND ev.id=b.source_doc_id LEFT JOIN %[1]s.work_orders wo ON wo.id=ev.work_order_id LEFT JOIN %[1]s.produce_running_items ri2 ON ri2.id=wo.running_item_id", r.schema)
			}
			q := fmt.Sprintf(`SELECT d.deducted_units::float8,d.deducted_g::float8,COALESCE(b.qty_g,0)::float8,
 COALESCE(b.unit_cost,0)::float8,COALESCE((to_jsonb(b)->>'cost_unavailable')::boolean,false),
 COALESCE(b.owner_customer_id,0),b.id IS NULL,%s
 FROM %[2]s.order_stock_deductions d LEFT JOIN %[2]s.stock_batches b ON b.id=d.batch_id AND b.batch_code=d.batch_code
 AND b.item_type='finished_product' AND b.item_id=d.product_id AND COALESCE(b.bom_spec_id,0)=COALESCE(d.bom_spec_id,0) AND COALESCE(b.bom_variant_id,0)=COALESCE(d.bom_variant_id,0) %s
 WHERE d.order_id=$1 AND d.product_id=$2 AND COALESCE(d.bom_spec_id,0)=$3 AND COALESCE(d.bom_variant_id,0)=$4
 AND ($3>0 OR d.spec_g=$5::numeric) ORDER BY d.id`, lineage, r.schema, joins)
			rs, e := r.query().Query(ctx, q, i.orderID, i.product, i.spec, i.variant, i.grams)
			if e != nil {
				return nil, e
			}
			for rs.Next() {
				var units, grams, batchG, cost float64
				var unavailable, missing bool
				var owner int64
				var batch string
				if e = rs.Scan(&units, &grams, &batchG, &cost, &unavailable, &owner, &missing, &batch); e != nil {
					rs.Close()
					return nil, e
				}
				qty, convertedCost, valid := convertBatchCost(i.unit, i.grams, units, grams, batchG, cost)
				cost = convertedCost
				if !valid {
					unavailable = true
					qty = math.Max(units, 0)
				}
				if owner > 0 {
					if owner == i.customer {
						cost = 0
						// Customer ownership makes the factory cost zero, but it
						// does not establish how many order units were supplied.
						unavailable = !valid
					} else {
						unavailable = true
					}
				}
				if missing || qty <= 0 {
					unavailable = true
				}
				if batch == "" {
					unknownLineage = true
				} else {
					batch = "production:" + batch
				}
				parts = append(parts, salesCostPart{quantity: qty, unitCost: cost, unknown: unavailable, source: "actual", batch: batch})
			}
			e = rs.Err()
			rs.Close()
			if e != nil {
				return nil, e
			}
		}
		if !ambiguous && haveAllocations && haveRunning && haveCosts {
			q := fmt.Sprintf(`SELECT a.allocated_units::float8,COALESCE(pc.unit_cost_per_kg,0)::float8,
 COALESCE((to_jsonb(pc)->>'cost_unavailable')::boolean,false),pc.id IS NULL,
 COALESCE(pc.finished_g,0)::float8,COALESCE(ri.status,''),a.batch_id,
 count(ri.id) OVER(PARTITION BY a.id)::int, a.spec_g::float8,
 COALESCE((SELECT sum(a2.allocated_units) FROM %[1]s.produce_batch_allocations a2 WHERE a2.batch_id=a.batch_id AND a2.product_id=a.product_id AND a2.spec_g=a.spec_g),0)::float8
 FROM %[1]s.produce_batch_allocations a
 LEFT JOIN %[1]s.produce_running_items ri ON ri.batch_id=a.batch_id AND ri.product_id=a.product_id AND ri.spec_g=a.spec_g AND COALESCE(ri.bom_spec_id,0)=$2 AND COALESCE(ri.bom_variant_id,0)=$3 AND COALESCE(ri.output_type,'product')='product'
 LEFT JOIN %[1]s.production_batch_costs pc ON pc.running_item_id=ri.id
 WHERE a.order_item_id=$1 ORDER BY a.id,ri.id`, r.schema)
			rs, e := r.query().Query(ctx, q, i.id, i.spec, i.variant)
			if e != nil {
				return nil, e
			}
			for rs.Next() {
				var qty, cost, finished, allocatedSpec, totalAllocated float64
				var unknown, missing bool
				var status, batch string
				var count int
				if e = rs.Scan(&qty, &cost, &unknown, &missing, &finished, &status, &batch, &count, &allocatedSpec, &totalAllocated); e != nil {
					rs.Close()
					return nil, e
				}
				unit := strings.ToLower(strings.TrimSpace(i.unit))
				if i.grams <= 0 || allocatedSpec != i.grams || finished < totalAllocated*i.grams || count != 1 || status != "done" || missing || unknownLineage || unit == "kg" || unit == "g" || unit == "公斤" || unit == "千克" || unit == "克" {
					unknown = true
				}
				parts = append(parts, salesCostPart{quantity: qty, unitCost: cost * float64(i.grams) / 1000, unknown: unknown, source: "production", batch: "production:" + batch})
			}
			e = rs.Err()
			rs.Close()
			if e != nil {
				return nil, e
			}
		}
		matched := matchSalesCost(salesCostItem{quantity: i.qty}, parts)
		base := appfinance.SourceDetail{Section: "main_cost", SourceID: i.orderID, Date: i.date, Name: strings.TrimSpace(i.order + " · " + i.name), Category: "订单销售成本", Counterparty: i.client, Unit: i.unit, Link: fmt.Sprintf("/app/vue-shell?view=orders&order_id=%d", i.orderID)}
		if matched.knownQty > 0 {
			row := base
			row.SourceType = "order_sales_cost"
			row.Quantity = matched.knownQty
			row.CostSource = "实际出库或已完工生产批次"
			row.Amount = domain.Money(matched.knownCost)
			out = append(out, row)
		}
		if matched.pendingQty > 1e-9 {
			row := base
			row.SourceType = "order_sales_cost_pending"
			row.Quantity = math.Round(matched.pendingQty*10000) / 10000
			row.CostUnavailable = true
			row.PendingReason = "缺少可唯一确认的实际成本或完成数量"
			if ambiguous {
				row.PendingReason = "订单商品规格存在多行或缺少商品身份，无法唯一分配成本"
			}
			out = append(out, row)
		}
	}
	if haveReturns {
		q := fmt.Sprintf(`SELECT x.order_id,to_char(x.posting_date,'YYYY-MM-DD'),COALESCE(o.order_no,''),COALESCE(oi.item_name,''),COALESCE(oi.unit,''),COALESCE(c.name,''),x.quantity::float8,x.unit_cost::float8,x.source_kind,x.source_owner_customer_id
 FROM %[1]s.order_resolution_stock_returns x JOIN %[1]s.orders o ON o.id=x.order_id JOIN %[1]s.order_items oi ON oi.id=x.order_item_id LEFT JOIN %[1]s.customers c ON c.id=o.customer_id
 WHERE x.status='posted' AND x.posting_date >= $1::date AND x.posting_date < $1::date+interval '1 month' AND ($2::bigint=0 OR o.customer_id=$2) ORDER BY x.id`, r.schema)
		rs, e := r.query().Query(ctx, q, filter.Month+"-01", filter.CustomerID)
		if e != nil {
			return nil, e
		}
		for rs.Next() {
			var id int64
			var date, order, name, unit, client, sourceKind string
			var sourceOwner int64
			var qty, cost float64
			if e = rs.Scan(&id, &date, &order, &name, &unit, &client, &qty, &cost, &sourceKind, &sourceOwner); e != nil {
				rs.Close()
				return nil, e
			}
			if sourceKind == "shipment" && sourceOwner > 0 {
				cost = 0
			}
			out = append(out, appfinance.SourceDetail{Section: "main_cost", SourceType: "order_sales_cost_return", SourceID: id, Date: date, Name: order + " · " + name + " 退回", Category: "销售成本冲回", Counterparty: client, Quantity: -qty, Unit: unit, CostSource: "退回批次冻结成本", Amount: domain.Money(-math.Round(qty*cost*100) / 100), Link: fmt.Sprintf("/app/vue-shell?view=orders&order_id=%d", id)})
		}
		e = rs.Err()
		rs.Close()
		if e != nil {
			return nil, e
		}
	}
	return out, nil
}
