package production

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	productionapp "orderapp/internal/application/production"
)

// Required excludes this plan's upstream production; only existing stock must
// be physically available in WIP before work orders can be issued.
func requirePlanMaterialsInWIP(sources []productionapp.ProductionPlanComponentSource) error {
	for _, source := range sources {
		missingG := nonnegativeQuantity(source.RequiredG - source.WIPCoveredG)
		missingUnits := nonnegativeQuantity(source.RequiredUnits - source.WIPCoveredUnits)
		if missingG > 0 || missingUnits > 0 {
			return fmt.Errorf("物料「%s」尚有 %dg / %d件未进入WIP，请先由仓库确认调拨", source.ComponentName, missingG, missingUnits)
		}
	}
	return nil
}

func requireNoPendingPlanTransfersTx(ctx context.Context, tx pgx.Tx, schema string, planID int64) error {
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass(format('%I.%I',$1::text,$2::text)) IS NOT NULL`, schema, "production_plan_material_transfer_links").Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return nil
	}
	var pending bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS (SELECT 1 FROM %s.production_plan_material_transfer_links l JOIN %s.stock_entries e ON e.id=l.stock_entry_id WHERE l.production_plan_id=$1 AND e.status='draft')`, schema, schema), planID).Scan(&pending); err != nil {
		return err
	}
	if pending {
		return fmt.Errorf("仍有调拨单待仓库确认，请先完成或取消后再生成工单")
	}
	return nil
}
