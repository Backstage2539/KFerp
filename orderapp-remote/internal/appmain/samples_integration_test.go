package appmain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	bomapp "orderapp/internal/application/bom"
	portal "orderapp/internal/application/customerportal"
	messages "orderapp/internal/application/messagecenter"
	page "orderapp/internal/application/pageentry"
	salesapp "orderapp/internal/application/sales"
	app "orderapp/internal/application/samples"
	bompg "orderapp/internal/infrastructure/postgres/bom"
	portalpg "orderapp/internal/infrastructure/postgres/customerportal"
	messagepg "orderapp/internal/infrastructure/postgres/messagecenter"
	pagepg "orderapp/internal/infrastructure/postgres/pageentry"
	salespg "orderapp/internal/infrastructure/postgres/sales"
	samplepg "orderapp/internal/infrastructure/postgres/samples"
	portalhttp "orderapp/internal/interfaces/http/customerportal"
	pagehttp "orderapp/internal/interfaces/http/pageentry"
	samplehttp "orderapp/internal/interfaces/http/samples"
)

// Every test uses a dedicated random schema in the explicitly configured test
// database. No DATABASE_URL fallback is allowed for this mutating fixture.
type sampleFixture struct {
	t                                  *testing.T
	ctx                                context.Context
	pool                               *pgxpool.Pool
	schema                             string
	repo                               samplepg.Repository
	e                                  *echo.Echo
	portal                             *portal.Service
	admin, sales, other                app.Principal
	adminToken, salesToken, otherToken string
	visitor                            app.Principal
	visitorToken                       string
	productIDs                         []int64
	entry                              page.Entry
	publication                        int64
}

func (f *sampleFixture) validBOM(productID int64) int64 {
	f.t.Helper()
	f.exec("INSERT INTO %s.product_unit_definitions(code,name,unit_type,allow_decimal,active) VALUES('袋','袋','package',false,true) ON CONFLICT(code) DO UPDATE SET active=true,deleted_at=NULL")
	f.exec("UPDATE %s.products SET unit_rule_override_json='{}'::jsonb || jsonb_build_object('inventory_unit','袋') WHERE id=$1", productID)
	material := f.id("INSERT INTO %s.materials(code,name,kind,unit,cost_unit,purchase_price) VALUES($1,'测试熟豆','bean','kg','kg',80) RETURNING id", fmt.Sprintf("SAMPLE-MAT-%d", productID))
	svc := bomapp.NewService(bompg.NewRepository(f.pool, f.schema))
	bom, err := svc.CreateProductionBom(f.ctx, bomapp.CreateProductionBomCommand{Name: fmt.Sprintf("样品BOM%d", productID), OutputType: "product", OutputProductID: productID, OutputQty: 1, Actor: "sample-test", Variants: []bomapp.ProductionBomDraftVariant{{SpecKey: "100g", Name: "100g 样品袋", InventoryUnit: "袋", IsDefault: true, Items: []bomapp.ProductionBomDraftItem{{ComponentType: "material", MaterialID: material, ConsumeUnit: "kg", QtyPerUnit: 0.1}}}}})
	if err != nil {
		f.t.Fatal(err)
	}
	if err = svc.PublishProductionBomVersion(f.ctx, bomapp.PublishProductionBomVersionCommand{VersionID: bom.LatestVersionID, Actor: "sample-test"}); err != nil {
		f.t.Fatal(err)
	}
	if _, err = svc.BindProductProductionBom(f.ctx, bomapp.BindProductProductionBomCommand{ProductID: productID, BomID: bom.ID, BomVersionID: bom.LatestVersionID, Actor: "sample-test"}); err != nil {
		f.t.Fatal(err)
	}
	detail, err := svc.GetProductionBomDetail(f.ctx, bom.ID, bom.LatestVersionID)
	if err != nil {
		f.t.Fatal(err)
	}
	if len(detail.Variants) != 1 {
		f.t.Fatalf("missing published specification: %+v", detail)
	}
	return detail.Variants[0].BomSpecID
}

func TestSamplesPostgresAtomicConfirmation(t *testing.T) {
	f := newSampleFixture(t)
	specs := []int64{f.validBOM(f.productIDs[0]), f.validBOM(f.productIDs[1])}
	source := f.id("INSERT INTO %s.sources(name) VALUES('样品线索') RETURNING id")
	orderType := f.id("INSERT INTO %s.order_types(name) VALUES('样品销售') RETURNING id")
	v := f.assign(f.submit("sample-order-001"), f.sales.EmployeeID)
	cmd := app.ConfirmCommand{Revision: v.Revision, NewCustomer: &app.NewCustomer{Name: "样品新客户", CustomerType: "wholesale", SourceID: source, OrderTypeID: orderType}, Receiver: v.Receiver, Items: []app.ConfirmItem{{ProductID: f.productIDs[0], BomSpecID: specs[0], Qty: 2, UnitPrice: 12.3}, {ProductID: f.productIDs[1], BomSpecID: specs[1], Qty: 1, UnitPrice: 5}}, ShippingAmount: 8.5}
	t.Run("published specifications and server selected customer candidates", func(t *testing.T) {
		f.t = t
		opts, err := f.repo.Options(f.ctx, f.sales, v.ID, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(opts.Specs) != 2 {
			t.Fatalf("want two same-product published specs, got %+v", opts.Specs)
		}
		for _, s := range opts.Specs {
			if s.ProductID == f.productIDs[0] && s.ID != specs[0] || s.ProductID == f.productIDs[1] && s.ID != specs[1] {
				t.Fatal("BOM specification from another product")
			}
		}
		f.call("GET", fmt.Sprintf("/api/mini/employee/sample-requests/%d/options", v.ID), nil, f.otherToken, 403)
		f.exec("UPDATE %[1]s.production_bom_versions SET status='draft' WHERE id=(SELECT bom_version_id FROM %[1]s.production_bom_output_bindings WHERE output_type='product' AND output_id=$1)", f.productIDs[1])
		opts, err = f.repo.Options(f.ctx, f.sales, v.ID, "")
		if err != nil || len(opts.Specs) != 1 {
			t.Fatalf("unpublished specs offered: %+v %v", opts, err)
		}
		f.exec("UPDATE %[1]s.production_bom_versions SET status='published' WHERE id=(SELECT bom_version_id FROM %[1]s.production_bom_output_bindings WHERE output_type='product' AND output_id=$1)", f.productIDs[1])
	})
	t.Run("late specification failure rolls back customer order links audits and messages", func(t *testing.T) {
		f.t = t
		beforeCustomer := f.id("SELECT count(*) FROM %s.customers")
		beforeOrder := f.id("SELECT count(*) FROM %s.orders")
		beforeAudit := f.id("SELECT count(*) FROM %s.audit_logs")
		beforeMessage := f.id("SELECT count(*) FROM %s.message_events")
		bad := cmd
		bad.Items = append([]app.ConfirmItem(nil), cmd.Items...)
		bad.Items[0].BomSpecID = specs[1]
		_, err := f.repo.Confirm(f.ctx, f.sales, v.ID, bad)
		if err == nil {
			t.Fatal("foreign product BOM accepted")
		}
		if n := f.id("SELECT count(*) FROM %s.customers"); n != beforeCustomer {
			t.Fatalf("failed order retained customer: %d -> %d", beforeCustomer, n)
		}
		if n := f.id("SELECT count(*) FROM %s.orders"); n != beforeOrder {
			t.Fatal("failed confirmation retained order")
		}
		if n := f.id("SELECT count(*) FROM %s.audit_logs"); n != beforeAudit {
			t.Fatal("rollback retained customer/order audit")
		}
		if n := f.id("SELECT count(*) FROM %s.message_events"); n != beforeMessage {
			t.Fatal("rollback retained notification")
		}
		current, err := f.repo.Get(f.ctx, f.sales, v.ID)
		if err != nil || current.Status != "pending_contact" || current.OrderID != 0 || current.CustomerID != 0 || current.Revision != v.Revision {
			t.Fatalf("failed confirm mutated work order: %+v %v", current, err)
		}
		f.call("POST", fmt.Sprintf("/api/mini/employee/sample-requests/%d/confirm", v.ID), bad, f.salesToken, 400)
		bad = cmd
		invalidProfile := *cmd.NewCustomer
		invalidProfile.SourceID = 999999999
		bad.NewCustomer = &invalidProfile
		f.call("POST", fmt.Sprintf("/api/mini/employee/sample-requests/%d/confirm", v.ID), bad, f.salesToken, 400)
		if n := f.id("SELECT count(*) FROM %s.customers"); n != beforeCustomer {
			t.Fatal("invalid customer default source persisted")
		}
		// Fail only after both customer and normal sales order insertion have
		// succeeded inside the outer transaction, at its final work-order link.
		f.exec("CREATE FUNCTION %s.fail_sample_order_link() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.order_id IS NOT NULL THEN RAISE EXCEPTION 'sample test late association failure'; END IF; RETURN NEW; END $$")
		f.exec("CREATE TRIGGER fail_sample_order_link BEFORE UPDATE ON %[1]s.sample_work_orders FOR EACH ROW EXECUTE FUNCTION %[1]s.fail_sample_order_link()")
		_, err = f.repo.Confirm(f.ctx, f.sales, v.ID, cmd)
		f.exec("DROP TRIGGER fail_sample_order_link ON %s.sample_work_orders")
		f.exec("DROP FUNCTION %s.fail_sample_order_link()")
		if err == nil || !strings.Contains(err.Error(), "sample test late association failure") {
			t.Fatalf("late transactional failure did not execute: %v", err)
		}
		if n := f.id("SELECT count(*) FROM %s.customers"); n != beforeCustomer {
			t.Fatal("late link failure retained customer")
		}
		if n := f.id("SELECT count(*) FROM %s.orders"); n != beforeOrder {
			t.Fatal("late link failure retained order")
		}
		if n := f.id("SELECT count(*) FROM %s.audit_logs"); n != beforeAudit {
			t.Fatal("late link failure retained audit")
		}
		if n := f.id("SELECT count(*) FROM %s.message_events"); n != beforeMessage {
			t.Fatal("late link failure retained message")
		}
		current, err = f.repo.Get(f.ctx, f.sales, v.ID)
		if err != nil || current.OrderID != 0 || current.CustomerID != 0 || current.Revision != v.Revision {
			t.Fatalf("late failure linked half-completed work order: %+v %v", current, err)
		}
	})
	t.Run("concurrent paid confirmation creates one customer order and keeps portal closed", func(t *testing.T) {
		f.t = t
		var wg sync.WaitGroup
		results := make(chan app.Request, 4)
		errs := make(chan error, 4)
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				r, err := f.repo.Confirm(f.ctx, f.sales, v.ID, cmd)
				results <- r
				errs <- err
			}()
		}
		wg.Wait()
		close(results)
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatal(err)
			}
		}
		for r := range results {
			if r.OrderID == 0 || v.OrderID != 0 && r.OrderID != v.OrderID {
				t.Fatalf("duplicate or missing order: %+v", r)
			}
			v = r
		}
		if n := f.id("SELECT count(*) FROM %s.orders"); n != 1 {
			t.Fatalf("created %d orders", n)
		}
		if n := f.id("SELECT count(*) FROM %s.customers WHERE phone='13890000101'"); n != 1 {
			t.Fatalf("created %d customers", n)
		}
		var total, shipping, grand float64
		var owner int64
		if err := f.pool.QueryRow(f.ctx, fmt.Sprintf("SELECT total_amount,shipping_amount,grand_total,responsible_party_id FROM %s.orders WHERE id=$1", f.schema), v.OrderID).Scan(&total, &shipping, &grand, &owner); err != nil {
			t.Fatal(err)
		}
		if total != 29.6 || shipping != 8.5 || grand != 38.1 || owner != f.sales.EmployeeID {
			t.Fatalf("amount or responsibility mismatch: %v %v %v owner=%d", total, shipping, grand, owner)
		}
		if publication := f.id("SELECT COALESCE(bean_list_publication_id,0) FROM %s.orders WHERE id=$1", v.OrderID); publication != 0 {
			t.Fatalf("sample header acquired unrelated default publication %d", publication)
		}
		if n := f.id("SELECT count(*) FROM %s.order_items WHERE order_id=$1 AND COALESCE(bean_list_publication_id,0)<>0", v.OrderID); n != 0 {
			t.Fatal("sample lines acquired unrelated default publication")
		}
		if n := f.id("SELECT count(*) FROM %s.customer_portal_profiles WHERE customer_id=$1 AND enabled", v.CustomerID); n != 0 {
			t.Fatal("sample creation granted portal access")
		}
		if n := f.id("SELECT count(*) FROM %s.customer_portal_user_bindings WHERE customer_id=$1", v.CustomerID); n != 0 {
			t.Fatal("sample creation granted customer order access")
		}
		if n := f.id("SELECT responsible_employee_id FROM %s.customers WHERE id=$1", v.CustomerID); n != f.sales.EmployeeID {
			t.Fatal("new customer did not use work order owner")
		}
		if v.FollowupStatus != "following" || v.NextReminderAt == nil || time.Until(*v.NextReminderAt) < 7*24*time.Hour-time.Minute {
			t.Fatal("contact confirmation did not reset follow-up")
		}
	})
	if v.OrderID == 0 {
		t.Fatal("order fixture unavailable")
	}
	t.Run("paid sample editing preserves manual prices without catalogue publication", func(t *testing.T) {
		f.t = t
		unpaid := f.id("SELECT id FROM %s.pay_statuses WHERE name='未付款' LIMIT 1")
		editItems := []salesapp.OrderItemCommand{}
		for _, item := range cmd.Items {
			pid, price := item.ProductID, item.UnitPrice
			itemID := f.id("SELECT id FROM %s.order_items WHERE order_id=$1 AND product_id=$2 AND bom_spec_id=$3", v.OrderID, pid, item.BomSpecID)
			variantID := f.id("SELECT bom_variant_id FROM %s.order_items WHERE id=$1", itemID)
			editItems = append(editItems, salesapp.OrderItemCommand{ItemID: itemID, ProductID: &pid, ParentProductID: pid, BomSpecID: item.BomSpecID, BomVariantID: variantID, Units: item.Qty, ManualPrice: &price, PriceSelectionMode: "manual", BeanListPublicationID: 0})
		}
		edit := salesapp.SaveOrderCommand{Actor: "sample-test", EditID: v.OrderID, CustomerID: v.CustomerID, OrderDate: time.Now(), DocumentDate: time.Now(), PayStatusID: unpaid, PreserveResponsibleSnapshot: true, ResponsibleType: "employee", ResponsibleID: f.sales.EmployeeID, ResponsibleName: "样品销售甲", RequireCurrentDefaultPublications: true, ReceiverName: v.Receiver.Name, ReceiverPhone: v.Receiver.Phone, ReceiverAddress: "上海市收费样品更正路 303 号", ShippingAmount: cmd.ShippingAmount, Items: editItems}
		ordinary := edit
		ordinary.EditID = 0
		ordinary.Items = append([]salesapp.OrderItemCommand(nil), edit.Items...)
		for i := range ordinary.Items {
			ordinary.Items[i].ItemID = 0
		}
		if _, err := salesapp.NewService(salespg.NewRepository(f.pool, f.schema)).SaveOrder(f.ctx, ordinary); err == nil {
			t.Fatal("ordinary manual order bypassed current-publication requirements")
		}
		result, err := salesapp.NewService(salespg.NewRepository(f.pool, f.schema)).SaveOrder(f.ctx, edit)
		if err != nil || !result.Edited || result.OrderID != v.OrderID {
			t.Fatalf("paid sample edit requires a catalogue price for its existing lines: %+v %v", result, err)
		}
		var total, shipping, grand float64
		var address string
		if err := f.pool.QueryRow(f.ctx, fmt.Sprintf("SELECT total_amount,shipping_amount,grand_total,receiver_address FROM %s.orders WHERE id=$1", f.schema), v.OrderID).Scan(&total, &shipping, &grand, &address); err != nil {
			t.Fatal(err)
		}
		if total != 29.6 || shipping != 8.5 || grand != 38.1 || address != "上海市收费样品更正路 303 号" {
			t.Fatalf("paid sample edit changed agreed amount: %v %v %v %q", total, shipping, grand, address)
		}
		if n := f.id("SELECT count(*) FROM %s.order_items WHERE order_id=$1 AND COALESCE(bean_list_publication_id,0)<>0", v.OrderID); n != 0 {
			t.Fatal("sample edit invented a catalogue publication")
		}
	})
	t.Run("reassignment and abandonment retain historical order attribution", func(t *testing.T) {
		f.t = t
		orderID, customerID := v.OrderID, v.CustomerID
		next := *v.NextReminderAt
		shipID := f.id("SELECT id FROM %s.ship_statuses WHERE name='已发货' LIMIT 1")
		f.exec("UPDATE %s.orders SET ship_status_id=$2 WHERE id=$1", orderID, shipID)
		shipped, err := f.repo.Get(f.ctx, f.sales, v.ID)
		if err != nil || shipped.ShipmentStatus != "已发货" || shipped.FollowupStatus != "following" || shipped.NextReminderAt == nil || !shipped.NextReminderAt.Equal(next) {
			t.Fatalf("shipment did not sync independently of followup: %+v %v", shipped, err)
		}
		v = f.assign(v, f.other.EmployeeID)
		if n := f.id("SELECT responsible_employee_id FROM %s.customers WHERE id=$1", customerID); n != f.sales.EmployeeID {
			t.Fatal("reassignment changed existing customer owner")
		}
		if n := f.id("SELECT responsible_party_id FROM %s.orders WHERE id=$1", orderID); n != f.sales.EmployeeID {
			t.Fatal("reassignment changed historical order owner")
		}
		v, err = f.repo.Followup(f.ctx, f.other, v.ID, app.FollowupCommand{Revision: v.Revision, Status: "abandoned", Note: "客户试饮后暂不采购"})
		if err != nil {
			t.Fatal(err)
		}
		if v.OrderID != orderID || f.id("SELECT count(*) FROM %s.orders WHERE id=$1", orderID) != 1 {
			t.Fatal("abandonment removed order")
		}
		if n, err := f.repo.Remind(f.ctx, time.Now().Add(14*24*time.Hour)); err != nil || n != 0 {
			t.Fatal("abandoned sample still reminded")
		}
	})
	t.Run("free samples accept zero merchandise and freight with existing customer", func(t *testing.T) {
		f.t = t
		free := f.assign(f.submit("sample-free-001"), f.other.EmployeeID)
		f.exec("INSERT INTO %s.customer_portal_user_bindings(mini_user_id,customer_id,status,approved_by) VALUES($1,$2,'approved','test')", f.visitor.MiniUserID, v.CustomerID)
		candidate := f.id("INSERT INTO %s.customers(name,phone,active,responsible_employee_id,customer_type,default_source_id,default_order_type_id) VALUES('其他归属客户','13890000999',true,$1,'wholesale',$2,$3) RETURNING id", f.other.EmployeeID, source, orderType)
		opts, err := f.repo.Options(f.ctx, f.other, free.ID, "")
		if err != nil {
			t.Fatal(err)
		}
		if opts.SuggestedCustomerID != v.CustomerID {
			t.Fatalf("bound customer not preferred: %+v", opts)
		}
		foundCandidate := false
		for _, c := range opts.Customers {
			foundCandidate = foundCandidate || c.ID == candidate
		}
		if !foundCandidate {
			t.Fatal("employee customer match candidates missing")
		}
		freeCmd := app.ConfirmCommand{Revision: free.Revision, CustomerID: v.CustomerID, Receiver: free.Receiver, Items: []app.ConfirmItem{{ProductID: f.productIDs[0], BomSpecID: specs[0], Qty: 1, UnitPrice: 0}}}
		free = sampleDecode(t, f.call("POST", fmt.Sprintf("/api/mini/employee/sample-requests/%d/confirm", free.ID), freeCmd, f.otherToken, 200))
		if free.OrderID == 0 {
			t.Fatal("zero-cost sample order missing")
		}
		if publication := f.id("SELECT COALESCE(bean_list_publication_id,0) FROM %s.orders WHERE id=$1", free.OrderID); publication != 0 {
			t.Fatalf("free sample header acquired unrelated default publication %d", publication)
		}
		if n := f.id("SELECT count(*) FROM %s.order_items WHERE order_id=$1 AND COALESCE(bean_list_publication_id,0)<>0", free.OrderID); n != 0 {
			t.Fatal("free sample lines acquired unrelated default publication")
		}
		var total float64
		if err := f.pool.QueryRow(f.ctx, fmt.Sprintf("SELECT grand_total FROM %s.orders WHERE id=$1", f.schema), free.OrderID).Scan(&total); err != nil || total != 0 {
			t.Fatalf("free sample total: %v %v", total, err)
		}
		if n := f.id("SELECT responsible_party_id FROM %s.orders WHERE id=$1", free.OrderID); n != f.sales.EmployeeID {
			t.Fatal("existing customer's order responsibility changed")
		}
		zero := 0.0
		pid := f.productIDs[0]
		_, err = salesapp.NewService(salespg.NewRepository(f.pool, f.schema)).SaveOrder(f.ctx, salesapp.SaveOrderCommand{Actor: "sample-test", CustomerID: v.CustomerID, OrderDate: time.Now(), DocumentDate: time.Now(), Items: []salesapp.OrderItemCommand{{ProductID: &pid, ParentProductID: pid, BomSpecID: specs[0], Units: 1, ManualPrice: &zero, PriceSelectionMode: "manual"}}})
		if err == nil {
			t.Fatal("sample exception allowed ordinary manual order at zero price")
		}
		unpaid := f.id("SELECT id FROM %s.pay_statuses WHERE name='未付款' LIMIT 1")
		itemID := f.id("SELECT id FROM %s.order_items WHERE order_id=$1 AND product_id=$2 AND bom_spec_id=$3", free.OrderID, pid, specs[0])
		variantID := f.id("SELECT bom_variant_id FROM %s.order_items WHERE id=$1", itemID)
		edit := salesapp.SaveOrderCommand{Actor: "sample-test", EditID: free.OrderID, CustomerID: v.CustomerID, OrderDate: time.Now(), DocumentDate: time.Now(), PayStatusID: unpaid, PreserveResponsibleSnapshot: true, ResponsibleType: "employee", ResponsibleID: f.sales.EmployeeID, ResponsibleName: "样品销售甲", RequireCurrentDefaultPublications: true, ReceiverName: free.Receiver.Name, ReceiverPhone: free.Receiver.Phone, ReceiverAddress: "上海市更正收件路 202 号", Items: []salesapp.OrderItemCommand{{ItemID: itemID, ProductID: &pid, ParentProductID: pid, BomSpecID: specs[0], BomVariantID: variantID, Units: 1, ManualPrice: &zero, PriceSelectionMode: "manual", BeanListPublicationID: 0}}}
		edited, err := salesapp.NewService(salespg.NewRepository(f.pool, f.schema)).SaveOrder(f.ctx, edit)
		if err != nil || !edited.Edited || edited.OrderID != free.OrderID {
			t.Fatalf("ordinary editing cannot preserve legitimate free sample: %+v %v", edited, err)
		}
		var address string
		if err := f.pool.QueryRow(f.ctx, fmt.Sprintf("SELECT receiver_address FROM %s.orders WHERE id=$1", f.schema), free.OrderID).Scan(&address); err != nil || address != "上海市更正收件路 202 号" {
			t.Fatalf("sample order edit not saved: %q %v", address, err)
		}
		if n := f.id("SELECT count(*) FROM %s.order_items WHERE order_id=$1 AND COALESCE(bean_list_publication_id,0)<>0", free.OrderID); n != 0 {
			t.Fatal("free sample edit invented a catalogue publication")
		}
		currentItemID := f.id("SELECT id FROM %s.order_items WHERE order_id=$1 AND product_id=$2 AND bom_spec_id=$3", free.OrderID, pid, specs[0])
		foreignItemID := f.id("SELECT id FROM %s.order_items WHERE order_id=$1 AND product_id=$2 AND bom_spec_id=$3", v.OrderID, pid, specs[0])
		for _, forgery := range []string{"new-line", "another-order-item", "different-product", "duplicate-item-id"} {
			bad := edit
			// Exercise zero-price identity protection independently of the
			// unrelated ordinary publication guard; legitimate edit above uses it.
			bad.RequireCurrentDefaultPublications = false
			bad.Items = append([]salesapp.OrderItemCommand(nil), edit.Items...)
			bad.Items[0].ItemID = currentItemID
			switch forgery {
			case "new-line":
				bad.Items[0].ItemID = 0
			case "another-order-item":
				bad.Items[0].ItemID = foreignItemID
			case "different-product":
				otherProduct := f.productIDs[1]
				bad.Items[0].ProductID = &otherProduct
				bad.Items[0].ParentProductID = otherProduct
				bad.Items[0].BomSpecID = specs[1]
				bad.Items[0].BomVariantID = 0
			case "duplicate-item-id":
				bad.Items = append(bad.Items, bad.Items[0])
			}
			if _, err := salesapp.NewService(salespg.NewRepository(f.pool, f.schema)).SaveOrder(f.ctx, bad); err == nil {
				t.Errorf("sample historic-price exemption accepted %s", forgery)
			}
		}
		// Exercise the real employee HTTP read/edit path as well as the service:
		// its request mapping and catalogue preparation must preserve the stored
		// sample identity and manual zero price when only the address changes.
		orderPath := fmt.Sprintf("/api/mini/employee/orders/%d", free.OrderID)
		detail := f.call("GET", orderPath, nil, f.salesToken, 200)
		var canEdit bool
		if err := json.Unmarshal(detail["can_edit"], &canEdit); err != nil || !canEdit {
			t.Fatalf("employee cannot edit unfulfilled sample: %s", detail["edit_block_reason"])
		}
		var httpEdit map[string]any
		if err := json.Unmarshal(detail["order"], &httpEdit); err != nil {
			t.Fatal(err)
		}
		if httpEdit["edit_revision"] == "" || httpEdit["is_sample_order"] != true {
			t.Fatalf("employee order omitted sample edit metadata: %s", detail["order"])
		}
		// Detail amounts are formatted strings; the miniapp submits numbers.
		for _, key := range []string{"shipping_amount", "discount_amount", "prepayment_amount"} {
			amount, err := strconv.ParseFloat(httpEdit[key].(string), 64)
			if err != nil {
				t.Fatalf("invalid detail amount %s: %v", key, err)
			}
			httpEdit[key] = amount
		}
		httpItems := httpEdit["items"].([]any)
		if len(httpItems) != 1 {
			t.Fatalf("employee sample detail has %d items", len(httpItems))
		}
		httpItem := httpItems[0].(map[string]any)
		if int64(httpItem["item_id"].(float64)) != currentItemID || int64(httpItem["bom_variant_id"].(float64)) != variantID || httpItem["price_selection_mode"] != "manual" || httpItem["price_override"] != true || httpItem["bean_list_publication_id"] != float64(0) {
			t.Fatalf("employee detail lost sample item identity or price: %+v", httpItem)
		}
		for _, key := range []string{"qty", "unit_price", "unit_bean_g"} {
			amount, err := strconv.ParseFloat(httpItem[key].(string), 64)
			if err != nil {
				t.Fatalf("invalid detail item amount %s: %v", key, err)
			}
			httpItem[key] = amount
		}
		httpItem["name"] = httpItem["product_name"]
		httpEdit["receiver_address"] = "上海市员工接口更正路 404 号"
		f.call("PUT", orderPath, httpEdit, f.salesToken, 200)
		if err := f.pool.QueryRow(f.ctx, fmt.Sprintf("SELECT receiver_address,grand_total FROM %s.orders WHERE id=$1", f.schema), free.OrderID).Scan(&address, &total); err != nil || address != "上海市员工接口更正路 404 号" || total != 0 {
			t.Fatalf("employee HTTP edit did not preserve free sample: address=%q total=%v err=%v", address, total, err)
		}
		if publication := f.id("SELECT COALESCE(bean_list_publication_id,0) FROM %s.orders WHERE id=$1", free.OrderID); publication != 0 {
			t.Fatalf("employee HTTP edit attached unrelated header publication %d", publication)
		}
		if n := f.id("SELECT count(*) FROM %s.order_items WHERE order_id=$1 AND product_id=$2 AND bom_spec_id=$3 AND bom_variant_id=$4 AND unit_price=0 AND qty=1 AND price_overridden=true AND COALESCE(bean_list_publication_id,0)=0", free.OrderID, pid, specs[0], variantID); n != 1 {
			t.Fatal("employee HTTP edit lost persisted free sample item")
		}
		if _, err := f.repo.Followup(f.ctx, f.other, free.ID, app.FollowupCommand{Revision: free.Revision, Status: "won", Note: "已正式采购"}); err != nil {
			t.Fatal(err)
		}
		if n, err := f.repo.Remind(f.ctx, time.Now().Add(14*24*time.Hour)); err != nil || n != 0 {
			t.Fatal("won sample still reminded")
		}
	})
}

func newSampleFixture(t *testing.T) *sampleFixture {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL is required for isolated sample tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	f := &sampleFixture{t: t, ctx: ctx, pool: pool, schema: fmt.Sprintf("test_samples_%d_%d", os.Getpid(), time.Now().UnixNano())}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+f.schema+" CASCADE")
		pool.Close()
	})
	f.exec("CREATE SCHEMA " + f.schema)
	if err = ensureAppSchema(ctx, pool, f.schema); err != nil {
		t.Fatal(err)
	}
	f.repo = samplepg.NewRepository(pool, f.schema)
	f.portal = portal.NewService(portalpg.NewRepository(pool, f.schema), registrationIdentity{})
	f.e = echo.New()
	f.e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Request().Header.Get("X-Test-Admin") == "yes" {
				c.Set("basic_auth_admin", true)
			}
			return next(c)
		}
	})
	portalhttp.RegisterRoutes(f.e, portalhttp.Dependencies{CustomerPortal: f.portal, EmployeeSales: salesapp.NewService(salespg.NewRepository(pool, f.schema))})
	pagehttp.RegisterRoutes(f.e, pool, f.schema, f.portal, wechatTestAuthz{}, pagehttp.Config{})
	samplehttp.RegisterRoutes(f.e, samplehttp.Dependencies{Service: app.NewService(f.repo), Portal: f.portal, Authz: wechatTestAuthz{}, Messages: messages.NewService(messagepg.NewRepository(pool, f.schema))})
	f.admin, f.adminToken = f.employee("样品管理员", "13890000001", "admin")
	f.sales, f.salesToken = f.employee("样品销售甲", "13890000002", "sales")
	f.other, f.otherToken = f.employee("样品销售乙", "13890000003", "sales")
	f.visitor, f.visitorToken = f.register("sample-visitor", "13890000101")
	for _, name := range []string{"申请咖啡甲", "申请咖啡乙"} {
		f.productIDs = append(f.productIDs, f.id("INSERT INTO %s.products(name,active,product_kind) VALUES($1,true,'roasted_bean') RETURNING id", name))
	}
	f.publication = f.publish("official", "", "v-sample-1")
	pr := pagepg.NewRepository(pool, f.schema)
	f.entry, err = pr.Create(ctx, page.Document{Name: "样品价格表", Kind: "price", Visibility: "registered", PublicationID: f.publication, BeanCenter: true}, "test")
	if err != nil {
		t.Fatal(err)
	}
	f.entry, err = pr.Change(ctx, f.entry.Key, f.entry.Revision, "publish", page.Document{}, "test")
	if err != nil {
		t.Fatal(err)
	}
	return f
}
func (f *sampleFixture) exec(q string, args ...any) {
	f.t.Helper()
	if strings.Contains(q, "%s") || strings.Contains(q, "%[1]s") {
		q = fmt.Sprintf(q, f.schema)
	}
	if _, err := f.pool.Exec(f.ctx, q, args...); err != nil {
		f.t.Fatal(err)
	}
}
func (f *sampleFixture) id(q string, args ...any) int64 {
	f.t.Helper()
	var n int64
	if err := f.pool.QueryRow(f.ctx, fmt.Sprintf(q, f.schema), args...).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
}
func (f *sampleFixture) employee(name, phone, role string) (app.Principal, string) {
	id := f.id("INSERT INTO %[1]s.company_employees(name,phone,account_type,department_id,active) VALUES($1,$2,'internal_employee',(SELECT id FROM %[1]s.company_departments WHERE name='销售' LIMIT 1),true) RETURNING id", name, phone)
	f.exec("INSERT INTO %s.employee_roles(employee_id,role_code) VALUES($1,$2)", id, role)
	f.exec("INSERT INTO %s.employee_login_passwords(employee_id,password_hash,updated_at) VALUES($1,'fixture-only',now()-interval '1 minute')", id)
	uid := f.id("INSERT INTO %s.mini_users(openid) VALUES($1) RETURNING id", fmt.Sprintf("erp-internal-employee:%d", id))
	token := fmt.Sprintf("sample-employee-token-%d", id)
	f.exec("INSERT INTO %s.mini_sessions(token,mini_user_id,expire_at) VALUES($1,$2,now()+interval '1 hour')", token, uid)
	return app.Principal{EmployeeID: id, MiniUserID: uid, Admin: role == "admin", Sales: role == "sales", Actor: fmt.Sprintf("employee:%d", id)}, token
}
func (f *sampleFixture) register(openid, phone string) (app.Principal, string) {
	out := f.call("POST", "/api/mini/registration", map[string]any{"code": openid, "phone_code": phone + ":" + openid, "nickname": "访客 " + openid}, "", 200)
	var token string
	json.Unmarshal(out["token"], &token)
	cur, err := f.portal.Me(f.ctx, token)
	if err != nil {
		f.t.Fatal(err)
	}
	return app.Principal{MiniUserID: cur.MiniUserID, Registered: true, Actor: fmt.Sprintf("mini-user:%d", cur.MiniUserID)}, token
}
func (f *sampleFixture) publish(owner, key, version string) int64 {
	items := []map[string]any{}
	for _, id := range f.productIDs {
		items = append(items, map[string]any{"product_id": id, "name": fmt.Sprintf("咖啡 %d", id), "code": "显示序号", "prices": []map[string]any{{"label": "100g", "value": "15"}}})
	}
	content, _ := json.Marshal(map[string]any{"groups": []map[string]any{{"category": "咖啡", "items": items}}})
	return f.id("INSERT INTO %s.bean_list_publications(list_type,publication_table_key,publication_table_name,version_no,owner_type,owner_key,config_json,content_json) VALUES('commercial','sample-table','样品表',$1,$2,$3,'{}',$4) RETURNING id", version, owner, key, content)
}
func (f *sampleFixture) command(key string) app.SubmitCommand {
	return app.SubmitCommand{RequestID: key, EntryKey: f.entry.Key, PublicationID: f.publication, ProductKeys: []string{fmt.Sprintf("%d:0:0", f.publication), fmt.Sprintf("%d:0:1", f.publication)}, Receiver: app.Receiver{Name: "收件人", Phone: "13990000111", Address: "上海市测试路 101 号"}, Note: "请先联系"}
}
func (f *sampleFixture) call(method, path string, body any, token string, want int) map[string]json.RawMessage {
	f.t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, strings.NewReader(string(raw)))
	req.Header.Set("Content-Type", "application/json")
	if token == "basic-admin" {
		req.Header.Set("X-Test-Admin", "yes")
	} else if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	f.e.ServeHTTP(rec, req)
	if rec.Code != want {
		f.t.Fatalf("%s %s: got %d want %d: %s", method, path, rec.Code, want, rec.Body.String())
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		f.t.Fatal(err)
	}
	return out
}
func sampleDecode(t *testing.T, v any) app.Request {
	t.Helper()
	raw, _ := json.Marshal(v)
	var out app.Request
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
func (f *sampleFixture) submit(key string) app.Request {
	f.t.Helper()
	return sampleDecode(f.t, f.call("POST", "/api/mini/sample-requests", f.command(key), f.visitorToken, 201))
}
func (f *sampleFixture) assign(v app.Request, employee int64) app.Request {
	f.t.Helper()
	return sampleDecode(f.t, f.call("POST", fmt.Sprintf("/api/mini/employee/sample-requests/%d/assign", v.ID), app.AssignCommand{Revision: v.Revision, EmployeeID: employee}, f.adminToken, 200))
}

func TestSamplesPostgresAccessSubmissionAndReminders(t *testing.T) {
	f := newSampleFixture(t)
	t.Run("employee official center and existing customer restrictions", func(t *testing.T) {
		f.t = t
		f.call("GET", "/api/pages/"+f.entry.Key, nil, f.salesToken, 200)
		f.call("GET", "/api/pages/"+f.entry.Key, nil, f.adminToken, 200)
		privateID := f.publish("customer", "99999", "private-v1")
		pr := pagepg.NewRepository(f.pool, f.schema)
		private, err := pr.Create(f.ctx, page.Document{Name: "专属", Kind: "price", Visibility: "authenticated", PublicationID: privateID}, "test")
		if err != nil {
			t.Fatal(err)
		}
		private, err = pr.Change(f.ctx, private.Key, private.Revision, "publish", page.Document{}, "test")
		if err != nil {
			t.Fatal(err)
		}
		f.call("GET", "/api/pages/"+private.Key, nil, f.salesToken, 401)
		f.call("GET", "/api/mini/employee/sample-requests", nil, f.visitorToken, 403)
		f.call("GET", "/api/mini/sample-requests", nil, f.salesToken, 403)
		f.call("GET", "/api/sample-requests", nil, "", 401)
	})
	var v app.Request
	t.Run("trusted snapshot receiver and idempotency", func(t *testing.T) {
		f.t = t
		v = f.submit("sample-initial-001")
		if len(v.Items) != 2 || v.PublicationVersion != "v-sample-1" || v.Status != "pending_assignment" || v.Receiver.Phone != "13990000111" {
			t.Fatalf("incorrect snapshot: %+v", v)
		}
		if v.VerifiedPhone != "" || v.MiniUserID != 0 || v.FollowupStatus != "" {
			t.Fatal("customer response exposed internal fields")
		}
		var phone string
		if err := f.pool.QueryRow(f.ctx, fmt.Sprintf("SELECT verified_phone FROM %s.mini_registrations WHERE mini_user_id=$1", f.schema), f.visitor.MiniUserID).Scan(&phone); err != nil || phone != "13890000101" {
			t.Fatalf("receiver mutated verified phone: %q %v", phone, err)
		}
		again := f.submit("sample-initial-001")
		if again.ID != v.ID {
			t.Fatal("retry created another application")
		}
		if again.VerifiedPhone != "" || again.MiniUserID != 0 || again.FollowupStatus != "" {
			t.Error("idempotent retry exposed staff-only registration and follow-up fields")
		}
		cmd := f.command("sample-initial-001")
		cmd.Note = "changed"
		f.call("POST", "/api/mini/sample-requests", cmd, f.visitorToken, 409)
		cmd = f.command("sample-forged-001")
		cmd.ProductKeys = []string{"forged-product"}
		f.call("POST", "/api/mini/sample-requests", cmd, f.visitorToken, 400)
		cmd = f.command("sample-version-001")
		cmd.PublicationID = f.publish("official", "", "v-not-selected")
		f.call("POST", "/api/mini/sample-requests", cmd, f.visitorToken, 400)
		cmd = f.command("sample-address-001")
		cmd.Receiver.Address = ""
		f.call("POST", "/api/mini/sample-requests", cmd, f.visitorToken, 400)
		f.exec("UPDATE %s.products SET active=false WHERE id=$1", f.productIDs[1])
		f.call("POST", "/api/mini/sample-requests", f.command("sample-inactive-001"), f.visitorToken, 400)
		f.exec("UPDATE %s.products SET active=true WHERE id=$1", f.productIDs[1])
		if count := f.id("SELECT count(*) FROM %s.sample_requests"); count != 1 {
			t.Fatalf("invalid requests persisted: %d", count)
		}
	})
	if v.ID == 0 {
		t.Fatal("submission fixture unavailable")
	}
	t.Run("search accepts complete application number and recipient contact", func(t *testing.T) {
		f.t = t
		for _, q := range []string{v.Number, fmt.Sprint(v.ID), "访客 sample-visitor", "13890000101", "收件人", "13990000111"} {
			rows, err := f.repo.List(f.ctx, f.admin, app.Query{Search: q})
			if err != nil || rows.Total != 1 || len(rows.Rows) != 1 || rows.Rows[0].ID != v.ID {
				t.Errorf("search %q did not find the application: %+v %v", q, rows, err)
			}
		}
	})
	t.Run("customer and employee isolation", func(t *testing.T) {
		f.t = t
		_, otherToken := f.register("sample-other-visitor", "13890000102")
		f.call("GET", fmt.Sprintf("/api/mini/sample-requests/%d", v.ID), nil, otherToken, 403)
		out := f.call("GET", "/api/mini/sample-requests", nil, otherToken, 200)
		if string(out["total"]) != "0" {
			t.Fatal("another customer's application listed")
		}
		f.call("GET", fmt.Sprintf("/api/mini/employee/sample-requests/%d", v.ID), nil, f.salesToken, 403)
		f.call("POST", fmt.Sprintf("/api/mini/employee/sample-requests/%d/assign", v.ID), app.AssignCommand{EmployeeID: f.sales.EmployeeID}, f.salesToken, 403)
		f.exec("UPDATE %s.mini_users SET active=false WHERE id=$1", f.visitor.MiniUserID)
		f.call("GET", "/api/mini/sample-requests", nil, f.visitorToken, 401)
		f.exec("UPDATE %s.mini_users SET active=true WHERE id=$1", f.visitor.MiniUserID)
	})
	t.Run("assignment validates active roles and preserves one work order", func(t *testing.T) {
		f.t = t
		f.exec("UPDATE %s.company_employees SET active=false WHERE id=$1", f.other.EmployeeID)
		f.call("POST", fmt.Sprintf("/api/mini/employee/sample-requests/%d/assign", v.ID), app.AssignCommand{EmployeeID: f.other.EmployeeID}, f.adminToken, 400)
		f.exec("UPDATE %s.company_employees SET active=true WHERE id=$1", f.other.EmployeeID)
		f.exec("UPDATE %s.employee_login_passwords SET login_disabled=true WHERE employee_id=$1", f.other.EmployeeID)
		f.call("POST", fmt.Sprintf("/api/mini/employee/sample-requests/%d/assign", v.ID), app.AssignCommand{EmployeeID: f.other.EmployeeID}, f.adminToken, 400)
		f.exec("UPDATE %s.employee_login_passwords SET login_disabled=false WHERE employee_id=$1", f.other.EmployeeID)
		f.exec("DELETE FROM %s.employee_roles WHERE employee_id=$1", f.other.EmployeeID)
		f.call("POST", fmt.Sprintf("/api/mini/employee/sample-requests/%d/assign", v.ID), app.AssignCommand{EmployeeID: f.other.EmployeeID}, f.adminToken, 400)
		f.exec("INSERT INTO %s.employee_roles(employee_id,role_code) VALUES($1,'sales')", f.other.EmployeeID)
		v = f.assign(v, f.sales.EmployeeID)
		if v.WorkOrderID == 0 || v.NextReminderAt == nil || time.Until(*v.NextReminderAt) < 7*24*time.Hour-time.Minute {
			t.Fatalf("assignment did not start 7-day follow-up: %+v", v)
		}
		workID, next := v.WorkOrderID, *v.NextReminderAt
		f.call("GET", fmt.Sprintf("/api/mini/employee/sample-requests/%d", v.ID), nil, f.otherToken, 403)
		v = f.assign(v, f.other.EmployeeID)
		if v.WorkOrderID != workID || !v.NextReminderAt.Equal(next) {
			t.Fatal("reassignment recreated work order or reset follow-up")
		}
		if count := f.id("SELECT count(*) FROM %s.sample_work_orders WHERE request_id=$1", v.ID); count != 1 {
			t.Fatal("duplicate work orders")
		}
		f.call("GET", fmt.Sprintf("/api/mini/employee/sample-requests/%d", v.ID), nil, f.salesToken, 403)
		if n, err := f.repo.Remind(f.ctx, next.Add(-time.Second)); err != nil || n != 0 {
			t.Fatalf("early reminder: %d %v", n, err)
		}
	})
	t.Run("due concurrency periodic catchup and message ownership", func(t *testing.T) {
		f.t = t
		now := v.NextReminderAt.Add(40 * 24 * time.Hour)
		var wg sync.WaitGroup
		counts := make(chan int, 4)
		errs := make(chan error, 4)
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); n, err := f.repo.Remind(f.ctx, now); counts <- n; errs <- err }()
		}
		wg.Wait()
		close(counts)
		close(errs)
		total := 0
		for n := range counts {
			total += n
		}
		for err := range errs {
			if err != nil {
				t.Fatal(err)
			}
		}
		if total != 1 {
			t.Fatalf("concurrent/restart backlog emitted %d instead of current one", total)
		}
		if n, err := samplepg.NewRepository(f.pool, f.schema).Remind(f.ctx, now); err != nil || n != 0 {
			t.Fatalf("restart duplicated current reminder: %d %v", n, err)
		}
		if n, err := f.repo.Remind(f.ctx, now.Add(7*24*time.Hour)); err != nil || n != 1 {
			t.Fatalf("second weekly cycle: %d %v", n, err)
		}
		if count := f.id("SELECT count(*) FROM %[1]s.message_events e JOIN %[1]s.message_deliveries d ON d.event_id=e.id WHERE e.event_type='sample.followup_due' AND d.target_employee_id=$1", f.other.EmployeeID); count != 2 {
			t.Fatalf("reminder did not follow current owner: %d", count)
		}
		out := f.call("GET", "/api/mini/employee/notifications", nil, f.otherToken, 200)
		if !strings.Contains(string(out["notifications"]), "sample.followup_due") {
			t.Fatal("miniapp did not receive shared ERP events")
		}
		var notices []map[string]json.RawMessage
		json.Unmarshal(out["notifications"], &notices)
		if len(notices) == 0 {
			t.Fatal("no employee notices")
		}
		var eventID int64
		json.Unmarshal(notices[0]["id"], &eventID)
		if eventID == 0 {
			json.Unmarshal(notices[0]["event_id"], &eventID)
		}
		before, err := f.repo.Get(f.ctx, f.other, v.ID)
		if err != nil {
			t.Fatal(err)
		}
		f.call("POST", fmt.Sprintf("/api/mini/employee/notifications/%d/read", eventID), nil, f.otherToken, 200)
		f.call("POST", fmt.Sprintf("/api/mini/employee/notifications/%d/read", eventID), nil, f.salesToken, 200)
		if n := f.id("SELECT count(*) FROM %s.message_reads WHERE event_id=$1 AND employee_id=$2", eventID, f.sales.EmployeeID); n != 0 {
			t.Fatal("employee marked another recipient's event as read")
		}
		shared, err := messages.NewService(messagepg.NewRepository(f.pool, f.schema)).ListNotifications(f.ctx, messages.NotificationQuery{EmployeeID: f.other.EmployeeID, Limit: 100})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, n := range shared {
			if n.ID == eventID {
				found = n.Read
			}
		}
		if !found {
			t.Fatal("miniapp read was not visible to ERP message service")
		}
		after, err := f.repo.Get(f.ctx, f.other, v.ID)
		if err != nil || !before.NextReminderAt.Equal(*after.NextReminderAt) {
			t.Fatal("reading notification reset reminder")
		}
		v = after
	})
	t.Run("meaningful followup resets and terminals stop", func(t *testing.T) {
		f.t = t
		_, err := f.repo.Followup(f.ctx, f.other, v.ID, app.FollowupCommand{Revision: v.Revision, Status: "following", Note: " "})
		if err == nil {
			t.Fatal("empty followup accepted")
		}
		v, err = f.repo.Followup(f.ctx, f.other, v.ID, app.FollowupCommand{Revision: v.Revision, Status: "following", Note: "已电话沟通，等待试饮"})
		if err != nil {
			t.Fatal(err)
		}
		if time.Until(*v.NextReminderAt) > 7*24*time.Hour+time.Second || time.Until(*v.NextReminderAt) < 7*24*time.Hour-time.Minute {
			t.Fatal("valid follow-up did not reset from write time")
		}
		_, err = f.repo.Followup(f.ctx, f.sales, v.ID, app.FollowupCommand{Revision: v.Revision, Status: "won", Note: "越权"})
		if !errors.Is(err, app.ErrForbidden) {
			t.Fatalf("other salesperson updated: %v", err)
		}
		v, err = f.repo.Followup(f.ctx, f.other, v.ID, app.FollowupCommand{Revision: v.Revision, Status: "abandoned", Note: "客户明确暂不采购"})
		if err != nil {
			t.Fatal(err)
		}
		if v.NextReminderAt != nil || v.Status != "closed" {
			t.Fatal("terminal work order remains scheduled")
		}
		if n, err := f.repo.Remind(f.ctx, time.Now().Add(100*24*time.Hour)); err != nil || n != 0 {
			t.Fatalf("terminal reminder: %d %v", n, err)
		}
		if n := f.id("SELECT count(*) FROM %s.audit_logs WHERE entity_type='sample_request' AND entity_id=$1", v.ID); n < 5 {
			t.Fatalf("missing audit trail: %d", n)
		}
	})
	t.Run("unread totals and older pages do not truncate at one hundred", func(t *testing.T) {
		f.t = t
		f.exec("WITH events AS (INSERT INTO %[1]s.message_events(event_key,topic,event_type,source_type,source_id,title) SELECT 'sample-pagination-'||n,'samples','sample.followup_due','sample_request',$1,'分页消息' FROM generate_series(1,105)n RETURNING id) INSERT INTO %[1]s.message_deliveries(event_id,channel,target_type,target_employee_id) SELECT id,'erp_platform','employee',$2 FROM events", v.ID, f.other.EmployeeID)
		out := f.call("GET", "/api/mini/employee/notifications", nil, f.otherToken, 200)
		var notices []messages.Notification
		json.Unmarshal(out["notifications"], &notices)
		var unread int
		json.Unmarshal(out["unread_count"], &unread)
		if len(notices) != 100 || unread < 105 {
			t.Fatalf("unread truncated: visible=%d total=%d", len(notices), unread)
		}
		last := notices[len(notices)-1].ID
		older := f.call("GET", fmt.Sprintf("/api/mini/employee/notifications?before_id=%d", last), nil, f.otherToken, 200)
		json.Unmarshal(older["notifications"], &notices)
		if len(notices) == 0 {
			t.Fatal("old notifications inaccessible")
		}
		for _, n := range notices {
			if n.ID >= last {
				t.Fatal("notification page repeated newer events")
			}
		}
	})
}

func TestSamplesPostgresConcurrentSubmissionAndPublicationChange(t *testing.T) {
	f := newSampleFixture(t)
	ctx, cancel := context.WithTimeout(f.ctx, 15*time.Second)
	defer cancel()
	// Notification rules must not turn sample in-app messages into broad or
	// external deliveries. A sample publish supplies its own exact recipients.
	f.exec("INSERT INTO %s.message_notification_rules(code,name,event_type,topic,channel,target_type) VALUES('sample-wide','测试规则','sample.submitted','samples','wechat_official','broadcast')")
	cmd := f.command("sample-concurrent-submit")
	var wg sync.WaitGroup
	results := make(chan app.Request, 8)
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); v, err := f.repo.Submit(ctx, f.visitor, cmd); results <- v; errs <- err }()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var saved app.Request
	for v := range results {
		if v.ID == 0 || saved.ID > 0 && v.ID != saved.ID {
			t.Fatal("concurrent submissions duplicated application")
		}
		if v.VerifiedPhone != "" || v.MiniUserID != 0 || v.FollowupStatus != "" {
			t.Fatal("concurrent retry exposed staff fields")
		}
		saved = v
	}
	if n := f.id("SELECT count(*) FROM %s.sample_requests"); n != 1 {
		t.Fatal("duplicate application rows")
	}
	if n := f.id("SELECT count(*) FROM %s.message_events WHERE event_type='sample.submitted'"); n != 1 {
		t.Fatal("duplicate admin notifications")
	}
	if n := f.id("SELECT count(*) FROM %[1]s.message_deliveries d JOIN %[1]s.message_events e ON e.id=d.event_id WHERE e.topic='samples' AND (d.channel<>'erp_platform' OR d.target_type<>'role' OR d.target_key<>'admin')"); n != 0 {
		t.Fatal("sample notification escaped explicit internal admin audience")
	}
	oldPublication := f.publication
	newPublication := f.publish("official", "", "v-sample-2")
	pr := pagepg.NewRepository(f.pool, f.schema)
	draft := f.entry.Draft
	draft.PublicationID = newPublication
	var err error
	f.entry, err = pr.Change(f.ctx, f.entry.Key, f.entry.Revision, "save", draft, "test")
	if err != nil {
		t.Fatal(err)
	}
	f.entry, err = pr.Change(f.ctx, f.entry.Key, f.entry.Revision, "publish", page.Document{}, "test")
	if err != nil {
		t.Fatal(err)
	}
	f.call("POST", "/api/mini/sample-requests", f.command("sample-stale-version"), f.visitorToken, 400)
	old, err := f.repo.Get(f.ctx, f.visitor, saved.ID)
	if err != nil || old.PublicationID != oldPublication || old.PublicationVersion != "v-sample-1" || len(old.Items) != 2 {
		t.Fatalf("publication update rewrote request snapshot: %+v %v", old, err)
	}
	f.publication = newPublication
	newCmd := f.command("sample-current-version")
	newCmd.ProductKeys = []string{fmt.Sprintf("%d:0:0", oldPublication)}
	f.call("POST", "/api/mini/sample-requests", newCmd, f.visitorToken, 400)
	f.exec("UPDATE %s.bean_list_publications SET status='withdrawn' WHERE id=$1", newPublication)
	f.call("POST", "/api/mini/sample-requests", f.command("sample-withdrawn-version"), f.visitorToken, 400)
	f.exec("UPDATE %s.bean_list_publications SET status='published' WHERE id=$1", newPublication)
	f.exec("UPDATE %s.mini_users SET active=false WHERE id=$1", f.visitor.MiniUserID)
	if _, err := f.repo.Submit(f.ctx, f.visitor, f.command("sample-disabled-profile")); err == nil {
		t.Fatal("disabled registered account accepted by repository")
	}
	f.exec("UPDATE %s.mini_users SET active=true WHERE id=$1", f.visitor.MiniUserID)
	f.entry, err = pr.Change(f.ctx, f.entry.Key, f.entry.Revision, "disable", page.Document{}, "test")
	if err != nil {
		t.Fatal(err)
	}
	f.call("POST", "/api/mini/sample-requests", f.command("sample-disabled-entry"), f.visitorToken, 400)
	if n := f.id("SELECT count(*) FROM %s.sample_requests"); n != 1 {
		t.Fatal("stale/withdrawn/disabled requests persisted")
	}
}

func TestSamplesPostgresAuthenticatedEntryPreservesCustomerPermissions(t *testing.T) {
	f := newSampleFixture(t)
	pr := pagepg.NewRepository(f.pool, f.schema)
	draft := f.entry.Draft
	draft.Visibility = "authenticated"
	var err error
	f.entry, err = pr.Change(f.ctx, f.entry.Key, f.entry.Revision, "save", draft, "test")
	if err != nil {
		t.Fatal(err)
	}
	f.entry, err = pr.Change(f.ctx, f.entry.Key, f.entry.Revision, "publish", page.Document{}, "test")
	if err != nil {
		t.Fatal(err)
	}
	customer := f.id("INSERT INTO %s.customers(name,active) VALUES('已绑定无价格权限',true) RETURNING id")
	f.exec("INSERT INTO %s.customer_portal_profiles(customer_id,enabled) VALUES($1,true)", customer)
	f.exec("INSERT INTO %s.customer_portal_user_bindings(mini_user_id,customer_id,status,approved_by) VALUES($1,$2,'approved','test')", f.visitor.MiniUserID, customer)
	f.call("GET", "/api/pages/"+f.entry.Key, nil, f.visitorToken, 403)
	f.call("POST", "/api/mini/sample-requests", f.command("authenticated-no-capabilities"), f.visitorToken, 400)
	f.exec("INSERT INTO %s.customer_service_capabilities(customer_id,capability_code) VALUES($1,'bean_list')", customer)
	f.call("GET", "/api/pages/"+f.entry.Key, nil, f.visitorToken, 403)
	f.call("POST", "/api/mini/sample-requests", f.command("authenticated-one-capability"), f.visitorToken, 400)
	f.exec("INSERT INTO %s.customer_service_capabilities(customer_id,capability_code) VALUES($1,'product_order')", customer)
	f.call("GET", "/api/pages/"+f.entry.Key, nil, f.visitorToken, 200)
	f.call("POST", "/api/mini/sample-requests", f.command("authenticated-allowed"), f.visitorToken, 201)
	other := f.id("INSERT INTO %s.customers(name,active) VALUES('另一个无权限客户',true) RETURNING id")
	f.exec("INSERT INTO %s.customer_portal_profiles(customer_id,enabled) VALUES($1,true)", other)
	f.exec("INSERT INTO %s.customer_portal_user_bindings(mini_user_id,customer_id,status,approved_by) VALUES($1,$2,'approved','test')", f.visitor.MiniUserID, other)
	f.call("POST", "/api/mini/current-customer", map[string]any{"customer_id": other}, f.visitorToken, 200)
	f.call("GET", "/api/pages/"+f.entry.Key, nil, f.visitorToken, 403)
	f.call("POST", "/api/mini/sample-requests", f.command("authenticated-other-selected"), f.visitorToken, 400)
	f.exec("UPDATE %s.mini_sessions SET current_customer_id=NULL WHERE token=$1", f.visitorToken)
	f.call("GET", "/api/pages/"+f.entry.Key, nil, f.visitorToken, 403)
	f.call("POST", "/api/mini/sample-requests", f.command("authenticated-no-selected"), f.visitorToken, 400)
	if n := f.id("SELECT count(*) FROM %s.sample_requests"); n != 1 {
		t.Fatalf("inaccessible price entries accepted %d applications", n)
	}
}
