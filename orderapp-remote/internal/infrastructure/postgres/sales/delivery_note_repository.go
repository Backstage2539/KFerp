package sales

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	salesapp "orderapp/internal/application/sales"
	salesdomain "orderapp/internal/domain/sales"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (r Repository) LoadDeliveryNoteContext(ctx context.Context, orderID int64) (salesapp.DeliveryNoteContext, error) {
	q := fmt.Sprintf(`SELECT o.id, COALESCE(o.order_no,''), COALESCE(ss.name,''),
			COALESCE(c.id,0), COALESCE(c.name,''),
			COALESCE(NULLIF(c.company_name,''), c.name, ''), COALESCE(NULLIF(c.company_address,''), c.address, ''),
			COALESCE(NULLIF(c.company_phone,''), c.phone, ''), COALESCE(NULLIF(o.receiver_name,''), c.contact, ''),
			COALESCE(NULLIF(o.receiver_phone,''), c.phone, ''), COALESCE(NULLIF(o.receiver_address,''), c.address, '')
		FROM %s.orders o
		LEFT JOIN %s.ship_statuses ss ON ss.id=o.ship_status_id
		LEFT JOIN %s.customers c ON c.id=o.customer_id
		WHERE o.id=$1`, r.schema, r.schema, r.schema)
	var out salesapp.DeliveryNoteContext
	if err := r.pool.QueryRow(ctx, q, orderID).Scan(
		&out.OrderID,
		&out.OrderNo,
		&out.ShipStatus,
		&out.Customer.ID,
		&out.Customer.Name,
		&out.Customer.CompanyName,
		&out.Customer.CompanyAddress,
		&out.Customer.CompanyPhone,
		&out.Customer.Contact,
		&out.Customer.Phone,
		&out.Customer.Address,
	); err != nil {
		return salesapp.DeliveryNoteContext{}, err
	}
	return out, nil
}

func (r Repository) LoadDeliveryNoteForm(ctx context.Context, orderID int64) (salesapp.DeliveryNoteForm, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return salesapp.DeliveryNoteForm{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	return r.loadDeliveryNoteFormTx(ctx, tx, orderID)
}

func (r Repository) SaveDeliveryNoteForm(ctx context.Context, cmd salesapp.SaveDeliveryNoteFormCommand) (salesapp.DeliveryNoteForm, error) {
	sourceWarehouse := strings.TrimSpace(cmd.SourceWarehouse)
	if sourceWarehouse == "" {
		sourceWarehouse = "finished_goods"
	}
	q := fmt.Sprintf(`INSERT INTO %s.delivery_note_forms(order_id, posting_date, source_warehouse, delivery_method, tracking_no, note, updated_at, updated_by)
		VALUES($1, NULLIF($2,'')::date, $3, $4, $5, $6, now(), $7)
		ON CONFLICT(order_id) DO UPDATE SET
			posting_date=excluded.posting_date,
			source_warehouse=excluded.source_warehouse,
			delivery_method=excluded.delivery_method,
			tracking_no=excluded.tracking_no,
			note=excluded.note,
			updated_at=now(),
			updated_by=excluded.updated_by`, r.schema)
	if _, err := r.pool.Exec(ctx, q, cmd.OrderID, cmd.PostingDate, sourceWarehouse, cmd.DeliveryMethod, cmd.TrackingNo, cmd.Note, cmd.Actor); err != nil {
		return salesapp.DeliveryNoteForm{}, err
	}
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, cmd.Actor, "delivery_note_form", &cmd.OrderID, "update", postgresinfra.StrPtr("order_id"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", cmd.OrderID)), postgresinfra.AuditMeta{"source_warehouse": sourceWarehouse, "tracking_no": cmd.TrackingNo})
	return r.LoadDeliveryNoteForm(ctx, cmd.OrderID)
}

func (r Repository) loadDeliveryNoteFormTx(ctx context.Context, tx pgx.Tx, orderID int64) (salesapp.DeliveryNoteForm, error) {
	base, err := r.loadDeliveryNoteBaseTx(ctx, tx, orderID)
	if err != nil {
		return salesapp.DeliveryNoteForm{}, err
	}
	form := salesapp.DeliveryNoteForm{
		OrderID:             base.OrderID,
		OrderNo:             base.OrderNo,
		PostingDate:         time.Now().Format("2006-01-02"),
		SourceWarehouse:     firstNonEmpty(base.SourceWarehouse, "finished_goods"),
		SourceWarehouseName: deliveryWarehouseDisplayName(firstNonEmpty(base.SourceWarehouse, "finished_goods")),
		DeliveryMethod:      deliveryMethodDisplayName(base.DeliveryMethod),
		TrackingNo:          base.TrackingNo,
		Note:                base.Note,
	}
	q := fmt.Sprintf(`SELECT COALESCE(to_char(posting_date,'YYYY-MM-DD'),''), COALESCE(source_warehouse,''), COALESCE(delivery_method,''), COALESCE(tracking_no,''), COALESCE(note,''), to_char(updated_at,'YYYY-MM-DD HH24:MI:SS'), updated_by
		FROM %s.delivery_note_forms WHERE order_id=$1`, r.schema)
	err = tx.QueryRow(ctx, q, orderID).Scan(&form.PostingDate, &form.SourceWarehouse, &form.DeliveryMethod, &form.TrackingNo, &form.Note, &form.UpdatedAt, &form.UpdatedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return form, nil
	}
	if err != nil {
		return salesapp.DeliveryNoteForm{}, err
	}
	if strings.TrimSpace(form.SourceWarehouse) == "" {
		form.SourceWarehouse = firstNonEmpty(base.SourceWarehouse, "finished_goods")
	}
	form.SourceWarehouseName = deliveryWarehouseDisplayName(form.SourceWarehouse)
	if strings.TrimSpace(form.PostingDate) == "" {
		form.PostingDate = time.Now().Format("2006-01-02")
	}
	form.DeliveryMethod = deliveryMethodDisplayName(form.DeliveryMethod)
	return form, nil
}

type deliveryNoteBase struct {
	OrderID                int64
	OrderNo                string
	DocumentDate           string
	OrderDate              string
	CompanyName            string
	CompanyAddress         string
	CustomerName           string
	CustomerCompanyName    string
	CustomerCompanyAddress string
	CustomerCompanyPhone   string
	ReceiverName           string
	ReceiverPhone          string
	ReceiverAddress        string
	DeliveryMethod         string
	TrackingNo             string
	SourceWarehouse        string
	Note                   string
	ShipStatus             string
}

func (r Repository) loadDeliveryNoteBaseTx(ctx context.Context, tx pgx.Tx, orderID int64) (deliveryNoteBase, error) {
	q := fmt.Sprintf(`SELECT o.id, COALESCE(o.order_no,''),
			COALESCE(to_char(o.document_date,'YYYY-MM-DD'), to_char(o.order_date,'YYYY-MM-DD'), ''),
			COALESCE(to_char(o.order_date,'YYYY-MM-DD'),''),
			COALESCE(c.name,''), COALESCE(NULLIF(c.company_name,''), c.name, ''),
			COALESCE(NULLIF(c.company_address,''), c.address, ''), COALESCE(NULLIF(c.company_phone,''), c.phone, ''),
			COALESCE(NULLIF(o.receiver_name,''), c.contact, ''), COALESCE(NULLIF(o.receiver_phone,''), c.phone, ''), COALESCE(NULLIF(o.receiver_address,''), c.address, ''),
			COALESCE(o.ship_method,''), %s, COALESCE(o.source_warehouse,''), COALESCE(o.notes,''), COALESCE(ss.name,'')
		FROM %s.orders o
		LEFT JOIN %s.customers c ON c.id=o.customer_id
		LEFT JOIN %s.ship_statuses ss ON ss.id=o.ship_status_id
		WHERE o.id=$1`, orderTrackingSummaryExpr(r.schema, "o"), r.schema, r.schema, r.schema)
	var out deliveryNoteBase
	if err := tx.QueryRow(ctx, q, orderID).Scan(
		&out.OrderID,
		&out.OrderNo,
		&out.DocumentDate,
		&out.OrderDate,
		&out.CustomerName,
		&out.CustomerCompanyName,
		&out.CustomerCompanyAddress,
		&out.CustomerCompanyPhone,
		&out.ReceiverName,
		&out.ReceiverPhone,
		&out.ReceiverAddress,
		&out.DeliveryMethod,
		&out.TrackingNo,
		&out.SourceWarehouse,
		&out.Note,
		&out.ShipStatus,
	); err != nil {
		return deliveryNoteBase{}, err
	}
	companyProfile, err := r.loadCompanyProfileForSalesOrderTx(ctx, tx)
	if err != nil {
		return deliveryNoteBase{}, err
	}
	out.CompanyName = firstNonEmpty(companyProfile.Name, out.CustomerCompanyName, out.CustomerName)
	out.CompanyAddress = companyProfile.Address
	return out, nil
}

func (r Repository) PreviewDeliveryNoteDocument(ctx context.Context, orderID int64) (salesapp.DeliveryNotePreview, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return salesapp.DeliveryNotePreview{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	existing, err := r.loadDeliveryNoteVersionsTx(ctx, tx, orderID, false)
	if err != nil {
		return salesapp.DeliveryNotePreview{}, err
	}
	form, err := r.loadDeliveryNoteFormTx(ctx, tx, orderID)
	if err != nil {
		return salesapp.DeliveryNotePreview{}, err
	}
	snapshot, err := r.buildDeliveryNoteSnapshotTx(ctx, tx, orderID, form)
	if err != nil {
		return salesapp.DeliveryNotePreview{}, err
	}
	return salesapp.DeliveryNotePreview{
		OrderID:       snapshot.OrderID,
		OrderNo:       snapshot.OrderNo,
		NextVersionNo: salesdomain.NextDeliveryNoteVersion(existing),
		Form:          form,
		Snapshot:      snapshot,
	}, nil
}

func (r Repository) PreviewDeliveryNotePDF(ctx context.Context, orderID int64) (salesapp.DeliveryNotePreviewPDF, error) {
	preview, err := r.PreviewDeliveryNoteDocument(ctx, orderID)
	if err != nil {
		return salesapp.DeliveryNotePreviewPDF{}, err
	}
	pdfBytes, err := r.deliveryNoteRenderer.RenderPreview(preview.Snapshot)
	if err != nil {
		return salesapp.DeliveryNotePreviewPDF{}, err
	}
	return salesapp.DeliveryNotePreviewPDF{
		Preview:  preview,
		Data:     pdfBytes,
		Filename: fmt.Sprintf("%s-delivery-note-preview.pdf", safeDeliveryNotePathPart(preview.OrderNo)),
	}, nil
}

func (r Repository) GenerateDeliveryNoteDocument(ctx context.Context, cmd salesapp.GenerateDeliveryNoteDocumentCommand) (salesapp.GenerateDeliveryNoteDocumentResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return salesapp.GenerateDeliveryNoteDocumentResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", cmd.OrderID); err != nil {
		return salesapp.GenerateDeliveryNoteDocumentResult{}, err
	}
	existing, err := r.loadDeliveryNoteVersionsTx(ctx, tx, cmd.OrderID, true)
	if err != nil {
		return salesapp.GenerateDeliveryNoteDocumentResult{}, err
	}
	versionNo := salesdomain.NextDeliveryNoteVersion(existing)
	form, err := r.loadDeliveryNoteFormTx(ctx, tx, cmd.OrderID)
	if err != nil {
		return salesapp.GenerateDeliveryNoteDocumentResult{}, err
	}
	snapshot, err := r.buildDeliveryNoteSnapshotTx(ctx, tx, cmd.OrderID, form)
	if err != nil {
		return salesapp.GenerateDeliveryNoteDocumentResult{}, err
	}
	pdfBytes, err := r.deliveryNoteRenderer.Render(snapshot)
	if err != nil {
		return salesapp.GenerateDeliveryNoteDocumentResult{}, err
	}
	imageBytes, err := r.deliveryNoteRenderer.RenderPNG(snapshot)
	if err != nil {
		return salesapp.GenerateDeliveryNoteDocumentResult{}, err
	}
	objectDir := filepath.ToSlash(filepath.Join("delivery_note_documents", safeDeliveryNotePathPart(snapshot.OrderNo)))
	pdfObjectKey := filepath.ToSlash(filepath.Join(objectDir, fmt.Sprintf("V%d.pdf", versionNo)))
	imageObjectKey := filepath.ToSlash(filepath.Join(objectDir, fmt.Sprintf("V%d.png", versionNo)))
	writtenObjectKeys := make([]string, 0, 2)
	committed := false
	defer func() {
		if !committed {
			for _, objectKey := range writtenObjectKeys {
				cleanupGeneratedDeliveryNoteAssetFile(r.assetDir, objectKey)
			}
		}
	}()
	if err := os.MkdirAll(filepath.Join(r.assetDir, objectDir), 0755); err != nil {
		return salesapp.GenerateDeliveryNoteDocumentResult{}, err
	}
	writtenObjectKeys = append(writtenObjectKeys, pdfObjectKey)
	if err := r.writeDeliveryNoteAssetFile(filepath.Join(r.assetDir, pdfObjectKey), pdfBytes); err != nil {
		return salesapp.GenerateDeliveryNoteDocumentResult{}, err
	}
	writtenObjectKeys = append(writtenObjectKeys, imageObjectKey)
	if err := r.writeDeliveryNoteAssetFile(filepath.Join(r.assetDir, imageObjectKey), imageBytes); err != nil {
		return salesapp.GenerateDeliveryNoteDocumentResult{}, err
	}
	pdfSum := sha256.Sum256(pdfBytes)
	imageSum := sha256.Sum256(imageBytes)
	var pdfAssetID, imageAssetID int64
	assetQ := fmt.Sprintf(`INSERT INTO %s.delivery_note_assets(kind, filename, content_type, bytes, sha256, object_key, created_by)
		VALUES($1,$2,$3,$4,$5,$6,$7)
		RETURNING id`, r.schema)
	pdfFilename := fmt.Sprintf("%s-DN-V%d.pdf", snapshot.OrderNo, versionNo)
	if err := tx.QueryRow(ctx, assetQ, "delivery_note_pdf", pdfFilename, "application/pdf", int64(len(pdfBytes)), hex.EncodeToString(pdfSum[:]), pdfObjectKey, cmd.Actor).Scan(&pdfAssetID); err != nil {
		return salesapp.GenerateDeliveryNoteDocumentResult{}, err
	}
	imageFilename := fmt.Sprintf("%s-DN-V%d.png", snapshot.OrderNo, versionNo)
	if err := tx.QueryRow(ctx, assetQ, "delivery_note_image", imageFilename, "image/png", int64(len(imageBytes)), hex.EncodeToString(imageSum[:]), imageObjectKey, cmd.Actor).Scan(&imageAssetID); err != nil {
		return salesapp.GenerateDeliveryNoteDocumentResult{}, err
	}
	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		return salesapp.GenerateDeliveryNoteDocumentResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.delivery_note_documents SET is_latest=false WHERE order_id=$1`, r.schema), cmd.OrderID); err != nil {
		return salesapp.GenerateDeliveryNoteDocumentResult{}, err
	}
	var doc salesapp.DeliveryNoteDocument
	insertQ := fmt.Sprintf(`INSERT INTO %s.delivery_note_documents(order_id, order_no, version_no, snapshot_json, pdf_asset_id, image_asset_id, is_latest, created_by)
		VALUES($1,$2,$3,$4,$5,$6,true,$7)
		RETURNING id, order_id, order_no, version_no, pdf_asset_id, image_asset_id, is_latest, to_char(created_at,'YYYY-MM-DD HH24:MI:SS'), created_by`, r.schema)
	if err := tx.QueryRow(ctx, insertQ, cmd.OrderID, snapshot.OrderNo, versionNo, snapshotJSON, pdfAssetID, imageAssetID, cmd.Actor).Scan(&doc.ID, &doc.OrderID, &doc.OrderNo, &doc.VersionNo, &doc.PDFAssetID, &doc.ImageAssetID, &doc.IsLatest, &doc.CreatedAt, &doc.CreatedBy); err != nil {
		return salesapp.GenerateDeliveryNoteDocumentResult{}, err
	}
	doc.Snapshot = snapshot
	doc.DownloadURL = deliveryNoteDocumentDownloadURL(doc.OrderID, doc.ID)
	doc.ImageDownloadURL = deliveryNoteImageDownloadURL(doc.OrderID, doc.ID)
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "delivery_note_document", &doc.ID, "create", postgresinfra.StrPtr("version_no"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", versionNo)), postgresinfra.AuditMeta{"order_id": cmd.OrderID, "order_no": snapshot.OrderNo, "pdf_asset_id": pdfAssetID, "image_asset_id": imageAssetID}); err != nil {
		return salesapp.GenerateDeliveryNoteDocumentResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return salesapp.GenerateDeliveryNoteDocumentResult{}, err
	}
	committed = true
	return salesapp.GenerateDeliveryNoteDocumentResult{Document: doc, Snapshot: snapshot}, nil
}

func (r Repository) writeDeliveryNoteAssetFile(path string, data []byte) error {
	if r.deliveryNoteAssetWriter != nil {
		return r.deliveryNoteAssetWriter(path, data)
	}
	return os.WriteFile(path, data, 0644)
}

func (r Repository) buildDeliveryNoteSnapshotTx(ctx context.Context, tx pgx.Tx, orderID int64, form salesapp.DeliveryNoteForm) (salesdomain.DeliveryNoteSnapshot, error) {
	base, err := r.loadDeliveryNoteBaseTx(ctx, tx, orderID)
	if err != nil {
		return salesdomain.DeliveryNoteSnapshot{}, err
	}
	if !isDeliveryNoteShippedStatus(base.ShipStatus) {
		return salesdomain.DeliveryNoteSnapshot{}, fmt.Errorf("order must be shipped before delivery note")
	}
	sourceWarehouse := strings.TrimSpace(form.SourceWarehouse)
	if sourceWarehouse == "" {
		sourceWarehouse = "finished_goods"
	}
	snapshot := salesdomain.DeliveryNoteSnapshot{
		OrderID:                base.OrderID,
		OrderNo:                base.OrderNo,
		DeliveryNoteNo:         "DN-" + base.OrderNo,
		DocumentDate:           firstNonEmpty(base.DocumentDate, base.OrderDate),
		OrderDate:              base.OrderDate,
		PostingDate:            firstNonEmpty(form.PostingDate, time.Now().Format("2006-01-02")),
		CompanyName:            base.CompanyName,
		CompanyAddress:         base.CompanyAddress,
		CustomerName:           base.CustomerName,
		CustomerCompanyName:    base.CustomerCompanyName,
		CustomerCompanyAddress: base.CustomerCompanyAddress,
		CustomerCompanyPhone:   base.CustomerCompanyPhone,
		ReceiverName:           base.ReceiverName,
		ReceiverPhone:          base.ReceiverPhone,
		ReceiverAddress:        base.ReceiverAddress,
		SourceWarehouse:        sourceWarehouse,
		SourceWarehouseName:    deliveryWarehouseDisplayName(sourceWarehouse),
		DeliveryMethod:         deliveryMethodDisplayName(firstNonEmpty(form.DeliveryMethod, base.DeliveryMethod)),
		TrackingNo:             firstNonEmpty(form.TrackingNo, base.TrackingNo),
		Note:                   firstNonEmpty(form.Note, base.Note),
	}
	seal, err := r.loadDeliveryNoteSealRefTx(ctx, tx)
	if err != nil {
		return salesdomain.DeliveryNoteSnapshot{}, err
	}
	snapshot.Seal = seal
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT COALESCE(NULLIF(oi.item_name,''), p.name, ''), COALESCE(oi.item_note,''), COALESCE(oi.spec,''), COALESCE(oi.qty,0)::float8, COALESCE(oi.unit,'')
		FROM %s.order_items oi
		LEFT JOIN %s.products p ON p.id=oi.product_id
		WHERE oi.order_id=$1
		ORDER BY oi.line_no, oi.id`, r.schema, r.schema), orderID)
	if err != nil {
		return salesdomain.DeliveryNoteSnapshot{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var item salesdomain.DeliveryNoteSnapshotItem
		var qty float64
		if err := rows.Scan(&item.Name, &item.Note, &item.Spec, &qty, &item.Unit); err != nil {
			return salesdomain.DeliveryNoteSnapshot{}, err
		}
		item.Qty = trimFloatZero(qty)
		item.Warehouse = snapshot.SourceWarehouse
		item.WarehouseName = snapshot.SourceWarehouseName
		snapshot.Items = append(snapshot.Items, item)
	}
	if err := rows.Err(); err != nil {
		return salesdomain.DeliveryNoteSnapshot{}, err
	}
	if err := snapshot.Validate(); err != nil {
		return salesdomain.DeliveryNoteSnapshot{}, err
	}
	return snapshot, nil
}

func (r Repository) loadDeliveryNoteSealRefTx(ctx context.Context, tx pgx.Tx) (*salesdomain.SalesOrderAssetRef, error) {
	q := fmt.Sprintf(`SELECT
			COALESCE(a.id,0), COALESCE(a.content_type,''), COALESCE(a.object_key,''),
			COALESCE(s.seal_x_mm,32)::float8, COALESCE(s.seal_y_mm,5)::float8, COALESCE(s.seal_width_mm,36)::float8
		FROM %s.sales_order_settings s
		LEFT JOIN %s.sales_order_assets a ON a.id=s.seal_asset_id
		WHERE s.id=1`, r.schema, r.schema)
	ref := salesdomain.SalesOrderAssetRef{Label: "公章"}
	err := tx.QueryRow(ctx, q).Scan(&ref.ID, &ref.ContentType, &ref.ObjectKey, &ref.XMM, &ref.YMM, &ref.WidthMM)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if ref.ID <= 0 || strings.TrimSpace(ref.ObjectKey) == "" {
		return nil, nil
	}
	ref.URL = salesOrderAssetURL(ref.ObjectKey)
	return &ref, nil
}

func (r Repository) loadDeliveryNoteVersionsTx(ctx context.Context, tx pgx.Tx, orderID int64, lock bool) ([]int, error) {
	q := fmt.Sprintf(`SELECT version_no FROM %s.delivery_note_documents WHERE order_id=$1`, r.schema)
	if lock {
		q += " FOR UPDATE"
	}
	rows, err := tx.Query(ctx, q, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	existing := make([]int, 0)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		existing = append(existing, version)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return existing, nil
}

func (r Repository) ListDeliveryNoteDocuments(ctx context.Context, orderID int64) ([]salesapp.DeliveryNoteDocument, error) {
	q := fmt.Sprintf(`SELECT id, order_id, order_no, version_no, snapshot_json, COALESCE(pdf_asset_id,0), COALESCE(image_asset_id,0), is_latest, to_char(created_at,'YYYY-MM-DD HH24:MI:SS'), created_by
		FROM %s.delivery_note_documents
		WHERE order_id=$1
		ORDER BY version_no DESC`, r.schema)
	rows, err := r.pool.Query(ctx, q, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]salesapp.DeliveryNoteDocument, 0)
	for rows.Next() {
		var doc salesapp.DeliveryNoteDocument
		var raw []byte
		if err := rows.Scan(&doc.ID, &doc.OrderID, &doc.OrderNo, &doc.VersionNo, &raw, &doc.PDFAssetID, &doc.ImageAssetID, &doc.IsLatest, &doc.CreatedAt, &doc.CreatedBy); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(raw, &doc.Snapshot)
		doc.DownloadURL = deliveryNoteDocumentDownloadURL(doc.OrderID, doc.ID)
		if doc.ImageAssetID > 0 {
			doc.ImageDownloadURL = deliveryNoteImageDownloadURL(doc.OrderID, doc.ID)
		}
		out = append(out, doc)
	}
	return out, rows.Err()
}

func (r Repository) LoadDeliveryNoteDocumentFile(ctx context.Context, orderID, documentID int64, latest bool) (salesapp.DeliveryNoteDocumentFile, error) {
	where := "d.order_id=$1 AND d.id=$2"
	args := []any{orderID, documentID}
	if latest {
		where = "d.order_id=$1 AND d.is_latest=true"
		args = []any{orderID}
	}
	q := fmt.Sprintf(`SELECT d.id, d.order_id, d.order_no, d.version_no, COALESCE(d.pdf_asset_id,0), COALESCE(d.image_asset_id,0), d.is_latest, to_char(d.created_at,'YYYY-MM-DD HH24:MI:SS'), d.created_by,
			a.object_key
		FROM %s.delivery_note_documents d
		JOIN %s.delivery_note_assets a ON a.id=d.pdf_asset_id
		WHERE %s
		ORDER BY d.version_no DESC
		LIMIT 1`, r.schema, r.schema, where)
	var doc salesapp.DeliveryNoteDocument
	var objectKey string
	if err := r.pool.QueryRow(ctx, q, args...).Scan(&doc.ID, &doc.OrderID, &doc.OrderNo, &doc.VersionNo, &doc.PDFAssetID, &doc.ImageAssetID, &doc.IsLatest, &doc.CreatedAt, &doc.CreatedBy, &objectKey); err != nil {
		return salesapp.DeliveryNoteDocumentFile{}, err
	}
	doc.DownloadURL = deliveryNoteDocumentDownloadURL(doc.OrderID, doc.ID)
	if doc.ImageAssetID > 0 {
		doc.ImageDownloadURL = deliveryNoteImageDownloadURL(doc.OrderID, doc.ID)
	}
	return salesapp.DeliveryNoteDocumentFile{
		Document: doc,
		Path:     filepath.Join(r.assetDir, objectKey),
		Filename: fmt.Sprintf("%s-DN-V%d.pdf", doc.OrderNo, doc.VersionNo),
	}, nil
}

func (r Repository) LoadDeliveryNoteImageFile(ctx context.Context, orderID, documentID int64, latest bool) (salesapp.DeliveryNoteImageFile, error) {
	where := "d.order_id=$1 AND d.id=$2"
	args := []any{orderID, documentID}
	if latest {
		where = "d.order_id=$1 AND d.is_latest=true"
		args = []any{orderID}
	}
	q := fmt.Sprintf(`SELECT d.id, d.order_id, d.order_no, d.version_no, COALESCE(d.pdf_asset_id,0), d.image_asset_id, d.is_latest, to_char(d.created_at,'YYYY-MM-DD HH24:MI:SS'), d.created_by,
			a.object_key
		FROM %s.delivery_note_documents d
		JOIN %s.delivery_note_assets a ON a.id=d.image_asset_id AND a.kind='delivery_note_image'
		WHERE %s
		ORDER BY d.version_no DESC
		LIMIT 1`, r.schema, r.schema, where)
	var doc salesapp.DeliveryNoteDocument
	var objectKey string
	if err := r.pool.QueryRow(ctx, q, args...).Scan(&doc.ID, &doc.OrderID, &doc.OrderNo, &doc.VersionNo, &doc.PDFAssetID, &doc.ImageAssetID, &doc.IsLatest, &doc.CreatedAt, &doc.CreatedBy, &objectKey); err != nil {
		return salesapp.DeliveryNoteImageFile{}, err
	}
	doc.DownloadURL = deliveryNoteDocumentDownloadURL(doc.OrderID, doc.ID)
	doc.ImageDownloadURL = deliveryNoteImageDownloadURL(doc.OrderID, doc.ID)
	return salesapp.DeliveryNoteImageFile{
		Document: doc,
		Path:     filepath.Join(r.assetDir, objectKey),
		Filename: fmt.Sprintf("%s-DN-V%d.png", doc.OrderNo, doc.VersionNo),
	}, nil
}

func isDeliveryNoteShippedStatus(status string) bool {
	status = strings.TrimSpace(status)
	return status == "已发货" || strings.EqualFold(status, "shipped")
}

func deliveryWarehouseDisplayName(code string) string {
	switch strings.TrimSpace(code) {
	case "finished_goods":
		return "成品仓"
	case "finished_shop":
		return "门店成品仓"
	case "raw_materials":
		return "原料仓"
	case "wip":
		return "WIP在制仓"
	case "packaging":
		return "包材仓"
	default:
		return strings.TrimSpace(code)
	}
}

func deliveryMethodDisplayName(code string) string {
	value := strings.TrimSpace(code)
	switch value {
	case "sf_small":
		return "顺丰发货"
	case "sf_large":
		return "顺丰大件"
	case "sf_express":
		return "顺丰标快"
	case "sf_fast":
		return "顺丰特快"
	case "sf_cold":
		return "顺丰冷运"
	default:
		return value
	}
}

func deliveryNoteDocumentDownloadURL(orderID, documentID int64) string {
	return fmt.Sprintf("/orders/%d/delivery-notes/%d.pdf", orderID, documentID)
}

func deliveryNoteImageDownloadURL(orderID, documentID int64) string {
	return fmt.Sprintf("/orders/%d/delivery-notes/%d.png", orderID, documentID)
}

func safeDeliveryNotePathPart(s string) string {
	return safeSalesOrderPathPart(s)
}

func cleanupGeneratedDeliveryNoteAssetFile(assetDir string, objectKey string) {
	cleanupGeneratedSalesOrderAssetFile(assetDir, objectKey)
}
