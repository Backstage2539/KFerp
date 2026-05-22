package pdf

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderBeanListPDFProducesDownloadablePDF(t *testing.T) {
	doc := BeanListDocument{
		Title:       "我的豆单",
		ListType:    "commercial",
		VersionNo:   "V3.0.5",
		PublishedAt: "2026-05-03 12:00",
		Groups: []BeanListGroup{{
			Category: "原产地精选豆",
			Items: []BeanListItem{{
				Code:           "5.2",
				Name:           "乌拉嘎",
				RecommendedUse: "手冲/SOE/冷萃",
				Flavor:         "柑橘/莓果",
				Prices:         []BeanListPrice{{Label: "454g", Value: "¥118/包"}},
			}},
		}},
	}

	body, err := BeanListRenderer{}.Render(doc)
	if err != nil {
		t.Fatalf("Render() err=%v", err)
	}
	if !bytes.HasPrefix(body, []byte("%PDF")) || len(body) < 1024 {
		t.Fatalf("PDF body length=%d prefix=%q", len(body), string(body[:min(len(body), 8)]))
	}
}

func TestRenderBeanListPDFUsesPreviewCardStyle(t *testing.T) {
	doc := BeanListDocument{
		Title:               "棵凡咖啡批发豆单",
		Subtitle:            "报价不含税、不含运",
		ListType:            "commercial",
		VersionNo:           "V3.0.6",
		BrandName:           "棵凡咖啡",
		BackgroundColor:     "#f8f1e5",
		FontColor:           "#171717",
		LayoutStyle:         "card",
		CardsPerRow:         2,
		ShowVersion:         true,
		ShowCategoryNumbers: true,
		UsePreviewStyle:     true,
		DisableCompression:  true,
		Groups: []BeanListGroup{{
			Category:     "1、工厂量单",
			ShowCategory: true,
			Items: []BeanListItem{{
				Code:           "1.1",
				Name:           "曲奇拼配",
				RecommendedUse: "意式SOE",
				Flavor:         "坚果、焦糖、巧克力曲奇",
				Description:    "V1～最新",
				Prices: []BeanListPrice{
					{Label: "25-49kg", Value: "21/kg"},
					{Label: "50-99kg", Value: "20/kg"},
				},
			}},
		}},
	}

	body, err := BeanListRenderer{}.Render(doc)
	if err != nil {
		t.Fatalf("Render() err=%v", err)
	}
	for _, want := range [][]byte{
		[]byte("/MediaBox [0 0 306.14 544.25]"),
		[]byte("0.973 0.945 0.898 rg"),
		[]byte("0.875 0.961 0.851 rg"),
		[]byte("0.859 0.918 0.969 rg"),
	} {
		if !bytes.Contains(body, want) {
			t.Fatalf("preview style pdf missing %q", string(want))
		}
	}
}

func TestRenderBeanListPDFPreviewTypographyMatchesVuePreview(t *testing.T) {
	doc := BeanListDocument{
		Title:               "棵凡咖啡批发豆单",
		Subtitle:            "报价不含税、不含运",
		ListType:            "commercial",
		VersionNo:           "V3.0.7",
		BrandName:           "棵凡咖啡",
		BackgroundColor:     "#f8f1e5",
		FontColor:           "#171717",
		LayoutStyle:         "card",
		CardsPerRow:         2,
		ShowVersion:         true,
		ShowCategoryNumbers: true,
		UsePreviewStyle:     true,
		DisableCompression:  true,
		Groups: []BeanListGroup{{
			Category:     "1、定制咖啡熟豆",
			ShowCategory: true,
			Items: []BeanListItem{{
				Code:           "1.2",
				Name:           "芬纳定制-红酒日晒-中深烘",
				RecommendedUse: "客户定制",
				Flavor:         "甜度中、酵感、红酒、坚果、酸质柔和",
				Description:    "V1～最新",
				Prices: []BeanListPrice{
					{Label: "2磅-13磅", Value: "65/磅"},
					{Label: "14-23磅", Value: "59/磅"},
				},
			}},
		}},
	}

	body, err := BeanListRenderer{}.Render(doc)
	if err != nil {
		t.Fatalf("Render() err=%v", err)
	}
	for _, want := range [][]byte{
		[]byte("19.50 Tf"),
		[]byte("15.00 Tf"),
		[]byte("11.25 Tf"),
	} {
		if !bytes.Contains(body, want) {
			t.Fatalf("preview typography pdf missing %q; font markers: %s", string(want), pdfFontMarkers(body))
		}
	}
	if got := pdfDistinctFontCount(body); got < 2 {
		t.Fatalf("preview typography pdf font count = %d, want regular and bold fonts; markers: %s", got, pdfFontMarkers(body))
	}
}

func pdfFontMarkers(body []byte) string {
	chunks := bytes.Fields(body)
	markers := make([]string, 0)
	for i := 0; i+2 < len(chunks); i++ {
		if len(chunks[i]) > 0 && chunks[i][0] == '/' && chunks[i+2][0] == 'T' && bytes.Equal(chunks[i+2], []byte("Tf")) {
			markers = append(markers, string(chunks[i])+" "+string(chunks[i+1])+" Tf")
		}
	}
	return strings.Join(markers, ", ")
}

func pdfDistinctFontCount(body []byte) int {
	chunks := bytes.Fields(body)
	seen := map[string]bool{}
	for i := 0; i+2 < len(chunks); i++ {
		if len(chunks[i]) > 0 && chunks[i][0] == '/' && bytes.Equal(chunks[i+2], []byte("Tf")) {
			seen[string(chunks[i])] = true
		}
	}
	return len(seen)
}
