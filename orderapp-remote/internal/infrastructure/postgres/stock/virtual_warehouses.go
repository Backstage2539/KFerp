package stock

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	stockapp "orderapp/internal/application/stock"
	postgresinfra "orderapp/internal/infrastructure/postgres"
)

const virtualWarehouseColumns = `w.code,w.name,w.kind,w.parent_code,w.sort_order,w.is_default,w.active,w.description,w.customer_id,COALESCE(c.name,''),w.is_virtual,w.revision`

func scanVirtualWarehouse(row pgx.Row) (stockapp.WarehouseRow, error) {
	var w stockapp.WarehouseRow
	err := row.Scan(&w.Code, &w.Name, &w.Kind, &w.ParentCode, &w.SortOrder, &w.IsDefault, &w.Active, &w.Description, &w.CustomerID, &w.CustomerName, &w.IsVirtual, &w.Revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return w, stockapp.ErrVirtualWarehouseNotFound
	}
	return w, err
}

func (r Repository) loadVirtualWarehouseTx(ctx context.Context, tx pgx.Tx, code string, lock bool) (stockapp.WarehouseRow, error) {
	clause := ""
	if lock {
		clause = " FOR UPDATE OF w"
	}
	return scanVirtualWarehouse(tx.QueryRow(ctx, fmt.Sprintf(`SELECT %s FROM %s.warehouses w LEFT JOIN %s.customers c ON c.id=w.customer_id WHERE w.code=$1 AND w.is_virtual%s`, virtualWarehouseColumns, r.schema, r.schema, clause), code))
}

func (r Repository) ListVirtualWarehouses(ctx context.Context, q stockapp.VirtualWarehouseQuery) (stockapp.VirtualWarehouseResult, error) {
	result := stockapp.VirtualWarehouseResult{Rows: []stockapp.WarehouseRow{}, Limit: q.Limit, Offset: q.Offset}
	if result.Limit < 1 {
		result.Limit = 20
	}
	where := `w.is_virtual AND w.active AND ($1::bigint IS NULL OR w.customer_id=$1) AND ($2='' OR w.name ILIKE '%'||$2||'%' OR w.code ILIKE '%'||$2||'%' OR COALESCE(c.name,'') ILIKE '%'||$2||'%')`
	from := fmt.Sprintf(` FROM %s.warehouses w LEFT JOIN %s.customers c ON c.id=w.customer_id WHERE `, r.schema, r.schema)
	if err := r.pool.QueryRow(ctx, `SELECT count(*)`+from+where, q.CustomerID, q.Q).Scan(&result.Total); err != nil {
		return result, err
	}
	rows, err := r.pool.Query(ctx, `SELECT `+virtualWarehouseColumns+from+where+` ORDER BY w.sort_order,w.code LIMIT $3 OFFSET $4`, q.CustomerID, q.Q, result.Limit, result.Offset)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		w, err := scanVirtualWarehouse(rows)
		if err != nil {
			return result, err
		}
		result.Rows = append(result.Rows, w)
	}
	result.Page = result.Offset/result.Limit + 1
	result.TotalPages = (result.Total + result.Limit - 1) / result.Limit
	result.HasNext = result.Offset+len(result.Rows) < result.Total
	return result, rows.Err()
}

func (r Repository) validateVirtualWarehouseCustomerTx(ctx context.Context, tx pgx.Tx, id int64) error {
	if id == 0 {
		return nil
	}
	var exists bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.customers WHERE id=$1 AND active=true)`, r.schema), id).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("绑定客户不存在或已停用")
	}
	return nil
}

func virtualWarehouseCode() (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "virtual_" + hex.EncodeToString(b[:]), nil
}

func (r Repository) auditVirtualWarehouseTx(ctx context.Context, tx pgx.Tx, actor, action string, old, new stockapp.WarehouseRow) error {
	before, _ := json.Marshal(old)
	after, _ := json.Marshal(new)
	return postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "warehouse", nil, action, postgresinfra.StrPtr("configuration"), postgresinfra.StrPtr(string(before)), postgresinfra.StrPtr(string(after)), postgresinfra.AuditMeta{"warehouse": new.Code, "warehouse_name": new.Name, "is_virtual": true, "customer_id": new.CustomerID, "revision": new.Revision})
}

func (r Repository) CreateVirtualWarehouse(ctx context.Context, cmd stockapp.CreateVirtualWarehouseCommand) (stockapp.WarehouseRow, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return stockapp.WarehouseRow{}, err
	}
	defer tx.Rollback(ctx)
	if err := r.validateVirtualWarehouseCustomerTx(ctx, tx, cmd.CustomerID); err != nil {
		return stockapp.WarehouseRow{}, err
	}
	code, err := virtualWarehouseCode()
	if err != nil {
		return stockapp.WarehouseRow{}, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.warehouses(code,name,kind,description,customer_id,is_virtual,revision,sort_order)VALUES($1,$2,$3,$4,$5,true,1,100)`, r.schema), code, cmd.Name, cmd.Kind, cmd.Description, cmd.CustomerID); err != nil {
		return stockapp.WarehouseRow{}, err
	}
	row, err := r.loadVirtualWarehouseTx(ctx, tx, code, false)
	if err != nil {
		return row, err
	}
	if err := r.auditVirtualWarehouseTx(ctx, tx, cmd.Actor, "create", stockapp.WarehouseRow{}, row); err != nil {
		return stockapp.WarehouseRow{}, err
	}
	return row, tx.Commit(ctx)
}

func (r Repository) UpdateVirtualWarehouse(ctx context.Context, cmd stockapp.UpdateVirtualWarehouseCommand) (stockapp.WarehouseRow, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return stockapp.WarehouseRow{}, err
	}
	defer tx.Rollback(ctx)
	old, err := r.loadVirtualWarehouseTx(ctx, tx, cmd.Code, true)
	if err != nil {
		return old, err
	}
	if !old.Active {
		return old, stockapp.ErrVirtualWarehouseNotFound
	}
	if old.Revision != cmd.ExpectedRevision {
		return stockapp.WarehouseRow{}, stockapp.ErrWarehouseConflict
	}
	if err := r.validateVirtualWarehouseCustomerTx(ctx, tx, cmd.CustomerID); err != nil {
		return stockapp.WarehouseRow{}, err
	}
	if old.CustomerID != cmd.CustomerID {
		if err := r.guardVirtualWarehouseHistoryTx(ctx, tx, old.Code); err != nil {
			return stockapp.WarehouseRow{}, err
		}
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.warehouses SET name=$2,description=$3,customer_id=$4,revision=revision+1,updated_at=now() WHERE code=$1`, r.schema), old.Code, cmd.Name, cmd.Description, cmd.CustomerID); err != nil {
		return stockapp.WarehouseRow{}, err
	}
	row, err := r.loadVirtualWarehouseTx(ctx, tx, old.Code, false)
	if err != nil {
		return row, err
	}
	if err := r.auditVirtualWarehouseTx(ctx, tx, cmd.Actor, "update", old, row); err != nil {
		return stockapp.WarehouseRow{}, err
	}
	return row, tx.Commit(ctx)
}

func (r Repository) DeleteVirtualWarehouse(ctx context.Context, cmd stockapp.DeleteVirtualWarehouseCommand) (stockapp.WarehouseRow, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return stockapp.WarehouseRow{}, err
	}
	defer tx.Rollback(ctx)
	old, err := r.loadVirtualWarehouseTx(ctx, tx, cmd.Code, true)
	if err != nil {
		return old, err
	}
	if !old.Active {
		return old, stockapp.ErrVirtualWarehouseNotFound
	}
	if old.Revision != cmd.ExpectedRevision {
		return stockapp.WarehouseRow{}, stockapp.ErrWarehouseConflict
	}
	if old.IsDefault {
		return stockapp.WarehouseRow{}, fmt.Errorf("系统默认仓库不能删除")
	}
	if err := r.guardVirtualWarehouseDeleteTx(ctx, tx, old.Code); err != nil {
		return stockapp.WarehouseRow{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.warehouses SET active=false,revision=revision+1,updated_at=now() WHERE code=$1`, r.schema), old.Code); err != nil {
		return stockapp.WarehouseRow{}, err
	}
	row, err := r.loadVirtualWarehouseTx(ctx, tx, old.Code, false)
	if err != nil {
		return row, err
	}
	if err := r.auditVirtualWarehouseTx(ctx, tx, cmd.Actor, "delete", old, row); err != nil {
		return stockapp.WarehouseRow{}, err
	}
	return row, tx.Commit(ctx)
}

type virtualWarehouseReference struct{ table, reference, live, label string }

func warehouseReference(columns ...string) string {
	var terms []string
	for _, c := range columns {
		terms = append(terms, fmt.Sprintf("j->>'%s'=$1", c))
	}
	return "(" + strings.Join(terms, " OR ") + ")"
}
func warehouseNonzero(columns ...string) string {
	var terms []string
	for _, c := range columns {
		terms = append(terms, fmt.Sprintf("COALESCE((j->>'%s')::numeric,0)<>0", c))
	}
	return "(" + strings.Join(terms, " OR ") + ")"
}

// JSON projection keeps optional modules compatible without assuming their schema has been installed.
func virtualWarehouseReferences() []virtualWarehouseReference {
	ref := warehouseReference
	nonzero := warehouseNonzero
	active := `COALESCE(j->>'status','') NOT IN ('cancelled','canceled','completed','done','closed','released','consumed','inactive','void','superseded','withdrawn','received','fulfilled')`
	orderOpen := `COALESCE((to_jsonb(o)->>'is_void')::boolean,false)=false AND CASE WHEN to_jsonb(o)->>'delivery_mode'='customer_stock' THEN COALESCE(to_jsonb(o)->>'delivery_status','') NOT IN ('completed','received','已入客户仓','已完成') ELSE COALESCE(to_jsonb(o)->>'ship_status','') NOT IN ('已发货','已出库','已签收','已收货','已完成') END`
	orderReferenceOpen := `EXISTS(SELECT 1 FROM %[1]s.orders o WHERE to_jsonb(o)->>'id'=j->>'order_id' AND (` + orderOpen + `))`
	planOpen := `EXISTS(SELECT 1 FROM %[1]s.production_plans p WHERE to_jsonb(p)->>'id'=j->>'production_plan_id' AND COALESCE(to_jsonb(p)->>'status','') NOT IN ('completed','cancelled','canceled')) AND COALESCE(j->>'replan_status','')<>'withdrawn'`
	return []virtualWarehouseReference{
		{"finished_inventory", ref("warehouse"), nonzero("onhand_units", "onhand_loose_g"), "仍有成品库存"},
		{"material_batch_locations", ref("warehouse"), nonzero("qty_g", "qty_units"), "仍有原料或包材库存"},
		{"customer_inventory_items", ref("warehouse"), nonzero("qty_g", "qty_units"), "仍有客户库存"},
		{"stock_ledger_entries", ref("warehouse"), "false", "已有库存流水"},
		{"stock_adjustments", ref("warehouse"), "false", "已有盘点记录"},
		{"material_transfers", ref("from_warehouse", "to_warehouse"), "false", "已有原料转仓记录"},
		{"finished_product_transfers", ref("from_warehouse", "to_warehouse"), "false", "已有成品转仓记录"},
		{"stock_entry_items", ref("from_warehouse", "to_warehouse"), `EXISTS(SELECT 1 FROM ` + "%[1]s" + `.stock_entries e WHERE to_jsonb(e)->>'id'=j->>'stock_entry_id' AND to_jsonb(e)->>'status'='draft')`, "仍有待处理库存单据"},
		{"customer_stock_entitlements", ref("warehouse_code"), `COALESCE(j->>'status','active')='active' AND COALESCE((j->>'promised_qty')::numeric,0)>COALESCE((j->>'received_qty')::numeric,0)`, "仍有待入库客户权益"},
		{"customer_stock_entitlement_receipts", ref("warehouse_code"), "false", "已有客户入库记录"},
		{"customer_direct_ship_request_allocations", ref("warehouse_code"), active + " AND " + orderReferenceOpen, "仍有代发库存预留"},
		{"customer_direct_ship_request_orders", ref("warehouse_code"), orderReferenceOpen, "仍有代发订单引用"},
		{"order_stock_batch_allocations", ref("warehouse"), orderReferenceOpen, "仍有订单库存分配"},
		{"work_order_material_reservations", ref("source_warehouse"), active, "仍有生产物料预留"},
		{"customer_processing_material_reservations", ref("source_warehouse_code"), active, "仍有客户加工物料预留"},
		{"customer_processing_output_conversions", ref("warehouse"), "false", "已有加工入库记录"},
		{"orders", ref("delivery_warehouse_code", "source_warehouse"), strings.ReplaceAll(orderOpen, "to_jsonb(o)", "j"), "仍有未完成的订单"},
		{"work_orders", ref("target_warehouse"), `COALESCE(j->>'status','') NOT IN ('completed','done','cancelled','canceled','closed','void')`, "仍有未完成的生产工单"},
		{"production_plan_items", ref("target_warehouse"), planOpen, "仍有生产计划引用"},
		{"production_plan_component_sources", ref("source_warehouse"), `EXISTS(SELECT 1 FROM %[1]s.production_plan_items pi JOIN %[1]s.production_plans p ON p.id=pi.production_plan_id WHERE pi.id::text=j->>'production_plan_item_id' AND p.status NOT IN ('completed','cancelled','canceled') AND COALESCE(to_jsonb(pi)->>'replan_status','')<>'withdrawn')`, "仍有生产来源引用"},
		{"processing_job_request_items", ref("target_warehouse"), active, "仍有加工申请引用"},
		{"customer_processing_production_demands", ref("target_warehouse"), active, "仍有加工生产需求"},
		{"work_order_material_reservation_batches", ref("warehouse"), active, "仍有生产批次预留"},
		{"production_supply_allocations", ref("warehouse"), active, "仍有生产供料分配"},
		{"produce_running_items", ref("target_warehouse"), active, "仍有未完成的生产批次"},
		{"purchase_orders", ref("target_warehouse"), active, "仍有未完成的采购订单"},
		{"purchase_receipts", ref("target_warehouse"), "false", "已有采购入库记录"},
		{"delivery_note_forms", ref("source_warehouse"), orderReferenceOpen, "仍有待发货出库单"},
		{"stock_adjustment_batch_allocations", ref("warehouse"), "false", "已有批次调整记录"},
		{"customer_portal_profiles", ref("processing_warehouse_code"), "true", "仍被客户门户配置引用"},
		{"customer_fulfillment_settings", `EXISTS(SELECT 1 FROM jsonb_array_elements(COALESCE(j->'modes_json','[]'::jsonb)) m WHERE m->>'warehouse_code'=$1)`, `EXISTS(SELECT 1 FROM jsonb_array_elements(COALESCE(j->'modes_json','[]'::jsonb)) m WHERE m->>'warehouse_code'=$1 AND COALESCE((m->>'enabled')::boolean,false))`, "仍被启用的代发配置引用"},
	}
}

func (r Repository) virtualWarehouseReferenceExistsTx(ctx context.Context, tx pgx.Tx, code string, ref virtualWarehouseReference, history bool) (bool, error) {
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, r.schema+"."+ref.table).Scan(&exists); err != nil || !exists {
		return false, err
	}
	condition := ref.reference
	if !history {
		condition += " AND (" + strings.ReplaceAll(ref.live, "%[1]s", r.schema) + ")"
		// Order responses expose a ship_status label, but stored orders use ship_status_id.
		// Minimal legacy module schemas without the option table remain conservative.
		if strings.Contains(condition, "ship_status") {
			var statusesExist bool
			if err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, r.schema+".ship_statuses").Scan(&statusesExist); err != nil {
				return false, err
			}
			if statusesExist {
				for _, alias := range []string{"j", "to_jsonb(o)"} {
					label := fmt.Sprintf("COALESCE(%s->>'ship_status','')", alias)
					stored := fmt.Sprintf("COALESCE((SELECT ss.name FROM %s.ship_statuses ss WHERE ss.id::text=%s->>'ship_status_id' LIMIT 1),'')", r.schema, alias)
					condition = strings.ReplaceAll(condition, label, stored)
				}
			}
		}
	}
	query := fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM (SELECT to_jsonb(t) AS j FROM %s.%s t) records WHERE %s)`, r.schema, ref.table, condition)
	if err := tx.QueryRow(ctx, query, code).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (r Repository) guardVirtualWarehouseHistoryTx(ctx context.Context, tx pgx.Tx, code string) error {
	for _, ref := range virtualWarehouseReferences() {
		used, err := r.virtualWarehouseReferenceExistsTx(ctx, tx, code, ref, true)
		if err != nil {
			return err
		}
		if used {
			return fmt.Errorf("%s，不能更换或解除绑定客户；请新增仓库", ref.label)
		}
	}
	return nil
}

func (r Repository) guardVirtualWarehouseDeleteTx(ctx context.Context, tx pgx.Tx, code string) error {
	for _, ref := range virtualWarehouseReferences() {
		if ref.live == "false" {
			continue
		}
		used, err := r.virtualWarehouseReferenceExistsTx(ctx, tx, code, ref, false)
		if err != nil {
			return err
		}
		if used {
			return fmt.Errorf("%s，不能删除仓库", ref.label)
		}
	}
	return r.guardVirtualWarehouseRemainingBatchesTx(ctx, tx, code)
}

func (r Repository) guardVirtualWarehouseRemainingBatchesTx(ctx context.Context, tx pgx.Tx, code string) error {
	var present bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL AND to_regclass($2) IS NOT NULL`, r.schema+".stock_batches", r.schema+".stock_ledger_entries").Scan(&present); err != nil || !present {
		return err
	}
	var used bool
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.stock_batches b CROSS JOIN LATERAL(SELECT to_jsonb(l) AS entry FROM %s.stock_ledger_entries l WHERE to_jsonb(l)->>'source_batch_code'=to_jsonb(b)->>'batch_code' ORDER BY COALESCE((to_jsonb(l)->>'id')::bigint,0) DESC LIMIT 1) last WHERE last.entry->>'warehouse'=$1 AND (COALESCE((to_jsonb(b)->>'remaining_g')::numeric,0)<>0 OR COALESCE((to_jsonb(b)->>'remaining_units')::numeric,0)<>0))`, r.schema, r.schema), code).Scan(&used)
	if err != nil {
		return err
	}
	if used {
		return fmt.Errorf("仍有批次库存，不能删除仓库")
	}
	return nil
}
