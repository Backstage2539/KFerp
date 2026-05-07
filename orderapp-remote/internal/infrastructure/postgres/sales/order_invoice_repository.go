package sales

import (
	"context"
	"fmt"
	salesapp "orderapp/internal/application/sales"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
)

func (r Repository) LoadOrderInvoice(ctx context.Context, orderID int64) (salesapp.OrderInvoice, error) {
	q := fmt.Sprintf(`SELECT
			o.id,
			COALESCE(o.order_no,''),
			COALESCE(oi.status,''),
			COALESCE(to_char(oi.requested_at,'YYYY-MM-DD HH24:MI:SS'),''),
			COALESCE(oi.requested_by,''),
			COALESCE(to_char(oi.uploaded_at,'YYYY-MM-DD HH24:MI:SS'),''),
			COALESCE(oi.uploaded_by,''),
			COALESCE(a.id,0),
			COALESCE(a.kind,''),
			COALESCE(a.filename,''),
			COALESCE(a.content_type,''),
			COALESCE(a.bytes,0),
			COALESCE(a.sha256,''),
			COALESCE(a.object_key,''),
			COALESCE(to_char(a.created_at,'YYYY-MM-DD HH24:MI:SS'),''),
			COALESCE(a.created_by,'')
		FROM %s.orders o
		LEFT JOIN %s.order_invoices oi ON oi.order_id=o.id
		LEFT JOIN %s.sales_order_assets a ON a.id=oi.invoice_asset_id
		WHERE o.id=$1`, r.schema, r.schema, r.schema)
	var out salesapp.OrderInvoice
	var asset salesapp.SalesOrderAsset
	if err := r.pool.QueryRow(ctx, q, orderID).Scan(
		&out.OrderID,
		&out.OrderNo,
		&out.Status,
		&out.RequestedAt,
		&out.RequestedBy,
		&out.UploadedAt,
		&out.UploadedBy,
		&asset.ID,
		&asset.Kind,
		&asset.Filename,
		&asset.ContentType,
		&asset.Bytes,
		&asset.SHA256,
		&asset.ObjectKey,
		&asset.CreatedAt,
		&asset.CreatedBy,
	); err != nil {
		return salesapp.OrderInvoice{}, err
	}
	if asset.ID > 0 {
		asset.URL = salesOrderAssetURL(asset.ObjectKey)
		out.Asset = &asset
	}
	return out, nil
}

func (r Repository) RequestOrderInvoice(ctx context.Context, cmd salesapp.RequestOrderInvoiceCommand) (salesapp.OrderInvoice, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return salesapp.OrderInvoice{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var orderID int64
	q := fmt.Sprintf(`INSERT INTO %s.order_invoices(order_id, order_no, status, requested_by, updated_by)
		SELECT o.id, COALESCE(o.order_no,''), 'requested', $2, $2
		FROM %s.orders o
		WHERE o.id=$1
		ON CONFLICT(order_id) DO UPDATE SET
			order_no=excluded.order_no,
			updated_at=now(),
			updated_by=excluded.updated_by
		RETURNING order_id`, r.schema, r.schema)
	if err := tx.QueryRow(ctx, q, cmd.OrderID, cmd.Actor).Scan(&orderID); err != nil {
		if err == pgx.ErrNoRows {
			return salesapp.OrderInvoice{}, fmt.Errorf("order not found")
		}
		return salesapp.OrderInvoice{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "order_invoice", &orderID, "request", postgresinfra.StrPtr("status"), nil, postgresinfra.StrPtr("requested"), nil); err != nil {
		return salesapp.OrderInvoice{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return salesapp.OrderInvoice{}, err
	}
	return r.LoadOrderInvoice(ctx, cmd.OrderID)
}

func (r Repository) SaveOrderInvoiceFile(ctx context.Context, cmd salesapp.SaveOrderInvoiceFileCommand) (salesapp.OrderInvoice, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return salesapp.OrderInvoice{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var orderNo string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(order_no,'') FROM %s.orders WHERE id=$1`, r.schema), cmd.OrderID).Scan(&orderNo); err != nil {
		if err == pgx.ErrNoRows {
			return salesapp.OrderInvoice{}, fmt.Errorf("order not found")
		}
		return salesapp.OrderInvoice{}, err
	}

	var assetID int64
	assetQ := fmt.Sprintf(`INSERT INTO %s.sales_order_assets(kind, filename, content_type, bytes, sha256, object_key, created_by)
		VALUES('order_invoice',$1,$2,$3,$4,$5,$6)
		RETURNING id`, r.schema)
	if err := tx.QueryRow(ctx, assetQ, cmd.Filename, cmd.ContentType, cmd.Bytes, cmd.SHA256, cmd.ObjectKey, cmd.Actor).Scan(&assetID); err != nil {
		return salesapp.OrderInvoice{}, err
	}

	upsertQ := fmt.Sprintf(`INSERT INTO %s.order_invoices(order_id, order_no, status, requested_by, invoice_asset_id, uploaded_at, uploaded_by, updated_at, updated_by)
		VALUES($1,$2,'uploaded',$3,$4,now(),$3,now(),$3)
		ON CONFLICT(order_id) DO UPDATE SET
			order_no=excluded.order_no,
			status='uploaded',
			invoice_asset_id=excluded.invoice_asset_id,
			uploaded_at=now(),
			uploaded_by=excluded.uploaded_by,
			updated_at=now(),
			updated_by=excluded.updated_by`, r.schema)
	if _, err := tx.Exec(ctx, upsertQ, cmd.OrderID, orderNo, cmd.Actor, assetID); err != nil {
		return salesapp.OrderInvoice{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "order_invoice", &cmd.OrderID, "upload", postgresinfra.StrPtr("invoice_asset_id"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", assetID)), postgresinfra.AuditMeta{"filename": cmd.Filename, "content_type": cmd.ContentType, "bytes": cmd.Bytes}); err != nil {
		return salesapp.OrderInvoice{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return salesapp.OrderInvoice{}, err
	}
	return r.LoadOrderInvoice(ctx, cmd.OrderID)
}
