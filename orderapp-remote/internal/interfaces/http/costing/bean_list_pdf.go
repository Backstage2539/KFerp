package costing

import (
	"fmt"
	"strings"

	appcosting "orderapp/internal/application/costing"
	pdfinfra "orderapp/internal/infrastructure/pdf"
)

func renderBeanListPublicationPDF(row appcosting.BeanListPublication) ([]byte, error) {
	return pdfinfra.BeanListRenderer{}.Render(beanListPublicationPDFDocument(row))
}

func beanListPublicationPDFDocument(row appcosting.BeanListPublication) pdfinfra.BeanListDocument {
	cfg := row.Config
	content := row.Content
	changelog := mapString(cfg, "changelog", "")
	if strings.TrimSpace(changelog) == "" {
		changelog = row.Changelog
	}
	doc := pdfinfra.BeanListDocument{
		Title:               mapString(content, "title", buildPublicBeanListTitle(row.ListType, mapString(cfg, "brandName", "棵凡咖啡"))),
		Subtitle:            mapString(content, "subtitle", buildPublicBeanListSubtitle(row.ListType)),
		ListType:            row.ListType,
		VersionNo:           row.Version,
		PublishedAt:         row.PublishedAt,
		BrandName:           mapString(cfg, "brandName", "棵凡咖啡"),
		BrandIntro:          mapString(cfg, "brandIntro", ""),
		BackgroundColor:     mapString(cfg, "backgroundColor", "#f8f1e5"),
		FontColor:           mapString(cfg, "fontColor", "#171717"),
		LayoutStyle:         mapString(cfg, "layoutStyle", "card"),
		CardsPerRow:         int(mapNumber(cfg, "cardsPerRow", 2)),
		ShowVersion:         mapBool(cfg, "showVersion", true),
		ShowChangelog:       mapBool(cfg, "showChangelog", true),
		ShowCategoryNumbers: mapBool(cfg, "showCategoryNumbers", true),
		UsePreviewStyle:     true,
		Changelog:           changelog,
		Groups:              make([]pdfinfra.BeanListGroup, 0),
	}
	for _, groupMap := range publicMapsFromAny(content["groups"]) {
		group := pdfinfra.BeanListGroup{
			Category:     mapString(groupMap, "category", ""),
			ShowCategory: mapBool(groupMap, "showCategory", true),
			Items:        make([]pdfinfra.BeanListItem, 0),
		}
		for _, itemMap := range publicMapsFromAny(groupMap["items"]) {
			item := pdfinfra.BeanListItem{
				Code:           mapString(itemMap, "code", ""),
				Name:           mapString(itemMap, "name", ""),
				BadgeLabel:     mapString(itemMap, "badgeLabel", ""),
				RecommendedUse: mapString(itemMap, "recommendedUse", ""),
				Flavor:         mapString(itemMap, "flavor", ""),
				Description:    mapString(itemMap, "description", ""),
				AttributeLines: beanListPublicationPDFAttributeLines(itemMap),
				QualityLines:   beanListPublicationPDFQualityLines(itemMap),
				Prices:         make([]pdfinfra.BeanListPrice, 0),
			}
			if strings.TrimSpace(item.Name) == "" {
				continue
			}
			highlightTerms := publicStringList(itemMap["highlightTerms"])
			for _, priceMap := range publicMapsFromAny(itemMap["prices"]) {
				label := mapString(priceMap, "label", "")
				value := mapString(priceMap, "value", "")
				if value == "" {
					value = beanListPublicationPDFPriceDisplay(mapNumber(priceMap, "price", 0), mapString(priceMap, "unit", ""))
				}
				if strings.TrimSpace(label) == "" && strings.TrimSpace(value) == "" {
					continue
				}
				item.Prices = append(item.Prices, pdfinfra.BeanListPrice{
					Label: label,
					Value: value,
					Red:   mapBool(priceMap, "red", false) || beanListPDFContainsHighlight(label, highlightTerms) || beanListPDFContainsHighlight(value, highlightTerms),
				})
			}
			group.Items = append(group.Items, item)
		}
		if len(group.Items) > 0 {
			doc.Groups = append(doc.Groups, group)
		}
	}
	return doc
}

func beanListPublicationPDFAttributeLines(item map[string]any) []string {
	lines := publicStringList(item["attributeLines"])
	if len(lines) == 0 {
		lines = publicStringList(item["attribute_lines"])
	}
	if len(lines) > 0 {
		return lines
	}
	for _, attr := range publicMapsFromAny(item["productAttributes"]) {
		label := mapString(attr, "label", "")
		value := mapString(attr, "value", "")
		if strings.TrimSpace(label) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		lines = append(lines, strings.TrimSpace(label)+"："+strings.TrimSpace(value))
	}
	if len(lines) > 0 {
		return lines
	}
	for _, attr := range publicMapsFromAny(item["product_attributes"]) {
		label := mapString(attr, "label", "")
		value := mapString(attr, "value", "")
		if strings.TrimSpace(label) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		lines = append(lines, strings.TrimSpace(label)+"："+strings.TrimSpace(value))
	}
	return lines
}

func beanListPublicationPDFQualityLines(item map[string]any) []pdfinfra.BeanListQualityLine {
	quality := publicMapFromAny(item["beanListQuality"])
	if len(quality) == 0 {
		quality = publicMapFromAny(item["bean_list_quality"])
	}
	lines := []pdfinfra.BeanListQualityLine{}
	for _, row := range []struct {
		label string
		keys  []string
	}{
		{"工厂风味", []string{"factoryFlavorDescription", "factory_flavor_description"}},
		{"水分", []string{"moisture"}},
		{"密度", []string{"density"}},
		{"质检时间", []string{"inspectionCreatedAt", "inspection_created_at"}},
		{"质检单号", []string{"inspectionReferenceNo", "inspection_reference_no"}},
	} {
		if value := publicFirstString(quality, row.keys...); value != "" {
			lines = append(lines, pdfinfra.BeanListQualityLine{Label: row.label, Value: value})
		}
	}
	return lines
}

func beanListPublicationPDFPriceDisplay(price float64, unit string) string {
	if price <= 0 {
		return ""
	}
	value := fmt.Sprintf("%.0f", price)
	if unit = strings.TrimSpace(unit); unit != "" {
		return value + "/" + unit
	}
	return value
}

func beanListPDFContainsHighlight(text string, terms []string) bool {
	for _, term := range terms {
		if term = strings.TrimSpace(term); term != "" && strings.Contains(text, term) {
			return true
		}
	}
	return false
}
