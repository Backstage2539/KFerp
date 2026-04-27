package stock

import "fmt"

type MovementKind string

const (
	MovementMaterialReceipt MovementKind = "material_receipt"
	MovementMaterialIssue   MovementKind = "material_issue"
	MovementFinishedReceipt MovementKind = "finished_receipt"
	MovementAdjustment      MovementKind = "stock_adjustment"
)

func (k MovementKind) Valid() bool {
	switch k {
	case MovementMaterialReceipt, MovementMaterialIssue, MovementFinishedReceipt, MovementAdjustment:
		return true
	default:
		return false
	}
}

type BatchAvailability struct {
	BatchID    int64
	BatchCode  string
	AvailableG int64
}

type BatchAllocation struct {
	BatchID   int64
	BatchCode string
	QtyG      int64
}

func AllocateFIFO(batches []BatchAvailability, requiredG int64) ([]BatchAllocation, error) {
	if requiredG <= 0 {
		return nil, nil
	}
	remaining := requiredG
	out := make([]BatchAllocation, 0, len(batches))
	for _, batch := range batches {
		if remaining <= 0 {
			break
		}
		if batch.BatchID <= 0 || batch.AvailableG <= 0 {
			continue
		}
		qty := batch.AvailableG
		if qty > remaining {
			qty = remaining
		}
		out = append(out, BatchAllocation{BatchID: batch.BatchID, BatchCode: batch.BatchCode, QtyG: qty})
		remaining -= qty
	}
	if remaining > 0 {
		return nil, fmt.Errorf("insufficient stock: need %dg more", remaining)
	}
	return out, nil
}
