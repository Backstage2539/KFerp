package production

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/production"
	infra "orderapp/internal/infrastructure/postgres"
)

func mergeReportingTypes(oldJSON, currentJSON string) (string, []app.ReportingTypeChange, error) {
	var old, current map[string]json.RawMessage
	if json.Unmarshal([]byte(oldJSON), &old) != nil || json.Unmarshal([]byte(currentJSON), &current) != nil {
		return "", nil, fmt.Errorf("工艺路线快照无效")
	}
	var a, b []map[string]json.RawMessage
	if json.Unmarshal(old["operations"], &a) != nil || json.Unmarshal(current["operations"], &b) != nil || len(a) == 0 || len(a) != len(b) {
		return "", nil, fmt.Errorf("工艺路线结构已变化，请重新建立计划")
	}
	var changes []app.ReportingTypeChange
	for i := range a {
		var oldSeq, newSeq int
		var oldID, newID int64
		var name string
		_ = json.Unmarshal(a[i]["seq"], &oldSeq)
		_ = json.Unmarshal(b[i]["seq"], &newSeq)
		_ = json.Unmarshal(a[i]["operation_id"], &oldID)
		_ = json.Unmarshal(b[i]["operation_id"], &newID)
		_ = json.Unmarshal(a[i]["operation"], &name)
		if oldSeq <= 0 || oldSeq != newSeq || oldID <= 0 || oldID != newID {
			return "", nil, fmt.Errorf("工艺路线工序已变化，请重新建立计划")
		}
		var oldExec, newExec string
		_ = json.Unmarshal(a[i]["execution_json"], &oldExec)
		_ = json.Unmarshal(b[i]["execution_json"], &newExec)
		if oldExec == "" {
			oldExec = "{}"
		}
		if newExec == "" {
			newExec = "{}"
		}
		var x, y map[string]json.RawMessage
		if json.Unmarshal([]byte(oldExec), &x) != nil || json.Unmarshal([]byte(newExec), &y) != nil {
			return "", nil, fmt.Errorf("工序执行配置无效")
		}
		var oldNode, newNode, from, to string
		_ = json.Unmarshal(x["node_key"], &oldNode)
		_ = json.Unmarshal(y["node_key"], &newNode)
		_ = json.Unmarshal(x["reporting_type"], &from)
		_ = json.Unmarshal(y["reporting_type"], &to)
		if oldNode != "" && newNode != "" && oldNode != newNode {
			return "", nil, fmt.Errorf("工艺路线节点已变化，请重新建立计划")
		}
		if from == "" {
			from = "legacy"
		}
		if to == "" {
			to = "legacy"
		}
		if to != "legacy" && to != "batch" && to != "incoming" && to != "piece" {
			return "", nil, fmt.Errorf("工序报工类型无效")
		}
		if from != to {
			changes = append(changes, app.ReportingTypeChange{Seq: oldSeq, OperationID: oldID, Operation: name, From: from, To: to})
			x["reporting_type"], _ = json.Marshal(to)
			raw, _ := json.Marshal(x)
			a[i]["execution_json"], _ = json.Marshal(string(raw))
		}
	}
	old["operations"], _ = json.Marshal(a)
	merged, err := json.Marshal(old)
	return string(merged), changes, err
}

func (r Repository) RecheckReportingTypes(ctx context.Context, kind string, id int64, expectedVersion, actor string) (app.ReportingRecheck, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return app.ReportingRecheck{}, err
	}
	defer tx.Rollback(ctx)
	var planID, itemID, routeID, productID int64
	var frozen, itemFrozen, current, status, policy, rowVersion, itemVersion string
	if kind == "work_order" {
		var runningID int64
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT production_plan_id,production_plan_item_id,process_template_id,product_id,process_snapshot_json::text,status,material_consumption_policy,running_item_id,xmin::text FROM %s.work_orders WHERE id=$1 FOR UPDATE`, r.schema), id).Scan(&planID, &itemID, &routeID, &productID, &frozen, &status, &policy, &runningID, &rowVersion)
		if err != nil {
			return app.ReportingRecheck{}, err
		}
		if status != "released" || runningID > 0 || policy != "bom_start_v1" {
			return app.ReportingRecheck{}, fmt.Errorf("只有未开工的新策略工单可以重核")
		}
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT process_route_snapshot_json::text,xmin::text FROM %s.production_plan_items WHERE id=$1 AND production_plan_id=$2 FOR UPDATE`, r.schema), itemID, planID).Scan(&itemFrozen, &itemVersion); err != nil {
			return app.ReportingRecheck{}, fmt.Errorf("工单缺少关联计划项，不能重核")
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`SELECT 1 FROM %s.production_plans WHERE id=$1 FOR UPDATE`, r.schema), planID); err != nil {
			return app.ReportingRecheck{}, err
		}
		var unsafe bool
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.job_cards WHERE work_order_id=$1 AND status<>'pending') OR EXISTS(SELECT 1 FROM %[1]s.production_operation_events WHERE work_order_id=$1) OR EXISTS(SELECT 1 FROM %[1]s.stock_entries WHERE work_order_id=$1 AND status='submitted') OR EXISTS(SELECT 1 FROM %[1]s.work_orders WHERE production_plan_item_id=$2 AND id<>$1)`, r.schema), id, itemID).Scan(&unsafe)
		if err != nil {
			return app.ReportingRecheck{}, err
		}
		if unsafe {
			return app.ReportingRecheck{}, fmt.Errorf("工单已有执行或库存记录，或计划项被其他工单使用，不能重核")
		}
	} else {
		itemID = id
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT i.production_plan_id,i.process_route_id,i.product_id,i.process_route_snapshot_json::text,p.status,i.xmin::text FROM %[1]s.production_plan_items i JOIN %[1]s.production_plans p ON p.id=i.production_plan_id WHERE i.id=$1 FOR UPDATE OF i,p`, r.schema), id).Scan(&planID, &routeID, &productID, &frozen, &status, &rowVersion)
		if err != nil {
			return app.ReportingRecheck{}, err
		}
		if status != "draft" {
			return app.ReportingRecheck{}, fmt.Errorf("只有草稿计划可以重核")
		}
		var exists bool
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.work_orders WHERE production_plan_item_id=$1)`, r.schema), id).Scan(&exists)
		if err != nil {
			return app.ReportingRecheck{}, err
		}
		if exists {
			return app.ReportingRecheck{}, fmt.Errorf("计划项已生成工单，不能在草稿中重核")
		}
		itemFrozen = frozen
		itemVersion = rowVersion
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`SELECT 1 FROM %s.process_routes WHERE id=$1 FOR SHARE`, r.schema), routeID); err != nil {
		return app.ReportingRecheck{}, err
	}
	locked, err := tx.Query(ctx, fmt.Sprintf(`SELECT id FROM %s.process_route_operations WHERE route_id=$1 FOR SHARE`, r.schema), routeID)
	if err != nil {
		return app.ReportingRecheck{}, err
	}
	for locked.Next() {
		var ignored int64
		if err = locked.Scan(&ignored); err != nil {
			locked.Close()
			return app.ReportingRecheck{}, err
		}
	}
	err = locked.Err()
	locked.Close()
	if err != nil {
		return app.ReportingRecheck{}, err
	}
	route, raw, err := loadProcessRouteSnapshotByIDTx(ctx, tx, r.schema, routeID, productID)
	if err != nil {
		return app.ReportingRecheck{}, err
	}
	if route == nil {
		return app.ReportingRecheck{}, fmt.Errorf("原工艺路线已停用或不存在")
	}
	current = string(raw)
	merged, changes, err := mergeReportingTypes(frozen, current)
	if err != nil {
		return app.ReportingRecheck{}, err
	}
	itemMerged, _, err := mergeReportingTypes(itemFrozen, current)
	if err != nil {
		return app.ReportingRecheck{}, err
	}
	var planRevision int64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT revision FROM %s.production_plans WHERE id=$1`, r.schema), planID).Scan(&planRevision); err != nil {
		return app.ReportingRecheck{}, err
	}
	var splitVersion string
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(jsonb_agg(to_jsonb(s) ORDER BY s.id),'[]'::jsonb)::text FROM %s.production_plan_operation_splits s WHERE production_plan_item_id=$1`, r.schema), itemID).Scan(&splitVersion); err != nil {
		return app.ReportingRecheck{}, err
	}
	var cardVersion string
	if kind == "work_order" {
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(jsonb_agg(jsonb_build_object('id',id,'status',status,'metrics',metrics_json) ORDER BY id),'[]'::jsonb)::text FROM %s.job_cards WHERE work_order_id=$1`, r.schema), id).Scan(&cardVersion); err != nil {
			return app.ReportingRecheck{}, err
		}
	}
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%d:%s:%s:%s:%s:%s:%s:%s", kind, id, planRevision, rowVersion, itemVersion, frozen, itemFrozen, current, splitVersion, cardVersion)))
	version := fmt.Sprintf("%x", hash)
	result := app.ReportingRecheck{Version: version, Changes: changes, RequiresReplan: len(changes) > 0}
	if expectedVersion == "" {
		return result, nil
	}
	if expectedVersion != version {
		return app.ReportingRecheck{}, fmt.Errorf("重核内容已变化，请重新预览")
	}
	if len(changes) == 0 {
		return result, nil
	}
	for _, change := range changes {
		if _, err = tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.production_plan_operation_splits WHERE production_plan_item_id=$1 AND (operation_seq=$2 OR (operation_seq=0 AND (operation_id=$3 OR (operation_id=0 AND operation=$4))))`, r.schema), itemID, change.Seq, change.OperationID, change.Operation); err != nil {
			return app.ReportingRecheck{}, err
		}
	}
	if kind == "work_order" {
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.job_cards SET metrics_json=COALESCE(metrics_json,'{}'::jsonb)||'{"reporting_replan_required":true}'::jsonb WHERE work_order_id=$1`, r.schema), id); err != nil {
			return app.ReportingRecheck{}, err
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET process_snapshot_json=$2::jsonb WHERE id=$1`, r.schema), id, merged); err != nil {
			return app.ReportingRecheck{}, err
		}
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_plan_items SET process_route_snapshot_json=$2::jsonb WHERE id=$1`, r.schema), itemID, itemMerged); err != nil {
		return app.ReportingRecheck{}, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_plans SET revision=revision+1 WHERE id=$1`, r.schema), planID); err != nil {
		return app.ReportingRecheck{}, err
	}
	if err = infra.AuditInsertTx(ctx, tx, r.schema, actor, kind, &id, "reporting_recheck", nil, nil, nil, infra.AuditMeta{"changes": changes, "plan_id": planID, "plan_item_id": itemID}); err != nil {
		return app.ReportingRecheck{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return app.ReportingRecheck{}, err
	}
	return result, nil
}
