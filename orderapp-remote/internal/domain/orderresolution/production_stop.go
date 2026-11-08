package orderresolution

import "fmt"

// Quantities remain in their original unit, warehouse and ownership. These rows
// describe posted facts; accepting a stop never turns WIP into available stock.
type ProductionRemainder struct {
	Kind            string  `json:"kind"`
	ID              int64   `json:"id"`
	Label           string  `json:"label"`
	Warehouse       string  `json:"warehouse"`
	OwnerCustomerID int64   `json:"owner_customer_id"`
	Quantity        float64 `json:"quantity"`
	Unit            string  `json:"unit"`
	Cost            float64 `json:"cost"`
}
type StopWorkOrder struct {
	WorkOrderID     int64                 `json:"work_order_id"`
	Label           string                `json:"label"`
	Version         string                `json:"version"`
	Issue           string                `json:"issue,omitempty"`
	HasMaterials    bool                  `json:"has_materials"`
	HasWIP          bool                  `json:"has_wip"`
	HasReservations bool                  `json:"has_reservations"`
	HasOutputs      bool                  `json:"has_outputs"`
	Remainders      []ProductionRemainder `json:"remainders"`
}
type StopSelection struct {
	WorkOrderID  int64  `json:"work_order_id"`
	Materials    string `json:"materials"`
	WIP          string `json:"wip"`
	Reservations string `json:"reservations"`
	Outputs      string `json:"outputs"`
}

func validateProductionStop(d Dependency, choice Disposition, cmd Command) (string, error) {
	if cmd.Action != "close" || cmd.Scope != "all" {
		return "", fmt.Errorf("停止整批生产须关闭本单全部剩余履约；部分数量请先拆分生产需求")
	}
	if d.Shared {
		return "", fmt.Errorf("共享生产须先拆分剩余需求，不能整单停止其他订单生产")
	}
	if len(d.StopWorkOrders) == 0 || len(choice.Stops) != len(d.StopWorkOrders) {
		return "", fmt.Errorf("请逐工单核对余料、在制品、预留和已完成产出的处置")
	}
	seen := map[int64]bool{}
	label := ""
	for _, s := range choice.Stops {
		var w StopWorkOrder
		for _, v := range d.StopWorkOrders {
			if v.WorkOrderID == s.WorkOrderID {
				w = v
				break
			}
		}
		if w.WorkOrderID <= 0 || seen[s.WorkOrderID] {
			return "", fmt.Errorf("停工工单不匹配或重复，请重新核对")
		}
		seen[s.WorkOrderID] = true
		if w.Issue != "" {
			return "", fmt.Errorf("%s", w.Issue)
		}
		if (w.HasMaterials && s.Materials != "retain_original") || (!w.HasMaterials && s.Materials != "returned") {
			return "", fmt.Errorf("%s：请明确保留原货权原位置的已投余料；需要退料时先在工位按真实未加工数量退回，再刷新方案", w.Label)
		}
		if (w.HasWIP && s.WIP != "retain_original") || (!w.HasWIP && s.WIP != "none") {
			return "", fmt.Errorf("%s：请核对并明确保留原货权、工序、位置和成本的在制品", w.Label)
		}
		if (w.HasReservations && !oneOf(s.Reservations, "keep", "release_unconsumed")) || (!w.HasReservations && s.Reservations != "none") {
			return "", fmt.Errorf("%s：请选择未耗用预留保留或释放；不改变实际库存", w.Label)
		}
		if (w.HasOutputs && s.Outputs != "keep_original") || (!w.HasOutputs && s.Outputs != "none") {
			return "", fmt.Errorf("%s：请确认已完成产出保留原批次、货权及成本", w.Label)
		}
		labels := map[string]string{"retain_original": "保留原位和原货权", "returned": "无未处理余料", "none": "已核对无剩余", "keep": "保留原预留", "release_unconsumed": "释放未耗用预留", "keep_original": "保留原批次及货权"}
		label += fmt.Sprintf("；%s：余料 %s，在制品 %s，预留 %s，产出 %s", w.Label, labels[s.Materials], labels[s.WIP], labels[s.Reservations], labels[s.Outputs])
		for _, row := range w.Remainders {
			label += fmt.Sprintf("；%s %g %s，%s，货主 #%d，成本 %.2f", row.Label, row.Quantity, row.Unit, row.Warehouse, row.OwnerCustomerID, row.Cost)
		}
	}
	return label, nil
}
