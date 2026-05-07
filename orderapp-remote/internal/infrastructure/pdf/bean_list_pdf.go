package pdf

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jung-kurt/gofpdf"
)

type BeanListDocument struct {
	Title       string
	ListType    string
	VersionNo   string
	PublishedAt string
	Changelog   string
	Groups      []BeanListGroup
}

type BeanListGroup struct {
	Category string
	Items    []BeanListItem
}

type BeanListItem struct {
	Code           string
	Name           string
	BadgeLabel     string
	RecommendedUse string
	Flavor         string
	Description    string
	Prices         []BeanListPrice
}

type BeanListPrice struct {
	Label string
	Value string
	Red   bool
}

type BeanListRenderer struct {
	FontPath string
}

func (r BeanListRenderer) Render(doc BeanListDocument) ([]byte, error) {
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
	case "retail":
		return "零售"
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
