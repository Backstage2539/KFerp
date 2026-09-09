package pdf

import (
	"bytes"
	"fmt"
	"github.com/jung-kurt/gofpdf"
	app "orderapp/internal/application/customerfulfillment"
	"path/filepath"
)

func RenderCustomerAccount(d app.AccountData) ([]byte, error) {
	font, err := (SalesOrderRenderer{}).resolveFontPath()
	if err != nil {
		return nil, err
	}
	p := gofpdf.New("P", "mm", "A4", filepath.Dir(font))
	p.SetMargins(15, 14, 15)
	p.SetAutoPageBreak(true, 18)
	p.AddUTF8Font("account", "", filepath.Base(font))
	p.SetFont("account", "", 10)
	p.SetFooterFunc(func() {
		p.SetY(-13)
		p.SetFont("account", "", 9)
		p.CellFormat(0, 6, fmt.Sprintf("第 %d 页", p.PageNo()), "", 0, "C", false, 0, "")
	})
	p.AddPage()
	p.SetFont("account", "", 17)
	p.MultiCell(0, 9, "客户往来账单", "", "L", false)
	p.SetFont("account", "", 10)
	line := func(s string) { p.MultiCell(0, 6, s, "", "L", false) }
	money := func(v int64) string { return fmt.Sprintf("%.2f", float64(v)/100) }
	line(d.CustomerName + "　" + d.DateFrom + " 至 " + d.DateTo)
	line("查询时间：" + d.AsOf)
	line("按下单日期归集，显示当前付款情况。独立费用与正式结算单分列，不重复计入订单合计。")
	line("订单总额 ¥" + money(d.Summary.TotalCents) + "　已付 ¥" + money(d.Summary.PaidCents) + "　待付 ¥" + money(d.Summary.DueCents))
	p.Ln(3)
	for _, o := range d.Rows {
		if p.GetY() > 245 {
			p.AddPage()
		}
		line(o.OrderDate + "　" + o.OrderNo + "　" + accountBillStatus(o.PaymentStatus) + "　" + o.ShipStatus)
		line("商品 " + money(o.GoodsCents) + "　运费 " + money(o.ShippingCents) + "　优惠 " + money(o.DiscountCents))
		line("总额 " + money(o.TotalCents) + "　已付 " + money(o.PaidCents) + "　待付 " + money(o.DueCents))
		p.Ln(3)
	}
	if len(d.Fees) > 0 {
		p.AddPage()
		line("独立费用（按发生日期查询）")
	}
	for _, f := range d.Fees {
		line(f.OccurredAt + "　" + f.OrderNo + "　" + app.AccountFeeLabel(f.FeeType) + "　" + money(f.AmountCents) + " " + f.Currency)
		line("结算单：" + f.SettlementNo + "　" + accountBillStatus(f.SettlementStatus) + "　付款：" + accountBillStatus(f.PaymentStatus))
		p.Ln(2)
	}
	if len(d.Settlements) > 0 {
		p.AddPage()
		line("已有正式结算单")
	}
	for _, b := range d.Settlements {
		line(b.SettlementNo + "　" + b.PeriodFrom + " 至 " + b.PeriodTo)
		line("金额 " + money(b.TotalCents) + "　" + accountBillStatus(b.Status) + "　确认：" + b.ConfirmedAt + "　付款：" + b.PaidAt)
		p.Ln(2)
	}
	var out bytes.Buffer
	if err = p.Output(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
func accountBillStatus(s string) string {
	if v, ok := map[string]string{"unpaid": "未付款", "partial": "部分付款", "paid": "已付款", "confirmed": "已确认", "draft": "待确认", "reversed": "已冲销", "settled": "已入结算，付款待核实", "unknown": "待核实", "": "未入结算"}[s]; ok {
		return v
	}
	return s
}
