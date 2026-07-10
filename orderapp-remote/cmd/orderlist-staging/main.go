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
		source           = flag.String("source", "", "source orderlist.xlsx path")
		sourceLabel      = flag.String("source-label", "/data/orderlist.xlsx", "audited source path stored in the manifest")
		output           = flag.String("output", "", "protected output directory")
		startPeriod      = flag.String("start", "2025-01", "first included month")
		endPeriod        = flag.String("end", "2026-06", "last included month")
		customerSnapshot = flag.String("erp-customers", "", "development ERP customer TSV snapshot")
		productSnapshot  = flag.String("erp-products", "", "development ERP product TSV snapshot")
		previousMapping  = flag.String("previous-mapping", "", "previous source-key-mapping.json")
	)
	flag.Parse()
	if strings.TrimSpace(*source) == "" || strings.TrimSpace(*output) == "" {
		fmt.Fprintln(os.Stderr, "--source and --output are required")
		os.Exit(2)
	}

	customers, err := readCustomerSnapshot(*customerSnapshot)
	check(err)
	products, err := readProductSnapshot(*productSnapshot)
	check(err)
	mappings, err := readMappings(*previousMapping)
	check(err)
	dataset, err := orderliststaging.PrepareWorkbook(*source, orderliststaging.PrepareOptions{
		StartPeriod:      *startPeriod,
		EndPeriod:        *endPeriod,
		PreviousMappings: mappings,
		ERPCustomers:     customers,
		ERPProducts:      products,
	})
	check(err)
	if strings.TrimSpace(*sourceLabel) != "" {
		dataset.Run.SourcePath = strings.TrimSpace(*sourceLabel)
	}
	check(orderliststaging.WriteExports(dataset, *output))

	summary := map[string]any{
		"run_id":          dataset.Run.RunID,
		"source_sha256":   dataset.Run.SourceSHA256,
		"workbook_sheets": len(dataset.Sheets),
		"included_sheets": dataset.Run.IncludedSheetCount,
		"raw_orders":      len(dataset.RawOrders),
		"customers":       len(dataset.Customers),
		"products":        len(dataset.Products),
		"skus":            len(dataset.SKUs),
		"orders":          len(dataset.Orders),
		"order_items":     len(dataset.OrderItems),
		"issues":          len(dataset.Issues),
		"output":          *output,
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
