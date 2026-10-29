package excel

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
	app "orderapp/internal/application/customerfulfillment"
)

func RenderCustomerBill(bill app.BillingBill) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetSheetName("Sheet1", "费用明细"); err != nil {
		return nil, err
	}
	money := func(v int64) float64 { return float64(v) / 100 }
	issuedReceived := int64(0)
	chargeTotals := make(map[string]int64)
	for _, line := range bill.Lines {
		issuedReceived += line.IssuedReceivedCents
		chargeTotals[line.ChargeType] += line.AmountCents
	}
	chargeTypes := make([]string, 0, len(chargeTotals))
	for chargeType := range chargeTotals {
		chargeTypes = append(chargeTypes, chargeType)
	}
	sort.Strings(chargeTypes)
	head := [][]any{
		{"客户", bill.CustomerName, "账单号", bill.SettlementNo},
		{"状态", map[string]string{"preview": "出账预览", "confirmed": "已出账", "paid": "已收清", "settled": "已出账"}[bill.Status]},
		{"出账日期", bill.IssuedAt, "费用期间", bill.PeriodFrom + " 至 " + bill.PeriodTo},
		{"账单金额", money(bill.TotalAmountCents), "出账前已收", money(issuedReceived), "当前已收", money(bill.ReceivedCents), "当前未收", money(bill.DueAmountCents)},
	}
	for i, row := range head {
		if err := f.SetSheetRow("费用明细", fmt.Sprintf("A%d", i+1), &row); err != nil {
			return nil, err
		}
	}
	if err := f.SetSheetRow("费用明细", "A5", &[]any{"费用类型分类小计", "金额（元）"}); err != nil {
		return nil, err
	}
	for i, chargeType := range chargeTypes {
		row := []any{app.AccountFeeLabel(chargeType), money(chargeTotals[chargeType])}
		if err := f.SetSheetRow("费用明细", fmt.Sprintf("A%d", i+6), &row); err != nil {
			return nil, err
		}
	}
	headerRow := 6 + len(chargeTypes)
	if err := f.SetSheetRow("费用明细", fmt.Sprintf("A%d", headerRow), &[]any{"费用类型", "发生日期", "来源单号", "费用名称", "应收", "出账金额", "出账前已收", "当前已收", "当前未收", "订单及发货明细"}); err != nil {
		return nil, err
	}
	for i, line := range bill.Lines {
		detail := ""
		if line.Detail != nil {
			detail = strings.Join(app.BillingDetailText(*line.Detail), "\n")
		}
		row := []any{app.AccountFeeLabel(line.ChargeType), line.OccurredAt, line.SourceNo, line.Description, money(line.ReceivableCents), money(line.AmountCents), money(line.IssuedReceivedCents), money(line.ReceivedCents), money(line.DueCents), detail}
		if err := f.SetSheetRow("费用明细", fmt.Sprintf("A%d", i+headerRow+1), &row); err != nil {
			return nil, err
		}
	}
	if err := f.SetColWidth("费用明细", "J", "J", 70); err != nil {
		return nil, err
	}
	style, err := f.NewStyle(&excelize.Style{Alignment: &excelize.Alignment{WrapText: true, Vertical: "top"}})
	if err != nil {
		return nil, err
	}
	if err := f.SetCellStyle("费用明细", fmt.Sprintf("J%d", headerRow+1), fmt.Sprintf("J%d", headerRow+len(bill.Lines)+1), style); err != nil {
		return nil, err
	}
	if _, err := f.NewSheet("订单明细"); err != nil {
		return nil, err
	}
	if err := f.SetSheetRow("订单明细", "A1", &[]any{"订单号", "费用类型", "本次出账金额", "信息来源", "商品", "规格", "数量", "单位", "单价", "商品金额", "订单优惠", "发货时间", "运单号", "寄件重量（kg）"}); err != nil {
		return nil, err
	}
	rowNum := 2
	parseNumber := func(s string) any {
		if n, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
			return n
		}
		return ""
	}
	for _, line := range bill.Lines {
		if line.Detail == nil {
			continue
		}
		d := line.Detail
		rows := make([][]any, 0, len(d.Items)+len(d.Shipments))
		for _, item := range d.Items {
			rows = append(rows, []any{d.OrderNo, app.AccountFeeLabel(line.ChargeType), nil, d.InformationSource, item.Name, item.Spec, parseNumber(item.Quantity), item.Unit, parseNumber(item.UnitPrice), parseNumber(item.LineTotal), nil, "", "", ""})
		}
		for _, shipment := range d.Shipments {
			rows = append(rows, []any{d.OrderNo, app.AccountFeeLabel(line.ChargeType), nil, d.InformationSource, "", "", "", "", "", "", nil, shipment.ShippedAt, shipment.TrackingNo, parseNumber(shipment.SettlementWeightKG)})
		}
		if len(rows) == 0 {
			rows = append(rows, []any{d.OrderNo, app.AccountFeeLabel(line.ChargeType), nil, d.InformationSource, "未记录"})
		}
		rows[0][2] = money(line.AmountCents)
		if len(rows[0]) > 10 && (line.ChargeType == "order_goods" || line.ChargeType == "product") {
			rows[0][10] = money(d.DiscountCents)
		}
		for _, row := range rows {
			if err := f.SetSheetRow("订单明细", fmt.Sprintf("A%d", rowNum), &row); err != nil {
				return nil, err
			}
			rowNum++
		}
	}
	if err := f.SetColWidth("订单明细", "A", "N", 20); err != nil {
		return nil, err
	}
	if err := f.SetColWidth("费用明细", "A", "I", 20); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := f.Write(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
