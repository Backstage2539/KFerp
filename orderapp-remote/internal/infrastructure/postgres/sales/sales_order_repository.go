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

	"github.com/jackc/pgx/v5"
)

func (r Repository) LoadSalesOrderSettings(ctx context.Context) (salesapp.SalesOrderSettings, error) {
	var settings salesapp.SalesOrderSettings
	q := fmt.Sprintf(`SELECT s.company_name, s.note, s.payment_text,
			COALESCE(a.id,0), COALESCE(a.kind,''), COALESCE(a.filename,''), COALESCE(a.content_type,''), COALESCE(a.bytes,0), COALESCE(a.sha256,''), COALESCE(a.object_key,''), COALESCE(to_char(a.created_at,'YYYY-MM-DD HH24:MI:SS'),''), COALESCE(a.created_by,'')
		FROM %s.sales_order_settings s
		LEFT JOIN %s.sales_order_assets a ON a.id=s.seal_asset_id
		WHERE s.id=1`, r.schema, r.schema)
	var seal salesapp.SalesOrderAsset
	err := r.pool.QueryRow(ctx, q).Scan(&settings.CompanyName, &settings.Note, &settings.PaymentText, &seal.ID, &seal.Kind, &seal.Filename, &seal.ContentType, &seal.Bytes, &seal.SHA256, &seal.ObjectKey, &seal.CreatedAt, &seal.CreatedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return salesapp.SalesOrderSettings{}, nil
	}
	if err != nil {
		return salesapp.SalesOrderSettings{}, err
	}
	if seal.ID > 0 {
		seal.URL = salesOrderAssetURL(seal.ObjectKey)
		settings.Seal = &seal
	}
	codes, err := r.loadSalesOrderPaymentCodes(ctx)
	if err != nil {
		return salesapp.SalesOrderSettings{}, err
	}
	settings.PaymentCodes = codes
	return settings, nil
}

func (r Repository) SaveSalesOrderSettings(ctx context.Context, cmd salesapp.SaveSalesOrderSettingsCommand) error {
	q := fmt.Sprintf(`INSERT INTO %s.sales_order_settings(id, company_name, note, payment_text, updated_at, updated_by)
		VALUES(1,$1,$2,$3,now(),$4)
		ON CONFLICT(id) DO UPDATE SET
			company_name=excluded.company_name,
			note=excluded.note,
			payment_text=excluded.payment_text,
			updated_at=now(),
			updated_by=excluded.updated_by`, r.schema)
	_, err := r.pool.Exec(ctx, q, cmd.CompanyName, cmd.Note, cmd.PaymentText, cmd.Actor)
	if err == nil {
		postgresinfra.AuditInsert(ctx, r.pool, r.schema, cmd.Actor, "sales_order_settings", nil, "update", postgresinfra.StrPtr("settings"), nil, postgresinfra.StrPtr(cmd.CompanyName), postgresinfra.AuditMeta{"company_name": cmd.CompanyName})
	}
	return err
}

func (r Repository) SaveSalesOrderAsset(ctx context.Context, cmd salesapp.SaveSalesOrderAssetCommand) (salesapp.SalesOrderAsset, error) {
	var asset salesapp.SalesOrderAsset
	q := fmt.Sprintf(`INSERT INTO %s.sales_order_assets(kind, filename, content_type, bytes, sha256, object_key, created_by)
		VALUES($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, kind, filename, content_type, bytes, sha256, object_key, to_char(created_at,'YYYY-MM-DD HH24:MI:SS'), created_by`, r.schema)
	if err := r.pool.QueryRow(ctx, q, cmd.Kind, cmd.Filename, cmd.ContentType, cmd.Bytes, cmd.SHA256, cmd.ObjectKey, cmd.Actor).Scan(&asset.ID, &asset.Kind, &asset.Filename, &asset.ContentType, &asset.Bytes, &asset.SHA256, &asset.ObjectKey, &asset.CreatedAt, &asset.CreatedBy); err != nil {
		return salesapp.SalesOrderAsset{}, err
	}
	asset.URL = salesOrderAssetURL(asset.ObjectKey)
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, cmd.Actor, "sales_order_asset", &asset.ID, "create", postgresinfra.StrPtr("kind"), nil, postgresinfra.StrPtr(cmd.Kind), postgresinfra.AuditMeta{"object_key": asset.ObjectKey, "bytes": asset.Bytes})
	return asset, nil
}

func (r Repository) SaveSalesOrderPaymentCode(ctx context.Context, cmd salesapp.SaveSalesOrderPaymentCodeCommand) (salesapp.SalesOrderPaymentCode, error) {
	var code salesapp.SalesOrderPaymentCode
	if cmd.ID > 0 {
		q := fmt.Sprintf(`UPDATE %s.sales_order_payment_codes
			SET label=$2, description=$3, asset_id=$4, sort=$5, active=$6, updated_at=now()
			WHERE id=$1
			RETURNING id, label, description, asset_id, sort, active`, r.schema)
		if err := r.pool.QueryRow(ctx, q, cmd.ID, cmd.Label, cmd.Description, cmd.AssetID, cmd.Sort, cmd.Active).Scan(&code.ID, &code.Label, &code.Description, &code.AssetID, &code.Sort, &code.Active); err != nil {
			return salesapp.SalesOrderPaymentCode{}, err
		}
	} else {
		q := fmt.Sprintf(`INSERT INTO %s.sales_order_payment_codes(label, description, asset_id, sort, active)
			VALUES($1,$2,$3,$4,$5)
			RETURNING id, label, description, asset_id, sort, active`, r.schema)
		if err := r.pool.QueryRow(ctx, q, cmd.Label, cmd.Description, cmd.AssetID, cmd.Sort, cmd.Active).Scan(&code.ID, &code.Label, &code.Description, &code.AssetID, &code.Sort, &code.Active); err != nil {
			return salesapp.SalesOrderPaymentCode{}, err
		}
	}
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, cmd.Actor, "sales_order_payment_code", &code.ID, "update", postgresinfra.StrPtr("label"), nil, postgresinfra.StrPtr(code.Label), postgresinfra.AuditMeta{"asset_id": code.AssetID})
	return code, nil
}

func (r Repository) DeleteSalesOrderPaymentCode(ctx context.Context, id int64, actor string) error {
	q := fmt.Sprintf(`UPDATE %s.sales_order_payment_codes SET active=false, updated_at=now() WHERE id=$1`, r.schema)
	_, err := r.pool.Exec(ctx, q, id)
	if err == nil {
		postgresinfra.AuditInsert(ctx, r.pool, r.schema, actor, "sales_order_payment_code", &id, "delete", postgresinfra.StrPtr("active"), postgresinfra.StrPtr("true"), postgresinfra.StrPtr("false"), nil)
	}
	return err
}

func (r Repository) SetSalesOrderSealAsset(ctx context.Context, assetID int64, actor string) error {
	q := fmt.Sprintf(`INSERT INTO %s.sales_order_settings(id, seal_asset_id, updated_at, updated_by)
		VALUES(1,$1,now(),$2)
		ON CONFLICT(id) DO UPDATE SET seal_asset_id=excluded.seal_asset_id, updated_at=now(), updated_by=excluded.updated_by`, r.schema)
	_, err := r.pool.Exec(ctx, q, assetID, actor)
	if err == nil {
		postgresinfra.AuditInsert(ctx, r.pool, r.schema, actor, "sales_order_settings", nil, "update", postgresinfra.StrPtr("seal_asset_id"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", assetID)), nil)
	}
	return err
}

func (r Repository) loadSalesOrderPaymentCodes(ctx context.Context) ([]salesapp.SalesOrderPaymentCode, error) {
	q := fmt.Sprintf(`SELECT pc.id, pc.label, pc.description, pc.asset_id, pc.sort, pc.active,
			a.id, a.kind, a.filename, a.content_type, a.bytes, a.sha256, a.object_key, to_char(a.created_at,'YYYY-MM-DD HH24:MI:SS'), a.created_by
		FROM %s.sales_order_payment_codes pc
		JOIN %s.sales_order_assets a ON a.id=pc.asset_id
		WHERE pc.active=true
		ORDER BY pc.sort, pc.id`, r.schema, r.schema)
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]salesapp.SalesOrderPaymentCode, 0)
	for rows.Next() {
		var code salesapp.SalesOrderPaymentCode
		if err := rows.Scan(&code.ID, &code.Label, &code.Description, &code.AssetID, &code.Sort, &code.Active, &code.Asset.ID, &code.Asset.Kind, &code.Asset.Filename, &code.Asset.ContentType, &code.Asset.Bytes, &code.Asset.SHA256, &code.Asset.ObjectKey, &code.Asset.CreatedAt, &code.Asset.CreatedBy); err != nil {
			return nil, err
		}
		code.Asset.URL = salesOrderAssetURL(code.Asset.ObjectKey)
		out = append(out, code)
	}
	return out, rows.Err()
}

func salesOrderAssetURL(objectKey string) string {
	if objectKey == "" {
		return ""
	}
	return "/assets/" + objectKey
}

func (r Repository) LoadSalesOrderContext(ctx context.Context, orderID int64) (salesapp.SalesOrderContext, error) {
	q := fmt.Sprintf(`SELECT o.id, COALESCE(o.order_no,''), COALESCE(c.id,0), COALESCE(c.name,''),
			COALESCE(NULLIF(c.company_name,''), c.name, ''), COALESCE(NULLIF(c.company_address,''), c.address, ''),
			COALESCE(NULLIF(c.company_phone,''), c.phone, ''), COALESCE(c.contact,''), COALESCE(c.phone,''), COALESCE(c.address,'')
		FROM %s.orders o
		LEFT JOIN %s.customers c ON c.id=o.customer_id
		WHERE o.id=$1`, r.schema, r.schema)
	var out salesapp.SalesOrderContext
	if err := r.pool.QueryRow(ctx, q, orderID).Scan(
		&out.OrderID,
		&out.OrderNo,
		&out.Customer.ID,
		&out.Customer.Name,
		&out.Customer.CompanyName,
		&out.Customer.CompanyAddress,
		&out.Customer.CompanyPhone,
		&out.Customer.Contact,
		&out.Customer.Phone,
		&out.Customer.Address,
	); err != nil {
		return salesapp.SalesOrderContext{}, err
	}
	return out, nil
}

func (r Repository) GenerateSalesOrderDocument(ctx context.Context, cmd salesapp.GenerateSalesOrderDocumentCommand) (salesapp.GenerateSalesOrderDocumentResult, error) {
	settings, err := r.LoadSalesOrderSettings(ctx)
	if err != nil {
		return salesapp.GenerateSalesOrderDocumentResult{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return salesapp.GenerateSalesOrderDocumentResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	versionRows, err := tx.Query(ctx, fmt.Sprintf(`SELECT version_no FROM %s.sales_order_documents WHERE order_id=$1 FOR UPDATE`, r.schema), cmd.OrderID)
	if err != nil {
		return salesapp.GenerateSalesOrderDocumentResult{}, err
	}
	existing := make([]int, 0)
	for versionRows.Next() {
		var version int
		if err := versionRows.Scan(&version); err != nil {
			versionRows.Close()
			return salesapp.GenerateSalesOrderDocumentResult{}, err
		}
		existing = append(existing, version)
	}
	if err := versionRows.Err(); err != nil {
		versionRows.Close()
		return salesapp.GenerateSalesOrderDocumentResult{}, err
	}
	versionRows.Close()
	versionNo := salesdomain.NextSalesOrderVersion(existing)

	snapshot, err := r.buildSalesOrderSnapshotTx(ctx, tx, cmd.OrderID, settings)
	if err != nil {
		return salesapp.GenerateSalesOrderDocumentResult{}, err
	}
	pdfBytes, err := r.renderer.Render(snapshot)
	if err != nil {
		return salesapp.GenerateSalesOrderDocumentResult{}, err
	}
	objectKey := filepath.ToSlash(filepath.Join("sales_order_documents", safeSalesOrderPathPart(snapshot.OrderNo), fmt.Sprintf("V%d.pdf", versionNo)))
	if err := os.MkdirAll(filepath.Dir(filepath.Join(r.assetDir, objectKey)), 0755); err != nil {
		return salesapp.GenerateSalesOrderDocumentResult{}, err
	}
	if err := os.WriteFile(filepath.Join(r.assetDir, objectKey), pdfBytes, 0644); err != nil {
		return salesapp.GenerateSalesOrderDocumentResult{}, err
	}
	sum := sha256.Sum256(pdfBytes)
	var assetID int64
	assetQ := fmt.Sprintf(`INSERT INTO %s.sales_order_assets(kind, filename, content_type, bytes, sha256, object_key, created_by)
		VALUES('sales_order_pdf',$1,'application/pdf',$2,$3,$4,$5)
		RETURNING id`, r.schema)
	filename := fmt.Sprintf("%s-V%d.pdf", snapshot.OrderNo, versionNo)
	if err := tx.QueryRow(ctx, assetQ, filename, int64(len(pdfBytes)), hex.EncodeToString(sum[:]), objectKey, cmd.Actor).Scan(&assetID); err != nil {
		return salesapp.GenerateSalesOrderDocumentResult{}, err
	}
	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		return salesapp.GenerateSalesOrderDocumentResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.sales_order_documents SET is_latest=false WHERE order_id=$1`, r.schema), cmd.OrderID); err != nil {
		return salesapp.GenerateSalesOrderDocumentResult{}, err
	}
	var doc salesapp.SalesOrderDocument
	insertQ := fmt.Sprintf(`INSERT INTO %s.sales_order_documents(order_id, order_no, version_no, snapshot_json, pdf_asset_id, is_latest, created_by)
		VALUES($1,$2,$3,$4,$5,true,$6)
		RETURNING id, order_id, order_no, version_no, pdf_asset_id, is_latest, to_char(created_at,'YYYY-MM-DD HH24:MI:SS'), created_by`, r.schema)
	if err := tx.QueryRow(ctx, insertQ, cmd.OrderID, snapshot.OrderNo, versionNo, snapshotJSON, assetID, cmd.Actor).Scan(&doc.ID, &doc.OrderID, &doc.OrderNo, &doc.VersionNo, &doc.PDFAssetID, &doc.IsLatest, &doc.CreatedAt, &doc.CreatedBy); err != nil {
		return salesapp.GenerateSalesOrderDocumentResult{}, err
	}
	doc.Snapshot = snapshot
	doc.DownloadURL = salesOrderDocumentDownloadURL(doc.OrderID, doc.ID)
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "sales_order_document", &doc.ID, "create", postgresinfra.StrPtr("version_no"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", versionNo)), postgresinfra.AuditMeta{"order_id": cmd.OrderID, "order_no": snapshot.OrderNo}); err != nil {
		return salesapp.GenerateSalesOrderDocumentResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return salesapp.GenerateSalesOrderDocumentResult{}, err
	}
	return salesapp.GenerateSalesOrderDocumentResult{Document: doc, Snapshot: snapshot}, nil
}

func (r Repository) buildSalesOrderSnapshotTx(ctx context.Context, tx pgx.Tx, orderID int64, settings salesapp.SalesOrderSettings) (salesdomain.SalesOrderSnapshot, error) {
	var snapshot salesdomain.SalesOrderSnapshot
	var total, shipping, discount, grand float64
	q := fmt.Sprintf(`SELECT o.id, o.order_no, COALESCE(to_char(o.order_date,'YYYY-MM-DD'),''), COALESCE(c.name,''),
			COALESCE(NULLIF(c.company_name,''), c.name, ''), COALESCE(NULLIF(c.company_address,''), c.address, ''),
			COALESCE(NULLIF(c.company_phone,''), c.phone, ''),
			COALESCE(o.total_amount,0)::float8, COALESCE(o.shipping_amount,0)::float8,
			COALESCE(o.discount_amount,0)::float8, COALESCE(o.grand_total,0)::float8
		FROM %s.orders o
		LEFT JOIN %s.customers c ON c.id=o.customer_id
		WHERE o.id=$1`, r.schema, r.schema)
	if err := tx.QueryRow(ctx, q, orderID).Scan(&snapshot.OrderID, &snapshot.OrderNo, &snapshot.OrderDate, &snapshot.CustomerName, &snapshot.CustomerCompanyName, &snapshot.CustomerCompanyAddress, &snapshot.CustomerCompanyPhone, &total, &shipping, &discount, &grand); err != nil {
		return salesdomain.SalesOrderSnapshot{}, err
	}
	snapshot.CompanyName = firstNonEmpty(settings.CompanyName, snapshot.CustomerCompanyName, snapshot.CustomerName)
	snapshot.Note = settings.Note
	snapshot.PaymentText = settings.PaymentText
	snapshot.TotalAmount = salesdomain.FormatSalesOrderMoney(total)
	snapshot.Shipping = salesdomain.FormatSalesOrderMoney(shipping)
	snapshot.Discount = salesdomain.FormatSalesOrderMoney(discount)
	snapshot.GrandTotal = salesdomain.FormatSalesOrderMoney(grand)
	for _, code := range settings.PaymentCodes {
		snapshot.PaymentCodes = append(snapshot.PaymentCodes, salesdomain.SalesOrderAssetRef{
			ID: code.Asset.ID, Label: code.Label, Description: code.Description, ObjectKey: code.Asset.ObjectKey, ContentType: code.Asset.ContentType,
		})
	}
	if settings.Seal != nil {
		snapshot.Seal = &salesdomain.SalesOrderAssetRef{ID: settings.Seal.ID, Label: settings.Seal.Filename, ObjectKey: settings.Seal.ObjectKey, ContentType: settings.Seal.ContentType}
	}

	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT COALESCE(NULLIF(oi.item_name,''), p.name, ''), COALESCE(oi.spec,''), COALESCE(oi.qty,0)::float8,
			COALESCE(oi.unit,''), COALESCE(oi.unit_price,0)::float8, COALESCE(oi.line_total,0)::float8
		FROM %s.order_items oi
		LEFT JOIN %s.products p ON p.id=oi.product_id
		WHERE oi.order_id=$1
		ORDER BY oi.line_no, oi.id`, r.schema, r.schema), orderID)
	if err != nil {
		return salesdomain.SalesOrderSnapshot{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var item salesdomain.SalesOrderSnapshotItem
		var qty, unitPrice, lineTotal float64
		if err := rows.Scan(&item.Name, &item.Spec, &qty, &item.Unit, &unitPrice, &lineTotal); err != nil {
			return salesdomain.SalesOrderSnapshot{}, err
		}
		item.Qty = trimFloatZero(qty)
		item.UnitPrice = salesdomain.FormatSalesOrderMoney(unitPrice)
		item.LineTotal = salesdomain.FormatSalesOrderMoney(lineTotal)
		snapshot.Items = append(snapshot.Items, item)
	}
	if err := rows.Err(); err != nil {
		return salesdomain.SalesOrderSnapshot{}, err
	}
	if err := snapshot.Validate(); err != nil {
		return salesdomain.SalesOrderSnapshot{}, err
	}
	return snapshot, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if s := strings.TrimSpace(value); s != "" {
			return s
		}
	}
	return ""
}

func (r Repository) ListSalesOrderDocuments(ctx context.Context, orderID int64) ([]salesapp.SalesOrderDocument, error) {
	q := fmt.Sprintf(`SELECT id, order_id, order_no, version_no, snapshot_json, COALESCE(pdf_asset_id,0), is_latest, to_char(created_at,'YYYY-MM-DD HH24:MI:SS'), created_by
		FROM %s.sales_order_documents
		WHERE order_id=$1
		ORDER BY version_no DESC`, r.schema)
	rows, err := r.pool.Query(ctx, q, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]salesapp.SalesOrderDocument, 0)
	for rows.Next() {
		var doc salesapp.SalesOrderDocument
		var raw []byte
		if err := rows.Scan(&doc.ID, &doc.OrderID, &doc.OrderNo, &doc.VersionNo, &raw, &doc.PDFAssetID, &doc.IsLatest, &doc.CreatedAt, &doc.CreatedBy); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(raw, &doc.Snapshot)
		doc.DownloadURL = salesOrderDocumentDownloadURL(doc.OrderID, doc.ID)
		out = append(out, doc)
	}
	return out, rows.Err()
}

func (r Repository) LoadSalesOrderDocumentFile(ctx context.Context, orderID, documentID int64, latest bool) (salesapp.SalesOrderDocumentFile, error) {
	where := "d.order_id=$1 AND d.id=$2"
	args := []any{orderID, documentID}
	if latest {
		where = "d.order_id=$1 AND d.is_latest=true"
		args = []any{orderID}
	}
	q := fmt.Sprintf(`SELECT d.id, d.order_id, d.order_no, d.version_no, COALESCE(d.pdf_asset_id,0), d.is_latest, to_char(d.created_at,'YYYY-MM-DD HH24:MI:SS'), d.created_by,
			a.object_key
		FROM %s.sales_order_documents d
		JOIN %s.sales_order_assets a ON a.id=d.pdf_asset_id
		WHERE %s
		ORDER BY d.version_no DESC
		LIMIT 1`, r.schema, r.schema, where)
	var doc salesapp.SalesOrderDocument
	var objectKey string
	if err := r.pool.QueryRow(ctx, q, args...).Scan(&doc.ID, &doc.OrderID, &doc.OrderNo, &doc.VersionNo, &doc.PDFAssetID, &doc.IsLatest, &doc.CreatedAt, &doc.CreatedBy, &objectKey); err != nil {
		return salesapp.SalesOrderDocumentFile{}, err
	}
	doc.DownloadURL = salesOrderDocumentDownloadURL(doc.OrderID, doc.ID)
	return salesapp.SalesOrderDocumentFile{
		Document: doc,
		Path:     filepath.Join(r.assetDir, objectKey),
		Filename: fmt.Sprintf("%s-V%d.pdf", doc.OrderNo, doc.VersionNo),
	}, nil
}

func salesOrderDocumentDownloadURL(orderID, documentID int64) string {
	return fmt.Sprintf("/orders/%d/sales-orders/%d.pdf", orderID, documentID)
}

func safeSalesOrderPathPart(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown"
	}
	replacer := strings.NewReplacer("/", "-", "\\", "-", ":", "-", "..", ".")
	return replacer.Replace(s)
}
