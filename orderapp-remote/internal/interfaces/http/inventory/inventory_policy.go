package inventory

import inventorydomain "orderapp/internal/domain/inventory"

// DeductionDecision is kept as a compatibility alias while inventory policy
// lives in the domain layer.
type DeductionDecision = inventorydomain.DeductionDecision

// decideDeduction applies DEV-046 policy at gram level:
// 1) insufficient inventory => forbidden
// 2) post-deduction below warning line => allowed with warning
// 3) otherwise allowed
func decideDeduction(currentG, needG, warningLineG int64) DeductionDecision {
	return inventorydomain.DecideDeduction(currentG, needG, warningLineG)
}
