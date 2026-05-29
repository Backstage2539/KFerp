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
	return r.render(snapshot, false)
}

func (r DeliveryNoteRenderer) RenderPreview(snapshot salesdomain.DeliveryNoteSnapshot) ([]byte, error) {
	return r.render(snapshot, true)
}

func (r DeliveryNoteRenderer) RenderCombinedDeliveryNote(snapshot salesdomain.CombinedDeliveryNoteSnapshot) ([]byte, error) {
	return r.renderCombinedDeliveryNote(snapshot, false)
}

func (r DeliveryNoteRenderer) RenderCombinedDeliveryNotePreview(snapshot salesdomain.CombinedDeliveryNoteSnapshot) ([]byte, error) {
	return r.renderCombinedDeliveryNote(snapshot, true)
}

func (r DeliveryNoteRenderer) render(snapshot salesdomain.DeliveryNoteSnapshot, preview bool) ([]byte, error) {
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

	r.renderDeliveryNoteHeader(pdf, snapshot)
	renderDeliveryNoteItemsTable(pdf, snapshot)
	renderDeliveryNoteNote(pdf, snapshot)
	if preview {
		renderDocumentPreviewLabel(pdf)
	}

	if pdf.Error() != nil {
		return nil, pdf.Error()
	}
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (r DeliveryNoteRenderer) renderCombinedDeliveryNote(snapshot salesdomain.CombinedDeliveryNoteSnapshot, preview bool) ([]byte, error) {
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
	pdf.AddUTF8Font("noto", "B", filepath.Base(fontPath))
	pdf.AddPage()

	r.renderCombinedDeliveryNoteHeader(pdf, snapshot)
	renderCombinedDeliveryNoteGroups(pdf, snapshot)
	if preview {
		renderDocumentPreviewLabel(pdf)
	}

	if pdf.Error() != nil {
		return nil, pdf.Error()
	}
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (r DeliveryNoteRenderer) renderDeliveryNoteHeader(pdf *gofpdf.Fpdf, snapshot salesdomain.DeliveryNoteSnapshot) {
	pdf.SetFont("noto", "", 14)
	pdf.CellFormat(100, 10, snapshot.CompanyName, "", 0, "L", false, 0, "")
	pdf.SetFont("noto", "", 12)
	pdf.CellFormat(0, 10, "出库单 DELIVERY NOTE", "", 1, "R", false, 0, "")
	y := pdf.GetY() + 2
	left, _, right, _ := pdf.GetMargins()
	pageW, _ := pdf.GetPageSize()
	pdf.Line(left, y, pageW-right, y)
	if snapshot.Seal != nil {
		SalesOrderRenderer{AssetBaseDir: r.AssetBaseDir}.renderSealStamp(pdf, *snapshot.Seal)
	}
	pdf.SetY(y + 5)
	pdf.SetFont("noto", "", 10)
	colW := (pageW - left - right) / 3
	widths := []float64{colW, colW, colW}
	for _, row := range deliveryNoteHeaderMetaRows(snapshot) {
		writeDeliveryNoteMetaRow(pdf, widths, row, 6)
	}
	writeDeliveryNoteMetaRow(pdf, []float64{pageW - left - right}, []string{
		"收货地址：" + firstNonEmpty(snapshot.ReceiverAddress, snapshot.CustomerCompanyAddress),
	}, 6)
	pdf.Ln(3)
}

func (r DeliveryNoteRenderer) renderCombinedDeliveryNoteHeader(pdf *gofpdf.Fpdf, snapshot salesdomain.CombinedDeliveryNoteSnapshot) {
	pdf.SetFont("noto", "", 14)
	pdf.CellFormat(100, 10, snapshot.CompanyName, "", 0, "L", false, 0, "")
	pdf.SetFont("noto", "", 12)
	pdf.CellFormat(0, 10, "组合出库单 COMBINED DELIVERY NOTE", "", 1, "R", false, 0, "")
	y := pdf.GetY() + 2
	left, _, right, _ := pdf.GetMargins()
	pageW, _ := pdf.GetPageSize()
	pdf.Line(left, y, pageW-right, y)
	if snapshot.Seal != nil {
		SalesOrderRenderer{AssetBaseDir: r.AssetBaseDir}.renderSealStamp(pdf, *snapshot.Seal)
	}
	pdf.SetY(y + 5)
	pdf.SetFont("noto", "", 10)
	colW := (pageW - left - right) / 3
	rows := [][]string{
		{"组合出库单号：" + snapshot.DeliveryNoteNo, fmt.Sprintf("订单数：%d", len(snapshot.OrderIDs)), "客户：" + snapshot.CustomerName},
		{"客户公司：" + firstNonEmpty(snapshot.CustomerCompanyName, snapshot.CustomerName), "联系电话：" + snapshot.CustomerCompanyPhone, ""},
	}
	for _, row := range rows {
		writeDeliveryNoteMetaRow(pdf, []float64{colW, colW, colW}, row, 6)
	}
	writeDeliveryNoteMetaRow(pdf, []float64{pageW - left - right}, []string{"关联订单：" + strings.Join(snapshot.OrderNos, "、")}, 6)
	if addr := strings.TrimSpace(snapshot.CustomerCompanyAddress); addr != "" {
		writeDeliveryNoteMetaRow(pdf, []float64{pageW - left - right}, []string{"客户地址：" + addr}, 6)
	}
	pdf.Ln(3)
}

func deliveryNoteHeaderMetaRows(snapshot salesdomain.DeliveryNoteSnapshot) [][]string {
	return [][]string{
		{
			"出库单号：" + snapshot.DeliveryNoteNo,
			"单据日期：" + firstNonEmpty(snapshot.DocumentDate, snapshot.OrderDate),
			"出库日期：" + snapshot.PostingDate,
		},
		{
			"订单号：" + snapshot.OrderNo,
			"订单日期：" + snapshot.OrderDate,
			"客户：" + snapshot.CustomerName,
		},
		{
			"客户公司：" + firstNonEmpty(snapshot.CustomerCompanyName, snapshot.CustomerName),
			"联系电话：" + firstNonEmpty(snapshot.CustomerCompanyPhone, snapshot.ReceiverPhone),
			"出库仓：" + firstNonEmpty(snapshot.SourceWarehouseName, snapshot.SourceWarehouse),
		},
		{
			"收货人：" + snapshot.ReceiverName,
			"快递单号：" + snapshot.TrackingNo,
			"",
		},
	}
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
	colWidths := []float64{usableW * 0.28, usableW * 0.15, usableW * 0.17, usableW * 0.20, usableW * 0.20}
	headers := []string{"商品", "规格", "出库数量", "出库仓", "备注"}
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
		pdf.CellFormat(colWidths[4], 8, item.Note, "B", 0, "L", false, 0, "")
		pdf.Ln(-1)
	}
	pdf.Ln(4)
}

func renderCombinedDeliveryNoteGroups(pdf *gofpdf.Fpdf, snapshot salesdomain.CombinedDeliveryNoteSnapshot) {
	left, _, right, _ := pdf.GetMargins()
	pageW, _ := pdf.GetPageSize()
	usableW := pageW - left - right
	colWidths := []float64{usableW * 0.28, usableW * 0.15, usableW * 0.17, usableW * 0.20, usableW * 0.20}
	headers := []string{"商品", "规格", "出库数量", "出库仓", "备注"}
	for _, group := range snapshot.Groups {
		if pdf.GetY() > 246 {
			pdf.AddPage()
		}
		pdf.SetFont("noto", "B", 10)
		pdf.SetFillColor(241, 248, 244)
		pdf.CellFormat(usableW, 8, fmt.Sprintf("订单 %s    单据日期：%s    订单日期：%s    出库日期：%s", group.OrderNo, firstNonEmpty(group.DocumentDate, group.OrderDate), group.OrderDate, group.PostingDate), "1", 1, "L", true, 0, "")
		pdf.SetFont("noto", "", 9.5)
		writeDeliveryNoteMetaRow(pdf, []float64{usableW * 0.25, usableW * 0.25, usableW * 0.25, usableW * 0.25}, []string{
			"收货人：" + group.ReceiverName,
			"电话：" + group.ReceiverPhone,
			"发货方式：" + group.DeliveryMethod,
			"快递单号：" + group.TrackingNo,
		}, 5.5)
		writeDeliveryNoteMetaRow(pdf, []float64{usableW}, []string{"收货地址：" + group.ReceiverAddress}, 5.5)
		pdf.SetFont("noto", "", 10)
		for i, h := range headers {
			pdf.CellFormat(colWidths[i], 8, h, "B", 0, "L", false, 0, "")
		}
		pdf.Ln(-1)
		for _, item := range group.Items {
			writeCombinedDeliveryNoteItemRow(pdf, item, colWidths, group, 6)
		}
		if note := strings.TrimSpace(group.Note); note != "" {
			pdf.SetFont("noto", "", 9)
			pdf.MultiCell(usableW, 5.5, "备注："+note, "B", "L", false)
		}
		pdf.Ln(2)
	}
	pdf.SetFont("noto", "", 10)
	pdf.Ln(4)
	pdf.CellFormat(0, 8, "仓库经办：________________        客户/承运签收：________________", "", 1, "L", false, 0, "")
}

func writeCombinedDeliveryNoteItemRow(pdf *gofpdf.Fpdf, item salesdomain.DeliveryNoteSnapshotItem, colWidths []float64, group salesdomain.CombinedDeliveryNoteGroup, lineHeight float64) {
	pdf.CellFormat(colWidths[0], lineHeight+2, item.Name, "B", 0, "L", false, 0, "")
	pdf.CellFormat(colWidths[1], lineHeight+2, item.Spec, "B", 0, "L", false, 0, "")
	pdf.CellFormat(colWidths[2], lineHeight+2, strings.TrimSpace(item.Qty+item.Unit), "B", 0, "L", false, 0, "")
	pdf.CellFormat(colWidths[3], lineHeight+2, firstNonEmpty(item.WarehouseName, item.Warehouse, group.SourceWarehouseName, group.SourceWarehouse), "B", 0, "L", false, 0, "")
	pdf.CellFormat(colWidths[4], lineHeight+2, item.Note, "B", 0, "L", false, 0, "")
	pdf.Ln(-1)
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
