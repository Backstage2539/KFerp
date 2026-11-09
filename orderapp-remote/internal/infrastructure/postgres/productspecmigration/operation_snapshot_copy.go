package productspecmigration

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
)

// targetVariantID is nonzero only when a legacy single-output BOM is converted
// to one explicit variant. A zero target preserves version-level rows and maps
// every source variant to its target variant by immutable specification ID.
func copyFrozenOperationCostsTx(ctx context.Context, tx pgx.Tx, schema string, targetVersionID, sourceVersionID, targetVariantID int64) error {
	columns := []string{"variant_id", "cost_unavailable", "scheduling_mode_snapshot", "throughput_per_hour_snapshot"}
	complete, err := tableHasColumnsTx(ctx, tx, schema, "production_bom_version_operation_costs", columns...)
	if err != nil {
		return err
	}
	if !complete {
		var any bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=$1 AND table_name='production_bom_version_operation_costs' AND column_name=ANY($2))`, schema, columns).Scan(&any); err != nil {
			return err
		}
		if any {
			return fmt.Errorf("工序成本快照字段不完整，无法安全复制")
		}
		_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.production_bom_version_operation_costs(version_id,operation_id,operation_name,workstation_id,workstation_name,workstation_capacity_id,capacity_name,hourly_rate_snapshot,standard_minutes_snapshot,batch_size_qty_snapshot,batch_size_unit_snapshot,cost_method,piece_rate_snapshot,rate_unit_snapshot,operation_unit_cost,operation_cost_unit,sort_order)
 SELECT $1,operation_id,operation_name,workstation_id,workstation_name,workstation_capacity_id,capacity_name,hourly_rate_snapshot,standard_minutes_snapshot,batch_size_qty_snapshot,batch_size_unit_snapshot,cost_method,piece_rate_snapshot,rate_unit_snapshot,operation_unit_cost,operation_cost_unit,sort_order FROM %[1]s.production_bom_version_operation_costs WHERE version_id=$2`, schema), targetVersionID, sourceVersionID)
		return err
	}
	if targetVariantID > 0 {
		var hasVariant bool
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.production_bom_version_operation_costs WHERE version_id=$1 AND variant_id<>0)`, schema), sourceVersionID).Scan(&hasVariant); err != nil {
			return err
		}
		if hasVariant {
			return fmt.Errorf("单产出 BOM 含规格工序快照，无法安全转换为单一规格")
		}
	} else {
		var missing bool
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.production_bom_version_operation_costs source LEFT JOIN %[1]s.production_bom_version_variants sv ON sv.id=source.variant_id AND sv.version_id=$2 LEFT JOIN %[1]s.production_bom_version_variants tv ON tv.version_id=$1 AND tv.bom_spec_id=sv.bom_spec_id WHERE source.version_id=$2 AND source.variant_id<>0 AND tv.id IS NULL)`, schema), targetVersionID, sourceVersionID).Scan(&missing); err != nil {
			return err
		}
		if missing {
			return fmt.Errorf("规格工序成本快照缺少对应目标规格，无法安全复制")
		}
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.production_bom_version_operation_costs(version_id,variant_id,operation_id,operation_name,workstation_id,workstation_name,workstation_capacity_id,capacity_name,hourly_rate_snapshot,standard_minutes_snapshot,batch_size_qty_snapshot,batch_size_unit_snapshot,cost_method,piece_rate_snapshot,rate_unit_snapshot,operation_unit_cost,operation_cost_unit,sort_order,cost_unavailable,scheduling_mode_snapshot,throughput_per_hour_snapshot)
 SELECT $1,CASE WHEN $3>0 THEN $3 WHEN source.variant_id=0 THEN 0 ELSE target_variant.id END,source.operation_id,source.operation_name,source.workstation_id,source.workstation_name,source.workstation_capacity_id,source.capacity_name,source.hourly_rate_snapshot,source.standard_minutes_snapshot,source.batch_size_qty_snapshot,source.batch_size_unit_snapshot,source.cost_method,source.piece_rate_snapshot,source.rate_unit_snapshot,source.operation_unit_cost,source.operation_cost_unit,source.sort_order,source.cost_unavailable,source.scheduling_mode_snapshot,source.throughput_per_hour_snapshot
 FROM %[1]s.production_bom_version_operation_costs source LEFT JOIN %[1]s.production_bom_version_variants source_variant ON source_variant.id=source.variant_id AND source_variant.version_id=$2 LEFT JOIN %[1]s.production_bom_version_variants target_variant ON target_variant.version_id=$1 AND target_variant.bom_spec_id=source_variant.bom_spec_id WHERE source.version_id=$2`, schema), targetVersionID, sourceVersionID, targetVariantID)
	return err
}
