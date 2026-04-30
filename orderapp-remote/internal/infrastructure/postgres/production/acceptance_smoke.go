package production

import (
	"context"
	"fmt"

	productionapp "orderapp/internal/application/production"
)

func (r Repository) AcceptanceSmoke(ctx context.Context) (productionapp.AcceptanceSmokeResult, error) {
	rows := []productionapp.AcceptanceSmokeRow{
		r.acceptanceCount(ctx, "warehouses", "仓库已配置", "warehouseInventory", fmt.Sprintf(`SELECT COUNT(*) FROM %s.warehouses WHERE active=true`, r.schema)),
		r.acceptanceCount(ctx, "raw_stock", "原料仓有批次库存", "warehouseInventory", fmt.Sprintf(`SELECT COUNT(*) FROM %s.material_batch_locations WHERE warehouse='raw_materials' AND qty_g > 0`, r.schema)),
		r.acceptanceCount(ctx, "wip_stock", "WIP有可用原料", "warehouseInventory", fmt.Sprintf(`SELECT COUNT(*) FROM %s.material_batch_locations WHERE warehouse='wip' AND qty_g > 0`, r.schema)),
		r.acceptanceCount(ctx, "work_orders", "生产工单已生成", "workOrders", fmt.Sprintf(`SELECT COUNT(*) FROM %s.work_orders`, r.schema)),
		r.acceptanceCount(ctx, "open_reservations", "WIP占用可查看", "workOrders", fmt.Sprintf(`SELECT COUNT(*) FROM %s.work_order_material_reservations WHERE status='reserved'`, r.schema)),
		r.acceptanceCount(ctx, "production_logs", "已有生产日志", "produceLogs", fmt.Sprintf(`SELECT COUNT(*) FROM %s.production_logs`, r.schema)),
		r.acceptanceCount(ctx, "quality_inspections", "已有质检记录", "qualityInspections", fmt.Sprintf(`SELECT COUNT(*) FROM %s.quality_inspections`, r.schema)),
		r.acceptanceCount(ctx, "finished_trace", "已有可追溯成品批次", "warehouseInventory", fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_batches WHERE item_type='finished_product' AND batch_code LIKE 'FP-%%'`, r.schema)),
	}
	return productionapp.AcceptanceSmokeResult{Rows: rows}, nil
}

func (r Repository) acceptanceCount(ctx context.Context, code, title, view, query string) productionapp.AcceptanceSmokeRow {
	var count int64
	if err := r.pool.QueryRow(ctx, query).Scan(&count); err != nil {
		return productionapp.AcceptanceSmokeRow{Code: code, Title: title, Status: "error", Detail: err.Error(), View: view}
	}
	status := "todo"
	detail := "暂无数据"
	if count > 0 {
		status = "ok"
		detail = "已具备"
	}
	return productionapp.AcceptanceSmokeRow{Code: code, Title: title, Status: status, Count: count, Detail: detail, View: view}
}
