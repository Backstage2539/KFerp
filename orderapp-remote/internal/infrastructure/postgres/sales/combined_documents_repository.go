package sales

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	salesapp "orderapp/internal/application/sales"
	salesdomain "orderapp/internal/domain/sales"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type combinedOrderInfo struct {
	OrderID                int64
	OrderNo                string
	CustomerID             int64
	CustomerName           string
	CustomerCompanyName    string
	CustomerCompanyAddress string
	CustomerCompanyPhone   string
}

func (r Repository) PreviewCombinedSalesOrderDocument(ctx context.Context, orderIDs []int64) (salesapp.CombinedSalesOrderPreview, error) {
	settings, err := r.LoadSalesOrderSettings(ctx)
	if err != nil {
		return salesapp.CombinedSalesOrderPreview{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return salesapp.CombinedSalesOrderPreview{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	key := combinedDocumentKey(orderIDs)
	existing, err := r.loadCombinedSalesOrderVersionsTx(ctx, tx, key, false)
	if err != nil {
		return salesapp.CombinedSalesOrderPreview{}, err
	}
	snapshot, err := r.buildCombinedSalesOrderSnapshotTx(ctx, tx, orderIDs, settings)
	if err != nil {
		return salesapp.CombinedSalesOrderPreview{}, err
	}
	return salesapp.CombinedSalesOrderPreview{
		OrderIDs:      snapshot.OrderIDs,
		OrderNos:      snapshot.OrderNos,
		NextVersionNo: salesdomain.NextCombinedDocumentVersion(existing),
		Snapshot:      snapshot,
	}, nil
}

func (r Repository) PreviewCombinedSalesOrderPDF(ctx context.Context, orderIDs []int64) (salesapp.CombinedSalesOrderPreviewPDF, error) {
	preview, err := r.PreviewCombinedSalesOrderDocument(ctx, orderIDs)
	if err != nil {
		return salesapp.CombinedSalesOrderPreviewPDF{}, err
	}
	pdfBytes, err := r.combinedSalesRenderer.RenderCombinedSalesOrderPreview(preview.Snapshot)
	if err != nil {
		return salesapp.CombinedSalesOrderPreviewPDF{}, err
	}
	return salesapp.CombinedSalesOrderPreviewPDF{
		Preview:  preview,
		Data:     pdfBytes,
		Filename: fmt.Sprintf("%s-preview.pdf", safeSalesOrderPathPart(preview.Snapshot.CombinedNo)),
	}, nil
}

func (r Repository) GenerateCombinedSalesOrderDocument(ctx context.Context, cmd salesapp.CombinedDocumentCommand) (salesapp.GenerateCombinedSalesOrderDocumentResult, error) {
	settings, err := r.LoadSalesOrderSettings(ctx)
	if err != nil {
		return salesapp.GenerateCombinedSalesOrderDocumentResult{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return salesapp.GenerateCombinedSalesOrderDocumentResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	key := combinedDocumentKey(cmd.OrderIDs)
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext($1)::bigint)", key); err != nil {
		return salesapp.GenerateCombinedSalesOrderDocumentResult{}, err
	}
	existing, err := r.loadCombinedSalesOrderVersionsTx(ctx, tx, key, true)
	if err != nil {
		return salesapp.GenerateCombinedSalesOrderDocumentResult{}, err
	}
	versionNo := salesdomain.NextCombinedDocumentVersion(existing)
	snapshot, err := r.buildCombinedSalesOrderSnapshotTx(ctx, tx, cmd.OrderIDs, settings)
	if err != nil {
		return salesapp.GenerateCombinedSalesOrderDocumentResult{}, err
	}
	pdfBytes, err := r.combinedSalesRenderer.RenderCombinedSalesOrder(snapshot)
	if err != nil {
		return salesapp.GenerateCombinedSalesOrderDocumentResult{}, err
	}
	objectKey := filepath.ToSlash(filepath.Join("combined_sales_order_documents", safeSalesOrderPathPart(snapshot.CombinationKey), fmt.Sprintf("V%d.pdf", versionNo)))
	fileWritten := false
	committed := false
	defer func() {
		if fileWritten && !committed {
			cleanupGeneratedSalesOrderAssetFile(r.assetDir, objectKey)
		}
	}()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(r.assetDir, objectKey)), 0755); err != nil {
		return salesapp.GenerateCombinedSalesOrderDocumentResult{}, err
	}
	if err := os.WriteFile(filepath.Join(r.assetDir, objectKey), pdfBytes, 0644); err != nil {
		return salesapp.GenerateCombinedSalesOrderDocumentResult{}, err
	}
	fileWritten = true
	sum := sha256.Sum256(pdfBytes)
	var assetID int64
	filename := fmt.Sprintf("%s-V%d.pdf", snapshot.CombinedNo, versionNo)
	if err := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.sales_order_assets(kind, filename, content_type, bytes, sha256, object_key, created_by)
		VALUES('combined_sales_order_pdf',$1,'application/pdf',$2,$3,$4,$5)
		RETURNING id`, r.schema), filename, int64(len(pdfBytes)), hex.EncodeToString(sum[:]), objectKey, cmd.Actor).Scan(&assetID); err != nil {
		return salesapp.GenerateCombinedSalesOrderDocumentResult{}, err
	}
	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		return salesapp.GenerateCombinedSalesOrderDocumentResult{}, err
	}
	orderIDsJSON, err := json.Marshal(snapshot.OrderIDs)
	if err != nil {
		return salesapp.GenerateCombinedSalesOrderDocumentResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.combined_sales_order_documents SET is_latest=false WHERE combination_key=$1`, r.schema), snapshot.CombinationKey); err != nil {
		return salesapp.GenerateCombinedSalesOrderDocumentResult{}, err
	}
	var doc salesapp.CombinedSalesOrderDocument
	var orderNosText string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.combined_sales_order_documents(combination_key, customer_id, order_ids, order_nos, version_no, snapshot_json, pdf_asset_id, is_latest, created_by)
		VALUES($1,$2,$3,$4,$5,$6,$7,true,$8)
		RETURNING id, combination_key, customer_id, order_ids, order_nos, version_no, pdf_asset_id, is_latest, to_char(created_at,'YYYY-MM-DD HH24:MI:SS'), created_by`, r.schema),
		snapshot.CombinationKey, snapshot.CustomerID, orderIDsJSON, strings.Join(snapshot.OrderNos, ", "), versionNo, snapshotJSON, assetID, cmd.Actor,
	).Scan(&doc.ID, &doc.CombinationKey, &doc.CustomerID, &orderIDsJSON, &orderNosText, &doc.VersionNo, &doc.PDFAssetID, &doc.IsLatest, &doc.CreatedAt, &doc.CreatedBy); err != nil {
		return salesapp.GenerateCombinedSalesOrderDocumentResult{}, err
	}
	_ = json.Unmarshal(orderIDsJSON, &doc.OrderIDs)
	doc.Snapshot = snapshot
	doc.OrderNos = append([]string(nil), snapshot.OrderNos...)
	doc.DownloadURL = combinedSalesOrderDocumentDownloadURL(doc.ID)
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "combined_sales_order_document", &doc.ID, "create", postgresinfra.StrPtr("version_no"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", versionNo)), postgresinfra.AuditMeta{
		"combination_key": snapshot.CombinationKey,
		"customer_id":     snapshot.CustomerID,
		"order_ids":       snapshot.OrderIDs,
		"order_nos":       strings.Join(snapshot.OrderNos, ", "),
		"version_no":      versionNo,
	}); err != nil {
		return salesapp.GenerateCombinedSalesOrderDocumentResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return salesapp.GenerateCombinedSalesOrderDocumentResult{}, err
	}
	committed = true
	return salesapp.GenerateCombinedSalesOrderDocumentResult{Document: doc, Snapshot: snapshot}, nil
}

func (r Repository) PreviewCombinedDeliveryNoteDocument(ctx context.Context, orderIDs []int64) (salesapp.CombinedDeliveryNotePreview, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return salesapp.CombinedDeliveryNotePreview{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	key := combinedDocumentKey(orderIDs)
	existing, err := r.loadCombinedDeliveryNoteVersionsTx(ctx, tx, key, false)
	if err != nil {
		return salesapp.CombinedDeliveryNotePreview{}, err
	}
	snapshot, err := r.buildCombinedDeliveryNoteSnapshotTx(ctx, tx, orderIDs)
	if err != nil {
		return salesapp.CombinedDeliveryNotePreview{}, err
	}
	return salesapp.CombinedDeliveryNotePreview{
		OrderIDs:      snapshot.OrderIDs,
		OrderNos:      snapshot.OrderNos,
		NextVersionNo: salesdomain.NextCombinedDocumentVersion(existing),
		Snapshot:      snapshot,
	}, nil
}

func (r Repository) PreviewCombinedDeliveryNotePDF(ctx context.Context, orderIDs []int64) (salesapp.CombinedDeliveryNotePreviewPDF, error) {
	preview, err := r.PreviewCombinedDeliveryNoteDocument(ctx, orderIDs)
	if err != nil {
		return salesapp.CombinedDeliveryNotePreviewPDF{}, err
	}
	pdfBytes, err := r.combinedDeliveryRenderer.RenderCombinedDeliveryNotePreview(preview.Snapshot)
	if err != nil {
		return salesapp.CombinedDeliveryNotePreviewPDF{}, err
	}
	return salesapp.CombinedDeliveryNotePreviewPDF{
		Preview:  preview,
		Data:     pdfBytes,
		Filename: fmt.Sprintf("%s-preview.pdf", safeDeliveryNotePathPart(preview.Snapshot.DeliveryNoteNo)),
	}, nil
}

func (r Repository) GenerateCombinedDeliveryNoteDocument(ctx context.Context, cmd salesapp.CombinedDocumentCommand) (salesapp.GenerateCombinedDeliveryNoteDocumentResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return salesapp.GenerateCombinedDeliveryNoteDocumentResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	key := combinedDocumentKey(cmd.OrderIDs)
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext($1)::bigint)", key); err != nil {
		return salesapp.GenerateCombinedDeliveryNoteDocumentResult{}, err
	}
	existing, err := r.loadCombinedDeliveryNoteVersionsTx(ctx, tx, key, true)
	if err != nil {
		return salesapp.GenerateCombinedDeliveryNoteDocumentResult{}, err
	}
	versionNo := salesdomain.NextCombinedDocumentVersion(existing)
	snapshot, err := r.buildCombinedDeliveryNoteSnapshotTx(ctx, tx, cmd.OrderIDs)
	if err != nil {
		return salesapp.GenerateCombinedDeliveryNoteDocumentResult{}, err
	}
	pdfBytes, err := r.combinedDeliveryRenderer.RenderCombinedDeliveryNote(snapshot)
	if err != nil {
		return salesapp.GenerateCombinedDeliveryNoteDocumentResult{}, err
	}
	objectKey := filepath.ToSlash(filepath.Join("combined_delivery_note_documents", safeDeliveryNotePathPart(snapshot.CombinationKey), fmt.Sprintf("V%d.pdf", versionNo)))
	fileWritten := false
	committed := false
	defer func() {
		if fileWritten && !committed {
			cleanupGeneratedDeliveryNoteAssetFile(r.assetDir, objectKey)
		}
	}()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(r.assetDir, objectKey)), 0755); err != nil {
		return salesapp.GenerateCombinedDeliveryNoteDocumentResult{}, err
	}
	if err := os.WriteFile(filepath.Join(r.assetDir, objectKey), pdfBytes, 0644); err != nil {
		return salesapp.GenerateCombinedDeliveryNoteDocumentResult{}, err
	}
	fileWritten = true
	sum := sha256.Sum256(pdfBytes)
	var assetID int64
	filename := fmt.Sprintf("%s-V%d.pdf", snapshot.DeliveryNoteNo, versionNo)
	if err := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.delivery_note_assets(kind, filename, content_type, bytes, sha256, object_key, created_by)
		VALUES('combined_delivery_note_pdf',$1,'application/pdf',$2,$3,$4,$5)
		RETURNING id`, r.schema), filename, int64(len(pdfBytes)), hex.EncodeToString(sum[:]), objectKey, cmd.Actor).Scan(&assetID); err != nil {
		return salesapp.GenerateCombinedDeliveryNoteDocumentResult{}, err
	}
	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		return salesapp.GenerateCombinedDeliveryNoteDocumentResult{}, err
	}
	orderIDsJSON, err := json.Marshal(snapshot.OrderIDs)
	if err != nil {
		return salesapp.GenerateCombinedDeliveryNoteDocumentResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.combined_delivery_note_documents SET is_latest=false WHERE combination_key=$1`, r.schema), snapshot.CombinationKey); err != nil {
		return salesapp.GenerateCombinedDeliveryNoteDocumentResult{}, err
	}
	var doc salesapp.CombinedDeliveryNoteDocument
	var orderNosText string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.combined_delivery_note_documents(combination_key, customer_id, order_ids, order_nos, version_no, snapshot_json, pdf_asset_id, is_latest, created_by)
		VALUES($1,$2,$3,$4,$5,$6,$7,true,$8)
		RETURNING id, combination_key, customer_id, order_ids, order_nos, version_no, pdf_asset_id, is_latest, to_char(created_at,'YYYY-MM-DD HH24:MI:SS'), created_by`, r.schema),
		snapshot.CombinationKey, snapshot.CustomerID, orderIDsJSON, strings.Join(snapshot.OrderNos, ", "), versionNo, snapshotJSON, assetID, cmd.Actor,
	).Scan(&doc.ID, &doc.CombinationKey, &doc.CustomerID, &orderIDsJSON, &orderNosText, &doc.VersionNo, &doc.PDFAssetID, &doc.IsLatest, &doc.CreatedAt, &doc.CreatedBy); err != nil {
		return salesapp.GenerateCombinedDeliveryNoteDocumentResult{}, err
	}
	_ = json.Unmarshal(orderIDsJSON, &doc.OrderIDs)
	doc.Snapshot = snapshot
	doc.OrderNos = append([]string(nil), snapshot.OrderNos...)
	doc.DownloadURL = combinedDeliveryNoteDocumentDownloadURL(doc.ID)
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "combined_delivery_note_document", &doc.ID, "create", postgresinfra.StrPtr("version_no"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", versionNo)), postgresinfra.AuditMeta{
		"combination_key": snapshot.CombinationKey,
		"customer_id":     snapshot.CustomerID,
		"order_ids":       snapshot.OrderIDs,
		"order_nos":       strings.Join(snapshot.OrderNos, ", "),
		"version_no":      versionNo,
	}); err != nil {
		return salesapp.GenerateCombinedDeliveryNoteDocumentResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return salesapp.GenerateCombinedDeliveryNoteDocumentResult{}, err
	}
	committed = true
	return salesapp.GenerateCombinedDeliveryNoteDocumentResult{Document: doc, Snapshot: snapshot}, nil
}

func (r Repository) buildCombinedSalesOrderSnapshotTx(ctx context.Context, tx pgx.Tx, orderIDs []int64, settings salesapp.SalesOrderSettings) (salesdomain.CombinedSalesOrderSnapshot, error) {
	orders, err := r.loadCombinedOrderInfosTx(ctx, tx, orderIDs)
	if err != nil {
		return salesdomain.CombinedSalesOrderSnapshot{}, err
	}
	key := combinedDocumentKey(orderIDs)
	snapshot := salesdomain.CombinedSalesOrderSnapshot{
		CombinationKey: key,
		CustomerID:     orders[0].CustomerID,
		CustomerName:   orders[0].CustomerName,
		OrderIDs:       append([]int64(nil), orderIDs...),
		OrderNos:       combinedOrderNosFromInfos(orders),
	}
	snapshot.CombinedNo = combinedDocumentNo("CSO", snapshot.OrderNos)
	var total, shipping, discount, grand float64
	for _, order := range orders {
		single, err := r.buildSalesOrderSnapshotTx(ctx, tx, order.OrderID, settings)
		if err != nil {
			return salesdomain.CombinedSalesOrderSnapshot{}, err
		}
		if snapshot.CompanyName == "" {
			snapshot.CompanyName = single.CompanyName
			snapshot.CompanyAddress = single.CompanyAddress
			snapshot.CustomerCompanyName = single.CustomerCompanyName
			snapshot.CustomerCompanyAddress = single.CustomerCompanyAddress
			snapshot.CustomerCompanyPhone = single.CustomerCompanyPhone
			snapshot.PaymentText = single.PaymentText
			snapshot.TaxpayerID = single.TaxpayerID
			snapshot.BankAccountName = single.BankAccountName
			snapshot.BankName = single.BankName
			snapshot.BankAccountNo = single.BankAccountNo
			snapshot.Note = single.Note
			snapshot.PaymentCodes = single.PaymentCodes
			snapshot.PaymentTextBox = single.PaymentTextBox
			snapshot.PaymentCodeBox = single.PaymentCodeBox
			snapshot.Seal = single.Seal
		}
		total += parseDocumentMoney(single.TotalAmount)
		shipping += parseDocumentMoney(single.Shipping)
		discount += parseDocumentMoney(single.Discount)
		grand += parseDocumentMoney(single.GrandTotal)
		snapshot.Groups = append(snapshot.Groups, salesdomain.CombinedSalesOrderGroup{
			OrderID:            single.OrderID,
			OrderNo:            single.OrderNo,
			DocumentDate:       single.DocumentDate,
			OrderDate:          single.OrderDate,
			Items:              single.Items,
			TotalAmount:        single.TotalAmount,
			Shipping:           single.Shipping,
			Discount:           single.Discount,
			ExpressFee:         single.ExpressFee,
			SalesOrderNote:     single.SalesOrderNote,
			GrandTotal:         single.GrandTotal,
			DiscountBreakdowns: single.DiscountBreakdowns,
		})
	}
	snapshot.TotalAmount = salesdomain.FormatSalesOrderMoney(total)
	snapshot.Shipping = salesdomain.FormatSalesOrderMoney(shipping)
	snapshot.Discount = salesdomain.FormatSalesOrderMoney(discount)
	snapshot.GrandTotal = salesdomain.FormatSalesOrderMoney(grand)
	if err := snapshot.Validate(); err != nil {
		return salesdomain.CombinedSalesOrderSnapshot{}, err
	}
	return snapshot, nil
}

func (r Repository) buildCombinedDeliveryNoteSnapshotTx(ctx context.Context, tx pgx.Tx, orderIDs []int64) (salesdomain.CombinedDeliveryNoteSnapshot, error) {
	orders, err := r.loadCombinedOrderInfosTx(ctx, tx, orderIDs)
	if err != nil {
		return salesdomain.CombinedDeliveryNoteSnapshot{}, err
	}
	key := combinedDocumentKey(orderIDs)
	snapshot := salesdomain.CombinedDeliveryNoteSnapshot{
		CombinationKey: key,
		CustomerID:     orders[0].CustomerID,
		CustomerName:   orders[0].CustomerName,
		OrderIDs:       append([]int64(nil), orderIDs...),
		OrderNos:       combinedOrderNosFromInfos(orders),
	}
	snapshot.DeliveryNoteNo = combinedDocumentNo("CDN", snapshot.OrderNos)
	for _, order := range orders {
		form, err := r.loadDeliveryNoteFormTx(ctx, tx, order.OrderID)
		if err != nil {
			return salesdomain.CombinedDeliveryNoteSnapshot{}, err
		}
		single, err := r.buildDeliveryNoteSnapshotTx(ctx, tx, order.OrderID, form)
		if err != nil {
			return salesdomain.CombinedDeliveryNoteSnapshot{}, err
		}
		if snapshot.CompanyName == "" {
			snapshot.CompanyName = single.CompanyName
			snapshot.CompanyAddress = single.CompanyAddress
			snapshot.CustomerCompanyName = single.CustomerCompanyName
			snapshot.CustomerCompanyAddress = single.CustomerCompanyAddress
			snapshot.CustomerCompanyPhone = single.CustomerCompanyPhone
			snapshot.Seal = single.Seal
		}
		snapshot.Groups = append(snapshot.Groups, salesdomain.CombinedDeliveryNoteGroup{
			OrderID:             single.OrderID,
			OrderNo:             single.OrderNo,
			DocumentDate:        single.DocumentDate,
			OrderDate:           single.OrderDate,
			PostingDate:         single.PostingDate,
			ReceiverName:        single.ReceiverName,
			ReceiverPhone:       single.ReceiverPhone,
			ReceiverAddress:     single.ReceiverAddress,
			SourceWarehouse:     single.SourceWarehouse,
			SourceWarehouseName: single.SourceWarehouseName,
			DeliveryMethod:      single.DeliveryMethod,
			TrackingNo:          single.TrackingNo,
			Note:                single.Note,
			Items:               single.Items,
		})
	}
	if err := snapshot.Validate(); err != nil {
		return salesdomain.CombinedDeliveryNoteSnapshot{}, err
	}
	return snapshot, nil
}

func (r Repository) loadCombinedOrderInfosTx(ctx context.Context, tx pgx.Tx, orderIDs []int64) ([]combinedOrderInfo, error) {
	if len(orderIDs) < 2 {
		return nil, fmt.Errorf("at least two orders required")
	}
	placeholders := make([]string, 0, len(orderIDs))
	args := make([]any, 0, len(orderIDs))
	for i, id := range orderIDs {
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
		args = append(args, id)
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT o.id, COALESCE(o.order_no,''), COALESCE(c.id,0), COALESCE(c.name,''),
			COALESCE(NULLIF(c.company_name,''), c.name, ''), COALESCE(NULLIF(c.company_address,''), c.address, ''), COALESCE(NULLIF(c.company_phone,''), c.phone, '')
		FROM %s.orders o
		LEFT JOIN %s.customers c ON c.id=o.customer_id
		WHERE o.id IN (%s)`, r.schema, r.schema, strings.Join(placeholders, ",")), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byID := map[int64]combinedOrderInfo{}
	for rows.Next() {
		var info combinedOrderInfo
		if err := rows.Scan(&info.OrderID, &info.OrderNo, &info.CustomerID, &info.CustomerName, &info.CustomerCompanyName, &info.CustomerCompanyAddress, &info.CustomerCompanyPhone); err != nil {
			return nil, err
		}
		byID[info.OrderID] = info
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]combinedOrderInfo, 0, len(orderIDs))
	var customerID int64
	for _, id := range orderIDs {
		info, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("selected order not found")
		}
		if customerID == 0 {
			customerID = info.CustomerID
		}
		if info.CustomerID <= 0 || info.CustomerID != customerID {
			return nil, fmt.Errorf("orders must belong to same customer")
		}
		out = append(out, info)
	}
	return out, nil
}

func (r Repository) loadCombinedSalesOrderVersionsTx(ctx context.Context, tx pgx.Tx, key string, lock bool) ([]int, error) {
	q := fmt.Sprintf(`SELECT version_no FROM %s.combined_sales_order_documents WHERE combination_key=$1`, r.schema)
	if lock {
		q += " FOR UPDATE"
	}
	return loadCombinedDocumentVersions(ctx, tx, q, key)
}

func (r Repository) loadCombinedDeliveryNoteVersionsTx(ctx context.Context, tx pgx.Tx, key string, lock bool) ([]int, error) {
	q := fmt.Sprintf(`SELECT version_no FROM %s.combined_delivery_note_documents WHERE combination_key=$1`, r.schema)
	if lock {
		q += " FOR UPDATE"
	}
	return loadCombinedDocumentVersions(ctx, tx, q, key)
}

func loadCombinedDocumentVersions(ctx context.Context, tx pgx.Tx, query, key string) ([]int, error) {
	rows, err := tx.Query(ctx, query, key)
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
	return existing, rows.Err()
}

func (r Repository) LoadCombinedSalesOrderDocumentFile(ctx context.Context, documentID int64) (salesapp.CombinedSalesOrderDocumentFile, error) {
	q := fmt.Sprintf(`SELECT d.id, d.combination_key, d.customer_id, d.order_ids, d.order_nos, d.version_no, d.snapshot_json, COALESCE(d.pdf_asset_id,0), d.is_latest, to_char(d.created_at,'YYYY-MM-DD HH24:MI:SS'), d.created_by, a.object_key
		FROM %s.combined_sales_order_documents d
		JOIN %s.sales_order_assets a ON a.id=d.pdf_asset_id
		WHERE d.id=$1`, r.schema, r.schema)
	var doc salesapp.CombinedSalesOrderDocument
	var orderIDsJSON, raw []byte
	var orderNosText, objectKey string
	if err := r.pool.QueryRow(ctx, q, documentID).Scan(&doc.ID, &doc.CombinationKey, &doc.CustomerID, &orderIDsJSON, &orderNosText, &doc.VersionNo, &raw, &doc.PDFAssetID, &doc.IsLatest, &doc.CreatedAt, &doc.CreatedBy, &objectKey); err != nil {
		return salesapp.CombinedSalesOrderDocumentFile{}, err
	}
	_ = json.Unmarshal(orderIDsJSON, &doc.OrderIDs)
	_ = json.Unmarshal(raw, &doc.Snapshot)
	doc.OrderNos = doc.Snapshot.OrderNos
	if len(doc.OrderNos) == 0 {
		doc.OrderNos = splitCombinedOrderNos(orderNosText)
	}
	doc.DownloadURL = combinedSalesOrderDocumentDownloadURL(doc.ID)
	return salesapp.CombinedSalesOrderDocumentFile{
		Document: doc,
		Path:     filepath.Join(r.assetDir, objectKey),
		Filename: fmt.Sprintf("%s-V%d.pdf", doc.Snapshot.CombinedNo, doc.VersionNo),
	}, nil
}

func (r Repository) LoadCombinedDeliveryNoteDocumentFile(ctx context.Context, documentID int64) (salesapp.CombinedDeliveryNoteDocumentFile, error) {
	q := fmt.Sprintf(`SELECT d.id, d.combination_key, d.customer_id, d.order_ids, d.order_nos, d.version_no, d.snapshot_json, COALESCE(d.pdf_asset_id,0), d.is_latest, to_char(d.created_at,'YYYY-MM-DD HH24:MI:SS'), d.created_by, a.object_key
		FROM %s.combined_delivery_note_documents d
		JOIN %s.delivery_note_assets a ON a.id=d.pdf_asset_id
		WHERE d.id=$1`, r.schema, r.schema)
	var doc salesapp.CombinedDeliveryNoteDocument
	var orderIDsJSON, raw []byte
	var orderNosText, objectKey string
	if err := r.pool.QueryRow(ctx, q, documentID).Scan(&doc.ID, &doc.CombinationKey, &doc.CustomerID, &orderIDsJSON, &orderNosText, &doc.VersionNo, &raw, &doc.PDFAssetID, &doc.IsLatest, &doc.CreatedAt, &doc.CreatedBy, &objectKey); err != nil {
		return salesapp.CombinedDeliveryNoteDocumentFile{}, err
	}
	_ = json.Unmarshal(orderIDsJSON, &doc.OrderIDs)
	_ = json.Unmarshal(raw, &doc.Snapshot)
	doc.OrderNos = doc.Snapshot.OrderNos
	if len(doc.OrderNos) == 0 {
		doc.OrderNos = splitCombinedOrderNos(orderNosText)
	}
	doc.DownloadURL = combinedDeliveryNoteDocumentDownloadURL(doc.ID)
	return salesapp.CombinedDeliveryNoteDocumentFile{
		Document: doc,
		Path:     filepath.Join(r.assetDir, objectKey),
		Filename: fmt.Sprintf("%s-V%d.pdf", doc.Snapshot.DeliveryNoteNo, doc.VersionNo),
	}, nil
}

func combinedDocumentKey(orderIDs []int64) string {
	ids := append([]int64(nil), orderIDs...)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, strconv.FormatInt(id, 10))
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, ",")))
	return "orders-" + hex.EncodeToString(sum[:])[:16]
}

func combinedOrderNosFromInfos(orders []combinedOrderInfo) []string {
	out := make([]string, 0, len(orders))
	for _, order := range orders {
		out = append(out, order.OrderNo)
	}
	return out
}

func combinedDocumentNo(prefix string, orderNos []string) string {
	parts := make([]string, 0, len(orderNos))
	for _, no := range orderNos {
		if trimmed := strings.TrimSpace(no); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	if len(parts) == 0 {
		return prefix + "-" + time.Now().Format("20060102")
	}
	return prefix + "-" + strings.Join(parts, "-")
}

func parseDocumentMoney(value string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return v
}

func splitCombinedOrderNos(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func combinedSalesOrderDocumentDownloadURL(documentID int64) string {
	return fmt.Sprintf("/orders/combined/sales-orders/%d.pdf", documentID)
}

func combinedDeliveryNoteDocumentDownloadURL(documentID int64) string {
	return fmt.Sprintf("/orders/combined/delivery-notes/%d.pdf", documentID)
}
