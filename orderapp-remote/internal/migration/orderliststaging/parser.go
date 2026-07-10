package orderliststaging

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

var monthlySheetPattern = regexp.MustCompile(`^(\d{4})年(\d{1,2})月$`)

func PrepareWorkbook(path string, opts PrepareOptions) (Dataset, error) {
	if opts.StartPeriod == "" {
		opts.StartPeriod = "2025-01"
	}
	if opts.EndPeriod == "" {
		opts.EndPeriod = "2026-06"
	}
	sha, size, err := fileSHA256(path)
	if err != nil {
		return Dataset{}, err
	}
	wb, err := excelize.OpenFile(path, excelize.Options{RawCellValue: true})
	if err != nil {
		return Dataset{}, fmt.Errorf("open workbook: %w", err)
	}
	defer wb.Close()

	now := time.Now().UTC()
	runID := fmt.Sprintf("orderlist-%s-%s-%s", strings.ReplaceAll(opts.StartPeriod, "-", ""), strings.ReplaceAll(opts.EndPeriod, "-", ""), sha[:8])
	dataset := Dataset{
		Run: ImportRun{
			RunID:      runID,
			SourcePath: path, SourceSHA256: sha, SourceBytes: size,
			StartPeriod: opts.StartPeriod, EndPeriod: opts.EndPeriod, CreatedAt: now,
			WorkbookSheetCount: len(wb.GetSheetList()),
		},
		ERPCustomers: opts.ERPCustomers,
		ERPProducts:  opts.ERPProducts,
	}

	for _, sheetName := range wb.GetSheetList() {
		period, monthly := parseSheetPeriod(sheetName)
		inventory := SheetInventory{SheetName: sheetName, Period: period}
		rows, err := wb.GetRows(sheetName, excelize.Options{RawCellValue: true})
		if err != nil {
			return Dataset{}, fmt.Errorf("read sheet %s: %w", sheetName, err)
		}
		inventory.UsedRowCount = len(rows)
		if !monthly {
			inventory.ExcludedReason = "非月度订单工作表"
			dataset.Sheets = append(dataset.Sheets, inventory)
			continue
		}
		if period < opts.StartPeriod || period > opts.EndPeriod {
			inventory.ExcludedReason = "不在本批次2025-01至2026-06范围"
			dataset.Sheets = append(dataset.Sheets, inventory)
			continue
		}
		inventory.Included = true
		dataset.Run.IncludedSheetCount++
		parsedRows, rowIssues, err := parseMonthlySheet(sheetName, period, rows)
		if err != nil {
			return Dataset{}, err
		}
		inventory.OrderRowCount = len(parsedRows)
		dataset.Sheets = append(dataset.Sheets, inventory)
		dataset.RawOrders = append(dataset.RawOrders, parsedRows...)
		dataset.Issues = append(dataset.Issues, rowIssues...)
	}

	dataset.Issues = append(dataset.Issues, AssignSourceKeys(dataset.RawOrders, opts.PreviousMappings)...)
	var customerIssues []Issue
	dataset.Customers, dataset.CustomerAliases, dataset.CustomerPhones, customerIssues = CurateCustomers(dataset.RawOrders, opts.ERPCustomers)
	dataset.Issues = append(dataset.Issues, customerIssues...)
	var productIssues []Issue
	dataset.Products, dataset.SKUs, dataset.ProductAliases, dataset.OrderItems, productIssues = CurateProducts(dataset.RawOrders, opts.ERPProducts)
	dataset.Issues = append(dataset.Issues, productIssues...)
	itemReviewOrders := map[string]struct{}{}
	for _, item := range dataset.OrderItems {
		if item.SourceOrderKey != "" && item.ReviewStatus == ReviewNeedsReview {
			itemReviewOrders[item.SourceOrderKey] = struct{}{}
		}
	}
	for i := range dataset.RawOrders {
		if _, ok := itemReviewOrders[dataset.RawOrders[i].SourceOrderKey]; ok {
			dataset.RawOrders[i].ReviewStatus = ReviewNeedsReview
		}
	}
	dataset.Orders = curateOrders(dataset.RawOrders)
	dataset.Run.RawOrderCount = len(dataset.RawOrders)
	dataset.Run.RawProductLines = len(dataset.OrderItems)
	sortDataset(&dataset)
	return dataset, nil
}

func parseMonthlySheet(sheetName, period string, rows [][]string) ([]RawOrder, []Issue, error) {
	headerRow := -1
	for i := 0; i < len(rows) && i < 6; i++ {
		if normalizeHeader(cell(rows[i], 0)) == "序号" {
			headerRow = i
			break
		}
	}
	if headerRow < 0 {
		return nil, nil, fmt.Errorf("sheet %s missing 序号 header", sheetName)
	}
	headers := map[string]int{}
	for col, raw := range rows[headerRow] {
		header := normalizeHeader(raw)
		if header != "" {
			if _, exists := headers[header]; !exists {
				headers[header] = col
			}
		}
	}

	parsed := make([]RawOrder, 0)
	issues := make([]Issue, 0)
	for rowIndex := headerRow + 1; rowIndex < len(rows); rowIndex++ {
		values := rows[rowIndex]
		if !hasOrderData(values) {
			continue
		}
		rawFields := make(map[string]string, len(headers))
		for header, col := range headers {
			rawFields[header] = strings.TrimSpace(cell(values, col))
		}
		row := RawOrder{
			SheetName: sheetName, SheetPeriod: period, SourceRowNumber: rowIndex + 1,
			SequenceOriginal:      field(rawFields, "序号"),
			OrderDateRaw:          field(rawFields, "订单日期"),
			CustomerRaw:           field(rawFields, "客户"),
			ProductRaw:            field(rawFields, "品种", "商品", "产品"),
			RemarkRaw:             field(rawFields, "备注"),
			GrindRaw:              field(rawFields, "磨粉", "是否磨粉"),
			RoastRaw:              field(rawFields, "烘焙程度", "烘焙度"),
			ShipmentStatusRaw:     field(rawFields, "发货状态"),
			PaymentStatusRaw:      field(rawFields, "付款状态"),
			AmountRaw:             field(rawFields, "货款+运费(元)", "货款+运费)", "货款(元)", "付款金额(元)"),
			ShippingAmountRaw:     field(rawFields, "运费(元)"),
			ReceiptDateRaw:        field(rawFields, "收款时间"),
			OrderSourceRaw:        field(rawFields, "订单来源"),
			ExpressFeeRaw:         field(rawFields, "快递费"),
			OrderTypeRaw:          field(rawFields, "订单类型"),
			TrackingNoRaw:         field(rawFields, "单号"),
			ShipmentDateRaw:       field(rawFields, "发货日期"),
			CreditTermRaw:         field(rawFields, "账期"),
			QuantityRaw:           field(rawFields, "数量"),
			LatestShipmentDateRaw: field(rawFields, "最晚发货日期"),
			RawFields:             rawFields, ReviewStatus: ReviewAutoReady,
		}
		row.Fingerprint = fingerprintRawOrder(row)
		if date, err := ParseDate(row.OrderDateRaw, period); err != nil {
			row.ReviewStatus = ReviewNeedsReview
			issues = append(issues, newIssue("order", rowLocator(row), "order_date_invalid", "warning", err.Error(), row))
		} else {
			row.OrderDate = date
			if date == "" {
				row.ReviewStatus = ReviewNeedsReview
				issues = append(issues, newIssue("order", rowLocator(row), "order_date_missing", "warning", "订单日期为空", row))
			}
		}
		amount := ParseAmount(row.AmountRaw)
		row.AmountValue, row.AmountDerived = amount.Value, amount.Derived
		if amount.NeedsReview {
			row.ReviewStatus = ReviewNeedsReview
			issues = append(issues, newIssue("order", rowLocator(row), "order_amount_unresolved", "warning", "金额包含说明或无法安全求值，已保留原文", row))
		}
		shipping := ParseAmount(row.ShippingAmountRaw)
		row.ShippingAmountValue = shipping.Value
		if strings.TrimSpace(row.CustomerRaw) == "" {
			row.ReviewStatus = ReviewNeedsReview
			issues = append(issues, newIssue("order", rowLocator(row), "order_customer_missing", "error", "订单未填写客户信息", row))
		}
		if strings.TrimSpace(row.ProductRaw) == "" {
			row.ReviewStatus = ReviewNeedsReview
		}
		parsed = append(parsed, row)
	}
	return parsed, issues, nil
}

func curateOrders(rows []RawOrder) []Order {
	orders := make([]Order, 0, len(rows))
	for _, row := range rows {
		if row.SourceOrderKey == "" {
			continue
		}
		orders = append(orders, Order{
			SourceOrderKey: row.SourceOrderKey, SheetName: row.SheetName,
			SequenceOriginal: row.SequenceOriginal, SequenceEffective: row.SequenceEffective,
			SourceRowNumber: row.SourceRowNumber, SourceFingerprint: row.Fingerprint,
			OrderDate: row.OrderDate, CustomerKey: row.CustomerKey, CustomerRaw: row.CustomerRaw,
			OrderSourceRaw: row.OrderSourceRaw, OrderTypeRaw: row.OrderTypeRaw,
			PaymentStatusRaw: row.PaymentStatusRaw, ShipmentStatusRaw: row.ShipmentStatusRaw,
			AmountValue: row.AmountValue, AmountRaw: row.AmountRaw, AmountDerived: row.AmountDerived,
			ShippingAmountValue: row.ShippingAmountValue, ShippingAmountRaw: row.ShippingAmountRaw,
			TrackingNoRaw: row.TrackingNoRaw, RemarkRaw: row.RemarkRaw, ReviewStatus: row.ReviewStatus,
		})
	}
	return orders
}

func parseSheetPeriod(sheetName string) (string, bool) {
	normalized := strings.ReplaceAll(strings.TrimSpace(sheetName), " ", "")
	if normalized == "026年6月" {
		return "2026-06", true
	}
	match := monthlySheetPattern.FindStringSubmatch(normalized)
	if len(match) != 3 {
		return "", false
	}
	month := 0
	_, _ = fmt.Sscanf(match[2], "%d", &month)
	if month < 1 || month > 12 {
		return "", false
	}
	return fmt.Sprintf("%s-%02d", match[1], month), true
}

func normalizeHeader(raw string) string {
	raw = strings.TrimSpace(raw)
	replacer := strings.NewReplacer("（", "(", "）", ")", " ", "", "\n", "", "\r", "")
	return replacer.Replace(raw)
}

func hasOrderData(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func cell(row []string, col int) string {
	if col < 0 || col >= len(row) {
		return ""
	}
	return row[col]
}

func field(values map[string]string, aliases ...string) string {
	for _, alias := range aliases {
		if value, ok := values[normalizeHeader(alias)]; ok {
			return value
		}
	}
	return ""
}

func fingerprintRawOrder(row RawOrder) string {
	payload := struct {
		SheetName        string            `json:"sheet_name"`
		SequenceOriginal string            `json:"sequence_original"`
		RawFields        map[string]string `json:"raw_fields"`
	}{row.SheetName, row.SequenceOriginal, row.RawFields}
	b, _ := json.Marshal(payload)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func fileSHA256(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", 0, err
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hash.Sum(nil)), info.Size(), nil
}

func sortDataset(dataset *Dataset) {
	sort.Slice(dataset.Sheets, func(i, j int) bool { return dataset.Sheets[i].SheetName < dataset.Sheets[j].SheetName })
	sort.Slice(dataset.RawOrders, func(i, j int) bool {
		if dataset.RawOrders[i].SheetName != dataset.RawOrders[j].SheetName {
			return dataset.RawOrders[i].SheetName < dataset.RawOrders[j].SheetName
		}
		return dataset.RawOrders[i].SourceRowNumber < dataset.RawOrders[j].SourceRowNumber
	})
	sort.Slice(dataset.Orders, func(i, j int) bool { return dataset.Orders[i].SourceOrderKey < dataset.Orders[j].SourceOrderKey })
	sort.Slice(dataset.OrderItems, func(i, j int) bool { return dataset.OrderItems[i].SourceItemKey < dataset.OrderItems[j].SourceItemKey })
	sort.Slice(dataset.Issues, func(i, j int) bool { return dataset.Issues[i].IssueKey < dataset.Issues[j].IssueKey })
}
