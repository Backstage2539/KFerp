package bom

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

type TemplateRolloutSpec struct {
	Key     string
	Variant ProductionBomDraftVariant
}
type TemplateRolloutChange struct {
	SpecKey string `json:"spec_key"`
	Field   string `json:"field"`
	Before  any    `json:"before"`
	After   any    `json:"after"`
}
type TemplateRolloutMerge struct {
	Specs     []TemplateRolloutSpec
	Changes   []TemplateRolloutChange
	Conflicts []string
}
type TemplateRolloutTarget struct {
	BomID             int64 `json:"bom_id"`
	ExpectedVersionID int64 `json:"expected_version_id"`
}
type TemplateRolloutRow struct {
	BomID                   int64                   `json:"bom_id"`
	BomName                 string                  `json:"bom_name"`
	ProductName             string                  `json:"product_name"`
	CurrentVersionID        int64                   `json:"current_version_id"`
	CurrentVersionNo        string                  `json:"current_version_no"`
	SourceTemplateVersionID int64                   `json:"source_template_version_id"`
	SourceTemplateVersionNo string                  `json:"source_template_version_no"`
	Status                  string                  `json:"status"`
	Reason                  string                  `json:"reason"`
	Changes                 []TemplateRolloutChange `json:"changes"`
	NewVersionID            int64                   `json:"new_version_id,omitempty"`
	NewVersionNo            string                  `json:"new_version_no,omitempty"`
}
type TemplateRolloutPreview struct {
	TemplateID        int64                `json:"template_id"`
	TemplateVersionID int64                `json:"template_version_id"`
	TemplateVersionNo string               `json:"template_version_no"`
	Rows              []TemplateRolloutRow `json:"rows"`
}
type TemplateRolloutCommand struct {
	VersionID int64                   `json:"-"`
	Targets   []TemplateRolloutTarget `json:"targets"`
	Actor     string                  `json:"-"`
}
type TemplateRolloutResult struct {
	BatchID string               `json:"batch_id"`
	Rows    []TemplateRolloutRow `json:"rows"`
}
type TemplateRolloutRepository interface {
	PreviewSpecTemplateRollout(context.Context, int64) (TemplateRolloutPreview, error)
	ApplySpecTemplateRollout(context.Context, TemplateRolloutCommand) (TemplateRolloutResult, error)
}

func (s *Service) PreviewSpecTemplateRollout(ctx context.Context, id int64) (TemplateRolloutPreview, error) {
	r, ok := s.repo.(TemplateRolloutRepository)
	if !ok {
		return TemplateRolloutPreview{}, fmt.Errorf("模板下发暂不可用")
	}
	return r.PreviewSpecTemplateRollout(ctx, id)
}
func (s *Service) ApplySpecTemplateRollout(ctx context.Context, c TemplateRolloutCommand) (TemplateRolloutResult, error) {
	if c.VersionID <= 0 || len(c.Targets) == 0 || len(c.Targets) > 100 {
		return TemplateRolloutResult{}, fmt.Errorf("每次请选择 1 至 100 个 BOM")
	}
	seen := map[int64]bool{}
	for _, target := range c.Targets {
		if target.BomID <= 0 || target.ExpectedVersionID <= 0 || seen[target.BomID] {
			return TemplateRolloutResult{}, fmt.Errorf("BOM 或预期版本无效或重复")
		}
		seen[target.BomID] = true
	}
	r, ok := s.repo.(TemplateRolloutRepository)
	if !ok {
		return TemplateRolloutResult{}, fmt.Errorf("模板下发暂不可用")
	}
	return r.ApplySpecTemplateRollout(ctx, c)
}

// Comparisons exclude database identity and cost snapshots. Component lists
// are an unordered whole; component loss follows the specification loss.
func rolloutFields(v ProductionBomDraftVariant) map[string]any {
	items := make([]ProductionBomDraftItem, len(v.Items))
	copy(items, v.Items)
	for i := range items {
		items[i].BomVariantID = 0
		items[i].MaterialLossRate = 0
		if items[i].ComponentType == "" {
			items[i].ComponentType = "material"
		}
	}
	sort.Slice(items, func(i, j int) bool {
		a, _ := json.Marshal(items[i])
		b, _ := json.Marshal(items[j])
		return string(a) < string(b)
	})
	raw, _ := json.Marshal(struct {
		Name    string                   `json:"名称"`
		Unit    string                   `json:"库存单位"`
		Default bool                     `json:"默认规格"`
		Sort    int                      `json:"排序"`
		Loss    float64                  `json:"损耗率"`
		Route   int64                    `json:"工艺"`
		Weight  *float64                 `json:"物流重量"`
		Items   []ProductionBomDraftItem `json:"组件及用量"`
	}{v.Name, v.InventoryUnit, v.IsDefault, v.SortOrder, v.MaterialLossRate, v.ProcessRouteID, v.LogisticsWeightG, items})
	out := map[string]any{}
	_ = json.Unmarshal(raw, &out)
	return out
}
func setRolloutFields(v *ProductionBomDraftVariant, f map[string]any) {
	v.Name = f["名称"].(string)
	v.InventoryUnit = f["库存单位"].(string)
	v.IsDefault = f["默认规格"].(bool)
	v.SortOrder = int(f["排序"].(float64))
	v.MaterialLossRate = f["损耗率"].(float64)
	v.ProcessRouteID = int64(f["工艺"].(float64))
	v.LogisticsWeightG = nil
	if w, ok := f["物流重量"].(float64); ok {
		v.LogisticsWeightG = &w
	}
	raw, _ := json.Marshal(f["组件及用量"])
	_ = json.Unmarshal(raw, &v.Items)
	for i := range v.Items {
		v.Items[i].MaterialLossRate = v.MaterialLossRate
	}
}
func MergeTemplateRollout(old, next, current []TemplateRolloutSpec) TemplateRolloutMerge {

	result := TemplateRolloutMerge{Specs: []TemplateRolloutSpec{}, Changes: []TemplateRolloutChange{}, Conflicts: []string{}}
	index := func(rows []TemplateRolloutSpec) map[string]TemplateRolloutSpec {
		m := map[string]TemplateRolloutSpec{}
		for _, v := range rows {
			key := strings.ToLower(v.Key)
			if _, exists := m[key]; exists {
				result.Conflicts = append(result.Conflicts, v.Key+"：规格来源重复")
			}
			m[key] = v
		}
		return m
	}
	before, after, live := index(old), index(next), index(current)
	fields := []string{"名称", "库存单位", "默认规格", "排序", "损耗率", "工艺", "物流重量", "组件及用量"}
	for _, c := range current {
		key := strings.ToLower(c.Key)
		o, had := before[key]
		n, has := after[key]
		if !had {
			if has {
				result.Conflicts = append(result.Conflicts, c.Key+"：新增模板规格与本地规格冲突")
			}
			result.Specs = append(result.Specs, c)
			continue
		}
		of, cf := rolloutFields(o.Variant), rolloutFields(c.Variant)
		if !has {
			if !reflect.DeepEqual(of, cf) {
				result.Conflicts = append(result.Conflicts, c.Key+"：待删除规格有局部修改")
				result.Changes = append(result.Changes, TemplateRolloutChange{c.Key, "删除规格", cf, nil})
			} else {
				result.Changes = append(result.Changes, TemplateRolloutChange{c.Key, "删除规格", of, nil})
			}
			continue
		}
		nf := rolloutFields(n.Variant)
		for _, field := range fields {
			if reflect.DeepEqual(of[field], nf[field]) {
				continue
			}
			if !reflect.DeepEqual(cf[field], of[field]) && !reflect.DeepEqual(cf[field], nf[field]) {
				result.Conflicts = append(result.Conflicts, c.Key+"："+field+"存在局部修改")
				result.Changes = append(result.Changes, TemplateRolloutChange{c.Key, field, cf[field], nf[field]})
				continue
			}
			if !reflect.DeepEqual(cf[field], nf[field]) {
				result.Changes = append(result.Changes, TemplateRolloutChange{c.Key, field, cf[field], nf[field]})
				cf[field] = nf[field]
			}
		}
		setRolloutFields(&c.Variant, cf)
		result.Specs = append(result.Specs, c)
	}
	for _, n := range next {
		key := strings.ToLower(n.Key)
		if _, exists := live[key]; exists {
			continue
		}
		if o, had := before[key]; had {
			if !reflect.DeepEqual(rolloutFields(o.Variant), rolloutFields(n.Variant)) {
				result.Conflicts = append(result.Conflicts, n.Key+"：BOM已删除该规格，模板又有更新")
			}
			continue
		}
		result.Specs = append(result.Specs, n)
		result.Changes = append(result.Changes, TemplateRolloutChange{n.Key, "新增规格", nil, rolloutFields(n.Variant)})
	}
	return result
}
