package production

import (
	"fmt"
	"math"
)

// StartedInputBalance is an unallocated portion of a confirmed start or top-up.
// Callers must supply balances in their persisted FIFO order.
type StartedInputBalance struct {
	ID            int64
	RemainingQty  float64
	RemainingCost float64
}

type StartedInputAllocation struct {
	ID   int64
	Qty  float64
	Cost float64
}

// AllocateStartedInputCost assigns already-consumed input and its cost to one
// completion. The final allocation of a balance takes its entire remaining
// cost, so rounding cannot leave a residual cost behind.
func AllocateStartedInputCost(balances []StartedInputBalance, qty float64) ([]StartedInputAllocation, error) {
	if !validStartedInputPrecision(qty, 1e8) {
		return nil, fmt.Errorf("实际处理投入数量无效")
	}
	available := 0.0
	seen := make(map[int64]bool, len(balances))
	for _, b := range balances {
		if b.ID <= 0 || seen[b.ID] || !validStartedInputPrecision(b.RemainingQty, 1e8) || !validStartedInputPrecision(b.RemainingCost, 1e6) {
			return nil, fmt.Errorf("开工投入余额无效")
		}
		seen[b.ID] = true
		available += b.RemainingQty
	}
	if math.Round(qty*1e8) > math.Round(available*1e8) {
		return nil, fmt.Errorf("实际处理投入超过已确认投入余额")
	}
	remaining := qty
	allocations := make([]StartedInputAllocation, 0, len(balances))
	for _, b := range balances {
		if remaining <= 0 {
			break
		}
		if b.RemainingQty <= 0 {
			continue
		}
		used := math.Min(remaining, b.RemainingQty)
		cost := b.RemainingCost
		if used < b.RemainingQty {
			cost = math.Round(b.RemainingCost*used/b.RemainingQty*1e6) / 1e6
		}
		allocations = append(allocations, StartedInputAllocation{ID: b.ID, Qty: used, Cost: cost})
		remaining -= used
	}
	return allocations, nil
}

// Ledger quantities and costs are persisted at scales 8 and 6 respectively.
// Reject unrepresentable values instead of silently rounding them during writes.
func validStartedInputPrecision(value, scale float64) bool {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return false
	}
	scaled := value * scale
	return !math.IsInf(scaled, 0) && (value == 0 || math.Round(scaled) > 0) && math.Abs(scaled-math.Round(scaled)) < 1e-6
}
