package production

import (
	"fmt"
	"math"
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
		if factor := OperationUnitFactor(row.Unit); factor > 0 && math.Abs(row.Qty*factor-math.Round(row.Qty*factor)) > 1e-8 {
			return fmt.Errorf("物料 %s 的实际用量必须能换算为整数克", row.Name)
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
			totalG += m.Qty * factor
		}
	}
	if !positive {
		return fmt.Errorf("本批实际用料不能全部为零")
	}
	if ratio && (OperationUnitFactor(unit) <= 0 || math.Abs(totalG-qty*OperationUnitFactor(unit)) > 1e-8) {
		return fmt.Errorf("各项实际用料合计必须等于本批实际投入")
	}
	return nil
}
