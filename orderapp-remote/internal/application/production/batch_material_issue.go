package production

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

// Quantities in the read model are integral base quantities (grams or pieces).
// Only user-entered Quantity is expressed in the displayed inventory unit.
type BatchMaterialIssueCommand struct {
	JobCardIDs     []int64                       `json:"job_card_ids"`
	PreviewVersion string                        `json:"preview_version"`
	RequestID      string                        `json:"request_id"`
	Groups         []BatchMaterialIssueSelection `json:"groups"`
	Operator       string                        `json:"-"`
}
type BatchMaterialIssueSelection struct {
	Key      string  `json:"key"`
	Version  string  `json:"version"`
	Quantity float64 `json:"quantity"`
}
type BatchMaterialIssueAllocation struct {
	JobCardID      int64  `json:"job_card_id"`
	WorkOrderID    int64  `json:"work_order_id"`
	WorkOrderNo    string `json:"work_order_no"`
	ReservationID  int64  `json:"reservation_id"`
	BindingID      int64  `json:"binding_id"`
	BatchCode      string `json:"batch_code"`
	Required       int64  `json:"required"`
	WIP            int64  `json:"wip"`
	Remaining      int64  `json:"remaining"`
	Available      int64  `json:"available"`
	ExtraAvailable int64  `json:"extra_available"`
	Quantity       int64  `json:"quantity"`
	Extra          int64  `json:"extra"`
	Priority       int    `json:"priority"`
	PlannedStart   string `json:"planned_start"`
}
type BatchMaterialIssueGroup struct {
	Key             string                         `json:"key"`
	Version         string                         `json:"version"`
	ItemType        string                         `json:"item_type"`
	ItemID          int64                          `json:"item_id"`
	Name            string                         `json:"name"`
	BomSpecID       int64                          `json:"bom_spec_id"`
	BomVariantID    int64                          `json:"bom_variant_id"`
	SpecG           int64                          `json:"spec_g"`
	OwnerCustomerID int64                          `json:"owner_customer_id"`
	OwnerName       string                         `json:"owner_name"`
	Unit            string                         `json:"unit"`
	QuantityBasis   string                         `json:"quantity_basis"`
	FromWarehouse   string                         `json:"from_warehouse"`
	ToWarehouse     string                         `json:"to_warehouse"`
	Required        int64                          `json:"required"`
	WIP             int64                          `json:"wip"`
	Remaining       int64                          `json:"remaining"`
	Available       int64                          `json:"available"`
	MaxQuantity     int64                          `json:"max_quantity"`
	Reason          string                         `json:"reason,omitempty"`
	Allocations     []BatchMaterialIssueAllocation `json:"allocations"`
}
type BatchMaterialIssueBlocked struct {
	JobCardID   int64  `json:"job_card_id"`
	WorkOrderNo string `json:"work_order_no"`
	Reason      string `json:"reason"`
	DraftID     int64  `json:"draft_id,omitempty"`
}
type BatchMaterialIssuePreview struct {
	JobCardIDs []int64                     `json:"job_card_ids"`
	Version    string                      `json:"version"`
	Groups     []BatchMaterialIssueGroup   `json:"groups"`
	Blocked    []BatchMaterialIssueBlocked `json:"blocked"`
}
type BatchMaterialIssueDocument struct {
	ID          int64  `json:"id"`
	EntryNo     string `json:"entry_no"`
	WorkOrderID int64  `json:"work_order_id"`
	JobCardID   int64  `json:"job_card_id"`
}
type BatchMaterialIssueGroupResult struct {
	Key           string                       `json:"key"`
	Name          string                       `json:"name"`
	Unit          string                       `json:"unit"`
	QuantityBasis string                       `json:"quantity_basis"`
	Status        string                       `json:"status"`
	Quantity      int64                        `json:"quantity"`
	Extra         int64                        `json:"extra"`
	Remaining     int64                        `json:"remaining"`
	Error         string                       `json:"error,omitempty"`
	Documents     []BatchMaterialIssueDocument `json:"documents"`
}
type BatchMaterialIssueResult struct {
	RequestID    string                          `json:"request_id"`
	SuccessCount int                             `json:"success_count"`
	FailedCount  int                             `json:"failed_count"`
	Groups       []BatchMaterialIssueGroupResult `json:"groups"`
}

type batchMaterialIssueRepository interface {
	PreviewBatchMaterialIssue(context.Context, BatchMaterialIssueCommand) (BatchMaterialIssuePreview, error)
	SubmitBatchMaterialIssue(context.Context, BatchMaterialIssueCommand) (BatchMaterialIssueResult, error)
}

func normalizeBatchMaterialIssue(cmd BatchMaterialIssueCommand, submit bool) (BatchMaterialIssueCommand, error) {
	if len(cmd.JobCardIDs) == 0 || len(cmd.JobCardIDs) > 500 {
		return cmd, fmt.Errorf("请选择 1 至 500 个工位任务")
	}
	seen := map[int64]bool{}
	ids := []int64{}
	for _, id := range cmd.JobCardIDs {
		if id <= 0 {
			return cmd, fmt.Errorf("无效任务")
		}
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	cmd.JobCardIDs = ids
	if submit {
		cmd.RequestID = strings.TrimSpace(cmd.RequestID)
		if cmd.RequestID == "" || len(cmd.RequestID) > 128 || len(cmd.Groups) == 0 || len(cmd.Groups) > 1000 {
			return cmd, fmt.Errorf("请求标识和所选物料不能为空")
		}
		keys := map[string]bool{}
		for _, g := range cmd.Groups {
			if g.Key == "" || g.Version == "" || keys[g.Key] || math.IsNaN(g.Quantity) || math.IsInf(g.Quantity, 0) || g.Quantity <= 0 || g.Quantity > 1e12 {
				return cmd, fmt.Errorf("物料、版本或实际领用数量无效")
			}
			keys[g.Key] = true
		}
		sort.Slice(cmd.Groups, func(i, j int) bool { return cmd.Groups[i].Key < cmd.Groups[j].Key })
	}
	return cmd, nil
}
func (s *Service) PreviewBatchMaterialIssue(ctx context.Context, cmd BatchMaterialIssueCommand) (BatchMaterialIssuePreview, error) {
	cmd, err := normalizeBatchMaterialIssue(cmd, false)
	if err != nil {
		return BatchMaterialIssuePreview{}, err
	}
	repo, ok := s.repo.(batchMaterialIssueRepository)
	if !ok {
		return BatchMaterialIssuePreview{}, fmt.Errorf("批量领料暂不可用")
	}
	return repo.PreviewBatchMaterialIssue(ctx, cmd)
}
func (s *Service) SubmitBatchMaterialIssue(ctx context.Context, cmd BatchMaterialIssueCommand) (BatchMaterialIssueResult, error) {
	cmd, err := normalizeBatchMaterialIssue(cmd, true)
	if err != nil {
		return BatchMaterialIssueResult{}, err
	}
	repo, ok := s.repo.(batchMaterialIssueRepository)
	if !ok {
		return BatchMaterialIssueResult{}, fmt.Errorf("批量领料暂不可用")
	}
	return repo.SubmitBatchMaterialIssue(ctx, cmd)
}
func BatchMaterialIssueHash(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func BatchMaterialIssueBaseQuantity(g BatchMaterialIssueGroup, quantity float64) (int64, error) {
	factor := float64(1)
	if g.QuantityBasis == "weight" {
		switch strings.ToLower(g.Unit) {
		case "kg", "公斤", "千克":
			factor = 1000
		case "吨", "t":
			factor = 1000000
		case "斤":
			factor = 500
		}
	}
	v := quantity * factor
	if math.Abs(v-math.Round(v)) > 0.00001 || v <= 0 || v > 1e15 {
		return 0, fmt.Errorf("数量须精确到克或整数件")
	}
	return int64(math.Round(v)), nil
}
func AllocateBatchMaterialIssue(g BatchMaterialIssueGroup, quantity int64) ([]BatchMaterialIssueAllocation, error) {
	if quantity <= 0 || quantity > g.MaxQuantity {
		return nil, fmt.Errorf("实际数量超过当前可领库存，请刷新领料建议")
	}
	rows := append([]BatchMaterialIssueAllocation(nil), g.Allocations...)
	left := quantity
	for i := range rows {
		rows[i].Quantity = min(left, rows[i].Available)
		rows[i].Extra = 0
		left -= rows[i].Quantity
	}
	for i := range rows {
		extra := min(left, rows[i].ExtraAvailable)
		rows[i].Quantity += extra
		rows[i].Extra = extra
		left -= extra
	}
	if left > 0 {
		return nil, fmt.Errorf("当前可领库存不足，请刷新领料建议")
	}
	return rows, nil
}
