package production

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// IncomingOutputQuantity derives good output from processed input and waste.
// Waste is expressed in the input unit; units must both denote weight.
func IncomingOutputQuantity(input float64, inputUnit string, waste float64, outputUnit, reason string) (float64, error) {
	inFactor, outFactor := OperationUnitFactor(inputUnit), OperationUnitFactor(outputUnit)
	if inFactor <= 0 || outFactor <= 0 || math.IsNaN(input) || math.IsInf(input, 0) || math.IsNaN(waste) || math.IsInf(waste, 0) || input < 0 || waste < 0 || waste > input {
		return 0, fmt.Errorf("废料重量必须大于或等于 0 且不超过本次处理投入，投入和产出须使用重量单位")
	}
	if waste > 0 && strings.TrimSpace(reason) == "" {
		return 0, fmt.Errorf("请填写废料原因")
	}
	return math.Round((input-waste)*inFactor/outFactor*1e8) / 1e8, nil
}

// AllocateIncomingQuantities distributes integer minimum units using the
// largest-remainder method. Caller supplies stable card order to break ties.
func AllocateIncomingQuantities(total int64, weights []float64) ([]int64, error) {
	if total < 0 || len(weights) == 0 {
		return nil, fmt.Errorf("来料分配数量无效")
	}
	sum := 0.0
	for _, w := range weights {
		if w < 0 || math.IsNaN(w) || math.IsInf(w, 0) {
			return nil, fmt.Errorf("来料分配比例无效")
		}
		sum += w
	}
	if sum <= 0 {
		return nil, fmt.Errorf("请先安排色选任务数量")
	}
	out := make([]int64, len(weights))
	type fraction struct {
		i int
		v float64
	}
	remainders := make([]fraction, len(weights))
	var used int64
	for i, w := range weights {
		exact := float64(total) * w / sum
		out[i] = int64(math.Floor(exact))
		used += out[i]
		remainders[i] = fraction{i, exact - float64(out[i])}
	}
	sort.SliceStable(remainders, func(i, j int) bool { return remainders[i].v > remainders[j].v })
	for i := int64(0); i < total-used; i++ {
		out[remainders[i%int64(len(remainders))].i]++
	}
	return out, nil
}
