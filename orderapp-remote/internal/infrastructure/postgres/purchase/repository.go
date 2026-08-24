package purchase

import (
	"context"
	"fmt"
	"strings"
	"time"

	purchaseapp "orderapp/internal/application/purchase"
	stockapp "orderapp/internal/application/stock"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	stockrepo "orderapp/internal/infrastructure/postgres/stock"

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

func (r Repository) WithMaterialPurchaseLock(ctx context.Context, materialID int64, fn func(context.Context) (purchaseapp.PurchaseReceipt, error)) (purchaseapp.PurchaseReceipt, error) {
	if materialID <= 0 {
		return purchaseapp.PurchaseReceipt{}, fmt.Errorf("material required")
	}
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return purchaseapp.PurchaseReceipt{}, err
	}
	lockKey := fmt.Sprintf("%s:material-manufacture-only:%d", r.schema, materialID)
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock(hashtextextended($1,0))`, lockKey); err != nil {
		conn.Release()
		return purchaseapp.PurchaseReceipt{}, err
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var unlocked bool
		if err := conn.QueryRow(unlockCtx, `SELECT pg_advisory_unlock(hashtextextended($1,0))`, lockKey).Scan(&unlocked); err != nil || !unlocked {
			_ = conn.Conn().Close(unlockCtx)
		}
		conn.Release()
	}()
	return fn(ctx)
}

func (r Repository) ListSuppliers(ctx context.Context) ([]purchaseapp.Supplier, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id,name,contact,phone,address,active
		FROM %s.purchase_suppliers
		ORDER BY active DESC, name, id
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]purchaseapp.Supplier, 0)
	for rows.Next() {
		var row purchaseapp.Supplier
		if err := rows.Scan(&row.ID, &row.Name, &row.Contact, &row.Phone, &row.Address, &row.Active); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) SaveSupplier(ctx context.Context, cmd purchaseapp.SaveSupplierCommand) (purchaseapp.Supplier, error) {
	if cmd.ID > 0 {
		if _, err := r.pool.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.purchase_suppliers
			SET name=$2,contact=$3,phone=$4,address=$5,active=$6,updated_at=now()
			WHERE id=$1
		`, r.schema), cmd.ID, cmd.Name, cmd.Contact, cmd.Phone, cmd.Address, cmd.Active); err != nil {
			return purchaseapp.Supplier{}, err
		}
		return purchaseapp.Supplier{ID: cmd.ID, Name: cmd.Name, Contact: cmd.Contact, Phone: cmd.Phone, Address: cmd.Address, Active: cmd.Active}, nil
	}
	var id int64
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.purchase_suppliers(name,contact,phone,address,active,updated_at)
		VALUES($1,$2,$3,$4,true,now())
		ON CONFLICT (name) DO UPDATE SET
			contact=excluded.contact,
			phone=excluded.phone,
			address=excluded.address,
			active=true,
			updated_at=now()
		RETURNING id
	`, r.schema), cmd.Name, cmd.Contact, cmd.Phone, cmd.Address).Scan(&id); err != nil {
		return purchaseapp.Supplier{}, err
	}
	return purchaseapp.Supplier{ID: id, Name: cmd.Name, Contact: cmd.Contact, Phone: cmd.Phone, Address: cmd.Address, Active: true}, nil
}

func (r Repository) ListPurchaseOrders(ctx context.Context) ([]purchaseapp.PurchaseOrder, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT po.id,po.order_no,po.supplier_id,COALESCE(s.name,''),po.material_id,COALESCE(m.name,''),
		       po.qty_g,COALESCE(po.qty,0)::float8,COALESCE(po.unit_code,'kg'),po.qty_units,COALESCE(po.target_warehouse,'raw_materials'),
		       COALESCE(po.unit_cost,0),po.status,po.operator,to_char(po.created_at,'YYYY-MM-DD HH24:MI')
		FROM %s.purchase_orders po
		LEFT JOIN %s.purchase_suppliers s ON s.id=po.supplier_id
		LEFT JOIN %s.materials m ON m.id=po.material_id
		ORDER BY po.created_at DESC, po.id DESC
		LIMIT 200
	`, r.schema, r.schema, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]purchaseapp.PurchaseOrder, 0)
	for rows.Next() {
		var row purchaseapp.PurchaseOrder
		if err := rows.Scan(&row.ID, &row.OrderNo, &row.SupplierID, &row.SupplierName, &row.MaterialID, &row.MaterialName,
			&row.QtyG, &row.Qty, &row.UnitCode, &row.QtyUnits, &row.TargetWarehouse, &row.UnitCost, &row.Status, &row.Operator, &row.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) CreatePurchaseOrder(ctx context.Context, cmd purchaseapp.CreatePurchaseOrderCommand) (purchaseapp.PurchaseOrder, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return purchaseapp.PurchaseOrder{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := r.assertMaterialPurchasableTx(ctx, tx, cmd.MaterialID); err != nil {
		return purchaseapp.PurchaseOrder{}, err
	}
	unitCode, warehouse, err := r.resolvePurchaseStockIdentityTx(ctx, tx, cmd.MaterialID, cmd.UnitCode, cmd.TargetWarehouse)
	if err != nil {
		return purchaseapp.PurchaseOrder{}, err
	}
	cmd.UnitCode, cmd.TargetWarehouse = unitCode, warehouse
	if _, err := tx.Exec(ctx, fmt.Sprintf("LOCK TABLE %s.purchase_orders IN SHARE ROW EXCLUSIVE MODE", r.schema)); err != nil {
		return purchaseapp.PurchaseOrder{}, err
	}
	orderNo, err := nextNo(ctx, tx, r.schema, "purchase_orders", "order_no", "PO")
	if err != nil {
		return purchaseapp.PurchaseOrder{}, err
	}
	var out purchaseapp.PurchaseOrder
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.purchase_orders(order_no,supplier_id,material_id,qty_g,qty,unit_code,qty_units,target_warehouse,unit_cost,status,note,operator,created_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'ordered',$10,$11,now())
		RETURNING id,order_no,supplier_id,material_id,qty_g,qty::float8,unit_code,qty_units,target_warehouse,unit_cost,status,operator,to_char(created_at,'YYYY-MM-DD HH24:MI')
	`, r.schema), orderNo, cmd.SupplierID, cmd.MaterialID, cmd.QtyG, cmd.Qty, cmd.UnitCode, cmd.QtyUnits, cmd.TargetWarehouse, cmd.UnitCost, cmd.Note, cmd.Operator).Scan(
		&out.ID, &out.OrderNo, &out.SupplierID, &out.MaterialID, &out.QtyG, &out.Qty, &out.UnitCode, &out.QtyUnits, &out.TargetWarehouse, &out.UnitCost, &out.Status, &out.Operator, &out.CreatedAt,
	); err != nil {
		return purchaseapp.PurchaseOrder{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "purchase_order", &out.ID, "create", postgresinfra.StrPtr("status"), nil, postgresinfra.StrPtr("ordered"), postgresinfra.AuditMeta{
		"order_no": orderNo, "material_id": cmd.MaterialID, "qty": cmd.Qty, "qty_g": cmd.QtyG, "qty_units": cmd.QtyUnits,
		"unit_code": cmd.UnitCode, "target_warehouse": cmd.TargetWarehouse, "estimated_unit_cost": cmd.UnitCost,
	}); err != nil {
		return purchaseapp.PurchaseOrder{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return purchaseapp.PurchaseOrder{}, err
	}
	return out, nil
}

func (r Repository) ListPurchaseReceipts(ctx context.Context) ([]purchaseapp.PurchaseReceipt, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT pr.id,pr.receipt_no,pr.purchase_order_id,pr.supplier_id,pr.supplier_name,pr.material_id,COALESCE(m.name,''),
		       pr.qty_g,COALESCE(pr.qty,0)::float8,COALESCE(pr.unit_code,'kg'),pr.qty_units,COALESCE(pr.target_warehouse,'raw_materials'),
		       COALESCE(pr.unit_cost,0),pr.stock_receipt_id,pr.stock_batch_code,pr.operator,
		       to_char(pr.created_at,'YYYY-MM-DD HH24:MI'),pr.note
		FROM %s.purchase_receipts pr
		LEFT JOIN %s.materials m ON m.id=pr.material_id
		ORDER BY pr.created_at DESC, pr.id DESC
		LIMIT 200
	`, r.schema, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]purchaseapp.PurchaseReceipt, 0)
	for rows.Next() {
		var row purchaseapp.PurchaseReceipt
		if err := rows.Scan(&row.ID, &row.ReceiptNo, &row.PurchaseOrderID, &row.SupplierID, &row.SupplierName, &row.MaterialID, &row.MaterialName,
			&row.QtyG, &row.Qty, &row.UnitCode, &row.QtyUnits, &row.TargetWarehouse, &row.UnitCost, &row.StockReceiptID, &row.StockBatchCode, &row.Operator, &row.CreatedAt, &row.Note); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) CreatePurchaseReceipt(ctx context.Context, cmd purchaseapp.CreatePurchaseReceiptCommand, stockResult stockapp.MaterialReceiptResult) (purchaseapp.PurchaseReceipt, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return purchaseapp.PurchaseReceipt{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := r.assertMaterialPurchasableTx(ctx, tx, cmd.MaterialID); err != nil {
		return purchaseapp.PurchaseReceipt{}, err
	}
	unitCode, warehouse, err := r.resolvePurchaseStockIdentityTx(ctx, tx, cmd.MaterialID, cmd.UnitCode, cmd.TargetWarehouse)
	if err != nil {
		return purchaseapp.PurchaseReceipt{}, err
	}
	cmd.UnitCode, cmd.TargetWarehouse = unitCode, warehouse
	if _, err := tx.Exec(ctx, fmt.Sprintf("LOCK TABLE %s.purchase_receipts IN SHARE ROW EXCLUSIVE MODE", r.schema)); err != nil {
		return purchaseapp.PurchaseReceipt{}, err
	}
	receiptNo, err := nextNo(ctx, tx, r.schema, "purchase_receipts", "receipt_no", "PRC")
	if err != nil {
		return purchaseapp.PurchaseReceipt{}, err
	}
	var out purchaseapp.PurchaseReceipt
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.purchase_receipts(
			receipt_no,purchase_order_id,supplier_id,supplier_name,material_id,qty_g,qty,unit_code,qty_units,target_warehouse,unit_cost,
			stock_receipt_id,stock_batch_code,note,operator,created_at
		) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,now())
		RETURNING id,receipt_no,purchase_order_id,supplier_id,supplier_name,material_id,qty_g,qty::float8,unit_code,qty_units,target_warehouse,unit_cost,stock_receipt_id,stock_batch_code,operator,to_char(created_at,'YYYY-MM-DD HH24:MI'),note
	`, r.schema), receiptNo, cmd.PurchaseOrderID, cmd.SupplierID, cmd.SupplierName, cmd.MaterialID, cmd.QtyG, cmd.Qty, cmd.UnitCode, cmd.QtyUnits, cmd.TargetWarehouse, cmd.UnitCost, stockResult.ReceiptID, stockResult.BatchCode, cmd.Note, cmd.Operator).Scan(
		&out.ID, &out.ReceiptNo, &out.PurchaseOrderID, &out.SupplierID, &out.SupplierName, &out.MaterialID, &out.QtyG, &out.Qty, &out.UnitCode, &out.QtyUnits, &out.TargetWarehouse, &out.UnitCost, &out.StockReceiptID, &out.StockBatchCode, &out.Operator, &out.CreatedAt, &out.Note,
	); err != nil {
		return purchaseapp.PurchaseReceipt{}, err
	}
	if cmd.PurchaseOrderID > 0 {
		_, _ = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.purchase_orders SET status='received' WHERE id=$1`, r.schema), cmd.PurchaseOrderID)
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "purchase_receipt", &out.ID, "submit", postgresinfra.StrPtr("unit_cost"), nil, postgresinfra.StrPtr(fmt.Sprintf("%.4f", cmd.UnitCost)), postgresinfra.AuditMeta{
		"receipt_no": receiptNo, "purchase_order_id": cmd.PurchaseOrderID, "material_id": cmd.MaterialID, "batch_code": stockResult.BatchCode,
		"qty": cmd.Qty, "qty_g": cmd.QtyG, "qty_units": cmd.QtyUnits, "unit_code": cmd.UnitCode, "warehouse": cmd.TargetWarehouse,
	}); err != nil {
		return purchaseapp.PurchaseReceipt{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return purchaseapp.PurchaseReceipt{}, err
	}
	return out, nil
}

func (r Repository) CreatePurchaseReceiptAtomic(ctx context.Context, cmd purchaseapp.CreatePurchaseReceiptCommand) (purchaseapp.PurchaseReceipt, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return purchaseapp.PurchaseReceipt{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := r.assertMaterialPurchasableTx(ctx, tx, cmd.MaterialID); err != nil {
		return purchaseapp.PurchaseReceipt{}, err
	}
	if cmd.PurchaseOrderID > 0 {
		var orderMaterialID, orderSupplierID int64
		var status string
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT material_id,supplier_id,status FROM %s.purchase_orders WHERE id=$1 FOR UPDATE
		`, r.schema), cmd.PurchaseOrderID).Scan(&orderMaterialID, &orderSupplierID, &status); err != nil {
			if err == pgx.ErrNoRows {
				return purchaseapp.PurchaseReceipt{}, fmt.Errorf("purchase order not found")
			}
			return purchaseapp.PurchaseReceipt{}, err
		}
		if status != "ordered" {
			return purchaseapp.PurchaseReceipt{}, fmt.Errorf("purchase order is not awaiting receipt")
		}
		if orderMaterialID != cmd.MaterialID || (cmd.SupplierID > 0 && orderSupplierID != cmd.SupplierID) {
			return purchaseapp.PurchaseReceipt{}, fmt.Errorf("receipt identity must match purchase order")
		}
		if cmd.SupplierID <= 0 {
			cmd.SupplierID = orderSupplierID
		}
	}
	unitCode, warehouse, err := r.resolvePurchaseStockIdentityTx(ctx, tx, cmd.MaterialID, cmd.UnitCode, cmd.TargetWarehouse)
	if err != nil {
		return purchaseapp.PurchaseReceipt{}, err
	}
	cmd.UnitCode, cmd.TargetWarehouse = unitCode, warehouse
	if cmd.SupplierName == "" && cmd.SupplierID > 0 {
		_ = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(name,'') FROM %s.purchase_suppliers WHERE id=$1`, r.schema), cmd.SupplierID).Scan(&cmd.SupplierName)
	}
	var oldPurchasePrice float64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(purchase_price,0)::float8 FROM %s.materials WHERE id=$1 FOR UPDATE`, r.schema), cmd.MaterialID).Scan(&oldPurchasePrice); err != nil {
		return purchaseapp.PurchaseReceipt{}, err
	}
	stockDetail, err := stockrepo.NewRepository(r.pool, r.schema).CreateAndSubmitStockDocumentTx(ctx, tx, stockapp.StockDocumentCommand{
		Purpose: stockapp.PurposeMaterialReceipt, SourceType: "purchase_order", SourceID: cmd.PurchaseOrderID,
		Operator: cmd.Operator, Note: cmd.Note,
		Items: []stockapp.StockDocumentItemCommand{{
			MaterialID: cmd.MaterialID, ItemType: "material", InventoryUnit: cmd.UnitCode, ToWarehouse: cmd.TargetWarehouse,
			QtyG: cmd.QtyG, QtyUnits: cmd.QtyUnits, UnitCost: cmd.UnitCost, Supplier: cmd.SupplierName,
		}},
	})
	if err != nil {
		return purchaseapp.PurchaseReceipt{}, err
	}
	stockBatchID, stockBatchCode := int64(0), ""
	if len(stockDetail.Items) > 0 {
		stockBatchCode = stockDetail.Items[0].BatchCode
		if len(stockDetail.Items[0].Allocations) > 0 {
			stockBatchID = stockDetail.Items[0].Allocations[0].MaterialBatchID
			stockBatchCode = stockDetail.Items[0].Allocations[0].BatchCode
		}
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.materials SET purchase_price=$2,updated_at=now() WHERE id=$1`, r.schema), cmd.MaterialID, cmd.UnitCost); err != nil {
		return purchaseapp.PurchaseReceipt{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf("LOCK TABLE %s.purchase_receipts IN SHARE ROW EXCLUSIVE MODE", r.schema)); err != nil {
		return purchaseapp.PurchaseReceipt{}, err
	}
	receiptNo, err := nextNo(ctx, tx, r.schema, "purchase_receipts", "receipt_no", "PRC")
	if err != nil {
		return purchaseapp.PurchaseReceipt{}, err
	}
	var out purchaseapp.PurchaseReceipt
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.purchase_receipts(
			receipt_no,purchase_order_id,supplier_id,supplier_name,material_id,qty_g,qty,unit_code,qty_units,target_warehouse,unit_cost,
			stock_receipt_id,stock_batch_code,note,operator,created_at
		) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,now())
		RETURNING id,receipt_no,purchase_order_id,supplier_id,supplier_name,material_id,qty_g,qty::float8,unit_code,qty_units,target_warehouse,unit_cost,stock_receipt_id,stock_batch_code,operator,to_char(created_at,'YYYY-MM-DD HH24:MI'),note
	`, r.schema), receiptNo, cmd.PurchaseOrderID, cmd.SupplierID, cmd.SupplierName, cmd.MaterialID, cmd.QtyG, cmd.Qty, cmd.UnitCode, cmd.QtyUnits, cmd.TargetWarehouse, cmd.UnitCost,
		stockDetail.ID, stockBatchCode, cmd.Note, cmd.Operator).Scan(
		&out.ID, &out.ReceiptNo, &out.PurchaseOrderID, &out.SupplierID, &out.SupplierName, &out.MaterialID,
		&out.QtyG, &out.Qty, &out.UnitCode, &out.QtyUnits, &out.TargetWarehouse, &out.UnitCost, &out.StockReceiptID, &out.StockBatchCode, &out.Operator, &out.CreatedAt, &out.Note,
	); err != nil {
		return purchaseapp.PurchaseReceipt{}, err
	}
	if cmd.PurchaseOrderID > 0 {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.purchase_orders SET status='received' WHERE id=$1`, r.schema), cmd.PurchaseOrderID); err != nil {
			return purchaseapp.PurchaseReceipt{}, err
		}
		if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "purchase_order", &cmd.PurchaseOrderID, "receive", postgresinfra.StrPtr("status"), postgresinfra.StrPtr("ordered"), postgresinfra.StrPtr("received"), postgresinfra.AuditMeta{"receipt_no": receiptNo, "stock_entry_no": stockDetail.EntryNo}); err != nil {
			return purchaseapp.PurchaseReceipt{}, err
		}
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "purchase_receipt", &out.ID, "submit", postgresinfra.StrPtr("unit_cost"), nil, postgresinfra.StrPtr(fmt.Sprintf("%.4f", cmd.UnitCost)), postgresinfra.AuditMeta{
		"receipt_no": receiptNo, "purchase_order_id": cmd.PurchaseOrderID, "material_id": cmd.MaterialID, "material_batch_id": stockBatchID,
		"batch_code": stockBatchCode, "stock_entry_no": stockDetail.EntryNo, "qty": cmd.Qty, "qty_g": cmd.QtyG, "qty_units": cmd.QtyUnits,
		"unit_code": cmd.UnitCode, "warehouse": cmd.TargetWarehouse,
	}); err != nil {
		return purchaseapp.PurchaseReceipt{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "material", &cmd.MaterialID, "purchase_receipt_price", postgresinfra.StrPtr("purchase_price"), postgresinfra.StrPtr(fmt.Sprintf("%.4f", oldPurchasePrice)), postgresinfra.StrPtr(fmt.Sprintf("%.4f", cmd.UnitCost)), postgresinfra.AuditMeta{
		"receipt_no": receiptNo, "purchase_order_id": cmd.PurchaseOrderID, "batch_code": stockBatchCode, "warehouse": cmd.TargetWarehouse,
	}); err != nil {
		return purchaseapp.PurchaseReceipt{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return purchaseapp.PurchaseReceipt{}, err
	}
	return out, nil
}

func (r Repository) resolvePurchaseStockIdentityTx(ctx context.Context, tx pgx.Tx, materialID int64, requestedUnit, requestedWarehouse string) (string, string, error) {
	var kind, unit string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(kind,''),COALESCE(unit,'') FROM %s.materials WHERE id=$1`, r.schema), materialID).Scan(&kind, &unit); err != nil {
		if err == pgx.ErrNoRows {
			return "", "", fmt.Errorf("material not found")
		}
		return "", "", err
	}
	requestedUnit = strings.TrimSpace(requestedUnit)
	if requestedUnit != "" && !strings.EqualFold(requestedUnit, unit) {
		return "", "", fmt.Errorf("采购库存单位必须与物料档案一致：%s", unit)
	}
	warehouse := strings.TrimSpace(requestedWarehouse)
	if warehouse == "" {
		warehouse = "raw_materials"
		if strings.EqualFold(strings.TrimSpace(kind), "pack") {
			warehouse = "packaging"
		}
	}
	var warehouseKind string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(kind,'') FROM %s.warehouses WHERE code=$1 AND active=true`, r.schema), warehouse).Scan(&warehouseKind); err != nil {
		if err == pgx.ErrNoRows {
			return "", "", fmt.Errorf("target warehouse not found")
		}
		return "", "", err
	}
	if strings.EqualFold(strings.TrimSpace(warehouseKind), "finished") {
		return "", "", fmt.Errorf("采购收货不能进入成品仓")
	}
	return unit, warehouse, nil
}

func (r Repository) UpdateMaterialPurchasePrice(ctx context.Context, materialID int64, unitCost float64) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := r.assertMaterialPurchasableTx(ctx, tx, materialID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.materials SET purchase_price=$2,updated_at=now() WHERE id=$1`, r.schema), materialID, unitCost); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) assertMaterialPurchasableTx(ctx context.Context, tx pgx.Tx, materialID int64) error {
	var hasSemiFinishedColumn bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM information_schema.columns
			WHERE table_schema=$1 AND table_name='materials' AND column_name='is_semi_finished'
		)
	`, r.schema).Scan(&hasSemiFinishedColumn); err != nil {
		return err
	}
	if !hasSemiFinishedColumn {
		return nil
	}
	var isSemiFinished bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(is_semi_finished,false)
		FROM %s.materials WHERE id=$1 FOR UPDATE
	`, r.schema), materialID).Scan(&isSemiFinished); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("material not found")
		}
		return err
	}
	if isSemiFinished {
		return fmt.Errorf("半成品只能通过生产入库，不能采购或采购收货")
	}
	return nil
}

func nextNo(ctx context.Context, tx pgx.Tx, schema, table, column, prefix string) (string, error) {
	pfx := prefix + "-" + time.Now().Format("20060102") + "-"
	var maxNo int
	q := fmt.Sprintf(`
		SELECT COALESCE(MAX(CAST(right(%s,4) AS INT)), 0)
		FROM %s.%s
		WHERE %s LIKE $1
	`, column, schema, table, column)
	if err := tx.QueryRow(ctx, q, pfx+"%").Scan(&maxNo); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%04d", pfx, maxNo+1), nil
}
