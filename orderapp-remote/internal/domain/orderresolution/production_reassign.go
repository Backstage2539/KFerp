package orderresolution

import "fmt"

type ReassignWorkOrder struct {
	WorkOrderID  int64                `json:"work_order_id"`
	SourceItemID int64                `json:"source_item_id"`
	Quantity     int64                `json:"quantity"`
	Unit         string               `json:"unit"`
	Label        string               `json:"label"`
	Version      string               `json:"version"`
	Issue        string               `json:"issue,omitempty"`
	Targets      []CompanyStockTarget `json:"targets"`
}
type ReassignSelection struct {
	WorkOrderID  int64 `json:"work_order_id"`
	TargetItemID int64 `json:"target_item_id"`
	Quantity     int64 `json:"quantity"`
}

func validateProductionReassign(d Dependency, choice Disposition, cmd Command, effects []LineEffect) (string, error) {
	if cmd.Action != "close" || cmd.Scope != "all" || d.Shared || d.OwnershipIssue != "" || oneOf(d.Status, "completed", "cancelled", "stopped") {
		return "", fmt.Errorf("转供须关闭本单全部剩余履约，并核对非共享、公司货权的未完成生产")
	}
	if len(d.ReassignWorkOrders) != 1 || len(choice.Reassignments) != 1 {
		return "", fmt.Errorf("请明确选择可完整转供的原工单、目标商品行和数量；多批生产请先拆分核对")
	}
	w, s := d.ReassignWorkOrders[0], choice.Reassignments[0]
	if w.Issue != "" {
		return "", fmt.Errorf("%s", w.Issue)
	}
	if s.WorkOrderID != w.WorkOrderID || w.Quantity <= 0 || s.Quantity != w.Quantity {
		return "", fmt.Errorf("转供数量须等于所选工单的全部未产出数量，不能隐式拆分或超量")
	}
	closed := float64(0)
	for _, l := range effects {
		if l.ItemID == w.SourceItemID {
			closed = l.ClosedDelta
		}
	}
	if closed < float64(s.Quantity) {
		return "", fmt.Errorf("本次关闭数量不足以释放所选生产需求")
	}
	for _, t := range w.Targets {
		if t.OrderItemID == s.TargetItemID && s.TargetItemID > 0 && t.RemainingQuantity == s.Quantity {
			return fmt.Sprintf("；%s 转供 %s，%d %s；保留原工单、投料、在制品和成本", w.Label, t.Label, s.Quantity, w.Unit), nil
		}
	}
	return "", fmt.Errorf("目标需求的商品、冻结规格、单位、仓库或未履约数量不完全匹配，请先拆分核对目标需求")
}
