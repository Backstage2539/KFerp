package production

import (
	"context"
	"fmt"
	productionapp "orderapp/internal/application/production"
	stockdomain "orderapp/internal/domain/stock"
	"strings"

	"github.com/jackc/pgx/v5"
)

// GetWorkOrderFrozenSourceIssueItems projects the exact batch reservations into
// an issue document. A row is omitted after its batch has reached WIP, making
// repeated previews idempotent and preventing a second transfer.
func (r Repository) GetWorkOrderFrozenSourceIssueItems(ctx context.Context, workOrderID int64) (bool, []productionapp.StockEntryItemCommand, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return false, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	usesFrozenSources, err := workOrderUsesFrozenComponentSourcesTx(ctx, tx, r.schema, workOrderID)
	if err != nil || !usesFrozenSources {
		return usesFrozenSources, nil, err
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT reservation.material_id,COALESCE(NULLIF(reservation.material_name,''),material.name),
		       COALESCE(NULLIF(reservation.unit,''),material.unit),binding.batch_code,binding.warehouse,
		       COALESCE(binding.owner_customer_id,0),
		       GREATEST(0,binding.reserved_g-binding.consumed_g-binding.returned_g)::bigint,
		       GREATEST(0,binding.reserved_units-binding.consumed_units-binding.returned_units)::bigint
		FROM %s.work_order_material_reservation_batches binding
		JOIN %s.work_order_material_reservations reservation ON reservation.id=binding.reservation_id
		JOIN %s.materials material ON material.id=reservation.material_id
		WHERE binding.work_order_id=$1 AND binding.status='reserved'
		  AND reservation.status='reserved' AND binding.component_type='material'
		  AND binding.material_batch_id>0 AND COALESCE(NULLIF(binding.warehouse,''),$2)<>$2
		  AND (binding.reserved_g>binding.consumed_g+binding.returned_g
		       OR binding.reserved_units>binding.consumed_units+binding.returned_units)
		ORDER BY reservation.id,binding.id
	`, r.schema, r.schema, r.schema), workOrderID, stockdomain.WarehouseWIP)
	if err != nil {
		return true, nil, err
	}
	defer rows.Close()
	items := make([]productionapp.StockEntryItemCommand, 0)
	for rows.Next() {
		var item productionapp.StockEntryItemCommand
		if err := rows.Scan(&item.MaterialID, &item.ItemName, &item.InventoryUnit, &item.BatchCode,
			&item.FromWarehouse, &item.OwnerCustomerID, &item.QtyG, &item.QtyUnits); err != nil {
			return true, nil, err
		}
		item.ItemType = "material"
		item.ToWarehouse = stockdomain.WarehouseWIP
		item.QuantityBasis = "count"
		item.DefaultQty = float64(item.QtyUnits)
		if item.QtyG > 0 {
			item.QuantityBasis = "weight"
			item.DefaultQty = productionInventoryQuantity(item.QtyG, strings.TrimSpace(item.InventoryUnit))
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return true, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return true, nil, err
	}
	return true, items, nil
}
