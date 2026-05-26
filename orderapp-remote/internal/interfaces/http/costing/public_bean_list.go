package costing

import (
	"bytes"
	"fmt"
	"html"
	"html/template"
	"math"
	"strings"

	appcosting "orderapp/internal/application/costing"
)

type publicBeanListView struct {
	HTMLTitle           string
	Title               string
	Subtitle            string
	Version             string
	ListTypeLabel       string
	BrandName           string
	BrandIntro          string
	Changelog           string
	ShowVersion         bool
	ShowChangelog       bool
	ShowCategoryNumbers bool
	LayoutStyle         string
	PageStyle           template.CSS
	LogoImage           template.URL
	HasLogo             bool
	Groups              []publicBeanListGroup
}

type publicBeanListGroup struct {
	Category     string
	ShowCategory bool
	Rows         [][]publicBeanListItem
	Items        []publicBeanListItem
}

type publicBeanListItem struct {
	Code               string
	NameHTML           template.HTML
	BadgeLabel         string
	BadgeClass         string
	RecommendedUseHTML template.HTML
	FlavorHTML         template.HTML
	DescriptionHTML    template.HTML
	AttributeHTML      template.HTML
	QualityHTML        template.HTML
	Prices             []publicBeanListPrice
}

type publicBeanListPrice struct {
	LabelHTML template.HTML
	ValueHTML template.HTML
	Red       bool
}

var publicBeanListTemplate = template.Must(template.New("public-bean-list").Funcs(template.FuncMap{
	"rowGrid": func(row []publicBeanListItem) template.CSS {
		cols := len(row)
		if cols < 1 {
			cols = 1
		}
		return template.CSS(fmt.Sprintf("grid-template-columns:repeat(%d,minmax(0,1fr));", cols))
	},
}).Parse(`<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.HTMLTitle}}</title>
  <style>
    * { box-sizing: border-box; }
    body { margin: 0; background: #f4f4f4; color: #171717; font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
    .public-toolbar { position: sticky; top: 0; z-index: 5; display: flex; justify-content: center; gap: 10px; padding: 10px; background: rgba(255,255,255,.92); border-bottom: 1px solid #dedede; backdrop-filter: blur(8px); }
    .public-toolbar button { min-height: 36px; border: 1px solid #111; background: #fff; border-radius: 8px; padding: 0 14px; font: inherit; font-weight: 700; cursor: pointer; }
    .bean-list-public-shell { width: 100%; padding: 18px 10px 36px; }
    .bean-list-pdf-surface { width: min(760px, 100%); min-height: 100vh; margin: 0 auto; padding: 24px; background-size: cover; background-position: center; box-shadow: 0 16px 50px rgba(0,0,0,.12); }
    .pdf-cover { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; border-bottom: 4px solid currentColor; padding-bottom: 16px; margin-bottom: 24px; }
    .pdf-logo { display: block; max-width: 72px; max-height: 72px; object-fit: contain; margin-bottom: 10px; }
    .pdf-version { margin: 0 0 4px; color: #666; font-size: 14px; }
    h1 { margin: 0; font-size: 34px; line-height: 1.12; letter-spacing: 0; }
    .pdf-subtitle, .pdf-brand-intro { margin: 8px 0 0; line-height: 1.5; }
    .pdf-badge { flex: 0 0 auto; border: 2px solid currentColor; border-radius: 999px; padding: 5px 13px; font-weight: 800; }
    .pdf-group { margin: 0 0 28px; }
    .pdf-group h2 { margin: 0 0 14px; padding: 8px 0 8px 12px; border-left: 6px solid currentColor; font-size: 24px; line-height: 1.25; }
    .pdf-card-row { display: grid; gap: 18px; align-items: stretch; margin-bottom: 18px; }
    .pdf-item { display: flex; min-width: 0; min-height: 100%; flex-direction: column; gap: 11px; border: 1px solid rgba(0,0,0,.18); border-radius: 10px; background: rgba(255,255,255,.86); padding: 12px 12px 18px; }
    .pdf-item-head { display: grid; grid-template-columns: auto minmax(0,1fr); align-items: center; gap: 10px; }
    .pdf-code { border: 1px solid currentColor; border-radius: 8px; padding: 5px 9px; font-weight: 800; font-size: 18px; line-height: 1.1; white-space: nowrap; }
    .pdf-name { min-width: 0; font-size: 25px; font-weight: 900; line-height: 1.15; overflow-wrap: anywhere; }
    .pdf-tag { display: inline-block; margin-left: 6px; border: 1px solid currentColor; border-radius: 6px; padding: 1px 5px; font-size: 12px; vertical-align: middle; }
    .pdf-tag-new { color: #d81717; }
    .pdf-tag-thumb, .pdf-tag-medal { color: #7a4d00; }
    .pdf-detail { display: grid; grid-template-columns: 64px minmax(0,1fr); align-items: start; gap: 8px; line-height: 1.55; font-weight: 700; }
    .pdf-detail dt { margin: 0; color: #777; font-weight: 900; }
    .pdf-detail dd { margin: 0; min-width: 0; overflow-wrap: anywhere; }
    .pdf-price-block { margin-top: auto; padding-top: 12px; }
    .pdf-section-label { margin: 0 0 8px; color: #777; font-size: 18px; font-weight: 900; }
    .pdf-price-list { display: grid; gap: 10px; }
    .pdf-price { display: grid; grid-template-columns: minmax(0,1fr) auto; align-items: center; gap: 8px; min-height: 52px; border: 1px solid rgba(55,128,55,.18); border-radius: 8px; background: #dff5d8; padding: 10px; }
    .pdf-price:nth-child(even) { background: #d9ebf8; border-color: rgba(46,93,125,.18); }
    .pdf-price-label { min-width: 0; overflow-wrap: anywhere; font-weight: 800; }
    .pdf-price-value { justify-self: end; white-space: nowrap; font-size: 22px; line-height: 1; font-weight: 950; }
    .pdf-compact-table { width: 100%; border-collapse: collapse; background: rgba(255,255,255,.84); }
    .pdf-compact-table td { border: 1px solid rgba(0,0,0,.22); padding: 8px; vertical-align: top; }
    .pdf-code-cell { width: 62px; text-align: center; font-weight: 900; }
    .pdf-table-name { font-weight: 900; }
    .pdf-table-line { margin-top: 4px; color: #444; font-size: 14px; line-height: 1.45; }
    .pdf-table-prices { width: 190px; }
    .pdf-table-prices div { display: grid; grid-template-columns: minmax(0,1fr) auto; gap: 8px; margin-bottom: 4px; }
    .pdf-table-prices strong { white-space: nowrap; }
    .pdf-red { color: #d81717 !important; font-weight: 950; }
    .pdf-changelog { margin-top: 28px; border-top: 2px solid rgba(0,0,0,.18); padding-top: 12px; line-height: 1.6; }
    .pdf-footer { display: flex; justify-content: space-between; gap: 12px; margin-top: 18px; color: #666; font-size: 13px; }
    .empty-page { width: min(620px, 100%); margin: 44px auto; border: 1px solid #ddd; border-radius: 10px; background: #fff; padding: 26px; }
    @media (max-width: 640px) {
      .bean-list-public-shell { padding: 0; }
      .bean-list-pdf-surface { width: 100%; box-shadow: none; padding: 18px 14px 28px; }
      h1 { font-size: 28px; }
      .pdf-cover { margin-bottom: 18px; }
      .pdf-card-row { grid-template-columns: 1fr !important; }
      .pdf-name { font-size: 24px; }
      .pdf-table-prices { width: 150px; }
    }
    @media print {
      body { background: #fff; }
      .public-toolbar { display: none !important; }
      .bean-list-public-shell { padding: 0; }
      .bean-list-pdf-surface { width: 100%; min-height: auto; box-shadow: none; margin: 0; }
      .pdf-item { break-inside: avoid; page-break-inside: avoid; }
    }
  </style>
</head>
<body>
  <div class="public-toolbar">
    <button type="button" onclick="window.print()">打印 / 保存 PDF</button>
  </div>
  <main class="bean-list-public-shell">
    <section class="bean-list-pdf-surface" style="{{.PageStyle}}">
      <header class="pdf-cover">
        <div>
          {{if .HasLogo}}<img class="pdf-logo" src="{{.LogoImage}}" alt="logo">{{end}}
          {{if .ShowVersion}}<p class="pdf-version">{{.Version}}</p>{{end}}
          <h1>{{.Title}}</h1>
          <p class="pdf-subtitle">{{.Subtitle}}</p>
          {{if .BrandIntro}}<p class="pdf-brand-intro">{{.BrandIntro}}</p>{{end}}
        </div>
        <div class="pdf-badge">{{.ListTypeLabel}}</div>
      </header>

      {{range .Groups}}
        <section class="pdf-group">
          {{if and .ShowCategory $.ShowCategoryNumbers}}<h2>{{.Category}}</h2>{{end}}
          {{if eq $.LayoutStyle "table"}}
            <table class="pdf-compact-table">
              <tbody>
                {{range .Items}}
                  <tr>
                    <td class="pdf-code-cell">{{.Code}}</td>
                    <td>
                      <div class="pdf-table-name">{{.NameHTML}}{{if .BadgeLabel}} <span class="pdf-tag {{.BadgeClass}}">{{.BadgeLabel}}</span>{{end}}</div>
                      {{if .RecommendedUseHTML}}<div class="pdf-table-line">出品建议 {{.RecommendedUseHTML}}</div>{{end}}
                      {{if .FlavorHTML}}<div class="pdf-table-line">风味 {{.FlavorHTML}}</div>{{end}}
                      {{if .DescriptionHTML}}<div class="pdf-table-line">特点 {{.DescriptionHTML}}</div>{{end}}
                      {{if .AttributeHTML}}<div class="pdf-table-line">属性 {{.AttributeHTML}}</div>{{end}}
                      {{if .QualityHTML}}<div class="pdf-table-line">质检 {{.QualityHTML}}</div>{{end}}
                    </td>
                    <td class="pdf-table-prices">
                      {{range .Prices}}<div><span class="{{if .Red}}pdf-red{{end}}">{{.LabelHTML}}</span><strong class="{{if .Red}}pdf-red{{end}}">{{.ValueHTML}}</strong></div>{{end}}
                    </td>
                  </tr>
                {{end}}
              </tbody>
            </table>
          {{else}}
            {{range .Rows}}
              <div class="pdf-card-row" style="{{rowGrid .}}">
                {{range .}}
                  <article class="pdf-item">
                    <div class="pdf-item-head">
                      <span class="pdf-code">{{.Code}}</span>
                      <div class="pdf-name">{{.NameHTML}}{{if .BadgeLabel}} <span class="pdf-tag {{.BadgeClass}}">{{.BadgeLabel}}</span>{{end}}</div>
                    </div>
                    {{if .RecommendedUseHTML}}<dl class="pdf-detail"><dt>出品建议</dt><dd>{{.RecommendedUseHTML}}</dd></dl>{{end}}
                    {{if .FlavorHTML}}<dl class="pdf-detail"><dt>风味</dt><dd>{{.FlavorHTML}}</dd></dl>{{end}}
                    {{if .DescriptionHTML}}<dl class="pdf-detail"><dt>特点</dt><dd>{{.DescriptionHTML}}</dd></dl>{{end}}
                    {{if .AttributeHTML}}<dl class="pdf-detail"><dt>属性</dt><dd>{{.AttributeHTML}}</dd></dl>{{end}}
                    {{if .QualityHTML}}<dl class="pdf-detail"><dt>质检</dt><dd>{{.QualityHTML}}</dd></dl>{{end}}
                    <div class="pdf-price-block">
                      <div class="pdf-section-label">报价</div>
                      <div class="pdf-price-list">
                        {{range .Prices}}<div class="pdf-price"><span class="pdf-price-label {{if .Red}}pdf-red{{end}}">{{.LabelHTML}}</span><strong class="pdf-price-value {{if .Red}}pdf-red{{end}}">{{.ValueHTML}}</strong></div>{{end}}
                      </div>
                    </div>
                  </article>
                {{end}}
              </div>
            {{end}}
          {{end}}
        </section>
      {{end}}

      {{if and .ShowChangelog .Changelog}}<div class="pdf-changelog"><b>更新</b> {{.Changelog}}</div>{{end}}
      <footer class="pdf-footer"><span>{{.BrandName}}</span><span>{{.Version}}</span></footer>
    </section>
  </main>
</body>
</html>`))

func renderPublicBeanListPage(row appcosting.BeanListPublication) (string, error) {
	view := publicBeanListViewFromPublication(row)
	var buf bytes.Buffer
	if err := publicBeanListTemplate.Execute(&buf, view); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func renderNoPublishedBeanListPage(listType string) string {
	label := "商用"
	switch normalizePublicBeanListType(listType) {
	case "retail":
		label = "零售"
	case "green":
		label = "生豆"
	}
	return fmt.Sprintf(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>暂无已发布%s豆单</title><style>body{margin:0;background:#f5f5f5;font-family:system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;color:#171717}.empty-page{width:min(620px,100%%);margin:44px auto;border:1px solid #ddd;border-radius:10px;background:#fff;padding:26px;box-sizing:border-box}h1{margin:0 0 8px;font-size:28px}p{margin:0;color:#666}</style></head><body><main class="empty-page"><h1>暂无已发布%s豆单</h1><p>请先在 ERP 生成豆单页面发布后，再把链接发给客户。</p></main></body></html>`, label, label)
}

func publicBeanListViewFromPublication(row appcosting.BeanListPublication) publicBeanListView {
	cfg := row.Config
	content := row.Content
	brandName := mapString(cfg, "brandName", "棵凡咖啡")
	title := mapString(content, "title", buildPublicBeanListTitle(row.ListType, brandName))
	subtitle := mapString(content, "subtitle", buildPublicBeanListSubtitle(row.ListType))
	changelog := mapString(cfg, "changelog", "")
	if strings.TrimSpace(changelog) == "" {
		changelog = row.Changelog
	}
	logo, hasLogo := safePublicImageURL(mapString(cfg, "logoImage", ""))
	view := publicBeanListView{
		HTMLTitle:           strings.TrimSpace(title + " " + row.Version),
		Title:               title,
		Subtitle:            subtitle,
		Version:             row.Version,
		ListTypeLabel:       publicBeanListTypeLabel(row.ListType),
		BrandName:           brandName,
		BrandIntro:          mapString(cfg, "brandIntro", ""),
		Changelog:           changelog,
		ShowVersion:         mapBool(cfg, "showVersion", true),
		ShowChangelog:       mapBool(cfg, "showChangelog", true),
		ShowCategoryNumbers: mapBool(cfg, "showCategoryNumbers", true),
		LayoutStyle:         publicLayoutStyle(mapString(cfg, "layoutStyle", "card")),
		PageStyle:           publicPageStyle(cfg),
		LogoImage:           template.URL(logo),
		HasLogo:             hasLogo,
	}
	cardsPerRow := clampPublicInt(mapNumber(cfg, "cardsPerRow", 2), 2, 1, 4)
	for _, groupMap := range publicMapsFromAny(content["groups"]) {
		group := publicBeanListGroup{
			Category:     mapString(groupMap, "category", ""),
			ShowCategory: mapBool(groupMap, "showCategory", true),
			Items:        make([]publicBeanListItem, 0),
		}
		for _, itemMap := range publicMapsFromAny(groupMap["items"]) {
			item := publicItemFromMap(itemMap)
			group.Items = append(group.Items, item)
		}
		group.Rows = chunkPublicItems(group.Items, cardsPerRow)
		view.Groups = append(view.Groups, group)
	}
	return view
}

func publicItemFromMap(item map[string]any) publicBeanListItem {
	terms := publicStringList(item["highlightTerms"])
	out := publicBeanListItem{
		Code:               mapString(item, "code", ""),
		NameHTML:           publicHighlightHTML(mapString(item, "name", ""), terms),
		BadgeLabel:         mapString(item, "badgeLabel", ""),
		BadgeClass:         publicBadgeClass(mapString(item, "badge", "")),
		RecommendedUseHTML: publicHighlightHTML(mapString(item, "recommendedUse", ""), terms),
		FlavorHTML:         publicHighlightHTML(mapString(item, "flavor", ""), terms),
		DescriptionHTML:    publicHighlightHTML(mapString(item, "description", ""), terms),
		AttributeHTML:      publicHighlightHTML(strings.Join(beanListPublicationPDFAttributeLines(item), " / "), terms),
		QualityHTML:        publicQualityHTML(item, terms),
	}
	for _, priceMap := range publicMapsFromAny(item["prices"]) {
		label := mapString(priceMap, "label", "")
		value := publicPriceDisplay(mapNumber(priceMap, "price", 0), mapString(priceMap, "unit", ""))
		labelHTML := publicHighlightHTML(label, terms)
		valueHTML := publicHighlightHTML(value, terms)
		out.Prices = append(out.Prices, publicBeanListPrice{
			LabelHTML: labelHTML,
			ValueHTML: valueHTML,
			Red:       mapBool(priceMap, "red", false) || strings.Contains(string(labelHTML), `class="pdf-red"`) || strings.Contains(string(valueHTML), `class="pdf-red"`),
		})
	}
	return out
}

func chunkPublicItems(items []publicBeanListItem, size int) [][]publicBeanListItem {
	if size <= 0 {
		size = 1
	}
	rows := make([][]publicBeanListItem, 0, (len(items)+size-1)/size)
	for i := 0; i < len(items); i += size {
		end := i + size
		if end > len(items) {
			end = len(items)
		}
		rows = append(rows, items[i:end])
	}
	return rows
}

func buildPublicBeanListTitle(listType, brandName string) string {
	brand := strings.TrimSpace(brandName)
	if brand == "" {
		brand = "棵凡咖啡"
	}
	switch normalizePublicBeanListType(listType) {
	case "retail":
		return brand + "零售产品价格表"
	case "green":
		return brand + "生豆产品价格表"
	}
	return brand + "批发产品价格表"
}

func buildPublicBeanListSubtitle(listType string) string {
	switch normalizePublicBeanListType(listType) {
	case "retail":
		return "报价含税运"
	case "green":
		return "生豆销售报价"
	}
	return "报价不含税、不含运"
}

func publicBeanListTypeLabel(listType string) string {
	switch normalizePublicBeanListType(listType) {
	case "retail":
		return "零售"
	case "green":
		return "生豆"
	}
	return "商用"
}

func normalizePublicBeanListType(listType string) string {
	switch strings.TrimSpace(listType) {
	case "retail":
		return "retail"
	case "green", "green_bean":
		return "green"
	default:
		return "commercial"
	}
}

func publicLayoutStyle(value string) string {
	if strings.TrimSpace(value) == "table" {
		return "table"
	}
	return "card"
}

func publicPageStyle(cfg map[string]any) template.CSS {
	color := publicHexColor(mapString(cfg, "fontColor", "#171717"), "#171717")
	bg := publicHexColor(mapString(cfg, "backgroundColor", "#f8f1e5"), "#f8f1e5")
	style := fmt.Sprintf("color:%s;background-color:%s;", color, bg)
	if img, ok := safePublicImageURL(mapString(cfg, "backgroundImage", "")); ok {
		style += `background-image:url("` + cssEscapePublicURL(img) + `");`
	}
	return template.CSS(style)
}

func publicPriceDisplay(price float64, unit string) string {
	value := fmt.Sprintf("%.0f", math.Round(price))
	unit = strings.TrimSpace(unit)
	if unit == "" {
		return value
	}
	return value + "/" + unit
}

func publicHighlightHTML(text string, terms []string) template.HTML {
	source := text
	needles := publicSortTerms(terms)
	if source == "" || len(needles) == 0 {
		return template.HTML(html.EscapeString(source))
	}
	var out strings.Builder
	index := 0
	for index < len(source) {
		match := ""
		matchIndex := len(source)
		for _, term := range needles {
			if i := strings.Index(source[index:], term); i >= 0 {
				absolute := index + i
				if absolute < matchIndex {
					matchIndex = absolute
					match = term
				}
			}
		}
		if match == "" {
			out.WriteString(html.EscapeString(source[index:]))
			break
		}
		if matchIndex > index {
			out.WriteString(html.EscapeString(source[index:matchIndex]))
		}
		out.WriteString(`<span class="pdf-red">`)
		out.WriteString(html.EscapeString(match))
		out.WriteString(`</span>`)
		index = matchIndex + len(match)
	}
	return template.HTML(out.String())
}

func publicQualityHTML(item map[string]any, terms []string) template.HTML {
	quality := publicMapFromAny(item["beanListQuality"])
	if len(quality) == 0 {
		quality = publicMapFromAny(item["bean_list_quality"])
	}
	lines := []string{}
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
			lines = append(lines, row.label+" "+value)
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return publicHighlightHTML(strings.Join(lines, " / "), terms)
}

func publicMapFromAny(value any) map[string]any {
	if value == nil {
		return nil
	}
	if m, ok := value.(map[string]any); ok {
		return m
	}
	return nil
}

func publicFirstString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := mapString(m, key, ""); value != "" {
			return value
		}
	}
	return ""
}

func publicSortTerms(terms []string) []string {
	out := make([]string, 0, len(terms))
	for _, term := range terms {
		term = strings.TrimSpace(term)
		if term != "" {
			out = append(out, term)
		}
	}
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if len(out[j]) > len(out[i]) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func publicMapsFromAny(value any) []map[string]any {
	switch rows := value.(type) {
	case []any:
		out := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			if m, ok := row.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	case []map[string]any:
		return rows
	default:
		return nil
	}
}

func publicStringList(value any) []string {
	switch rows := value.(type) {
	case []string:
		return rows
	case []any:
		out := make([]string, 0, len(rows))
		for _, row := range rows {
			if s := strings.TrimSpace(fmt.Sprint(row)); s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		raw := strings.FieldsFunc(rows, func(r rune) bool { return r == ',' || r == '，' || r == '\n' })
		out := make([]string, 0, len(raw))
		for _, row := range raw {
			if s := strings.TrimSpace(row); s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func mapString(m map[string]any, key, fallback string) string {
	if m == nil {
		return fallback
	}
	if s := strings.TrimSpace(fmt.Sprint(m[key])); s != "" && s != "<nil>" {
		return s
	}
	return fallback
}

func mapBool(m map[string]any, key string, fallback bool) bool {
	if m == nil {
		return fallback
	}
	switch v := m[key].(type) {
	case bool:
		return v
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "1", "yes":
			return true
		case "false", "0", "no":
			return false
		}
	}
	return fallback
}

func mapNumber(m map[string]any, key string, fallback float64) float64 {
	if m == nil {
		return fallback
	}
	switch v := m[key].(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case interface{ Float64() (float64, error) }:
		parsed, err := v.Float64()
		if err == nil {
			return parsed
		}
	case string:
		var parsed float64
		if _, err := fmt.Sscanf(strings.TrimSpace(v), "%f", &parsed); err == nil {
			return parsed
		}
	}
	return fallback
}

func clampPublicInt(value float64, fallback, min, max int) int {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return fallback
	}
	n := int(value)
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}

func publicHexColor(value, fallback string) string {
	value = strings.TrimSpace(value)
	if len(value) != 7 || value[0] != '#' {
		return fallback
	}
	for _, r := range value[1:] {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return fallback
		}
	}
	return value
}

func safePublicImageURL(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "data:image/") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") || strings.HasPrefix(value, "/") {
		return value, true
	}
	return "", false
}

func cssEscapePublicURL(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `%22`, "\n", "", "\r", "")
	return replacer.Replace(value)
}

func publicBadgeClass(badge string) string {
	switch strings.TrimSpace(badge) {
	case "new":
		return "pdf-tag-new"
	case "thumb":
		return "pdf-tag-thumb"
	case "medal":
		return "pdf-tag-medal"
	default:
		return ""
	}
}
