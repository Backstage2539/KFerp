package customerfulfillment

import (
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

var numberRE = regexp.MustCompile(`[-+]?\d+(?:\.\d+)?`)

func ParseWorkbook(importType ImportType, r io.Reader) (ParsedWorkbook, error) {
	wb, err := excelize.OpenReader(r)
	if err != nil {
		return ParsedWorkbook{}, err
	}
	defer func() { _ = wb.Close() }()

	parsed := ParsedWorkbook{ImportType: importType}
	switch importType {
	case ImportTypeProcessingWorkbook:
		parseProcessingWorkbook(wb, &parsed)
	case ImportTypeDirectShipWorkbook:
		parseDirectShipWorkbook(wb, &parsed)
	case ImportTypeSettlementWorkbook:
		parseSettlementWorkbook(wb, &parsed)
	default:
		return ParsedWorkbook{}, fmt.Errorf("import type invalid")
	}
	return parsed, nil
}

func parseProcessingWorkbook(wb *excelize.File, parsed *ParsedWorkbook) {
	parseRawBeanReceiptSheet(wb, parsed)
	parseRawBeanIssueSheet(wb, parsed)
	parseRawBeanBalanceSheet(wb, parsed)
	parseProcessingOrderSheet(wb, parsed)
	parsePackagingJobSheet(wb, parsed)
	parseConversionJobSheet(wb, parsed)
	parseCustomerSKUSheet(wb, parsed)
	parsePackagingBalanceSheet(wb, parsed)
}

func parseRawBeanReceiptSheet(wb *excelize.File, parsed *ParsedWorkbook) {
	const sheet = "生豆入库表"
	rows := rowsForSheet(wb, sheet)
	if len(rows) < 2 {
		return
	}
	headers := headerMap(rows[0])
	for i, row := range rows[1:] {
		bean := valueByHeader(row, headers, "生豆名称", "品名", "名称")
		qtyText := valueByHeader(row, headers, "入库重量", "重量", "数量", "入库数量")
		qtyG, ok := parseQtyG(qtyText)
		if bean == "" && qtyText == "" {
			continue
		}
		receiptNo := valueByHeader(row, headers, "入库单号", "单号", "编号")
		errText := ""
		if bean == "" || !ok {
			errText = "生豆名称或入库重量无效"
		}
		addParsedRow(parsed, ParsedRow{
			SheetName:   sheet,
			RowNo:       i + 2,
			RowType:     "raw_bean_receipt",
			ExternalKey: externalKey("raw_bean_receipt", fallback(receiptNo, strconv.Itoa(i+2)), bean),
			Payload: map[string]any{
				"date":          parseExcelDateText(valueByHeader(row, headers, "日期", "入库日期")),
				"receipt_no":    receiptNo,
				"raw_bean_name": bean,
				"quantity_g":    qtyG,
				"warehouse":     valueByHeader(row, headers, "仓库", "存放仓库"),
			},
			Error: errText,
		})
	}
}

func parseRawBeanIssueSheet(wb *excelize.File, parsed *ParsedWorkbook) {
	const sheet = "生豆出库表"
	rows := rowsForSheet(wb, sheet)
	if len(rows) < 2 {
		return
	}
	headers := headerMap(rows[0])
	for i, row := range rows[1:] {
		bean := valueByHeader(row, headers, "生豆名称", "品名", "名称")
		qtyText := valueByHeader(row, headers, "出库重量", "重量", "数量", "出库数量")
		qtyG, ok := parseQtyG(qtyText)
		if bean == "" && qtyText == "" {
			continue
		}
		issueNo := valueByHeader(row, headers, "出库单号", "单号", "编号")
		errText := ""
		if bean == "" || !ok {
			errText = "生豆名称或出库重量无效"
		}
		addParsedRow(parsed, ParsedRow{
			SheetName:   sheet,
			RowNo:       i + 2,
			RowType:     "raw_bean_issue",
			ExternalKey: externalKey("raw_bean_issue", fallback(issueNo, strconv.Itoa(i+2)), bean),
			Payload: map[string]any{
				"date":          parseExcelDateText(valueByHeader(row, headers, "日期", "出库日期")),
				"issue_no":      issueNo,
				"raw_bean_name": bean,
				"quantity_g":    qtyG,
				"warehouse":     valueByHeader(row, headers, "仓库", "存放仓库"),
			},
			Error: errText,
		})
	}
}

func parseRawBeanBalanceSheet(wb *excelize.File, parsed *ParsedWorkbook) {
	const sheet = "生豆库存表"
	rows := rowsForSheet(wb, sheet)
	if len(rows) < 2 {
		return
	}
	headers := headerMap(rows[0])
	for i, row := range rows[1:] {
		bean := valueByHeader(row, headers, "生豆名称", "品名", "名称")
		qtyText := valueByHeader(row, headers, "库存重量", "库存", "剩余重量", "数量")
		qtyG, ok := parseQtyG(qtyText)
		if bean == "" && qtyText == "" {
			continue
		}
		errText := ""
		if bean == "" || !ok {
			errText = "生豆名称或库存重量无效"
		}
		addParsedRow(parsed, ParsedRow{
			SheetName:   sheet,
			RowNo:       i + 2,
			RowType:     "raw_bean_balance",
			ExternalKey: externalKey("raw_bean_balance", bean),
			Payload: map[string]any{
				"raw_bean_name": bean,
				"quantity_g":    qtyG,
				"warehouse":     valueByHeader(row, headers, "仓库", "存放仓库"),
			},
			Error: errText,
		})
	}
}

func parseProcessingOrderSheet(wb *excelize.File, parsed *ParsedWorkbook) {
	const sheet = "生产工单"
	rows := rowsForSheet(wb, sheet)
	if len(rows) < 2 {
		return
	}
	headers := headerMap(rows[0])
	for i, row := range rows[1:] {
		workOrderNo := valueByHeader(row, headers, "工单编号", "工单号", "编号")
		productName := valueByHeader(row, headers, "产品名称", "商品名称", "SKU")
		rawBeanName := valueByHeader(row, headers, "生豆名称", "生豆")
		inputQtyG, inputOK := parseQtyG(valueByHeader(row, headers, "投豆量", "生豆用量", "投豆重量"))
		plannedUnits, unitsOK := parseQtyUnits(valueByHeader(row, headers, "计划产量", "成品数量", "数量"))
		if workOrderNo == "" && productName == "" && rawBeanName == "" {
			continue
		}
		errText := ""
		if workOrderNo == "" || productName == "" || !inputOK || !unitsOK {
			errText = "工单编号、产品名称、投豆量或计划产量无效"
		}
		addParsedRow(parsed, ParsedRow{
			SheetName:   sheet,
			RowNo:       i + 2,
			RowType:     "processing_work_order",
			ExternalKey: externalKey("processing_work_order", fallback(workOrderNo, strconv.Itoa(i+2))),
			Payload: map[string]any{
				"date":                 parseExcelDateText(valueByHeader(row, headers, "日期", "生产日期")),
				"work_order_no":        workOrderNo,
				"product_name":         productName,
				"raw_bean_name":        rawBeanName,
				"input_quantity_g":     inputQtyG,
				"planned_output_units": plannedUnits,
				"status":               valueByHeader(row, headers, "状态", "完成情况"),
			},
			Error: errText,
		})
	}
}

func parsePackagingJobSheet(wb *excelize.File, parsed *ParsedWorkbook) {
	const sheet = "生产子工单-包装"
	rows := rowsForSheet(wb, sheet)
	if len(rows) < 2 {
		return
	}
	headers := headerMap(rows[0])
	for i, row := range rows[1:] {
		workOrderNo := valueByHeader(row, headers, "工单编号", "工单号", "生产工单")
		productName := valueByHeader(row, headers, "产品名称", "商品名称", "SKU")
		packagingName := valueByHeader(row, headers, "包装耗材", "耗材名称", "包装")
		units, ok := parseQtyUnits(valueByHeader(row, headers, "数量", "包装数量"))
		if workOrderNo == "" && productName == "" && packagingName == "" {
			continue
		}
		errText := ""
		if workOrderNo == "" || productName == "" || !ok {
			errText = "工单编号、产品名称或包装数量无效"
		}
		addParsedRow(parsed, ParsedRow{
			SheetName:   sheet,
			RowNo:       i + 2,
			RowType:     "packaging_job",
			ExternalKey: externalKey("packaging_job", fallback(workOrderNo, strconv.Itoa(i+2)), productName, packagingName),
			Payload: map[string]any{
				"work_order_no":  workOrderNo,
				"product_name":   productName,
				"packaging_name": packagingName,
				"quantity_units": units,
			},
			Error: errText,
		})
	}
}

func parseConversionJobSheet(wb *excelize.File, parsed *ParsedWorkbook) {
	const sheet = "库存转换工单"
	rows := rowsForSheet(wb, sheet)
	if len(rows) < 2 {
		return
	}
	headers := headerMap(rows[0])
	for i, row := range rows[1:] {
		jobNo := valueByHeader(row, headers, "工单编号", "转换单号", "编号")
		fromProduct := valueByHeader(row, headers, "转换前产品", "来源产品")
		toProduct := valueByHeader(row, headers, "转换后产品", "目标产品")
		units, ok := parseQtyUnits(valueByHeader(row, headers, "数量", "转换数量"))
		if jobNo == "" && fromProduct == "" && toProduct == "" {
			continue
		}
		errText := ""
		if fromProduct == "" || toProduct == "" || !ok {
			errText = "转换前产品、转换后产品或数量无效"
		}
		addParsedRow(parsed, ParsedRow{
			SheetName:   sheet,
			RowNo:       i + 2,
			RowType:     "conversion_job",
			ExternalKey: externalKey("conversion_job", fallback(jobNo, strconv.Itoa(i+2)), fromProduct, toProduct),
			Payload: map[string]any{
				"job_no":         jobNo,
				"from_product":   fromProduct,
				"to_product":     toProduct,
				"quantity_units": units,
			},
			Error: errText,
		})
	}
}

func parseCustomerSKUSheet(wb *excelize.File, parsed *ParsedWorkbook) {
	const sheet = "SKU"
	rows := rowsForSheet(wb, sheet)
	if len(rows) < 2 {
		return
	}
	headers := headerMap(rows[0])
	for i, row := range rows[1:] {
		skuCode := valueByHeader(row, headers, "SKU编码", "SKU", "编码")
		skuName := valueByHeader(row, headers, "产品名称", "商品名称", "名称")
		if skuCode == "" && skuName == "" {
			continue
		}
		errText := ""
		if skuName == "" {
			errText = "产品名称无效"
		}
		addParsedRow(parsed, ParsedRow{
			SheetName:   sheet,
			RowNo:       i + 2,
			RowType:     "customer_sku",
			ExternalKey: externalKey("customer_sku", fallback(skuCode, skuName)),
			Payload: map[string]any{
				"sku_code":     skuCode,
				"sku_name":     skuName,
				"spec":         valueByHeader(row, headers, "规格", "商品规格"),
				"roast_degree": valueByHeader(row, headers, "烘焙度", "烘焙程度"),
			},
			Error: errText,
		})
	}
}

func parsePackagingBalanceSheet(wb *excelize.File, parsed *ParsedWorkbook) {
	const sheet = "耗材库存（预估）"
	rows := rowsForSheet(wb, sheet)
	if len(rows) < 2 {
		return
	}
	headers := headerMap(rows[0])
	for i, row := range rows[1:] {
		name := valueByHeader(row, headers, "耗材名称", "包装耗材", "名称")
		units, ok := parseQtyUnits(valueByHeader(row, headers, "库存数量", "库存", "数量"))
		if name == "" {
			continue
		}
		errText := ""
		if !ok {
			errText = "耗材库存数量无效"
		}
		addParsedRow(parsed, ParsedRow{
			SheetName:   sheet,
			RowNo:       i + 2,
			RowType:     "packaging_balance",
			ExternalKey: externalKey("packaging_balance", name),
			Payload: map[string]any{
				"packaging_name": name,
				"quantity_units": units,
			},
			Error: errText,
		})
	}
}

func parseDirectShipWorkbook(wb *excelize.File, parsed *ParsedWorkbook) {
	const sheet = "代发信息"
	rows := rowsForSheet(wb, sheet)
	if len(rows) < 2 {
		return
	}
	headers := headerMap(rows[0])
	type carriedHeader struct {
		Date    string
		Seq     string
		OrderNo string
		Address string
		Remark  string
		Waybill string
		ShipAt  string
		Status  string
	}
	var carried carriedHeader
	createdOrders := map[string]bool{}
	for i, row := range rows[1:] {
		date := parseExcelDateText(valueByHeader(row, headers, "时间", "日期", "下单时间"))
		seq := valueByHeader(row, headers, "序号")
		orderNo := valueByHeader(row, headers, "订单编号", "订单号")
		address := valueByHeader(row, headers, "收货地址", "地址")
		remark := valueByHeader(row, headers, "备注")
		waybill := valueByHeader(row, headers, "运单号", "快递单号")
		shipAt := parseExcelDateText(valueByHeader(row, headers, "发货日期", "发货时间"))
		status := valueByHeader(row, headers, "状态")
		if orderNo != "" || address != "" || date != "" {
			if date != "" {
				carried.Date = date
			}
			if seq != "" {
				carried.Seq = seq
			}
			if orderNo != "" {
				carried.OrderNo = orderNo
			}
			if address != "" {
				carried.Address = address
			}
			if remark != "" {
				carried.Remark = remark
			}
			if waybill != "" {
				carried.Waybill = waybill
			}
			if shipAt != "" {
				carried.ShipAt = shipAt
			}
			if status != "" {
				carried.Status = status
			}
		}
		productTitle := valueByHeader(row, headers, "商品标题", "产品名称", "商品名称")
		if carried.OrderNo == "" && productTitle == "" {
			continue
		}
		if carried.OrderNo != "" && !createdOrders[carried.OrderNo] {
			createdOrders[carried.OrderNo] = true
			addParsedRow(parsed, ParsedRow{
				SheetName:   sheet,
				RowNo:       i + 2,
				RowType:     "direct_ship_order",
				ExternalKey: externalKey("direct_ship_order", carried.OrderNo),
				Payload: map[string]any{
					"order_date":       carried.Date,
					"sequence_no":      carried.Seq,
					"order_no":         carried.OrderNo,
					"receiver_address": carried.Address,
					"remark":           carried.Remark,
					"waybill_no":       carried.Waybill,
					"ship_date":        carried.ShipAt,
					"status":           carried.Status,
				},
			})
		}
		if productTitle == "" {
			continue
		}
		quantityText := valueByHeader(row, headers, "数量")
		units, ok := parseQtyUnits(quantityText)
		if !ok && quantityText == "" && carried.OrderNo != "" {
			units = 1
			ok = true
		}
		errText := ""
		if carried.OrderNo == "" || !ok {
			errText = "订单编号或数量无效"
		}
		addParsedRow(parsed, ParsedRow{
			SheetName:   sheet,
			RowNo:       i + 2,
			RowType:     "direct_ship_item",
			ExternalKey: externalKey("direct_ship_item", fallback(carried.OrderNo, strconv.Itoa(i+2)), strconv.Itoa(i+2), productTitle),
			Payload: map[string]any{
				"order_date":       carried.Date,
				"sequence_no":      carried.Seq,
				"order_no":         carried.OrderNo,
				"receiver_address": carried.Address,
				"product_title":    productTitle,
				"attribute":        valueByHeader(row, headers, "属性"),
				"spec":             valueByHeader(row, headers, "商品规格", "规格"),
				"quantity_units":   units,
				"grind_service":    valueByHeader(row, headers, "磨粉服务"),
				"remark":           firstNonEmpty(valueByHeader(row, headers, "备注"), carried.Remark),
				"waybill_no":       firstNonEmpty(valueByHeader(row, headers, "运单号", "快递单号"), carried.Waybill),
				"ship_date":        firstNonEmpty(parseExcelDateText(valueByHeader(row, headers, "发货日期", "发货时间")), carried.ShipAt),
				"status":           firstNonEmpty(valueByHeader(row, headers, "状态"), carried.Status),
			},
			Error: errText,
		})
	}
}

func parseSettlementWorkbook(wb *excelize.File, parsed *ParsedWorkbook) {
	for _, sheet := range wb.GetSheetList() {
		rows := rowsForSheet(wb, sheet)
		section := ""
		for i, row := range rows {
			if len(row) == 0 {
				continue
			}
			name := normalizedCell(cell(row, 0))
			if name == "" {
				continue
			}
			if name == "项目" || name == "费用项目" || strings.Contains(name, "合计") {
				continue
			}
			feeType := settlementFeeType(name, section)
			if feeType == "" {
				if settlementSectionName(name) != "" {
					section = settlementSectionName(name)
				}
				continue
			}
			if len(row) <= 1 {
				if settlementSectionName(name) != "" {
					section = settlementSectionName(name)
				}
				continue
			}
			amountCents, amountOK := parseAmountCents(firstNonEmpty(cell(row, 3), cell(row, 2)))
			errText := ""
			if !amountOK {
				errText = "费用金额无效"
			}
			qtyText := cell(row, 1)
			payload := map[string]any{
				"fee_type":      feeType,
				"fee_name":      name,
				"section":       section,
				"quantity_text": normalizedCell(qtyText),
				"amount_cents":  amountCents,
			}
			if qtyG, ok := parseQtyG(qtyText); ok {
				payload["quantity_g"] = qtyG
			}
			if units, ok := parseQtyUnits(qtyText); ok {
				payload["quantity_units"] = units
			}
			if unitPriceCents, ok := parseAmountCents(cell(row, 2)); ok {
				payload["unit_price_cents"] = unitPriceCents
			}
			addParsedRow(parsed, ParsedRow{
				SheetName:   sheet,
				RowNo:       i + 1,
				RowType:     "fee_item",
				ExternalKey: externalKey("fee_item", feeType, strconv.Itoa(i+1), name),
				Payload:     payload,
				Error:       errText,
			})
		}
	}
}

func rowsForSheet(wb *excelize.File, sheet string) [][]string {
	idx, err := wb.GetSheetIndex(sheet)
	if err != nil || idx < 0 {
		return nil
	}
	rows, err := wb.GetRows(sheet)
	if err != nil {
		return nil
	}
	return rows
}

func headerMap(header []string) map[string]int {
	out := make(map[string]int, len(header))
	for i, v := range header {
		key := normalizedCell(v)
		if key != "" {
			out[key] = i
		}
	}
	return out
}

func valueByHeader(row []string, headers map[string]int, names ...string) string {
	for _, name := range names {
		if idx, ok := headers[normalizedCell(name)]; ok {
			return normalizedCell(cell(row, idx))
		}
	}
	return ""
}

func cell(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return row[idx]
}

func normalizedCell(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func parseQtyG(value string) (int64, bool) {
	clean := strings.ToLower(strings.ReplaceAll(normalizedCell(value), ",", ""))
	if clean == "" {
		return 0, false
	}
	match := numberRE.FindString(clean)
	if match == "" {
		return 0, false
	}
	n, err := strconv.ParseFloat(match, 64)
	if err != nil {
		return 0, false
	}
	if strings.Contains(clean, "kg") || strings.Contains(clean, "公斤") || strings.Contains(clean, "千克") {
		n *= 1000
	}
	return int64(math.Round(n)), true
}

func parseQtyUnits(value string) (int64, bool) {
	clean := strings.ReplaceAll(normalizedCell(value), ",", "")
	if clean == "" {
		return 0, false
	}
	match := numberRE.FindString(clean)
	if match == "" {
		return 0, false
	}
	n, err := strconv.ParseFloat(match, 64)
	if err != nil {
		return 0, false
	}
	return int64(math.Round(n)), true
}

func parseExcelDateText(value string) string {
	clean := normalizedCell(value)
	if clean == "" {
		return ""
	}
	layouts := []string{
		"2006-01-02",
		"2006-1-2",
		"2006/01/02",
		"2006/1/2",
		"2006-01-02 15:04:05",
		"2006-1-2 15:04:05",
		"2006/01/02 15:04:05",
		"2006/1/2 15:04:05",
	}
	for _, layout := range layouts {
		if ts, err := time.ParseInLocation(layout, clean, time.Local); err == nil {
			return ts.Format("2006-01-02")
		}
	}
	if n, err := strconv.ParseFloat(strings.ReplaceAll(clean, ",", ""), 64); err == nil && n > 1000 && n < 100000 {
		if ts, err := excelize.ExcelDateToTime(n, false); err == nil {
			return ts.Format("2006-01-02")
		}
	}
	return clean
}

func parseAmountCents(value string) (int64, bool) {
	clean := normalizedCell(value)
	clean = strings.ReplaceAll(clean, ",", "")
	clean = strings.ReplaceAll(clean, "¥", "")
	clean = strings.ReplaceAll(clean, "￥", "")
	clean = strings.ReplaceAll(clean, "元", "")
	if clean == "" {
		return 0, false
	}
	match := numberRE.FindString(clean)
	if match == "" {
		return 0, false
	}
	n, err := strconv.ParseFloat(match, 64)
	if err != nil {
		return 0, false
	}
	return int64(math.Round(n * 100)), true
}

func settlementSectionName(name string) string {
	switch {
	case strings.Contains(name, "代发") || strings.Contains(name, "仓储"):
		return "direct_ship_storage"
	case strings.Contains(name, "物流"):
		return "shipping"
	case strings.Contains(name, "烘焙"):
		return "roasting"
	case strings.Contains(name, "装袋") || strings.Contains(name, "挂靠") || strings.Contains(name, "磨粉"):
		return "processing"
	default:
		return ""
	}
}

func settlementFeeType(name, section string) string {
	switch {
	case strings.Contains(name, "烘焙费"):
		return "roasting"
	case strings.Contains(name, "代发费"):
		return "direct_ship_service"
	case strings.Contains(name, "仓储"):
		return "storage"
	case strings.Contains(name, "物流") || section == "shipping":
		return "shipping"
	case strings.Contains(name, "新品") || strings.Contains(name, "测试"):
		return "new_product_test"
	case strings.Contains(name, "磨粉"):
		return "grinding"
	case strings.Contains(name, "装袋"):
		return "bagging"
	case strings.Contains(name, "挂靠"):
		return "sc_license"
	case strings.Contains(name, "耗材") || strings.Contains(name, "包装"):
		return "packaging"
	default:
		return ""
	}
}

func addParsedRow(parsed *ParsedWorkbook, row ParsedRow) {
	parsed.Rows = append(parsed.Rows, row)
	parsed.Summary.TotalRows++
	if row.Error == "" {
		parsed.Summary.ValidRows++
	} else {
		parsed.Summary.InvalidRows++
	}
	switch row.RowType {
	case "raw_bean_receipt":
		parsed.Summary.RawBeanReceipts++
	case "raw_bean_issue":
		parsed.Summary.RawBeanIssues++
	case "raw_bean_balance":
		parsed.Summary.RawBeanBalances++
	case "customer_sku":
		parsed.Summary.CustomerSKUs++
	case "packaging_balance":
		parsed.Summary.PackagingBalances++
	case "processing_work_order":
		parsed.Summary.ProcessingOrders++
	case "packaging_job":
		parsed.Summary.PackagingJobs++
	case "conversion_job":
		parsed.Summary.ConversionJobs++
	case "direct_ship_order":
		parsed.Summary.DirectShipOrders++
	case "direct_ship_item":
		parsed.Summary.DirectShipItems++
	case "fee_item":
		parsed.Summary.FeeItems++
	case "settlement_batch":
		parsed.Summary.SettlementBatches++
	}
}

func externalKey(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if v := normalizedCell(part); v != "" {
			out = append(out, v)
		}
	}
	return strings.Join(out, ":")
}

func fallback(value, fallback string) string {
	if normalizedCell(value) != "" {
		return normalizedCell(value)
	}
	return normalizedCell(fallback)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if v := normalizedCell(value); v != "" {
			return v
		}
	}
	return ""
}
