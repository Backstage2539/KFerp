package production

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/production"
)

func lockSemiFinishedCapacityReviewTx(ctx context.Context, tx pgx.Tx, schema string, plan app.ProductionPlanDetail) (json.RawMessage, error) {
	ids := []int64{}
	seen := map[int64]bool{}
	add := func(s app.ProductionPlanOperationSplit) {
		if s.WorkstationCapacityID > 0 && !seen[s.WorkstationCapacityID] {
			seen[s.WorkstationCapacityID] = true
			ids = append(ids, s.WorkstationCapacityID)
		}
	}
	for _, s := range plan.OperationSplits {
		add(s)
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT operation_splits_snapshot FROM %s.production_plan_frozen_manufacturing_outputs WHERE production_plan_id=$1`, schema), plan.ID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			rows.Close()
			return nil, err
		}
		var splits []app.ProductionPlanOperationSplit
		if err = json.Unmarshal(raw, &splits); err != nil {
			rows.Close()
			return nil, err
		}
		for _, s := range splits {
			add(s)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = tx.Query(ctx, fmt.Sprintf(`SELECT c.id FROM %[1]s.manufacturing_workstation_capacities c JOIN %[1]s.manufacturing_workstations w ON w.id=c.workstation_id WHERE c.id=ANY($1) ORDER BY c.id FOR SHARE OF c,w`, schema), ids)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = tx.Query(ctx, fmt.Sprintf(`SELECT wo.workstation_id,wo.operation_id FROM %[1]s.manufacturing_workstation_operations wo WHERE wo.workstation_id IN (SELECT workstation_id FROM %[1]s.manufacturing_workstation_capacities WHERE id=ANY($1)) ORDER BY wo.workstation_id,wo.operation_id FOR SHARE OF wo`, schema), ids)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var a, b int64
		if err = rows.Scan(&a, &b); err != nil {
			rows.Close()
			return nil, err
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	var snapshot []byte
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(jsonb_agg(jsonb_build_object('capacity',to_jsonb(c),'workstation',to_jsonb(w),'operations',COALESCE((SELECT jsonb_agg(to_jsonb(wo) ORDER BY wo.operation_id) FROM %[1]s.manufacturing_workstation_operations wo WHERE wo.workstation_id=w.id),'[]'::jsonb)) ORDER BY c.id),'[]'::jsonb) FROM %[1]s.manufacturing_workstation_capacities c JOIN %[1]s.manufacturing_workstations w ON w.id=c.workstation_id WHERE c.id=ANY($1)`, schema), ids).Scan(&snapshot)
	return snapshot, err
}

func semiFinishedPreviewToken(cmd app.SemiFinishedStockCommand, result app.SemiFinishedStockResult, expansion manufacturingExpansion, capacitySnapshot json.RawMessage) (string, error) {
	payload := struct {
		DraftToken string                        `json:"draft_token"`
		Selections []app.SemiFinishedSelection   `json:"selections"`
		Items      []app.SemiFinishedStockItem   `json:"items"`
		Changes    []app.SemiFinishedStockChange `json:"changes"`
		Graph      any                           `json:"graph"`
		Bases      any                           `json:"bases"`
		Capacity   json.RawMessage               `json:"capacity"`
	}{cmd.DraftToken, cmd.Selections, result.Items, result.Changes, expansion.Plan, expansion.Bases, capacitySnapshot}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}
