package pdf

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	app "orderapp/internal/application/customerfulfillment"

	"github.com/jung-kurt/gofpdf"
)

func RenderCustomerOrderFeeAccount(d app.AccountData) ([]byte, error) {
	font, err := (SalesOrderRenderer{}).resolveFontPath()
	if err != nil {
		return nil, err
	}
	p := gofpdf.New("P", "mm", "A4", filepath.Dir(font))
	p.SetMargins(14, 12, 14)
	p.SetAutoPageBreak(false, 0)
	p.AddUTF8Font("account", "", filepath.Base(font))
	p.SetFooterFunc(func() {
		p.SetY(-12)
		p.SetFont("account", "", 8)
		p.CellFormat(0, 5, fmt.Sprintf("第 %d 页", p.PageNo()), "", 0, "C", false, 0, "")
	})
	money := func(value int64) string { return fmt.Sprintf("%.2f", float64(value)/100) }
	widths := []float64{20, 36, 32, 37, 35, 22}
	labels := []string{"发生日期", "费用及商品", "来源/运单", "发货时间/重量", "订单号", "金额"}
	newPage := func(first bool) {
		p.AddPage()
		p.SetXY(14, 12)
		if first {
			p.SetFont("account", "", 17)
			p.CellFormat(0, 9, "客户费用明细", "", 1, "L", false, 0, "")
			p.SetFont("account", "", 10)
			p.CellFormat(0, 6, fmt.Sprintf("%s　账期：%s 至 %s", d.CustomerName, d.DateFrom, d.DateTo), "", 1, "L", false, 0, "")
			p.CellFormat(0, 6, "生成时间："+d.AsOf, "", 1, "L", false, 0, "")
			p.MultiCell(0, 5, fmt.Sprintf("商品货款 ¥%s　加工费 ¥%s　代发服务费 ¥%s\n运费 ¥%s　其他 ¥%s\n应付 ¥%s　已付 ¥%s　未付 ¥%s", money(d.FeeSummary.GoodsCents), money(d.FeeSummary.ProcessingCents), money(d.FeeSummary.DirectShipServiceCents), money(d.FeeSummary.ShippingCents), money(d.FeeSummary.OtherCents), money(d.FeeSummary.PayableCents), money(d.FeeSummary.PaidCents), money(d.FeeSummary.DueCents)), "", "L", false)
			p.Ln(3)
		} else {
			p.SetFont("account", "", 11)
			p.CellFormat(0, 7, fmt.Sprintf("客户费用明细（续）　%s　%s 至 %s", d.CustomerName, d.DateFrom, d.DateTo), "", 1, "L", false, 0, "")
			p.Ln(1)
		}
		p.SetFont("account", "", 9)
		for i, label := range labels {
			p.CellFormat(widths[i], 7, label, "1", 0, "C", false, 0, "")
		}
		p.Ln(-1)
	}
	newPage(true)
	drawGroup := func(group app.AccountOrderFeeGroup) {
		if p.GetY() > 263 {
			newPage(false)
		}
		p.SetFont("account", "", 10)
		name := group.Order.OrderNo
		if group.Unlinked {
			name = "未关联订单费用"
		}
		p.CellFormat(0, 6, fmt.Sprintf("%s　应付 ¥%s　已付 ¥%s　未付 ¥%s", name, money(group.PayableCents), money(group.PaidCents), money(group.DueCents)), "B", 1, "L", false, 0, "")
	}
	drawFee := func(group app.AccountOrderFeeGroup, line app.AccountFeeLine) {
		name := line.FeeName
		if name == "" {
			name = app.AccountFeeLabel(line.FeeType)
		}
		if line.Category == "product" && len(group.Order.Items) > 0 {
			items := make([]string, 0, len(group.Order.Items))
			for _, item := range group.Order.Items {
				items = append(items, fmt.Sprintf("%s %s × %s%s，单价¥%s，折后¥%s", item.Name, item.Spec, item.Quantity, item.Unit, item.UnitPrice, item.LineTotal))
			}
			name += "：" + strings.Join(items, "；")
			if group.Order.DiscountCents != 0 {
				name += fmt.Sprintf("；订单优惠¥%s", money(group.Order.DiscountCents))
			}
		}
		tracking := line.TrackingNo
		shipmentText := "未记录"
		shipments := group.Shipments
		if line.TrackingNo != "" {
			shipments = make([]app.AccountShipment, 0, 1)
			for _, shipment := range group.Shipments {
				if shipment.TrackingNo == line.TrackingNo {
					shipments = append(shipments, shipment)
					break
				}
			}
		}
		if len(shipments) > 0 {
			waybills := make([]string, 0, len(shipments))
			var timeWeight []string
			for _, shipment := range shipments {
				if shipment.TrackingNo != "" {
					waybills = append(waybills, shipment.TrackingNo)
				}
				if shipment.ShippedAt != "" || shipment.SettlementWeightKG != "" {
					timeValue, weight := shipment.ShippedAt, shipment.SettlementWeightKG
					if timeValue == "" {
						timeValue = "未记录"
					} else if shipment.Source != "" {
						timeValue += "（" + shipment.Source + "）"
					}
					if weight == "" {
						weight = "未记录"
					}
					timeWeight = append(timeWeight, timeValue+" / "+weight+"kg")
				}
			}
			if tracking == "" {
				tracking = strings.Join(waybills, ", ")
			}
			if len(timeWeight) > 0 {
				shipmentText = strings.Join(timeWeight, "; ")
			}
		}
		source := line.SourceType
		if source != "" {
			source += " " + fmt.Sprint(line.SourceID)
		}
		values := []string{line.OccurredAt, name, source + "\n" + tracking, shipmentText, group.Order.OrderNo, "¥" + money(line.AmountCents)}
		lineTexts := make([][]string, len(values))
		lines := 1
		for i, value := range values {
			lineTexts[i] = p.SplitText(value, widths[i]-2)
			if len(lineTexts[i]) > lines {
				lines = len(lineTexts[i])
			}
		}
		height := float64(lines)*4.6 + 2
		if p.GetY()+height > 278 {
			newPage(false)
			drawGroup(group)
		}
		y, x := p.GetY(), 14.0
		for i, valueLines := range lineTexts {
			p.Rect(x, y, widths[i], height, "D")
			p.SetXY(x+1, y+1)
			p.SetFont("account", "", 8)
			p.MultiCell(widths[i]-2, 4.6, strings.Join(valueLines, "\n"), "", "L", false)
			x += widths[i]
		}
		p.SetY(y + height)
	}
	for _, group := range d.OrderFeeGroups {
		drawGroup(group)
		if len(group.FeeLines) == 0 {
			p.SetFont("account", "", 9)
			p.CellFormat(0, 6, "当前账期无可列示费用明细", "", 1, "L", false, 0, "")
		}
		for _, line := range group.FeeLines {
			drawFee(group, line)
		}
		p.Ln(1)
	}
	var output bytes.Buffer
	if err = p.Output(&output); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
