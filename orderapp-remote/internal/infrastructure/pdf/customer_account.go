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
	line("按 ERP 应收、费用和结算来源归集；已包含在订单中的费用仅展示来源，不重复计费。")
	line("商品货款 ¥" + money(d.Summary.GoodsCents) + "　加工费 ¥" + money(d.Summary.ProcessingCents) + "　代发服务费 ¥" + money(d.Summary.DirectShipServiceCents))
	line("运费 ¥" + money(d.Summary.ShippingCents+d.Summary.FeeShippingCents) + "　调整 ¥" + money(d.Summary.AdjustmentCents) + "　退款 ¥" + money(d.Summary.RefundCents))
	line("应付 ¥" + money(d.Summary.PayableCents) + "　已付 ¥" + money(d.Summary.PaidCents) + "　未付 ¥" + money(d.Summary.DueCents))
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
		name := f.FeeName
		if name == "" {
			name = app.AccountFeeLabel(f.FeeType)
		}
		included := ""
		if f.IncludedInOrder {
			included = "　已计入订单"
		}
		line(f.OccurredAt + "　" + f.OrderNo + "　" + name + "　" + money(f.AmountCents) + " " + f.Currency + included)
		line("来源：" + f.SourceType + " / " + fmt.Sprint(f.SourceID))
		line("结算单：" + f.SettlementNo + "　" + accountBillStatus(f.SettlementStatus) + "　付款：" + accountBillStatus(f.PaymentStatus))
		p.Ln(2)
	}
	if len(d.Settlements) > 0 {
		p.AddPage()
		line("已有正式结算单")
	}
	for _, b := range d.Settlements {
		line(b.SettlementNo + "　" + b.PeriodFrom + " 至 " + b.PeriodTo)
		line("金额 " + money(b.TotalCents) + "　付款：" + accountBillStatus(b.Status) + "　对账：" + accountReconciliationStatus(b.ReconciliationStatus))
		line("ERP确认：" + b.ConfirmedAt + "　客户确认：" + b.ReconciledAt + "　付款时间：" + b.PaidAt)
		for _, dispute := range b.Disputes {
			line("异议：" + dispute.Reason + "　状态：" + accountReconciliationStatus(dispute.Status))
			if dispute.Reply != "" {
				line("ERP回复：" + dispute.Reply + "　" + dispute.RepliedBy + " " + dispute.RepliedAt)
			}
		}
		p.Ln(2)
	}
	var out bytes.Buffer
	if err = p.Output(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func accountReconciliationStatus(s string) string {
	if v, ok := map[string]string{
		"pending": "待对账", "confirmed": "已确认", "changed": "账单已变更，需重新确认", "disputed": "有异议",
		"open": "待处理", "replied": "已回复", "resolved": "已解决", "closed": "已关闭",
	}[s]; ok {
		return v
	}
	return s
}
func accountBillStatus(s string) string {
	if v, ok := map[string]string{"unpaid": "未付款", "partial": "部分付款", "paid": "已付款", "confirmed": "已确认", "draft": "待确认", "reversed": "已冲销", "settled": "已入结算，付款待核实", "unknown": "待核实", "": "未入结算"}[s]; ok {
		return v
	}
	return s
}
