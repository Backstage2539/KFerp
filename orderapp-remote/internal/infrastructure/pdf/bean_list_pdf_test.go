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
