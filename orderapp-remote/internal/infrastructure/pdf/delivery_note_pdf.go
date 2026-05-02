package pdf

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	salesdomain "orderapp/internal/domain/sales"

	"github.com/jung-kurt/gofpdf"
)

type DeliveryNoteRenderer struct {
	FontPath     string
	AssetBaseDir string
}

func (r DeliveryNoteRenderer) Render(snapshot salesdomain.DeliveryNoteSnapshot) ([]byte, error) {
	if err := snapshot.Validate(); err != nil {
		return nil, err
	}
	fontPath, err := r.resolveFontPath()
	if err != nil {
		return nil, err
	}

	pdf := gofpdf.NewCustom(&gofpdf.InitType{
		OrientationStr: "P",
		UnitStr:        "mm",
		SizeStr:        "A4",
		FontDirStr:     filepath.Dir(fontPath),
	})
	pdf.SetMargins(16, 14, 16)
	pdf.SetAutoPageBreak(true, 18)
	pdf.AddUTF8Font("noto", "", filepath.Base(fontPath))
	pdf.AddPage()

	renderDeliveryNoteHeader(pdf, snapshot)
	renderDeliveryNoteItemsTable(pdf, snapshot)
	renderDeliveryNoteNote(pdf, snapshot)

	if pdf.Error() != nil {
		return nil, pdf.Error()
	}
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func renderDeliveryNoteHeader(pdf *gofpdf.Fpdf, snapshot salesdomain.DeliveryNoteSnapshot) {
	pdf.SetFont("noto", "", 14)
	pdf.CellFormat(100, 10, snapshot.CompanyName, "", 0, "L", false, 0, "")
	pdf.SetFont("noto", "", 12)
	pdf.CellFormat(0, 10, "出库单 DELIVERY NOTE", "", 1, "R", false, 0, "")
	y := pdf.GetY() + 2
	left, _, right, _ := pdf.GetMargins()
	pageW, _ := pdf.GetPageSize()
	pdf.Line(left, y, pageW-right, y)
	pdf.SetY(y + 5)
	pdf.SetFont("noto", "", 10)
	colW := (pageW - left - right) / 3
	widths := []float64{colW, colW, colW}
	writeDeliveryNoteMetaRow(pdf, widths, []string{
		"出库单号：" + snapshot.DeliveryNoteNo,
		"订单号：" + snapshot.OrderNo,
		"出库日期：" + snapshot.PostingDate,
	}, 6)
	writeDeliveryNoteMetaRow(pdf, widths, []string{
		"客户：" + snapshot.CustomerName,
		"客户公司：" + firstNonEmpty(snapshot.CustomerCompanyName, snapshot.CustomerName),
		"联系电话：" + firstNonEmpty(snapshot.CustomerCompanyPhone, snapshot.ReceiverPhone),
	}, 6)
	writeDeliveryNoteMetaRow(pdf, widths, []string{
		"收货人：" + snapshot.ReceiverName,
		"快递单号：" + snapshot.TrackingNo,
		"出库仓：" + firstNonEmpty(snapshot.SourceWarehouseName, snapshot.SourceWarehouse),
	}, 6)
	writeDeliveryNoteMetaRow(pdf, []float64{pageW - left - right}, []string{
		"收货地址：" + firstNonEmpty(snapshot.ReceiverAddress, snapshot.CustomerCompanyAddress),
	}, 6)
	pdf.Ln(3)
}

func writeDeliveryNoteMetaRow(pdf *gofpdf.Fpdf, widths []float64, texts []string, lineHeight float64) {
	if lineHeight <= 0 {
		lineHeight = 6
	}
	startX, startY := pdf.GetXY()
	rowH := lineHeight
	for i, text := range texts {
		if i >= len(widths) {
			break
		}
		lines := pdf.SplitLines([]byte(text), widths[i])
		if len(lines) == 0 {
			lines = [][]byte{[]byte("")}
		}
		if h := float64(len(lines)) * lineHeight; h > rowH {
			rowH = h
		}
	}
	x := startX
	for i, text := range texts {
		if i >= len(widths) {
			break
		}
		pdf.SetXY(x, startY)
		pdf.MultiCell(widths[i], lineHeight, text, "", "L", false)
		x += widths[i]
	}
	pdf.SetXY(startX, startY+rowH)
}

func renderDeliveryNoteItemsTable(pdf *gofpdf.Fpdf, snapshot salesdomain.DeliveryNoteSnapshot) {
	left, _, right, _ := pdf.GetMargins()
	pageW, _ := pdf.GetPageSize()
	usableW := pageW - left - right
	colWidths := []float64{usableW * 0.38, usableW * 0.18, usableW * 0.20, usableW * 0.24}
	headers := []string{"商品", "规格", "出库数量", "出库仓"}
	pdf.SetFont("noto", "", 10)
	for i, h := range headers {
		pdf.CellFormat(colWidths[i], 8, h, "B", 0, "L", false, 0, "")
	}
	pdf.Ln(-1)
	for _, item := range snapshot.Items {
		pdf.CellFormat(colWidths[0], 8, item.Name, "B", 0, "L", false, 0, "")
		pdf.CellFormat(colWidths[1], 8, item.Spec, "B", 0, "L", false, 0, "")
		pdf.CellFormat(colWidths[2], 8, strings.TrimSpace(item.Qty+item.Unit), "B", 0, "L", false, 0, "")
		pdf.CellFormat(colWidths[3], 8, firstNonEmpty(item.WarehouseName, item.Warehouse), "B", 0, "L", false, 0, "")
		pdf.Ln(-1)
	}
	pdf.Ln(4)
}

func renderDeliveryNoteNote(pdf *gofpdf.Fpdf, snapshot salesdomain.DeliveryNoteSnapshot) {
	pdf.SetFont("noto", "", 10)
	if method := strings.TrimSpace(snapshot.DeliveryMethod); method != "" {
		pdf.MultiCell(0, 6, "发货方式："+method, "", "L", false)
	}
	if note := strings.TrimSpace(snapshot.Note); note != "" {
		pdf.MultiCell(0, 6, "备注："+note, "", "L", false)
	}
	pdf.Ln(8)
	pdf.CellFormat(0, 8, "仓库经办：________________        客户/承运签收：________________", "", 1, "L", false, 0, "")
}

func (r DeliveryNoteRenderer) resolveFontPath() (string, error) {
	if r.FontPath != "" {
		if _, err := os.Stat(r.FontPath); err == nil {
			return r.FontPath, nil
		}
		return "", fmt.Errorf("font not found: %s", r.FontPath)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(cwd, "assets", "fonts", "NotoSansSC-VF.ttf")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			break
		}
		cwd = parent
	}
	return "", fmt.Errorf("delivery note PDF font not found")
}
