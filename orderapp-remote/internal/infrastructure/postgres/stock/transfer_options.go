package stock

import (
	"context"
	"fmt"
	"sort"
	"strings"

	stockapp "orderapp/internal/application/stock"

	"github.com/jackc/pgx/v5"
)

func (r Repository) ListTransferOptions(ctx context.Context, q stockapp.TransferOptionsQuery) (stockapp.TransferOptionsResult, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly, IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return stockapp.TransferOptionsResult{}, err
	}
	defer tx.Rollback(ctx)
	type candidate struct {
		row             stockapp.TransferOptionRow
		quality, status string
		masterOwner     int64
	}
	all := []candidate{}
	if q.ItemType == "" || q.ItemType == "material" {
		rows, e := tx.Query(ctx, fmt.Sprintf(`SELECT l.material_id,COALESCE(m.name,''),COALESCE(m.code,''),COALESCE(m.unit,''),COALESCE(b.owner_customer_id,0),
COALESCE(m.owner_customer_id,0),b.id,b.batch_code,l.qty_g,l.qty_units,COALESCE(b.quality_status,'unchecked'),b.status
FROM %[1]s.material_batch_locations l JOIN %[1]s.material_batches b ON b.id=l.material_batch_id
JOIN %[1]s.materials m ON m.id=l.material_id WHERE l.warehouse=$1 AND (l.qty_g>0 OR l.qty_units>0)
ORDER BY m.name,b.batch_code`, r.schema), q.Warehouse)
		if e != nil {
			return stockapp.TransferOptionsResult{}, e
		}
		for rows.Next() {
			c := candidate{}
			c.row.Warehouse, c.row.ItemType = q.Warehouse, "material"
			if e = rows.Scan(&c.row.ItemID, &c.row.ItemName, &c.row.ItemCode, &c.row.InventoryUnit, &c.row.OwnerCustomerID, &c.masterOwner, &c.row.BatchID, &c.row.BatchCode, &c.row.QtyG, &c.row.QtyUnits, &c.quality, &c.status); e != nil {
				rows.Close()
				return stockapp.TransferOptionsResult{}, e
			}
			c.row.SpecName = c.row.InventoryUnit
			all = append(all, c)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return stockapp.TransferOptionsResult{}, e
		}

	}
	if q.ItemType == "" || q.ItemType == "finished_product" {
		specExpr, joins := "''::text", ""
		hasSpec, e := stockSchemaRelationExistsTx(ctx, tx, r.schema, "production_bom_specs")
		if e != nil {
			return stockapp.TransferOptionsResult{}, e
		}
		hasVariant, e := stockSchemaRelationExistsTx(ctx, tx, r.schema, "production_bom_version_variants")
		if e != nil {
			return stockapp.TransferOptionsResult{}, e
		}
		unitExpr := "''::text"
		productCodeExpr := "''::text"
		hasCode, e := stockSchemaColumnExistsTx(ctx, tx, r.schema, "products", "sku_code")
		if e != nil {
			return stockapp.TransferOptionsResult{}, e
		}
		if hasCode {
			productCodeExpr = "COALESCE(p.sku_code,'')"
		}
		if hasSpec && hasVariant {
			specExpr = "COALESCE(NULLIF(v.spec_name_snapshot,''),NULLIF(s.name,''),'')"
			unitExpr = "COALESCE(NULLIF(v.inventory_unit,''),NULLIF(s.inventory_unit,''),'')"
			joins = fmt.Sprintf("LEFT JOIN %s.production_bom_specs s ON s.id=b.bom_spec_id LEFT JOIN %s.production_bom_version_variants v ON v.id=b.bom_variant_id AND v.bom_spec_id=b.bom_spec_id", r.schema, r.schema)
		}
		rows, e := tx.Query(ctx, fmt.Sprintf(`SELECT b.item_id,COALESCE(p.name,''),%s,b.bom_spec_id,b.bom_variant_id,b.spec_g,%s,%s,
b.owner_customer_id,b.id,b.batch_code,b.remaining_g,b.remaining_units,COALESCE(b.quality_status,'unchecked')
FROM %s.stock_batches b JOIN %s.products p ON p.id=b.item_id %s
LEFT JOIN LATERAL (SELECT l.warehouse FROM %s.stock_ledger_entries l WHERE l.source_batch_code=b.batch_code AND l.item_type=b.item_type
AND l.item_id=b.item_id AND l.bom_spec_id=b.bom_spec_id AND l.spec_g=b.spec_g AND l.owner_customer_id=b.owner_customer_id ORDER BY l.id DESC LIMIT 1) last ON true
WHERE b.item_type='finished_product' AND (b.remaining_g>0 OR b.remaining_units>0) AND COALESCE(last.warehouse,'finished_goods')=$1
ORDER BY p.name,b.batch_code`, productCodeExpr, specExpr, unitExpr, r.schema, r.schema, joins, r.schema), q.Warehouse)
		if e != nil {
			return stockapp.TransferOptionsResult{}, e
		}
		for rows.Next() {
			c := candidate{}
			c.row.Warehouse, c.row.ItemType, c.status = q.Warehouse, "finished_product", "active"
			if e = rows.Scan(&c.row.ItemID, &c.row.ItemName, &c.row.ItemCode, &c.row.BomSpecID, &c.row.BomVariantID, &c.row.SpecG, &c.row.SpecName, &c.row.InventoryUnit, &c.row.OwnerCustomerID, &c.row.BatchID, &c.row.BatchCode, &c.row.QtyG, &c.row.QtyUnits, &c.quality); e != nil {
				rows.Close()
				return stockapp.TransferOptionsResult{}, e
			}
			if c.row.SpecName == "" && c.row.SpecG > 0 {
				c.row.SpecName = fmt.Sprintf("%dg", c.row.SpecG)
			}
			all = append(all, c)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return stockapp.TransferOptionsResult{}, e
		}
		summarySpec, summaryUnit, summaryJoins := "''::text", "''::text", ""
		if hasSpec && hasVariant {
			summarySpec = "COALESCE(NULLIF(v.spec_name_snapshot,''),NULLIF(s.name,''),'')"
			summaryUnit = "COALESCE(NULLIF(v.inventory_unit,''),NULLIF(s.inventory_unit,''),'')"
			summaryJoins = fmt.Sprintf("LEFT JOIN %s.production_bom_specs s ON s.id=fi.bom_spec_id LEFT JOIN %s.production_bom_version_variants v ON v.id=fi.bom_variant_id AND v.bom_spec_id=fi.bom_spec_id", r.schema, r.schema)
		}
		summaryRows, e := tx.Query(ctx, fmt.Sprintf(`SELECT fi.product_id,COALESCE(p.name,''),%s,fi.bom_spec_id,fi.bom_variant_id,fi.spec_g,%s,%s,fi.owner_customer_id,fi.onhand_units,fi.onhand_loose_g
FROM %s.finished_inventory fi JOIN %s.products p ON p.id=fi.product_id %s
WHERE fi.warehouse=$1 AND (fi.onhand_units>0 OR fi.onhand_loose_g>0)`, productCodeExpr, summarySpec, summaryUnit, r.schema, r.schema, summaryJoins), q.Warehouse)
		if e != nil {
			return stockapp.TransferOptionsResult{}, e
		}
		for summaryRows.Next() {
			c := candidate{}
			c.row.Warehouse, c.row.ItemType, c.status = q.Warehouse, "finished_product", "unbatched"
			var loose int64
			if e = summaryRows.Scan(&c.row.ItemID, &c.row.ItemName, &c.row.ItemCode, &c.row.BomSpecID, &c.row.BomVariantID, &c.row.SpecG, &c.row.SpecName, &c.row.InventoryUnit, &c.row.OwnerCustomerID, &c.row.QtyUnits, &loose); e != nil {
				summaryRows.Close()
				return stockapp.TransferOptionsResult{}, e
			}
			c.row.QtyG = c.row.QtyUnits*c.row.SpecG + loose
			if c.row.SpecName == "" && c.row.SpecG > 0 {
				c.row.SpecName = fmt.Sprintf("%dg", c.row.SpecG)
			}
			found := false
			for _, existing := range all {
				if existing.row.ItemType == c.row.ItemType && existing.row.ItemID == c.row.ItemID && existing.row.BomSpecID == c.row.BomSpecID && existing.row.SpecG == c.row.SpecG && existing.row.OwnerCustomerID == c.row.OwnerCustomerID {
					found = true
					break
				}
			}
			if !found {
				all = append(all, c)
			}
		}
		e = summaryRows.Err()
		summaryRows.Close()
		if e != nil {
			return stockapp.TransferOptionsResult{}, e
		}
	}
	result := stockapp.TransferOptionsResult{Rows: []stockapp.TransferOptionRow{}}
	summaryMatches := make(map[string]bool)
	for _, c := range all {
		row := c.row
		if q.ItemID > 0 && row.ItemID != q.ItemID || q.BomSpecID > 0 && row.BomSpecID != q.BomSpecID || q.SpecG > 0 && row.SpecG != q.SpecG {
			continue
		}
		if q.Q != "" && !strings.Contains(strings.ToLower(fmt.Sprintf("%s %s %s %s %d", row.ItemName, row.ItemCode, row.BatchCode, row.SpecName, row.ItemID)), strings.ToLower(q.Q)) {
			continue
		}
		if c.status == "unbatched" {
			row.UnavailableReason = "历史库存缺少批次，请先核对"
		} else if c.status != "active" || c.quality == "hold" || c.quality == "reject" {
			row.UnavailableReason = "批次停用或质检冻结"
		}
		if row.ItemType == "material" && row.OwnerCustomerID != c.masterOwner {
			row.UnavailableReason = "批次货主与物料档案不一致"
		}
		unbound, err := hasUnboundTransferReservationTx(ctx, tx, r.schema, row.ItemID, row.OwnerCustomerID, q.Warehouse, row.ItemType == "finished_product")
		if err != nil {
			return stockapp.TransferOptionsResult{}, err
		}
		if unbound {
			row.UnavailableReason = "存在未绑定批次的预留，请先核对"
		}
		if row.ItemType == "finished_product" && row.BatchID > 0 {
			key := fmt.Sprintf("%d:%d:%d:%d", row.ItemID, row.BomSpecID, row.SpecG, row.OwnerCustomerID)
			matched, cached := summaryMatches[key]
			if !cached {
				matched, err = finishedTransferBalancesMatchTx(ctx, tx, r.schema, row.ItemID, row.BomSpecID, row.SpecG, row.OwnerCustomerID, q.Warehouse)
				if err != nil {
					return stockapp.TransferOptionsResult{}, err
				}
				summaryMatches[key] = matched
			}
			if !matched {
				row.UnavailableReason = "库存与批次汇总不一致，请先核对"
			}
			retained, err := transferBatchRetainedTx(ctx, tx, r.schema, row.BatchID)
			if err != nil {
				return stockapp.TransferOptionsResult{}, err
			}
			if retained {
				row.UnavailableReason = "客户寄存库存不能普通转仓"
			}
		}
		var rg, rn int64
		var e error
		if row.BatchID > 0 {
			rg, rn, e = transferReservedTx(ctx, tx, r.schema, row.BatchID, row.ItemID, row.OwnerCustomerID, q.Warehouse, row.ItemType == "finished_product")
		}
		if e != nil {
			return stockapp.TransferOptionsResult{}, e
		}
		row.AvailableUnits = nonnegativeStockQty(row.QtyUnits - rn)
		row.AvailableG = nonnegativeStockQty(row.QtyG - rg)
		if row.ItemType == "finished_product" && row.BomSpecID == 0 && row.SpecG > 0 {
			loose := row.QtyG - row.QtyUnits*row.SpecG
			if loose < 0 {
				row.UnavailableReason = "批次重量与件数不一致，请核对"
			}
			row.AvailableG = nonnegativeStockQty(loose - nonnegativeStockQty(rg-rn*row.SpecG))
		}
		if row.UnavailableReason != "" {
			row.AvailableG, row.AvailableUnits = 0, 0
		}
		result.Rows = append(result.Rows, row)
	}
	sort.SliceStable(result.Rows, func(i, j int) bool {
		a, b := result.Rows[i], result.Rows[j]
		if a.ItemName != b.ItemName {
			return a.ItemName < b.ItemName
		}
		return a.BatchCode < b.BatchCode
	})
	result.Total = len(result.Rows)
	if q.Limit <= 0 {
		q.Limit = 100
	}
	if q.Offset < 0 {
		q.Offset = 0
	}
	result.Limit, result.Page = q.Limit, q.Offset/q.Limit+1
	if q.Offset >= len(result.Rows) {
		result.Rows = []stockapp.TransferOptionRow{}
	} else {
		end := q.Offset + q.Limit
		if end > len(result.Rows) {
			end = len(result.Rows)
		}
		result.HasNext = end < len(result.Rows)
		result.Rows = result.Rows[q.Offset:end]
	}
	return result, nil
}
