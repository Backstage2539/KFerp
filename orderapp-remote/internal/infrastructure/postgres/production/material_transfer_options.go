package production

import (
	"context"
	"fmt"
	"strings"

	app "orderapp/internal/application/production"
	stockapp "orderapp/internal/application/stock"
	stockpg "orderapp/internal/infrastructure/postgres/stock"

	"github.com/jackc/pgx/v5"
)

func filterPlanTransferRows(source app.ProductionPlanComponentSource, rows []stockapp.TransferOptionRow) []stockapp.TransferOptionRow {
	out := make([]stockapp.TransferOptionRow, 0, len(rows))
	for _, row := range rows {
		if !planTransferOptionMatchesSource(source, row) {
			continue
		}
		for _, option := range source.Options {
			if option.Warehouse == row.Warehouse && option.OwnerCustomerID == row.OwnerCustomerID {
				row.OwnerName = option.OwnerName
				break
			}
		}
		out = append(out, row)
	}
	return out
}

// PlanMaterialTransferOptions reads one plan source in a consistent snapshot.
// A requested warehouse must be available to that task's customer and owner.
func (r Repository) PlanMaterialTransferOptions(ctx context.Context, planID, sourceID int64, warehouse string) (app.PlanMaterialTransferOptions, error) {
	out := app.PlanMaterialTransferOptions{Warehouses: []app.ProductionPlanComponentSourceOption{}, Options: []app.PlanMaterialTransferOption{}, PendingDocuments: []app.PlanMaterialTransferPending{}}
	if planID <= 0 || sourceID <= 0 {
		return out, fmt.Errorf("生产计划和物料来源编号无效")
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly, IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	sources, err := loadProductionPlanComponentSourcesTx(ctx, tx, r.schema, planID)
	if err != nil {
		return out, err
	}
	found := false
	for _, source := range sources {
		if source.ID == sourceID {
			out.Source = source
			found = true
			break
		}
	}
	if !found {
		return out, fmt.Errorf("计划物料来源不存在或已变化")
	}
	out.Warehouses = append(out.Warehouses, out.Source.Options...)
	warehouse = strings.TrimSpace(warehouse)
	selected := make([]string, 0)
	for _, option := range out.Source.Options {
		if warehouse != "" && option.Warehouse != warehouse {
			continue
		}
		duplicate := false
		for _, existing := range selected {
			if existing == option.Warehouse {
				duplicate = true
				break
			}
		}
		if !duplicate {
			selected = append(selected, option.Warehouse)
		}
	}
	if warehouse != "" && len(selected) == 0 {
		return out, fmt.Errorf("该物料无权使用所选仓库")
	}
	kind := out.Source.ComponentType
	if kind == "product" {
		kind = "finished_product"
	}
	if kind != "material" && kind != "finished_product" {
		return out, fmt.Errorf("该物料类型不支持调拨")
	}
	// Drafts do not reserve physical stock, but within this plan must count
	// against the same batch across all component sources, just as creation does.
	type batchKey struct {
		warehouse string
		owner     int64
		code      string
	}
	pendingByBatch := map[batchKey][2]int64{}
	pendingRows, err := tx.Query(ctx, fmt.Sprintf(`SELECT i.from_warehouse,i.owner_customer_id,i.batch_code,COALESCE(SUM(i.qty_g),0),COALESCE(SUM(i.qty_units),0)
		FROM %[1]s.production_plan_material_transfer_links l
		JOIN %[1]s.stock_entries e ON e.id=l.stock_entry_id AND e.status='draft'
		JOIN %[1]s.stock_entry_items i ON i.id=l.stock_entry_item_id AND i.stock_entry_id=e.id
		WHERE l.production_plan_id=$1 AND i.item_type=$2
		AND (($2='material' AND i.material_id=$3) OR ($2='finished_product' AND i.product_id=$3 AND i.bom_spec_id=$4 AND i.bom_variant_id=$5 AND i.spec_g=$6))
		GROUP BY i.from_warehouse,i.owner_customer_id,i.batch_code`, r.schema), planID, kind, out.Source.ComponentID, out.Source.ComponentBOMSpecID, out.Source.ComponentBOMVariantID, out.Source.ComponentSpecG)
	if err != nil {
		return out, err
	}
	for pendingRows.Next() {
		var key batchKey
		var qty [2]int64
		if err := pendingRows.Scan(&key.warehouse, &key.owner, &key.code, &qty[0], &qty[1]); err != nil {
			pendingRows.Close()
			return out, err
		}
		pendingByBatch[key] = qty
	}
	err = pendingRows.Err()
	pendingRows.Close()
	if err != nil {
		return out, err
	}
	stockRepo := stockpg.NewRepository(r.pool, r.schema)
	for _, location := range selected {
		query := stockapp.TransferOptionsQuery{Warehouse: location, ItemType: kind, ItemID: out.Source.ComponentID, BomSpecID: out.Source.ComponentBOMSpecID, SpecG: out.Source.ComponentSpecG, Limit: 500}
		for {
			page, err := stockRepo.ListTransferOptionsTx(ctx, tx, query)
			if err != nil {
				return out, err
			}
			for _, row := range filterPlanTransferRows(out.Source, page.Rows) {
				pending := pendingByBatch[batchKey{row.Warehouse, row.OwnerCustomerID, row.BatchCode}]
				out.Options = append(out.Options, app.PlanMaterialTransferOption{TransferOptionRow: row, PendingPlanG: pending[0], PendingPlanUnits: pending[1], TransferableG: max(0, row.AvailableG-pending[0]), TransferableUnits: max(0, row.AvailableUnits-pending[1])})
			}
			if !page.HasNext {
				break
			}
			query.Offset += query.Limit
		}
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT e.id,e.entry_no,e.status,l.component_source_id,COALESCE(SUM(COALESCE(NULLIF(l.equivalent_qty_g,0),NULLIF(l.qty_g,0),CASE WHEN s.component_type IN ('product','finished_product') AND s.component_spec_g>0 THEN l.qty_units*s.component_spec_g ELSE 0 END)),0)::bigint,COALESCE(SUM(l.qty_units),0)::bigint FROM %[1]s.production_plan_material_transfer_links l JOIN %[1]s.stock_entries e ON e.id=l.stock_entry_id LEFT JOIN %[1]s.production_plan_component_sources s ON s.id=l.component_source_id WHERE l.production_plan_id=$1 AND l.component_source_id=$2 GROUP BY e.id,e.entry_no,e.status,l.component_source_id ORDER BY e.id`, r.schema), planID, sourceID)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var pending app.PlanMaterialTransferPending
		if err := rows.Scan(&pending.ID, &pending.EntryNo, &pending.Status, &pending.SourceID, &pending.QtyG, &pending.QtyUnits); err != nil {
			rows.Close()
			return out, err
		}
		out.PendingDocuments = append(out.PendingDocuments, pending)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}
