package excel

import (
	"bytes"
	"fmt"
	"github.com/xuri/excelize/v2"
	app "orderapp/internal/application/customerfulfillment"
)

func RenderCustomerAccount(d app.AccountData) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	m := func(v int64) float64 { return float64(v) / 100 }
	sheets := []struct {
		name string
		rows [][]any
	}{
		{"订单账单", [][]any{
			{d.CustomerName, "ERP 统一客户账单", d.DateFrom, d.DateTo, "查询时间", d.AsOf},
			{"商品货款", m(d.Summary.GoodsCents), "加工费", m(d.Summary.ProcessingCents), "代发服务费", m(d.Summary.DirectShipServiceCents), "运费", m(d.Summary.ShippingCents + d.Summary.FeeShippingCents)},
			{"应付", m(d.Summary.PayableCents), "已付", m(d.Summary.PaidCents), "未付", m(d.Summary.DueCents), "退款", m(d.Summary.RefundCents), "调整", m(d.Summary.AdjustmentCents)},
			{"订单号", "下单日期", "商品金额", "运费", "优惠", "订单总额", "已付", "待付", "付款状态", "发货状态"},
		}},
		{"费用明细", [][]any{{"同一 ERP 来源只保留一项；已计入订单的费用仅展示，不重复计费"}, {"费用ID", "关联订单", "发生日期", "费用名称", "费用类型", "金额", "币种", "来源类型", "来源ID", "已计入订单", "结算单", "结算状态", "付款状态"}}},
		{"正式结算单", [][]any{{"客户确认对账不会改变 ERP 付款状态；账单金额调整后需要重新确认"}, {"结算单号", "开始日期", "结束日期", "金额", "ERP状态", "ERP确认时间", "付款时间", "客户对账状态", "客户确认时间", "账单版本"}}},
		{"账单异议", [][]any{{"结算单号", "费用ID", "提出时间", "异议原因", "状态", "ERP回复", "回复人", "回复时间"}}},
	}
	for _, o := range d.Rows {
		sheets[0].rows = append(sheets[0].rows, []any{o.OrderNo, o.OrderDate, m(o.GoodsCents), m(o.ShippingCents), m(o.DiscountCents), m(o.TotalCents), m(o.PaidCents), m(o.DueCents), accountStatus(o.PaymentStatus), o.ShipStatus})
	}
	s := d.Summary
	sheets[0].rows = append(sheets[0].rows, []any{"有效订单合计", "", m(s.GoodsCents), m(s.ShippingCents), m(s.DiscountCents), m(s.TotalCents), m(s.PaidCents), m(s.DueCents)})
	for _, v := range d.Fees {
		name := v.FeeName
		if name == "" {
			name = app.AccountFeeLabel(v.FeeType)
		}
		sheets[1].rows = append(sheets[1].rows, []any{v.ID, v.OrderNo, v.OccurredAt, name, v.FeeType, m(v.AmountCents), v.Currency, v.SourceType, v.SourceID, yesNo(v.IncludedInOrder), v.SettlementNo, accountStatus(v.SettlementStatus), accountStatus(v.PaymentStatus)})
	}
	for _, v := range d.Settlements {
		sheets[2].rows = append(sheets[2].rows, []any{v.SettlementNo, v.PeriodFrom, v.PeriodTo, m(v.TotalCents), accountStatus(v.Status), v.ConfirmedAt, v.PaidAt, reconciliationStatus(v.ReconciliationStatus), v.ReconciledAt, v.StatementRevision})
		for _, dispute := range v.Disputes {
			sheets[3].rows = append(sheets[3].rows, []any{v.SettlementNo, dispute.FeeItemID, dispute.CreatedAt, dispute.Reason, reconciliationStatus(dispute.Status), dispute.Reply, dispute.RepliedBy, dispute.RepliedAt})
		}
	}
	for i, sheet := range sheets {
		if i == 0 {
			if err := f.SetSheetName("Sheet1", sheet.name); err != nil {
				return nil, err
			}
		} else {
			if _, err := f.NewSheet(sheet.name); err != nil {
				return nil, err
			}
		}
		for n, row := range sheet.rows {
			if err := f.SetSheetRow(sheet.name, fmt.Sprintf("A%d", n+1), &row); err != nil {
				return nil, err
			}
		}
		if err := f.SetColWidth(sheet.name, "A", "M", 22); err != nil {
			return nil, err
		}
	}
	var b bytes.Buffer
	if err := f.Write(&b); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func yesNo(value bool) string {
	if value {
		return "是"
	}
	return "否"
}

func reconciliationStatus(s string) string {
	if v, ok := map[string]string{
		"pending": "待对账", "confirmed": "已确认", "changed": "账单已变更，需重新确认", "disputed": "有异议",
		"open": "待处理", "replied": "已回复", "resolved": "已解决", "closed": "已关闭",
	}[s]; ok {
		return v
	}
	return s
}
func accountStatus(s string) string {
	if v, ok := map[string]string{"unpaid": "未付款", "partial": "部分付款", "paid": "已付款", "confirmed": "已确认", "draft": "待确认", "reversed": "已冲销", "settled": "已入结算，付款待核实", "unknown": "待核实", "": "未入结算"}[s]; ok {
		return v
	}
	return s
}
