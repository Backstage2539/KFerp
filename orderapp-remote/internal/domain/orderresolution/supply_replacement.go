package orderresolution

import "fmt"

type SupplyTarget struct {
	ID           int64  `json:"id"`
	Label        string `json:"label"`
	AvailableQty int64  `json:"available_qty"`
}
type SupplyReservation struct {
	ID            int64          `json:"id"`
	EntitlementID int64          `json:"entitlement_id"`
	OrderID       int64          `json:"order_id"`
	OrderItemID   int64          `json:"order_item_id"`
	Label         string         `json:"label"`
	Unit          string         `json:"unit"`
	Quantity      int64          `json:"quantity"`
	Targets       []SupplyTarget `json:"targets"`
}
type SupplyReplacement struct {
	ReservationID int64 `json:"reservation_id"`
	TargetID      int64 `json:"target_id"`
	Quantity      int64 `json:"quantity"`
}

func ValidateSupplyReplacement(dependency Dependency, disposition Disposition) error {
	if dependency.ReplacementIssue != "" {
		return fmt.Errorf("%s", dependency.ReplacementIssue)
	}
	if dependency.Relation != "supply" || len(dependency.SupplyReservations) == 0 {
		return fmt.Errorf("没有可切换的待入库预订；已入库或已发货部分须另外核对")
	}
	if len(disposition.Supplies) != len(dependency.SupplyReservations) {
		return fmt.Errorf("请逐笔选择替代货源和完整待入库预订数量")
	}
	selected := map[int64]SupplyReplacement{}
	totals := map[int64]int64{}
	var sourceQuantity int64
	for _, source := range dependency.SupplyReservations {
		sourceQuantity += source.Quantity
	}
	if dependency.Quantity > 0 && float64(sourceQuantity) != dependency.Quantity {
		return fmt.Errorf("关联预订的商品、包裹或货权身份不完整，请先核对")
	}
	for _, choice := range disposition.Supplies {
		if _, exists := selected[choice.ReservationID]; exists || choice.Quantity <= 0 || choice.TargetID <= 0 {
			return fmt.Errorf("替代货源、数量未选择或预订重复")
		}
		selected[choice.ReservationID] = choice
	}
	for _, source := range dependency.SupplyReservations {
		choice, ok := selected[source.ID]
		if !ok || choice.Quantity != source.Quantity {
			return fmt.Errorf("替换数量必须与本笔未入库预订一致，请重新核对")
		}
		found := false
		for _, target := range source.Targets {
			if target.ID != choice.TargetID {
				continue
			}
			if choice.Quantity > target.AvailableQty-totals[target.ID] {
				return fmt.Errorf("替代货源可用数量不足")
			}
			totals[target.ID] += choice.Quantity
			found = true
			break
		}
		if !found {
			return fmt.Errorf("替代货源的客户、商品、冻结规格或仓库不匹配，或已不可用")
		}
	}
	return nil
}
