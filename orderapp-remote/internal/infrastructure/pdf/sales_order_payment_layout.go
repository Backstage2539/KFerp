package pdf

import (
	"fmt"
	"github.com/jung-kurt/gofpdf"
	salesdomain "orderapp/internal/domain/sales"
	"path/filepath"
	"strings"
)

// PrepareSalesOrderLayout also supplies preview handles with the measured box
// sizes. Page zero means the last page, determined only after the body is laid out.
func (r SalesOrderRenderer) PrepareSalesOrderLayout(snapshot salesdomain.SalesOrderSnapshot) (salesdomain.SalesOrderSnapshot, error) {
	if !salesOrderSnapshotHasPaymentInfo(snapshot) {
		return snapshot, nil
	}
	fontPath, err := r.resolveFontPath()
	if err != nil {
		return snapshot, err
	}
	p := gofpdf.NewCustom(&gofpdf.InitType{OrientationStr: "P", UnitStr: "mm", SizeStr: "A4", FontDirStr: filepath.Dir(fontPath)})
	p.SetMargins(16, 14, 16)
	p.SetAutoPageBreak(true, 18)
	p.AddUTF8Font("noto", "", filepath.Base(fontPath))
	p.AddPage()
	return prepareSalesOrderPaymentBoxes(p, snapshot)
}

func prepareSalesOrderPaymentBoxes(p *gofpdf.Fpdf, snapshot salesdomain.SalesOrderSnapshot) (salesdomain.SalesOrderSnapshot, error) {
	textBox, codeBox := salesOrderPaymentLayoutBoxes(snapshot)
	p.SetFont("noto", "", 10)
	textHeight := 0.0
	for _, section := range salesOrderPaymentTextSections(snapshot) {
		textHeight += 6 + salesOrderPaymentBlockGapMM
		for _, line := range section.lines {
			textHeight += float64(len(salesOrderWrapCellText(p, line, textBox.WidthMM))) * salesOrderPaymentLineMM
		}
	}
	textBox.HeightMM = maxFloat64(textBox.HeightMM, textHeight)
	if count := len(snapshot.PaymentCodes); count > 0 {
		extra := salesOrderPaymentCodeExtraHeight(p, snapshot.PaymentCodes, codeBox.WidthMM)
		codeBox.HeightMM = maxFloat64(codeBox.HeightMM, float64(count)*(24+extra)+float64(count-1)*6)
	}
	_, pageH := p.GetPageSize()
	_, top, _, bottom := p.GetMargins()
	for _, box := range []*salesdomain.SalesOrderLayoutBox{&textBox, &codeBox} {
		if box.HeightMM > pageH-top-bottom || box.WidthMM > 194 || box.WidthMM < 12 {
			return snapshot, fmt.Errorf("收款说明或收款码超过一页容量，请缩短说明、减少收款码或调整版式")
		}
		*box = fitSalesOrderLayoutBoxWithinPDFPage(p, *box)
		box.PageNumber = 0
	}
	snapshot.PaymentTextBox, snapshot.PaymentCodeBox = textBox, codeBox
	return snapshot, p.Error()
}

func salesOrderPaymentCodeExtraHeight(p *gofpdf.Fpdf, codes []salesdomain.SalesOrderAssetRef, width float64) float64 {
	extra := 18.0
	for _, code := range codes {
		label := strings.TrimSpace(code.Label)
		if label == "" {
			label = "收款码"
		}
		h := float64(len(salesOrderWrapCellText(p, label, width)))*5 + 6
		if code.Description != "" {
			h += float64(len(salesOrderWrapCellText(p, code.Description, width))) * 4.5
		}
		extra = maxFloat64(extra, h)
	}
	return extra
}
