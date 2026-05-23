package pdf

import (
	"bytes"
	"image"
	"image/color"
	imagedraw "image/draw"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"math"
	"os"
	"strings"

	salesdomain "orderapp/internal/domain/sales"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	salesOrderPNGDesignWidth            = 1240
	salesOrderPNGDesignHeight           = 1754
	salesOrderPNGScale                  = 2
	salesOrderPNGWidth                  = salesOrderPNGDesignWidth * salesOrderPNGScale
	salesOrderPNGHeight                 = salesOrderPNGDesignHeight * salesOrderPNGScale
	salesOrderPNGMargin                 = 70
	salesOrderPNGDPI                    = 72
	salesOrderPNGTextWeightOffsetPixels = 1
)

type salesOrderPNGCanvas struct {
	img      *image.RGBA
	renderer SalesOrderRenderer
	font     *opentype.Font
	scale    int
}

func (r SalesOrderRenderer) RenderPNG(snapshot salesdomain.SalesOrderSnapshot) ([]byte, error) {
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

	canvas := salesOrderPNGCanvas{
		img:      image.NewRGBA(image.Rect(0, 0, salesOrderPNGWidth, salesOrderPNGHeight)),
		renderer: r,
		font:     parsedFont,
		scale:    salesOrderPNGScale,
	}
	imagedraw.Draw(canvas.img, canvas.img.Bounds(), image.NewUniform(color.White), image.Point{}, imagedraw.Src)
	canvas.render(snapshot)

	var buf bytes.Buffer
	if err := png.Encode(&buf, canvas.img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (c *salesOrderPNGCanvas) render(snapshot salesdomain.SalesOrderSnapshot) {
	left := salesOrderPNGMargin
	right := salesOrderPNGDesignWidth - salesOrderPNGMargin
	y := 62
	c.text(left, y, 24, color.RGBA{R: 20, G: 20, B: 20, A: 255}, snapshot.CompanyName)
	c.textRight(right, y, 22, color.RGBA{R: 20, G: 20, B: 20, A: 255}, "销售单 SALES ORDER")
	y += 58
	c.line(left, y, right, y, color.RGBA{R: 20, G: 20, B: 20, A: 255})
	if snapshot.Seal != nil {
		c.drawSeal(*snapshot.Seal)
	}

	y += 42
	colW := (right - left) / 3
	rowH := c.metaRow(y, []string{
		"订单号：" + snapshot.OrderNo,
		"订单日期：" + snapshot.OrderDate,
		"客户：" + snapshot.CustomerName,
	}, colW)
	y += rowH + 6
	rowH = c.metaRow(y, []string{
		"客户公司：" + firstNonEmpty(snapshot.CustomerCompanyName, snapshot.CustomerName),
		"联系电话：" + snapshot.CustomerCompanyPhone,
		"公司地址：" + snapshot.CustomerCompanyAddress,
	}, colW)
	y += rowH + 30

	y = c.itemsTable(left, right, y, snapshot)
	y = c.totals(left, right, y, snapshot)
	c.paymentInfo(left, right, y+28, snapshot)
}

func (c *salesOrderPNGCanvas) metaRow(y int, cols []string, colW int) int {
	x := salesOrderPNGMargin
	maxH := 0
	for _, text := range cols {
		h := c.wrappedText(x, y, colW-24, 20, 28, color.RGBA{R: 82, G: 82, B: 82, A: 255}, []string{text})
		if h-y > maxH {
			maxH = h - y
		}
		x += colW
	}
	return maxH
}

func (c *salesOrderPNGCanvas) itemsTable(left, right, y int, snapshot salesdomain.SalesOrderSnapshot) int {
	hasDiscount := salesOrderSnapshotHasDiscount(snapshot)
	widths := salesOrderPNGItemColumnWidths(right-left, hasDiscount)
	headers := salesOrderItemHeaders(hasDiscount)
	x := left
	for i, header := range headers {
		c.text(x+8, y, 21, color.RGBA{R: 20, G: 20, B: 20, A: 255}, header)
		x += widths[i]
	}
	y += 36
	c.line(left, y, right, y, color.RGBA{R: 30, G: 30, B: 30, A: 255})
	y += 20
	for _, item := range snapshot.Items {
		rowH := c.salesOrderPNGItemRowHeight(item, widths, hasDiscount, 20, 28)
		x = left
		// salesOrderItemCells keeps item.Note in the final table column.
		for i, text := range salesOrderItemCells(item, hasDiscount) {
			if i >= len(widths) {
				break
			}
			c.wrappedText(x+8, y, widths[i]-16, 20, 28, color.RGBA{R: 28, G: 28, B: 28, A: 255}, []string{text})
			x += widths[i]
		}
		y += rowH
		c.line(left, y, right, y, color.RGBA{R: 222, G: 216, B: 207, A: 255})
		y += 8
	}
	return y + 4
}

func salesOrderPNGItemColumnWidths(usableW int, hasDiscount bool) []int {
	if hasDiscount {
		return salesOrderScalePNGWidths(usableW, []float64{0.27, 0.11, 0.10, 0.11, 0.12, 0.11, 0.18})
	}
	return salesOrderScalePNGWidths(usableW, []float64{0.33, 0.13, 0.12, 0.13, 0.12, 0.17})
}

func salesOrderScalePNGWidths(usableW int, ratios []float64) []int {
	widths := make([]int, len(ratios))
	used := 0
	for i, ratio := range ratios {
		if i == len(ratios)-1 {
			widths[i] = usableW - used
			break
		}
		widths[i] = int(float64(usableW) * ratio)
		used += widths[i]
	}
	return widths
}

func (c *salesOrderPNGCanvas) salesOrderPNGItemRowHeight(item salesdomain.SalesOrderSnapshotItem, widths []int, hasDiscount bool, fontSize float64, lineHeight int) int {
	maxLines := 1
	for i, text := range salesOrderItemCells(item, hasDiscount) {
		if i >= len(widths) {
			break
		}
		lineCount := len(c.wrapLine(text, widths[i]-16, fontSize))
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

func (c *salesOrderPNGCanvas) totals(left, right, y int, snapshot salesdomain.SalesOrderSnapshot) int {
	for _, row := range salesOrderFinancialRows(snapshot) {
		size := 22.0
		col := color.RGBA{R: 20, G: 20, B: 20, A: 255}
		if row.Bold {
			size = 23
			col = color.RGBA{R: 0, G: 0, B: 0, A: 255}
		}
		if len(row.Cells) > 0 {
			cellW := (right - left) / len(row.Cells)
			for i, text := range row.Cells {
				cellLeft := left + i*cellW
				cellRight := cellLeft + cellW
				if i == 0 {
					c.text(cellLeft+8, y+8, size, col, text)
				} else {
					c.textRight(cellRight-8, y+8, size, col, text)
				}
			}
			y += 38
			continue
		}
		text := row.Label + "： " + row.Value
		if salesOrderFinancialRowWrapLeft(row) {
			y = c.wrappedText(left+8, y+8, right-left-16, size, 32, col, []string{text})
			continue
		}
		c.textRight(right, y+8, size, col, text)
		y += 34
	}
	y += 8
	c.line(left, y, right, y, color.RGBA{R: 30, G: 30, B: 30, A: 255})
	return y
}

func (c *salesOrderPNGCanvas) paymentInfo(_, _, _ int, snapshot salesdomain.SalesOrderSnapshot) {
	sections := salesOrderPaymentTextSections(snapshot)
	if len(sections) == 0 && len(snapshot.PaymentCodes) == 0 {
		return
	}
	textBox, codeBox := salesOrderPaymentLayoutBoxes(snapshot)
	textX := salesOrderPNGMMToPX(textBox.XMM)
	textY := salesOrderPNGMMToPX(textBox.YMM)
	textW := salesOrderPNGMMToPX(textBox.WidthMM)
	textBottom := salesOrderPNGMMToPX(textBox.YMM + textBox.HeightMM)
	for _, section := range sections {
		textY = c.textBlock(textX, textY, textW, textBottom, section.title, section.lines)
	}
	if len(snapshot.PaymentCodes) > 0 {
		c.paymentCodes(
			salesOrderPNGMMToPX(codeBox.XMM),
			salesOrderPNGMMToPX(codeBox.YMM),
			salesOrderPNGMMToPX(codeBox.WidthMM),
			salesOrderPNGMMToPX(codeBox.HeightMM),
			snapshot.PaymentCodes,
		)
	}
}

func (c *salesOrderPNGCanvas) textBlock(x, y, width, bottom int, title string, lines []string) int {
	if len(lines) == 0 {
		return y
	}
	if y+28 > bottom {
		return y
	}
	c.text(x, y, 20, color.RGBA{R: 74, G: 74, B: 74, A: 255}, title)
	y += 30
	y = c.wrappedTextInBox(x, y, width, bottom, 20, 30, color.RGBA{R: 74, G: 74, B: 74, A: 255}, lines)
	return y + 18
}

func (c *salesOrderPNGCanvas) paymentCodes(x, y, width, height int, codes []salesdomain.SalesOrderAssetRef) {
	if len(codes) == 0 {
		return
	}
	metrics := salesOrderPNGPaymentCodeMetrics(len(codes), width, height)
	for _, code := range codes {
		label := strings.TrimSpace(code.Label)
		if label == "" {
			label = "收款码"
		}
		c.textCenter(x, y, width, 20, color.RGBA{R: 30, G: 30, B: 30, A: 255}, label)
		imageX := x + (width-metrics.ImageSize)/2
		imageY := y + salesOrderPNGMMToPX(8)
		if !c.assetImageSharp(code.ObjectKey, imageX, imageY, metrics.ImageSize, metrics.ImageSize) {
			c.rect(imageX, imageY, metrics.ImageSize, metrics.ImageSize, color.RGBA{R: 238, G: 232, B: 222, A: 255})
		}
		if desc := strings.TrimSpace(code.Description); desc != "" {
			c.wrappedText(x, imageY+metrics.ImageSize+24, width, 16, 24, color.RGBA{R: 90, G: 90, B: 90, A: 255}, []string{desc})
		}
		y += metrics.ImageSize + 88 + metrics.Gap
	}
}

type salesOrderPNGPaymentCodeLayout struct {
	ImageSize int
	Gap       int
	CellH     int
}

func salesOrderPNGPaymentCodeMetrics(count, width, height int) salesOrderPNGPaymentCodeLayout {
	if count <= 1 {
		size := minInt(width, height-80)
		if size < 330 {
			size = 330
		}
		return salesOrderPNGPaymentCodeLayout{ImageSize: size, Gap: 28, CellH: height}
	}
	cellH := (height - (count-1)*24) / count
	size := minInt(width, cellH-80)
	if size < 270 {
		size = 270
	}
	return salesOrderPNGPaymentCodeLayout{ImageSize: size, Gap: 24, CellH: cellH}
}

func (c *salesOrderPNGCanvas) drawSeal(ref salesdomain.SalesOrderAssetRef) {
	pos := salesOrderSealPosition(ref.XMM, ref.YMM, ref.WidthMM)
	scale := float64(salesOrderPNGDesignWidth) / 210.0
	x := int(math.Round(pos.XMM * scale))
	y := int(math.Round(pos.YMM * scale))
	w := int(math.Round(pos.WidthMM * scale))
	h := int(math.Round(pos.HeightMM * scale))
	c.assetImage(ref.ObjectKey, x, y, w, h)
}

func (c *salesOrderPNGCanvas) assetImage(objectKey string, x, y, maxW, maxH int) bool {
	return c.assetImageWithScaler(objectKey, x, y, maxW, maxH, false)
}

func (c *salesOrderPNGCanvas) assetImageSharp(objectKey string, x, y, maxW, maxH int) bool {
	return c.assetImageWithScaler(objectKey, x, y, maxW, maxH, true)
}

func (c *salesOrderPNGCanvas) assetImageWithScaler(objectKey string, x, y, maxW, maxH int, sharp bool) bool {
	path, ok := c.renderer.resolveAssetPath(objectKey)
	if !ok {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	src, _, err := image.Decode(f)
	if err != nil {
		return false
	}
	bounds := src.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 || maxW <= 0 || maxH <= 0 {
		return false
	}
	scale := math.Min(float64(maxW)/float64(bounds.Dx()), float64(maxH)/float64(bounds.Dy()))
	w := int(math.Round(float64(bounds.Dx()) * scale))
	h := int(math.Round(float64(bounds.Dy()) * scale))
	if w <= 0 || h <= 0 {
		return false
	}
	dstX := x + (maxW-w)/2
	dstY := y + (maxH-h)/2
	dst := image.Rect(c.px(dstX), c.px(dstY), c.px(dstX+w), c.px(dstY+h))
	if sharp {
		xdraw.NearestNeighbor.Scale(c.img, dst, src, bounds, imagedraw.Over, nil)
		return true
	}
	xdraw.CatmullRom.Scale(c.img, dst, src, bounds, imagedraw.Over, nil)
	return true
}

func (c *salesOrderPNGCanvas) wrappedText(x, y, width int, size float64, lineH int, col color.Color, paragraphs []string) int {
	for _, paragraph := range paragraphs {
		lines := c.wrapLine(paragraph, width, size)
		for _, line := range lines {
			c.text(x, y, size, col, line)
			y += lineH
		}
		if paragraph == "" {
			y += lineH
		}
	}
	return y
}

func (c *salesOrderPNGCanvas) wrappedTextInBox(x, y, width, bottom int, size float64, lineH int, col color.Color, paragraphs []string) int {
	for _, paragraph := range paragraphs {
		lines := c.wrapLine(paragraph, width, size)
		for _, line := range lines {
			if y+lineH > bottom {
				return bottom
			}
			c.text(x, y, size, col, line)
			y += lineH
		}
		if paragraph == "" {
			y += lineH
		}
	}
	return y
}

func (c *salesOrderPNGCanvas) wrapLine(line string, width int, size float64) []string {
	line = strings.TrimRight(line, "\n")
	if strings.TrimSpace(line) == "" {
		return []string{""}
	}
	out := make([]string, 0, 2)
	current := ""
	for _, r := range line {
		next := current + string(r)
		if current != "" && c.measure(next, size) > width {
			out = append(out, current)
			current = string(r)
			continue
		}
		current = next
	}
	if current != "" {
		out = append(out, current)
	}
	return out
}

func (c *salesOrderPNGCanvas) text(x, y int, size float64, col color.Color, text string) {
	face := c.face(size)
	dot := fixed.P(c.px(x), c.px(y)+face.Metrics().Ascent.Ceil())
	d := &font.Drawer{
		Dst:  c.img,
		Src:  image.NewUniform(col),
		Face: face,
		Dot:  dot,
	}
	d.DrawString(text)
	if salesOrderPNGTextWeightOffsetPixels > 0 {
		d.Dot = fixed.Point26_6{
			X: dot.X + fixed.I(salesOrderPNGTextWeightOffsetPixels),
			Y: dot.Y,
		}
		d.DrawString(text)
	}
}

func (c *salesOrderPNGCanvas) textRight(x, y int, size float64, col color.Color, text string) {
	c.text(x-c.measure(text, size), y, size, col, text)
}

func (c *salesOrderPNGCanvas) textCenter(x, y, width int, size float64, col color.Color, text string) {
	c.text(x+(width-c.measure(text, size))/2, y, size, col, text)
}

func (c *salesOrderPNGCanvas) measure(text string, size float64) int {
	face := c.face(size)
	d := &font.Drawer{Face: face}
	return int(math.Ceil(float64(d.MeasureString(text).Ceil()) / float64(c.effectiveScale())))
}

func (c *salesOrderPNGCanvas) face(size float64) font.Face {
	face, err := opentype.NewFace(c.font, &opentype.FaceOptions{
		Size:    size * float64(c.effectiveScale()),
		DPI:     salesOrderPNGDPI,
		Hinting: font.HintingFull,
	})
	if err != nil {
		panic(err)
	}
	return face
}

func (c *salesOrderPNGCanvas) line(x1, y1, x2, y2 int, col color.Color) {
	if y1 == y2 {
		thickness := 2 * c.effectiveScale()
		imagedraw.Draw(c.img, image.Rect(c.px(x1), c.px(y1), c.px(x2), c.px(y1)+thickness), image.NewUniform(col), image.Point{}, imagedraw.Src)
		return
	}
	imagedraw.Draw(c.img, image.Rect(c.px(x1), c.px(y1), c.px(x2), c.px(y2)), image.NewUniform(col), image.Point{}, imagedraw.Src)
}

func (c *salesOrderPNGCanvas) rect(x, y, w, h int, col color.Color) {
	imagedraw.Draw(c.img, image.Rect(c.px(x), c.px(y), c.px(x+w), c.px(y+h)), image.NewUniform(col), image.Point{}, imagedraw.Src)
}

func (c *salesOrderPNGCanvas) effectiveScale() int {
	if c.scale <= 0 {
		return 1
	}
	return c.scale
}

func (c *salesOrderPNGCanvas) px(v int) int {
	return v * c.effectiveScale()
}

func salesOrderPNGMMToPX(mm float64) int {
	return int(math.Round(mm * float64(salesOrderPNGDesignWidth) / 210.0))
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
