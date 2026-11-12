package production

import (
	"context"
	"fmt"
)

// GetWorkOrderStartedMaterialCost returns net posted component cost when every
// source batch has an available cost. It includes allocated and in-process
// input, subtracts documented returns, and never treats missing costs as zero.
func (r Repository) GetWorkOrderStartedMaterialCost(ctx context.Context, id int64) (*float64, error) {
	var amount *float64
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
WITH inputs AS (
 SELECT id,material_cost FROM %[1]s.production_started_inputs WHERE work_order_id=$1 AND status='posted'
), components AS (
 SELECT i.id AS input_id,i.material_cost AS input_cost,c.id,c.cost,
   CASE WHEN c.material_batch_id>0 THEN mb.id IS NULL OR COALESCE(mb.cost_unavailable,false)
        WHEN c.stock_batch_id>0 THEN sb.id IS NULL OR COALESCE(sb.cost_unavailable,false)
        ELSE true END AS unavailable,
   COALESCE((SELECT SUM(x.cost) FROM %[1]s.production_started_input_returns x WHERE x.component_id=c.id),0) AS returned_cost
 FROM inputs i LEFT JOIN %[1]s.production_started_input_components c ON c.input_id=i.id
 LEFT JOIN %[1]s.material_batches mb ON mb.id=c.material_batch_id
 LEFT JOIN %[1]s.stock_batches sb ON sb.id=c.stock_batch_id
)
, input_costs AS (
 SELECT input_id, MAX(input_cost) AS input_cost, SUM(cost) AS component_cost,
   SUM(returned_cost) AS returned_cost,
   bool_or(id IS NULL OR unavailable OR returned_cost>cost) AS unavailable
 FROM components GROUP BY input_id
)
SELECT CASE WHEN COUNT(*)=0 OR bool_or(unavailable OR input_cost<>component_cost) THEN NULL
            ELSE SUM(component_cost-returned_cost)::float8 END FROM input_costs`, r.schema), id).Scan(&amount)
	return amount, err
}
