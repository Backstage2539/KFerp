package production

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	productionapp "orderapp/internal/application/production"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProduceRunOutputRow struct {
	ID             int64
	RunningItemID  int64
	ProductID      int64
	BomSpecID      int64
	BomVariantID   int64
	Product        string
	SpecG          int64
	NeedG          int64
	OrderNos       string
	PlanUnits      int64
	PlanLooseG     int64
	FinishedUnits  int64
	FinishedLooseG int64
}

type startRunGroup struct {
	ProductID                int64
	ParentProductID          int64
	BomSpecID                int64
	BomVariantID             int64
	ProductName              string
	SpecLabel                string
	SalesUnit                string
	SpecG                    int64
	NeedG                    int64
	InputG                   int64
	SalesSpecCount           float64
	InventoryQtyPerSalesUnit float64
	InventoryUnit            string
	PlannedInventoryQty      float64
	SalesSpecSnapshotJSON    string
	ManualInput              bool
	OrderNos                 string
	OperationTemplateID      int64
	Outputs                  []ProduceRunOutputRow
	CustomerID               int64
	TargetWarehouse          string
	ProcessingRequestItemID  int64
}

func groupStartNeedsForRuns(needs []productionapp.StartNeed, inputByKey map[string]int64) []startRunGroup {
	type productGroup struct {
		startRunGroup
		orderNosSeen map[string]bool
	}
	groupsBySnapshot := map[string]*productGroup{}
	order := make([]string, 0)
	for _, need := range needs {
		if need.ProductID <= 0 || (need.GapG <= 0 && need.PlannedInventoryQty <= 0) {
			continue
		}
		groupKey := startNeedProductionSnapshotGroupKey(need)
		group := groupsBySnapshot[groupKey]
		if group == nil {
			group = &productGroup{orderNosSeen: map[string]bool{}}
			group.ProductID = need.ProductID
			group.ParentProductID = need.ParentProductID
			group.BomSpecID = need.BomSpecID
			group.BomVariantID = need.BomVariantID
			group.ProductName = strings.TrimSpace(need.ProductName)
			group.SpecLabel = strings.TrimSpace(need.SpecLabel)
			group.SalesUnit = strings.TrimSpace(need.SalesUnit)
			group.InventoryQtyPerSalesUnit = need.InventoryQtyPerSalesUnit
			group.InventoryUnit = strings.TrimSpace(need.InventoryUnit)
			group.SalesSpecSnapshotJSON = strings.TrimSpace(need.SalesSpecSnapshotJSON)
			group.CustomerID = need.CustomerID
			group.TargetWarehouse = strings.TrimSpace(need.TargetWarehouse)
			group.ProcessingRequestItemID = need.ProcessingRequestItemID
			groupsBySnapshot[groupKey] = group
			order = append(order, groupKey)
		}
		if group.ProductName == "" {
			group.ProductName = strings.TrimSpace(need.ProductName)
		}
		if group.SpecLabel == "" {
			group.SpecLabel = strings.TrimSpace(need.SpecLabel)
		}
		if group.SalesUnit == "" {
			group.SalesUnit = strings.TrimSpace(need.SalesUnit)
		}
		if group.OperationTemplateID <= 0 && need.OperationTemplateID > 0 {
			group.OperationTemplateID = need.OperationTemplateID
		}
		group.NeedG += need.GapG
		group.SalesSpecCount += need.SalesSpecCount
		group.PlannedInventoryQty += need.PlannedInventoryQty
		if input := inputByKey[productionDemandSelectionKey(need.ProductID, need.BomSpecID, need.SpecG)]; input > 0 {
			group.InputG += input
			group.ManualInput = true
		}
		for _, no := range splitOrderNos(need.OrderNos) {
			if group.orderNosSeen[no] {
				continue
			}
			group.orderNosSeen[no] = true
			if group.OrderNos == "" {
				group.OrderNos = no
			} else {
				group.OrderNos += "," + no
			}
		}
		plan := plannedFinishedInventoryAddition(need.SpecG, need.GapG)
		if need.BomSpecID > 0 || (need.PlannedInventoryQty > 0 && productionWeightUnitGrams(need.InventoryUnit) <= 0) {
			plan.Units = int64(math.Ceil(need.PlannedInventoryQty))
			plan.LooseG = 0
		}
		group.Outputs = append(group.Outputs, ProduceRunOutputRow{
			ProductID:    need.ProductID,
			BomSpecID:    need.BomSpecID,
			BomVariantID: need.BomVariantID,
			Product:      strings.TrimSpace(need.ProductName),
			SpecG:        need.SpecG,
			NeedG:        need.GapG,
			OrderNos:     strings.TrimSpace(need.OrderNos),
			PlanUnits:    plan.Units,
			PlanLooseG:   plan.LooseG,
		})
	}
	out := make([]startRunGroup, 0, len(order))
	for _, groupKey := range order {
		group := groupsBySnapshot[groupKey]
		productID := group.ProductID
		sort.SliceStable(group.Outputs, func(i, j int) bool {
			return group.Outputs[i].SpecG > group.Outputs[j].SpecG
		})
		if len(group.Outputs) == 1 {
			group.SpecG = group.Outputs[0].SpecG
			group.Outputs[0].Product = firstNonEmpty(group.Outputs[0].Product, group.ProductName)
			group.Outputs[0].OrderNos = firstNonEmpty(group.Outputs[0].OrderNos, group.OrderNos)
			if input := inputByKey[fmt.Sprintf("product:%d", productID)]; input > 0 {
				group.InputG = firstPositive(group.InputG, input)
				group.ManualInput = true
			}
		} else {
			group.SpecG = 0
			if input := inputByKey[fmt.Sprintf("product:%d", productID)]; input > 0 {
				group.InputG = firstPositive(input, group.InputG)
				group.ManualInput = true
			}
		}
		if group.InputG <= 0 {
			group.InputG = group.NeedG
		}
		out = append(out, group.startRunGroup)
	}
	return out
}

func startNeedProductionSnapshotGroupKey(need productionapp.StartNeed) string {
	snapshot := productionQuantitySnapshot{
		SKUID:                    need.ProductID,
		ParentProductID:          need.ParentProductID,
		BomSpecID:                need.BomSpecID,
		BomVariantID:             need.BomVariantID,
		SpecLabel:                strings.TrimSpace(need.SpecLabel),
		SalesUnit:                strings.TrimSpace(need.SalesUnit),
		InventoryUnit:            strings.TrimSpace(need.InventoryUnit),
		InventoryQtyPerSalesUnit: need.InventoryQtyPerSalesUnit,
		CustomerID:               need.CustomerID,
		TargetWarehouse:          strings.TrimSpace(need.TargetWarehouse),
		ProcessingRequestItemID:  need.ProcessingRequestItemID,
	}
	if raw := strings.TrimSpace(need.SalesSpecSnapshotJSON); raw != "" {
		var frozen productionQuantitySnapshot
		if json.Unmarshal([]byte(raw), &frozen) == nil {
			snapshot.ConversionSource = frozen.ConversionSource
			if frozen.CustomerID > 0 {
				snapshot.CustomerID = frozen.CustomerID
			}
			if strings.TrimSpace(frozen.TargetWarehouse) != "" {
				snapshot.TargetWarehouse = strings.TrimSpace(frozen.TargetWarehouse)
			}
			if frozen.ProcessingRequestItemID > 0 {
				snapshot.ProcessingRequestItemID = frozen.ProcessingRequestItemID
			}
		}
	}
	return productionQuantitySnapshotGroupKey(snapshot)
}

func firstPositive(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func attachRunningOutputs(ctx context.Context, pool *pgxpool.Pool, schema string, rows []ProduceRunRow) error {
	if len(rows) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(rows))
	index := map[int64]int{}
	for i := range rows {
		ids = append(ids, rows[i].ID)
		index[rows[i].ID] = i
	}
	identitySelect := "0::bigint,0::bigint"
	hasBomSpec, err := productionDemandColumnExists(ctx, pool, schema, "produce_running_outputs", "bom_spec_id")
	if err != nil {
		return err
	}
	hasBomVariant, err := productionDemandColumnExists(ctx, pool, schema, "produce_running_outputs", "bom_variant_id")
	if err != nil {
		return err
	}
	if hasBomSpec && hasBomVariant {
		identitySelect = "bom_spec_id,bom_variant_id"
	}
	dbRows, err := pool.Query(ctx, fmt.Sprintf(`
		SELECT id,running_item_id,product_id,%s,COALESCE(product_name,''),spec_g,need_g,order_nos,
		       planned_units,planned_loose_g,finished_units,finished_loose_g
		FROM %s.produce_running_outputs
		WHERE running_item_id = ANY($1)
		ORDER BY running_item_id,spec_g DESC,id
	`, identitySelect, schema), ids)
	if err != nil {
		if strings.Contains(err.Error(), "produce_running_outputs") {
			return nil
		}
		return err
	}
	defer dbRows.Close()
	for dbRows.Next() {
		var output ProduceRunOutputRow
		if err := dbRows.Scan(&output.ID, &output.RunningItemID, &output.ProductID, &output.BomSpecID, &output.BomVariantID, &output.Product, &output.SpecG, &output.NeedG, &output.OrderNos, &output.PlanUnits, &output.PlanLooseG, &output.FinishedUnits, &output.FinishedLooseG); err != nil {
			return err
		}
		i, ok := index[output.RunningItemID]
		if !ok {
			continue
		}
		rows[i].Outputs = append(rows[i].Outputs, output)
	}
	return dbRows.Err()
}

func insertRunningOutputsTx(ctx context.Context, tx pgx.Tx, schema string, runningItemID int64, outputs []ProduceRunOutputRow) error {
	for _, output := range outputs {
		if (output.BomSpecID <= 0 && output.SpecG <= 0) || (output.NeedG <= 0 && output.PlanUnits <= 0) {
			continue
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.produce_running_outputs(
				running_item_id,product_id,bom_spec_id,bom_variant_id,product_name,spec_g,need_g,order_nos,planned_units,planned_loose_g
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
			ON CONFLICT (running_item_id,product_id,bom_spec_id,spec_g) DO UPDATE SET
				bom_variant_id=excluded.bom_variant_id,
				need_g=excluded.need_g,
				order_nos=excluded.order_nos,
				planned_units=excluded.planned_units,
				planned_loose_g=excluded.planned_loose_g
		`, schema), runningItemID, output.ProductID, output.BomSpecID, output.BomVariantID, output.Product, output.SpecG, output.NeedG, output.OrderNos, output.PlanUnits, output.PlanLooseG); err != nil {
			return err
		}
	}
	return nil
}

func loadRunningOutputsForUpdateTx(ctx context.Context, tx pgx.Tx, schema string, runningItemID int64) ([]ProduceRunOutputRow, error) {
	identitySelect := "0::bigint,0::bigint"
	hasBomSpec, err := schemaColumnExistsTx(ctx, tx, schema, "produce_running_outputs", "bom_spec_id")
	if err != nil {
		return nil, err
	}
	hasBomVariant, err := schemaColumnExistsTx(ctx, tx, schema, "produce_running_outputs", "bom_variant_id")
	if err != nil {
		return nil, err
	}
	if hasBomSpec && hasBomVariant {
		identitySelect = "bom_spec_id,bom_variant_id"
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id,running_item_id,product_id,%s,COALESCE(product_name,''),spec_g,need_g,order_nos,
		       planned_units,planned_loose_g,finished_units,finished_loose_g
		FROM %s.produce_running_outputs
		WHERE running_item_id=$1
		ORDER BY spec_g DESC,id
		FOR UPDATE
	`, identitySelect, schema), runningItemID)
	if err != nil {
		if strings.Contains(err.Error(), "produce_running_outputs") {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	out := make([]ProduceRunOutputRow, 0)
	for rows.Next() {
		var output ProduceRunOutputRow
		if err := rows.Scan(&output.ID, &output.RunningItemID, &output.ProductID, &output.BomSpecID, &output.BomVariantID, &output.Product, &output.SpecG, &output.NeedG, &output.OrderNos, &output.PlanUnits, &output.PlanLooseG, &output.FinishedUnits, &output.FinishedLooseG); err != nil {
			return nil, err
		}
		out = append(out, output)
	}
	return out, rows.Err()
}
