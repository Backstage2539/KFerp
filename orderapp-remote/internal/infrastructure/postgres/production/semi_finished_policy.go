package production

import (
	"fmt"
	"math"
)

// semiFinishedStockBudget limits automatic planning to the explicitly selected
// stock for self-manufactured components; legacy plans retain their old rule.
func semiFinishedStockBudget(policy string, semiFinished bool, available, selected float64) (float64, error) {
	if policy != "manual" || !semiFinished {
		return available, nil
	}
	if math.IsNaN(selected) || math.IsInf(selected, 0) || selected < 0 || selected > available+0.000001 {
		return 0, fmt.Errorf("所选半成品库存已变化，请重新选择")
	}
	return selected, nil
}

// legacySemiFinishedManufacturableSQL uses the same criterion for the legacy
// refresh guard and the selection UI, including historical frozen tasks.
func legacySemiFinishedManufacturableSQL(schema, planID, materialID string) string {
	return fmt.Sprintf(`(EXISTS(SELECT 1 FROM %[1]s.production_plan_frozen_manufacturing_outputs f WHERE f.production_plan_id=%[2]s AND f.item_key='material:'||(%[3]s)::text)
 OR EXISTS(SELECT 1 FROM %[1]s.production_plan_items i WHERE i.production_plan_id=%[2]s AND i.output_type='material' AND i.output_material_id=%[3]s)
 OR EXISTS(SELECT 1 FROM %[1]s.production_bom_output_bindings x JOIN %[1]s.production_boms b ON b.id=x.bom_id AND b.output_type='material' AND b.output_material_id=%[3]s JOIN %[1]s.production_bom_versions v ON v.id=x.bom_version_id AND v.status='published' WHERE x.output_type='material' AND x.output_id=%[3]s AND x.is_default AND COALESCE(NULLIF(b.status,''),'active')='active'))`, schema, planID, materialID)
}
