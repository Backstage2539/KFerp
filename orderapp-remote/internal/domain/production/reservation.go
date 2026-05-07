package production

import "fmt"

type WIPReservationQuantity struct {
	ReservedG int64
	ConsumedG int64
	ReturnedG int64
}

func (q WIPReservationQuantity) RemainingG() int64 {
	remaining := q.ReservedG - q.ConsumedG - q.ReturnedG
	if remaining < 0 {
		return 0
	}
	return remaining
}

type WIPReservationAdjustment struct {
	Current         WIPReservationQuantity
	TargetReservedG int64
	WIPG            int64
	OtherReservedG  int64
}

func ValidateWIPReservationAdjustment(adjustment WIPReservationAdjustment) (WIPReservationQuantity, error) {
	current := adjustment.Current
	if adjustment.TargetReservedG < 0 {
		return current, fmt.Errorf("reserved_g must be >= 0")
	}
	minReserved := current.ConsumedG + current.ReturnedG
	if adjustment.TargetReservedG < minReserved {
		return current, fmt.Errorf("reserved_g cannot be less than consumed and returned quantity")
	}
	target := WIPReservationQuantity{
		ReservedG: adjustment.TargetReservedG,
		ConsumedG: current.ConsumedG,
		ReturnedG: current.ReturnedG,
	}
	if target.RemainingG()+maxInt64(adjustment.OtherReservedG, 0) > maxInt64(adjustment.WIPG, 0) {
		return current, fmt.Errorf("reserved_g exceeds available WIP stock")
	}
	return target, nil
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
