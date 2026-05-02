package excel

import (
	"bytes"
	"fmt"
	domain "orderapp/internal/domain/finance"

	"github.com/xuri/excelize/v2"
)

func RenderFinanceMonthlyReport(report domain.MonthlyReport) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	sheet := "Summary"
	f.SetSheetName("Sheet1", sheet)
	rows := [][]any{
		{"Metric", "Amount"},
		{"Month", report.Month},
		{"Revenue excl. tax", float64(report.TaxExclusiveRevenue)},
		{"Main business cost", float64(report.MainBusinessCost)},
		{"Gross profit", float64(report.GrossProfit)},
		{"Period expenses", float64(report.PeriodExpenses)},
		{"VAT payable estimate", float64(report.Tax.VATPayable)},
		{"Surtax estimate", float64(report.Tax.Surtax)},
		{"CIT estimate", float64(report.Tax.CITPayable)},
		{"Operating net profit", float64(report.OperatingNetProfit)},
		{"Adjusted net profit", float64(report.AdjustedNetProfit)},
		{"Note", "Tax values are management estimates and are not official filing results."},
	}
	for i, row := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		if err := f.SetSheetRow(sheet, cell, &row); err != nil {
			return nil, err
		}
	}
	if err := f.SetColWidth(sheet, "A", "A", 28); err != nil {
		return nil, err
	}
	if err := f.SetColWidth(sheet, "B", "B", 22); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, fmt.Errorf("write finance report: %w", err)
	}
	return buf.Bytes(), nil
}
