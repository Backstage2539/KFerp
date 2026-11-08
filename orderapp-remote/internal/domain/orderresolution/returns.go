package orderresolution

import (
	"fmt"
	"math"
)

type ReturnDestination struct {
	Ownership       string `json:"ownership"`
	Warehouse       string `json:"warehouse"`
	OwnerCustomerID int64  `json:"owner_customer_id"`
	Label           string `json:"label"`
}
type ReturnSource struct {
	Kind                   string              `json:"kind"`
	ID                     int64               `json:"id"`
	ItemID                 int64               `json:"item_id"`
	BatchID                int64               `json:"batch_id"`
	EntitlementID          int64               `json:"entitlement_id,omitempty"`
	Label                  string              `json:"label"`
	Unit                   string              `json:"unit"`
	AvailableQuantity      int64               `json:"available_quantity"`
	BatchAvailableQuantity int64               `json:"batch_available_quantity"`
	Version                string              `json:"version"`
	Destinations           []ReturnDestination `json:"destinations"`
}
type ReturnSelection struct {
	SourceKind string `json:"source_kind"`
	SourceID   int64  `json:"source_id"`
	BatchID    int64  `json:"batch_id"`
	Quantity   int64  `json:"quantity"`
	Ownership  string `json:"ownership"`
	Warehouse  string `json:"warehouse"`
	Quality    string `json:"quality"`
}

func validateReturnSelections(c Context, cmd Command, plan Plan) (string, error) {
	if len(cmd.ReturnSelections) == 0 {
		return "", fmt.Errorf("请选择实际退回来源、批次、数量、货权、仓库及验收处置")
	}
	seen := map[string]bool{}
	quantities := map[int64]int64{}
	batchQuantities := map[int64]int64{}
	label := ""
	for _, selected := range cmd.ReturnSelections {
		key := fmt.Sprintf("%s:%d", selected.SourceKind, selected.SourceID)
		if seen[key] || selected.Quantity <= 0 || selected.BatchID <= 0 || !oneOf(selected.Quality, "restock", "quarantine") {
			return "", fmt.Errorf("退回来源重复、数量无效或未选择验收处置")
		}
		seen[key] = true
		var source *ReturnSource
		for n := range c.ReturnSources {
			s := &c.ReturnSources[n]
			if s.Kind == selected.SourceKind && s.ID == selected.SourceID && s.BatchID == selected.BatchID {
				source = s
				break
			}
		}
		if source == nil || selected.Quantity > source.AvailableQuantity {
			return "", fmt.Errorf("退回来源或实际可退数量已变化，请重新核对原批次")
		}
		valid := false
		for _, destination := range source.Destinations {
			if selected.Ownership == destination.Ownership && selected.Warehouse == destination.Warehouse && selected.Ownership != "" && selected.Warehouse != "" {
				valid = true
				break
			}
		}
		if !valid {
			return "", fmt.Errorf("请选择与原实物货权匹配的退回仓库和货权处置")
		}
		if source.Kind == "ownership" {
			batchQuantities[source.BatchID] += selected.Quantity
			if batchQuantities[source.BatchID] > source.BatchAvailableQuantity {
				return "", fmt.Errorf("本批货权退回总量超过未预留实物库存")
			}
		}
		quantities[source.ItemID] += selected.Quantity
		quality := "验收合格入库"
		if selected.Quality == "quarantine" {
			quality = "待检隔离，禁止销售出库"
		}
		label += fmt.Sprintf("%s · %d %s → %s · %s；", source.Label, selected.Quantity, source.Unit, selected.Warehouse, quality)
	}
	for _, line := range plan.Lines {
		if line.ReturnDelta <= 0 || line.ReturnDelta != math.Trunc(line.ReturnDelta) || int64(line.ReturnDelta) != quantities[line.ItemID] {
			return "", fmt.Errorf("逐批退回数量须与本次所选商品数量一致")
		}
		delete(quantities, line.ItemID)
	}
	if len(quantities) > 0 {
		return "", fmt.Errorf("退回批次不属于本次所选商品")
	}
	return label, nil
}
