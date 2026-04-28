package production

import (
	"context"
	"fmt"
	"strings"

	productionapp "orderapp/internal/application/production"
	catalogdomain "orderapp/internal/domain/catalog"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool   *pgxpool.Pool
	schema string
}

func NewRepository(pool *pgxpool.Pool, schema string) Repository {
	return Repository{pool: pool, schema: schema}
}

func (r Repository) CreateBatch(ctx context.Context, cmd productionapp.CreateBatchCommand) (productionapp.CreateBatchResult, error) {
	res, err := createProduceBatchFromOrders(ctx, r.pool, r.schema, cmd.OrderIDs, cmd.Operator, cmd.IdempotencyKey, cmd.RequestUnitsByItemID)
	if err != nil {
		return productionapp.CreateBatchResult{}, err
	}
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, cmd.Operator, "produce_batch", nil, "create", postgresinfra.StrPtr("batch_id"), nil, postgresinfra.StrPtr(res.BatchID), postgresinfra.AuditMeta{"batch_id": res.BatchID, "order_count": res.OrderCount})
	return productionapp.CreateBatchResult{
		BatchID:    res.BatchID,
		OrderCount: res.OrderCount,
		Summary:    productionSummaryToApp(res.Summary),
	}, nil
}

func (r Repository) ListBatches(ctx context.Context, cmd productionapp.ListBatchesCommand) ([]productionapp.BatchListItem, error) {
	args := []any{}
	where := " WHERE 1=1"
	if cmd.Status != "" {
		args = append(args, cmd.Status)
		where += fmt.Sprintf(" AND b.status=$%d", len(args))
	}
	if cmd.Operator != "" {
		args = append(args, cmd.Operator)
		where += fmt.Sprintf(" AND b.operator=$%d", len(args))
	}
	if cmd.From != "" {
		args = append(args, cmd.From)
		where += fmt.Sprintf(" AND b.created_at >= $%d::date", len(args))
	}
	if cmd.To != "" {
		args = append(args, cmd.To)
		where += fmt.Sprintf(" AND b.created_at < ($%d::date + INTERVAL '1 day')", len(args))
	}
	args = append(args, cmd.Limit)
	limitArg := len(args)

	q := fmt.Sprintf(`
		SELECT b.batch_id, b.status, b.operator, to_char(b.created_at,'YYYY-MM-DD HH24:MI:SS'),
		       COALESCE((SELECT COUNT(DISTINCT x.order_id) FROM %s.produce_batch_order_items x WHERE x.batch_id=b.batch_id),0),
		       CASE
		         WHEN l.cnt IS NULL THEN 'none'
		         WHEN l.total_gap_g = 0 THEN 'done'
		         ELSE 'partial'
		       END AS deduct_status,
		       COALESCE(to_char(l.last_deducted_at,'YYYY-MM-DD HH24:MI:SS'),'') AS deducted_at,
		       COALESCE(i.total_need_g,0) AS need_g,
		       COALESCE(l.total_deducted_g,0) AS deducted_g,
		       GREATEST(0, COALESCE(i.total_need_g,0) - COALESCE(l.total_deducted_g,0)) AS gap_g
		FROM %s.produce_batches b
		LEFT JOIN (
		  SELECT batch_id, SUM(need_g)::bigint AS total_need_g
		  FROM %s.produce_batch_items
		  GROUP BY batch_id
		) i ON i.batch_id=b.batch_id
		LEFT JOIN (
		  SELECT batch_id, COUNT(*) AS cnt, SUM(gap_g)::bigint AS total_gap_g, SUM(deducted_g)::bigint AS total_deducted_g, MAX(created_at) AS last_deducted_at
		  FROM %s.finished_allocation_logs
		  GROUP BY batch_id
		) l ON l.batch_id=b.batch_id
		%s
		ORDER BY b.created_at DESC
		LIMIT $%d
	`, r.schema, r.schema, r.schema, r.schema, where, limitArg)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]productionapp.BatchListItem, 0)
	for rows.Next() {
		var item productionapp.BatchListItem
		if err := rows.Scan(&item.BatchID, &item.Status, &item.Operator, &item.CreatedAt, &item.OrderCount, &item.DeductStatus, &item.DeductedAt, &item.NeedG, &item.DeductedG, &item.GapG); err != nil {
			return nil, err
		}
		item.CreatedBy = item.Operator
		item.CreatedTime = item.CreatedAt
		item.StatusChangedAt = item.CreatedAt
		if strings.TrimSpace(item.DeductedAt) != "" {
			item.StatusChangedAt = item.DeductedAt
		}
		item.StatusText = item.Status
		item.CreateTime = item.CreatedAt
		item.DeductTime = item.DeductedAt
		item.DeductState = item.DeductStatus
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r Repository) Detail(ctx context.Context, batchID string) (productionapp.BatchDetail, error) {
	var d productionapp.BatchDetail
	err := r.pool.QueryRow(ctx,
		"SELECT batch_id,status,operator,to_char(created_at,'YYYY-MM-DD HH24:MI:SS') FROM "+r.schema+".produce_batches WHERE batch_id=$1",
		batchID,
	).Scan(&d.BatchID, &d.Status, &d.Operator, &d.CreatedAt)
	if err != nil {
		return productionapp.BatchDetail{}, fmt.Errorf("batch not found")
	}

	orows, err := r.pool.Query(ctx, "SELECT DISTINCT order_id FROM "+r.schema+".produce_batch_order_items WHERE batch_id=$1 ORDER BY order_id", batchID)
	if err != nil {
		return productionapp.BatchDetail{}, err
	}
	defer orows.Close()
	d.Orders = make([]int64, 0)
	for orows.Next() {
		var orderID int64
		if err := orows.Scan(&orderID); err != nil {
			return productionapp.BatchDetail{}, err
		}
		d.Orders = append(d.Orders, orderID)
	}
	if err := orows.Err(); err != nil {
		return productionapp.BatchDetail{}, err
	}

	srows, err := r.pool.Query(ctx,
		"SELECT i.product_id,COALESCE((SELECT name FROM "+r.schema+".products p WHERE p.id=i.product_id),''),i.spec_g,i.need_units,i.need_g,COALESCE(l.deducted_g,0),COALESCE(l.gap_g,0) FROM "+r.schema+".produce_batch_items i LEFT JOIN (SELECT product_id,spec_g,SUM(deducted_g)::bigint AS deducted_g,SUM(gap_g)::bigint AS gap_g FROM "+r.schema+".finished_allocation_logs WHERE batch_id=$1 GROUP BY product_id,spec_g) l ON l.product_id=i.product_id AND l.spec_g=i.spec_g WHERE i.batch_id=$1 ORDER BY i.product_id,i.spec_g", batchID)
	if err != nil {
		return productionapp.BatchDetail{}, err
	}
	defer srows.Close()
	d.Summary = make([]productionapp.SummaryItem, 0)
	for srows.Next() {
		var item productionapp.SummaryItem
		if err := srows.Scan(&item.ProductID, &item.ProductName, &item.SpecG, &item.NeedUnits, &item.NeedG, &item.DeductedG, &item.GapG); err != nil {
			return productionapp.BatchDetail{}, err
		}
		d.Summary = append(d.Summary, item)
	}
	if err := srows.Err(); err != nil {
		return productionapp.BatchDetail{}, err
	}

	d.CreatedBy = d.Operator
	d.CreatedTime = d.CreatedAt
	d.StatusSource = "produce_batches.status"
	return d, nil
}

func (r Repository) PreviewDeduct(ctx context.Context, batchID string) (productionapp.DeductPreview, error) {
	batchID = strings.TrimSpace(batchID)
	if batchID == "" {
		return productionapp.DeductPreview{}, fmt.Errorf("batch_id required")
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return productionapp.DeductPreview{}, err
	}
	defer tx.Rollback(ctx)

	var exists int
	if err := tx.QueryRow(ctx, "SELECT 1 FROM "+r.schema+".produce_batches WHERE batch_id=$1 FOR UPDATE", batchID).Scan(&exists); err != nil {
		return productionapp.DeductPreview{}, fmt.Errorf("batch not found")
	}

	rows, err := tx.Query(ctx,
		"SELECT i.product_id,COALESCE((SELECT name FROM "+r.schema+".products p WHERE p.id=i.product_id),''),i.spec_g,i.need_units,i.need_g,COALESCE(fi.onhand_units,0),COALESCE(fi.onhand_loose_g,0) FROM "+r.schema+".produce_batch_items i LEFT JOIN LATERAL (SELECT onhand_units,onhand_loose_g FROM "+r.schema+".finished_inventory f WHERE f.product_id=i.product_id AND f.spec_g=i.spec_g AND f.warehouse='finished_goods' FOR UPDATE) fi ON true WHERE i.batch_id=$1 ORDER BY i.product_id,i.spec_g FOR UPDATE OF i", batchID)
	if err != nil {
		return productionapp.DeductPreview{}, err
	}
	defer rows.Close()

	out := productionapp.DeductPreview{BatchID: batchID, Summary: make([]productionapp.DeductPreviewItem, 0)}
	for rows.Next() {
		var s productionapp.DeductPreviewItem
		if err := rows.Scan(&s.ProductID, &s.ProductName, &s.SpecG, &s.NeedUnits, &s.NeedG, &s.InvUnits, &s.InvLooseG); err != nil {
			return productionapp.DeductPreview{}, err
		}
		totalG, err := invTotalG(s.SpecG, InvQty{Units: s.InvUnits, LooseG: s.InvLooseG})
		if err != nil {
			return productionapp.DeductPreview{}, err
		}
		s.InvTotalG = totalG
		_, deductedG, gapG, err := invDeduct(s.SpecG, InvQty{Units: s.InvUnits, LooseG: s.InvLooseG}, s.NeedG)
		if err != nil {
			return productionapp.DeductPreview{}, err
		}
		s.DeductedG = deductedG
		s.GapG = gapG
		if s.DeductedG > 0 && (s.InvTotalG-s.DeductedG) < s.SpecG {
			s.WarningLowStock = true
		}
		out.Summary = append(out.Summary, s)
	}
	if err := rows.Err(); err != nil {
		return productionapp.DeductPreview{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return productionapp.DeductPreview{}, err
	}
	return out, nil
}

func (r Repository) ConfirmDeduct(ctx context.Context, batchID, operator string) (productionapp.DeductConfirmResult, error) {
	summary, err := confirmProduceBatchDeduct(ctx, r.pool, r.schema, batchID, operator)
	if err != nil {
		return productionapp.DeductConfirmResult{}, err
	}
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, operator, "produce_batch", nil, "update", postgresinfra.StrPtr("deduct_status"), nil, postgresinfra.StrPtr("deducted"), postgresinfra.AuditMeta{"batch_id": strings.TrimSpace(batchID), "summary_count": len(summary)})
	return productionapp.DeductConfirmResult{BatchID: strings.TrimSpace(batchID), Status: "deducted", Summary: productionSummaryToApp(summary)}, nil
}

func (r Repository) ListRunning(ctx context.Context) ([]productionapp.RunningItem, error) {
	rows, err := listRunningItems(ctx, r.pool, r.schema)
	if err != nil {
		return nil, err
	}
	return productionRunningToApp(rows), nil
}

func (r Repository) ListStartNeeds(ctx context.Context, cmd productionapp.StartCommand) ([]productionapp.StartNeed, error) {
	rows, err := fetchUnproducedNeeds(ctx, r.pool, r.schema, cmd.From, cmd.To, cmd.CustomerID)
	if err != nil {
		return nil, err
	}
	return startNeedsToApp(rows), nil
}

func (r Repository) Start(ctx context.Context, cmd productionapp.StartExecutionCommand) (productionapp.StartResult, error) {
	batchID := newBatchID()
	if len(cmd.Needs) == 0 {
		return productionapp.StartResult{BatchID: batchID}, nil
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return productionapp.StartResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	yieldByProductID, err := loadProductYieldRateMapTx(ctx, tx, r.schema)
	if err != nil {
		return productionapp.StartResult{}, err
	}
	for _, need := range cmd.Needs {
		if need.GapG <= 0 {
			continue
		}
		var units, loose int64
		row := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT onhand_units, onhand_loose_g
			FROM %s.finished_inventory
			WHERE product_id=$1 AND spec_g=$2 AND warehouse='finished_goods'
			FOR UPDATE
		`, r.schema), need.ProductID, need.SpecG)
		if scanErr := row.Scan(&units, &loose); scanErr != nil {
			if scanErr == pgx.ErrNoRows {
				units, loose = 0, 0
			} else {
				return productionapp.StartResult{}, scanErr
			}
		}
		remain, deductedG, gapG, err := invDeduct(need.SpecG, InvQty{Units: units, LooseG: loose}, need.GapG)
		if err != nil {
			return productionapp.StartResult{}, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g,updated_at)
			VALUES($1,$2,'finished_goods',$3,$4,now())
			ON CONFLICT (product_id,spec_g,warehouse) DO UPDATE
			SET onhand_units=excluded.onhand_units, onhand_loose_g=excluded.onhand_loose_g, updated_at=now()
		`, r.schema), need.ProductID, need.SpecG, remain.Units, remain.LooseG); err != nil {
			return productionapp.StartResult{}, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.finished_allocation_logs(batch_id,product_id,spec_g,need_g,deducted_g,gap_g,operator)
			VALUES($1,$2,$3,$4,$5,$6,$7)
		`, r.schema), batchID, need.ProductID, need.SpecG, need.GapG, deductedG, gapG, cmd.Operator); err != nil {
			return productionapp.StartResult{}, err
		}

		key := fmt.Sprintf("%d-%d", need.ProductID, need.SpecG)
		inputG := cmd.InputByKey[key]
		yieldRate := normalizeYieldRate(yieldByProductID[need.ProductID])
		if inputG <= 0 {
			inputG = defaultProductionInputG(need.GapG, yieldRate)
		}
		plan := runningInventoryPlan(need.SpecG, need.GapG, inputG, yieldRate)
		snapshotRun := ProduceRunRow{
			Product:      need.ProductName,
			ProductID:    need.ProductID,
			SpecG:        need.SpecG,
			NeedG:        need.GapG,
			InputG:       inputG,
			BomYieldRate: yieldRate,
			PlanUnits:    plan.Units,
			PlanLooseG:   plan.LooseG,
		}
		materialSnapshot, err := buildMaterialSnapshotForRunningItemTx(ctx, tx, r.schema, snapshotRun)
		if err != nil {
			return productionapp.StartResult{}, err
		}
		if err := ensureWIPStockForRunningItemTx(ctx, tx, r.schema, snapshotRun, materialSnapshot); err != nil {
			return productionapp.StartResult{}, err
		}
		snapshotRun.MaterialSnapshot = strings.TrimSpace(string(materialSnapshot))
		reservationNeeds, _, err := materialSnapshotNeedsTx(snapshotRun, InvQty{Units: plan.Units, LooseG: plan.LooseG})
		if err != nil {
			return productionapp.StartResult{}, err
		}
		var runningItemID int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.produce_running_items(batch_id,product_id,product_name,spec_g,need_g,order_nos,status,started_by,started_at,input_g,bom_yield_rate,planned_units,planned_loose_g,material_snapshot) VALUES($1,$2,$3,$4,$5,$6,'running',$7,now(),$8,$9,$10,$11,$12) RETURNING id`, r.schema), batchID, need.ProductID, need.ProductName, need.SpecG, need.GapG, need.OrderNos, cmd.Operator, inputG, yieldRate, plan.Units, plan.LooseG, materialSnapshot).Scan(&runningItemID); err != nil {
			return productionapp.StartResult{}, err
		}
		workOrderID, err := createWorkOrderForRunningItemTx(ctx, tx, r.schema, runningItemID, batchID, need.ProductID, need.ProductName, need.SpecG, inputG, materialSnapshot, cmd.Operator)
		if err != nil {
			return productionapp.StartResult{}, err
		}
		if err := createMaterialReservationsForRunningItemTx(ctx, tx, r.schema, workOrderID, runningItemID, reservationNeeds); err != nil {
			return productionapp.StartResult{}, err
		}
	}
	if err := setOrdersProcessStatusByNeedsTx(ctx, tx, r.schema, startNeedsFromApp(cmd.Needs), "生产中"); err != nil {
		return productionapp.StartResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return productionapp.StartResult{}, err
	}
	return productionapp.StartResult{BatchID: batchID}, nil
}

func productionSummaryToApp(items []ProduceBatchSummaryItem) []productionapp.SummaryItem {
	out := make([]productionapp.SummaryItem, 0, len(items))
	for _, it := range items {
		out = append(out, productionapp.SummaryItem{
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

func loadProductYieldRateMapTx(ctx context.Context, tx pgx.Tx, schema string) (map[int64]float64, error) {
	rows, err := tx.Query(ctx, `
		SELECT p.id, COALESCE(p.roast_level,''), COALESCE(b.yield_rate,0.8)
		FROM `+schema+`.products p
		LEFT JOIN `+schema+`.product_bom b ON b.product_id=p.id
		WHERE p.active=true`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[int64]float64{}
	for rows.Next() {
		var productID int64
		var roastLevel string
		var yieldRate float64
		if err := rows.Scan(&productID, &roastLevel, &yieldRate); err != nil {
			return nil, err
		}
		out[productID] = catalogdomain.ResolveYieldRate(roastLevel, yieldRate)
	}
	return out, rows.Err()
}
