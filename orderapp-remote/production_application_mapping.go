package main

import productionapp "orderapp/internal/application/production"

func productionSummaryFromApp(items []productionapp.SummaryItem) []ProduceBatchSummaryItem {
	out := make([]ProduceBatchSummaryItem, 0, len(items))
	for _, it := range items {
		out = append(out, ProduceBatchSummaryItem{
			ProductID:   it.ProductID,
			ProductName: it.ProductName,
			SpecG:       it.SpecG,
			NeedUnits:   it.NeedUnits,
			NeedG:       it.NeedG,
			DeductedG:   it.DeductedG,
			GapG:        it.GapG,
		})
	}
	return out
}

func productionCreateResultFromApp(res productionapp.CreateBatchResult) ProduceBatchCreateResult {
	return ProduceBatchCreateResult{
		BatchID:    res.BatchID,
		OrderCount: res.OrderCount,
		Summary:    productionSummaryFromApp(res.Summary),
	}
}

func productionPreviewFromApp(res productionapp.DeductPreview) ProduceBatchDeductPreview {
	out := ProduceBatchDeductPreview{BatchID: res.BatchID, Summary: make([]ProduceBatchPreviewItem, 0, len(res.Summary))}
	for _, it := range res.Summary {
		out.Summary = append(out.Summary, ProduceBatchPreviewItem{
			ProductID:       it.ProductID,
			ProductName:     it.ProductName,
			SpecG:           it.SpecG,
			NeedUnits:       it.NeedUnits,
			NeedG:           it.NeedG,
			InvUnits:        it.InvUnits,
			InvLooseG:       it.InvLooseG,
			InvTotalG:       it.InvTotalG,
			DeductedG:       it.DeductedG,
			GapG:            it.GapG,
			WarningLowStock: it.WarningLowStock,
		})
	}
	return out
}

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
