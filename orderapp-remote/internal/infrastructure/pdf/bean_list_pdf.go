package pdf

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jung-kurt/gofpdf"
)

type BeanListDocument struct {
	Title               string
	Subtitle            string
	ListType            string
	VersionNo           string
	PublishedAt         string
	BrandName           string
	BrandIntro          string
	BackgroundColor     string
	FontColor           string
	LayoutStyle         string
	CardsPerRow         int
	ShowVersion         bool
	ShowChangelog       bool
	ShowCategoryNumbers bool
	UsePreviewStyle     bool
	DisableCompression  bool
	Changelog           string
	Groups              []BeanListGroup
}

type BeanListGroup struct {
	Category     string
	ShowCategory bool
	Items        []BeanListItem
}

type BeanListItem struct {
	Code           string
	Name           string
	BadgeLabel     string
	RecommendedUse string
	Flavor         string
	Description    string
	QualityLines   []BeanListQualityLine
	Prices         []BeanListPrice
}

type BeanListQualityLine struct {
	Label string
	Value string
}

type BeanListPrice struct {
	Label string
	Value string
	Red   bool
}

type BeanListRenderer struct {
	FontPath string
}

const (
	beanListFontRegular = ""
	beanListFontBold    = "B"

	previewVersionFontSize = 9.0
	previewTitleFontSize   = 19.5
	previewBodyFontSize    = 9.0
	previewGroupFontSize   = 11.25
	previewCodeFontSize    = 9.0
	previewNameFontSize    = 15.0
	previewPriceFontSize   = 11.25
)

func (r BeanListRenderer) Render(doc BeanListDocument) ([]byte, error) {
	fontPath, err := r.resolveFontPath()
	if err != nil {
		return nil, err
	}
	if doc.UsePreviewStyle {
		return r.renderPreviewStyle(doc, fontPath)
	}
	pdf := gofpdf.NewCustom(&gofpdf.InitType{
		OrientationStr: "P",
		UnitStr:        "mm",
		SizeStr:        "A4",
		FontDirStr:     filepath.Dir(fontPath),
	})
	pdf.SetMargins(14, 12, 14)
	pdf.SetAutoPageBreak(true, 14)
	pdf.AddUTF8Font("noto", "", filepath.Base(fontPath))
	pdf.AddPage()

	renderBeanListHeader(pdf, doc)
	for _, group := range doc.Groups {
		renderBeanListGroup(pdf, group)
	}
	if strings.TrimSpace(doc.Changelog) != "" {
		pdf.Ln(2)
		pdf.SetFont("noto", "", 9)
		pdf.MultiCell(0, 5, "更新说明："+strings.TrimSpace(doc.Changelog), "T", "L", false)
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

type beanListPreviewState struct {
	pdf      *gofpdf.Fpdf
	doc      BeanListDocument
	pageW    float64
	pageH    float64
	margin   float64
	contentW float64
	y        float64
	bg       pdfRGB
	fg       pdfRGB
}

type pdfRGB struct {
	R int
	G int
	B int
}

func (r BeanListRenderer) renderPreviewStyle(doc BeanListDocument, fontPath string) ([]byte, error) {
	pdf := gofpdf.NewCustom(&gofpdf.InitType{
		OrientationStr: "P",
		UnitStr:        "mm",
		Size:           gofpdf.SizeType{Wd: 108, Ht: 192},
		FontDirStr:     filepath.Dir(fontPath),
	})
	pdf.SetCompression(!doc.DisableCompression)
	pdf.SetMargins(0, 0, 0)
	pdf.SetAutoPageBreak(false, 0)
	pdf.AddUTF8Font("noto", "", filepath.Base(fontPath))
	pdf.AddUTF8Font("noto", "B", filepath.Base(fontPath))
	state := beanListPreviewState{
		pdf:      pdf,
		doc:      normalizeBeanListPreviewDocument(doc),
		pageW:    108,
		pageH:    192,
		margin:   8,
		contentW: 92,
		bg:       hexRGB(doc.BackgroundColor, pdfRGB{248, 241, 229}),
		fg:       hexRGB(doc.FontColor, pdfRGB{23, 23, 23}),
	}
	state.addPage()
	state.renderHeader()
	for _, group := range state.doc.Groups {
		state.renderGroup(group)
	}
	state.renderChangelogAndFooter()
	if pdf.Error() != nil {
		return nil, pdf.Error()
	}
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func normalizeBeanListPreviewDocument(doc BeanListDocument) BeanListDocument {
	doc.Title = strings.TrimSpace(doc.Title)
	if doc.Title == "" {
		doc.Title = "我的豆单"
	}
	doc.Subtitle = strings.TrimSpace(doc.Subtitle)
	doc.BrandName = strings.TrimSpace(doc.BrandName)
	if doc.BrandName == "" {
		doc.BrandName = "棵凡咖啡"
	}
	doc.LayoutStyle = strings.TrimSpace(doc.LayoutStyle)
	if doc.LayoutStyle != "table" {
		doc.LayoutStyle = "card"
	}
	if doc.CardsPerRow < 1 || doc.CardsPerRow > 4 {
		doc.CardsPerRow = 2
	}
	return doc
}

func (s *beanListPreviewState) addPage() {
	s.pdf.AddPage()
	s.setFill(s.bg)
	s.pdf.Rect(0, 0, s.pageW, s.pageH, "F")
	s.setText(s.fg)
	s.pdf.SetDrawColor(s.fg.R, s.fg.G, s.fg.B)
	s.pdf.SetFont("noto", "", 8)
	s.y = s.margin
}

func (s *beanListPreviewState) renderHeader() {
	x := s.margin
	right := s.pageW - s.margin
	if s.doc.ShowVersion && strings.TrimSpace(s.doc.VersionNo) != "" {
		s.pdf.SetFont("noto", beanListFontRegular, previewVersionFontSize)
		s.pdf.SetXY(x, s.y)
		s.pdf.CellFormat(40, 5, strings.TrimSpace(s.doc.VersionNo), "", 0, "L", false, 0, "")
		s.y += 5
	}
	s.pdf.SetFont("noto", beanListFontBold, previewTitleFontSize)
	titleLines := s.pdf.SplitText(s.doc.Title, s.contentW-21)
	if len(titleLines) == 0 {
		titleLines = []string{s.doc.Title}
	}
	for _, line := range titleLines {
		s.drawText(x, s.y, s.contentW-21, 7.2, line, "L", true)
		s.y += 7.2
	}
	if subtitle := strings.TrimSpace(s.doc.Subtitle); subtitle != "" {
		s.pdf.SetFont("noto", beanListFontRegular, previewBodyFontSize)
		s.pdf.SetXY(x, s.y+0.8)
		s.pdf.CellFormat(s.contentW-21, 4.5, subtitle, "", 0, "L", false, 0, "")
		s.y += 5.5
	}
	if intro := strings.TrimSpace(s.doc.BrandIntro); intro != "" {
		s.pdf.SetFont("noto", beanListFontRegular, 8)
		for _, line := range s.pdf.SplitText(intro, s.contentW-22) {
			s.pdf.SetXY(x, s.y)
			s.pdf.CellFormat(s.contentW-22, 4, line, "", 0, "L", false, 0, "")
			s.y += 4
		}
	}
	badge := beanListTypeLabel(s.doc.ListType)
	if badge != "" {
		badgeW := math.Max(14, s.pdf.GetStringWidth(badge)+8)
		badgeX := right - badgeW
		s.pdf.RoundedRect(badgeX, s.margin+1.5, badgeW, 8, 4, "1234", "D")
		s.pdf.SetFont("noto", beanListFontBold, previewBodyFontSize)
		s.drawText(badgeX, s.margin+3, badgeW, 5, badge, "C", true)
	}
	s.y += 3
	s.pdf.SetLineWidth(0.65)
	s.pdf.Line(x, s.y, right, s.y)
	s.pdf.SetLineWidth(0.2)
	s.y += 7
}

func (s *beanListPreviewState) renderGroup(group BeanListGroup) {
	if len(group.Items) == 0 {
		return
	}
	if s.doc.ShowCategoryNumbers && group.ShowCategory && strings.TrimSpace(group.Category) != "" {
		s.renderGroupTitle(group.Category)
	}
	if s.doc.LayoutStyle == "table" {
		s.renderTableItems(group.Items)
		return
	}
	maxCols := clampInt(s.doc.CardsPerRow, 2, 1, 4)
	for i := 0; i < len(group.Items); i += maxCols {
		end := i + maxCols
		if end > len(group.Items) {
			end = len(group.Items)
		}
		row := group.Items[i:end]
		cols := clampInt(len(row), 1, 1, maxCols)
		s.renderCardRow(row, cols)
	}
	s.y += 2
}

func (s *beanListPreviewState) renderGroupTitle(title string) {
	lines := s.splitPreviewText(title, s.contentW-8, 10)
	height := math.Max(9, float64(len(lines))*5+3)
	s.ensureSpace(height + 3)
	x := s.margin
	s.setFill(pdfRGB{255, 252, 246})
	s.pdf.Rect(x, s.y, s.contentW, height, "F")
	s.setFill(s.fg)
	s.pdf.Rect(x, s.y, 1.4, height, "F")
	s.pdf.SetFont("noto", beanListFontBold, previewGroupFontSize)
	textY := s.y + 2
	for _, line := range lines {
		s.drawText(x+4, textY, s.contentW-8, 5, line, "L", true)
		textY += 5
	}
	s.y += height + 4
}

func (s *beanListPreviewState) renderCardRow(items []BeanListItem, columns int) {
	if len(items) == 0 {
		return
	}
	gap := 2.5
	cardW := (s.contentW - gap*float64(columns-1)) / float64(columns)
	heights := make([]float64, len(items))
	rowH := 0.0
	for i, item := range items {
		heights[i] = s.estimateCardHeight(item, cardW, columns)
		rowH = math.Max(rowH, heights[i])
	}
	s.ensureSpace(rowH + 4)
	for i, item := range items {
		x := s.margin + float64(i)*(cardW+gap)
		s.renderCard(item, x, s.y, cardW, rowH, columns)
	}
	s.y += rowH + 4
}

func (s *beanListPreviewState) estimateCardHeight(item BeanListItem, width float64, rowColumns int) float64 {
	innerW := width - 6
	nameW := math.Max(12, innerW-14)
	nameLines := s.splitPreviewText(item.Name, nameW, cardNameFontSize(width))
	headH := math.Max(15.3, float64(len(nameLines))*6.2)
	height := 7 + headH
	if strings.TrimSpace(item.RecommendedUse) != "" {
		height += float64(len(s.splitPreviewText(item.RecommendedUse, innerW-14, previewBodyFontSize)))*4.6 + 2
	}
	if strings.TrimSpace(item.Flavor) != "" {
		height += float64(len(s.splitPreviewText(item.Flavor, innerW-12, previewBodyFontSize)))*4.6 + 3
	}
	if strings.TrimSpace(item.Description) != "" {
		height += float64(len(s.splitPreviewText(item.Description, innerW-12, previewBodyFontSize)))*4.6 + 3
	}
	priceCols := 1
	if rowColumns == 1 && len(item.Prices) > 1 {
		priceCols = 2
	}
	priceRows := int(math.Ceil(float64(maxInt(1, len(item.Prices))) / float64(priceCols)))
	height += 7 + float64(priceRows)*11.1 + float64(maxInt(0, priceRows-1))*1.6
	return math.Max(height+6, 46)
}

func (s *beanListPreviewState) renderCard(item BeanListItem, x, y, w, h float64, rowColumns int) {
	s.pdf.SetDrawColor(220, 211, 196)
	s.setFill(pdfRGB{255, 254, 250})
	s.pdf.RoundedRect(x, y, w, h, 2, "1234", "FD")
	innerX := x + 3
	innerY := y + 4
	innerW := w - 6
	code := strings.TrimSpace(item.Code)
	codeW := math.Max(10, s.textWidth(code, previewCodeFontSize)+4)
	s.pdf.SetDrawColor(s.fg.R, s.fg.G, s.fg.B)
	s.pdf.RoundedRect(innerX, innerY, codeW, 8, 1.6, "1234", "D")
	s.pdf.SetFont("noto", beanListFontBold, previewCodeFontSize)
	s.drawText(innerX, innerY+1.7, codeW, 4, code, "C", true)

	nameX := innerX + codeW + 3
	nameW := innerW - codeW - 3
	nameSize := cardNameFontSize(w)
	s.pdf.SetFont("noto", beanListFontBold, nameSize)
	nameLines := s.splitPreviewText(strings.TrimSpace(item.Name), nameW, nameSize)
	s.pdf.SetFont("noto", beanListFontBold, nameSize)
	textY := innerY
	for _, line := range nameLines {
		s.drawText(nameX, textY, nameW, 6.2, line, "L", true)
		textY += 6.2
	}
	if badge := strings.TrimSpace(item.BadgeLabel); badge != "" && len(nameLines) > 0 {
		s.pdf.SetFont("noto", beanListFontBold, 6.5)
		s.pdf.CellFormat(s.pdf.GetStringWidth(badge)+4, 4, badge, "1", 0, "C", false, 0, "")
	}
	bodyY := math.Max(innerY+10, textY+1)
	if use := strings.TrimSpace(item.RecommendedUse); use != "" {
		bodyY = s.renderInlineDetail(innerX, bodyY, innerW, "出品建议", use)
	}
	if flavor := strings.TrimSpace(item.Flavor); flavor != "" {
		bodyY = s.renderBlockDetail(innerX, bodyY, innerW, "风味", flavor, true)
	}
	if desc := strings.TrimSpace(item.Description); desc != "" {
		bodyY = s.renderBlockDetail(innerX, bodyY, innerW, "特点", desc, false)
	}
	priceHeight := s.estimatePriceBlockHeight(item.Prices, rowColumns)
	priceY := math.Max(bodyY+3, y+h-priceHeight-4)
	s.renderPriceBlock(item.Prices, innerX, priceY, innerW, rowColumns)
}

func (s *beanListPreviewState) renderInlineDetail(x, y, w float64, label, value string) float64 {
	s.setMutedText()
	s.pdf.SetFont("noto", beanListFontBold, previewBodyFontSize)
	s.drawText(x, y, 14, 4.5, label, "L", true)
	s.setText(s.fg)
	lines := s.splitPreviewText(value, w-15, previewBodyFontSize)
	for i, line := range lines {
		s.pdf.SetXY(x+15, y+float64(i)*4.5)
		s.pdf.CellFormat(w-15, 4.5, line, "", 0, "L", false, 0, "")
	}
	return y + float64(maxInt(1, len(lines)))*4.5 + 1
}

func (s *beanListPreviewState) renderBlockDetail(x, y, w float64, label, value string, strong bool) float64 {
	s.setMutedText()
	s.pdf.SetFont("noto", beanListFontBold, previewBodyFontSize)
	s.drawText(x, y, 10, 4.5, label, "L", true)
	s.setText(s.fg)
	size := previewBodyFontSize
	style := beanListFontRegular
	if strong {
		style = beanListFontBold
	}
	s.pdf.SetFont("noto", style, size)
	lines := s.splitPreviewText(value, w-12, size)
	s.pdf.SetFont("noto", style, size)
	for i, line := range lines {
		s.pdf.SetXY(x+12, y+float64(i)*4.5)
		s.pdf.CellFormat(w-12, 4.5, line, "", 0, "L", false, 0, "")
	}
	return y + float64(maxInt(1, len(lines)))*4.5 + 2
}

func (s *beanListPreviewState) estimatePriceBlockHeight(prices []BeanListPrice, rowColumns int) float64 {
	priceCols := 1
	if rowColumns == 1 && len(prices) > 1 {
		priceCols = 2
	}
	priceRows := int(math.Ceil(float64(maxInt(1, len(prices))) / float64(priceCols)))
	return 7 + float64(priceRows)*11.1 + float64(maxInt(0, priceRows-1))*1.6
}

func (s *beanListPreviewState) renderPriceBlock(prices []BeanListPrice, x, y, w float64, rowColumns int) {
	s.setMutedText()
	s.pdf.SetFont("noto", beanListFontBold, previewBodyFontSize)
	s.drawText(x, y, w, 4.5, "报价", "L", true)
	s.setText(s.fg)
	priceCols := 1
	if rowColumns == 1 && len(prices) > 1 {
		priceCols = 2
	}
	gap := 2.2
	boxW := (w - gap*float64(priceCols-1)) / float64(priceCols)
	for i, price := range prices {
		col := i % priceCols
		row := i / priceCols
		px := x + float64(col)*(boxW+gap)
		py := y + 6 + float64(row)*14
		fill := pdfRGB{223, 245, 217}
		if i%2 == 1 {
			fill = pdfRGB{219, 234, 247}
		}
		s.pdf.SetDrawColor(198, 220, 192)
		s.setFill(fill)
		s.pdf.RoundedRect(px, py, boxW, 12, 1.6, "1234", "FD")
		if price.Red {
			s.pdf.SetTextColor(197, 22, 22)
		} else {
			s.setText(s.fg)
		}
		s.pdf.SetFont("noto", beanListFontRegular, previewBodyFontSize)
		s.pdf.SetXY(px+2, py+4)
		s.pdf.CellFormat(boxW-18, 4, strings.TrimSpace(price.Label), "", 0, "L", false, 0, "")
		s.pdf.SetFont("noto", beanListFontBold, previewPriceFontSize)
		s.drawText(px+boxW-22, py+3.4, 20, 4.5, strings.TrimSpace(price.Value), "R", true)
		s.setText(s.fg)
	}
}

func (s *beanListPreviewState) drawText(x, y, w, h float64, text, align string, strong bool) {
	s.pdf.SetXY(x, y)
	s.pdf.CellFormat(w, h, text, "", 0, align, false, 0, "")
	if !strong {
		return
	}
	for _, dx := range []float64{0.07, 0.14} {
		s.pdf.SetXY(x+dx, y)
		s.pdf.CellFormat(w, h, text, "", 0, align, false, 0, "")
	}
}

func (s *beanListPreviewState) renderTableItems(items []BeanListItem) {
	for _, item := range items {
		height := math.Max(20, 10+float64(len(item.Prices))*5)
		s.ensureSpace(height + 2)
		x := s.margin
		s.pdf.SetDrawColor(220, 211, 196)
		s.setFill(pdfRGB{255, 254, 250})
		s.pdf.Rect(x, s.y, s.contentW, height, "FD")
		s.pdf.Line(x+13, s.y, x+13, s.y+height)
		s.pdf.Line(x+s.contentW-27, s.y, x+s.contentW-27, s.y+height)
		s.pdf.SetFont("noto", "", 8)
		s.pdf.SetXY(x, s.y+3)
		s.pdf.CellFormat(13, 5, item.Code, "", 0, "C", false, 0, "")
		s.pdf.SetXY(x+15, s.y+3)
		s.pdf.CellFormat(s.contentW-44, 5, item.Name, "", 0, "L", false, 0, "")
		lineY := s.y + 9
		for _, value := range nonEmptyStrings(item.RecommendedUse, item.Flavor, item.Description) {
			for _, line := range s.splitPreviewText(value, s.contentW-45, 6.5) {
				s.pdf.SetXY(x+15, lineY)
				s.pdf.CellFormat(s.contentW-45, 4, line, "", 0, "L", false, 0, "")
				lineY += 4
			}
		}
		priceY := s.y + 3
		for _, price := range item.Prices {
			s.pdf.SetXY(x+s.contentW-25, priceY)
			s.pdf.SetFont("noto", "", 6.5)
			s.pdf.CellFormat(11, 4, price.Label, "", 0, "L", false, 0, "")
			s.pdf.SetFont("noto", "", 7)
			s.pdf.CellFormat(12, 4, price.Value, "", 0, "R", false, 0, "")
			priceY += 5
		}
		s.y += height + 2
	}
}

func (s *beanListPreviewState) renderChangelogAndFooter() {
	if s.doc.ShowChangelog && strings.TrimSpace(s.doc.Changelog) != "" {
		lines := s.splitPreviewText("更新 "+strings.TrimSpace(s.doc.Changelog), s.contentW, 7)
		height := float64(len(lines))*4.5 + 4
		s.ensureSpace(height + 2)
		s.pdf.SetDrawColor(220, 211, 196)
		s.setFill(pdfRGB{255, 252, 246})
		s.pdf.RoundedRect(s.margin, s.y, s.contentW, height, 2, "1234", "FD")
		s.pdf.SetFont("noto", "", 7)
		textY := s.y + 2
		for _, line := range lines {
			s.pdf.SetXY(s.margin+2, textY)
			s.pdf.CellFormat(s.contentW-4, 4.5, line, "", 0, "L", false, 0, "")
			textY += 4.5
		}
		s.y += height + 3
	}
	footer := strings.TrimSpace(s.doc.BrandName)
	if footer == "" {
		footer = "棵凡咖啡"
	}
	right := strings.TrimSpace(s.doc.VersionNo)
	s.ensureSpace(9)
	s.pdf.Line(s.margin, s.y, s.pageW-s.margin, s.y)
	s.pdf.SetFont("noto", "", 6.5)
	s.pdf.SetXY(s.margin, s.y+2)
	s.pdf.CellFormat(s.contentW/2, 4, footer, "", 0, "L", false, 0, "")
	s.pdf.SetXY(s.margin+s.contentW/2, s.y+2)
	s.pdf.CellFormat(s.contentW/2, 4, right, "", 0, "R", false, 0, "")
}

func (s *beanListPreviewState) ensureSpace(height float64) {
	if s.y+height <= s.pageH-s.margin {
		return
	}
	s.addPage()
}

func (s *beanListPreviewState) splitPreviewText(text string, width float64, fontSize float64) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	s.pdf.SetFont("noto", "", fontSize)
	lines := s.pdf.SplitText(text, width)
	if len(lines) == 0 {
		return []string{text}
	}
	return lines
}

func (s *beanListPreviewState) textWidth(text string, fontSize float64) float64 {
	s.pdf.SetFont("noto", "", fontSize)
	return s.pdf.GetStringWidth(text)
}

func (s *beanListPreviewState) setFill(color pdfRGB) {
	s.pdf.SetFillColor(color.R, color.G, color.B)
}

func (s *beanListPreviewState) setText(color pdfRGB) {
	s.pdf.SetTextColor(color.R, color.G, color.B)
}

func (s *beanListPreviewState) setMutedText() {
	s.pdf.SetTextColor(112, 112, 112)
}

func renderBeanListHeader(pdf *gofpdf.Fpdf, doc BeanListDocument) {
	title := strings.TrimSpace(doc.Title)
	if title == "" {
		title = "我的豆单"
	}
	pdf.SetFont("noto", "", 18)
	pdf.CellFormat(0, 10, title, "", 1, "L", false, 0, "")
	pdf.SetFont("noto", "", 9)
	meta := strings.Join(nonEmptyStrings(doc.VersionNo, doc.PublishedAt, beanListTypeLabel(doc.ListType)), " / ")
	if meta != "" {
		pdf.CellFormat(0, 6, meta, "", 1, "L", false, 0, "")
	}
	y := pdf.GetY() + 2
	left, _, right, _ := pdf.GetMargins()
	pageW, _ := pdf.GetPageSize()
	pdf.Line(left, y, pageW-right, y)
	pdf.SetY(y + 4)
}

func renderBeanListGroup(pdf *gofpdf.Fpdf, group BeanListGroup) {
	if len(group.Items) == 0 {
		return
	}
	category := strings.TrimSpace(group.Category)
	if category != "" {
		pdf.SetFont("noto", "", 12)
		pdf.SetFillColor(240, 240, 240)
		pdf.CellFormat(0, 8, category, "", 1, "L", true, 0, "")
		pdf.Ln(1)
	}
	for _, item := range group.Items {
		renderBeanListItem(pdf, item)
	}
	pdf.Ln(2)
}

func renderBeanListItem(pdf *gofpdf.Fpdf, item BeanListItem) {
	name := strings.TrimSpace(item.Name)
	if name == "" {
		return
	}
	code := strings.TrimSpace(item.Code)
	if code != "" {
		name = code + " " + name
	}
	if badge := strings.TrimSpace(item.BadgeLabel); badge != "" {
		name += " [" + badge + "]"
	}
	pdf.SetFont("noto", "", 11)
	pdf.MultiCell(0, 6, name, "", "L", false)
	pdf.SetFont("noto", "", 9)
	for _, line := range []struct {
		label string
		value string
	}{
		{"出品建议", item.RecommendedUse},
		{"风味", item.Flavor},
		{"特点", item.Description},
	} {
		if value := strings.TrimSpace(line.value); value != "" {
			pdf.MultiCell(0, 5, line.label+"："+value, "", "L", false)
		}
	}
	for _, line := range item.QualityLines {
		label := strings.TrimSpace(line.Label)
		value := strings.TrimSpace(line.Value)
		if label == "" || value == "" {
			continue
		}
		pdf.MultiCell(0, 5, label+"："+value, "", "L", false)
	}
	for _, price := range item.Prices {
		label := strings.TrimSpace(price.Label)
		value := strings.TrimSpace(price.Value)
		if label == "" && value == "" {
			continue
		}
		if price.Red {
			pdf.SetTextColor(190, 0, 0)
		}
		pdf.CellFormat(0, 5, strings.TrimSpace(label+" "+value), "", 1, "L", false, 0, "")
		if price.Red {
			pdf.SetTextColor(0, 0, 0)
		}
	}
	pdf.Ln(2)
}

func (r BeanListRenderer) resolveFontPath() (string, error) {
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
	return "", fmt.Errorf("bean list PDF font not found")
}

func beanListTypeLabel(listType string) string {
	switch strings.TrimSpace(listType) {
	case "commercial":
		return "商用"
	case "drip":
		return "挂耳"
	case "retail":
		return "零售"
	case "green", "green_bean":
		return "生豆"
	default:
		return ""
	}
}

func nonEmptyStrings(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func hexRGB(value string, fallback pdfRGB) pdfRGB {
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
	return pdfRGB{R: int(r), G: int(g), B: int(b)}
}

func clampInt(value, fallback, minValue, maxValue int) int {
	if value <= 0 {
		value = fallback
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func cardNameFontSize(width float64) float64 {
	if width >= 42 {
		return previewNameFontSize
	}
	return 13
}
