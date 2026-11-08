package orderresolution

import (
	"fmt"
	"strings"
)

type CompanyStockTarget struct {
	Version           string `json:"version"`
	CustomerID        int64  `json:"customer_id"`
	OrderID           int64  `json:"order_id"`
	OrderItemID       int64  `json:"order_item_id"`
	Label             string `json:"label"`
	RemainingQuantity int64  `json:"remaining_quantity"`
}
type CompanyStockRelease struct {
	ID       int64 `json:"id"`
	Quantity int64 `json:"quantity"`
}
type CompanyStockSource struct {
	UnitCostMicros     int64                 `json:"unit_cost_micros"`
	CostRecordID       int64                 `json:"cost_record_id"`
	CostDate           string                `json:"cost_date"`
	CostMonth          string                `json:"cost_month"`
	ScrapIssue         string                `json:"scrap_issue,omitempty"`
	Version            string                `json:"version"`
	FreeQuantity       int64                 `json:"free_quantity"`
	UnreservedQuantity int64                 `json:"unreserved_quantity"`
	Releases           []CompanyStockRelease `json:"releases,omitempty"`
	BatchID            int64                 `json:"batch_id"`
	SourceItemID       int64                 `json:"source_item_id"`
	Label              string                `json:"label"`
	AvailableQuantity  int64                 `json:"available_quantity"`
	CloseLimit         int64                 `json:"close_limit"`
	Targets            []CompanyStockTarget  `json:"targets"`
}
type StockSelection struct {
	BatchID      int64 `json:"batch_id"`
	TargetItemID int64 `json:"target_item_id"`
	Quantity     int64 `json:"quantity"`
}

type companyAllocationUsage struct {
	batches, targets, items map[int64]int64
	seen                    map[string]bool
}

func validateCompanyAllocation(d Dependency, choice Disposition, effects []LineEffect, usage *companyAllocationUsage, dependencies []Dependency, dispositions []Disposition) (string, error) {
	scrap := choice.Choice == "scrap"
	if scrap && (!oneOf(choice.CostTreatment, "retain_cost", "reclassify_loss") || strings.TrimSpace(choice.Reference) == "") {
		return "", fmt.Errorf("请选择报损成本处理方式，并填写实际报损原因或验收依据")
	}
	if d.OwnershipIssue != "" {
		return "", fmt.Errorf("%s", d.OwnershipIssue)
	}
	if d.Status != "completed" || len(choice.StockSelections) == 0 {
		return "", fmt.Errorf("请选择已完成公司成品的原批次、目标订单和分配数量")
	}
	batches, targets, items := usage.batches, usage.targets, usage.items
	seen := usage.seen
	label := ""
	for _, selected := range choice.StockSelections {
		key := fmt.Sprintf("%d:%d", selected.BatchID, selected.TargetItemID)
		if seen[key] || selected.Quantity <= 0 {
			return "", fmt.Errorf("成品分配数量未填写或批次与目标重复")
		}
		seen[key] = true
		var source CompanyStockSource
		var target CompanyStockTarget
		for _, s := range d.CompanyStock {
			if s.BatchID == selected.BatchID {
				source = s
				break
			}
		}
		for _, t := range source.Targets {
			if t.OrderItemID == selected.TargetItemID {
				target = t
				break
			}
		}
		if source.BatchID == 0 || (!scrap && target.OrderID == 0) {
			return "", fmt.Errorf("所选原批次或目标订单的商品、冻结规格、仓库或货权不匹配")
		}
		if scrap && (selected.TargetItemID != 0 || source.CostRecordID <= 0 || source.ScrapIssue != "") {
			return "", fmt.Errorf("报损须核对原批次及已确认成本，不得指定转供目标：%s", source.ScrapIssue)
		}
		available := source.AvailableQuantity
		var releasedOnBatch, releasedForItem int64
		for _, disposition := range dispositions {
			if disposition.Kind != "stock_reservation" || disposition.Choice != "release" {
				continue
			}
			for _, option := range source.Releases {
				if option.ID == disposition.ID && disposition.Quantity <= float64(option.Quantity) {
					releasedOnBatch += int64(disposition.Quantity)
				}
			}
			for _, dep := range dependencies {
				if dep.Kind == "stock_reservation" && dep.ID == disposition.ID && dep.ItemID == source.SourceItemID {
					releasedForItem += int64(disposition.Quantity)
				}
			}
		}
		if len(source.Releases) > 0 {
			available = min(available, max(int64(0), source.FreeQuantity+releasedOnBatch))
		}
		if selected.Quantity > available-batches[source.BatchID] {
			return "", fmt.Errorf("成品可分配数量不足，不能占用其他预留")
		}
		if !scrap && selected.Quantity > target.RemainingQuantity-targets[target.OrderItemID] {
			return "", fmt.Errorf("分配数量超过目标订单的剩余未预留需求")
		}
		var closed float64
		for _, effect := range effects {
			if effect.ItemID == source.SourceItemID {
				closed = effect.ClosedDelta
			}
		}
		protected := max(int64(0), source.UnreservedQuantity-int64(closed)+releasedForItem)
		if selected.Quantity > available-protected-batches[source.BatchID] {
			return "", fmt.Errorf("分配后无法保留本单继续履约所需的未预留数量")
		}
		if selected.Quantity > source.CloseLimit-items[source.SourceItemID] || float64(selected.Quantity) > closed-float64(items[source.SourceItemID]) {
			return "", fmt.Errorf("分配数量超过本次关闭商品数量，不能占用共享生产的其他需求")
		}
		batches[source.BatchID] += selected.Quantity
		targets[target.OrderItemID] += selected.Quantity
		items[source.SourceItemID] += selected.Quantity
		if scrap {
			label += fmt.Sprintf("；%s：报损 %d，原成本 %.2f 元", source.Label, selected.Quantity, float64(source.UnitCostMicros)/1e6*float64(selected.Quantity))
		} else {
			label += fmt.Sprintf("；%s → %s，%d", source.Label, target.Label, selected.Quantity)
		}
	}
	return label, nil
}
