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
	sheets := []struct {
		name string
		rows [][]any
	}{
		{"订单账单", [][]any{{d.CustomerName, "当期订单及当前付款情况", d.DateFrom, d.DateTo, "查询时间", d.AsOf}, {"订单号", "下单日期", "商品金额", "运费", "优惠", "订单总额", "已付", "待付", "付款状态", "发货状态"}}},
		{"独立费用", [][]any{{"费用不重复计入订单合计；已进入结算单不代表已付款"}, {"关联订单", "发生日期", "费用类型", "金额", "币种", "结算单", "结算状态", "付款状态"}}},
		{"正式结算单", [][]any{{"正式结算单为来源费用的汇总，不重复累加到订单账单"}, {"结算单号", "开始日期", "结束日期", "金额", "状态", "确认时间", "付款时间"}}},
	}
	m := func(v int64) float64 { return float64(v) / 100 }
	for _, o := range d.Rows {
		sheets[0].rows = append(sheets[0].rows, []any{o.OrderNo, o.OrderDate, m(o.GoodsCents), m(o.ShippingCents), m(o.DiscountCents), m(o.TotalCents), m(o.PaidCents), m(o.DueCents), accountStatus(o.PaymentStatus), o.ShipStatus})
	}
	s := d.Summary
	sheets[0].rows = append(sheets[0].rows, []any{"有效订单合计", "", m(s.GoodsCents), m(s.ShippingCents), m(s.DiscountCents), m(s.TotalCents), m(s.PaidCents), m(s.DueCents)})
	for _, v := range d.Fees {
		sheets[1].rows = append(sheets[1].rows, []any{v.OrderNo, v.OccurredAt, app.AccountFeeLabel(v.FeeType), m(v.AmountCents), v.Currency, v.SettlementNo, accountStatus(v.SettlementStatus), accountStatus(v.PaymentStatus)})
	}
	for _, v := range d.Settlements {
		sheets[2].rows = append(sheets[2].rows, []any{v.SettlementNo, v.PeriodFrom, v.PeriodTo, m(v.TotalCents), accountStatus(v.Status), v.ConfirmedAt, v.PaidAt})
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
		if err := f.SetColWidth(sheet.name, "A", "J", 22); err != nil {
			return nil, err
		}
	}
	var b bytes.Buffer
	if err := f.Write(&b); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
func accountStatus(s string) string {
	if v, ok := map[string]string{"unpaid": "未付款", "partial": "部分付款", "paid": "已付款", "confirmed": "已确认", "draft": "待确认", "reversed": "已冲销", "settled": "已入结算，付款待核实", "unknown": "待核实", "": "未入结算"}[s]; ok {
		return v
	}
	return s
}
