package appmain

import (
	"encoding/json"
	"fmt"
	"html/template"
	"regexp"
	"strings"
)

func templateFuncMap() template.FuncMap {
	return template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int {
			if a-b < 0 {
				return 0
			}
			return a - b
		},
		"f0p": func(p *float64) string {
			if p == nil {
				return ""
			}
			return fmt.Sprintf("%.0f", *p)
		},
		"f2p": func(p *float64) string {
			if p == nil {
				return ""
			}
			return fmt.Sprintf("%.2f", *p)
		},
		"py":  func(s string) string { return pinyinFull(s) },
		"pyi": func(s string) string { return pinyinInitials(s) },
		"jsstr": func(s string) template.JS {
			b, _ := json.Marshal(s)
			return template.JS(b)
		},
		"assetLabel":          func(kind string) string { return kindLabel(kind) },
		"bomURL":              bomVueURL,
		"custShort":           customerShortLabel,
		"eq64":                func(a, b int64) bool { return a == b },
		"eqi":                 func(a, b int) bool { return a == b },
		"materialSummaryText": materialSummaryText,
		"pct":                 func(v float64) string { return fmt.Sprintf("%.2f%%", v*100) },
		"retailLines":         retailPriceLines,
	}
}

func retailPriceLines(price100G, price200G, price227G, price250G float64) []string {
	lines := make([]string, 0, 4)
	for _, item := range []struct {
		specG int64
		price float64
	}{
		{specG: 100, price: price100G},
		{specG: 200, price: price200G},
		{specG: 227, price: price227G},
		{specG: 250, price: price250G},
	} {
		if item.price <= 0 {
			continue
		}
		lines = append(lines, fmt.Sprintf("%dg %.2f", item.specG, item.price))
	}
	return lines
}

func customerShortLabel(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	for _, sep := range []string{"\n", "|", "｜"} {
		if i := strings.Index(s, sep); i >= 0 {
			s = strings.TrimSpace(s[:i])
		}
	}
	re := regexp.MustCompile(`1\d{10}`)
	if loc := re.FindStringIndex(s); loc != nil {
		s = strings.TrimSpace(s[:loc[0]])
	}
	s = strings.NewReplacer("地址：", "", "地址:", "", "收件人:", "", "收件人：", "", "姓名:", "", "姓名：", "").Replace(s)
	s = strings.Trim(s, " ,，:：")
	runes := []rune(s)
	if len(runes) > 30 {
		s = string(runes[:30])
	}
	return s
}

func materialSummaryText(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return "-"
	}

	var items []materialConsumptionSummaryItem
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return raw
	}
	if len(items) == 0 {
		return "-"
	}

	lines := make([]string, 0, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.MaterialName)
		if name == "" {
			continue
		}
		switch {
		case item.DeductG > 0:
			lines = append(lines, fmt.Sprintf("%s 扣减 %dg", name, item.DeductG))
		case item.DeductUnits > 0:
			unit := strings.TrimSpace(item.Unit)
			if unit == "" {
				unit = "个"
			}
			lines = append(lines, fmt.Sprintf("%s 扣减 %d%s", name, item.DeductUnits, unit))
		default:
			lines = append(lines, name)
		}
	}
	if len(lines) == 0 {
		return "-"
	}
	return strings.Join(lines, "\n")
}
