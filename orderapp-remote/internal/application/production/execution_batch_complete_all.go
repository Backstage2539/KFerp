package production

import (
	"context"
	"fmt"
	"math"
	"strings"
)

type ExecutionBatchCompleteItem struct {
	ExecutionBatchID int64    `json:"execution_batch_id"`
	ActualInputQty   *float64 `json:"actual_input_qty"`
	ActualOutputQty  *float64 `json:"actual_output_qty"`
	ActualMinutes    *int     `json:"actual_minutes"`
	DifferenceReason string   `json:"difference_reason"`
	InputUnit        string   `json:"input_unit"`
	OutputUnit       string   `json:"output_unit"`
}
type ExecutionBatchCompleteAllCommand struct {
	ID              int64                        `json:"-"`
	EmployeeID      int64                        `json:"-"`
	Operator        string                       `json:"-"`
	ExpectedVersion string                       `json:"expected_version"`
	RequestID       string                       `json:"request_id"`
	Batches         []ExecutionBatchCompleteItem `json:"batches"`
}
type ExecutionBatchCompleteAllResult struct {
	Items []JobCardActionResult `json:"items"`
}

func (s *Service) CompleteExecutionBatches(ctx context.Context, cmd ExecutionBatchCompleteAllCommand) (ExecutionBatchCompleteAllResult, error) {
	if cmd.ID <= 0 || cmd.EmployeeID <= 0 || strings.TrimSpace(cmd.RequestID) == "" || strings.TrimSpace(cmd.ExpectedVersion) == "" || len(cmd.Batches) == 0 || len(cmd.Batches) > 10000 {
		return ExecutionBatchCompleteAllResult{}, fmt.Errorf("请刷新任务并填写全部待完成批次")
	}
	seen := map[int64]bool{}
	for _, b := range cmd.Batches {
		if b.ExecutionBatchID <= 0 || seen[b.ExecutionBatchID] || b.ActualInputQty == nil || b.ActualOutputQty == nil || b.ActualMinutes == nil || *b.ActualMinutes < 0 || math.IsNaN(*b.ActualInputQty) || math.IsInf(*b.ActualInputQty, 0) || *b.ActualInputQty < 0 || math.IsNaN(*b.ActualOutputQty) || math.IsInf(*b.ActualOutputQty, 0) || *b.ActualOutputQty < 0 {
			return ExecutionBatchCompleteAllResult{}, fmt.Errorf("请核对批次投入、产出和工时")
		}
		seen[b.ExecutionBatchID] = true
	}
	repo, ok := s.repo.(interface {
		CompleteExecutionBatches(context.Context, ExecutionBatchCompleteAllCommand) (ExecutionBatchCompleteAllResult, error)
	})
	if !ok {
		return ExecutionBatchCompleteAllResult{}, fmt.Errorf("分批完成不可用")
	}
	return repo.CompleteExecutionBatches(ctx, cmd)
}
