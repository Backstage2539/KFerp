package production

import "fmt"

// firstFrozenRouteSequence deliberately reads the frozen route, not live job
// cards: cancelling a first-operation batch cannot make the next step consume.
func firstFrozenRouteSequence(route processTemplateSnapshot) (int, error) {
	if len(route.Operations) == 0 {
		return 0, fmt.Errorf("冻结工艺路线没有工序")
	}
	seen := make(map[int]bool, len(route.Operations))
	first := 0
	for _, operation := range route.Operations {
		if operation.Seq <= 0 || seen[operation.Seq] {
			return 0, fmt.Errorf("冻结工艺路线工序顺序无效")
		}
		seen[operation.Seq] = true
		if first == 0 || operation.Seq < first {
			first = operation.Seq
		}
	}
	return first, nil
}
