package customerportal

import (
	"encoding/json"
	portal "orderapp/internal/application/customerportal"
	"reflect"
	"testing"
)

func TestPublishedPriceAttributesReachCustomerResponse(t *testing.T) {
	for _, key := range []string{"productAttributes", "product_attributes", "special_attrs_snapshot"} {
		t.Run(key, func(t *testing.T) {
			content, _ := json.Marshal(map[string]any{"groups": []any{map[string]any{"category": "咖啡", "items": []any{map[string]any{"name": "冻结商品", key: []any{
				map[string]any{"key": "roast", "label": "烘焙度", "value": "浅烘", "template_id": 11, "template_position": 1, "position": 1},
				map[string]any{"key": "use", "label": "适用出品", "value": "手冲", "template_id": 11, "template_position": 1, "position": 2},
				map[string]any{"key": "origin", "label": "产地", "value": "云南", "template_id": 12, "template_position": 2, "position": 1},
				map[string]any{"key": "empty", "label": "空值", "value": ""},
				map[string]any{"key": "secret", "label": "内部备注", "value": "不得显示", "show_in_price_list": false},
			}}}}}})
			row := portal.BeanListSummary{ID: 1, VersionNo: "v1"}
			if err := parseBeanListDisplaySummary(nil, content, &row); err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(row)
			var response struct {
				Groups []struct {
					Items []struct {
						Attributes []struct {
							Key              string `json:"key"`
							Label            string `json:"label"`
							Value            string `json:"value"`
							TemplateID       int64  `json:"template_id"`
							TemplatePosition int64  `json:"template_position"`
							Position         int64  `json:"position"`
						} `json:"product_attributes"`
						Lines []string `json:"attribute_lines"`
					} `json:"items"`
				} `json:"groups"`
			}
			if err := json.Unmarshal(raw, &response); err != nil {
				t.Fatal(err)
			}
			item := response.Groups[0].Items[0]
			if len(item.Attributes) != 3 || item.Attributes[0].Key != "roast" || item.Attributes[0].Value != "浅烘" || item.Attributes[0].TemplateID != 11 || item.Attributes[0].TemplatePosition != 1 || item.Attributes[0].Position != 1 {
				t.Fatalf("frozen attributes missing or changed: %s", raw)
			}
			if !reflect.DeepEqual(item.Lines, []string{"烘焙度：浅烘；适用出品：手冲", "产地：云南"}) {
				t.Fatalf("template grouping changed: %v", item.Lines)
			}
		})
	}
}
func TestPublishedLegacyAttributeLinesStayFrozen(t *testing.T) {
	for _, key := range []string{"attributeLines", "attribute_lines"} {
		content, _ := json.Marshal(map[string]any{"groups": []any{map[string]any{"items": []any{map[string]any{"name": "历史商品", key: []any{"历史字段：旧值", "规格：227g"}}}}}})
		row := portal.BeanListSummary{}
		if err := parseBeanListDisplaySummary(nil, content, &row); err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(row)
		var response map[string]any
		json.Unmarshal(raw, &response)
		item := response["groups"].([]any)[0].(map[string]any)["items"].([]any)[0].(map[string]any)
		if !reflect.DeepEqual(item["attribute_lines"], []any{"历史字段：旧值", "规格：227g"}) {
			t.Fatalf("legacy text lost: %s", raw)
		}
	}
}
