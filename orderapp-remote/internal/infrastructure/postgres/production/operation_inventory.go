package production

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
)

func (r Repository) upsertOperationInventoryTx(ctx context.Context, tx pgx.Tx, product, spec, variant, grams int64, warehouse string, units, loose, owner int64) error {
	tag, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.finished_inventory(product_id,bom_spec_id,bom_variant_id,spec_g,warehouse,owner_customer_id,onhand_units,onhand_loose_g,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,now()) ON CONFLICT(product_id,bom_spec_id,spec_g,warehouse) DO UPDATE SET bom_variant_id=EXCLUDED.bom_variant_id,onhand_units=EXCLUDED.onhand_units,onhand_loose_g=EXCLUDED.onhand_loose_g,updated_at=now() WHERE finished_inventory.owner_customer_id=EXCLUDED.owner_customer_id`, r.schema), product, spec, variant, grams, warehouse, owner, units, loose)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("目标仓已有其他货主的同规格库存，请核对仓库与货主")
	}
	return nil
}
