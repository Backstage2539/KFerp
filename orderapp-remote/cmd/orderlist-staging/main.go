package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"orderapp/internal/migration/orderliststaging"
)

func main() {
	var (
		source                   = flag.String("source", "", "source orderlist.xlsx path")
		sourceLabel              = flag.String("source-label", "/data/orderlist.xlsx", "audited source path stored in the manifest")
		output                   = flag.String("output", "", "protected output directory")
		startPeriod              = flag.String("start", "2025-01", "first included month")
		endPeriod                = flag.String("end", "2026-06", "last included month")
		customerSnapshot         = flag.String("erp-customers", "", "development ERP customer TSV snapshot")
		targetCustomerSnapshot   = flag.String("erp-target-customers", "", "target ERP full customer TSV snapshot")
		customerTypeSnapshot     = flag.String("erp-customer-types", "", "target ERP customer type option TSV snapshot")
		customerOptionSnapshot   = flag.String("erp-customer-options", "", "target ERP source/order type/employee TSV snapshot")
		capabilityOptionSnapshot = flag.String("erp-capability-options", "", "target ERP customer capability template TSV snapshot")
		productSnapshot          = flag.String("erp-products", "", "development ERP product TSV snapshot")
		previousMapping          = flag.String("previous-mapping", "", "previous source-key-mapping.json")
	)
	flag.Parse()
	if strings.TrimSpace(*source) == "" || strings.TrimSpace(*output) == "" {
		fmt.Fprintln(os.Stderr, "--source and --output are required")
		os.Exit(2)
	}

	customers, err := readCustomerSnapshot(*customerSnapshot)
	check(err)
	targetCustomers, err := readFullCustomerSnapshot(*targetCustomerSnapshot)
	check(err)
	customerTypes, err := readOptionSnapshot(*customerTypeSnapshot, "customer_type")
	check(err)
	referenceOptions, err := readOptionSnapshot(*customerOptionSnapshot, "")
	check(err)
	capabilityTemplates, err := readOptionSnapshot(*capabilityOptionSnapshot, "capability_template")
	check(err)
	products, err := readProductSnapshot(*productSnapshot)
	check(err)
	mappings, err := readMappings(*previousMapping)
	check(err)
	dataset, err := orderliststaging.PrepareWorkbook(*source, orderliststaging.PrepareOptions{
		StartPeriod:        *startPeriod,
		EndPeriod:          *endPeriod,
		PreviousMappings:   mappings,
		ERPCustomers:       customers,
		TargetERPCustomers: targetCustomers,
		CustomerImportOptions: orderliststaging.CustomerImportOptions{
			CustomerTypes:       customerTypes["customer_type"],
			Sources:             referenceOptions["source"],
			OrderTypes:          referenceOptions["order_type"],
			Employees:           referenceOptions["employee"],
			CapabilityTemplates: capabilityTemplates["capability_template"],
		},
		ERPProducts: products,
	})
	check(err)
	if strings.TrimSpace(*sourceLabel) != "" {
		dataset.Run.SourcePath = strings.TrimSpace(*sourceLabel)
	}
	check(orderliststaging.WriteExports(dataset, *output))

	summary := map[string]any{
		"run_id":               dataset.Run.RunID,
		"source_sha256":        dataset.Run.SourceSHA256,
		"workbook_sheets":      len(dataset.Sheets),
		"included_sheets":      dataset.Run.IncludedSheetCount,
		"raw_orders":           len(dataset.RawOrders),
		"customers":            len(dataset.Customers),
		"customer_import_rows": len(dataset.CustomerImportRows),
		"products":             len(dataset.Products),
		"skus":                 len(dataset.SKUs),
		"orders":               len(dataset.Orders),
		"order_items":          len(dataset.OrderItems),
		"issues":               len(dataset.Issues),
		"output":               *output,
	}
	encoded, _ := json.MarshalIndent(summary, "", "  ")
	fmt.Println(string(encoded))
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func readMappings(path string) (map[string]orderliststaging.SourceKeyAssignment, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var mappings map[string]orderliststaging.SourceKeyAssignment
	if err := json.Unmarshal(b, &mappings); err != nil {
		return nil, err
	}
	return mappings, nil
}

func readCustomerSnapshot(path string) ([]orderliststaging.ERPReferenceCustomer, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	rows, err := readTSV(path)
	if err != nil {
		return nil, err
	}
	result := make([]orderliststaging.ERPReferenceCustomer, 0, len(rows))
	for _, row := range rows {
		if len(row) < 4 || strings.EqualFold(strings.TrimSpace(row[0]), "id") {
			continue
		}
		id, err := strconv.ParseInt(strings.TrimSpace(row[0]), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("customer snapshot id %q: %w", row[0], err)
		}
		result = append(result, orderliststaging.ERPReferenceCustomer{
			ID: id, Name: row[1], Phone: row[2], Active: parseBool(row[3]),
		})
	}
	return result, nil
}

func readProductSnapshot(path string) ([]orderliststaging.ERPReferenceProduct, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	rows, err := readTSV(path)
	if err != nil {
		return nil, err
	}
	result := make([]orderliststaging.ERPReferenceProduct, 0, len(rows))
	for _, row := range rows {
		if len(row) < 4 || strings.EqualFold(strings.TrimSpace(row[0]), "id") {
			continue
		}
		id, err := strconv.ParseInt(strings.TrimSpace(row[0]), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("product snapshot id %q: %w", row[0], err)
		}
		result = append(result, orderliststaging.ERPReferenceProduct{
			ID: id, Name: row[1], ProductKind: row[2], Active: parseBool(row[3]),
		})
	}
	return result, nil
}

func readFullCustomerSnapshot(path string) ([]orderliststaging.ERPReferenceCustomer, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	rows, err := readTSV(path)
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return nil, nil
	}
	header := headerLookup(rows[0])
	result := make([]orderliststaging.ERPReferenceCustomer, 0, len(rows)-1)
	for _, row := range rows[1:] {
		id, err := strconv.ParseInt(snapshotValue(row, header, "id"), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("target customer snapshot id %q: %w", snapshotValue(row, header, "id"), err)
		}
		result = append(result, orderliststaging.ERPReferenceCustomer{
			ID:                    id,
			Name:                  snapshotValue(row, header, "name"),
			RawName:               snapshotValue(row, header, "raw_name"),
			CustomerType:          snapshotValue(row, header, "customer_type"),
			CompanyName:           snapshotValue(row, header, "company_name"),
			CompanyAddress:        snapshotValue(row, header, "company_address"),
			CompanyPhone:          snapshotValue(row, header, "company_phone"),
			Contact:               snapshotValue(row, header, "contact"),
			Phone:                 snapshotValue(row, header, "phone"),
			Address:               snapshotValue(row, header, "address"),
			DefaultSourceID:       parseInt64(snapshotValue(row, header, "default_source_id")),
			DefaultOrderTypeID:    parseInt64(snapshotValue(row, header, "default_order_type_id")),
			ResponsibleEmployeeID: parseInt64(snapshotValue(row, header, "responsible_employee_id")),
			PortalEnabled:         parseBool(snapshotValue(row, header, "portal_enabled")),
			CapabilityTemplateKey: snapshotValue(row, header, "capability_template_key"),
			Active:                parseBool(snapshotValue(row, header, "active")),
			UpdatedAt:             snapshotValue(row, header, "updated_at"),
		})
	}
	return result, nil
}

func readOptionSnapshot(path, defaultKind string) (map[string][]orderliststaging.ERPReferenceOption, error) {
	result := map[string][]orderliststaging.ERPReferenceOption{}
	if strings.TrimSpace(path) == "" {
		return result, nil
	}
	rows, err := readTSV(path)
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return result, nil
	}
	header := headerLookup(rows[0])
	for _, row := range rows[1:] {
		kind := strings.TrimSpace(defaultKind)
		if kind == "" {
			kind = strings.TrimSpace(snapshotValue(row, header, "kind"))
		}
		if kind == "" {
			continue
		}
		value := snapshotValue(row, header, "value")
		if value == "" {
			value = snapshotValue(row, header, "template_key")
		}
		label := snapshotValue(row, header, "label")
		if value == "" || label == "" {
			continue
		}
		result[kind] = append(result[kind], orderliststaging.ERPReferenceOption{
			Value: value, Label: label, Active: parseBool(snapshotValue(row, header, "active")),
		})
	}
	return result, nil
}

func headerLookup(row []string) map[string]int {
	result := map[string]int{}
	for index, value := range row {
		result[strings.ToLower(strings.TrimSpace(value))] = index
	}
	return result
}

func snapshotValue(row []string, header map[string]int, name string) string {
	index, exists := header[strings.ToLower(strings.TrimSpace(name))]
	if !exists || index < 0 || index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

func parseInt64(raw string) int64 {
	value, _ := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	return value
}

func readTSV(path string) ([][]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := csv.NewReader(bufio.NewReader(file))
	reader.Comma = '\t'
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true
	return reader.ReadAll()
}

func parseBool(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "t", "true", "yes", "y":
		return true
	default:
		return false
	}
}
