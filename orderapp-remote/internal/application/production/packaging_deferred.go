package production

import (
	"fmt"
	"math"
)

// DeferredPackagingUsage allocates frozen standard grams to this report using
// cumulative rounding so partial reports sum to the frozen total exactly.
func DeferredPackagingUsage(totalG, before, quantity, target, availableG float64) (float64, error) {
	for _, n := range []float64{totalG, before, quantity, target, availableG} {
		if math.IsNaN(n) || math.IsInf(n, 0) || n < 0 {
			return 0, fmt.Errorf("包装投入数量无效")
		}
	}
	if target <= 0 || quantity <= 0 || before+quantity > target+1e-8 || totalG <= 0 || math.Abs(totalG-math.Round(totalG)) > 1e-8 || math.Abs(availableG-math.Round(availableG)) > 1e-8 {
		return 0, fmt.Errorf("包装冻结用量或完成数量无效")
	}
	used := math.Round(totalG*(before+quantity)/target) - math.Round(totalG*before/target)
	if used > availableG+1e-8 {
		return 0, fmt.Errorf("已确认包装投入不足，请核对")
	}
	return used, nil
}
