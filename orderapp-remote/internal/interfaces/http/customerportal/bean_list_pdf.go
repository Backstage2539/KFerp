package customerportal

import (
	"fmt"
	"regexp"
	"strings"

	customerportalapp "orderapp/internal/application/customerportal"
	pdfinfra "orderapp/internal/infrastructure/pdf"
)

func beanListPDFDocument(row customerportalapp.BeanListSummary) pdfinfra.BeanListDocument {
	doc := pdfinfra.BeanListDocument{
		Title:               beanListPDFTitle(row),
		Subtitle:            row.Subtitle,
		ListType:            row.ListType,
		VersionNo:           row.VersionNo,
		PublishedAt:         row.PublishedAt,
		BrandName:           row.BrandName,
		BrandIntro:          row.BrandIntro,
		BackgroundColor:     row.BackgroundColor,
		FontColor:           row.FontColor,
		LayoutStyle:         row.LayoutStyle,
		CardsPerRow:         row.CardsPerRow,
		ShowVersion:         row.ShowVersion,
		ShowChangelog:       row.ShowChangelog,
		ShowCategoryNumbers: row.ShowCategoryNumbers,
		Changelog:           row.Changelog,
		Groups:              make([]pdfinfra.BeanListGroup, 0, len(row.Groups)),
	}
	for _, group := range row.Groups {
		nextGroup := pdfinfra.BeanListGroup{
			Category: group.Category,
			Items:    make([]pdfinfra.BeanListItem, 0, len(group.Items)),
		}
		for _, item := range group.Items {
			nextItem := pdfinfra.BeanListItem{
				Code:           item.Code,
				Name:           item.Name,
				BadgeLabel:     item.BadgeLabel,
				RecommendedUse: item.RecommendedUse,
				Flavor:         item.Flavor,
				Description:    item.Description,
				QualityLines:   beanListPDFQualityLines(item.BeanListQuality),
				Prices:         make([]pdfinfra.BeanListPrice, 0, len(item.Prices)),
			}
			for _, price := range item.Prices {
				nextItem.Prices = append(nextItem.Prices, pdfinfra.BeanListPrice{
					Label: price.Label,
					Value: price.Value,
					Red:   price.Red,
				})
			}
			nextGroup.Items = append(nextGroup.Items, nextItem)
		}
		doc.Groups = append(doc.Groups, nextGroup)
	}
	return doc
}

func beanListPDFQualityLines(quality customerportalapp.BeanListQualitySummary) []pdfinfra.BeanListQualityLine {
	lines := []pdfinfra.BeanListQualityLine{}
	for _, row := range []struct {
		label string
		value string
	}{
		{"工厂风味", quality.FactoryFlavorDescription},
		{"水分", quality.Moisture},
		{"密度", quality.Density},
		{"质检时间", quality.InspectionCreatedAt},
		{"质检单号", quality.InspectionReferenceNo},
	} {
		if value := strings.TrimSpace(row.value); value != "" {
			lines = append(lines, pdfinfra.BeanListQualityLine{Label: row.label, Value: value})
		}
	}
	return lines
}

func beanListPDFTitle(row customerportalapp.BeanListSummary) string {
	if strings.TrimSpace(row.Title) != "" {
		return strings.TrimSpace(row.Title)
	}
	switch strings.TrimSpace(row.ListType) {
	case "retail":
		return "零售豆单"
	case "commercial":
		return "商用豆单"
	case "green", "green_bean":
		return "生豆豆单"
	default:
		return "我的豆单"
	}
}

var beanListPDFFilenameUnsafeChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func beanListPDFFilename(row customerportalapp.BeanListSummary) string {
	listType := strings.TrimSpace(row.ListType)
	if listType == "" {
		listType = "bean-list"
	}
	version := strings.TrimSpace(row.VersionNo)
	if version == "" {
		version = fmt.Sprintf("%d", row.ID)
	}
	return "bean-list-" + beanListPDFFilenameUnsafeChars.ReplaceAllString(listType+"-"+version, "-") + ".pdf"
}

func beanListPNGFilename(row customerportalapp.BeanListSummary) string {
	listType := strings.TrimSpace(row.ListType)
	if listType == "" {
		listType = "bean-list"
	}
	version := strings.TrimSpace(row.VersionNo)
	if version == "" {
		version = fmt.Sprintf("%d", row.ID)
	}
	return "bean-list-" + beanListPDFFilenameUnsafeChars.ReplaceAllString(listType+"-"+version, "-") + ".png"
}
