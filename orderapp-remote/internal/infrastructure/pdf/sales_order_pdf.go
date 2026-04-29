package pdf

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	salesdomain "orderapp/internal/domain/sales"

	"github.com/jung-kurt/gofpdf"
)

type SalesOrderRenderer struct {
	FontPath string
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
	pdf.CellFormat(88, 7, "客户："+snapshot.CustomerName, "", 0, "L", false, 0, "")
	pdf.CellFormat(0, 7, "日期："+snapshot.OrderDate, "", 1, "R", false, 0, "")
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
		pdf.MultiCell(0, 6, "收款方式："+snapshot.PaymentText, "", "L", false)
	}
	for _, code := range snapshot.PaymentCodes {
		text := "收款码：" + code.Label
		if code.Description != "" {
			text += " - " + code.Description
		}
		pdf.MultiCell(0, 6, text, "", "L", false)
	}
	if snapshot.Note != "" {
		pdf.MultiCell(0, 6, "说明："+snapshot.Note, "", "L", false)
	}
	if snapshot.Seal != nil && snapshot.Seal.Label != "" {
		pdf.CellFormat(0, 7, "公章："+snapshot.Seal.Label, "", 1, "L", false, 0, "")
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
