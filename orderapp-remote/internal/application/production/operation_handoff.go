package production

import (
	"fmt"
	"math"
)

// Handoff is an operation-order constraint, never a planned quantity target.
// Actual reports use the work order inventory unit; planned_input_qty may be
// canonical grams, so those values must not be compared or substituted here.
func applyTaskOperationHandoff(tasks []ProductionTask, cards []JobCardRow) {
	for i := range tasks {
		task := &tasks[i]
		if task.JobCardID <= 0 {
			continue
		}
		previousSequence, incomplete := 0, 0
		for _, card := range cards {
			if card.WorkOrderID != task.WorkOrderID || card.SequenceNo >= task.SequenceNo {
				continue
			}
			task.HasPredecessor = true
			if card.Status == "cancelled" {
				continue
			}
			if card.SequenceNo > previousSequence {
				previousSequence = card.SequenceNo
			}
			if card.Status != "completed" {
				incomplete++
			}
		}
		if !task.HasPredecessor {
			continue
		}
		task.PredecessorCompleted = incomplete == 0
		for _, card := range cards {
			if card.WorkOrderID == task.WorkOrderID && card.SequenceNo == previousSequence && card.Status == "completed" {
				task.PredecessorOutputQty += card.ActualOutputQty
			}
		}
		task.PredecessorOutputQty = math.Round(task.PredecessorOutputQty*1e9) / 1e9
		task.StockAction = "none"
		task.MaterialDataComplete = true
		task.MaterialReadiness = nil
		task.MaterialReadinessState = "ready"
		if incomplete > 0 {
			reason := fmt.Sprintf("前序工序未完成：仍有 %d 项任务，请先完成前序工序", incomplete)
			task.IsBlocked = true
			task.Readiness, task.ReadinessLabel = "blocked", "待前序完成"
			task.StatusLabel = productionTaskStatusLabel(task.Status, reason)
			if task.BlockingReason == "" {
				task.BlockingReason = reason
				task.NextHandler = "前序工位"
			}
			task.BlockingReasons = append(task.BlockingReasons, productionBlockingReasonRow("prior_operation_incomplete", reason, "blocked", "前序工位", nil))
			for _, action := range []string{"start", "resume", "complete"} {
				task.AvailableActions = removeProductionAction(task.AvailableActions, action)
			}
		}
		applyProductionTaskReadinessDetail(task)
	}
}
