package main

// DeductionDecision describes whether a deduction can proceed under DEV-046 rules.
type DeductionDecision struct {
	Allowed     bool
	WarningLow  bool
	Reason      string
	DeductAfter int64
}

// decideDeduction applies DEV-046 policy at gram level:
// 1) insufficient inventory => forbidden
// 2) post-deduction below warning line => allowed with warning
// 3) otherwise allowed
func decideDeduction(currentG, needG, warningLineG int64) DeductionDecision {
	if needG < 0 {
		return DeductionDecision{Allowed: false, Reason: "invalid_need"}
	}
	if currentG < 0 || warningLineG < 0 {
		return DeductionDecision{Allowed: false, Reason: "invalid_inventory"}
	}
	if currentG < needG {
		return DeductionDecision{Allowed: false, Reason: "insufficient"}
	}
	after := currentG - needG
	if after < warningLineG {
		return DeductionDecision{Allowed: true, WarningLow: true, DeductAfter: after}
	}
	return DeductionDecision{Allowed: true, WarningLow: false, DeductAfter: after}
}
