package production

import (
	"context"
	"fmt"
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
	ProductID           int64
	ProductName         string
	SpecG               int64
	NeedG               int64
	InputG              int64
	OrderNos            string
	OperationTemplateID int64
	Outputs             []ProduceRunOutputRow
}

func groupStartNeedsForRuns(needs []productionapp.StartNeed, inputByKey map[string]int64, yieldByProductID map[int64]float64) []startRunGroup {
	type productGroup struct {
		startRunGroup
		orderNosSeen map[string]bool
	}
	groupsByProduct := map[int64]*productGroup{}
	order := make([]int64, 0)
	for _, need := range needs {
		if need.ProductID <= 0 || need.GapG <= 0 {
			continue
		}
		group := groupsByProduct[need.ProductID]
		if group == nil {
			group = &productGroup{orderNosSeen: map[string]bool{}}
			group.ProductID = need.ProductID
			group.ProductName = strings.TrimSpace(need.ProductName)
			groupsByProduct[need.ProductID] = group
			order = append(order, need.ProductID)
		}
		if group.ProductName == "" {
			group.ProductName = strings.TrimSpace(need.ProductName)
		}
		if group.OperationTemplateID <= 0 && need.OperationTemplateID > 0 {
			group.OperationTemplateID = need.OperationTemplateID
		}
		group.NeedG += need.GapG
		group.InputG += inputByKey[producePlanKey(need.ProductID, need.SpecG)]
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
		group.Outputs = append(group.Outputs, ProduceRunOutputRow{
			ProductID:  need.ProductID,
			Product:    strings.TrimSpace(need.ProductName),
			SpecG:      need.SpecG,
			NeedG:      need.GapG,
			OrderNos:   strings.TrimSpace(need.OrderNos),
			PlanUnits:  plan.Units,
			PlanLooseG: plan.LooseG,
		})
	}
	out := make([]startRunGroup, 0, len(order))
	for _, productID := range order {
		group := groupsByProduct[productID]
		sort.SliceStable(group.Outputs, func(i, j int) bool {
			return group.Outputs[i].SpecG > group.Outputs[j].SpecG
		})
		if len(group.Outputs) == 1 {
			group.SpecG = group.Outputs[0].SpecG
			group.Outputs[0].Product = firstNonEmpty(group.Outputs[0].Product, group.ProductName)
			group.Outputs[0].OrderNos = firstNonEmpty(group.Outputs[0].OrderNos, group.OrderNos)
			group.InputG = firstPositive(group.InputG, inputByKey[fmt.Sprintf("product:%d", productID)])
		} else {
			group.SpecG = 0
			group.InputG = firstPositive(inputByKey[fmt.Sprintf("product:%d", productID)], group.InputG)
		}
		if group.InputG <= 0 {
			group.InputG = defaultProductionInputG(group.NeedG, yieldByProductID[productID])
		}
		out = append(out, group.startRunGroup)
	}
	return out
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
	dbRows, err := pool.Query(ctx, fmt.Sprintf(`
		SELECT id,running_item_id,product_id,COALESCE(product_name,''),spec_g,need_g,order_nos,
		       planned_units,planned_loose_g,finished_units,finished_loose_g
		FROM %s.produce_running_outputs
		WHERE running_item_id = ANY($1)
		ORDER BY running_item_id,spec_g DESC,id
	`, schema), ids)
	if err != nil {
		if strings.Contains(err.Error(), "produce_running_outputs") {
			return nil
		}
		return err
	}
	defer dbRows.Close()
	for dbRows.Next() {
		var output ProduceRunOutputRow
		if err := dbRows.Scan(&output.ID, &output.RunningItemID, &output.ProductID, &output.Product, &output.SpecG, &output.NeedG, &output.OrderNos, &output.PlanUnits, &output.PlanLooseG, &output.FinishedUnits, &output.FinishedLooseG); err != nil {
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
		if output.SpecG <= 0 || output.NeedG <= 0 {
			continue
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.produce_running_outputs(
				running_item_id,product_id,product_name,spec_g,need_g,order_nos,planned_units,planned_loose_g
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8)
			ON CONFLICT (running_item_id,product_id,spec_g) DO UPDATE SET
				need_g=excluded.need_g,
				order_nos=excluded.order_nos,
				planned_units=excluded.planned_units,
				planned_loose_g=excluded.planned_loose_g
		`, schema), runningItemID, output.ProductID, output.Product, output.SpecG, output.NeedG, output.OrderNos, output.PlanUnits, output.PlanLooseG); err != nil {
			return err
		}
	}
	return nil
}

func loadRunningOutputsForUpdateTx(ctx context.Context, tx pgx.Tx, schema string, runningItemID int64) ([]ProduceRunOutputRow, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id,running_item_id,product_id,COALESCE(product_name,''),spec_g,need_g,order_nos,
		       planned_units,planned_loose_g,finished_units,finished_loose_g
		FROM %s.produce_running_outputs
		WHERE running_item_id=$1
		ORDER BY spec_g DESC,id
		FOR UPDATE
	`, schema), runningItemID)
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
		if err := rows.Scan(&output.ID, &output.RunningItemID, &output.ProductID, &output.Product, &output.SpecG, &output.NeedG, &output.OrderNos, &output.PlanUnits, &output.PlanLooseG, &output.FinishedUnits, &output.FinishedLooseG); err != nil {
			return nil, err
		}
		out = append(out, output)
	}
	return out, rows.Err()
}
