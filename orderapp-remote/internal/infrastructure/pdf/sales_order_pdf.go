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

func (r SalesOrderRenderer) Render(snapshot salesdomain.SalesOrderSnapshot) ([]byte, error) {
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
	pdf.SetFont("noto", "", 20)
	pdf.CellFormat(0, 12, "销售单", "", 1, "C", false, 0, "")

	pdf.SetFont("noto", "", 10)
	pdf.CellFormat(88, 7, "公司："+snapshot.CompanyName, "", 0, "L", false, 0, "")
	pdf.CellFormat(0, 7, "订单号："+snapshot.OrderNo, "", 1, "R", false, 0, "")
	if snapshot.Seal != nil {
		r.renderSealStamp(pdf, *snapshot.Seal)
	}
	pdf.CellFormat(88, 7, "客户："+snapshot.CustomerName, "", 0, "L", false, 0, "")
	pdf.CellFormat(0, 7, "日期："+snapshot.OrderDate, "", 1, "R", false, 0, "")
	if snapshot.CustomerCompanyName != "" || snapshot.CustomerCompanyPhone != "" {
		pdf.CellFormat(88, 7, "客户公司："+snapshot.CustomerCompanyName, "", 0, "L", false, 0, "")
		pdf.CellFormat(0, 7, "联系电话："+snapshot.CustomerCompanyPhone, "", 1, "R", false, 0, "")
	}
	if snapshot.CustomerCompanyAddress != "" {
		pdf.MultiCell(0, 6, "公司地址："+snapshot.CustomerCompanyAddress, "", "L", false)
	}
	pdf.Ln(4)

	colWidths := []float64{62, 22, 20, 18, 30, 32}
	headers := []string{"商品", "规格", "数量", "单位", "单价", "金额"}
	pdf.SetFont("noto", "", 10)
	for i, h := range headers {
		pdf.CellFormat(colWidths[i], 8, h, "1", 0, "C", false, 0, "")
	}
	pdf.Ln(-1)
	for _, item := range snapshot.Items {
		pdf.CellFormat(colWidths[0], 8, item.Name, "1", 0, "L", false, 0, "")
		pdf.CellFormat(colWidths[1], 8, item.Spec, "1", 0, "C", false, 0, "")
		pdf.CellFormat(colWidths[2], 8, item.Qty, "1", 0, "R", false, 0, "")
		pdf.CellFormat(colWidths[3], 8, item.Unit, "1", 0, "C", false, 0, "")
		pdf.CellFormat(colWidths[4], 8, item.UnitPrice, "1", 0, "R", false, 0, "")
		pdf.CellFormat(colWidths[5], 8, item.LineTotal, "1", 0, "R", false, 0, "")
		pdf.Ln(-1)
	}
	pdf.Ln(4)
	pdf.CellFormat(0, 7, "商品金额："+snapshot.TotalAmount, "", 1, "R", false, 0, "")
	pdf.CellFormat(0, 7, "运费："+snapshot.Shipping+"    优惠："+snapshot.Discount, "", 1, "R", false, 0, "")
	pdf.SetFont("noto", "", 13)
	pdf.CellFormat(0, 9, "应收合计："+snapshot.GrandTotal, "", 1, "R", false, 0, "")
	pdf.SetFont("noto", "", 10)
	pdf.Ln(4)
	if snapshot.PaymentText != "" {
		writeSalesOrderLabeledMultiline(pdf, "收款方式", snapshot.PaymentText)
	}
	for _, code := range snapshot.PaymentCodes {
		r.renderAssetRef(pdf, code, "收款码", 36, 36)
	}
	if snapshot.Note != "" {
		writeSalesOrderLabeledMultiline(pdf, "说明", snapshot.Note)
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
	pdf.ImageOptions(path, pos.XMM, pos.YMM, pos.WidthMM, pos.HeightMM, false, opts, 0, "")
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
	default:
		return ""
	}
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

func salesOrderSealPosition(xMM, yMM, widthMM float64) salesOrderSealBox {
	if xMM <= 0 {
		xMM = 32
	}
	if yMM <= 0 {
		yMM = 22
	}
	if widthMM <= 0 {
		widthMM = 42
	}
	return salesOrderSealBox{XMM: xMM, YMM: yMM, WidthMM: widthMM, HeightMM: widthMM * 0.62}
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
