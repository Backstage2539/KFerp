package pdf

import (
	"bytes"
	"fmt"
	domain "orderapp/internal/domain/finance"

	"github.com/jung-kurt/gofpdf"
)

func RenderFinanceMonthlyReport(report domain.MonthlyReport) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(16, 14, 16)
	pdf.SetAutoPageBreak(true, 16)
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(0, 10, "KFerp Monthly Finance Report "+report.Month, "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	rows := [][]string{
		{"Revenue excl. tax", money(report.TaxExclusiveRevenue)},
		{"Main business cost", money(report.MainBusinessCost)},
		{"Gross profit", money(report.GrossProfit)},
		{"Period expenses", money(report.PeriodExpenses)},
		{"VAT payable estimate", money(report.Tax.VATPayable)},
		{"Surtax estimate", money(report.Tax.Surtax)},
		{"CIT estimate", money(report.Tax.CITPayable)},
		{"Operating net profit", money(report.OperatingNetProfit)},
		{"Adjusted net profit", money(report.AdjustedNetProfit)},
	}
	for _, row := range rows {
		pdf.CellFormat(78, 8, row[0], "B", 0, "L", false, 0, "")
		pdf.CellFormat(0, 8, row[1], "B", 1, "R", false, 0, "")
	}
	pdf.Ln(4)
	pdf.MultiCell(0, 6, "Tax values are management estimates and are not official filing results.", "", "L", false)
	if pdf.Error() != nil {
		return nil, pdf.Error()
	}
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func money(v domain.Money) string {
	return fmt.Sprintf("%.2f", v)
}
