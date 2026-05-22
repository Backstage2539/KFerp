package pdf

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	salesdomain "orderapp/internal/domain/sales"

	"github.com/jung-kurt/gofpdf"
)

type SalesOrderRenderer struct {
	FontPath     string
	AssetBaseDir string
}

type salesOrderSealBox struct {
	XMM      float64
	YMM      float64
	WidthMM  float64
	HeightMM float64
}

type salesOrderPaymentCodeLayout struct {
	CellWidth  float64
	ImageSize  float64
	CellHeight float64
	Gap        float64
	Stacked    bool
}

const (
	salesOrderSealDefaultXMM     = 32
	salesOrderSealDefaultYMM     = 5
	salesOrderSealDefaultWidthMM = 36
	salesOrderSealLegacyXMM      = 32
	salesOrderSealLegacyYMM      = 22
	salesOrderSealLegacyWidthMM  = 42
	salesOrderSealHeightRatio    = 1
	salesOrderPaymentLineMM      = 6.2
	salesOrderPaymentBlockGapMM  = 3.5
)

var (
	defaultSalesOrderPaymentTextBox = salesdomain.SalesOrderLayoutBox{XMM: 16, YMM: 118, WidthMM: 104, HeightMM: 78}
	defaultSalesOrderPaymentCodeBox = salesdomain.SalesOrderLayoutBox{XMM: 126, YMM: 106, WidthMM: 72, HeightMM: 122}
)

type salesOrderPaymentTextSection struct {
	title string
	lines []string
}

func (r SalesOrderRenderer) Render(snapshot salesdomain.SalesOrderSnapshot) ([]byte, error) {
	return r.render(snapshot, false)
}

func (r SalesOrderRenderer) RenderPreview(snapshot salesdomain.SalesOrderSnapshot) ([]byte, error) {
	return r.render(snapshot, true)
}

func (r SalesOrderRenderer) render(snapshot salesdomain.SalesOrderSnapshot, preview bool) ([]byte, error) {
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

	r.renderSalesOrderHeader(pdf, snapshot)
	r.renderSalesOrderItemsTable(pdf, snapshot)
	renderSalesOrderTotals(pdf, snapshot)
	r.renderSalesOrderPaymentInfoSection(pdf, snapshot)
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

func renderDocumentPreviewLabel(pdf *gofpdf.Fpdf) {
	pageW, _ := pdf.GetPageSize()
	pdf.SetFont("noto", "", 16)
	pdf.SetTextColor(190, 30, 30)
	pdf.SetXY(pageW-68, 4)
	pdf.CellFormat(58, 8, "PREVIEW 预览版", "1", 0, "C", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
}

func (r SalesOrderRenderer) renderSalesOrderHeader(pdf *gofpdf.Fpdf, snapshot salesdomain.SalesOrderSnapshot) {
	pdf.SetFont("noto", "", 14)
	pdf.CellFormat(100, 10, snapshot.CompanyName, "", 0, "L", false, 0, "")
	pdf.SetFont("noto", "", 12)
	pdf.CellFormat(0, 10, "销售单 SALES ORDER", "", 1, "R", false, 0, "")
	y := pdf.GetY() + 2
	left, _, right, _ := pdf.GetMargins()
	pageW, _ := pdf.GetPageSize()
	pdf.Line(left, y, pageW-right, y)
	if snapshot.Seal != nil {
		r.renderSealStamp(pdf, *snapshot.Seal)
	}
	pdf.SetY(y + 5)
	pdf.SetFont("noto", "", 10)
	colW := (pageW - left - right) / 3
	widths := []float64{colW, colW, colW}
	writeSalesOrderMetaRow(pdf, widths, []string{
		"订单号：" + snapshot.OrderNo,
		"订单日期：" + snapshot.OrderDate,
		"客户：" + snapshot.CustomerName,
	}, 6)
	writeSalesOrderMetaRow(pdf, widths, []string{
		"客户公司：" + firstNonEmpty(snapshot.CustomerCompanyName, snapshot.CustomerName),
		"联系电话：" + snapshot.CustomerCompanyPhone,
		"公司地址：" + snapshot.CustomerCompanyAddress,
	}, 6)
	pdf.Ln(3)
}

func writeSalesOrderMetaRow(pdf *gofpdf.Fpdf, widths []float64, texts []string, lineHeight float64) {
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

func (r SalesOrderRenderer) renderSalesOrderItemsTable(pdf *gofpdf.Fpdf, snapshot salesdomain.SalesOrderSnapshot) {
	left, _, right, _ := pdf.GetMargins()
	pageW, _ := pdf.GetPageSize()
	usableW := pageW - left - right
	hasDiscount := salesOrderSnapshotHasDiscount(snapshot)
	colWidths := salesOrderItemColumnWidths(usableW, hasDiscount)
	headers := salesOrderItemHeaders(hasDiscount)
	pdf.SetFont("noto", "", 10)
	for i, h := range headers {
		pdf.CellFormat(colWidths[i], 8, h, "B", 0, "L", false, 0, "")
	}
	pdf.Ln(-1)
	for _, item := range snapshot.Items {
		writeSalesOrderItemRow(pdf, item, colWidths, hasDiscount, 6)
	}
	pdf.Ln(4)
}

func salesOrderItemColumnWidths(usableW float64, hasDiscount bool) []float64 {
	if hasDiscount {
		return []float64{usableW * 0.27, usableW * 0.11, usableW * 0.10, usableW * 0.11, usableW * 0.12, usableW * 0.11, usableW * 0.18}
	}
	return []float64{usableW * 0.33, usableW * 0.13, usableW * 0.12, usableW * 0.13, usableW * 0.12, usableW * 0.17}
}

func writeSalesOrderItemRow(pdf *gofpdf.Fpdf, item salesdomain.SalesOrderSnapshotItem, colWidths []float64, hasDiscount bool, lineHeight float64) {
	startX, startY := pdf.GetXY()
	rowH := salesOrderItemRowHeightForColumns(pdf, item, colWidths, hasDiscount, lineHeight)
	cells := salesOrderItemCells(item, hasDiscount)
	x := startX
	for i, text := range cells {
		if i >= len(colWidths) {
			break
		}
		pdf.SetXY(x, startY+1)
		for _, line := range salesOrderWrapCellText(pdf, text, colWidths[i]) {
			pdf.SetX(x)
			pdf.CellFormat(colWidths[i], lineHeight, line, "", 2, "L", false, 0, "")
		}
		x += colWidths[i]
	}
	pdf.Line(startX, startY+rowH, startX+sumFloat64(colWidths), startY+rowH)
	pdf.SetXY(startX, startY+rowH)
}

func salesOrderItemRowHeight(pdf *gofpdf.Fpdf, item salesdomain.SalesOrderSnapshotItem, colWidths []float64, lineHeight float64) float64 {
	return salesOrderItemRowHeightForColumns(pdf, item, colWidths, salesOrderItemHasDiscount(item), lineHeight)
}

func salesOrderItemRowHeightForColumns(pdf *gofpdf.Fpdf, item salesdomain.SalesOrderSnapshotItem, colWidths []float64, hasDiscount bool, lineHeight float64) float64 {
	if lineHeight <= 0 {
		lineHeight = 6
	}
	maxLines := 1
	for i, text := range salesOrderItemCells(item, hasDiscount) {
		if i >= len(colWidths) {
			break
		}
		if lines := len(salesOrderWrapCellText(pdf, text, colWidths[i])); lines > maxLines {
			maxLines = lines
		}
	}
	return float64(maxLines)*lineHeight + 2
}

func salesOrderItemHeaders(hasDiscount bool) []string {
	if hasDiscount {
		return []string{"商品", "规格", "数量", "单价", "优惠折扣", "总价", "备注"}
	}
	return []string{"商品", "规格", "数量", "单价", "总价", "备注"}
}

func salesOrderItemCells(item salesdomain.SalesOrderSnapshotItem, hasDiscount bool) []string {
	cells := []string{
		item.Name,
		salesOrderSpecPerUnit(item),
		strings.TrimSpace(item.Qty + item.Unit),
		item.UnitPrice,
	}
	if hasDiscount {
		cells = append(cells, salesOrderDiscountCell(item.DiscountAmount))
	}
	cells = append(cells, item.LineTotal)
	cells = append(cells, item.Note)
	return cells
}

func salesOrderSpecPerUnit(item salesdomain.SalesOrderSnapshotItem) string {
	spec := strings.TrimSpace(item.Spec)
	unit := strings.TrimSpace(item.Unit)
	if spec == "" || unit == "" || strings.Contains(spec, "/") {
		return spec
	}
	return spec + "/" + unit
}

func salesOrderDiscountCell(amount string) string {
	if !salesOrderMoneyPositive(amount) {
		return ""
	}
	return "￥-" + salesOrderTrimMoney(amount) + "元"
}

func salesOrderTrimMoney(amount string) string {
	value, err := strconv.ParseFloat(strings.TrimSpace(amount), 64)
	if err != nil {
		return strings.TrimSpace(amount)
	}
	formatted := salesdomain.FormatSalesOrderMoney(value)
	formatted = strings.TrimRight(formatted, "0")
	return strings.TrimSuffix(formatted, ".")
}

func salesOrderWrapCellText(pdf *gofpdf.Fpdf, text string, width float64) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return []string{""}
	}
	lines := pdf.SplitText(text, width)
	if len(lines) == 0 {
		return []string{text}
	}
	return lines
}

func sumFloat64(values []float64) float64 {
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total
}

type salesOrderFinancialRow struct {
	Label string
	Value string
	Bold  bool
	Cells []string
}

func renderSalesOrderTotals(pdf *gofpdf.Fpdf, snapshot salesdomain.SalesOrderSnapshot) {
	left, _, right, _ := pdf.GetMargins()
	pageW, _ := pdf.GetPageSize()
	usableW := pageW - left - right
	for _, row := range salesOrderFinancialRows(snapshot) {
		if len(row.Cells) > 0 {
			pdf.SetFont("noto", salesOrderFontStyle(row.Bold), 11)
			cellW := usableW / float64(len(row.Cells))
			for i, text := range row.Cells {
				align := "R"
				if i == 0 {
					align = "L"
				}
				pdf.CellFormat(cellW, 7, text, "", 0, align, false, 0, "")
			}
			pdf.Ln(7)
			continue
		}
		style := ""
		align := "R"
		if row.Bold {
			style = "B"
		}
		if row.Label == "订单备注" {
			align = "L"
		}
		pdf.SetFont("noto", style, 11)
		text := row.Label + "： " + row.Value
		if row.Label == "订单备注" {
			for _, line := range pdf.SplitText(text, usableW) {
				pdf.CellFormat(0, 7, line, "", 1, align, false, 0, "")
			}
			continue
		}
		pdf.CellFormat(0, 7, text, "", 1, align, false, 0, "")
	}
	pdf.Line(16, pdf.GetY()+1, 194, pdf.GetY()+1)
	pdf.SetFont("noto", "", 11)
	pdf.Ln(4)
}

func salesOrderFinancialRows(snapshot salesdomain.SalesOrderSnapshot) []salesOrderFinancialRow {
	rows := make([]salesOrderFinancialRow, 0, 2)
	if note := strings.TrimSpace(snapshot.SalesOrderNote); note != "" {
		rows = append(rows, salesOrderFinancialRow{Label: "订单备注", Value: note})
	}
	cells := []string{"商品合计： " + snapshot.TotalAmount}
	if salesOrderMoneyPositive(snapshot.Discount) {
		cells = append(cells, "优惠合计： "+snapshot.Discount)
	}
	cells = append(cells, "运费： "+snapshot.Shipping, "应收： "+snapshot.GrandTotal)
	rows = append(rows, salesOrderFinancialRow{Bold: true, Cells: cells})
	return rows
}

func salesOrderSnapshotHasDiscount(snapshot salesdomain.SalesOrderSnapshot) bool {
	if salesOrderMoneyPositive(snapshot.Discount) {
		return true
	}
	for _, item := range snapshot.Items {
		if salesOrderItemHasDiscount(item) {
			return true
		}
	}
	return false
}

func salesOrderItemHasDiscount(item salesdomain.SalesOrderSnapshotItem) bool {
	return salesOrderMoneyPositive(item.DiscountAmount)
}

func salesOrderMoneyPositive(amount string) bool {
	value, err := strconv.ParseFloat(strings.TrimSpace(amount), 64)
	return err == nil && value > 0.004
}

func salesOrderFontStyle(bold bool) string {
	if bold {
		return "B"
	}
	return ""
}

func salesOrderDiscountTypeLabel(value string) string {
	switch strings.TrimSpace(value) {
	case "amount":
		return "减免金额"
	case "unit_amount":
		return "单价优惠"
	case "percent":
		return "折扣"
	case "free":
		return "免费"
	case "order_amount":
		return "整单优惠"
	default:
		return "优惠"
	}
}

func (r SalesOrderRenderer) renderSalesOrderPaymentInfoSection(pdf *gofpdf.Fpdf, snapshot salesdomain.SalesOrderSnapshot) {
	if snapshot.PaymentText == "" && snapshot.Note == "" && len(snapshot.PaymentCodes) == 0 && len(renderSalesOrderAccountLines(snapshot)) == 0 {
		return
	}
	currentPage := pdf.PageNo()
	textBox, codeBox := salesOrderPaymentLayoutBoxes(snapshot)
	pdf.SetPage(1)
	pdf.SetAutoPageBreak(false, 0)
	renderSalesOrderTextBlocks(pdf, textBox, snapshot)
	if len(snapshot.PaymentCodes) > 0 {
		r.renderPaymentCodes(pdf, snapshot.PaymentCodes, codeBox)
	}
	pdf.SetAutoPageBreak(true, 18)
	if currentPage > 0 {
		pdf.SetPage(currentPage)
	}
}

func salesOrderPaymentLayoutBoxes(snapshot salesdomain.SalesOrderSnapshot) (salesdomain.SalesOrderLayoutBox, salesdomain.SalesOrderLayoutBox) {
	return normalizeSalesOrderLayoutBox(snapshot.PaymentTextBox, defaultSalesOrderPaymentTextBox), normalizeSalesOrderLayoutBox(snapshot.PaymentCodeBox, defaultSalesOrderPaymentCodeBox)
}

func normalizeSalesOrderLayoutBox(box, fallback salesdomain.SalesOrderLayoutBox) salesdomain.SalesOrderLayoutBox {
	if box.XMM <= 0 {
		box.XMM = fallback.XMM
	}
	if box.YMM <= 0 {
		box.YMM = fallback.YMM
	}
	if box.WidthMM <= 0 {
		box.WidthMM = fallback.WidthMM
	}
	if box.HeightMM <= 0 {
		box.HeightMM = fallback.HeightMM
	}
	return box
}

func renderSalesOrderTextBlocks(pdf *gofpdf.Fpdf, box salesdomain.SalesOrderLayoutBox, snapshot salesdomain.SalesOrderSnapshot) float64 {
	y := box.YMM
	for _, section := range salesOrderPaymentTextSections(snapshot) {
		y = renderSalesOrderTextBlock(pdf, box, y, section.title, section.lines)
	}
	return y - box.YMM
}

func salesOrderPaymentTextSections(snapshot salesdomain.SalesOrderSnapshot) []salesOrderPaymentTextSection {
	sections := make([]salesOrderPaymentTextSection, 0, 3)
	if lines := salesOrderTextLines(snapshot.PaymentText); len(lines) > 0 {
		sections = append(sections, salesOrderPaymentTextSection{title: "收款方式", lines: lines})
	}
	if lines := salesOrderTextLines(snapshot.Note); len(lines) > 0 {
		sections = append(sections, salesOrderPaymentTextSection{title: "说明", lines: lines})
	}
	if lines := renderSalesOrderAccountLines(snapshot); len(lines) > 0 {
		sections = append(sections, salesOrderPaymentTextSection{title: "公账收款", lines: lines})
	}
	return sections
}

func renderSalesOrderTextBlock(pdf *gofpdf.Fpdf, box salesdomain.SalesOrderLayoutBox, y float64, title string, lines []string) float64 {
	if len(lines) == 0 {
		return y
	}
	bottom := box.YMM + box.HeightMM
	if y+5.5 > bottom {
		return y
	}
	pdf.SetXY(box.XMM, y)
	pdf.SetFont("noto", "", 10)
	pdf.CellFormat(box.WidthMM, 5.5, title, "", 0, "L", false, 0, "")
	y += 6
	for _, line := range lines {
		for _, wrapped := range pdf.SplitText(line, box.WidthMM) {
			if y+salesOrderPaymentLineMM > bottom {
				return bottom
			}
			pdf.SetXY(box.XMM, y)
			pdf.CellFormat(box.WidthMM, salesOrderPaymentLineMM, wrapped, "", 0, "L", false, 0, "")
			y += salesOrderPaymentLineMM
		}
	}
	return y + salesOrderPaymentBlockGapMM
}

func salesOrderTextLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

func renderSalesOrderAccountLines(snapshot salesdomain.SalesOrderSnapshot) []string {
	lines := make([]string, 0, 5)
	if snapshot.BankAccountName != "" {
		lines = append(lines, "户名："+snapshot.BankAccountName)
	}
	if snapshot.TaxpayerID != "" {
		lines = append(lines, "纳税人识别号："+snapshot.TaxpayerID)
	}
	if snapshot.CompanyAddress != "" {
		lines = append(lines, "地址："+snapshot.CompanyAddress)
	}
	if snapshot.BankName != "" {
		lines = append(lines, "开户行："+snapshot.BankName)
	}
	if snapshot.BankAccountNo != "" {
		lines = append(lines, "账号："+snapshot.BankAccountNo)
	}
	return lines
}

func writeSalesOrderLabeledMultiline(pdf *gofpdf.Fpdf, label, text string) {
	for _, line := range salesOrderMultilineLines(label, text) {
		pdf.MultiCell(0, 6, line, "", "L", false)
	}
}

func salesOrderMultilineLines(label, text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return nil
	}
	prefix := strings.TrimSpace(label)
	if prefix != "" {
		lines[0] = prefix + "：" + lines[0]
	}
	return lines
}

func (r SalesOrderRenderer) renderSealStamp(pdf *gofpdf.Fpdf, ref salesdomain.SalesOrderAssetRef) {
	path, ok := r.resolveAssetPath(ref.ObjectKey)
	if !ok {
		return
	}
	imageType := salesOrderImageType(ref.ContentType, path)
	if imageType == "" {
		return
	}
	opts := gofpdf.ImageOptions{ImageType: imageType, ReadDpi: true}
	info := pdf.RegisterImageOptions(path, opts)
	if info == nil || pdf.Error() != nil {
		return
	}
	pos := salesOrderSealPosition(ref.XMM, ref.YMM, ref.WidthMM)
	imageW, imageH := info.Extent()
	box := fitSalesOrderImageInBox(imageW, imageH, pos.XMM, pos.YMM, pos.WidthMM, pos.HeightMM)
	pdf.ImageOptions(path, box.XMM, box.YMM, box.WidthMM, box.HeightMM, false, opts, 0, "")
}

func (r SalesOrderRenderer) renderPaymentCodes(pdf *gofpdf.Fpdf, codes []salesdomain.SalesOrderAssetRef, box salesdomain.SalesOrderLayoutBox) float64 {
	if len(codes) == 0 {
		return 0
	}
	metrics := salesOrderPaymentCodeMetricsForBox(len(codes), box)
	x := box.XMM
	y := box.YMM
	for _, code := range codes {
		r.renderPaymentCodeCell(pdf, code, x, y, metrics)
		if metrics.Stacked {
			y += metrics.CellHeight + metrics.Gap
			x = box.XMM
		} else {
			x += metrics.CellWidth + metrics.Gap
		}
	}
	if metrics.Stacked {
		return y - metrics.Gap - box.YMM
	}
	return metrics.CellHeight
}

func salesOrderPaymentCodeMetrics(count int) salesOrderPaymentCodeLayout {
	if count <= 1 {
		return salesOrderPaymentCodeLayout{CellWidth: 88, ImageSize: 70, CellHeight: 96, Gap: 0, Stacked: false}
	}
	return salesOrderPaymentCodeLayout{CellWidth: 88, ImageSize: 52, CellHeight: 78, Gap: 6, Stacked: true}
}

func salesOrderPaymentCodeMetricsForBox(count int, box salesdomain.SalesOrderLayoutBox) salesOrderPaymentCodeLayout {
	metrics := salesOrderPaymentCodeMetrics(count)
	if box.WidthMM > 0 {
		metrics.CellWidth = box.WidthMM
	}
	if box.HeightMM > 0 {
		if count <= 1 {
			metrics.CellHeight = box.HeightMM
		} else {
			gaps := float64(count-1) * metrics.Gap
			metrics.CellHeight = (box.HeightMM - gaps) / float64(count)
			if metrics.CellHeight < 32 {
				metrics.CellHeight = 32
			}
		}
	}
	maxImage := metrics.CellWidth
	if byHeight := metrics.CellHeight - 18; byHeight < maxImage {
		maxImage = byHeight
	}
	if maxImage > 0 && maxImage > metrics.ImageSize {
		metrics.ImageSize = maxImage
	}
	if metrics.ImageSize > metrics.CellWidth {
		metrics.ImageSize = metrics.CellWidth
	}
	return metrics
}

func (r SalesOrderRenderer) renderPaymentCodeCell(pdf *gofpdf.Fpdf, ref salesdomain.SalesOrderAssetRef, x, y float64, metrics salesOrderPaymentCodeLayout) {
	label := strings.TrimSpace(ref.Label)
	if label == "" {
		label = "收款码"
	}
	pdf.SetXY(x, y)
	pdf.MultiCell(metrics.CellWidth, 5, label, "", "C", false)

	path, ok := r.resolveAssetPath(ref.ObjectKey)
	if ok {
		imageType := salesOrderImageType(ref.ContentType, path)
		if imageType != "" {
			opts := gofpdf.ImageOptions{ImageType: imageType, ReadDpi: true}
			info := pdf.RegisterImageOptions(path, opts)
			if info != nil && pdf.Error() == nil {
				w, h := fitSalesOrderImage(info, metrics.ImageSize, metrics.ImageSize)
				pdf.ImageOptions(path, x+(metrics.CellWidth-w)/2, y+8, w, h, false, opts, 0, "")
			}
		}
	}
	if desc := strings.TrimSpace(ref.Description); desc != "" {
		pdf.SetXY(x, y+metrics.ImageSize+11)
		pdf.MultiCell(metrics.CellWidth, 4.5, desc, "", "C", false)
	}
}

func (r SalesOrderRenderer) renderAssetRef(pdf *gofpdf.Fpdf, ref salesdomain.SalesOrderAssetRef, labelPrefix string, maxW, maxH float64) {
	label := strings.TrimSpace(labelPrefix)
	if ref.Label != "" {
		label += "：" + ref.Label
	}
	if ref.Description != "" {
		label += " - " + ref.Description
	}
	if label != "" {
		pdf.MultiCell(0, 6, label, "", "L", false)
	}
	path, ok := r.resolveAssetPath(ref.ObjectKey)
	if !ok {
		return
	}
	imageType := salesOrderImageType(ref.ContentType, path)
	if imageType == "" {
		return
	}
	opts := gofpdf.ImageOptions{ImageType: imageType, ReadDpi: true}
	info := pdf.RegisterImageOptions(path, opts)
	if info == nil || pdf.Error() != nil {
		return
	}
	w, h := fitSalesOrderImage(info, maxW, maxH)
	x, y := pdf.GetXY()
	pdf.ImageOptions(path, x, y, w, h, false, opts, 0, "")
	pdf.Ln(h + 3)
}

func (r SalesOrderRenderer) resolveAssetPath(objectKey string) (string, bool) {
	base := strings.TrimSpace(r.AssetBaseDir)
	key := strings.TrimSpace(objectKey)
	if base == "" || key == "" {
		return "", false
	}
	cleanKey := filepath.Clean(filepath.FromSlash(key))
	if filepath.IsAbs(cleanKey) || cleanKey == ".." || strings.HasPrefix(cleanKey, ".."+string(os.PathSeparator)) {
		return "", false
	}
	path := filepath.Join(base, cleanKey)
	if _, err := os.Stat(path); err != nil {
		return "", false
	}
	return path, true
}

func salesOrderImageType(contentType, path string) string {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/png":
		return "PNG"
	case "image/jpeg", "image/jpg":
		return "JPG"
	case "image/gif":
		return "GIF"
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return "PNG"
	case ".jpg", ".jpeg":
		return "JPG"
	case ".gif":
		return "GIF"
	}
	return salesOrderImageTypeByMagic(path)
}

func salesOrderImageTypeByMagic(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	buf := make([]byte, 12)
	n, _ := f.Read(buf)
	head := buf[:n]
	if len(head) >= 3 && head[0] == 0xff && head[1] == 0xd8 && head[2] == 0xff {
		return "JPG"
	}
	if bytes.HasPrefix(head, []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		return "PNG"
	}
	if bytes.HasPrefix(head, []byte("GIF87a")) || bytes.HasPrefix(head, []byte("GIF89a")) {
		return "GIF"
	}
	return ""
}

func fitSalesOrderImage(info *gofpdf.ImageInfoType, maxW, maxH float64) (float64, float64) {
	w, h := info.Extent()
	if w <= 0 || h <= 0 {
		return maxW, maxH
	}
	scale := maxW / w
	if maxH/h < scale {
		scale = maxH / h
	}
	if scale <= 0 {
		return maxW, maxH
	}
	return w * scale, h * scale
}

func fitSalesOrderImageInBox(imageW, imageH, x, y, maxW, maxH float64) salesOrderSealBox {
	if imageW <= 0 || imageH <= 0 || maxW <= 0 || maxH <= 0 {
		return salesOrderSealBox{XMM: x, YMM: y, WidthMM: maxW, HeightMM: maxH}
	}
	scale := maxW / imageW
	if maxH/imageH < scale {
		scale = maxH / imageH
	}
	width := imageW * scale
	height := imageH * scale
	return salesOrderSealBox{
		XMM:      x + (maxW-width)/2,
		YMM:      y + (maxH-height)/2,
		WidthMM:  width,
		HeightMM: height,
	}
}

func salesOrderSealPosition(xMM, yMM, widthMM float64) salesOrderSealBox {
	if isLegacyDefaultSalesOrderSeal(xMM, yMM, widthMM) {
		xMM = salesOrderSealDefaultXMM
		yMM = salesOrderSealDefaultYMM
		widthMM = salesOrderSealDefaultWidthMM
	}
	if xMM <= 0 {
		xMM = salesOrderSealDefaultXMM
	}
	if yMM <= 0 {
		yMM = salesOrderSealDefaultYMM
	}
	if widthMM <= 0 {
		widthMM = salesOrderSealDefaultWidthMM
	}
	return salesOrderSealBox{XMM: xMM, YMM: yMM, WidthMM: widthMM, HeightMM: widthMM * salesOrderSealHeightRatio}
}

func isLegacyDefaultSalesOrderSeal(xMM, yMM, widthMM float64) bool {
	return xMM == salesOrderSealLegacyXMM && yMM == salesOrderSealLegacyYMM && widthMM == salesOrderSealLegacyWidthMM
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if s := strings.TrimSpace(value); s != "" {
			return s
		}
	}
	return ""
}

func (r SalesOrderRenderer) resolveFontPath() (string, error) {
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
	return "", fmt.Errorf("sales order PDF font not found")
}
