package excel

import (
	"bytes"
	"fmt"

	appfinance "orderapp/internal/application/finance"

	"github.com/xuri/excelize/v2"
)

func RenderFinanceAccountantHandoff(handoff appfinance.AccountantHandoff) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	if err := writeHandoffSummary(f, handoff); err != nil {
		return nil, err
	}
	if err := writeChecklistSheet(f, handoff); err != nil {
		return nil, err
	}
	if err := writeDrilldownSheet(f, handoff); err != nil {
		return nil, err
	}
	if err := writeTaxLedgerSheet(f, handoff); err != nil {
		return nil, err
	}
	if err := writeVoucherDraftSheet(f, handoff); err != nil {
		return nil, err
	}
	f.SetActiveSheet(0)

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, fmt.Errorf("write finance accountant handoff: %w", err)
	}
	return buf.Bytes(), nil
}

func writeHandoffSummary(f *excelize.File, handoff appfinance.AccountantHandoff) error {
	sheet := "Summary"
	f.SetSheetName("Sheet1", sheet)
	rows := [][]any{
		{"Metric", "Value"},
		{"Month", handoff.Month},
		{"Revenue excl. tax", float64(handoff.Report.TaxExclusiveRevenue)},
		{"Main business cost", float64(handoff.Report.MainBusinessCost)},
		{"Gross profit", float64(handoff.Report.GrossProfit)},
		{"Period expenses", float64(handoff.Report.PeriodExpenses)},
		{"VAT payable estimate", float64(handoff.Report.Tax.VATPayable)},
		{"Surtax estimate", float64(handoff.Report.Tax.Surtax)},
		{"CIT estimate", float64(handoff.Report.Tax.CITPayable)},
		{"Adjusted net profit", float64(handoff.Report.AdjustedNetProfit)},
		{"Note", "Management estimate only; accountant should reconcile invoices and official filings."},
	}
	return setSheetRows(f, sheet, rows, []string{"A", "B"}, []float64{28, 26})
}

func writeChecklistSheet(f *excelize.File, handoff appfinance.AccountantHandoff) error {
	sheet := "Checklist"
	f.NewSheet(sheet)
	rows := [][]any{{"Code", "Title", "Status", "Severity", "Message", "Count"}}
	for _, item := range handoff.Checklist {
		rows = append(rows, []any{item.Code, item.Title, item.Status, item.Severity, item.Message, item.Count})
	}
	return setSheetRows(f, sheet, rows, []string{"A", "B", "C", "D", "E", "F"}, []float64{24, 24, 14, 14, 72, 12})
}

func writeDrilldownSheet(f *excelize.File, handoff appfinance.AccountantHandoff) error {
	sheet := "Drilldown"
	f.NewSheet(sheet)
	rows := [][]any{{"Section", "Source type", "Source id", "Date", "Name", "Category", "Counterparty", "Payment method", "Amount", "Link"}}
	for _, section := range handoff.Drilldown.Sections {
		for _, row := range section.Rows {
			rows = append(rows, []any{section.Section, row.SourceType, row.SourceID, row.Date, row.Name, row.Category, row.Counterparty, row.PaymentMethod, float64(row.Amount), row.Link})
		}
	}
	return setSheetRows(f, sheet, rows, []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J"}, []float64{18, 18, 12, 14, 28, 20, 24, 18, 16, 28})
}

func writeTaxLedgerSheet(f *excelize.File, handoff appfinance.AccountantHandoff) error {
	sheet := "TaxLedger"
	f.NewSheet(sheet)
	rows := [][]any{{"ID", "Month", "Kind", "Invoice no", "Counterparty", "Total amount", "Tax amount", "Status", "Note", "Actor", "Created at"}}
	for _, row := range handoff.TaxLedger {
		rows = append(rows, []any{row.ID, row.Month, row.Kind, row.InvoiceNo, row.Counterparty, float64(row.TotalAmount), float64(row.TaxAmount), row.Status, row.Note, row.Actor, row.CreatedAt})
	}
	return setSheetRows(f, sheet, rows, []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K"}, []float64{10, 14, 18, 22, 28, 16, 16, 14, 32, 16, 18})
}

func writeVoucherDraftSheet(f *excelize.File, handoff appfinance.AccountantHandoff) error {
	sheet := "VoucherDrafts"
	f.NewSheet(sheet)
	rows := [][]any{{"Summary", "Debit", "Credit", "Amount", "Source"}}
	for _, row := range handoff.VoucherDrafts {
		rows = append(rows, []any{row.Summary, row.Debit, row.Credit, float64(row.Amount), row.Source})
	}
	return setSheetRows(f, sheet, rows, []string{"A", "B", "C", "D", "E"}, []float64{30, 28, 28, 16, 28})
}

func setSheetRows(f *excelize.File, sheet string, rows [][]any, cols []string, widths []float64) error {
	for i, row := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		if err := f.SetSheetRow(sheet, cell, &row); err != nil {
			return err
		}
	}
	for i, col := range cols {
		width := 18.0
		if i < len(widths) {
			width = widths[i]
		}
		if err := f.SetColWidth(sheet, col, col, width); err != nil {
			return err
		}
	}
	return nil
}
