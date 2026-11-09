package production

import (
	"context"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/production"
)

// packagingInputComponentRequest is constructed from frozen BOM and verified
// actual weights, never from client-supplied batch or ownership identifiers.
type packagingInputComponentRequest struct {
	InputID    int64
	InputQty   float64
	Components map[string]float64
}

// allocatePackagingComponentsTx performs component-specific cost allocation.
// Callers must use the same transaction to return the remaining components,
// release reservations and post the optional transfer before completing a card.
func (r Repository) allocatePackagingComponentsTx(ctx context.Context, tx pgx.Tx, cardID, eventID int64, requests []packagingInputComponentRequest) (float64, error) {
	if len(requests) == 0 || cardID <= 0 || eventID <= 0 {
		return 0, fmt.Errorf("包装投入明细无效")
	}
	seen := map[int64]bool{}
	totalCost := 0.0
	for _, request := range requests {
		if request.InputID <= 0 || seen[request.InputID] || request.InputQty < 0 || math.IsNaN(request.InputQty) || math.IsInf(request.InputQty, 0) || math.Trunc(request.InputQty) != request.InputQty || (request.InputQty == 0 && len(request.Components) == 0) {
			return 0, fmt.Errorf("包装投入明细无效")
		}
		seen[request.InputID] = true
		var remain, cost float64
		err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT remaining_qty::float8,remaining_cost::float8 FROM %s.production_started_inputs WHERE id=$1 AND job_card_id=$2 AND status='posted' FOR UPDATE`, r.schema), request.InputID, cardID).Scan(&remain, &cost)
		if err != nil {
			return 0, fmt.Errorf("原包装投入不存在或已改变：%w", err)
		}
		if request.InputQty > remain+1e-8 {
			return 0, fmt.Errorf("包装处理件数超过已确认投入余额")
		}
		rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT id,component_key,remaining_qty::float8,remaining_cost::float8 FROM %s.production_started_input_components WHERE input_id=$1 ORDER BY id FOR UPDATE`, r.schema), request.InputID)
		if err != nil {
			return 0, err
		}
		var balances []app.PackagingComponentBalance
		for rows.Next() {
			var b app.PackagingComponentBalance
			if err = rows.Scan(&b.ID, &b.Key, &b.Quantity, &b.Cost); err != nil {
				rows.Close()
				return 0, err
			}
			balances = append(balances, b)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return 0, err
		}
		allocations, err := app.AllocatePackagingComponents(balances, request.Components)
		if err != nil {
			return 0, err
		}
		if len(allocations) == 0 && request.InputQty == 0 {
			return 0, fmt.Errorf("包装组件用量不能为空")
		}
		inputCost := 0.0
		for _, a := range allocations {
			if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_started_input_component_allocations(component_id,event_id,qty,cost) VALUES($1,$2,$3,$4)`, r.schema), a.ID, eventID, a.Quantity, a.Cost); err != nil {
				return 0, err
			}
			tag, e := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_started_input_components SET remaining_qty=remaining_qty-$2,remaining_cost=remaining_cost-$3 WHERE id=$1 AND remaining_qty>=$2 AND remaining_cost+0.000001>=$3`, r.schema), a.ID, a.Quantity, a.Cost)
			if e != nil {
				return 0, e
			}
			if tag.RowsAffected() != 1 {
				return 0, fmt.Errorf("包装组件余额已改变")
			}
			inputCost += a.Cost
		}
		inputCost = math.Round(inputCost*1e6) / 1e6
		if inputCost > cost+1e-6 {
			return 0, fmt.Errorf("包装组件成本超过投入余额")
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_started_input_allocations(input_id,event_id,qty,cost) VALUES($1,$2,$3,$4)`, r.schema), request.InputID, eventID, request.InputQty, inputCost); err != nil {
			return 0, err
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_started_inputs SET remaining_qty=remaining_qty-$2,remaining_cost=remaining_cost-$3 WHERE id=$1`, r.schema), request.InputID, request.InputQty, inputCost); err != nil {
			return 0, err
		}
		totalCost += inputCost
	}
	return math.Round(totalCost*1e6) / 1e6, nil
}
