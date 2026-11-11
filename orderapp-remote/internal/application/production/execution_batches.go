package production

import (
	"context"
	"fmt"
	"math"
	"strings"
)

type ExecutionBatch struct {
	ID              int64    `json:"id"`
	BatchNo         int      `json:"batch_no"`
	PlannedInputQty float64  `json:"planned_input_qty"`
	InputUnit       string   `json:"input_unit"`
	ActualInputQty  *float64 `json:"actual_input_qty"`
	ActualOutputQty *float64 `json:"actual_output_qty"`
	Status          string   `json:"status"`
}
type ExecutionBatchList struct {
	Batches       []ExecutionBatch `json:"batches"`
	CanInitialize bool             `json:"can_initialize"`
	Reason        string           `json:"reason"`
	Version       string           `json:"version"`
}
type ExecutionBatchInitCommand struct {
	ID              int64  `json:"-"`
	ExpectedVersion string `json:"expected_version"`
	RequestID       string `json:"request_id"`
	Operator        string `json:"-"`
	EmployeeID      int64  `json:"-"`
}

func ExecutionBatchPlan(total, size float64, count int) ([]float64, error) {
	if total <= 0 || size <= 0 || count <= 0 || count > 10000 || math.IsNaN(total) || math.IsNaN(size) || math.IsInf(total, 0) || math.IsInf(size, 0) {
		return nil, fmt.Errorf("批次安排无效")
	}
	total = math.Round(total*1e6) / 1e6
	size = math.Round(size*1e6) / 1e6
	if total > size*float64(count)+1e-8 || total <= size*float64(count-1)+1e-8 {
		return nil, fmt.Errorf("批次数与安排投入不一致，请先核对拆分")
	}
	out := make([]float64, count)
	remaining := total
	for i := range out {
		n := math.Min(size, remaining)
		out[i] = math.Round(n*1e6) / 1e6
		remaining = math.Round((remaining-out[i])*1e6) / 1e6
	}
	return out, nil
}
func ValidateExecutionBatchCompletion(output *float64, reason string) error {
	if output == nil || math.IsNaN(*output) || math.IsInf(*output, 0) || *output < 0 {
		return fmt.Errorf("请填写本批实际产出")
	}
	if *output == 0 && strings.TrimSpace(reason) == "" {
		return fmt.Errorf("零产出请填写原因")
	}
	return nil
}
func (s *Service) ListExecutionBatches(ctx context.Context, id int64) (ExecutionBatchList, error) {
	if r, ok := s.repo.(interface {
		ListExecutionBatches(context.Context, int64) (ExecutionBatchList, error)
	}); ok {
		return r.ListExecutionBatches(ctx, id)
	}
	return ExecutionBatchList{}, fmt.Errorf("分批报工不可用")
}
func (s *Service) InitializeExecutionBatches(ctx context.Context, c ExecutionBatchInitCommand) (ExecutionBatchList, error) {
	if r, ok := s.repo.(interface {
		InitializeExecutionBatches(context.Context, ExecutionBatchInitCommand) (ExecutionBatchList, error)
	}); ok {
		return r.InitializeExecutionBatches(ctx, c)
	}
	return ExecutionBatchList{}, fmt.Errorf("分批报工不可用")
}
