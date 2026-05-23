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
	settings := salesapp.SalesOrderSettings{
		SealXMM:             32,
		SealYMM:             5,
		SealWidthMM:         36,
		PaymentTextXMM:      salesapp.DefaultSalesOrderPaymentTextXMM,
		PaymentTextYMM:      salesapp.DefaultSalesOrderPaymentTextYMM,
		PaymentTextWidthMM:  salesapp.DefaultSalesOrderPaymentTextWidthMM,
		PaymentTextHeightMM: salesapp.DefaultSalesOrderPaymentTextHeightMM,
		PaymentCodeXMM:      salesapp.DefaultSalesOrderPaymentCodeXMM,
		PaymentCodeYMM:      salesapp.DefaultSalesOrderPaymentCodeYMM,
		PaymentCodeWidthMM:  salesapp.DefaultSalesOrderPaymentCodeWidthMM,
		PaymentCodeHeightMM: salesapp.DefaultSalesOrderPaymentCodeHeightMM,
	}
	q := fmt.Sprintf(`SELECT s.company_name, s.note, s.payment_text,
			COALESCE(s.bank_account_name,''), COALESCE(s.bank_name,''), COALESCE(s.bank_account_no,''),
			COALESCE(s.seal_x_mm,32)::float8, COALESCE(s.seal_y_mm,5)::float8, COALESCE(s.seal_width_mm,36)::float8,
			COALESCE(s.payment_text_x_mm,16)::float8, COALESCE(s.payment_text_y_mm,118)::float8, COALESCE(s.payment_text_width_mm,104)::float8, COALESCE(s.payment_text_height_mm,78)::float8,
			COALESCE(s.payment_code_x_mm,126)::float8, COALESCE(s.payment_code_y_mm,106)::float8, COALESCE(s.payment_code_width_mm,72)::float8, COALESCE(s.payment_code_height_mm,122)::float8,
			COALESCE(a.id,0), COALESCE(a.kind,''), COALESCE(a.filename,''), COALESCE(a.content_type,''), COALESCE(a.bytes,0), COALESCE(a.sha256,''), COALESCE(a.object_key,''), COALESCE(to_char(a.created_at,'YYYY-MM-DD HH24:MI:SS'),''), COALESCE(a.created_by,'')
		FROM %s.sales_order_settings s
		LEFT JOIN %s.sales_order_assets a ON a.id=s.seal_asset_id
		WHERE s.id=1`, r.schema, r.schema)
	var seal salesapp.SalesOrderAsset
	err := r.pool.QueryRow(ctx, q).Scan(&settings.CompanyName, &settings.Note, &settings.PaymentText, &settings.BankAccountName, &settings.BankName, &settings.BankAccountNo, &settings.SealXMM, &settings.SealYMM, &settings.SealWidthMM, &settings.PaymentTextXMM, &settings.PaymentTextYMM, &settings.PaymentTextWidthMM, &settings.PaymentTextHeightMM, &settings.PaymentCodeXMM, &settings.PaymentCodeYMM, &settings.PaymentCodeWidthMM, &settings.PaymentCodeHeightMM, &seal.ID, &seal.Kind, &seal.Filename, &seal.ContentType, &seal.Bytes, &seal.SHA256, &seal.ObjectKey, &seal.CreatedAt, &seal.CreatedBy)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
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
	q := fmt.Sprintf(`INSERT INTO %s.sales_order_settings(id, company_name, note, payment_text, bank_account_name, bank_name, bank_account_no, seal_x_mm, seal_y_mm, seal_width_mm, payment_text_x_mm, payment_text_y_mm, payment_text_width_mm, payment_text_height_mm, payment_code_x_mm, payment_code_y_mm, payment_code_width_mm, payment_code_height_mm, updated_at, updated_by)
		VALUES(1,$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,now(),$18)
		ON CONFLICT(id) DO UPDATE SET
			company_name=excluded.company_name,
			note=excluded.note,
			payment_text=excluded.payment_text,
			bank_account_name=excluded.bank_account_name,
			bank_name=excluded.bank_name,
			bank_account_no=excluded.bank_account_no,
			seal_x_mm=excluded.seal_x_mm,
			seal_y_mm=excluded.seal_y_mm,
			seal_width_mm=excluded.seal_width_mm,
			payment_text_x_mm=excluded.payment_text_x_mm,
			payment_text_y_mm=excluded.payment_text_y_mm,
			payment_text_width_mm=excluded.payment_text_width_mm,
			payment_text_height_mm=excluded.payment_text_height_mm,
			payment_code_x_mm=excluded.payment_code_x_mm,
			payment_code_y_mm=excluded.payment_code_y_mm,
			payment_code_width_mm=excluded.payment_code_width_mm,
			payment_code_height_mm=excluded.payment_code_height_mm,
			updated_at=now(),
			updated_by=excluded.updated_by`, r.schema)
	_, err := r.pool.Exec(ctx, q, cmd.CompanyName, cmd.Note, cmd.PaymentText, cmd.BankAccountName, cmd.BankName, cmd.BankAccountNo, cmd.SealXMM, cmd.SealYMM, cmd.SealWidthMM, cmd.PaymentTextXMM, cmd.PaymentTextYMM, cmd.PaymentTextWidthMM, cmd.PaymentTextHeightMM, cmd.PaymentCodeXMM, cmd.PaymentCodeYMM, cmd.PaymentCodeWidthMM, cmd.PaymentCodeHeightMM, cmd.Actor)
	if err != nil {
		return err
	}
	return postgresinfra.NewAuditService(r.pool, r.schema).Insert(ctx, postgresinfra.AuditEntry{
		Actor:      cmd.Actor,
		EntityType: "sales_order_settings",
		Action:     "update",
		Field:      postgresinfra.StrPtr("settings"),
		NewValue:   postgresinfra.StrPtr(cmd.CompanyName),
		Meta: postgresinfra.AuditMeta{
			"company_name":           cmd.CompanyName,
			"bank_account_name":      cmd.BankAccountName,
			"bank_name":              cmd.BankName,
			"bank_account_no":        cmd.BankAccountNo,
			"seal_x_mm":              cmd.SealXMM,
			"seal_y_mm":              cmd.SealYMM,
			"seal_width_mm":          cmd.SealWidthMM,
			"payment_text_x_mm":      cmd.PaymentTextXMM,
			"payment_text_y_mm":      cmd.PaymentTextYMM,
			"payment_text_width_mm":  cmd.PaymentTextWidthMM,
			"payment_text_height_mm": cmd.PaymentTextHeightMM,
			"payment_code_x_mm":      cmd.PaymentCodeXMM,
			"payment_code_y_mm":      cmd.PaymentCodeYMM,
			"payment_code_width_mm":  cmd.PaymentCodeWidthMM,
			"payment_code_height_mm": cmd.PaymentCodeHeightMM,
		},
	})
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

func (r Repository) DeleteSalesOrderAsset(ctx context.Context, id int64, actor string) error {
	q := fmt.Sprintf(`DELETE FROM %s.sales_order_assets WHERE id=$1`, r.schema)
	tag, err := r.pool.Exec(ctx, q, id)
	if err == nil && tag.RowsAffected() > 0 {
		postgresinfra.AuditInsert(ctx, r.pool, r.schema, actor, "sales_order_asset", &id, "delete", postgresinfra.StrPtr("id"), postgresinfra.StrPtr(fmt.Sprintf("%d", id)), nil, nil)
	}
	return err
}

func (r Repository) ListSalesOrderSealAssets(ctx context.Context) ([]salesapp.SalesOrderAsset, error) {
	q := fmt.Sprintf(`SELECT id, kind, filename, content_type, bytes, sha256, object_key, to_char(created_at,'YYYY-MM-DD HH24:MI:SS'), created_by
		FROM %s.sales_order_assets
		WHERE kind='seal'
		ORDER BY id DESC`, r.schema)
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]salesapp.SalesOrderAsset, 0)
	for rows.Next() {
		var asset salesapp.SalesOrderAsset
		if err := rows.Scan(&asset.ID, &asset.Kind, &asset.Filename, &asset.ContentType, &asset.Bytes, &asset.SHA256, &asset.ObjectKey, &asset.CreatedAt, &asset.CreatedBy); err != nil {
			return nil, err
		}
		asset.URL = salesOrderAssetURL(asset.ObjectKey)
		out = append(out, asset)
	}
	return out, rows.Err()
}

func (r Repository) SaveSalesOrderPaymentCode(ctx context.Context, cmd salesapp.SaveSalesOrderPaymentCodeCommand) (salesapp.SalesOrderPaymentCode, error) {
	var code salesapp.SalesOrderPaymentCode
	if cmd.ID > 0 {
		q := fmt.Sprintf(`UPDATE %s.sales_order_payment_codes
			SET label=$2, description=$3, asset_id=$4, sort=$5, active=$6, updated_at=now()
			WHERE id=$1 AND deleted_at IS NULL
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

func (r Repository) SaveSalesOrderNote(ctx context.Context, cmd salesapp.SaveSalesOrderNoteCommand) error {
	var oldNote string
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(sales_order_note,'') FROM %s.orders WHERE id=$1`, r.schema), cmd.OrderID).Scan(&oldNote); err != nil {
		return err
	}
	note := strings.TrimSpace(cmd.Note)
	if _, err := r.pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET sales_order_note=$2 WHERE id=$1`, r.schema), cmd.OrderID, note); err != nil {
		return err
	}
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, cmd.Actor, "order", &cmd.OrderID, "update", postgresinfra.StrPtr("sales_order_note"), postgresinfra.StrPtr(oldNote), postgresinfra.StrPtr(note), nil)
	return nil
}

func (r Repository) DeactivateSalesOrderPaymentCode(ctx context.Context, id int64, actor string) error {
	q := fmt.Sprintf(`UPDATE %s.sales_order_payment_codes SET active=false, updated_at=now() WHERE id=$1 AND deleted_at IS NULL`, r.schema)
	tag, err := r.pool.Exec(ctx, q, id)
	if err == nil && tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	if err == nil {
		postgresinfra.AuditInsert(ctx, r.pool, r.schema, actor, "sales_order_payment_code", &id, "deactivate", postgresinfra.StrPtr("active"), postgresinfra.StrPtr("true"), postgresinfra.StrPtr("false"), nil)
	}
	return err
}

func (r Repository) ActivateSalesOrderPaymentCode(ctx context.Context, id int64, actor string) error {
	q := fmt.Sprintf(`UPDATE %s.sales_order_payment_codes SET active=true, updated_at=now() WHERE id=$1 AND deleted_at IS NULL`, r.schema)
	tag, err := r.pool.Exec(ctx, q, id)
	if err == nil && tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	if err == nil {
		postgresinfra.AuditInsert(ctx, r.pool, r.schema, actor, "sales_order_payment_code", &id, "activate", postgresinfra.StrPtr("active"), postgresinfra.StrPtr("false"), postgresinfra.StrPtr("true"), nil)
	}
	return err
}

func (r Repository) DeleteSalesOrderPaymentCode(ctx context.Context, id int64, actor string) error {
	q := fmt.Sprintf(`UPDATE %s.sales_order_payment_codes SET active=false, deleted_at=now(), deleted_by=$2, updated_at=now() WHERE id=$1 AND deleted_at IS NULL`, r.schema)
	tag, err := r.pool.Exec(ctx, q, id, actor)
	if err == nil && tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	if err == nil {
		postgresinfra.AuditInsert(ctx, r.pool, r.schema, actor, "sales_order_payment_code", &id, "delete", postgresinfra.StrPtr("deleted_at"), nil, postgresinfra.StrPtr("now"), nil)
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
		WHERE pc.deleted_at IS NULL
		ORDER BY pc.active DESC, pc.sort, pc.id`, r.schema, r.schema)
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

	existing, err := r.loadSalesOrderVersionsTx(ctx, tx, cmd.OrderID, true)
	if err != nil {
		return salesapp.GenerateSalesOrderDocumentResult{}, err
	}
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
	fileWritten := false
	committed := false
	defer func() {
		if fileWritten && !committed {
			cleanupGeneratedSalesOrderAssetFile(r.assetDir, objectKey)
		}
	}()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(r.assetDir, objectKey)), 0755); err != nil {
		return salesapp.GenerateSalesOrderDocumentResult{}, err
	}
	if err := os.WriteFile(filepath.Join(r.assetDir, objectKey), pdfBytes, 0644); err != nil {
		return salesapp.GenerateSalesOrderDocumentResult{}, err
	}
	fileWritten = true
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
	committed = true
	return salesapp.GenerateSalesOrderDocumentResult{Document: doc, Snapshot: snapshot}, nil
}

func (r Repository) GenerateSalesOrderImage(ctx context.Context, cmd salesapp.GenerateSalesOrderImageCommand) (salesapp.GenerateSalesOrderImageResult, error) {
	settings, err := r.LoadSalesOrderSettings(ctx)
	if err != nil {
		return salesapp.GenerateSalesOrderImageResult{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return salesapp.GenerateSalesOrderImageResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	existing, err := r.loadSalesOrderImageVersionsTx(ctx, tx, cmd.OrderID, true)
	if err != nil {
		return salesapp.GenerateSalesOrderImageResult{}, err
	}
	versionNo := salesdomain.NextSalesOrderVersion(existing)

	snapshot, err := r.buildSalesOrderSnapshotTx(ctx, tx, cmd.OrderID, settings)
	if err != nil {
		return salesapp.GenerateSalesOrderImageResult{}, err
	}
	imageBytes, err := r.renderer.RenderPNG(snapshot)
	if err != nil {
		return salesapp.GenerateSalesOrderImageResult{}, err
	}
	objectKey := filepath.ToSlash(filepath.Join("sales_order_images", safeSalesOrderPathPart(snapshot.OrderNo), fmt.Sprintf("V%d.png", versionNo)))
	fileWritten := false
	committed := false
	defer func() {
		if fileWritten && !committed {
			cleanupGeneratedSalesOrderAssetFile(r.assetDir, objectKey)
		}
	}()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(r.assetDir, objectKey)), 0755); err != nil {
		return salesapp.GenerateSalesOrderImageResult{}, err
	}
	if err := os.WriteFile(filepath.Join(r.assetDir, objectKey), imageBytes, 0644); err != nil {
		return salesapp.GenerateSalesOrderImageResult{}, err
	}
	fileWritten = true
	sum := sha256.Sum256(imageBytes)
	var assetID int64
	assetQ := fmt.Sprintf(`INSERT INTO %s.sales_order_assets(kind, filename, content_type, bytes, sha256, object_key, created_by)
		VALUES('sales_order_image',$1,'image/png',$2,$3,$4,$5)
		RETURNING id`, r.schema)
	filename := fmt.Sprintf("%s-V%d.png", snapshot.OrderNo, versionNo)
	if err := tx.QueryRow(ctx, assetQ, filename, int64(len(imageBytes)), hex.EncodeToString(sum[:]), objectKey, cmd.Actor).Scan(&assetID); err != nil {
		return salesapp.GenerateSalesOrderImageResult{}, err
	}
	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		return salesapp.GenerateSalesOrderImageResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.sales_order_images SET is_latest=false WHERE order_id=$1`, r.schema), cmd.OrderID); err != nil {
		return salesapp.GenerateSalesOrderImageResult{}, err
	}
	var doc salesapp.SalesOrderImageDocument
	insertQ := fmt.Sprintf(`INSERT INTO %s.sales_order_images(order_id, order_no, version_no, snapshot_json, image_asset_id, is_latest, created_by)
		VALUES($1,$2,$3,$4,$5,true,$6)
		RETURNING id, order_id, order_no, version_no, image_asset_id, is_latest, to_char(created_at,'YYYY-MM-DD HH24:MI:SS'), created_by`, r.schema)
	if err := tx.QueryRow(ctx, insertQ, cmd.OrderID, snapshot.OrderNo, versionNo, snapshotJSON, assetID, cmd.Actor).Scan(&doc.ID, &doc.OrderID, &doc.OrderNo, &doc.VersionNo, &doc.ImageAssetID, &doc.IsLatest, &doc.CreatedAt, &doc.CreatedBy); err != nil {
		return salesapp.GenerateSalesOrderImageResult{}, err
	}
	doc.Snapshot = snapshot
	doc.DownloadURL = salesOrderImageDownloadURL(doc.OrderID, doc.ID)
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "sales_order_image", &doc.ID, "create", postgresinfra.StrPtr("version_no"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", versionNo)), postgresinfra.AuditMeta{"order_id": cmd.OrderID, "order_no": snapshot.OrderNo}); err != nil {
		return salesapp.GenerateSalesOrderImageResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return salesapp.GenerateSalesOrderImageResult{}, err
	}
	committed = true
	return salesapp.GenerateSalesOrderImageResult{Document: doc, Snapshot: snapshot}, nil
}

func (r Repository) PreviewSalesOrderDocument(ctx context.Context, orderID int64) (salesapp.SalesOrderPreview, error) {
	settings, err := r.LoadSalesOrderSettings(ctx)
	if err != nil {
		return salesapp.SalesOrderPreview{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return salesapp.SalesOrderPreview{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	existing, err := r.loadSalesOrderVersionsTx(ctx, tx, orderID, false)
	if err != nil {
		return salesapp.SalesOrderPreview{}, err
	}
	snapshot, err := r.buildSalesOrderSnapshotTx(ctx, tx, orderID, settings)
	if err != nil {
		return salesapp.SalesOrderPreview{}, err
	}
	return salesapp.SalesOrderPreview{
		OrderID:       snapshot.OrderID,
		OrderNo:       snapshot.OrderNo,
		NextVersionNo: salesdomain.NextSalesOrderVersion(existing),
		Snapshot:      snapshot,
	}, nil
}

func (r Repository) PreviewSalesOrderPDF(ctx context.Context, orderID int64) (salesapp.SalesOrderPreviewPDF, error) {
	preview, err := r.PreviewSalesOrderDocument(ctx, orderID)
	if err != nil {
		return salesapp.SalesOrderPreviewPDF{}, err
	}
	pdfBytes, err := r.renderer.RenderPreview(preview.Snapshot)
	if err != nil {
		return salesapp.SalesOrderPreviewPDF{}, err
	}
	return salesapp.SalesOrderPreviewPDF{
		Preview:  preview,
		Data:     pdfBytes,
		Filename: fmt.Sprintf("%s-preview.pdf", safeSalesOrderPathPart(preview.OrderNo)),
	}, nil
}

func (r Repository) loadSalesOrderVersionsTx(ctx context.Context, tx pgx.Tx, orderID int64, lock bool) ([]int, error) {
	q := fmt.Sprintf(`SELECT version_no FROM %s.sales_order_documents WHERE order_id=$1`, r.schema)
	if lock {
		q += " FOR UPDATE"
	}
	versionRows, err := tx.Query(ctx, q, orderID)
	if err != nil {
		return nil, err
	}
	defer versionRows.Close()
	existing := make([]int, 0)
	for versionRows.Next() {
		var version int
		if err := versionRows.Scan(&version); err != nil {
			return nil, err
		}
		existing = append(existing, version)
	}
	if err := versionRows.Err(); err != nil {
		return nil, err
	}
	return existing, nil
}

func (r Repository) loadSalesOrderImageVersionsTx(ctx context.Context, tx pgx.Tx, orderID int64, lock bool) ([]int, error) {
	q := fmt.Sprintf(`SELECT version_no FROM %s.sales_order_images WHERE order_id=$1`, r.schema)
	if lock {
		q += " FOR UPDATE"
	}
	versionRows, err := tx.Query(ctx, q, orderID)
	if err != nil {
		return nil, err
	}
	defer versionRows.Close()
	existing := make([]int, 0)
	for versionRows.Next() {
		var version int
		if err := versionRows.Scan(&version); err != nil {
			return nil, err
		}
		existing = append(existing, version)
	}
	if err := versionRows.Err(); err != nil {
		return nil, err
	}
	return existing, nil
}

func (r Repository) buildSalesOrderSnapshotTx(ctx context.Context, tx pgx.Tx, orderID int64, settings salesapp.SalesOrderSettings) (salesdomain.SalesOrderSnapshot, error) {
	var snapshot salesdomain.SalesOrderSnapshot
	var total, shipping, discount, grand float64
	q := fmt.Sprintf(`SELECT o.id, o.order_no,
			COALESCE(to_char(o.document_date,'YYYY-MM-DD'), to_char(o.order_date,'YYYY-MM-DD'), ''),
			COALESCE(to_char(o.order_date,'YYYY-MM-DD'),''), COALESCE(c.name,''),
			COALESCE(NULLIF(c.company_name,''), c.name, ''), COALESCE(NULLIF(c.company_address,''), c.address, ''),
			COALESCE(NULLIF(c.company_phone,''), c.phone, ''),
			COALESCE(o.total_amount,0)::float8, COALESCE(o.shipping_amount,0)::float8,
			COALESCE(o.discount_amount,0)::float8, COALESCE(o.grand_total,0)::float8,
			COALESCE(o.sales_order_note,'')
		FROM %s.orders o
		LEFT JOIN %s.customers c ON c.id=o.customer_id
		WHERE o.id=$1`, r.schema, r.schema)
	if err := tx.QueryRow(ctx, q, orderID).Scan(&snapshot.OrderID, &snapshot.OrderNo, &snapshot.DocumentDate, &snapshot.OrderDate, &snapshot.CustomerName, &snapshot.CustomerCompanyName, &snapshot.CustomerCompanyAddress, &snapshot.CustomerCompanyPhone, &total, &shipping, &discount, &grand, &snapshot.SalesOrderNote); err != nil {
		return salesdomain.SalesOrderSnapshot{}, err
	}
	companyProfile, err := r.loadCompanyProfileForSalesOrderTx(ctx, tx)
	if err != nil {
		return salesdomain.SalesOrderSnapshot{}, err
	}
	snapshot.CompanyName = firstNonEmpty(companyProfile.Name, settings.CompanyName, snapshot.CustomerCompanyName, snapshot.CustomerName)
	snapshot.CompanyAddress = companyProfile.Address
	snapshot.Note = settings.Note
	snapshot.PaymentText = settings.PaymentText
	snapshot.TaxpayerID = companyProfile.TaxpayerID
	snapshot.BankAccountName = firstNonEmpty(companyProfile.BankAccountName, settings.BankAccountName)
	snapshot.BankName = firstNonEmpty(companyProfile.BankName, settings.BankName)
	snapshot.BankAccountNo = firstNonEmpty(companyProfile.BankAccountNo, settings.BankAccountNo)
	snapshot.PaymentTextBox = salesdomain.SalesOrderLayoutBox{XMM: settings.PaymentTextXMM, YMM: settings.PaymentTextYMM, WidthMM: settings.PaymentTextWidthMM, HeightMM: settings.PaymentTextHeightMM}
	snapshot.PaymentCodeBox = salesdomain.SalesOrderLayoutBox{XMM: settings.PaymentCodeXMM, YMM: settings.PaymentCodeYMM, WidthMM: settings.PaymentCodeWidthMM, HeightMM: settings.PaymentCodeHeightMM}
	snapshot.TotalAmount = salesdomain.FormatSalesOrderMoney(total)
	snapshot.Shipping = salesdomain.FormatSalesOrderMoney(shipping)
	snapshot.Discount = salesdomain.FormatSalesOrderMoney(discount)
	snapshot.GrandTotal = salesdomain.FormatSalesOrderMoney(grand)
	breakdowns, err := r.loadSalesOrderDiscountBreakdownsTx(ctx, tx, orderID, discount)
	if err != nil {
		return salesdomain.SalesOrderSnapshot{}, err
	}
	snapshot.DiscountBreakdowns = breakdowns
	for _, code := range settings.PaymentCodes {
		if !code.Active {
			continue
		}
		snapshot.PaymentCodes = append(snapshot.PaymentCodes, salesdomain.SalesOrderAssetRef{
			ID: code.Asset.ID, Label: code.Label, Description: code.Description, ObjectKey: code.Asset.ObjectKey, ContentType: code.Asset.ContentType, URL: code.Asset.URL,
		})
	}
	if settings.Seal != nil {
		snapshot.Seal = &salesdomain.SalesOrderAssetRef{ID: settings.Seal.ID, Label: settings.Seal.Filename, ObjectKey: settings.Seal.ObjectKey, ContentType: settings.Seal.ContentType, URL: settings.Seal.URL, XMM: settings.SealXMM, YMM: settings.SealYMM, WidthMM: settings.SealWidthMM}
	}

	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT COALESCE(NULLIF(oi.item_name,''), p.name, ''), COALESCE(oi.item_note,''), COALESCE(oi.spec,''), COALESCE(oi.qty,0)::float8,
			COALESCE(oi.unit,''), COALESCE(oi.unit_price,0)::float8, COALESCE(oi.discount_amount,0)::float8, COALESCE(oi.line_total,0)::float8
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
		var qty, unitPrice, discountAmount, lineTotal float64
		if err := rows.Scan(&item.Name, &item.Note, &item.Spec, &qty, &item.Unit, &unitPrice, &discountAmount, &lineTotal); err != nil {
			return salesdomain.SalesOrderSnapshot{}, err
		}
		item.Qty = trimFloatZero(qty)
		item.UnitPrice = salesdomain.FormatSalesOrderMoney(unitPrice)
		item.DiscountAmount = salesdomain.FormatSalesOrderMoney(discountAmount)
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

func (r Repository) loadSalesOrderDiscountBreakdownsTx(ctx context.Context, tx pgx.Tx, orderID int64, orderDiscount float64) ([]salesdomain.SalesOrderDiscountBreakdown, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT COALESCE(discount_type,''), COALESCE(SUM(discount_amount),0)::float8
		FROM %s.order_items
		WHERE order_id=$1 AND COALESCE(discount_amount,0) > 0
		GROUP BY COALESCE(discount_type,'')
		ORDER BY MIN(line_no), MIN(id)
	`, r.schema), orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]salesdomain.SalesOrderDiscountBreakdown, 0)
	itemDiscount := 0.0
	for rows.Next() {
		var typ string
		var amount float64
		if err := rows.Scan(&typ, &amount); err != nil {
			return nil, err
		}
		itemDiscount += amount
		out = append(out, salesdomain.SalesOrderDiscountBreakdown{Type: typ, Amount: salesdomain.FormatSalesOrderMoney(amount)})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	manualDiscount := orderDiscount - itemDiscount
	if manualDiscount > 0.004 {
		out = append(out, salesdomain.SalesOrderDiscountBreakdown{Type: "order_amount", Amount: salesdomain.FormatSalesOrderMoney(manualDiscount)})
	}
	return out, nil
}

type salesOrderCompanyProfile struct {
	Name            string
	Address         string
	TaxpayerID      string
	BankAccountName string
	BankName        string
	BankAccountNo   string
}

func (r Repository) loadCompanyProfileForSalesOrderTx(ctx context.Context, tx pgx.Tx) (salesOrderCompanyProfile, error) {
	var profile salesOrderCompanyProfile
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(company_name,''), COALESCE(company_address,''), COALESCE(taxpayer_id,''), COALESCE(bank_account_name,''), COALESCE(bank_name,''), COALESCE(bank_account_no,'') FROM %s.company_profile WHERE id=1`, r.schema)).Scan(&profile.Name, &profile.Address, &profile.TaxpayerID, &profile.BankAccountName, &profile.BankName, &profile.BankAccountNo)
	if errors.Is(err, pgx.ErrNoRows) {
		return salesOrderCompanyProfile{}, nil
	}
	if err != nil {
		return salesOrderCompanyProfile{}, err
	}
	profile.Name = strings.TrimSpace(profile.Name)
	profile.Address = strings.TrimSpace(profile.Address)
	profile.TaxpayerID = strings.TrimSpace(profile.TaxpayerID)
	profile.BankAccountName = strings.TrimSpace(profile.BankAccountName)
	profile.BankName = strings.TrimSpace(profile.BankName)
	profile.BankAccountNo = strings.TrimSpace(profile.BankAccountNo)
	return profile, nil
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

func (r Repository) ListSalesOrderImageDocuments(ctx context.Context, orderID int64) ([]salesapp.SalesOrderImageDocument, error) {
	q := fmt.Sprintf(`SELECT id, order_id, order_no, version_no, snapshot_json, COALESCE(image_asset_id,0), is_latest, to_char(created_at,'YYYY-MM-DD HH24:MI:SS'), created_by
		FROM %s.sales_order_images
		WHERE order_id=$1
		ORDER BY version_no DESC`, r.schema)
	rows, err := r.pool.Query(ctx, q, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]salesapp.SalesOrderImageDocument, 0)
	for rows.Next() {
		var doc salesapp.SalesOrderImageDocument
		var raw []byte
		if err := rows.Scan(&doc.ID, &doc.OrderID, &doc.OrderNo, &doc.VersionNo, &raw, &doc.ImageAssetID, &doc.IsLatest, &doc.CreatedAt, &doc.CreatedBy); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(raw, &doc.Snapshot)
		doc.DownloadURL = salesOrderImageDownloadURL(doc.OrderID, doc.ID)
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

func (r Repository) LoadSalesOrderImageFile(ctx context.Context, orderID, imageID int64, latest bool) (salesapp.SalesOrderImageFile, error) {
	where := "d.order_id=$1 AND d.id=$2"
	args := []any{orderID, imageID}
	if latest {
		where = "d.order_id=$1 AND d.is_latest=true"
		args = []any{orderID}
	}
	q := fmt.Sprintf(`SELECT d.id, d.order_id, d.order_no, d.version_no, COALESCE(d.image_asset_id,0), d.is_latest, to_char(d.created_at,'YYYY-MM-DD HH24:MI:SS'), d.created_by,
			a.object_key
		FROM %s.sales_order_images d
		JOIN %s.sales_order_assets a ON a.id=d.image_asset_id
		WHERE %s
		ORDER BY d.version_no DESC
		LIMIT 1`, r.schema, r.schema, where)
	var doc salesapp.SalesOrderImageDocument
	var objectKey string
	if err := r.pool.QueryRow(ctx, q, args...).Scan(&doc.ID, &doc.OrderID, &doc.OrderNo, &doc.VersionNo, &doc.ImageAssetID, &doc.IsLatest, &doc.CreatedAt, &doc.CreatedBy, &objectKey); err != nil {
		return salesapp.SalesOrderImageFile{}, err
	}
	doc.DownloadURL = salesOrderImageDownloadURL(doc.OrderID, doc.ID)
	return salesapp.SalesOrderImageFile{
		Document: doc,
		Path:     filepath.Join(r.assetDir, objectKey),
		Filename: fmt.Sprintf("%s-V%d.png", doc.OrderNo, doc.VersionNo),
	}, nil
}

func salesOrderDocumentDownloadURL(orderID, documentID int64) string {
	return fmt.Sprintf("/orders/%d/sales-orders/%d.pdf", orderID, documentID)
}

func salesOrderImageDownloadURL(orderID, imageID int64) string {
	return fmt.Sprintf("/orders/%d/sales-order-images/%d.png", orderID, imageID)
}

func safeSalesOrderPathPart(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown"
	}
	replacer := strings.NewReplacer("/", "-", "\\", "-", ":", "-", "..", ".")
	return replacer.Replace(s)
}

func cleanupGeneratedSalesOrderAssetFile(assetDir string, objectKey string) {
	clean := filepath.Clean(filepath.FromSlash(strings.TrimSpace(objectKey)))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return
	}
	assetDir = filepath.Clean(assetDir)
	path := filepath.Clean(filepath.Join(assetDir, clean))
	if err := os.Remove(path); err != nil {
		return
	}
	for dir := filepath.Dir(path); dir != "." && dir != assetDir; dir = filepath.Dir(dir) {
		if err := os.Remove(dir); err != nil {
			return
		}
	}
}
