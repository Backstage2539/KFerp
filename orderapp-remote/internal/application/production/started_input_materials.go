package production

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// ValidateStartedInputMaterials prevents a start confirmation from changing
// the frozen component identity or silently overriding the suggested amount.
func ValidateStartedInputMaterials(expected, actual []OperationMaterialUse, reason string) error {
	if len(expected) != len(actual) {
		return fmt.Errorf("用料清单不完整，请刷新")
	}
	byKey := make(map[string]OperationMaterialUse, len(expected))
	for _, row := range expected {
		if strings.TrimSpace(row.Key) == "" {
			return fmt.Errorf("冻结用料缺少身份")
		}
		if _, exists := byKey[row.Key]; exists {
			return fmt.Errorf("冻结用料身份重复")
		}
		byKey[row.Key] = row
	}
	seen := make(map[string]bool, len(actual))
	for _, row := range actual {
		want, exists := byKey[row.Key]
		if !exists || seen[row.Key] || row.ReservationID != want.ReservationID || row.MaterialID != want.MaterialID || row.ProductID != want.ProductID || row.BomSpecID != want.BomSpecID || row.BomVariantID != want.BomVariantID || row.SpecG != want.SpecG || row.Unit != want.Unit || row.Warehouse != want.Warehouse || row.OwnerCustomerID != want.OwnerCustomerID {
			return fmt.Errorf("用料身份或来源已变化，请刷新")
		}
		seen[row.Key] = true
		if err := ValidateOperationQuantity(row.Qty, row.Unit); err != nil {
			return err
		}
		if factor := OperationUnitFactor(row.Unit); factor > 0 && row.Qty > 0 && math.Round(row.Qty*factor) < 1 {
			return fmt.Errorf("物料 %s 的实际用量不足 1 克", row.Name)
		}
		if !validStartedInputPrecision(row.Qty, 1e8) || !validStartedInputPrecision(want.Qty, 1e8) {
			return fmt.Errorf("用料数量超过支持的精度")
		}
		if math.Round(row.Qty*1e8) != math.Round(want.Qty*1e8) && strings.TrimSpace(reason) == "" {
			return fmt.Errorf("实际用料与建议数量不一致，请填写原因")
		}
	}
	return nil
}

func ValidateStartedInputTotals(materials []OperationMaterialUse, qty float64, unit string, ratio bool) error {
	totalG := 0.0
	positive := false
	for _, m := range materials {
		positive = positive || m.Qty > 0
		if ratio {
			factor := OperationUnitFactor(m.Unit)
			if factor <= 0 {
				return fmt.Errorf("比例配方必须使用重量单位")
			}
			totalG += math.Round(m.Qty * factor)
		}
	}
	if !positive {
		return fmt.Errorf("本批实际用料不能全部为零")
	}
	if ratio && (OperationUnitFactor(unit) <= 0 || math.Abs(totalG-math.Round(qty*OperationUnitFactor(unit))) > 1e-8) {
		return fmt.Errorf("各项实际用料合计必须等于本批实际投入")
	}
	return nil
}

// ResolveBOMStartMaterialUses distributes physical integer grams exactly once.
// Ties retain BOM order, keeping preview and posting deterministic.
func ResolveBOMStartMaterialUses(expected []OperationMaterialUse, qty float64, inputUnit string) ([]OperationMaterialUse, error) {
	factor := OperationUnitFactor(inputUnit)
	if factor <= 0 || math.IsNaN(qty) || math.IsInf(qty, 0) || qty <= 0 {
		return nil, fmt.Errorf("本批实际投入数量无效")
	}
	ratio := 0.0
	for _, m := range expected {
		ratio += m.RatioPct
		if m.RatioPct <= 0 || OperationUnitFactor(m.Unit) <= 0 {
			return nil, fmt.Errorf("物料 %s 缺少有效比例或重量单位", m.Name)
		}
	}
	if reason := IncompleteRoastingBOMReason(ratio); reason != "" {
		return nil, fmt.Errorf("%s", reason)
	}
	grams := int64(math.Round(qty * factor))
	if grams <= 0 {
		return nil, fmt.Errorf("本批投入不足 1 克")
	}
	type share struct {
		idx      int
		grams    int64
		fraction float64
	}
	parts := make([]share, len(expected))
	allocated := int64(0)
	for i, m := range expected {
		raw := float64(grams) * m.RatioPct / ratio
		whole := int64(math.Floor(raw))
		parts[i] = share{i, whole, raw - float64(whole)}
		allocated += whole
	}
	sort.SliceStable(parts, func(i, j int) bool { return parts[i].fraction > parts[j].fraction })
	for i := int64(0); i < grams-allocated; i++ {
		parts[int(i)%len(parts)].grams++
	}
	out := append([]OperationMaterialUse(nil), expected...)
	for _, p := range parts {
		f := OperationUnitFactor(out[p.idx].Unit)
		v := math.Round(float64(p.grams)/f*1e8) / 1e8
		out[p.idx].Qty = v
		out[p.idx].TotalQty = v
	}
	return out, nil
}
