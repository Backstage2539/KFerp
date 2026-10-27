package sales

import (
	"context"
	"encoding/json"
	"fmt"
	salesapp "orderapp/internal/application/sales"
	salesdomain "orderapp/internal/domain/sales"
	pdfinfra "orderapp/internal/infrastructure/pdf"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSalesOrderSettingsRoundTrip(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	prepareSalesSchemaPrerequisites(t, ctx, pool, schema)
	repo := NewRepository(pool, schema)

	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	if err := repo.SaveSalesOrderSettings(ctx, salesapp.SaveSalesOrderSettingsCommand{
		Actor: "测试员", CompanyName: "浅焙作坊咖啡", Note: "请密封保存", PaymentText: "微信或对公转账",
		BankAccountName: "孟连口加农业科技有限公司", BankName: "中国农业银行孟连支行", BankAccountNo: "6222000000000000",
		PaymentTextXMM: 18, PaymentTextYMM: 142, PaymentTextWidthMM: 98, PaymentTextHeightMM: 54, PaymentTextPageNumber: 1,
		PaymentCodeXMM: 126, PaymentCodeYMM: 104, PaymentCodeWidthMM: 76, PaymentCodeHeightMM: 126, PaymentCodePageNumber: 2,
	}); err != nil {
		t.Fatalf("SaveSalesOrderSettings: %v", err)
	}
	got, err := repo.LoadSalesOrderSettings(ctx)
	if err != nil {
		t.Fatalf("LoadSalesOrderSettings: %v", err)
	}
	if got.CompanyName != "浅焙作坊咖啡" || got.Note != "请密封保存" || got.PaymentText != "微信或对公转账" {
		t.Fatalf("settings = %+v", got)
	}
	if got.BankAccountName != "孟连口加农业科技有限公司" || got.BankName != "中国农业银行孟连支行" || got.BankAccountNo != "6222000000000000" {
		t.Fatalf("bank account settings = %+v", got)
	}
	if got.PaymentTextXMM != 18 || got.PaymentTextYMM != 142 || got.PaymentTextWidthMM != 98 || got.PaymentTextHeightMM != 54 {
		t.Fatalf("payment text layout settings = %+v", got)
	}
	if got.PaymentTextPageNumber != 1 {
		t.Fatalf("payment text layout page = %+v", got)
	}
	if got.PaymentCodeXMM != 126 || got.PaymentCodeYMM != 104 || got.PaymentCodeWidthMM != 76 || got.PaymentCodeHeightMM != 126 {
		t.Fatalf("payment code layout settings = %+v", got)
	}
	if got.PaymentCodePageNumber != 2 {
		t.Fatalf("payment code layout page = %+v", got)
	}
}

func TestSalesOrderPaymentCodeRoundTrip(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	prepareSalesSchemaPrerequisites(t, ctx, pool, schema)
	repo := NewRepository(pool, schema)
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	asset, err := repo.SaveSalesOrderAsset(ctx, salesapp.SaveSalesOrderAssetCommand{
		Actor: "测试员", Kind: "payment_code", Filename: "wx.png", ContentType: "image/png", Bytes: 12, SHA256: "abc", ObjectKey: "sales-order/payment/wx.png",
	})
	if err != nil {
		t.Fatalf("SaveSalesOrderAsset: %v", err)
	}
	code, err := repo.SaveSalesOrderPaymentCode(ctx, salesapp.SaveSalesOrderPaymentCodeCommand{
		Actor: "测试员", Label: "微信", Description: "扫码付款", AssetID: asset.ID, Sort: 10, Active: true,
	})
	if err != nil {
		t.Fatalf("SaveSalesOrderPaymentCode: %v", err)
	}
	settings, err := repo.LoadSalesOrderSettings(ctx)
	if err != nil {
		t.Fatalf("LoadSalesOrderSettings: %v", err)
	}
	if len(settings.PaymentCodes) != 1 || settings.PaymentCodes[0].ID != code.ID || settings.PaymentCodes[0].Label != "微信" || settings.PaymentCodes[0].Asset.ObjectKey != "sales-order/payment/wx.png" {
		t.Fatalf("payment codes = %+v", settings.PaymentCodes)
	}
}

func TestDeactivateSalesOrderPaymentCodeKeepsVisibleForSettingsAndCanReactivate(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	prepareSalesSchemaPrerequisites(t, ctx, pool, schema)
	repo := NewRepository(pool, schema)
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	asset, err := repo.SaveSalesOrderAsset(ctx, salesapp.SaveSalesOrderAssetCommand{
		Actor: "测试员", Kind: "payment_code", Filename: "wx.png", ContentType: "image/png", Bytes: 12, SHA256: "abc", ObjectKey: "sales-order/payment/wx.png",
	})
	if err != nil {
		t.Fatalf("SaveSalesOrderAsset: %v", err)
	}
	code, err := repo.SaveSalesOrderPaymentCode(ctx, salesapp.SaveSalesOrderPaymentCodeCommand{
		Actor: "测试员", Label: "微信", Description: "扫码付款", AssetID: asset.ID, Sort: 10, Active: true,
	})
	if err != nil {
		t.Fatalf("SaveSalesOrderPaymentCode: %v", err)
	}

	if err := repo.DeactivateSalesOrderPaymentCode(ctx, code.ID, "测试员"); err != nil {
		t.Fatalf("DeactivateSalesOrderPaymentCode: %v", err)
	}

	settings, err := repo.LoadSalesOrderSettings(ctx)
	if err != nil {
		t.Fatalf("LoadSalesOrderSettings: %v", err)
	}
	if len(settings.PaymentCodes) != 1 || settings.PaymentCodes[0].ID != code.ID || settings.PaymentCodes[0].Active {
		t.Fatalf("inactive payment code should remain visible in settings with active=false, got %+v", settings.PaymentCodes)
	}
	var codeRows, assetRows int
	var active bool
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*)::int, COALESCE(bool_or(active), false) FROM %s.sales_order_payment_codes WHERE id=$1`, schema), code.ID).Scan(&codeRows, &active); err != nil {
		t.Fatalf("query payment code row: %v", err)
	}
	if codeRows != 1 || active {
		t.Fatalf("payment code row count/active = %d/%v, want 1/false", codeRows, active)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*)::int FROM %s.sales_order_assets WHERE id=$1`, schema), asset.ID).Scan(&assetRows); err != nil {
		t.Fatalf("query payment code asset row: %v", err)
	}
	if assetRows != 1 {
		t.Fatalf("payment code asset rows = %d, want 1", assetRows)
	}
	var action string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT action FROM %s.audit_logs WHERE entity_type='sales_order_payment_code' AND entity_id=$1 AND field='active' ORDER BY id DESC LIMIT 1`, schema), code.ID).Scan(&action); err != nil {
		t.Fatalf("query payment code audit action: %v", err)
	}
	if action != "deactivate" {
		t.Fatalf("payment code audit action = %q, want deactivate", action)
	}

	if err := repo.ActivateSalesOrderPaymentCode(ctx, code.ID, "测试员"); err != nil {
		t.Fatalf("ActivateSalesOrderPaymentCode: %v", err)
	}
	settings, err = repo.LoadSalesOrderSettings(ctx)
	if err != nil {
		t.Fatalf("LoadSalesOrderSettings after activate: %v", err)
	}
	if len(settings.PaymentCodes) != 1 || settings.PaymentCodes[0].ID != code.ID || !settings.PaymentCodes[0].Active {
		t.Fatalf("reactivated payment code should remain visible in settings with active=true, got %+v", settings.PaymentCodes)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT action FROM %s.audit_logs WHERE entity_type='sales_order_payment_code' AND entity_id=$1 AND field='active' ORDER BY id DESC LIMIT 1`, schema), code.ID).Scan(&action); err != nil {
		t.Fatalf("query payment code activate audit action: %v", err)
	}
	if action != "activate" {
		t.Fatalf("payment code audit action = %q, want activate", action)
	}

	if err := repo.DeleteSalesOrderPaymentCode(ctx, code.ID, "测试员"); err != nil {
		t.Fatalf("DeleteSalesOrderPaymentCode: %v", err)
	}
	settings, err = repo.LoadSalesOrderSettings(ctx)
	if err != nil {
		t.Fatalf("LoadSalesOrderSettings after delete: %v", err)
	}
	if len(settings.PaymentCodes) != 0 {
		t.Fatalf("deleted payment code should not remain visible in settings, got %+v", settings.PaymentCodes)
	}
	var deletedAtCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*)::int FROM %s.sales_order_payment_codes WHERE id=$1 AND deleted_at IS NOT NULL AND deleted_by='测试员'`, schema), code.ID).Scan(&deletedAtCount); err != nil {
		t.Fatalf("query deleted payment code row: %v", err)
	}
	if deletedAtCount != 1 {
		t.Fatalf("deleted payment code marker rows = %d, want 1", deletedAtCount)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*)::int FROM %s.sales_order_assets WHERE id=$1`, schema), asset.ID).Scan(&assetRows); err != nil {
		t.Fatalf("query deleted payment code asset row: %v", err)
	}
	if assetRows != 1 {
		t.Fatalf("deleted payment code should keep asset row, got %d", assetRows)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT action FROM %s.audit_logs WHERE entity_type='sales_order_payment_code' AND entity_id=$1 AND field='deleted_at' ORDER BY id DESC LIMIT 1`, schema), code.ID).Scan(&action); err != nil {
		t.Fatalf("query payment code delete audit action: %v", err)
	}
	if action != "delete" {
		t.Fatalf("payment code audit action = %q, want delete", action)
	}
}

func TestSalesOrderDocumentSkipsInactivePaymentCodes(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	prepareSalesSchemaPrerequisites(t, ctx, pool, schema)
	repo := NewRepository(pool, schema, WithSalesOrderAssetDir(t.TempDir()), WithSalesOrderRenderer(fakeSalesOrderRenderer{}))
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	seedSalesOrderDocumentOrder(t, ctx, pool, schema)
	if err := repo.SaveSalesOrderSettings(ctx, salesapp.SaveSalesOrderSettingsCommand{Actor: "测试员", CompanyName: "浅焙作坊咖啡"}); err != nil {
		t.Fatalf("SaveSalesOrderSettings: %v", err)
	}
	asset, err := repo.SaveSalesOrderAsset(ctx, salesapp.SaveSalesOrderAssetCommand{
		Actor: "测试员", Kind: "payment_code", Filename: "wx.png", ContentType: "image/png", Bytes: 12, SHA256: "abc", ObjectKey: "sales-order/payment/wx.png",
	})
	if err != nil {
		t.Fatalf("SaveSalesOrderAsset: %v", err)
	}
	code, err := repo.SaveSalesOrderPaymentCode(ctx, salesapp.SaveSalesOrderPaymentCodeCommand{
		Actor: "测试员", Label: "微信", Description: "扫码付款", AssetID: asset.ID, Sort: 10, Active: false,
	})
	if err != nil {
		t.Fatalf("SaveSalesOrderPaymentCode inactive: %v", err)
	}
	inactiveDoc, err := repo.GenerateSalesOrderDocument(ctx, salesapp.GenerateSalesOrderDocumentCommand{Actor: "测试员", OrderID: 1})
	if err != nil {
		t.Fatalf("Generate inactive: %v", err)
	}
	if len(inactiveDoc.Snapshot.PaymentCodes) != 0 {
		t.Fatalf("inactive payment code should not render in snapshot: %+v", inactiveDoc.Snapshot.PaymentCodes)
	}
	if err := repo.ActivateSalesOrderPaymentCode(ctx, code.ID, "测试员"); err != nil {
		t.Fatalf("ActivateSalesOrderPaymentCode: %v", err)
	}
	activeDoc, err := repo.GenerateSalesOrderDocument(ctx, salesapp.GenerateSalesOrderDocumentCommand{Actor: "测试员", OrderID: 1})
	if err != nil {
		t.Fatalf("Generate active: %v", err)
	}
	if len(activeDoc.Snapshot.PaymentCodes) != 1 || activeDoc.Snapshot.PaymentCodes[0].Label != "微信" {
		t.Fatalf("active payment code should render in snapshot: %+v", activeDoc.Snapshot.PaymentCodes)
	}
}

func TestGenerateSalesOrderDocumentCreatesVersions(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	prepareSalesSchemaPrerequisites(t, ctx, pool, schema)
	repo := NewRepository(pool, schema, WithSalesOrderAssetDir(t.TempDir()), WithSalesOrderRenderer(fakeSalesOrderRenderer{}))
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	seedSalesOrderDocumentOrder(t, ctx, pool, schema)
	if err := repo.SaveSalesOrderSettings(ctx, salesapp.SaveSalesOrderSettingsCommand{
		Actor: "测试员", CompanyName: "浅焙作坊咖啡", Note: "请密封保存",
	}); err != nil {
		t.Fatalf("SaveSalesOrderSettings: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.company_profile(id, company_name, company_address, taxpayer_id, bank_account_name, bank_name, bank_account_no) VALUES(1, '棵凡咖啡', '云南省普洱市孟连县', '91530827MACGJ29D6J', '孟连口加农业科技有限公司', '中国农业银行孟连支行', '6222000000000000')`, schema)); err != nil {
		t.Fatalf("insert company profile: %v", err)
	}

	first, err := repo.GenerateSalesOrderDocument(ctx, salesapp.GenerateSalesOrderDocumentCommand{Actor: "测试员", OrderID: 1})
	if err != nil {
		t.Fatalf("Generate first: %v", err)
	}
	if first.Snapshot.CompanyName != "棵凡咖啡" {
		t.Fatalf("generated snapshot company_name = %q, want global company profile name", first.Snapshot.CompanyName)
	}
	if first.Snapshot.RenderVersion != salesdomain.SalesOrderRenderVersion {
		t.Fatalf("generated snapshot render_version = %q, want %q", first.Snapshot.RenderVersion, salesdomain.SalesOrderRenderVersion)
	}
	if first.Snapshot.BankAccountName != "孟连口加农业科技有限公司" || first.Snapshot.BankName != "中国农业银行孟连支行" || first.Snapshot.BankAccountNo != "6222000000000000" {
		t.Fatalf("generated snapshot bank account fields = %+v", first.Snapshot)
	}
	if first.Snapshot.CompanyAddress != "云南省普洱市孟连县" || first.Snapshot.TaxpayerID != "91530827MACGJ29D6J" {
		t.Fatalf("generated snapshot company account identity fields = %+v", first.Snapshot)
	}
	second, err := repo.GenerateSalesOrderDocument(ctx, salesapp.GenerateSalesOrderDocumentCommand{Actor: "测试员", OrderID: 1})
	if err != nil {
		t.Fatalf("Generate second: %v", err)
	}
	if first.Document.VersionNo != 1 || second.Document.VersionNo != 2 || !second.Document.IsLatest {
		t.Fatalf("versions first=%+v second=%+v", first.Document, second.Document)
	}
	docs, err := repo.ListSalesOrderDocuments(ctx, 1)
	if err != nil {
		t.Fatalf("ListSalesOrderDocuments: %v", err)
	}
	if len(docs) != 2 || docs[0].VersionNo != 2 || !docs[0].IsLatest || docs[1].VersionNo != 1 || docs[1].IsLatest {
		t.Fatalf("docs = %+v", docs)
	}
	file, err := repo.LoadSalesOrderDocumentFile(ctx, 1, 0, true)
	if err != nil {
		t.Fatalf("LoadSalesOrderDocumentFile latest: %v", err)
	}
	b, err := os.ReadFile(file.Path)
	if err != nil {
		t.Fatalf("read pdf: %v", err)
	}
	if string(b) != "%PDF-test" || file.Filename != "SO-20260430-0008-V2.pdf" {
		t.Fatalf("file=%+v bytes=%q", file, b)
	}
	if file.Document.Snapshot.RenderVersion != salesdomain.SalesOrderRenderVersion {
		t.Fatalf("loaded document render_version = %q, want %q", file.Document.Snapshot.RenderVersion, salesdomain.SalesOrderRenderVersion)
	}
}

func TestSalesOrderPreviewIncludesNoteAndDiscountBreakdowns(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	prepareSalesSchemaPrerequisites(t, ctx, pool, schema)
	repo := NewRepository(pool, schema, WithSalesOrderAssetDir(t.TempDir()), WithSalesOrderRenderer(fakeSalesOrderRenderer{}))
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	seedSalesOrderDocumentOrder(t, ctx, pool, schema)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS notes TEXT`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET total_amount=2455, shipping_amount=169, discount_amount=261.65, grand_total=2362.35, notes='订单原始备注：到店后联系', express_fee='顺丰保价备注' WHERE id=1`, schema)); err != nil {
		t.Fatalf("update order amounts: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_items SET discount_type='unit_amount', discount_amount=100 WHERE order_id=1 AND line_no=1`, schema)); err != nil {
		t.Fatalf("update first item discount: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_items(order_id, line_no, product_id, item_name, spec, qty, unit, unit_price, discount_type, discount_amount, line_total)
		VALUES(1, 2, 1, '芬纳-曲奇定制（20%%乌干达，15%%云南厌氧日晒，65%%云南水洗）', '1000g', 2, '件', 117, 'percent', 61.65, 93.35)`, schema)); err != nil {
		t.Fatalf("insert discounted item: %v", err)
	}

	if err := repo.SaveSalesOrderNote(ctx, salesapp.SaveSalesOrderNoteCommand{Actor: "销售", OrderID: 1, Note: "  末行备注：随货附赠杯测样  "}); err != nil {
		t.Fatalf("SaveSalesOrderNote: %v", err)
	}
	preview, err := repo.PreviewSalesOrderDocument(ctx, 1)
	if err != nil {
		t.Fatalf("PreviewSalesOrderDocument: %v", err)
	}
	if preview.Snapshot.OrderNote != "订单原始备注：到店后联系" || preview.Snapshot.SalesOrderNote != "末行备注：随货附赠杯测样" || preview.Snapshot.ExpressFee != "顺丰保价备注" || preview.Snapshot.Shipping != "169.00" || preview.Snapshot.Discount != "261.65" {
		t.Fatalf("snapshot financial fields = %+v", preview.Snapshot)
	}
	if len(preview.Snapshot.Items) < 2 || preview.Snapshot.Items[0].DiscountAmount != "100.00" || preview.Snapshot.Items[1].DiscountAmount != "61.65" {
		t.Fatalf("snapshot item discounts = %+v", preview.Snapshot.Items)
	}
	want := []salesdomain.SalesOrderDiscountBreakdown{
		{Type: "unit_amount", Amount: "100.00"},
		{Type: "percent", Amount: "61.65"},
		{Type: "order_amount", Amount: "100.00"},
	}
	if fmt.Sprint(preview.Snapshot.DiscountBreakdowns) != fmt.Sprint(want) {
		t.Fatalf("discount breakdowns = %+v, want %+v", preview.Snapshot.DiscountBreakdowns, want)
	}

	var auditCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.audit_logs WHERE entity_type='order' AND entity_id=1 AND field='sales_order_note' AND old_value='' AND new_value='末行备注：随货附赠杯测样'`, schema)).Scan(&auditCount); err != nil {
		t.Fatalf("query audit: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("sales_order_note audit count = %d, want 1", auditCount)
	}
}

func TestSalesOrderPreviewPreservesPublishedSalesSpecCountSnapshot(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	prepareSalesSchemaPrerequisites(t, ctx, pool, schema)
	repo := NewRepository(pool, schema, WithSalesOrderAssetDir(t.TempDir()), WithSalesOrderRenderer(fakeSalesOrderRenderer{}))
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	seedSalesOrderDocumentOrder(t, ctx, pool, schema)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.order_items
		SET spec='1Kg',
		    qty=30,
		    unit='1Kg',
		    item_note='2.5Kg袋装，共12袋',
		    price_source_json='{"quantity_basis":"sales_spec_count"}'::jsonb
		WHERE order_id=1 AND line_no=1
	`, schema)); err != nil {
		t.Fatalf("update sales spec count order item: %v", err)
	}
	preview, err := repo.PreviewSalesOrderDocument(ctx, 1)
	if err != nil {
		t.Fatalf("PreviewSalesOrderDocument: %v", err)
	}
	if len(preview.Snapshot.Items) != 1 {
		t.Fatalf("snapshot items=%+v", preview.Snapshot.Items)
	}
	item := preview.Snapshot.Items[0]
	if item.Spec != "1Kg" || item.Qty != "30" || item.Unit != "1Kg" || item.Note != "2.5Kg袋装，共12袋" {
		t.Fatalf("sales spec count snapshot item=%+v", item)
	}
	payload, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `"quantity_basis":"sales_spec_count"`) {
		t.Fatalf("sales spec count snapshot missing quantity basis: %s", payload)
	}
	if preview.Snapshot.SalesOrderNote != "" {
		t.Fatalf("sales order note=%q, want empty; line note must stay on item", preview.Snapshot.SalesOrderNote)
	}
}

func TestGenerateSalesOrderImageCreatesIndependentImageVersions(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	prepareSalesSchemaPrerequisites(t, ctx, pool, schema)
	assetDir := t.TempDir()
	repo := NewRepository(pool, schema, WithSalesOrderAssetDir(assetDir), WithSalesOrderRenderer(fakeSalesOrderRenderer{}))
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	seedSalesOrderDocumentOrder(t, ctx, pool, schema)
	if err := repo.SaveSalesOrderSettings(ctx, salesapp.SaveSalesOrderSettingsCommand{
		Actor: "测试员", CompanyName: "浅焙作坊咖啡", Note: "请密封保存",
	}); err != nil {
		t.Fatalf("SaveSalesOrderSettings: %v", err)
	}

	pdf, err := repo.GenerateSalesOrderDocument(ctx, salesapp.GenerateSalesOrderDocumentCommand{Actor: "测试员", OrderID: 1})
	if err != nil {
		t.Fatalf("Generate PDF: %v", err)
	}
	first, err := repo.GenerateSalesOrderImage(ctx, salesapp.GenerateSalesOrderImageCommand{Actor: "测试员", OrderID: 1})
	if err != nil {
		t.Fatalf("Generate first image: %v", err)
	}
	second, err := repo.GenerateSalesOrderImage(ctx, salesapp.GenerateSalesOrderImageCommand{Actor: "测试员", OrderID: 1})
	if err != nil {
		t.Fatalf("Generate second image: %v", err)
	}
	if first.Document.VersionNo != 1 || second.Document.VersionNo != 2 || !second.Document.IsLatest {
		t.Fatalf("image versions first=%+v second=%+v", first.Document, second.Document)
	}
	if pdfDocs, err := repo.ListSalesOrderDocuments(ctx, 1); err != nil || len(pdfDocs) != 1 || !pdfDocs[0].IsLatest || pdfDocs[0].ID != pdf.Document.ID {
		t.Fatalf("PDF latest should remain independent of image generation, docs=%+v err=%v", pdfDocs, err)
	}
	images, err := repo.ListSalesOrderImageDocuments(ctx, 1)
	if err != nil {
		t.Fatalf("ListSalesOrderImageDocuments: %v", err)
	}
	if len(images) != 2 || images[0].VersionNo != 2 || !images[0].IsLatest || images[0].DownloadURL == "" || images[1].IsLatest {
		t.Fatalf("images = %+v", images)
	}
	file, err := repo.LoadSalesOrderImageFile(ctx, 1, 0, true)
	if err != nil {
		t.Fatalf("LoadSalesOrderImageFile latest: %v", err)
	}
	b, err := os.ReadFile(file.Path)
	if err != nil {
		t.Fatalf("read png: %v", err)
	}
	if string(b) != "\x89PNG\r\n\x1a\nimage-test" || file.Filename != "SO-20260430-0008-V2.png" {
		t.Fatalf("file=%+v bytes=%q", file, b)
	}
}

func TestGenerateSalesOrderDocumentCleansFileWhenDocumentInsertFails(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	prepareSalesSchemaPrerequisites(t, ctx, pool, schema)
	assetDir := t.TempDir()
	repo := NewRepository(pool, schema, WithSalesOrderAssetDir(assetDir), WithSalesOrderRenderer(fakeSalesOrderRenderer{}))
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	seedSalesOrderDocumentOrder(t, ctx, pool, schema)
	if err := repo.SaveSalesOrderSettings(ctx, salesapp.SaveSalesOrderSettingsCommand{
		Actor: "测试员", CompanyName: "浅焙作坊咖啡", Note: "请密封保存",
	}); err != nil {
		t.Fatalf("SaveSalesOrderSettings: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.sales_order_documents ADD CONSTRAINT sales_order_document_test_reject_latest CHECK (is_latest = false)`, schema)); err != nil {
		t.Fatalf("add reject latest constraint: %v", err)
	}

	if _, err := repo.GenerateSalesOrderDocument(ctx, salesapp.GenerateSalesOrderDocumentCommand{Actor: "测试员", OrderID: 1}); err == nil {
		t.Fatalf("GenerateSalesOrderDocument should fail when document insert is rejected")
	}
	assertSalesPostgresAssetDirEmpty(t, assetDir)
}

func TestGenerateSalesOrderImageCleansFileWhenImageInsertFails(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	prepareSalesSchemaPrerequisites(t, ctx, pool, schema)
	assetDir := t.TempDir()
	repo := NewRepository(pool, schema, WithSalesOrderAssetDir(assetDir), WithSalesOrderRenderer(fakeSalesOrderRenderer{}))
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	seedSalesOrderDocumentOrder(t, ctx, pool, schema)
	if err := repo.SaveSalesOrderSettings(ctx, salesapp.SaveSalesOrderSettingsCommand{
		Actor: "测试员", CompanyName: "浅焙作坊咖啡", Note: "请密封保存",
	}); err != nil {
		t.Fatalf("SaveSalesOrderSettings: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.sales_order_images ADD CONSTRAINT sales_order_image_test_reject_latest CHECK (is_latest = false)`, schema)); err != nil {
		t.Fatalf("add reject latest constraint: %v", err)
	}

	if _, err := repo.GenerateSalesOrderImage(ctx, salesapp.GenerateSalesOrderImageCommand{Actor: "测试员", OrderID: 1}); err == nil {
		t.Fatalf("GenerateSalesOrderImage should fail when image insert is rejected")
	}
	assertSalesPostgresAssetDirEmpty(t, assetDir)
}

func TestNewRepositoryPassesAssetDirToDefaultSalesOrderRenderer(t *testing.T) {
	dir := t.TempDir()
	repo := NewRepository(nil, "public", WithSalesOrderAssetDir(dir))
	renderer, ok := repo.renderer.(pdfinfra.SalesOrderRenderer)
	if !ok {
		t.Fatalf("renderer type = %T, want pdf.SalesOrderRenderer", repo.renderer)
	}
	if renderer.AssetBaseDir != dir {
		t.Fatalf("renderer AssetBaseDir = %q, want %q", renderer.AssetBaseDir, dir)
	}
}

type fakeSalesOrderRenderer struct{}

func (fakeSalesOrderRenderer) Render(snapshot salesdomain.SalesOrderSnapshot) ([]byte, error) {
	if err := snapshot.Validate(); err != nil {
		return nil, err
	}
	return []byte("%PDF-test"), nil
}

func (fakeSalesOrderRenderer) RenderPreview(snapshot salesdomain.SalesOrderSnapshot) ([]byte, error) {
	if err := snapshot.Validate(); err != nil {
		return nil, err
	}
	return []byte("%PDF-preview-test"), nil
}

func (fakeSalesOrderRenderer) RenderPNG(snapshot salesdomain.SalesOrderSnapshot) ([]byte, error) {
	if err := snapshot.Validate(); err != nil {
		return nil, err
	}
	return []byte("\x89PNG\r\n\x1a\nimage-test"), nil
}

func newSalesPostgresTestDB(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required for sales postgres tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	schema := fmt.Sprintf("test_sales_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		pool.Close()
		t.Fatalf("create schema: %v", err)
	}
	return pool, schema
}

func prepareSalesSchemaPrerequisites(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) {
	t.Helper()
	stmts := []string{
		fmt.Sprintf(`CREATE TABLE %s.pay_statuses(id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.order_process_statuses (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL, sort INTEGER NOT NULL DEFAULT 0, active BOOLEAN NOT NULL DEFAULT true)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.ship_statuses (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.customers (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			company_name TEXT NOT NULL DEFAULT '',
			company_address TEXT NOT NULL DEFAULT '',
			company_phone TEXT NOT NULL DEFAULT '',
			contact TEXT NOT NULL DEFAULT '',
			phone TEXT NOT NULL DEFAULT '',
			address TEXT NOT NULL DEFAULT ''
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.company_employees (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			active BOOLEAN NOT NULL DEFAULT true
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.company_profile (
			id INTEGER PRIMARY KEY DEFAULT 1,
			company_name TEXT NOT NULL DEFAULT '',
			company_address TEXT NOT NULL DEFAULT '',
			company_phone TEXT NOT NULL DEFAULT '',
			taxpayer_id TEXT NOT NULL DEFAULT '',
			bank_account_name TEXT NOT NULL DEFAULT '',
			bank_name TEXT NOT NULL DEFAULT '',
			bank_account_no TEXT NOT NULL DEFAULT '',
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_by TEXT NOT NULL DEFAULT '',
			CONSTRAINT company_profile_singleton CHECK (id = 1)
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.products (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL DEFAULT '')`, schema),
		fmt.Sprintf(`CREATE TABLE %s.orders (
			id BIGSERIAL PRIMARY KEY,
			order_no TEXT NOT NULL DEFAULT '',
            pay_status_id BIGINT,
            prepayment_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
			order_date DATE,
			customer_id BIGINT,
			total_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
			shipping_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
			discount_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
			express_fee TEXT NOT NULL DEFAULT '',
			sales_order_note TEXT NOT NULL DEFAULT '',
			grand_total NUMERIC(12,2) NOT NULL DEFAULT 0
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.order_items (
			id BIGSERIAL PRIMARY KEY,
			order_id BIGINT NOT NULL,
			line_no INTEGER NOT NULL DEFAULT 0,
			product_id BIGINT,
			item_name TEXT NOT NULL DEFAULT '',
			item_note TEXT NOT NULL DEFAULT '',
			spec TEXT NOT NULL DEFAULT '',
			qty NUMERIC(12,2) NOT NULL DEFAULT 0,
			unit TEXT NOT NULL DEFAULT '',
			unit_price NUMERIC(12,2) NOT NULL DEFAULT 0,
			discount_type TEXT NOT NULL DEFAULT '',
			discount_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
			line_total NUMERIC(12,2) NOT NULL DEFAULT 0
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.audit_logs (
			id BIGSERIAL PRIMARY KEY,
			ts TIMESTAMPTZ NOT NULL DEFAULT now(),
			actor TEXT NOT NULL DEFAULT '',
			entity_type TEXT NOT NULL DEFAULT '',
			entity_id BIGINT,
			action TEXT NOT NULL DEFAULT '',
			field TEXT,
			old_value TEXT,
			new_value TEXT,
			meta JSONB
		)`, schema),
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("prepare schema: %v", err)
		}
	}
}

func assertSalesPostgresAssetDirEmpty(t *testing.T, assetDir string) {
	t.Helper()
	entries, err := os.ReadDir(assetDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("asset dir should be empty after failed sales order generation, entries=%+v", entries)
	}
}

func seedSalesOrderDocumentOrder(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) {
	t.Helper()
	stmts := []string{
		fmt.Sprintf(`INSERT INTO %s.customers(id, name, company_name, company_address, company_phone) VALUES(1, '某某咖啡馆', '某某咖啡贸易公司', '上海市徐汇区', '021-12345678')`, schema),
		fmt.Sprintf(`INSERT INTO %s.products(id, name) VALUES(1, '橘皮乌龙')`, schema),
		fmt.Sprintf(`INSERT INTO %s.orders(id, order_no, order_date, customer_id, total_amount, shipping_amount, discount_amount, grand_total)
			VALUES(1, 'SO-20260430-0008', '2026-04-30', 1, 134, 0, 0, 134)`, schema),
		fmt.Sprintf(`INSERT INTO %s.order_items(order_id, line_no, product_id, item_name, spec, qty, unit, unit_price, line_total)
			VALUES(1, 1, 1, '橘皮乌龙', '300g', 2, '件', 67, 134)`, schema),
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("seed order: %v", err)
		}
	}
}

func TestSourceWarehouseShipmentDeductsBatchAndSummaryTogether(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	for _, stmt := range []string{
		fmt.Sprintf(`CREATE TABLE %s.products(id BIGINT PRIMARY KEY,name TEXT NOT NULL)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.orders(id BIGINT PRIMARY KEY,source_warehouse TEXT NOT NULL)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.order_items(
			id BIGSERIAL PRIMARY KEY,order_id BIGINT NOT NULL,product_id BIGINT,bom_spec_id BIGINT,bom_variant_id BIGINT,
			item_name TEXT NOT NULL,spec TEXT NOT NULL,qty NUMERIC NOT NULL
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.product_bom_spec_authorities(product_id BIGINT PRIMARY KEY,state TEXT NOT NULL)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.stock_batches(
			id BIGSERIAL PRIMARY KEY,batch_code TEXT NOT NULL,item_type TEXT NOT NULL,item_id BIGINT NOT NULL,item_name TEXT NOT NULL,
			bom_spec_id BIGINT NOT NULL DEFAULT 0,bom_variant_id BIGINT NOT NULL DEFAULT 0,spec_g BIGINT NOT NULL,
			remaining_g BIGINT NOT NULL,remaining_units BIGINT NOT NULL,quality_status TEXT NOT NULL DEFAULT 'unchecked',created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.stock_ledger_entries(
			id BIGSERIAL PRIMARY KEY,item_type TEXT NOT NULL,item_id BIGINT NOT NULL,item_name TEXT NOT NULL,
			bom_spec_id BIGINT NOT NULL,bom_variant_id BIGINT NOT NULL,spec_g BIGINT NOT NULL,warehouse TEXT NOT NULL,
			source_doc_type TEXT NOT NULL,source_doc_id BIGINT NOT NULL,source_batch_code TEXT NOT NULL,source_batch_id TEXT NOT NULL DEFAULT '',
			qty_before_g BIGINT NOT NULL,qty_change_g BIGINT NOT NULL,qty_after_g BIGINT NOT NULL,
			qty_before_units BIGINT NOT NULL,qty_change_units BIGINT NOT NULL,qty_after_units BIGINT NOT NULL,
			operator TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.finished_inventory(
			product_id BIGINT NOT NULL,bom_spec_id BIGINT NOT NULL DEFAULT 0,bom_variant_id BIGINT NOT NULL DEFAULT 0,
			spec_g BIGINT NOT NULL,warehouse TEXT NOT NULL,onhand_units BIGINT NOT NULL,onhand_loose_g BIGINT NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),PRIMARY KEY(product_id,bom_spec_id,spec_g,warehouse)
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.order_stock_batch_allocations(
			id BIGSERIAL PRIMARY KEY,order_id BIGINT NOT NULL,product_id BIGINT NOT NULL,bom_spec_id BIGINT NOT NULL,
			bom_variant_id BIGINT NOT NULL,spec_g BIGINT NOT NULL,batch_id BIGINT NOT NULL DEFAULT 0,batch_code TEXT NOT NULL,
			allocated_g BIGINT NOT NULL,allocated_units BIGINT NOT NULL
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.order_stock_deductions(
			id BIGSERIAL PRIMARY KEY,order_id BIGINT NOT NULL,product_id BIGINT NOT NULL,bom_spec_id BIGINT NOT NULL,
			bom_variant_id BIGINT NOT NULL,spec_g BIGINT NOT NULL,batch_id BIGINT NOT NULL,batch_code TEXT NOT NULL,
			deducted_g BIGINT NOT NULL,deducted_units BIGINT NOT NULL,source_doc_type TEXT NOT NULL,source_doc_id BIGINT NOT NULL,
			operator TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now(),UNIQUE(order_id,product_id,bom_spec_id,batch_code)
		)`, schema),
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("create source-warehouse shipment test table: %v", err)
		}
	}
	_, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(id,name) VALUES(9,'测试商品');
		INSERT INTO %s.orders(id,source_warehouse) VALUES(17,'customer_virtual');
		INSERT INTO %s.order_items(order_id,product_id,bom_spec_id,bom_variant_id,item_name,spec,qty)
		VALUES(17,9,0,0,'测试商品','1000g',1);
		INSERT INTO %s.finished_inventory(product_id,bom_spec_id,bom_variant_id,spec_g,warehouse,onhand_units,onhand_loose_g)
		VALUES(9,0,0,1000,'customer_virtual',1,500);
		INSERT INTO %s.stock_batches(batch_code,item_type,item_id,item_name,bom_spec_id,spec_g,remaining_g,remaining_units)
		VALUES('SOURCE-WH-001','finished_product',9,'测试商品',0,1000,1500,1);
		INSERT INTO %s.stock_ledger_entries(item_type,item_id,item_name,bom_spec_id,bom_variant_id,spec_g,warehouse,source_doc_type,source_doc_id,source_batch_code,
			qty_before_g,qty_change_g,qty_after_g,qty_before_units,qty_change_units,qty_after_units,operator)
		VALUES('finished_product',9,'测试商品',0,0,1000,'customer_virtual','seed',1,'SOURCE-WH-001',0,1500,1500,0,1,1,'qa');
	`, schema, schema, schema, schema, schema, schema))
	if err != nil {
		t.Fatalf("seed source-warehouse shipment stock: %v", err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	repo := Repository{pool: pool, schema: schema}
	if err := repo.deductOrderSourceWarehouseItemsTx(ctx, tx, 17, "customer_virtual", "qa"); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("deduct order source warehouse: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var summaryUnits, summaryLoose, batchUnits, batchG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units,onhand_loose_g FROM %s.finished_inventory WHERE product_id=9 AND bom_spec_id=0 AND spec_g=1000 AND warehouse='customer_virtual'`, schema)).Scan(&summaryUnits, &summaryLoose); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT remaining_units,remaining_g FROM %s.stock_batches WHERE batch_code='SOURCE-WH-001'`, schema)).Scan(&batchUnits, &batchG); err != nil {
		t.Fatal(err)
	}
	var deductions int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.order_stock_deductions WHERE order_id=17 AND batch_code='SOURCE-WH-001' AND deducted_g=1000`, schema)).Scan(&deductions); err != nil {
		t.Fatal(err)
	}
	if summaryUnits != 0 || summaryLoose != 500 || batchUnits != 0 || batchG != 500 || deductions != 1 {
		t.Fatalf("source warehouse summary=%d+%dg, batch=%d/%dg, deductions=%d; want 0+500g, 0/500g, 1", summaryUnits, summaryLoose, batchUnits, batchG, deductions)
	}
	_, err = pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(id,name) VALUES(10,'汇总库存商品');
		INSERT INTO %s.orders(id,source_warehouse) VALUES(18,'customer_virtual'),(19,'finished_goods');
		INSERT INTO %s.order_items(order_id,product_id,bom_spec_id,bom_variant_id,item_name,spec,qty)
		VALUES(18,10,0,0,'汇总库存商品','1000g',1);
		INSERT INTO %s.finished_inventory(product_id,bom_spec_id,bom_variant_id,spec_g,warehouse,onhand_units,onhand_loose_g)
		VALUES(10,0,0,1000,'customer_virtual',1,0);
		INSERT INTO %s.stock_batches(batch_code,item_type,item_id,item_name,bom_spec_id,spec_g,remaining_g,remaining_units)
		VALUES('OTHER-WH-001','finished_product',10,'汇总库存商品',0,1000,1000,1);
		INSERT INTO %s.order_stock_batch_allocations(order_id,product_id,bom_spec_id,bom_variant_id,spec_g,batch_id,batch_code,allocated_g,allocated_units)
		SELECT 19,10,0,0,1000,id,batch_code,1000,0 FROM %s.stock_batches WHERE batch_code='OTHER-WH-001';
	`, schema, schema, schema, schema, schema, schema, schema))
	if err != nil {
		t.Fatalf("seed cross-warehouse reservation: %v", err)
	}
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.deductOrderSourceWarehouseItemsTx(ctx, tx, 18, "customer_virtual", "qa"); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("deduct summary-only source warehouse stock despite another warehouse reservation: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT onhand_units,onhand_loose_g FROM %s.finished_inventory
		WHERE product_id=10 AND bom_spec_id=0 AND spec_g=1000 AND warehouse='customer_virtual'
	`, schema)).Scan(&summaryUnits, &summaryLoose); err != nil {
		t.Fatal(err)
	}
	if summaryUnits != 0 || summaryLoose != 0 {
		t.Fatalf("summary-only source stock after shipment=%d+%dg, want zero", summaryUnits, summaryLoose)
	}
	_, err = pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(id,name) VALUES(11,'BOM汇总库存商品');
		INSERT INTO %s.orders(id,source_warehouse) VALUES(20,'customer_virtual'),(21,'finished_goods');
		INSERT INTO %s.order_items(order_id,product_id,bom_spec_id,bom_variant_id,item_name,spec,qty)
		VALUES(20,11,37,0,'BOM汇总库存商品','454g',1);
		INSERT INTO %s.finished_inventory(product_id,bom_spec_id,bom_variant_id,spec_g,warehouse,onhand_units,onhand_loose_g)
		VALUES(11,37,0,0,'customer_virtual',1,0);
		INSERT INTO %s.order_stock_batch_allocations(order_id,product_id,bom_spec_id,bom_variant_id,spec_g,batch_id,batch_code,allocated_g,allocated_units)
		VALUES(21,11,37,0,0,0,'BOM-SPEC-FP-11-37',0,1);
	`, schema, schema, schema, schema, schema))
	if err != nil {
		t.Fatalf("seed cross-warehouse BOM reservation: %v", err)
	}
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.deductOrderSourceWarehouseItemsTx(ctx, tx, 20, "customer_virtual", "qa"); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("deduct BOM summary-only source warehouse stock despite another warehouse reservation: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT onhand_units FROM %s.finished_inventory
		WHERE product_id=11 AND bom_spec_id=37 AND warehouse='customer_virtual'
	`, schema)).Scan(&summaryUnits); err != nil {
		t.Fatal(err)
	}
	if summaryUnits != 0 {
		t.Fatalf("BOM summary-only source stock after shipment=%d units, want zero", summaryUnits)
	}
}

func TestMultiOrderShipmentIdentityLocksAreOrderIndependent(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	for _, stmt := range []string{
		fmt.Sprintf(`CREATE TABLE %s.product_bom_spec_authorities(product_id BIGINT PRIMARY KEY,state TEXT NOT NULL)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.order_items(
			id BIGSERIAL PRIMARY KEY,order_id BIGINT NOT NULL,product_id BIGINT,bom_spec_id BIGINT,bom_variant_id BIGINT,
			item_name TEXT NOT NULL,spec TEXT NOT NULL,qty NUMERIC NOT NULL
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.order_stock_batch_allocations(
			id BIGSERIAL PRIMARY KEY,order_id BIGINT NOT NULL,product_id BIGINT NOT NULL,bom_spec_id BIGINT NOT NULL,
			bom_variant_id BIGINT NOT NULL,spec_g BIGINT NOT NULL,allocated_g BIGINT NOT NULL,allocated_units BIGINT NOT NULL
		)`, schema),
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("create multi-order identity lock table: %v", err)
		}
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_bom_spec_authorities(product_id,state) VALUES(9,'cutover'),(10,'cutover');
		INSERT INTO %s.order_items(order_id,product_id,bom_spec_id,bom_variant_id,item_name,spec,qty)
		VALUES(101,9,20,200,'商品9','454g',1),(102,10,30,300,'商品10','227g',1);
	`, schema, schema)); err != nil {
		t.Fatalf("seed multi-order identity locks: %v", err)
	}
	repo := Repository{pool: pool, schema: schema}
	first, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = first.Rollback(ctx) }()
	if err := repo.lockOrdersFinishedIdentitiesTx(ctx, first, []int64{101, 102}); err != nil {
		t.Fatalf("lock first shipment identity union: %v", err)
	}

	secondDone := make(chan error, 1)
	go func() {
		tx, err := pool.Begin(ctx)
		if err != nil {
			secondDone <- err
			return
		}
		if err := repo.lockOrdersFinishedIdentitiesTx(ctx, tx, []int64{102, 101}); err != nil {
			_ = tx.Rollback(ctx)
			secondDone <- err
			return
		}
		secondDone <- tx.Commit(ctx)
	}()

	deadline := time.Now().Add(5 * time.Second)
	waitingForIdentity := false
	for time.Now().Before(deadline) {
		if err := pool.QueryRow(ctx, `SELECT EXISTS(
			SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND state='active'
			AND wait_event_type='Lock' AND query ILIKE '%pg_advisory_xact_lock%'
		)`).Scan(&waitingForIdentity); err != nil {
			t.Fatal(err)
		}
		if waitingForIdentity {
			break
		}
		select {
		case err := <-secondDone:
			t.Fatalf("reversed multi-order lock completed while first union was held: %v", err)
		default:
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !waitingForIdentity {
		t.Fatal("reversed multi-order lock did not wait for the shared identity union")
	}
	if err := first.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("lock reversed multi-order identity union: %v", err)
	}
}

func TestSalesShipmentLocksFinishedIdentityBeforeBatchAndSummary(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	for _, stmt := range []string{
		fmt.Sprintf(`CREATE TABLE %s.products(id BIGINT PRIMARY KEY,name TEXT NOT NULL)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.orders(id BIGINT PRIMARY KEY,source_warehouse TEXT NOT NULL)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.order_stock_batch_allocations(
			id BIGSERIAL PRIMARY KEY,order_id BIGINT NOT NULL,product_id BIGINT NOT NULL,bom_spec_id BIGINT NOT NULL,
			bom_variant_id BIGINT NOT NULL,spec_g BIGINT NOT NULL,batch_id BIGINT NOT NULL,batch_code TEXT NOT NULL,
			allocated_g BIGINT NOT NULL,allocated_units BIGINT NOT NULL
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.stock_batches(
			id BIGSERIAL PRIMARY KEY,batch_code TEXT NOT NULL,item_type TEXT NOT NULL,item_id BIGINT NOT NULL,item_name TEXT NOT NULL,
			bom_spec_id BIGINT NOT NULL DEFAULT 0,bom_variant_id BIGINT NOT NULL DEFAULT 0,spec_g BIGINT NOT NULL,
			remaining_g BIGINT NOT NULL,remaining_units BIGINT NOT NULL,quality_status TEXT NOT NULL DEFAULT 'unchecked'
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.stock_ledger_entries(
			id BIGSERIAL PRIMARY KEY,item_type TEXT NOT NULL,item_id BIGINT NOT NULL,item_name TEXT NOT NULL,
			bom_spec_id BIGINT NOT NULL DEFAULT 0,bom_variant_id BIGINT NOT NULL DEFAULT 0,spec_g BIGINT NOT NULL,
			warehouse TEXT NOT NULL,source_doc_type TEXT NOT NULL,source_doc_id BIGINT NOT NULL,source_batch_code TEXT NOT NULL,
			source_batch_id TEXT NOT NULL DEFAULT '',qty_before_g BIGINT NOT NULL,qty_change_g BIGINT NOT NULL,qty_after_g BIGINT NOT NULL,
			qty_before_units BIGINT NOT NULL,qty_change_units BIGINT NOT NULL,qty_after_units BIGINT NOT NULL,operator TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.finished_inventory(
			product_id BIGINT NOT NULL,bom_spec_id BIGINT NOT NULL DEFAULT 0,spec_g BIGINT NOT NULL,warehouse TEXT NOT NULL,
			onhand_units BIGINT NOT NULL,onhand_loose_g BIGINT NOT NULL,updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			PRIMARY KEY(product_id,bom_spec_id,spec_g,warehouse)
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.order_stock_deductions(
			id BIGSERIAL PRIMARY KEY,order_id BIGINT NOT NULL,product_id BIGINT NOT NULL,bom_spec_id BIGINT NOT NULL,
			bom_variant_id BIGINT NOT NULL,spec_g BIGINT NOT NULL,batch_id BIGINT NOT NULL,batch_code TEXT NOT NULL,
			deducted_g BIGINT NOT NULL,deducted_units BIGINT NOT NULL,source_doc_type TEXT NOT NULL,source_doc_id BIGINT NOT NULL,
			operator TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now(),UNIQUE(order_id,product_id,bom_spec_id,batch_code)
		)`, schema),
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("create stock deduction test table: %v", err)
		}
	}
	_, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(id,name) VALUES(9,'测试商品');
		INSERT INTO %s.orders(id,source_warehouse) VALUES(17,'finished_goods');
		INSERT INTO %s.finished_inventory(product_id,bom_spec_id,spec_g,warehouse,onhand_units,onhand_loose_g)
		VALUES(9,0,1000,'finished_goods',1,500);
		INSERT INTO %s.stock_batches(batch_code,item_type,item_id,item_name,bom_spec_id,spec_g,remaining_g,remaining_units)
		VALUES('SHIP-LOCK-001','finished_product',9,'测试商品',0,1000,1500,1);
		INSERT INTO %s.stock_ledger_entries(item_type,item_id,item_name,bom_spec_id,spec_g,warehouse,source_doc_type,source_doc_id,source_batch_code,qty_before_g,qty_change_g,qty_after_g,qty_before_units,qty_change_units,qty_after_units,operator)
		VALUES('finished_product',9,'测试商品',0,1000,'finished_goods','seed',1,'SHIP-LOCK-001',0,1500,1500,0,1,1,'qa');
		INSERT INTO %s.order_stock_batch_allocations(order_id,product_id,bom_spec_id,bom_variant_id,spec_g,batch_id,batch_code,allocated_g,allocated_units)
		SELECT 17,9,0,0,1000,b.id,b.batch_code,250,0 FROM %s.stock_batches b WHERE b.batch_code='SHIP-LOCK-001';
	`, schema, schema, schema, schema, schema, schema, schema))
	if err != nil {
		t.Fatalf("seed shipment stock: %v", err)
	}

	transfer, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = transfer.Rollback(ctx) }()
	if err := postgresinfra.LockFinishedInventoryIdentityTx(ctx, transfer, schema, 9, 0, 1000); err != nil {
		t.Fatal(err)
	}

	deductionDone := make(chan error, 1)
	go func() {
		tx, err := pool.Begin(ctx)
		if err != nil {
			deductionDone <- err
			return
		}
		if err := (Repository{pool: pool, schema: schema}).deductOrderAllocatedStockTx(ctx, tx, 17, "qa"); err != nil {
			_ = tx.Rollback(ctx)
			deductionDone <- err
			return
		}
		deductionDone <- tx.Commit(ctx)
	}()

	select {
	case err := <-deductionDone:
		t.Fatalf("shipment completed while transfer held the finished identity lock: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	var waitingForIdentity bool
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := pool.QueryRow(ctx, `SELECT EXISTS(
			SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND state='active'
			AND wait_event_type='Lock' AND query ILIKE '%pg_advisory_xact_lock%'
		)`).Scan(&waitingForIdentity); err != nil {
			t.Fatal(err)
		}
		if waitingForIdentity {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !waitingForIdentity {
		t.Fatal("shipment did not wait for the shared finished identity lock")
	}
	if err := transfer.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-deductionDone; err != nil {
		t.Fatalf("shipment deduction after lock release: %v", err)
	}
}
