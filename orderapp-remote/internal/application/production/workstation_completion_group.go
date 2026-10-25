package production

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
)

type OperationCompletionGroupCommand struct {
	JobCardIDs       []int64                `json:"job_card_ids"`
	ActualOutputQty  float64                `json:"actual_output_qty"`
	OutputUnit       string                 `json:"output_unit"`
	ActualMinutes    int                    `json:"actual_minutes"`
	Materials        []OperationMaterialUse `json:"materials"`
	DifferenceReason string                 `json:"difference_reason"`
	FinishGroup      bool                   `json:"finish_group"`
	ExpectedVersion  string                 `json:"expected_version"`
	RequestID        string                 `json:"request_id"`

	EmployeeID           int64  `json:"-"`
	AssignedEmployeeName string `json:"-"`
	Operator             string `json:"-"`
}

type OperationCompletionGroupItem struct {
	JobCardID     int64   `json:"job_card_id"`
	WorkOrderID   int64   `json:"work_order_id"`
	WorkOrderNo   string  `json:"work_order_no"`
	ProductName   string  `json:"product_name"`
	SpecG         int64   `json:"spec_g"`
	PlannedQty    float64 `json:"planned_qty"`
	CompletedQty  float64 `json:"completed_qty"`
	RemainingQty  float64 `json:"remaining_qty"`
	Unit          string  `json:"unit"`
	Warehouse     string  `json:"warehouse"`
	WarehouseName string  `json:"warehouse_name"`
	Version       string  `json:"version"`
}

type OperationCompletionGroupMaterial struct {
	Key             string                 `json:"key"`
	MaterialID      int64                  `json:"material_id"`
	Name            string                 `json:"name"`
	Unit            string                 `json:"unit"`
	RequiredQty     float64                `json:"required_qty"`
	AvailableQty    float64                `json:"available_qty"`
	ShortageQty     float64                `json:"shortage_qty"`
	Warehouse       string                 `json:"warehouse"`
	WarehouseName   string                 `json:"warehouse_name"`
	OwnerCustomerID int64                  `json:"owner_customer_id"`
	Sources         []OperationMaterialUse `json:"sources"`
}

type OperationCompletionGroupPreview struct {
	GroupKey      string                             `json:"group_key"`
	Version       string                             `json:"version"`
	Items         []OperationCompletionGroupItem     `json:"items"`
	Materials     []OperationCompletionGroupMaterial `json:"materials"`
	PlannedQty    float64                            `json:"planned_qty"`
	CompletedQty  float64                            `json:"completed_qty"`
	RemainingQty  float64                            `json:"remaining_qty"`
	MaxOutputQty  float64                            `json:"max_output_qty"`
	OutputUnit    string                             `json:"output_unit"`
	WarehouseName string                             `json:"warehouse_name"`
	BlockedReason string                             `json:"blocked_reason"`
}

type OperationCompletionGroupResult struct {
	RequestedQty float64               `json:"requested_qty"`
	CompletedQty float64               `json:"completed_qty"`
	RemainingQty float64               `json:"remaining_qty"`
	OutputUnit   string                `json:"output_unit"`
	Items        []JobCardActionResult `json:"items"`
}

type operationCompletionGroupRepository interface {
	PreviewOperationCompletionGroup(context.Context, []int64) (OperationCompletionGroupPreview, error)
	CompleteOperationGroup(context.Context, OperationCompletionGroupCommand) (OperationCompletionGroupResult, error)
}

// OperationGroupMaxOutputQty returns the largest whole-group output supported
// by the common available balance of its frozen materials. Each material row
// already represents one inventory identity, so shared free stock must only
// be counted once before calling this helper.
func OperationGroupMaxOutputQty(remaining float64, outputUnit string, materials []OperationCompletionGroupMaterial) float64 {
	if remaining <= 0 || math.IsNaN(remaining) || math.IsInf(remaining, 0) {
		return 0
	}
	maxOutput := math.Inf(1)
	hasConstraint := false
	for _, material := range materials {
		if material.RequiredQty <= 0 {
			continue
		}
		hasConstraint = true
		perOutput := material.RequiredQty / remaining
		if perOutput <= 0 || math.IsNaN(perOutput) || math.IsInf(perOutput, 0) {
			continue
		}
		possible := math.Max(0, material.AvailableQty) / perOutput
		if OperationUnitFactor(outputUnit) == 0 {
			possible = math.Floor(possible + 1e-9)
		}
		maxOutput = math.Min(maxOutput, possible)
	}
	if !hasConstraint || math.IsInf(maxOutput, 1) {
		return remaining
	}
	return math.Max(0, maxOutput)
}

// AllocateOperationGroupMinutes distributes the one reported group duration
// in proportion to actual output, assigning rounding remainder to the last
// source that produced output. Zero-output sources receive no labor time.
func AllocateOperationGroupMinutes(outputByWorkOrder []float64, minutes int) ([]int, error) {
	if minutes < 0 || len(outputByWorkOrder) == 0 {
		return nil, fmt.Errorf("工时或来源工单数量无效")
	}
	total := 0.0
	lastActive := -1
	for i, qty := range outputByWorkOrder {
		if math.IsNaN(qty) || math.IsInf(qty, 0) || qty < 0 {
			return nil, fmt.Errorf("来源工单完成数量无效")
		}
		if qty > 0 {
			total += qty
			lastActive = i
		}
	}
	if minutes > 0 && total <= 0 {
		return nil, fmt.Errorf("没有产出时不能分配工时")
	}
	out := make([]int, len(outputByWorkOrder))
	assigned := 0
	for i, qty := range outputByWorkOrder {
		if qty <= 0 || total <= 0 {
			continue
		}
		out[i] = int(math.Floor(float64(minutes)*qty/total + 1e-9))
		assigned += out[i]
	}
	if lastActive >= 0 {
		out[lastActive] += minutes - assigned
	}
	return out, nil
}

func (s *Service) PreviewOperationCompletionGroup(ctx context.Context, ids []int64) (OperationCompletionGroupPreview, error) {
	ids, err := normalizeOperationGroupIDs(ids)
	if err != nil {
		return OperationCompletionGroupPreview{}, err
	}
	repo, ok := s.repo.(operationCompletionGroupRepository)
	if !ok {
		return OperationCompletionGroupPreview{}, fmt.Errorf("合并完成暂不可用")
	}
	return repo.PreviewOperationCompletionGroup(ctx, ids)
}

func (s *Service) CompleteOperationGroup(ctx context.Context, cmd OperationCompletionGroupCommand) (OperationCompletionGroupResult, error) {
	ids, err := normalizeOperationGroupIDs(cmd.JobCardIDs)
	if err != nil {
		return OperationCompletionGroupResult{}, err
	}
	cmd.JobCardIDs = ids
	cmd.RequestID = strings.TrimSpace(cmd.RequestID)
	cmd.ExpectedVersion = strings.TrimSpace(cmd.ExpectedVersion)
	cmd.OutputUnit = strings.TrimSpace(cmd.OutputUnit)
	cmd.DifferenceReason = strings.TrimSpace(cmd.DifferenceReason)
	if cmd.EmployeeID <= 0 {
		return OperationCompletionGroupResult{}, fmt.Errorf("请先绑定员工")
	}
	if cmd.RequestID == "" || cmd.ExpectedVersion == "" {
		return OperationCompletionGroupResult{}, fmt.Errorf("请刷新合并完成建议后提交")
	}
	if cmd.ActualMinutes < 0 {
		return OperationCompletionGroupResult{}, fmt.Errorf("实际工时不能小于 0")
	}
	if err := ValidateOperationQuantity(cmd.ActualOutputQty, cmd.OutputUnit); err != nil {
		return OperationCompletionGroupResult{}, err
	}
	if cmd.ActualOutputQty == 0 {
		return OperationCompletionGroupResult{}, fmt.Errorf("请填写本次实际完成袋数")
	}
	repo, ok := s.repo.(operationCompletionGroupRepository)
	if !ok {
		return OperationCompletionGroupResult{}, fmt.Errorf("合并完成暂不可用")
	}
	return repo.CompleteOperationGroup(ctx, cmd)
}

func normalizeOperationGroupIDs(ids []int64) ([]int64, error) {
	if len(ids) < 2 || len(ids) > 100 {
		return nil, fmt.Errorf("请选择 2 至 100 张同组包装工序卡")
	}
	out := append([]int64(nil), ids...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	for i, id := range out {
		if id <= 0 {
			return nil, fmt.Errorf("工序卡编号无效")
		}
		if i > 0 && id == out[i-1] {
			return nil, fmt.Errorf("工序卡不能重复选择")
		}
	}
	return out, nil
}

func AllocateOperationGroupOutput(remainingByWorkOrder []float64, actual float64) ([]float64, error) {
	if math.IsNaN(actual) || math.IsInf(actual, 0) || actual < 0 {
		return nil, fmt.Errorf("请填写有效的合并完成数量")
	}
	if len(remainingByWorkOrder) == 0 {
		return nil, fmt.Errorf("没有可执行的来源工单")
	}
	out := make([]float64, len(remainingByWorkOrder))
	remaining := actual
	for i, planned := range remainingByWorkOrder {
		planned = math.Max(0, planned)
		out[i] = math.Min(planned, remaining)
		remaining -= out[i]
	}
	if remaining > 1e-8 {
		out[len(out)-1] += remaining
	}
	return out, nil
}
