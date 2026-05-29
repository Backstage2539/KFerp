package production

import productionapp "orderapp/internal/application/production"

func productionRunningToApp(rows []ProduceRunRow) []productionapp.RunningItem {
	out := make([]productionapp.RunningItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, productionapp.RunningItem{
			ID:            r.ID,
			BatchID:       r.BatchID,
			ProductID:     r.ProductID,
			ProductName:   r.Product,
			SpecG:         r.SpecG,
			NeedG:         r.NeedG,
			InputG:        r.InputG,
			BomYieldRate:  r.BomYieldRate,
			PlanUnits:     r.PlanUnits,
			PlanLooseG:    r.PlanLooseG,
			OrderNos:      r.OrderNos,
			StartedBy:     r.StartedBy,
			StartedAt:     r.StartedAt,
			StartedAtTime: r.StartedAtTime,
			Outputs:       runningOutputsToApp(r.Outputs),
		})
	}
	return out
}

func runningOutputsToApp(rows []ProduceRunOutputRow) []productionapp.RunningOutput {
	out := make([]productionapp.RunningOutput, 0, len(rows))
	for _, row := range rows {
		out = append(out, productionapp.RunningOutput{
			ID:             row.ID,
			SpecG:          row.SpecG,
			NeedG:          row.NeedG,
			OrderNos:       row.OrderNos,
			PlanUnits:      row.PlanUnits,
			PlanLooseG:     row.PlanLooseG,
			FinishedUnits:  row.FinishedUnits,
			FinishedLooseG: row.FinishedLooseG,
		})
	}
	return out
}

func productionRunningFromApp(rows []productionapp.RunningItem) []ProduceRunRow {
	out := make([]ProduceRunRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, ProduceRunRow{
			ID:            r.ID,
			BatchID:       r.BatchID,
			ProductID:     r.ProductID,
			Product:       r.ProductName,
			SpecG:         r.SpecG,
			NeedG:         r.NeedG,
			InputG:        r.InputG,
			BomYieldRate:  r.BomYieldRate,
			PlanUnits:     r.PlanUnits,
			PlanLooseG:    r.PlanLooseG,
			OrderNos:      r.OrderNos,
			StartedBy:     r.StartedBy,
			StartedAt:     r.StartedAt,
			StartedAtTime: r.StartedAtTime,
		})
	}
	return out
}

func startNeedsToApp(rows []UnprodNeedRow) []productionapp.StartNeed {
	out := make([]productionapp.StartNeed, 0, len(rows))
	for _, r := range rows {
		out = append(out, productionapp.StartNeed{
			ProductID:           r.ProductID,
			ProductName:         r.Product,
			SpecG:               r.SpecG,
			GapG:                r.GapG,
			OrderNos:            r.OrderNos,
			OperationTemplateID: r.OperationTemplateID,
		})
	}
	return out
}

func startNeedsFromApp(rows []productionapp.StartNeed) []UnprodNeedRow {
	out := make([]UnprodNeedRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, UnprodNeedRow{
			ProductID:           r.ProductID,
			Product:             r.ProductName,
			SpecG:               r.SpecG,
			GapG:                r.GapG,
			OrderNos:            r.OrderNos,
			OperationTemplateID: r.OperationTemplateID,
		})
	}
	return out
}
