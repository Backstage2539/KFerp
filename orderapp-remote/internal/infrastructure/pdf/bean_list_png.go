package pdf

import (
	"bytes"
	"image"
	"image/color"
	imagedraw "image/draw"
	"image/png"
	"os"
	"strconv"
	"strings"

	"golang.org/x/image/font/opentype"
)

const (
	beanListPNGDesignWidth  = 1240
	beanListPNGDesignHeight = 1754
	beanListPNGScale        = 2
	beanListPNGMargin       = 70
)

func (r BeanListRenderer) RenderPNG(doc BeanListDocument) ([]byte, error) {
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

	measurer := salesOrderPNGCanvas{font: parsedFont, scale: beanListPNGScale}
	designHeight := beanListPNGDocumentHeight(measurer, doc)
	canvas := salesOrderPNGCanvas{
		img:   image.NewRGBA(image.Rect(0, 0, beanListPNGDesignWidth*beanListPNGScale, designHeight*beanListPNGScale)),
		font:  parsedFont,
		scale: beanListPNGScale,
	}
	bg := beanListPNGColor(doc.BackgroundColor, color.RGBA{R: 250, G: 247, B: 241, A: 255})
	imagedraw.Draw(canvas.img, canvas.img.Bounds(), image.NewUniform(bg), image.Point{}, imagedraw.Src)
	beanListPNGRender(&canvas, doc)

	var buf bytes.Buffer
	if err := png.Encode(&buf, canvas.img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func beanListPNGDocumentHeight(canvas salesOrderPNGCanvas, doc BeanListDocument) int {
	left := beanListPNGMargin
	right := beanListPNGDesignWidth - beanListPNGMargin
	y := 72
	y += canvas.wrappedTextHeight(right-left, 36, 48, []string{beanListPNGTitle(doc)}) + 16
	if meta := beanListPNGMeta(doc); len(meta) > 0 {
		y += canvas.wrappedTextHeight(right-left, 20, 30, []string{meta}) + 28
	}
	for _, group := range doc.Groups {
		if strings.TrimSpace(group.Category) != "" {
			y += 52
		}
		for _, item := range group.Items {
			y += beanListPNGItemHeight(canvas, right-left, item) + 22
		}
		y += 14
	}
	if strings.TrimSpace(doc.Changelog) != "" {
		y += canvas.wrappedTextHeight(right-left, 20, 30, []string{"版本说明：" + strings.TrimSpace(doc.Changelog)}) + 36
	}
	return maxInt(beanListPNGDesignHeight, y+80)
}

func beanListPNGRender(canvas *salesOrderPNGCanvas, doc BeanListDocument) {
	left := beanListPNGMargin
	right := beanListPNGDesignWidth - beanListPNGMargin
	textColor := beanListPNGColor(doc.FontColor, color.RGBA{R: 28, G: 28, B: 28, A: 255})
	mutedColor := color.RGBA{R: 92, G: 86, B: 78, A: 255}
	lineColor := color.RGBA{R: 208, G: 198, B: 184, A: 255}
	y := 72
	y = canvas.wrappedText(left, y, right-left, 36, 48, textColor, []string{beanListPNGTitle(doc)})
	meta := beanListPNGMeta(doc)
	if meta != "" {
		y += 16
		y = canvas.wrappedText(left, y, right-left, 20, 30, mutedColor, []string{meta})
	}
	y += 30
	canvas.line(left, y, right, y, lineColor)
	y += 36
	for _, group := range doc.Groups {
		if strings.TrimSpace(group.Category) != "" {
			canvas.text(left, y, 24, textColor, strings.TrimSpace(group.Category))
			y += 44
		}
		for _, item := range group.Items {
			y = beanListPNGRenderItem(canvas, left, right, y, item, textColor, mutedColor)
			y += 22
		}
		y += 14
	}
	if strings.TrimSpace(doc.Changelog) != "" {
		canvas.line(left, y, right, y, lineColor)
		y += 30
		canvas.wrappedText(left, y, right-left, 20, 30, mutedColor, []string{"版本说明：" + strings.TrimSpace(doc.Changelog)})
	}
}

func beanListPNGRenderItem(canvas *salesOrderPNGCanvas, left, right, y int, item BeanListItem, textColor color.Color, mutedColor color.Color) int {
	boxH := beanListPNGItemHeight(*canvas, right-left, item)
	canvas.rect(left, y, right-left, boxH, color.RGBA{R: 255, G: 255, B: 255, A: 230})
	innerX := left + 28
	innerW := right - left - 56
	cursor := y + 28
	title := strings.TrimSpace(item.Name)
	if strings.TrimSpace(item.Code) != "" {
		title = strings.TrimSpace(item.Code) + "  " + title
	}
	cursor = canvas.wrappedText(innerX, cursor, innerW-180, 26, 36, textColor, []string{title})
	if strings.TrimSpace(item.BadgeLabel) != "" {
		canvas.textRight(right-28, y+32, 20, color.RGBA{R: 178, G: 54, B: 38, A: 255}, strings.TrimSpace(item.BadgeLabel))
	}
	for _, line := range []string{item.RecommendedUse, item.Flavor, item.Description} {
		if strings.TrimSpace(line) == "" {
			continue
		}
		cursor += 8
		cursor = canvas.wrappedText(innerX, cursor, innerW, 19, 28, mutedColor, []string{strings.TrimSpace(line)})
	}
	if len(item.QualityLines) > 0 {
		parts := make([]string, 0, len(item.QualityLines))
		for _, line := range item.QualityLines {
			if strings.TrimSpace(line.Value) != "" {
				parts = append(parts, strings.TrimSpace(line.Label)+"："+strings.TrimSpace(line.Value))
			}
		}
		if len(parts) > 0 {
			cursor += 8
			cursor = canvas.wrappedText(innerX, cursor, innerW, 18, 26, mutedColor, []string{strings.Join(parts, " / ")})
		}
	}
	if len(item.Prices) > 0 {
		cursor += 18
		colW := innerW / maxInt(1, len(item.Prices))
		x := innerX
		for _, price := range item.Prices {
			priceColor := textColor
			if price.Red {
				priceColor = color.RGBA{R: 178, G: 54, B: 38, A: 255}
			}
			canvas.text(x, cursor, 18, mutedColor, strings.TrimSpace(price.Label))
			canvas.text(x, cursor+28, 24, priceColor, strings.TrimSpace(price.Value))
			x += colW
		}
		cursor += 70
	}
	return maxInt(y+boxH, cursor+24)
}

func beanListPNGItemHeight(canvas salesOrderPNGCanvas, width int, item BeanListItem) int {
	innerW := width - 56
	h := 56
	title := strings.TrimSpace(item.Name)
	if strings.TrimSpace(item.Code) != "" {
		title = strings.TrimSpace(item.Code) + "  " + title
	}
	h += canvas.wrappedTextHeight(innerW-180, 26, 36, []string{title})
	for _, line := range []string{item.RecommendedUse, item.Flavor, item.Description} {
		if strings.TrimSpace(line) != "" {
			h += 8 + canvas.wrappedTextHeight(innerW, 19, 28, []string{strings.TrimSpace(line)})
		}
	}
	if len(item.QualityLines) > 0 {
		h += 34
	}
	if len(item.Prices) > 0 {
		h += 88
	}
	return maxInt(168, h+24)
}

func beanListPNGTitle(doc BeanListDocument) string {
	for _, value := range []string{doc.Title, doc.BrandName} {
		if text := strings.TrimSpace(value); text != "" {
			return text
		}
	}
	return "销售豆单"
}

func beanListPNGMeta(doc BeanListDocument) string {
	parts := []string{}
	if strings.TrimSpace(doc.VersionNo) != "" {
		parts = append(parts, "版本 "+strings.TrimSpace(doc.VersionNo))
	}
	if strings.TrimSpace(doc.PublishedAt) != "" {
		parts = append(parts, strings.TrimSpace(doc.PublishedAt))
	}
	if strings.TrimSpace(doc.BrandIntro) != "" {
		parts = append(parts, strings.TrimSpace(doc.BrandIntro))
	}
	return strings.Join(parts, " · ")
}

func beanListPNGColor(value string, fallback color.RGBA) color.RGBA {
	value = strings.TrimSpace(value)
	if len(value) != 7 || value[0] != '#' {
		return fallback
	}
	r, errR := strconv.ParseUint(value[1:3], 16, 8)
	g, errG := strconv.ParseUint(value[3:5], 16, 8)
	b, errB := strconv.ParseUint(value[5:7], 16, 8)
	if errR != nil || errG != nil || errB != nil {
		return fallback
	}
	return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}
}
