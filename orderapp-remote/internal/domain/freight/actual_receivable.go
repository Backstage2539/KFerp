package freight

import "fmt"

type ActualReceivableInput struct {
	Actual, Surcharge, Current, Paid int64
	Free, UnknownPayment             bool
}
type ActualReceivableDecision struct {
	TargetCents int64
	Review      bool
	Reason      string
}

func ConfirmActualReceivable(a ActualReceivableInput) (ActualReceivableDecision, error) {
	d := ActualReceivableDecision{TargetCents: a.Current}
	if !validMoney(a.Actual) || !validMoney(a.Surcharge) || a.Actual > MaxCents-a.Surcharge {
		return d, fmt.Errorf("实际运费或附加费无效")
	}
	switch {
	case a.Free:
		d.Reason = "包邮，仅记录成本"
	case a.UnknownPayment:
		d.Review = true
		d.Reason = "历史收款待核对，仅记录成本"
	case a.Paid > 0 && a.Paid >= a.Current:
		d.Reason = "运费已收清，仅记录成本"
	case a.Actual+a.Surcharge < a.Paid:
		d.Review = true
		d.Reason = "实际费用低于已收运费，请财务核对"
	default:
		d.TargetCents = a.Actual + a.Surcharge
		d.Reason = "已按实际成本及附加费确认运费"
	}
	return d, nil
}
