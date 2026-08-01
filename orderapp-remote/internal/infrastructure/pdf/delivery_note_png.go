package pdf

import (
	"bytes"
	"image"
	"image/color"
	imagedraw "image/draw"
	"image/png"
	"os"
	"strings"

	salesdomain "orderapp/internal/domain/sales"

	"golang.org/x/image/font/opentype"
)

// RenderPNG renders the same delivery-note snapshot as the PDF into one
// high-resolution, vertically growing image suitable for WeChat sharing.
func (r DeliveryNoteRenderer) RenderPNG(snapshot salesdomain.DeliveryNoteSnapshot) ([]byte, error) {
	if err := snapshot.Validate(); err != nil {
		return nil, err
	}
	fontPath, err := r.resolveFontPath()
	if err != nil {
		return nil, err
	}
	fontBytes, err := os.ReadFile(fontPath)
	if err != nil {
		return nil, err
	}
	parsedFont, err := opentype.Parse(fontBytes)
	if err != nil {
		return nil, err
	}

	base := salesOrderPNGCanvas{
		renderer: SalesOrderRenderer{FontPath: r.FontPath, AssetBaseDir: r.AssetBaseDir},
		font:     parsedFont,
		scale:    salesOrderPNGScale,
	}
	measurer := deliveryNotePNGCanvas{salesOrderPNGCanvas: base}
	designHeight := measurer.documentHeight(snapshot)
	base.img = image.NewRGBA(image.Rect(0, 0, salesOrderPNGWidth, designHeight*salesOrderPNGScale))
	imagedraw.Draw(base.img, base.img.Bounds(), image.NewUniform(color.White), image.Point{}, imagedraw.Src)
	canvas := deliveryNotePNGCanvas{salesOrderPNGCanvas: base}
	canvas.render(snapshot)

	var buf bytes.Buffer
	if err := png.Encode(&buf, canvas.img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type deliveryNotePNGCanvas struct {
	salesOrderPNGCanvas
}

func (c *deliveryNotePNGCanvas) render(snapshot salesdomain.DeliveryNoteSnapshot) {
	left := salesOrderPNGMargin
	right := salesOrderPNGDesignWidth - salesOrderPNGMargin
	y := 62
	c.text(left, y, 24, color.RGBA{R: 20, G: 20, B: 20, A: 255}, snapshot.CompanyName)
	c.textRight(right, y, 22, color.RGBA{R: 20, G: 20, B: 20, A: 255}, "出库单 DELIVERY NOTE")
	y += 58
	c.line(left, y, right, y, color.RGBA{R: 20, G: 20, B: 20, A: 255})
	if snapshot.Seal != nil {
		c.drawSeal(*snapshot.Seal)
	}

	y += 42
	colW := (right - left) / 3
	for _, row := range deliveryNoteHeaderMetaRows(snapshot) {
		rowH := c.metaRow(y, row, colW)
		y += rowH + 6
	}
	address := "收货地址：" + firstNonEmpty(snapshot.ReceiverAddress, snapshot.CustomerCompanyAddress)
	y = c.wrappedText(left, y, right-left, 20, 28, color.RGBA{R: 82, G: 82, B: 82, A: 255}, []string{address})
	y += 24

	y = c.itemsTable(left, right, y, snapshot)
	c.footer(left, right, y, snapshot)
}

func (c *deliveryNotePNGCanvas) documentHeight(snapshot salesdomain.DeliveryNoteSnapshot) int {
	left := salesOrderPNGMargin
	right := salesOrderPNGDesignWidth - salesOrderPNGMargin
	y := 62 + 58 + 42
	colW := (right - left) / 3
	for _, row := range deliveryNoteHeaderMetaRows(snapshot) {
		y += c.metaRowHeight(row, colW) + 6
	}
	address := "收货地址：" + firstNonEmpty(snapshot.ReceiverAddress, snapshot.CustomerCompanyAddress)
	y += c.wrappedTextHeight(right-left, 20, 28, []string{address}) + 24
	y = c.itemsTableEndY(left, right, y, snapshot)
	y = c.footerEndY(left, right, y, snapshot)
	if snapshot.Seal != nil {
		y = maxInt(y, salesOrderPNGSealBottom(*snapshot.Seal))
	}
	return maxInt(salesOrderPNGDesignHeight, y+80)
}

func (c *deliveryNotePNGCanvas) itemsTable(left, right, y int, snapshot salesdomain.DeliveryNoteSnapshot) int {
	widths := deliveryNotePNGItemColumnWidths(right - left)
	headers := []string{"商品", "规格", "出库数量", "出库仓", "备注"}
	x := left
	for i, header := range headers {
		c.text(x+8, y, 21, color.RGBA{R: 20, G: 20, B: 20, A: 255}, header)
		x += widths[i]
	}
	y += 36
	c.line(left, y, right, y, color.RGBA{R: 30, G: 30, B: 30, A: 255})
	y += 20
	for _, item := range snapshot.Items {
		rowH := c.itemRowHeight(item, widths, 20, 28)
		x = left
		for i, value := range deliveryNotePNGItemCells(item) {
			c.wrappedText(x+8, y, widths[i]-16, 20, 28, color.RGBA{R: 28, G: 28, B: 28, A: 255}, []string{value})
			x += widths[i]
		}
		y += rowH
		c.line(left, y, right, y, color.RGBA{R: 222, G: 216, B: 207, A: 255})
		y += 8
	}
	return y + 4
}

func (c *deliveryNotePNGCanvas) itemsTableEndY(left, right, y int, snapshot salesdomain.DeliveryNoteSnapshot) int {
	widths := deliveryNotePNGItemColumnWidths(right - left)
	y += 36 + 20
	for _, item := range snapshot.Items {
		y += c.itemRowHeight(item, widths, 20, 28) + 8
	}
	return y + 4
}

func deliveryNotePNGItemColumnWidths(usableW int) []int {
	return salesOrderScalePNGWidths(usableW, []float64{0.28, 0.15, 0.17, 0.20, 0.20})
}

func deliveryNotePNGItemCells(item salesdomain.DeliveryNoteSnapshotItem) []string {
	return []string{
		item.Name,
		item.Spec,
		strings.TrimSpace(item.Qty + item.Unit),
		firstNonEmpty(item.WarehouseName, item.Warehouse),
		item.Note,
	}
}

func (c *deliveryNotePNGCanvas) itemRowHeight(item salesdomain.DeliveryNoteSnapshotItem, widths []int, fontSize float64, lineHeight int) int {
	maxLines := 1
	for i, value := range deliveryNotePNGItemCells(item) {
		lineCount := len(c.wrapLine(value, widths[i]-16, fontSize))
		if lineCount > maxLines {
			maxLines = lineCount
		}
	}
	rowH := maxLines*lineHeight + 20
	if rowH < 48 {
		return 48
	}
	return rowH
}

func (c *deliveryNotePNGCanvas) footer(left, right, y int, snapshot salesdomain.DeliveryNoteSnapshot) int {
	c.line(left, y, right, y, color.RGBA{R: 30, G: 30, B: 30, A: 255})
	y += 24
	if method := strings.TrimSpace(snapshot.DeliveryMethod); method != "" {
		y = c.wrappedText(left+8, y, right-left-16, 20, 30, color.RGBA{R: 40, G: 40, B: 40, A: 255}, []string{"发货方式：" + method})
		y += 8
	}
	if note := strings.TrimSpace(snapshot.Note); note != "" {
		y = c.wrappedText(left+8, y, right-left-16, 20, 30, color.RGBA{R: 40, G: 40, B: 40, A: 255}, []string{"备注：" + note})
		y += 8
	}
	y += 34
	c.text(left+8, y, 20, color.RGBA{R: 30, G: 30, B: 30, A: 255}, "仓库经办：________________        客户/承运签收：________________")
	return y + 36
}

func (c *deliveryNotePNGCanvas) footerEndY(left, right, y int, snapshot salesdomain.DeliveryNoteSnapshot) int {
	y += 24
	if method := strings.TrimSpace(snapshot.DeliveryMethod); method != "" {
		y += c.wrappedTextHeight(right-left-16, 20, 30, []string{"发货方式：" + method}) + 8
	}
	if note := strings.TrimSpace(snapshot.Note); note != "" {
		y += c.wrappedTextHeight(right-left-16, 20, 30, []string{"备注：" + note}) + 8
	}
	return y + 34 + 36
}
