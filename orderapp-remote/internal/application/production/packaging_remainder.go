package production

import (
	"fmt"
	"math"
	"strings"
)

// PackagingRemainderInput is expressed in grams and whole sale units; no
// inventory identity is supplied by the client to this calculation.
type PackagingRemainderInput struct {
	AvailableG, CompletedQty, RemainingTargetQty, StandardGPerPiece, ActualRemainderG float64
	DifferenceReason                                                                  string
}
type PackagingRemainderResult struct{ UsedG, EstimatedRemainderG float64 }

type PackagingUnissuedRemainder struct {
	BatchCode       string `json:"batch_code"`
	Warehouse       string `json:"warehouse"`
	OwnerCustomerID int64  `json:"owner_customer_id"`
	QuantityG       int64  `json:"quantity_g,omitempty"`
	AvailableG      int64  `json:"available_g,omitempty"`
}

func CalculatePackagingRemainder(in PackagingRemainderInput) (PackagingRemainderResult, error) {
	for _, n := range []float64{in.AvailableG, in.CompletedQty, in.RemainingTargetQty, in.StandardGPerPiece, in.ActualRemainderG} {
		if math.IsNaN(n) || math.IsInf(n, 0) || n < 0 {
			return PackagingRemainderResult{}, fmt.Errorf("包装数量及余料必须为非负数")
		}
	}
	if math.Trunc(in.AvailableG) != in.AvailableG || math.Trunc(in.ActualRemainderG) != in.ActualRemainderG {
		return PackagingRemainderResult{}, fmt.Errorf("熟豆数量精确到克")
	}
	if in.CompletedQty <= 0 || math.Trunc(in.CompletedQty) != in.CompletedQty || in.CompletedQty > in.RemainingTargetQty {
		return PackagingRemainderResult{}, fmt.Errorf("完成件数必须为正整数且不能超过剩余目标")
	}
	if in.StandardGPerPiece <= 0 {
		return PackagingRemainderResult{}, fmt.Errorf("请先配置冻结的包装规格")
	}
	if in.ActualRemainderG > in.AvailableG {
		return PackagingRemainderResult{}, fmt.Errorf("余料不能超过实际可用数量")
	}
	standardUsed := math.Round(in.CompletedQty * in.StandardGPerPiece)
	estimate := math.Max(0, in.AvailableG-standardUsed)
	used := in.AvailableG - in.ActualRemainderG
	if used <= 0 {
		return PackagingRemainderResult{}, fmt.Errorf("完成包装时实际用料必须大于 0")
	}
	if math.Abs(used-standardUsed) > 1e-8 && strings.TrimSpace(in.DifferenceReason) == "" {
		return PackagingRemainderResult{}, fmt.Errorf("实际用量与标准用量不同，请填写原因")
	}
	return PackagingRemainderResult{UsedG: used, EstimatedRemainderG: estimate}, nil
}

// PackagingComponentBalance identifies an already-consumed component batch.
// Quantities are stored in the inventory's smallest unit (grams or pieces).
type PackagingComponentBalance struct {
	ID       int64
	Key      string
	Quantity float64
	Cost     float64
}
type PackagingComponentAllocation struct {
	ID       int64
	Quantity float64
	Cost     float64
}

// AllocatePackagingComponents consumes each component independently. In
// particular, an adjustment to bean weight must not change packaging counts.
func AllocatePackagingComponents(balances []PackagingComponentBalance, requested map[string]float64) ([]PackagingComponentAllocation, error) {
	remaining := make(map[string]float64, len(requested))
	for key, n := range requested {
		if key == "" || math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || math.Trunc(n) != n {
			return nil, fmt.Errorf("包装组件数量无效")
		}
		remaining[key] = n
	}
	seen := map[int64]bool{}
	allocations := make([]PackagingComponentAllocation, 0, len(balances))
	for _, b := range balances {
		if b.ID <= 0 || b.Key == "" || seen[b.ID] || math.IsNaN(b.Quantity) || math.IsInf(b.Quantity, 0) || b.Quantity < 0 || math.Trunc(b.Quantity) != b.Quantity || math.IsNaN(b.Cost) || math.IsInf(b.Cost, 0) || b.Cost < 0 {
			return nil, fmt.Errorf("包装组件余额无效")
		}
		seen[b.ID] = true
		qty := math.Min(b.Quantity, remaining[b.Key])
		if qty <= 0 {
			continue
		}
		cost := b.Cost
		if qty < b.Quantity {
			cost = math.Round(b.Cost*qty/b.Quantity*1e6) / 1e6
		}
		allocations = append(allocations, PackagingComponentAllocation{ID: b.ID, Quantity: qty, Cost: cost})
		remaining[b.Key] -= qty
	}
	for _, n := range remaining {
		if n > 1e-8 {
			return nil, fmt.Errorf("包装组件余额不足")
		}
	}
	return allocations, nil
}

// ResolvePackagingWeightComponent requires an unambiguous frozen BOM
// component. A display name is never used to infer inventory identity.
func ResolvePackagingWeightComponent(materials []OperationMaterialUse) (OperationMaterialUse, error) {
	var candidate OperationMaterialUse
	found := false
	for _, m := range materials {
		if OperationUnitFactor(m.Unit) <= 0 {
			continue
		}
		if math.IsNaN(m.TotalQty) || math.IsInf(m.TotalQty, 0) || m.TotalQty <= 0 {
			return OperationMaterialUse{}, fmt.Errorf("请在冻结 BOM 中配置唯一可核对的重量半成品组件")
		}
		if found || m.Key == "" || (m.MaterialID <= 0 && m.ProductID <= 0) || (m.MaterialID > 0 && m.ProductID > 0) {
			return OperationMaterialUse{}, fmt.Errorf("请在冻结 BOM 中配置唯一可核对的重量半成品组件")
		}
		candidate = m
		found = true
	}
	if !found {
		return OperationMaterialUse{}, fmt.Errorf("请在冻结 BOM 中配置唯一可核对的重量半成品组件")
	}
	return candidate, nil
}
