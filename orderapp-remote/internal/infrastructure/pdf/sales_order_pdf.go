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
	salesOrderSealHeightRatio    = 0.62
	salesOrderPaymentLineMM      = 6.2
	salesOrderPaymentBlockGapMM  = 3.5
)

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
	colWidths := []float64{usableW * 0.27, usableW * 0.12, usableW * 0.13, usableW * 0.15, usableW * 0.15, usableW * 0.18}
	headers := []string{"商品", "规格", "数量", "单价", "小计", "备注"}
	pdf.SetFont("noto", "", 10)
	for i, h := range headers {
		pdf.CellFormat(colWidths[i], 8, h, "B", 0, "L", false, 0, "")
	}
	pdf.Ln(-1)
	for _, item := range snapshot.Items {
		pdf.CellFormat(colWidths[0], 8, item.Name, "B", 0, "L", false, 0, "")
		pdf.CellFormat(colWidths[1], 8, item.Spec, "B", 0, "L", false, 0, "")
		pdf.CellFormat(colWidths[2], 8, strings.TrimSpace(item.Qty+item.Unit), "B", 0, "L", false, 0, "")
		pdf.CellFormat(colWidths[3], 8, item.UnitPrice, "B", 0, "L", false, 0, "")
		pdf.CellFormat(colWidths[4], 8, item.LineTotal, "B", 0, "L", false, 0, "")
		pdf.CellFormat(colWidths[5], 8, item.Note, "B", 0, "L", false, 0, "")
		pdf.Ln(-1)
	}
	pdf.Ln(4)
}

func renderSalesOrderTotals(pdf *gofpdf.Fpdf, snapshot salesdomain.SalesOrderSnapshot) {
	pdf.SetFont("noto", "", 11)
	line := "商品合计： " + snapshot.TotalAmount + "   运费： " + snapshot.Shipping + "   优惠： " + snapshot.Discount + "   应收： " + snapshot.GrandTotal
	pdf.CellFormat(0, 8, line, "B", 1, "R", false, 0, "")
	pdf.Ln(4)
}

func (r SalesOrderRenderer) renderSalesOrderPaymentInfoSection(pdf *gofpdf.Fpdf, snapshot salesdomain.SalesOrderSnapshot) {
	if snapshot.PaymentText == "" && snapshot.Note == "" && len(snapshot.PaymentCodes) == 0 && len(renderSalesOrderAccountLines(snapshot)) == 0 {
		return
	}
	left, _, right, _ := pdf.GetMargins()
	pageW, _ := pdf.GetPageSize()
	contentW := pageW - left - right
	startX, startY := pdf.GetXY()
	pdf.SetFont("noto", "", 11)
	pdf.CellFormat(0, 7, "收款与说明", "B", 1, "L", false, 0, "")
	startY = pdf.GetY() + 3
	codeW := 0.0
	if len(snapshot.PaymentCodes) > 0 {
		codeW = salesOrderPaymentCodeMetrics(len(snapshot.PaymentCodes)).CellWidth
	}
	gap := 8.0
	textW := contentW
	if codeW > 0 {
		textW = contentW - codeW - gap
	}
	textH := renderSalesOrderTextBlocks(pdf, startX, startY, textW, snapshot)
	codeH := 0.0
	if codeW > 0 {
		codeH = r.renderPaymentCodes(pdf, snapshot.PaymentCodes, startX+textW+gap, startY, codeW)
	}
	if codeH > textH {
		textH = codeH
	}
	pdf.SetXY(startX, startY+textH+5)
}

func renderSalesOrderTextBlocks(pdf *gofpdf.Fpdf, x, y, width float64, snapshot salesdomain.SalesOrderSnapshot) float64 {
	startY := y
	y = renderSalesOrderTextBlock(pdf, x, y, width, "收款方式", salesOrderTextLines(snapshot.PaymentText))
	y = renderSalesOrderTextBlock(pdf, x, y, width, "公账收款", renderSalesOrderAccountLines(snapshot))
	y = renderSalesOrderTextBlock(pdf, x, y, width, "说明", salesOrderTextLines(snapshot.Note))
	return y - startY
}

func renderSalesOrderTextBlock(pdf *gofpdf.Fpdf, x, y, width float64, title string, lines []string) float64 {
	if len(lines) == 0 {
		return y
	}
	pdf.SetXY(x, y)
	pdf.SetFont("noto", "", 10)
	pdf.CellFormat(width, 6, title, "", 1, "L", false, 0, "")
	y = pdf.GetY()
	for _, line := range lines {
		pdf.SetXY(x, y)
		pdf.MultiCell(width, salesOrderPaymentLineMM, line, "", "L", false)
		y = pdf.GetY()
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

func (r SalesOrderRenderer) renderPaymentCodes(pdf *gofpdf.Fpdf, codes []salesdomain.SalesOrderAssetRef, startX, startY, availableW float64) float64 {
	if len(codes) == 0 {
		return 0
	}
	metrics := salesOrderPaymentCodeMetrics(len(codes))
	if availableW > 0 && availableW < metrics.CellWidth {
		metrics.CellWidth = availableW
	}
	x := startX
	y := startY
	for _, code := range codes {
		r.renderPaymentCodeCell(pdf, code, x, y, metrics)
		if metrics.Stacked {
			y += metrics.CellHeight + metrics.Gap
			x = startX
		} else {
			x += metrics.CellWidth + metrics.Gap
		}
	}
	if metrics.Stacked {
		return y - metrics.Gap - startY
	}
	return metrics.CellHeight
}

func salesOrderPaymentCodeMetrics(count int) salesOrderPaymentCodeLayout {
	if count <= 1 {
		return salesOrderPaymentCodeLayout{CellWidth: 88, ImageSize: 64, CellHeight: 90, Gap: 0, Stacked: false}
	}
	return salesOrderPaymentCodeLayout{CellWidth: 88, ImageSize: 52, CellHeight: 78, Gap: 6, Stacked: true}
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
