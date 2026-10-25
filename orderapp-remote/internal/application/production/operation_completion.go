package production

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

func (s *Service) PreviewOperationCompletion(ctx context.Context, id int64) (OperationCompletionPreview, error) {
	if repo, ok := s.repo.(interface {
		PreviewOperationCompletion(context.Context, int64) (OperationCompletionPreview, error)
	}); ok {
		return repo.PreviewOperationCompletion(ctx, id)
	}
	return OperationCompletionPreview{}, fmt.Errorf("工序自动记账暂不可用")
}

// OperationExecution is frozen with a route node, never resolved from live
// configuration while completing an existing work order.
type OperationExecution struct {
	Version          int    `json:"version"`
	NodeKey          string `json:"node_key"`
	ReceiptEnabled   bool   `json:"receipt_enabled"`
	ReceiptWarehouse string `json:"receipt_warehouse"`
	SourceWarehouse  string `json:"source_warehouse"`
	OutputUnit       string `json:"output_unit"`
}

type OperationCompletionInput struct {
	PlannedQty, ActualQty                                     float64
	Unit, TargetWarehouse, CurrentWarehouse, DifferenceReason string
	Final, ReceiptEnabled                                     bool
	FinishOperation                                           *bool
}

type OperationCompletionPlan struct {
	OutputQty, Difference    float64
	Warehouse                string
	CloseOperation, Saleable bool
}

func OperationUnitFactor(unit string) float64 {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "g", "克":
		return 1
	case "kg", "千克", "公斤":
		return 1000
	case "mg", "毫克":
		return .001
	}
	return 0
}

func ValidateOperationQuantity(qty float64, unit string) error {
	if strings.TrimSpace(unit) == "" || math.IsNaN(qty) || math.IsInf(qty, 0) || qty < 0 {
		return fmt.Errorf("请填写有效数量和单位")
	}
	if OperationUnitFactor(unit) == 0 && math.Abs(qty-math.Round(qty)) > 1e-8 {
		return fmt.Errorf("%s数必须是整数", unit)
	}
	return nil
}

func PlanOperationCompletion(in OperationCompletionInput) (OperationCompletionPlan, error) {
	var out OperationCompletionPlan
	if err := ValidateOperationQuantity(in.ActualQty, in.Unit); err != nil {
		return out, err
	}
	closeOperation := in.FinishOperation == nil || *in.FinishOperation
	if (in.ActualQty > in.PlannedQty+1e-8 || (closeOperation && math.Abs(in.ActualQty-in.PlannedQty) > 1e-8)) && strings.TrimSpace(in.DifferenceReason) == "" {
		return out, fmt.Errorf("请记录实际数量与计划不同的原因")
	}
	warehouse := strings.TrimSpace(in.CurrentWarehouse)
	if warehouse == "" {
		warehouse = "wip"
	}
	if in.ReceiptEnabled {
		warehouse = strings.TrimSpace(in.TargetWarehouse)
		if warehouse == "" {
			return out, fmt.Errorf("请配置完成后的入库位置")
		}
	}
	return OperationCompletionPlan{OutputQty: in.ActualQty, Difference: in.ActualQty - in.PlannedQty, Warehouse: warehouse, CloseOperation: closeOperation, Saleable: in.Final && in.ReceiptEnabled}, nil
}

type OperationBatchBalance struct {
	ID              int64   `json:"id"`
	WorkOrderID     int64   `json:"work_order_id"`
	JobCardID       int64   `json:"job_card_id"`
	OwnerCustomerID int64   `json:"owner_customer_id"`
	Remaining       float64 `json:"remaining"`
	Unit            string  `json:"unit"`
	Warehouse       string  `json:"warehouse"`
	WarehouseName   string  `json:"warehouse_name"`
	Operation       string  `json:"operation"`
	ProductName     string  `json:"product_name"`
	WorkOrderNo     string  `json:"work_order_no"`
	TotalCost       float64 `json:"total_cost"`
}

type OperationHandoffAllocation struct {
	BatchID   int64   `json:"batch_id"`
	Qty       float64 `json:"qty"`
	Unit      string  `json:"unit"`
	Warehouse string  `json:"warehouse"`
	Cost      float64 `json:"cost"`
}

// Caller supplies locked balances in source-event order. Quantities remain in
// each source's unit; only compatible physical dimensions may be converted.
func AllocateOperationHandoff(rows []OperationBatchBalance, qty float64, unit string) ([]OperationHandoffAllocation, error) {
	if err := ValidateOperationQuantity(qty, unit); err != nil {
		return nil, err
	}
	out := []OperationHandoffAllocation{}
	remaining := qty
	for _, row := range rows {
		if remaining <= 1e-8 {
			break
		}
		if row.Remaining <= 0 {
			continue
		}
		factor := 1.0
		if row.Unit != unit {
			a, b := OperationUnitFactor(row.Unit), OperationUnitFactor(unit)
			if a == 0 || b == 0 {
				return nil, fmt.Errorf("前序单位%s与投入单位%s不可换算", row.Unit, unit)
			}
			factor = a / b
		}
		take := math.Min(row.Remaining, remaining/factor)
		take = math.Round(take*1e8) / 1e8
		out = append(out, OperationHandoffAllocation{BatchID: row.ID, Qty: take, Unit: row.Unit, Warehouse: row.Warehouse, Cost: row.TotalCost * take / row.Remaining})
		remaining -= take * factor
	}
	if remaining > 1e-8 {
		return nil, fmt.Errorf("前序可用产出不足，还缺 %.4f %s", remaining, unit)
	}
	return out, nil
}

type OperationMaterialUse struct {
	Key             string  `json:"key"`
	ReservationID   int64   `json:"reservation_id"`
	MaterialID      int64   `json:"material_id"`
	ProductID       int64   `json:"product_id"`
	BomSpecID       int64   `json:"bom_spec_id"`
	BomVariantID    int64   `json:"bom_variant_id"`
	SpecG           int64   `json:"spec_g"`
	Name            string  `json:"name"`
	Unit            string  `json:"unit"`
	Qty             float64 `json:"qty"`
	TotalQty        float64 `json:"total_qty,omitempty"`
	ConsumedQty     float64 `json:"consumed_qty,omitempty"`
	AvailableQty    float64 `json:"available_qty,omitempty"`
	ShortageQty     float64 `json:"shortage_qty,omitempty"`
	Warehouse       string  `json:"warehouse"`
	WarehouseName   string  `json:"warehouse_name"`
	OwnerCustomerID int64   `json:"owner_customer_id"`
	NodeKey         string  `json:"node_key"`
}

type OperationCompletionPreview struct {
	ActualMinutes       int                     `json:"actual_minutes"`
	CompletedOutputQty  float64                 `json:"completed_output_qty"`
	MaxOutputQty        float64                 `json:"max_output_qty"`
	Draft               json.RawMessage         `json:"draft"`
	JobCardID           int64                   `json:"job_card_id"`
	WorkOrderID         int64                   `json:"work_order_id"`
	Version             string                  `json:"version"`
	Enabled             bool                    `json:"enabled"`
	Final               bool                    `json:"final"`
	PredecessorRequired bool                    `json:"predecessor_required"`
	Execution           OperationExecution      `json:"execution"`
	OutputUnit          string                  `json:"output_unit"`
	InputUnit           string                  `json:"input_unit"`
	PlannedQty          float64                 `json:"planned_qty"`
	TotalPlannedQty     float64                 `json:"total_planned_qty"`
	InputQty            float64                 `json:"input_qty"`
	ProductName         string                  `json:"product_name"`
	OutputSpecG         int64                   `json:"output_spec_g"`
	PackageSpecLabel    string                  `json:"package_spec_label"`
	Warehouse           string                  `json:"warehouse"`
	WarehouseName       string                  `json:"warehouse_name"`
	Materials           []OperationMaterialUse  `json:"materials"`
	PredecessorBatches  []OperationBatchBalance `json:"predecessor_batches"`
	BlockedReason       string                  `json:"blocked_reason"`
}

func (s *Service) SaveOperationProgress(ctx context.Context, cmd JobCardActionCommand) (OperationCompletionPreview, error) {
	if r, ok := s.repo.(interface {
		SaveOperationProgress(context.Context, JobCardActionCommand) (OperationCompletionPreview, error)
	}); ok {
		return r.SaveOperationProgress(ctx, cmd)
	}
	return OperationCompletionPreview{}, fmt.Errorf("保存进度暂不可用")
}

type OperationCompletionReceipt struct {
	OutputWeightG float64                      `json:"output_weight_g"`
	EventID       int64                        `json:"event_id"`
	OutputQty     float64                      `json:"output_qty"`
	OutputUnit    string                       `json:"output_unit"`
	Warehouse     string                       `json:"warehouse"`
	WarehouseName string                       `json:"warehouse_name"`
	StockEntryIDs []int64                      `json:"stock_entry_ids"`
	Handoffs      []OperationHandoffAllocation `json:"handoffs"`
	Message       string                       `json:"message"`
	EstimatedCost bool                         `json:"estimated_cost"`
}

// Frozen weights are numeric snapshots. Display labels are never parsed.
func OperationOutputWeight(qty float64, unit string, specG int64, plannedG int64, plannedQty float64) float64 {
	if factor := OperationUnitFactor(unit); factor > 0 {
		return qty * factor
	}
	if specG > 0 {
		return qty * float64(specG)
	}
	if plannedG > 0 && plannedQty > 0 {
		return qty * float64(plannedG) / plannedQty
	}
	return 0
}
