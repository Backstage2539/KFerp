package inventory

// DeductionDecision describes whether a deduction can proceed.
type DeductionDecision struct {
	Allowed     bool
	WarningLow  bool
	Reason      string
	DeductAfter int64
}

// DecideDeduction applies inventory deduction policy at gram level:
// 1) insufficient inventory is forbidden
// 2) post-deduction below warning line is allowed with warning
// 3) otherwise allowed
func DecideDeduction(currentG, needG, warningLineG int64) DeductionDecision {
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
