package sales

import (
	salesapp "orderapp/internal/application/sales"
	"os"
	"strings"
	"testing"
)

func TestMiniPreProductionEditGuardIsCheckedAfterOrderRowLock(t *testing.T) {
	source, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	guardAt := strings.Index(text, "cmd.RequirePreProductionEdit")
	if guardAt < 0 {
		t.Fatal("SaveOrder must opt into the mini pre-production edit guard")
	}
	prefix := text[:guardAt]
	lockAt := strings.LastIndex(prefix, "FOR UPDATE")
	if lockAt < 0 {
		t.Fatal("pre-production guard must run after locking the order row")
	}
	guardWindow := text[lockAt:]
	guardSource, err := os.ReadFile("order_editability.go")
	if err != nil {
		t.Fatal(err)
	}
	guardContract := guardWindow + string(guardSource)
	for _, want := range []string{
		"production_plan_items",
		"production_plans",
		"work_orders",
		"produce_batch_order_items",
		"produce_batches",
		"order_shipment_orders",
		"order_stock_deductions",
		"EvaluateOrderEditability",
		"NewOrderEditConflictError",
		"cmd.ExpectedEditRevision",
		"currentOrderEditRevisionTx",
		"订单已被其他操作修改，请重新打开后再编辑",
	} {
		if !strings.Contains(guardContract, want) {
			t.Fatalf("pre-production guard missing %q", want)
		}
	}
}

func TestMiniOrderEditLocksAndChecksRevisionBeforePricing(t *testing.T) {
	source, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	lockAt := strings.Index(text, "FROM %s.orders\n\t\t\tWHERE id=$1\n\t\t\tFOR UPDATE")
	revisionAt := strings.Index(text, "currentOrderEditRevisionTx")
	pricingAt := strings.Index(text, "// Pricing:")
	if lockAt < 0 || revisionAt < 0 || pricingAt < 0 || lockAt > revisionAt || revisionAt > pricingAt {
		t.Fatal("mini edit must lock the order and verify its revision before any pricing or item replacement work")
	}
}

func TestOrderEditUpdatesReceiverFieldsAndInvalidatesOldDocuments(t *testing.T) {
	source, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, want := range []string{
		"receiver_name=",
		"receiver_phone=",
		"receiver_address=",
		"receiver_company=",
		"UPDATE %s.sales_order_documents SET is_latest=false WHERE order_id=$1",
		"UPDATE %s.sales_order_images SET is_latest=false WHERE order_id=$1",
		"UPDATE %s.delivery_note_documents SET is_latest=false WHERE order_id=$1",
		"UPDATE %s.combined_sales_order_documents SET is_latest=false WHERE order_ids @> jsonb_build_array($1::bigint)",
		"UPDATE %s.combined_sales_order_images SET is_latest=false WHERE order_ids @> jsonb_build_array($1::bigint)",
		"UPDATE %s.combined_delivery_note_documents SET is_latest=false WHERE order_ids @> jsonb_build_array($1::bigint)",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("order edit contract missing %q", want)
		}
	}
}

func TestMiniOrderEditRepositoryHonorsPreservedResponsibleAndFulfillmentSnapshots(t *testing.T) {
	source, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, want := range []string{
		"cmd.PreserveResponsibleSnapshot",
		"cmd.ResponsibleName",
		"cmd.PreserveFulfillmentSnapshot",
		"cmd.SourceWarehouse",
		"applyOrderCustomerProfileDefaults(&cmd, customerProfile)",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("preserved order snapshot contract missing %q", want)
		}
	}
}

func TestApplyOrderCustomerProfileDefaultsPreservesOriginalEditSnapshot(t *testing.T) {
	cmd := salesapp.SaveOrderCommand{
		EditID:                      42,
		PreserveFulfillmentSnapshot: true,
		SourceID:                    11,
		OrderTypeID:                 12,
	}
	applyOrderCustomerProfileDefaults(&cmd, requiredOrderCustomerProfile{sourceID: 21, orderTypeID: 22})
	if cmd.SourceID != 11 || cmd.OrderTypeID != 12 {
		t.Fatalf("mini edit source/order type overwritten: %+v", cmd)
	}

	create := salesapp.SaveOrderCommand{SourceID: 1, OrderTypeID: 2}
	applyOrderCustomerProfileDefaults(&create, requiredOrderCustomerProfile{sourceID: 31, orderTypeID: 32})
	if create.SourceID != 31 || create.OrderTypeID != 32 {
		t.Fatalf("new order did not use current customer defaults: %+v", create)
	}
}

func TestOrderNumberSequencingDoesNotTableLockOrdersDuringEdit(t *testing.T) {
	source, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	if strings.Contains(text, "LOCK TABLE %s.orders") {
		t.Fatal("SaveOrder must not table-lock orders; it deadlocks with production order-row locks")
	}
	for _, want := range []string{"nextOrderNoWithAdvisoryLock", "pg_advisory_xact_lock(hashtext($1)::bigint)", "orders:order_no:", "nextOrderNo"} {
		if !strings.Contains(text, want) {
			t.Fatalf("order-number sequencing contract missing %q", want)
		}
	}
}

func TestMiniOrderSaveRevalidatesCurrentDefaultPublicationInsideTransaction(t *testing.T) {
	repositorySource, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	currentSource, err := os.ReadFile("order_current_publication.go")
	if err != nil {
		t.Fatal(err)
	}
	contract := string(repositorySource) + string(currentSource)
	for _, want := range []string{
		"cmd.RequireCurrentDefaultPublications",
		"LOCK TABLE %s.bean_list_publications IN SHARE MODE",
		"isCurrentDefaultOrderPublicationTx",
		"status='published'",
		"publication_purpose='factory_supply'",
		"价格表已更新，请重新选择商品规格后保存",
	} {
		if !strings.Contains(contract, want) {
			t.Fatalf("transactional publication contract missing %q", want)
		}
	}
}

func TestOrderSaveAuditIsAtomicAndKeepsActorAndSummary(t *testing.T) {
	source, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	auditAt := strings.Index(text, "r.logOrderSaveTx(ctx, tx, cmd.Actor")
	commitAt := strings.Index(text, "tx.Commit(ctx)")
	if auditAt < 0 || commitAt < 0 || auditAt > commitAt {
		t.Fatal("order audit must be written with actor inside the same transaction before commit")
	}
	for _, want := range []string{"beforeAuditSummary", "afterAuditSummary", "loadOrderSaveAuditSummaryTx", `'item_count'`, `'order_items'`, `'order_date'`, `'receiver_name'`, `'receiver_phone'`, `'receiver_address'`, `'receiver_company'`, `'notes'`, `'shipping_amount'`, `'order_discount_amount'`, "AuditInsertTx(ctx, tx", "order_audit_logs", `actor = "unknown"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("atomic order audit contract missing %q", want)
		}
	}
}
