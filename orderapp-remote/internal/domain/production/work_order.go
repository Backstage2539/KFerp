package production

type WorkOrderStatus string

const (
	StatusDraft     WorkOrderStatus = "draft"
	StatusReleased  WorkOrderStatus = "released"
	StatusRunning   WorkOrderStatus = "running"
	StatusCompleted WorkOrderStatus = "completed"
	StatusCancelled WorkOrderStatus = "cancelled"
)

func CanTransitionWorkOrder(from, to WorkOrderStatus) bool {
	if from == to {
		return true
	}
	switch from {
	case StatusDraft:
		return to == StatusReleased || to == StatusCancelled
	case StatusReleased:
		return to == StatusRunning || to == StatusCancelled
	case StatusRunning:
		return to == StatusCompleted || to == StatusCancelled
	default:
		return false
	}
}

type CostComponent struct {
	Amount float64
}

func BatchCost(items []CostComponent) float64 {
	total := 0.0
	for _, item := range items {
		total += item.Amount
	}
	return total
}
