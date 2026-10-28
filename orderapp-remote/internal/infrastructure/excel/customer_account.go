package excel

import (
	"bytes"
	"fmt"
	"github.com/xuri/excelize/v2"
	app "orderapp/internal/application/customerfulfillment"
	"strings"
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

func RenderCustomerOrderFeeAccount(d app.AccountData) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	for _, sheet := range []string{"账期汇总", "订单汇总", "费用明细"} {
		if sheet == "账期汇总" {
			if err := f.SetSheetName("Sheet1", sheet); err != nil {
				return nil, err
			}
		} else if _, err := f.NewSheet(sheet); err != nil {
			return nil, err
		}
	}
	money := func(v int64) float64 { return float64(v) / 100 }
	status := func(v string) string { return accountStatus(v) }
	put := func(sheet string, row int, values []any) error {
		return f.SetSheetRow(sheet, fmt.Sprintf("A%d", row), &values)
	}
	heading := []any{"客户费用明细", d.CustomerName, "账期", d.DateFrom, d.DateTo, "生成时间", d.AsOf}
	if d.UnpaidOnly {
		heading = []any{"客户费用明细", d.CustomerName, "范围", "所有未付", nil, "截至", d.AsOf}
	}
	if err := put("账期汇总", 1, heading); err != nil {
		return nil, err
	}
	s := d.FeeSummary
	summaryRows := [][]any{
		{"费用类型", "应付金额", "已付金额", "未付金额"},
		{"商品货款", money(s.GoodsCents), nil, nil}, {"加工费", money(s.ProcessingCents), nil, nil},
		{"代发服务费", money(s.DirectShipServiceCents), nil, nil}, {"运费", money(s.ShippingCents), nil, nil},
		{"其他费用", money(s.OtherCents), nil, nil}, {"合计", money(s.PayableCents), money(s.PaidCents), money(s.DueCents)},
	}
	if d.UnpaidOnly {
		summaryRows[0] = []any{"费用类型", "未付金额"}
		summaryRows[6] = []any{"合计", money(s.DueCents)}
	}
	for i, row := range summaryRows {
		if err := put("账期汇总", i+3, row); err != nil {
			return nil, err
		}
	}
	orderHeader := []any{"订单号", "订单日期", "商品货款", "加工费", "代发服务费", "运费", "其他费用", "应付", "已付", "未付"}
	if d.UnpaidOnly {
		orderHeader = []any{"订单号", "订单日期", "商品货款未付", "加工费未付", "代发服务费未付", "运费未付", "其他费用未付", "未付合计"}
	}
	if err := put("订单汇总", 1, orderHeader); err != nil {
		return nil, err
	}
	detailHeader := []any{"发生日期", "订单号", "费用名称及商品", "费用类型", "来源", "金额", "付款状态", "发货时间", "寄件重量(kg)", "运单号"}
	if d.UnpaidOnly {
		detailHeader[5] = "未付金额"
		detailHeader = append(detailHeader, "原应付", "已付")
	}
	if err := put("费用明细", 1, detailHeader); err != nil {
		return nil, err
	}
	row := 2
	orderRow := 2
	for _, group := range d.OrderFeeGroups {
		if !group.Unlinked {
			var goods, processing, service, shipping, other int64
			for _, line := range group.FeeLines {
				switch line.Category {
				case "product":
					goods += line.AmountCents
				case "processing":
					processing += line.AmountCents
				case "direct_ship_service":
					service += line.AmountCents
				case "shipping":
					shipping += line.AmountCents
				default:
					other += line.AmountCents
				}
			}
			orderValues := []any{group.Order.OrderNo, group.Order.OrderDate, money(goods), money(processing), money(service), money(shipping), money(other), money(group.PayableCents), money(group.PaidCents), money(group.DueCents)}
			if d.UnpaidOnly {
				orderValues = orderValues[:8]
			}
			if err := put("订单汇总", orderRow, orderValues); err != nil {
				return nil, err
			}
			orderRow++
		}
		for _, line := range group.FeeLines {
			name := line.FeeName
			if name == "" {
				name = app.AccountFeeLabel(line.FeeType)
			}
			if line.Category == "product" && len(group.Order.Items) > 0 {
				parts := make([]string, 0, len(group.Order.Items))
				for _, item := range group.Order.Items {
					parts = append(parts, fmt.Sprintf("%s %s × %s%s，单价¥%s，折后¥%s", item.Name, item.Spec, item.Quantity, item.Unit, item.UnitPrice, item.LineTotal))
				}
				name += "：" + strings.Join(parts, "；")
				if group.Order.DiscountCents != 0 {
					name += fmt.Sprintf("；订单优惠¥%.2f", money(group.Order.DiscountCents))
				}
			}
			shipments := group.Shipments
			if line.TrackingNo != "" {
				shipments = make([]app.AccountShipment, 0, 1)
				for _, item := range group.Shipments {
					if item.TrackingNo == line.TrackingNo {
						shipments = append(shipments, item)
						break
					}
				}
			}
			waybills := make([]string, 0, len(group.Shipments))
			for _, item := range shipments {
				if item.TrackingNo != "" {
					waybills = append(waybills, item.TrackingNo)
				}
			}
			tracking := line.TrackingNo
			if tracking == "" {
				tracking = strings.Join(waybills, ", ")
			}
			whenValues := make([]string, 0, len(shipments))
			weightValues := make([]string, 0, len(shipments))
			for _, shipment := range shipments {
				when := shipment.ShippedAt
				if when == "" {
					when = "未记录"
				} else if shipment.Source != "" {
					when += "（" + shipment.Source + "）"
				}
				whenValues = append(whenValues, when)
				weight := shipment.SettlementWeightKG
				if weight == "" {
					weight = "未记录"
				}
				weightValues = append(weightValues, weight)
			}
			when, weight := strings.Join(whenValues, "；"), strings.Join(weightValues, "；")
			if when == "" {
				when = "未记录"
			}
			if weight == "" {
				weight = "未记录"
			}
			detailValues := []any{line.OccurredAt, group.Order.OrderNo, name, app.AccountFeeLabel(line.FeeType), line.SourceType + " " + fmt.Sprint(line.SourceID), money(line.AmountCents), status(line.PaymentStatus), when, weight, tracking}
			if d.UnpaidOnly {
				detailValues = append(detailValues, money(line.ReceivableCents), money(line.ReceivedCents))
			}
			if err := put("费用明细", row, detailValues); err != nil {
				return nil, err
			}
			row++
		}
	}
	style, err := f.NewStyle(&excelize.Style{NumFmt: 2})
	if err != nil {
		return nil, err
	}
	for _, sheet := range []string{"账期汇总", "订单汇总", "费用明细"} {
		maxCol := "J"
		if err = f.SetColWidth(sheet, "A", maxCol, 22); err != nil {
			return nil, err
		}
		if sheet == "费用明细" {
			if d.UnpaidOnly {
				if err = f.SetColWidth(sheet, "K", "L", 22); err != nil {
					return nil, err
				}
				if err = f.SetCellStyle(sheet, "K2", fmt.Sprintf("L%d", max(2, row-1)), style); err != nil {
					return nil, err
				}
			}
			if err = f.SetCellStyle(sheet, "F2", fmt.Sprintf("F%d", max(2, row-1)), style); err != nil {
				return nil, err
			}
			if err = f.SetCellStyle(sheet, "J2", fmt.Sprintf("J%d", max(2, row-1)), mustTextStyle(f)); err != nil {
				return nil, err
			}
		}
		if sheet == "订单汇总" {
			if err = f.SetCellStyle(sheet, "C2", fmt.Sprintf("J%d", max(2, orderRow-1)), style); err != nil {
				return nil, err
			}
		}
		if sheet == "账期汇总" {
			if err = f.SetCellStyle(sheet, "B4", "D9", style); err != nil {
				return nil, err
			}
		}
	}
	var b bytes.Buffer
	if err := f.Write(&b); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func mustTextStyle(f *excelize.File) int {
	style, err := f.NewStyle(&excelize.Style{NumFmt: 49})
	if err != nil {
		return 0
	}
	return style
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
