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
	widths := []int{280, 130, 150, 160, 170, 210}
	headers := []string{"商品", "规格", "数量", "单价", "小计", "备注"}
	x := left
	for i, header := range headers {
		c.text(x+8, y, 21, color.RGBA{R: 20, G: 20, B: 20, A: 255}, header)
		x += widths[i]
	}
	y += 36
	c.line(left, y, right, y, color.RGBA{R: 30, G: 30, B: 30, A: 255})
	y += 20
	for _, item := range snapshot.Items {
		x = left
		c.text(x+8, y, 20, color.RGBA{R: 28, G: 28, B: 28, A: 255}, item.Name)
		x += widths[0]
		c.text(x+8, y, 20, color.RGBA{R: 28, G: 28, B: 28, A: 255}, item.Spec)
		x += widths[1]
		c.text(x+8, y, 20, color.RGBA{R: 28, G: 28, B: 28, A: 255}, strings.TrimSpace(item.Qty+item.Unit))
		x += widths[2]
		c.text(x+8, y, 20, color.RGBA{R: 28, G: 28, B: 28, A: 255}, item.UnitPrice)
		x += widths[3]
		c.text(x+8, y, 20, color.RGBA{R: 28, G: 28, B: 28, A: 255}, item.LineTotal)
		x += widths[4]
		c.text(x+8, y, 20, color.RGBA{R: 28, G: 28, B: 28, A: 255}, item.Note)
		y += 48
		c.line(left, y, right, y, color.RGBA{R: 222, G: 216, B: 207, A: 255})
		y += 8
	}
	return y + 4
}

func (c *salesOrderPNGCanvas) totals(left, right, y int, snapshot salesdomain.SalesOrderSnapshot) int {
	line := "商品合计： " + snapshot.TotalAmount + "   运费： " + snapshot.Shipping + "   优惠： " + snapshot.Discount + "   应收： " + snapshot.GrandTotal
	c.textRight(right, y+8, 22, color.RGBA{R: 20, G: 20, B: 20, A: 255}, line)
	y += 48
	c.line(left, y, right, y, color.RGBA{R: 30, G: 30, B: 30, A: 255})
	return y
}

func (c *salesOrderPNGCanvas) paymentInfo(left, right, y int, snapshot salesdomain.SalesOrderSnapshot) {
	accountLines := renderSalesOrderAccountLines(snapshot)
	if snapshot.PaymentText == "" && snapshot.Note == "" && len(snapshot.PaymentCodes) == 0 && len(accountLines) == 0 {
		return
	}
	c.text(left, y, 22, color.RGBA{R: 44, G: 44, B: 44, A: 255}, "收款与说明")
	y += 32
	c.line(left, y, right, y, color.RGBA{R: 30, G: 30, B: 30, A: 255})
	y += 32

	codeX := left + 700
	textW := 650
	textY := y
	textY = c.textBlock(left, textY, textW, "收款方式", salesOrderTextLines(snapshot.PaymentText))
	textY = c.textBlock(left, textY, textW, "公账收款", accountLines)
	_ = c.textBlock(left, textY, textW, "说明", salesOrderTextLines(snapshot.Note))
	if len(snapshot.PaymentCodes) > 0 {
		c.paymentCodes(codeX, y, right-codeX, snapshot.PaymentCodes)
	}
}

func (c *salesOrderPNGCanvas) textBlock(x, y, width int, title string, lines []string) int {
	if len(lines) == 0 {
		return y
	}
	c.text(x, y, 20, color.RGBA{R: 74, G: 74, B: 74, A: 255}, title)
	y += 30
	y = c.wrappedText(x, y, width, 20, 30, color.RGBA{R: 74, G: 74, B: 74, A: 255}, lines)
	return y + 18
}

func (c *salesOrderPNGCanvas) paymentCodes(x, y, width int, codes []salesdomain.SalesOrderAssetRef) {
	if len(codes) == 0 {
		return
	}
	metrics := salesOrderPNGPaymentCodeMetrics(len(codes))
	for _, code := range codes {
		label := strings.TrimSpace(code.Label)
		if label == "" {
			label = "收款码"
		}
		c.textCenter(x, y, width, 20, color.RGBA{R: 30, G: 30, B: 30, A: 255}, label)
		imageX := x + (width-metrics.ImageSize)/2
		imageY := y + 34
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
}

func salesOrderPNGPaymentCodeMetrics(count int) salesOrderPNGPaymentCodeLayout {
	if count <= 1 {
		return salesOrderPNGPaymentCodeLayout{ImageSize: 330, Gap: 28}
	}
	return salesOrderPNGPaymentCodeLayout{ImageSize: 270, Gap: 24}
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
