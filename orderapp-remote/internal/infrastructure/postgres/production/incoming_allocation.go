package production

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/production"
)

type incomingAllocation struct {
	JobCardID   int64   `json:"job_card_id"`
	Workstation string  `json:"workstation"`
	Quantity    float64 `json:"quantity"`
	Unit        string  `json:"unit"`
	Frozen      bool    `json:"frozen"`
	Status      string  `json:"-"`
	Weight      float64 `json:"-"`
}

// Incoming allocations are computed over an entire route step, never over the
// residual for just one card. The work-order row serializes freeze and start.
func (r Repository) incomingAllocationsTx(ctx context.Context, tx pgx.Tx, p app.OperationCompletionPreview, seq int, lock bool) ([]incomingAllocation, error) {
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT id,workstation,status,COALESCE(planned_input_qty,0)::float8,COALESCE(metrics_json->'incoming_allocation','null'::jsonb)::text FROM %s.job_cards WHERE work_order_id=$1 AND sequence_no=$2 AND status<>'cancelled' ORDER BY id`+suffix, r.schema), p.WorkOrderID, seq)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	allocations := []incomingAllocation{}
	frozen := 0
	for rows.Next() {
		var a incomingAllocation
		var raw string
		if err = rows.Scan(&a.JobCardID, &a.Workstation, &a.Status, &a.Weight, &raw); err != nil {
			return nil, err
		}
		if raw != "null" {
			var saved incomingAllocation
			if json.Unmarshal([]byte(raw), &saved) != nil || saved.Unit == "" {
				return nil, fmt.Errorf("色选来料分配快照无效，请联系管理员")
			}
			a.Quantity = saved.Quantity
			a.Unit = saved.Unit
			a.Frozen = true
			frozen++
		}
		allocations = append(allocations, a)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	if frozen > 0 {
		if frozen != len(allocations) {
			return nil, fmt.Errorf("来料分配不完整，请核对未开工任务")
		}
		currentFactor := app.OperationUnitFactor(p.InputUnit)
		if currentFactor <= 0 {
			return nil, fmt.Errorf("来料单位不可换算")
		}
		for i := range allocations {
			savedFactor := app.OperationUnitFactor(allocations[i].Unit)
			if savedFactor <= 0 {
				return nil, fmt.Errorf("来料单位不可换算")
			}
			allocations[i].Quantity = math.Round(allocations[i].Quantity*savedFactor/currentFactor*1e8) / 1e8
			allocations[i].Unit = p.InputUnit
		}
		return allocations, nil
	}
	weights := make([]float64, len(allocations))
	for i, a := range allocations {
		if a.Status != "pending" && a.Status != "ready" && a.Status != "released" {
			return nil, fmt.Errorf("已有任务开工，请按原规则处理并核对来料分配")
		}
		weights[i] = a.Weight
	}
	factor := app.OperationUnitFactor(p.InputUnit)
	if factor <= 0 {
		return nil, fmt.Errorf("来料加工仅支持重量单位")
	}
	totalGrams := 0.0
	for _, b := range p.PredecessorBatches {
		f := app.OperationUnitFactor(b.Unit)
		if f <= 0 {
			return nil, fmt.Errorf("前序来料单位不可换算")
		}
		totalGrams += b.Remaining * f
	}
	if math.Abs(totalGrams-math.Round(totalGrams)) > 1e-6 {
		return nil, fmt.Errorf("前序来料不足整克，请核对库存精度")
	}
	amounts, err := app.AllocateIncomingQuantities(int64(math.Round(totalGrams)), weights)
	if err != nil {
		return nil, err
	}
	for i := range allocations {
		allocations[i].Quantity = float64(amounts[i]) / factor
		allocations[i].Unit = p.InputUnit
	}
	return allocations, nil
}

func (r Repository) freezeIncomingAllocationsTx(ctx context.Context, tx pgx.Tx, allocations []incomingAllocation) error {
	for _, a := range allocations {
		if a.Frozen {
			continue
		}
		raw, err := json.Marshal(a)
		if err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.job_cards SET metrics_json=COALESCE(metrics_json,'{}'::jsonb)||jsonb_build_object('incoming_allocation',$2::jsonb) WHERE id=$1 AND status IN ('pending','ready','released')`, r.schema), a.JobCardID, string(raw))
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return fmt.Errorf("来料任务已变化，请刷新")
		}
	}
	return nil
}
