package pdf

import (
	"bytes"
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
