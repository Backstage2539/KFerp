package sales

import (
	"context"
	"encoding/json"
	"fmt"
	"orderapp/internal/infrastructure/postgres/orderweight"

	"github.com/jackc/pgx/v5"
)

// SaveOrder rebuilds ordinary lines. Carry snapshots only through the submitted
// original line ID and unchanged product/spec identity, never by SKU alone.
type frozenOrderItemWeight struct {
	ProductID, BomSpecID, BomVariantID                       int64
	Spec, Unit                                               string
	MaterialUnit, MaterialItem, LogisticsUnit, LogisticsItem *string
	Snapshot                                                 string
}

func captureOrderItemWeightsTx(ctx context.Context, tx pgx.Tx, schema string, orderID int64) (map[int64]frozenOrderItemWeight, error) {
	if err := orderweight.MarkHistoricalUnknown(ctx, tx, schema, orderID); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT id,COALESCE(product_id,0),COALESCE(bom_spec_id,0),COALESCE(bom_variant_id,0),COALESCE(spec,''),COALESCE(unit,''),to_jsonb(i)->>'material_unit_weight_g',to_jsonb(i)->>'material_item_weight_g',to_jsonb(i)->>'logistics_unit_weight_g',to_jsonb(i)->>'logistics_item_weight_g',COALESCE(to_jsonb(i)->>'weight_snapshot_json','{}') FROM %s.order_items i WHERE order_id=$1`, schema), orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]frozenOrderItemWeight{}
	for rows.Next() {
		var id int64
		var w frozenOrderItemWeight
		if err = rows.Scan(&id, &w.ProductID, &w.BomSpecID, &w.BomVariantID, &w.Spec, &w.Unit, &w.MaterialUnit, &w.MaterialItem, &w.LogisticsUnit, &w.LogisticsItem, &w.Snapshot); err != nil {
			return nil, err
		}

		out[id] = w
	}
	return out, rows.Err()
}

func (w frozenOrderItemWeight) matches(productID, bomSpecID, bomVariantID int64, spec, unit string) bool {
	return w.ProductID == productID && w.BomSpecID == bomSpecID && (bomSpecID > 0 || (w.BomVariantID == bomVariantID && w.Spec == spec && w.Unit == unit))
}

func restoreOrderItemWeightTx(ctx context.Context, tx pgx.Tx, schema string, orderID int64, lineNo int, weight frozenOrderItemWeight) error {
	if !json.Valid([]byte(weight.Snapshot)) {
		return fmt.Errorf("invalid frozen order weight snapshot")
	}
	_, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_items SET material_unit_weight_g=$3::numeric,material_item_weight_g=$4::numeric,logistics_unit_weight_g=$5::numeric,logistics_item_weight_g=$6::numeric,weight_snapshot_json=$7::jsonb WHERE order_id=$1 AND line_no=$2`, schema), orderID, lineNo, weight.MaterialUnit, weight.MaterialItem, weight.LogisticsUnit, weight.LogisticsItem, weight.Snapshot)
	return err
}
