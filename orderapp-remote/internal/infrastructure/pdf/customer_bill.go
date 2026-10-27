package pdf

import (
	"bytes"
	"fmt"
	"path/filepath"

	"github.com/jung-kurt/gofpdf"
	app "orderapp/internal/application/customerfulfillment"
)

func RenderCustomerBill(bill app.BillingBill) ([]byte, error) {
	font, err := (SalesOrderRenderer{}).resolveFontPath()
	if err != nil {
		return nil, err
	}
	p := gofpdf.New("L", "mm", "A4", filepath.Dir(font))
	p.SetMargins(14, 14, 14)
	p.SetAutoPageBreak(true, 18)
	p.AddUTF8Font("bill", "", filepath.Base(font))
	p.SetFooterFunc(func() {
		p.SetY(-13)
		p.SetFont("bill", "", 9)
		p.CellFormat(0, 6, fmt.Sprintf("第 %d 页", p.PageNo()), "", 0, "C", false, 0, "")
	})
	columns := []struct {
		width float64
		title string
	}{
		{25, "类型"}, {21, "发生日期"}, {31, "来源单号"}, {57, "费用名称"},
		{25, "应收"}, {24, "出账金额"}, {27, "出账前已收"}, {27, "出账后已收"}, {25, "当前未收"},
	}
	pageHeader := func() {
		p.SetFont("bill", "", 17)
		p.CellFormat(0, 10, "客户费用明细", "", 1, "L", false, 0, "")
		p.SetFont("bill", "", 10)
		p.CellFormat(0, 7, bill.CustomerName+"　账单号："+bill.SettlementNo, "", 1, "L", false, 0, "")
		p.CellFormat(0, 7, "出账日期："+bill.IssuedAt+"　费用期间："+bill.PeriodFrom+" 至 "+bill.PeriodTo, "", 1, "L", false, 0, "")
		p.Ln(2)
		p.SetFillColor(239, 243, 247)
		p.SetFont("bill", "", 8)
		for _, col := range columns {
			p.CellFormat(col.width, 8, col.title, "1", 0, "L", true, 0, "")
		}
		p.Ln(-1)
		p.SetFont("bill", "", 8)
	}
	p.AddPage()
	pageHeader()
	for lineIndex, line := range bill.Lines {
		values := []string{
			app.AccountFeeLabel(line.ChargeType), line.OccurredAt, line.SourceNo, line.Description,
			fmt.Sprintf("%.2f", float64(line.ReceivableCents)/100),
			fmt.Sprintf("%.2f", float64(line.AmountCents)/100),
			fmt.Sprintf("%.2f", float64(line.IssuedReceivedCents)/100),
			fmt.Sprintf("%.2f", float64(line.ReceivedCents)/100),
			fmt.Sprintf("%.2f", float64(line.DueCents)/100),
		}
		rowY := p.GetY()
		rowHeight := 8.0
		for i, value := range values {
			lines := p.SplitText(value, columns[i].width-2)
			if height := float64(len(lines))*4.5 + 1.5; height > rowHeight {
				rowHeight = height
			}
		}
		reserveTotalHeight := 0.0
		if lineIndex == len(bill.Lines)-1 {
			reserveTotalHeight = 12
		}
		if rowY+rowHeight+reserveTotalHeight > 190 {
			p.AddPage()
			pageHeader()
			rowY = p.GetY()
		}
		leftX := 14.0
		for i, value := range values {
			p.Rect(leftX, rowY, columns[i].width, rowHeight, "D")
			p.SetXY(leftX+1, rowY+0.75)
			p.MultiCell(columns[i].width-2, 4.5, value, "", "L", false)
			leftX += columns[i].width
		}
		p.SetXY(14, rowY+rowHeight)
	}
	p.Ln(3)
	p.SetFont("bill", "", 10)
	p.CellFormat(0, 8, fmt.Sprintf("合计：出账 ¥%.2f　出账前已收 ¥%.2f　出账后已收 ¥%.2f　当前未收 ¥%.2f", float64(bill.TotalAmountCents)/100, float64(sumIssuedReceived(bill.Lines))/100, float64(bill.ReceivedCents)/100, float64(bill.DueAmountCents)/100), "T", 1, "R", false, 0, "")
	var out bytes.Buffer
	if err := p.Output(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func sumIssuedReceived(lines []app.SettlementLineSummary) int64 {
	var total int64
	for _, line := range lines {
		total += line.IssuedReceivedCents
	}
	return total
}
