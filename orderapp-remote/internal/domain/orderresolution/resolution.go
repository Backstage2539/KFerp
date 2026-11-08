// Package orderresolution describes explicit, auditable order dispositions.
package orderresolution

import (
	"fmt"
	"math"
	"strings"
	"time"
)

type Line struct {
	ReturnCreditQuantity float64 `json:"return_credit_quantity"`
	ReturnCreditCents    int64   `json:"return_credit_cents"`
	ID                   int64   `json:"id"`
	Name                 string  `json:"name"`
	Unit                 string  `json:"unit"`
	Quantity             float64 `json:"quantity"`
	Delivered            float64 `json:"delivered"`
	Closed               float64 `json:"closed"`
	Returned             float64 `json:"returned"`
	AmountCents          int64   `json:"amount_cents"`
	CreditedCents        int64   `json:"credited_cents"`
	ClosedCents          int64   `json:"closed_cents"`
}
type Dependency struct {
	StopWorkOrders     []StopWorkOrder      `json:"stop_work_orders,omitempty"`
	BatchID            int64                `json:"batch_id,omitempty"`
	CompanyStock       []CompanyStockSource `json:"company_stock,omitempty"`
	CashIssue          string               `json:"cash_issue,omitempty"`
	SettledOutCents    int64                `json:"settled_out_cents"`
	ReplacementIssue   string               `json:"replacement_issue,omitempty"`
	SupplyReservations []SupplyReservation  `json:"supply_reservations,omitempty"`
	OwnershipIssue     string               `json:"ownership_issue,omitempty"`
	ItemID             int64                `json:"item_id,omitempty"`
	Version            string               `json:"version,omitempty"`
	Unit               string               `json:"unit,omitempty"`
	Relation           string               `json:"relation,omitempty"`
	Orders             []LinkedOrder        `json:"orders,omitempty"`
	AmountCents        int64                `json:"amount_cents"`
	PaidCents          int64                `json:"paid_cents"`
	Kind               string               `json:"kind"`
	ID                 int64                `json:"id"`
	Label              string               `json:"label"`
	Status             string               `json:"status"`
	Shared             bool                 `json:"shared"`
	Quantity           float64              `json:"quantity"`
}
type LinkedOrder struct {
	ID     int64  `json:"id"`
	Number string `json:"number"`
	Status string `json:"status"`
}
type Context struct {
	ReturnSources    []ReturnSource    `json:"return_sources"`
	Unavailable      map[string]string `json:"unavailable,omitempty"`
	Policy           Policy            `json:"policy"`
	OrderID          int64             `json:"order_id"`
	OrderNo          string            `json:"order_no"`
	CustomerID       int64             `json:"customer_id"`
	Status           string            `json:"status"`
	Paused           bool              `json:"paused"`
	Void             bool              `json:"void"`
	FulfillmentMode  string            `json:"fulfillment_mode"`
	FulfillmentIssue string            `json:"fulfillment_issue,omitempty"`
	DeliveryMode     string            `json:"delivery_mode"`
	OrderDate        string            `json:"order_date"`
	TotalCents       int64             `json:"total_cents"`
	CreditCents      int64             `json:"credit_cents"`
	OffsetCents      int64             `json:"offset_cents"`
	PaidCents        int64             `json:"paid_cents"`
	CreditUsedCents  int64             `json:"credit_used_cents"`
	ExternalVersion  string            `json:"external_version"`
	Invoiced         bool              `json:"invoiced"`
	Billed           bool              `json:"billed"`
	Version          string            `json:"version"`
	Lines            []Line            `json:"lines"`
	Dependencies     []Dependency      `json:"dependencies"`
}
type Selection struct {
	ItemID   int64   `json:"item_id"`
	Quantity float64 `json:"quantity"`
}
type Disposition struct {
	CostTreatment   string              `json:"cost_treatment,omitempty"`
	Stops           []StopSelection     `json:"stops,omitempty"`
	StockSelections []StockSelection    `json:"stock_selections,omitempty"`
	SourceFeeID     int64               `json:"source_fee_id,omitempty"`
	Supplies        []SupplyReplacement `json:"supplies,omitempty"`
	AmountSpecified bool                `json:"amount_specified"`
	Kind            string              `json:"kind"`
	ID              int64               `json:"id"`
	Choice          string              `json:"choice"`
	TargetID        int64               `json:"target_id"`
	Quantity        float64             `json:"quantity"`
	AmountCents     int64               `json:"amount_cents"`
	Reference       string              `json:"reference"`
}
type Command struct {
	ReturnSelections []ReturnSelection `json:"return_selections,omitempty"`
	ReturnSettlement string            `json:"return_settlement,omitempty"`
	Action           string            `json:"action"`
	Reason           string            `json:"reason"`
	Note             string            `json:"note"`
	Scope            string            `json:"scope"`
	Lines            []Selection       `json:"lines"`
	Timing           string            `json:"timing"`
	ExternalStatus   string            `json:"external_status"`
	PostingMode      string            `json:"posting_mode"`
	PostingDate      string            `json:"posting_date"`
	Dispositions     []Disposition     `json:"dispositions"`
	Version          string            `json:"version"`
	IdempotencyKey   string            `json:"idempotency_key"`
}
type LineEffect struct {
	ItemID      int64   `json:"item_id"`
	Quantity    float64 `json:"quantity"`
	ClosedDelta float64 `json:"closed_delta"`
	ReturnDelta float64 `json:"return_delta"`
	CreditCents int64   `json:"credit_cents"`
}
type Task struct {
	Kind       string `json:"kind"`
	SourceID   int64  `json:"source_id"`
	Choice     string `json:"choice"`
	Label      string `json:"label"`
	Permission string `json:"permission"`
}
type FeeEffect struct {
	SourceID    int64 `json:"source_id"`
	CreditCents int64 `json:"credit_cents"`
}
type Plan struct {
	Fees        []FeeEffect  `json:"fees"`
	Status      string       `json:"status"`
	Lines       []LineEffect `json:"lines"`
	CreditCents int64        `json:"credit_cents"`
	RefundCents int64        `json:"refund_cents"`
	Tasks       []Task       `json:"tasks"`
}

// Preview never changes the source order and never chooses a disposition for the user.
func preview(c Context, cmd Command) (Plan, error) {
	p := Plan{Status: c.Status, Lines: []LineEffect{}, Tasks: []Task{}}
	fail := func(s string) (Plan, error) { return Plan{}, fmt.Errorf("%s", s) }
	if !oneOf(cmd.Action, "hold", "resume", "close", "reopen", "cancel", "amend", "return", "credit") {
		return fail("请选择处理操作")
	}
	if !oneOf(cmd.Reason, "customer_cancel", "entry_error", "duplicate", "other") || (cmd.Reason == "other" && strings.TrimSpace(cmd.Note) == "") {
		return fail("请选择原因，其他原因须填写说明")
	}
	if c.Void {
		return fail("原单已取消，请通过关联新草稿处理更正")
	}
	if cmd.Action == "hold" {
		if c.Paused {
			return fail("订单已暂停")
		}
		return p, nil
	}
	if cmd.Action == "resume" {
		if !c.Paused {
			return fail("订单未暂停")
		}
		return p, nil
	}
	if c.FulfillmentIssue != "" {
		return fail(c.FulfillmentIssue)
	}
	feesOnly := cmd.Scope == "fees_only"
	if feesOnly && cmd.Action != "credit" {
		return fail("仅调整费用只适用于金额冲减")
	}
	if !oneOf(cmd.Scope, "all", "selected", "fees_only") {
		return fail("请选择全部剩余或指定商品数量")
	}
	if feesOnly && cmd.Timing != "settle_first" {
		return fail("仅调整费用请先完成费用确认和资金清算，不停止商品履约")
	}
	if !oneOf(cmd.Timing, "stop_first", "settle_first") {
		return fail("请选择处理时点")
	}
	if !oneOf(cmd.ExternalStatus, "none", "recorded", "unknown") {
		return fail("请选择开票及外部入账情况")
	}
	if cmd.Action == "return" && !oneOf(cmd.ReturnSettlement, "keep_receivable", "credit_returned_goods") {
		return fail("请选择退回商品款的处理方式：保留应收或按退回数量冲减")
	}
	if !oneOf(cmd.PostingMode, "open_period", "historical_correction") {
		return fail("请选择账期处理方式")
	}
	if _, err := time.Parse("2006-01-02", cmd.PostingDate); err != nil {
		return fail("请选择有效的记账日期")
	}
	if (cmd.Action == "cancel" || cmd.Action == "amend") && (c.PaidCents > 0 || c.Invoiced || c.Billed) {
		return fail("已有收款或账单，请先处理财务关联，或选择关闭剩余履约")
	}
	selected := map[int64]float64{}
	for _, v := range cmd.Lines {
		if _, ok := selected[v.ItemID]; ok || !positive(v.Quantity) {
			return fail("商品数量重复或无效")
		}
		selected[v.ItemID] = v.Quantity
	}
	if cmd.Scope == "selected" && len(selected) == 0 {
		return fail("请选择要处理的商品数量")
	}
	remaining, stillClosed := 0.0, 0.0
	for _, v := range c.Lines {
		if feesOnly {
			continue
		}
		if !positive(v.Quantity) || v.AmountCents < 0 {
			return fail("原单商品数量或金额异常，请先核对原单")
		}
		if (cmd.Action == "cancel" || cmd.Action == "amend") && (v.Delivered > 0 || v.Closed > 0 || v.Returned > 0) {
			return fail("已有履约或处理记录，请关闭剩余履约或办理退货")
		}
		limit := math.Max(0, v.Quantity-v.Delivered-v.Closed)
		switch cmd.Action {
		case "reopen":
			limit = v.Closed
		case "return":
			limit = math.Max(0, v.Delivered-v.Returned)
		case "credit":
			limit = math.Max(0, v.Quantity-v.Closed)
		}
		q := limit
		if cmd.Scope == "selected" {
			q = selected[v.ID]
		}
		delete(selected, v.ID)
		if q > limit+1e-9 {
			return fail("处理数量超过可处理余额；已交付和已入客户仓的数量请办理退货")
		}
		if q > 0 {
			delta := q
			credit := int64(math.Round(float64(v.AmountCents)*(v.Closed+v.ReturnCreditQuantity+q)/v.Quantity)) - v.ClosedCents - v.ReturnCreditCents
			effect := LineEffect{ItemID: v.ID, Quantity: q, ClosedDelta: delta, CreditCents: credit}
			switch cmd.Action {
			case "reopen":
				effect.ClosedDelta = -q
				effect.CreditCents = int64(math.Round(float64(v.AmountCents)*(v.Closed+v.ReturnCreditQuantity-q)/v.Quantity)) - v.ClosedCents - v.ReturnCreditCents
			case "return":
				effect.ClosedDelta = 0
				effect.ReturnDelta = q
				effect.CreditCents = int64(math.Round(float64(v.AmountCents)*(v.Closed+v.ReturnCreditQuantity+q)/v.Quantity)) - v.ClosedCents - v.ReturnCreditCents
				if cmd.ReturnSettlement == "keep_receivable" {
					effect.CreditCents = 0
				}
			case "credit":
				effect.ClosedDelta = 0
				effect.CreditCents = int64(math.Round(float64(v.AmountCents) * q / v.Quantity))
			}
			if effect.CreditCents > 0 && effect.CreditCents > v.AmountCents-v.CreditedCents {
				return fail("累计冲减超过商品原应收金额")
			}
			p.Lines = append(p.Lines, effect)
			p.CreditCents += effect.CreditCents
		}
		remaining += math.Max(0, v.Quantity-v.Delivered-v.Closed-q)
		if cmd.Action == "reopen" {
			stillClosed += math.Max(0, v.Closed-q)
		}
	}
	if len(selected) > 0 {
		return fail("所选商品不属于本订单")
	}
	if len(p.Lines) == 0 && !feesOnly {
		return fail("没有可处理的数量")
	}
	if p.CreditCents > 0 && p.CreditCents > c.TotalCents-c.CreditCents {
		return fail("累计冲减超过订单冻结的应收金额，请核对折扣和既有冲减")
	}
	if (cmd.Action == "cancel" || cmd.Action == "amend") && (cmd.Scope != "all" || len(p.Lines) != len(c.Lines)) {
		return fail("取消原单必须处理整单；已履约部分请办理退货或关闭剩余履约")
	}
	choices := map[string]Disposition{}
	for _, d := range cmd.Dispositions {
		key := fmt.Sprintf("%s:%d", d.Kind, d.ID)
		if _, ok := choices[key]; ok {
			return fail("同一关联单据重复选择处置")
		}
		if !AllowedChoice(d.Kind, d.Choice) {
			return fail("处置选项无效")
		}
		if oneOf(d.Choice, "allocate", "reassign", "offset") && d.TargetID <= 0 && !(d.Kind == "production" && d.Choice == "allocate") {
			return fail("请选择明确的替代货源或目标单据")
		}
		if d.SourceFeeID < 0 || (d.SourceFeeID > 0 && d.Kind != "funds") || d.AmountCents < 0 || math.IsNaN(d.Quantity) || math.IsInf(d.Quantity, 0) || d.Quantity < 0 {
			return fail("处置金额或数量无效")
		}
		choices[key] = d
	}
	companyUsage := &companyAllocationUsage{map[int64]int64{}, map[int64]int64{}, map[int64]int64{}, map[string]bool{}}
	reserved := map[int64]float64{}
	feeFunds := map[int64]int64{}
	for _, d := range c.Dependencies {
		if feesOnly && d.Kind != "fee" {
			continue
		}
		key := fmt.Sprintf("%s:%d", d.Kind, d.ID)
		choice, ok := choices[key]
		delete(choices, key)
		if d.Kind == "production" && d.Status == "stopped" {
			if oneOf(cmd.Action, "cancel", "amend") {
				return fail("已停工生产仍有原扣料和产出历史，请保留原单并关闭剩余履约")
			}
			continue
		}
		if oneOf(d.Status, "cancelled", "released") {
			continue
		}
		if !ok {
			return fail("请选择关联单据的处置方式：" + d.Label)
		}
		if choice.Choice == "replace_supply" {
			if err := ValidateSupplyReplacement(d, choice); err != nil {
				return Plan{}, err
			}
			for _, selected := range choice.Supplies {
				for _, source := range d.SupplyReservations {
					if source.ID != selected.ReservationID {
						continue
					}
					if source.ConvertedQuantity > 0 {
						d.Label += fmt.Sprintf("；已入库 %d %s，批次 %s 保留原预留", source.ConvertedQuantity, source.Unit, source.PhysicalBatches)
					}
					for _, target := range source.Targets {
						if target.ID == selected.TargetID {
							d.Label += fmt.Sprintf("；%s → %s，%d %s", source.Label, target.Label, selected.Quantity, source.Unit)
						}
					}
				}
			}
		}
		if d.Kind == "stock_reservation" {
			left := d.Quantity
			if choice.Choice == "release" {
				if !positive(choice.Quantity) || choice.Quantity > d.Quantity+1e-9 {
					return fail("请明确填写本批预留释放数量，且不得超过未发货预留")
				}
				left -= choice.Quantity
				d.Label = fmt.Sprintf("%s · 释放 %g %s", d.Label, choice.Quantity, d.Unit)
			}
			reserved[d.ItemID] += left
		}
		if d.Kind == "direct_ship" && choice.Choice == "keep_fulfilled" && (cmd.Action != "return" || d.Relation != "package") {
			return fail("保留已发生包裹仅用于本单实际退货，不解除原履约义务")
		}
		if d.Kind == "direct_ship" && d.Relation == "package" {
			if cmd.Action == "return" {
				if !oneOf(choice.Choice, "keep_fulfilled", "defer") {
					return fail("实际退货须保留原代发包裹及发货记录，不能取消已发生的发货")
				}
			} else if !oneOf(cmd.Action, "close", "cancel", "amend") || cmd.Scope != "all" || choice.Choice != "cancel" {
				return fail("当前代发包裹须整单取消预订；部分调整请先明确拆分包裹")
			}
		}
		if d.Kind == "production" && oneOf(choice.Choice, "keep_company_stock", "continue_company") && d.OwnershipIssue != "" {
			return fail(d.OwnershipIssue)
		}
		if d.Kind == "production" && oneOf(choice.Choice, "allocate", "scrap") {
			label, err := validateCompanyAllocation(d, choice, p.Lines, companyUsage, c.Dependencies, cmd.Dispositions)
			if err != nil {
				return Plan{}, err
			}
			if choice.Choice == "scrap" {
				if cmd.Action != "close" {
					return fail("已完成成品报损请关闭剩余履约，保留原生产历史")
				}
				for _, selected := range choice.StockSelections {
					for _, source := range d.CompanyStock {
						if source.BatchID == selected.BatchID && (len(source.CostDate) != 10 || source.CostDate > cmd.PostingDate) {
							return fail("报损日期不能早于原批次形成及原成本日期，请核对来源记录")
						}
					}
				}
				label += "；成本处理：" + map[string]string{"retain_cost": "保留原生产成本，报损单列不重复计费", "reclassify_loss": "原生产成本重分类为损失费用，不重复增加总成本"}[choice.CostTreatment]
				p.Tasks = append(p.Tasks, Task{Kind: "production_finance", SourceID: d.ID, Choice: choice.CostTreatment, Label: d.Label + label, Permission: "finance.resolve"})
			}
			d.Label += label
		}
		if d.Kind == "production" && choice.Choice == "stop" {
			label, err := validateProductionStop(d, choice, cmd)
			if err != nil {
				return Plan{}, err
			}
			d.Label += label
			p.Tasks = append(p.Tasks, Task{Kind: "production_stock", SourceID: d.ID, Choice: "stop", Label: d.Label, Permission: "stock.resolve"})
		}
		if d.Kind == "production" && (cmd.Action == "cancel" || cmd.Action == "amend") && d.Status == "completed" {
			return fail("已完成生产必须保留，请选择关闭剩余履约")
		}
		if d.Kind == "production" && oneOf(cmd.Action, "cancel", "amend") && choice.Choice != "cancel_unstarted" {
			return fail("保留或继续生产请关闭剩余履约；取消原单须先撤销尚未开工的关联生产")
		}
		if d.Kind == "production" && d.Status != "completed" && oneOf(choice.Choice, "keep_company_stock", "allocate", "scrap") {
			return fail("未完成生产请明确选择停止、继续备货或转供，不能作为已完成库存处理")
		}
		if d.Kind == "production" && d.Status == "completed" && !oneOf(choice.Choice, "keep_company_stock", "allocate", "scrap", "defer") {
			return fail("已完成生产不能停止或取消")
		}
		if d.Shared && oneOf(choice.Choice, "cancel_unstarted", "stop") {
			return fail("共享生产须先拆分剩余需求，不能整单停止其他订单生产")
		}
		if d.Kind == "customer_stock" && cmd.Action != "return" && oneOf(choice.Choice, "ownership_return", "physical_return") {
			return fail("已入客户仓的数量请单独选择退货，明确数量和货权后处理")
		}
		if d.Kind == "fee" && choice.Choice != "defer" {
			credit := int64(0)
			switch choice.Choice {
			case "waive":
				credit = d.AmountCents
			case "partial_waiver":
				if choice.AmountCents <= 0 || choice.AmountCents >= d.AmountCents {
					return fail("部分减免金额须大于零且小于当前费用")
				}
				credit = choice.AmountCents
			case "negotiated":
				if !choice.AmountSpecified {
					return fail("请明确填写协商后的费用金额，零金额也须填写")
				}
				credit = d.AmountCents - choice.AmountCents
			}
			if credit > 0 && d.CashIssue != "" {
				return fail(d.CashIssue)
			}
			if credit < 0 || credit > max(int64(0), d.AmountCents) {
				return fail("协商金额须在当前费用范围内")
			}
			if d.ID > 0 {
				available := max(int64(0), d.PaidCents-max(int64(0), d.AmountCents-credit)-d.SettledOutCents)
				feeFunds[d.ID] = available
				if available > 0 {
					p.Tasks = append(p.Tasks, Task{Kind: "funds", SourceID: d.ID, Label: fmt.Sprintf("费用 #%d · %s · 本次待处置合计 ¥%.2f（可分次登记）", d.ID, d.Label, float64(available)/100), Permission: "finance.resolve"})
				}
			}
			if d.ID < 0 && (cmd.Action == "cancel" || cmd.Action == "amend") && credit != d.AmountCents {
				return fail("订单仍保留费用应收，请选择关闭剩余履约")
			}
			d.Label = fmt.Sprintf("%s · 减免 ¥%.2f，保留 ¥%.2f", d.Label, float64(credit)/100, float64(d.AmountCents-credit)/100)
			p.Fees = append(p.Fees, FeeEffect{SourceID: d.ID, CreditCents: credit})
			if d.ID < 0 {
				p.CreditCents += credit
			}
		}
		permission := "orders.resolve"
		switch d.Kind {
		case "direct_ship":
			if choice.Choice == "replace_supply" {
				permission = "stock.resolve"
			}
		case "production":
			permission = "production.resolve"
			if oneOf(choice.Choice, "allocate", "scrap") {
				permission = "stock.resolve"
			}
		case "customer_stock", "stock", "stock_reservation":
			permission = "stock.resolve"
		case "fee", "invoice", "billing":
			permission = "finance.resolve"
		}
		if choice.Choice != "keep_company_stock" && choice.Choice != "keep_sale" && choice.Choice != "keep_fulfilled" && !(cmd.Action == "return" && d.Kind == "customer_stock" && choice.Choice == "ownership_return") {
			p.Tasks = append(p.Tasks, Task{Kind: d.Kind, SourceID: d.ID, Choice: choice.Choice, Label: d.Label, Permission: permission})
		}
	}
	if feesOnly {
		changed := false
		for _, f := range p.Fees {
			changed = changed || f.CreditCents > 0
		}
		if !changed {
			return fail("请选择需要减免或协商的费用")
		}
	}
	for itemID, quantity := range reserved {
		for _, line := range c.Lines {
			if line.ID != itemID {
				continue
			}
			closed := line.Closed
			for _, effect := range p.Lines {
				if effect.ItemID == itemID {
					closed += effect.ClosedDelta
				}
			}
			if quantity > math.Max(0, line.Quantity-line.Delivered-closed)+1e-9 {
				return fail("关闭后剩余履约数量不足以保留现有预留，请明确释放相应批次数量")
			}
		}
	}
	if p.CreditCents > c.TotalCents-c.CreditCents {
		return fail("累计冲减超过订单冻结的应收金额")
	}
	allocatedFunds := map[int64]int64{}
	availableFunds := max(int64(0), c.PaidCents+c.OffsetCents-max(int64(0), c.TotalCents-c.CreditCents-p.CreditCents)-c.CreditUsedCents)
	for _, d := range choices {
		if d.Kind != "funds" {
			return fail("关联单据已变化，请刷新预览")
		}
		if d.AmountCents <= 0 {
			return fail("请填写资金处置金额")
		}
		budget := availableFunds
		if d.SourceFeeID != 0 {
			budget = feeFunds[d.SourceFeeID]
		}
		if d.AmountCents > budget-allocatedFunds[d.SourceFeeID] {
			return fail("资金分配超过当前可退金额")
		}
		allocatedFunds[d.SourceFeeID] += d.AmountCents
		if d.Choice == "refund" {
			p.RefundCents += d.AmountCents
		}
		if !planHasTaskSource(p, "funds", d.SourceFeeID) {
			p.Tasks = append(p.Tasks, Task{Kind: "funds", SourceID: d.SourceFeeID, Label: "资金处置（可拆分登记）", Permission: "finance.resolve"})
		}
	}
	if cmd.ExternalStatus != "none" || c.Invoiced {
		p.Tasks = append(p.Tasks, Task{Kind: "external", Choice: cmd.ExternalStatus, Label: "核实开票和外部账务并登记处理凭据", Permission: "finance.resolve"})
	}
	if cmd.PostingMode == "historical_correction" {
		p.Tasks = append(p.Tasks, Task{Kind: "period", Choice: cmd.PostingMode, Label: "历史账期更正审批", Permission: "finance.resolve"})
	}
	if c.Billed {
		p.Tasks = append(p.Tasks, Task{Kind: "billing", Choice: "adjust", Label: "已出账金额更正", Permission: "finance.resolve"})
	}
	if cmd.Action != "reopen" && max(int64(0), c.PaidCents+c.OffsetCents-max(int64(0), c.TotalCents-c.CreditCents-p.CreditCents))-c.CreditUsedCents > 0 && !planHasTaskSource(p, "funds", 0) {
		p.Tasks = append(p.Tasks, Task{Kind: "funds", Label: "请选择可退金额去向", Permission: "finance.resolve"})
	}
	if cmd.Action == "return" {
		label := "保留退回商品原应收；后续账款另行处理"
		if cmd.ReturnSettlement == "credit_returned_goods" {
			label = "按退回数量冲减商品款，退款另行登记"
		}
		p.Tasks = append(p.Tasks, Task{Kind: "return_finance", Choice: cmd.ReturnSettlement, Label: label, Permission: "finance.resolve"})
		label, err := validateReturnSelections(c, cmd, p)
		if err != nil {
			return Plan{}, err
		}
		p.Tasks = append(p.Tasks, Task{Kind: "return_stock", Choice: "receive", Label: label, Permission: "stock.resolve"})
	}
	switch cmd.Action {
	case "close":
		p.Status = "closed"
		if remaining > 1e-9 {
			p.Status = "partially_closed"
		}
	case "reopen":
		p.Status = "open"
		if stillClosed > 1e-9 {
			p.Status = "partially_closed"
		}
	case "cancel", "amend":
		p.Status = "cancelled"
	}
	return p, nil
}
func positive(v float64) bool { return v > 0 && !math.IsNaN(v) && !math.IsInf(v, 0) }
func oneOf(v string, items ...string) bool {
	for _, s := range items {
		if v == s {
			return true
		}
	}
	return false
}
func planHasTask(p Plan, k string) bool {
	for _, v := range p.Tasks {
		if v.Kind == k {
			return true
		}
	}
	return false
}
func AllowedChoice(kind, choice string) bool {
	switch kind {
	case "stock_reservation":
		return oneOf(choice, "release", "keep", "defer")
	case "production":
		return oneOf(choice, "cancel_unstarted", "stop", "continue_company", "reassign", "keep_company_stock", "allocate", "scrap", "defer")
	case "direct_ship":
		return oneOf(choice, "cancel", "replace_supply", "keep_fulfilled", "defer")
	case "stock":
		return oneOf(choice, "keep_company_stock", "allocate", "scrap", "defer")
	case "customer_stock":
		return oneOf(choice, "keep_sale", "ownership_return", "physical_return", "defer")
	case "fee":
		return oneOf(choice, "keep", "partial_waiver", "waive", "negotiated", "defer")
	case "funds":
		return oneOf(choice, "refund", "customer_credit", "offset")
	case "billing", "invoice":
		return oneOf(choice, "adjust", "defer")
	}
	return false
}

type Option struct {
	Value    string `json:"value"`
	Label    string `json:"label"`
	Disabled bool   `json:"disabled"`
	Reason   string `json:"reason,omitempty"`
}

func Options() map[string][]Option {
	out := map[string][]Option{}
	add := func(k string, v ...string) {
		for i := 0; i < len(v); i += 2 {
			out[k] = append(out[k], Option{Value: v[i], Label: v[i+1]})
		}
	}
	add("action", "hold", "暂停履约", "resume", "恢复履约", "close", "关闭剩余履约", "reopen", "重新开启", "cancel", "取消原单", "amend", "取消并修订", "return", "退货", "credit", "金额冲减")
	add("return_finance", "keep_receivable", "保留商品原应收", "credit_returned_goods", "按退回数量冲减商品款")
	add("return_stock", "receive", "已核对实物、原批次和验收凭据，确认入库")
	add("return_quality", "restock", "验收合格，可用库存", "quarantine", "待检隔离，禁止销售出库")
	add("return_ownership", "company", "收回公司货权", "customer", "保留原客户货权")
	add("reason", "customer_cancel", "客户取消", "entry_error", "录单错误", "duplicate", "重复订单", "other", "其他")
	add("scope", "all", "全部可处理数量", "selected", "指定商品和数量", "fees_only", "仅调整费用（商品履约保持原状）")
	add("timing", "stop_first", "先停止新增履约，后完成清算", "settle_first", "完成处置和清算后最终关闭")
	add("external_status", "none", "未开票且无外部入账", "recorded", "已开票或已外部入账", "unknown", "待核实")
	add("posting_mode", "open_period", "开放账期调整", "historical_correction", "申请历史账期更正")
	add("scrap_cost", "retain_cost", "保留原生产成本，单列报损", "reclassify_loss", "原成本重分类为损失费用")
	add("production", "cancel_unstarted", "取消尚未开工部分", "stop", "停止剩余生产并处理余料", "continue_company", "继续生产作为公司备货", "reassign", "转供指定其他需求", "keep_company_stock", "保留公司库存", "allocate", "分配指定其他需求", "scrap", "另行批准报损", "defer", "暂不处理")
	add("direct_ship", "cancel", "取消该代发", "replace_supply", "保留并更换货源", "keep_fulfilled", "保留原包裹与已发生发货，另行登记退回", "defer", "暂不处理")
	add("customer_stock", "keep_sale", "保留原销售与客户库存", "ownership_return", "办理货权退回", "physical_return", "办理实物退货", "defer", "暂不处理")
	add("stock", "keep_company_stock", "保留公司库存", "allocate", "分配指定其他需求", "scrap", "另行批准报损", "defer", "暂不处理")
	add("stock_reservation", "release", "释放指定数量的未发货预留", "keep", "保留本批预留用于剩余履约", "defer", "暂不处理")
	add("fee", "keep", "保留收费", "partial_waiver", "部分减免", "waive", "全部减免", "negotiated", "协商金额", "defer", "暂不处理")
	add("funds", "refund", "登记实际退款", "customer_credit", "保留客户余额", "offset", "抵扣指定应收")
	add("billing", "adjust", "保留原账单并登记本次调整")
	return out
}

func planHasTaskSource(p Plan, kind string, id int64) bool {
	for _, t := range p.Tasks {
		if t.Kind == kind && t.SourceID == id {
			return true
		}
	}
	return false
}
