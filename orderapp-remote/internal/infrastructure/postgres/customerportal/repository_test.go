package customerportal

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	customerportalapp "orderapp/internal/application/customerportal"
	postgrescompany "orderapp/internal/infrastructure/postgres/company"
	postgrescore "orderapp/internal/infrastructure/postgres/core"
	postgrescosting "orderapp/internal/infrastructure/postgres/costing"
	postgrescustomerfulfillment "orderapp/internal/infrastructure/postgres/customerfulfillment"
	postgresmaterials "orderapp/internal/infrastructure/postgres/materials"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCurrentContextByTokenClearsRevokedCurrentCustomer(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	repo := NewRepository(pool, schema)

	var miniUserID, customerAID, customerBID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.mini_users(openid) VALUES('openid-a') RETURNING id
	`, schema)).Scan(&miniUserID); err != nil {
		t.Fatalf("insert mini user: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('客户A') RETURNING id
	`, schema)).Scan(&customerAID); err != nil {
		t.Fatalf("insert customer A: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('客户B') RETURNING id
	`, schema)).Scan(&customerBID); err != nil {
		t.Fatalf("insert customer B: %v", err)
	}
	if customerBID <= 0 || customerBID == customerAID {
		t.Fatalf("customer B id = %d, customer A id = %d", customerBID, customerAID)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_portal_user_bindings(mini_user_id, customer_id, role, status)
		VALUES($1,$2,'owner','approved')
	`, schema), miniUserID, customerAID); err != nil {
		t.Fatalf("insert binding A: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_service_capabilities(customer_id, capability_code, enabled, config_json)
		VALUES($1,$2,true,'{"source":"stale"}'::jsonb)
	`, schema), customerAID, customerportalapp.CapabilityDirectShip); err != nil {
		t.Fatalf("insert capability A: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.mini_sessions(token, mini_user_id, current_customer_id, expire_at)
		VALUES('token-stale',$1,$2,now() + INTERVAL '1 day')
	`, schema), miniUserID, customerAID); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		DELETE FROM %[1]s.customer_portal_user_bindings WHERE mini_user_id=$1 AND customer_id=$2
	`, schema), miniUserID, customerAID); err != nil {
		t.Fatalf("delete binding A: %v", err)
	}

	got, err := repo.CurrentContextByToken(ctx, " token-stale ")
	if err != nil {
		t.Fatalf("CurrentContextByToken: %v", err)
	}
	if got.CurrentCustomerID != 0 || got.CurrentCustomerName != "" {
		t.Fatalf("current customer = %d/%q, want cleared", got.CurrentCustomerID, got.CurrentCustomerName)
	}
	if len(got.Capabilities) != 0 {
		t.Fatalf("capabilities = %+v, want none for revoked customer %d", got.Capabilities, customerAID)
	}
	var storedCurrent int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(current_customer_id,0) FROM %[1]s.mini_sessions WHERE token='token-stale'
	`, schema)).Scan(&storedCurrent); err != nil {
		t.Fatalf("load stored current: %v", err)
	}
	if storedCurrent != 0 {
		t.Fatalf("stored current customer = %d, want cleared", storedCurrent)
	}
}

func TestCurrentContextByTokenSwitchesStaleCurrentCustomerToFirstApprovedBinding(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	repo := NewRepository(pool, schema)

	var miniUserID, customerAID, customerBID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.mini_users(openid) VALUES('openid-b') RETURNING id
	`, schema)).Scan(&miniUserID); err != nil {
		t.Fatalf("insert mini user: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('客户A') RETURNING id
	`, schema)).Scan(&customerAID); err != nil {
		t.Fatalf("insert customer A: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('客户B') RETURNING id
	`, schema)).Scan(&customerBID); err != nil {
		t.Fatalf("insert customer B: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_portal_user_bindings(mini_user_id, customer_id, role, status)
		VALUES($1,$2,'owner','approved'),($1,$3,'member','approved')
	`, schema), miniUserID, customerAID, customerBID); err != nil {
		t.Fatalf("insert bindings: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_portal_profiles(customer_id, display_name)
		VALUES($1,'客户B展示名')
	`, schema), customerBID); err != nil {
		t.Fatalf("insert customer B profile: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_service_capabilities(customer_id, capability_code, enabled, config_json)
		VALUES($1,$2,true,'{"source":"stale"}'::jsonb),($3,$4,true,'{"source":"fresh"}'::jsonb)
	`, schema), customerAID, customerportalapp.CapabilityDirectShip, customerBID, customerportalapp.CapabilitySettlement); err != nil {
		t.Fatalf("insert capabilities: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.mini_sessions(token, mini_user_id, current_customer_id, expire_at)
		VALUES('token-switch',$1,$2,now() + INTERVAL '1 day')
	`, schema), miniUserID, customerAID); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		UPDATE %[1]s.customer_portal_user_bindings SET status='revoked' WHERE mini_user_id=$1 AND customer_id=$2
	`, schema), miniUserID, customerAID); err != nil {
		t.Fatalf("revoke binding A: %v", err)
	}

	got, err := repo.CurrentContextByToken(ctx, "token-switch")
	if err != nil {
		t.Fatalf("CurrentContextByToken: %v", err)
	}
	if got.CurrentCustomerID != customerBID || got.CurrentCustomerName != "客户B展示名" {
		t.Fatalf("current customer = %d/%q, want %d/客户B展示名", got.CurrentCustomerID, got.CurrentCustomerName, customerBID)
	}
	if len(got.Capabilities) != 1 || got.Capabilities[0].Code != customerportalapp.CapabilitySettlement {
		t.Fatalf("capabilities = %+v, want only customer B capability", got.Capabilities)
	}
}

func TestCurrentContextByTokenSwitchesInactiveCurrentCustomerToFirstActiveBinding(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	repo := NewRepository(pool, schema)

	var miniUserID, inactiveCustomerID, activeCustomerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.mini_users(openid) VALUES('openid-inactive-current') RETURNING id
	`, schema)).Scan(&miniUserID); err != nil {
		t.Fatalf("insert mini user: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name, active) VALUES('已停用客户', false) RETURNING id
	`, schema)).Scan(&inactiveCustomerID); err != nil {
		t.Fatalf("insert inactive customer: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name, active) VALUES('可用客户', true) RETURNING id
	`, schema)).Scan(&activeCustomerID); err != nil {
		t.Fatalf("insert active customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_portal_user_bindings(mini_user_id, customer_id, role, status)
		VALUES($1,$2,'owner','approved'),($1,$3,'member','approved')
	`, schema), miniUserID, inactiveCustomerID, activeCustomerID); err != nil {
		t.Fatalf("insert bindings: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_service_capabilities(customer_id, capability_code, enabled, config_json)
		VALUES($1,$2,true,'{"source":"inactive"}'::jsonb),($3,$4,true,'{"source":"active"}'::jsonb)
	`, schema), inactiveCustomerID, customerportalapp.CapabilityDirectShip, activeCustomerID, customerportalapp.CapabilitySettlement); err != nil {
		t.Fatalf("insert capabilities: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.mini_sessions(token, mini_user_id, current_customer_id, expire_at)
		VALUES('token-inactive-current',$1,$2,now() + INTERVAL '1 day')
	`, schema), miniUserID, inactiveCustomerID); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	got, err := repo.CurrentContextByToken(ctx, "token-inactive-current")
	if err != nil {
		t.Fatalf("CurrentContextByToken: %v", err)
	}
	if got.CurrentCustomerID != activeCustomerID || got.CurrentCustomerName != "可用客户" {
		t.Fatalf("current customer = %d/%q, want active customer %d/可用客户", got.CurrentCustomerID, got.CurrentCustomerName, activeCustomerID)
	}
	if len(got.Bindings) != 1 || got.Bindings[0].CustomerID != activeCustomerID {
		t.Fatalf("bindings = %+v, want only active customer binding", got.Bindings)
	}
	if len(got.Capabilities) != 1 || got.Capabilities[0].Code != customerportalapp.CapabilitySettlement {
		t.Fatalf("capabilities = %+v, want only active customer capability", got.Capabilities)
	}
	var storedCurrent int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(current_customer_id,0) FROM %[1]s.mini_sessions WHERE token='token-inactive-current'
	`, schema)).Scan(&storedCurrent); err != nil {
		t.Fatalf("load stored current: %v", err)
	}
	if storedCurrent != activeCustomerID {
		t.Fatalf("stored current customer = %d, want active customer %d", storedCurrent, activeCustomerID)
	}
}

func TestSwitchCurrentCustomerRejectsInactiveApprovedCustomer(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	repo := NewRepository(pool, schema)

	var miniUserID, activeCustomerID, inactiveCustomerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.mini_users(openid) VALUES('openid-switch-inactive') RETURNING id
	`, schema)).Scan(&miniUserID); err != nil {
		t.Fatalf("insert mini user: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name, active) VALUES('当前可用客户', true) RETURNING id
	`, schema)).Scan(&activeCustomerID); err != nil {
		t.Fatalf("insert active customer: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name, active) VALUES('已停用客户', false) RETURNING id
	`, schema)).Scan(&inactiveCustomerID); err != nil {
		t.Fatalf("insert inactive customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_portal_user_bindings(mini_user_id, customer_id, role, status)
		VALUES($1,$2,'owner','approved'),($1,$3,'member','approved')
	`, schema), miniUserID, activeCustomerID, inactiveCustomerID); err != nil {
		t.Fatalf("insert bindings: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.mini_sessions(token, mini_user_id, current_customer_id, expire_at)
		VALUES('token-switch-inactive',$1,$2,now() + INTERVAL '1 day')
	`, schema), miniUserID, activeCustomerID); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	_, err := repo.SwitchCurrentCustomer(ctx, "token-switch-inactive", inactiveCustomerID)
	if !errors.Is(err, customerportalapp.ErrCustomerBindingNotFound) {
		t.Fatalf("SwitchCurrentCustomer err=%v, want ErrCustomerBindingNotFound for inactive customer", err)
	}
	var storedCurrent int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(current_customer_id,0) FROM %[1]s.mini_sessions WHERE token='token-switch-inactive'
	`, schema)).Scan(&storedCurrent); err != nil {
		t.Fatalf("load stored current: %v", err)
	}
	if storedCurrent != activeCustomerID {
		t.Fatalf("stored current customer = %d, want unchanged active customer %d", storedCurrent, activeCustomerID)
	}
}

func TestCurrentContextByTokenReturnsCurrentCustomerTheme(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	repo := NewRepository(pool, schema)

	var miniUserID, customerAID, customerBID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.mini_users(openid) VALUES('openid-theme') RETURNING id
	`, schema)).Scan(&miniUserID); err != nil {
		t.Fatalf("insert mini user: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('主题客户A') RETURNING id
	`, schema)).Scan(&customerAID); err != nil {
		t.Fatalf("insert customer A: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('主题客户B') RETURNING id
	`, schema)).Scan(&customerBID); err != nil {
		t.Fatalf("insert customer B: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_portal_profiles(customer_id, display_name, theme_key)
		VALUES($1,'主题A展示名','clean_ops'),($2,'主题B展示名','premium_partner')
	`, schema), customerAID, customerBID); err != nil {
		t.Fatalf("insert profiles: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_portal_user_bindings(mini_user_id, customer_id, role, status)
		VALUES($1,$2,'owner','approved'),($1,$3,'member','approved')
	`, schema), miniUserID, customerAID, customerBID); err != nil {
		t.Fatalf("insert bindings: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.mini_sessions(token, mini_user_id, current_customer_id, expire_at)
		VALUES('token-theme',$1,$2,now() + INTERVAL '1 day')
	`, schema), miniUserID, customerBID); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	got, err := repo.CurrentContextByToken(ctx, "token-theme")
	if err != nil {
		t.Fatalf("CurrentContextByToken: %v", err)
	}
	if got.CurrentCustomerID != customerBID || got.ThemeKey != customerportalapp.PortalThemePremiumPartner {
		t.Fatalf("current=%d theme=%q, want customerB premium_partner", got.CurrentCustomerID, got.ThemeKey)
	}
}

func TestCurrentContextByTokenReturnsCurrentCustomerMiniappEntryMode(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	repo := NewRepository(pool, schema)

	var miniUserID, customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.mini_users(openid) VALUES('openid-entry-mode') RETURNING id
	`, schema)).Scan(&miniUserID); err != nil {
		t.Fatalf("insert mini user: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('商城入口客户') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_portal_profiles(customer_id, display_name, miniapp_entry_mode)
		VALUES($1,'商城入口客户','mall')
	`, schema), customerID); err != nil {
		t.Fatalf("insert profile: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_portal_user_bindings(mini_user_id, customer_id, role, status)
		VALUES($1,$2,'owner','approved')
	`, schema), miniUserID, customerID); err != nil {
		t.Fatalf("insert binding: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.mini_sessions(token, mini_user_id, current_customer_id, expire_at)
		VALUES('token-entry-mode',$1,$2,now() + INTERVAL '1 day')
	`, schema), miniUserID, customerID); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	got, err := repo.CurrentContextByToken(ctx, "token-entry-mode")
	if err != nil {
		t.Fatalf("CurrentContextByToken: %v", err)
	}
	if got.CurrentCustomerID != customerID || got.MiniappEntryMode != customerportalapp.MiniappEntryModeMall {
		t.Fatalf("current=%d miniapp_entry_mode=%q, want customer %d mall", got.CurrentCustomerID, got.MiniappEntryMode, customerID)
	}
}

func TestMiniappCurrentCustomerSwitchScopesOrderServicePage(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	repo := NewRepository(pool, schema)
	svc := customerportalapp.NewService(repo, nil)

	var miniUserID, customerAID, customerBID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.mini_users(openid) VALUES('openid-current-order-scope') RETURNING id
	`, schema)).Scan(&miniUserID); err != nil {
		t.Fatalf("insert mini user: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name, contact, phone, address)
		VALUES('当前客户A','客户A联系人','13800000001','A地址') RETURNING id
	`, schema)).Scan(&customerAID); err != nil {
		t.Fatalf("insert customer A: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name, contact, phone, address)
		VALUES('当前客户B','客户B联系人','13800000002','B地址') RETURNING id
	`, schema)).Scan(&customerBID); err != nil {
		t.Fatalf("insert customer B: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_portal_user_bindings(mini_user_id, customer_id, role, status)
		VALUES($1,$2,'owner','approved'),($1,$3,'member','approved')
	`, schema), miniUserID, customerAID, customerBID); err != nil {
		t.Fatalf("insert bindings: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_service_capabilities(customer_id, capability_code, enabled, config_json)
		VALUES
			($1,$3,true,'{}'::jsonb),
			($2,$3,true,'{}'::jsonb)
	`, schema), customerAID, customerBID, customerportalapp.CapabilityProductOrder); err != nil {
		t.Fatalf("insert capabilities: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.mini_sessions(token, mini_user_id, current_customer_id, expire_at)
		VALUES('token-current-order-scope',$1,$2,now() + INTERVAL '1 day')
	`, schema), miniUserID, customerAID); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	var orderAID, orderBID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.orders(order_no, order_date, customer_id, grand_total, is_void)
		VALUES('SO-CURRENT-A','2026-05-13',$1,101,false)
		RETURNING id
	`, schema), customerAID).Scan(&orderAID); err != nil {
		t.Fatalf("insert order A: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.orders(order_no, order_date, customer_id, grand_total, is_void)
		VALUES('SO-CURRENT-B','2026-05-13',$1,202,false)
		RETURNING id
	`, schema), customerBID).Scan(&orderBID); err != nil {
		t.Fatalf("insert order B: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.order_items(order_id, line_no, item_name, qty, unit, spec, line_total)
		VALUES
			($1,1,'客户A订单商品',1,'袋','454g',101),
			($2,1,'客户B订单商品',2,'袋','454g',202)
	`, schema), orderAID, orderBID); err != nil {
		t.Fatalf("insert order items: %v", err)
	}

	before, err := svc.GetServicePage(ctx, "token-current-order-scope", customerportalapp.ServiceKeyOrders, customerportalapp.ServicePageFilter{})
	if err != nil {
		t.Fatalf("GetServicePage before switch: %v", err)
	}
	if len(before.Orders) != 1 || before.Orders[0].OrderNo != "SO-CURRENT-A" {
		t.Fatalf("before switch orders=%+v, want only SO-CURRENT-A", before.Orders)
	}
	if strings.Contains(fmt.Sprintf("%+v", before), "SO-CURRENT-B") {
		t.Fatalf("customer B order leaked before switch: %+v", before)
	}

	switched, err := svc.SwitchCurrentCustomer(ctx, "token-current-order-scope", customerBID)
	if err != nil {
		t.Fatalf("SwitchCurrentCustomer: %v", err)
	}
	if switched.CurrentCustomerID != customerBID {
		t.Fatalf("switched current customer=%d, want %d", switched.CurrentCustomerID, customerBID)
	}

	after, err := svc.GetServicePage(ctx, "token-current-order-scope", customerportalapp.ServiceKeyOrders, customerportalapp.ServicePageFilter{})
	if err != nil {
		t.Fatalf("GetServicePage after switch: %v", err)
	}
	if len(after.Orders) != 1 || after.Orders[0].OrderNo != "SO-CURRENT-B" {
		t.Fatalf("after switch orders=%+v, want only SO-CURRENT-B", after.Orders)
	}
	if strings.Contains(fmt.Sprintf("%+v", after), "SO-CURRENT-A") {
		t.Fatalf("customer A order leaked after switch: %+v", after)
	}
}

func TestCustomerOwnsOrderChecksActiveCustomerOrder(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	repo := NewRepository(pool, schema)

	var customerAID, customerBID, orderAID, orderBID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name, active) VALUES('文档客户A',true) RETURNING id
	`, schema)).Scan(&customerAID); err != nil {
		t.Fatalf("insert customer A: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name, active) VALUES('文档客户B',true) RETURNING id
	`, schema)).Scan(&customerBID); err != nil {
		t.Fatalf("insert customer B: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.orders(order_no, order_date, customer_id, is_void)
		VALUES('SO-DOC-A','2026-05-15',$1,false)
		RETURNING id
	`, schema), customerAID).Scan(&orderAID); err != nil {
		t.Fatalf("insert order A: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.orders(order_no, order_date, customer_id, is_void)
		VALUES('SO-DOC-B','2026-05-15',$1,true)
		RETURNING id
	`, schema), customerBID).Scan(&orderBID); err != nil {
		t.Fatalf("insert order B: %v", err)
	}

	ok, err := repo.CustomerOwnsOrder(ctx, customerAID, orderAID)
	if err != nil || !ok {
		t.Fatalf("CustomerOwnsOrder(customer A, order A) ok=%v err=%v", ok, err)
	}
	ok, err = repo.CustomerOwnsOrder(ctx, customerAID, orderBID)
	if err != nil || ok {
		t.Fatalf("CustomerOwnsOrder(customer A, void customer B order) ok=%v err=%v", ok, err)
	}
	ok, err = repo.CustomerOwnsOrder(ctx, customerBID, orderAID)
	if err != nil || ok {
		t.Fatalf("CustomerOwnsOrder(customer B, order A) ok=%v err=%v", ok, err)
	}
}

func TestMiniappCurrentCustomerSwitchRejectsUnapprovedCustomerWithoutChangingSession(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	repo := NewRepository(pool, schema)
	svc := customerportalapp.NewService(repo, nil)

	var miniUserID, customerAID, customerBID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.mini_users(openid) VALUES('openid-current-order-unapproved') RETURNING id
	`, schema)).Scan(&miniUserID); err != nil {
		t.Fatalf("insert mini user: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('已批准客户') RETURNING id
	`, schema)).Scan(&customerAID); err != nil {
		t.Fatalf("insert customer A: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('未批准客户') RETURNING id
	`, schema)).Scan(&customerBID); err != nil {
		t.Fatalf("insert customer B: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_portal_user_bindings(mini_user_id, customer_id, role, status)
		VALUES($1,$2,'owner','approved'),($1,$3,'member','pending')
	`, schema), miniUserID, customerAID, customerBID); err != nil {
		t.Fatalf("insert bindings: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.mini_sessions(token, mini_user_id, current_customer_id, expire_at)
		VALUES('token-current-order-unapproved',$1,$2,now() + INTERVAL '1 day')
	`, schema), miniUserID, customerAID); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	_, err := svc.SwitchCurrentCustomer(ctx, "token-current-order-unapproved", customerBID)
	if !errors.Is(err, customerportalapp.ErrCustomerBindingNotFound) {
		t.Fatalf("SwitchCurrentCustomer err=%v, want ErrCustomerBindingNotFound", err)
	}
	current, err := repo.CurrentContextByToken(ctx, "token-current-order-unapproved")
	if err != nil {
		t.Fatalf("CurrentContextByToken after rejected switch: %v", err)
	}
	if current.CurrentCustomerID != customerAID {
		t.Fatalf("unapproved switch changed current customer to %d, want %d", current.CurrentCustomerID, customerAID)
	}
}

func TestCreateLoginSessionReturnsDefaultThemeForUnconfiguredCustomer(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	repo := NewRepository(pool, schema)

	var miniUserID, customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.mini_users(openid) VALUES('openid-default-theme') RETURNING id
	`, schema)).Scan(&miniUserID); err != nil {
		t.Fatalf("insert mini user: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('默认主题客户') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_portal_user_bindings(mini_user_id, customer_id, role, status)
		VALUES($1,$2,'owner','approved')
	`, schema), miniUserID, customerID); err != nil {
		t.Fatalf("insert binding: %v", err)
	}

	got, err := repo.CreateLoginSession(ctx, customerportalapp.CreateLoginSessionCommand{OpenID: "openid-default-theme"})
	if err != nil {
		t.Fatalf("CreateLoginSession: %v", err)
	}
	if got.CurrentCustomerID != customerID || got.ThemeKey != customerportalapp.PortalThemeCoffeeFactory {
		t.Fatalf("current=%d theme=%q, want default coffee_factory", got.CurrentCustomerID, got.ThemeKey)
	}
}

func TestCreateLoginSessionReturnsCurrentCustomerMiniappEntryMode(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	repo := NewRepository(pool, schema)

	var miniUserID, customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.mini_users(openid) VALUES('openid-login-entry') RETURNING id
	`, schema)).Scan(&miniUserID); err != nil {
		t.Fatalf("insert mini user: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('登录商城入口客户') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_portal_profiles(customer_id, display_name, miniapp_entry_mode)
		VALUES($1,'登录商城入口客户','mall')
	`, schema), customerID); err != nil {
		t.Fatalf("insert profile: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_portal_user_bindings(mini_user_id, customer_id, role, status)
		VALUES($1,$2,'owner','approved')
	`, schema), miniUserID, customerID); err != nil {
		t.Fatalf("insert binding: %v", err)
	}

	got, err := repo.CreateLoginSession(ctx, customerportalapp.CreateLoginSessionCommand{OpenID: "openid-login-entry"})
	if err != nil {
		t.Fatalf("CreateLoginSession: %v", err)
	}
	if got.CurrentCustomerID != customerID || got.MiniappEntryMode != customerportalapp.MiniappEntryModeMall {
		t.Fatalf("current=%d miniapp_entry_mode=%q, want customer %d mall", got.CurrentCustomerID, got.MiniappEntryMode, customerID)
	}
}

func TestCreatePasswordLoginSessionAuthenticatesChannelCustomerBinding(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	ensureCustomerPortalERPBindingTestSchema(t, ctx, pool, schema)
	repo := NewRepository(pool, schema)

	var customerID, employeeID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customers(name, customer_type) VALUES('ERP密码登录客户','wholesale') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_portal_profiles(customer_id, display_name, theme_key, miniapp_entry_mode)
		VALUES($1,'ERP密码登录客户','premium_partner','mall')
	`, schema), customerID); err != nil {
		t.Fatalf("insert profile: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_service_capabilities(customer_id, capability_code, enabled, config_json)
		VALUES($1,$2,true,'{}'::jsonb)
	`, schema), customerID, customerportalapp.CapabilityMall); err != nil {
		t.Fatalf("insert capability: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.company_employees(name, phone, account_type, department_id, active)
		VALUES('渠道登录账号','13800138075','channel_customer',(SELECT id FROM %[1]s.company_departments WHERE name='销售' LIMIT 1),true)
		RETURNING id
	`, schema)).Scan(&employeeID); err != nil {
		t.Fatalf("insert employee: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.employee_login_passwords(employee_id, password_hash, login_disabled)
		VALUES($1,$2,false)
	`, schema), employeeID, customerPortalTestPasswordHash("secret")); err != nil {
		t.Fatalf("insert password: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_erp_user_bindings(customer_id, employee_id, role, status, updated_by)
		VALUES($1,$2,'owner','active','test')
	`, schema), customerID, employeeID); err != nil {
		t.Fatalf("insert erp binding: %v", err)
	}

	got, err := repo.CreatePasswordLoginSession(ctx, customerportalapp.CreatePasswordLoginSessionCommand{
		Login:    " 13800138075 ",
		Password: " secret ",
	})
	if err != nil {
		t.Fatalf("CreatePasswordLoginSession: %v", err)
	}
	if got.Token == "" || got.MiniUserID <= 0 || got.CurrentCustomerID != customerID {
		t.Fatalf("password login result=%+v, want token and customer %d", got, customerID)
	}
	if got.ThemeKey != customerportalapp.PortalThemePremiumPartner || got.MiniappEntryMode != customerportalapp.MiniappEntryModeMall {
		t.Fatalf("password login theme=%q entry=%q, want premium_partner/mall", got.ThemeKey, got.MiniappEntryMode)
	}
	if len(got.Bindings) != 1 || got.Bindings[0].CustomerID != customerID {
		t.Fatalf("password login bindings=%+v, want customer %d", got.Bindings, customerID)
	}

	current, err := repo.CurrentContextByToken(ctx, got.Token)
	if err != nil {
		t.Fatalf("CurrentContextByToken after password login: %v", err)
	}
	if current.CurrentCustomerID != customerID || current.CurrentCustomerName != "ERP密码登录客户" {
		t.Fatalf("current=%d/%q, want customer %d ERP密码登录客户", current.CurrentCustomerID, current.CurrentCustomerName, customerID)
	}
	var openID string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT openid FROM %[1]s.mini_users WHERE id=$1`, schema), got.MiniUserID).Scan(&openID); err != nil {
		t.Fatalf("load mini user openid: %v", err)
	}
	if openID != fmt.Sprintf("erp-employee:%d", employeeID) {
		t.Fatalf("mini user openid=%q, want erp employee namespace", openID)
	}
}

func TestCreatePasswordLoginSessionRejectsInternalEmployee(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	ensureCustomerPortalERPBindingTestSchema(t, ctx, pool, schema)
	repo := NewRepository(pool, schema)

	var customerID, employeeID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customers(name, customer_type) VALUES('内部员工绑定客户','wholesale') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.company_employees(name, phone, account_type, department_id, active)
		VALUES('内部员工','13800138076','internal_employee',(SELECT id FROM %[1]s.company_departments WHERE name='销售' LIMIT 1),true)
		RETURNING id
	`, schema)).Scan(&employeeID); err != nil {
		t.Fatalf("insert employee: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.employee_login_passwords(employee_id, password_hash, login_disabled)
		VALUES($1,$2,false)
	`, schema), employeeID, customerPortalTestPasswordHash("secret")); err != nil {
		t.Fatalf("insert password: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_erp_user_bindings(customer_id, employee_id, role, status, updated_by)
		VALUES($1,$2,'owner','active','test')
	`, schema), customerID, employeeID); err != nil {
		t.Fatalf("insert erp binding: %v", err)
	}

	_, err := repo.CreatePasswordLoginSession(ctx, customerportalapp.CreatePasswordLoginSessionCommand{Login: "13800138076", Password: "secret"})
	if !errors.Is(err, customerportalapp.ErrCustomerBindingNotFound) {
		t.Fatalf("CreatePasswordLoginSession internal err=%v, want ErrCustomerBindingNotFound", err)
	}
}

func TestCreatePasswordLoginSessionRejectsChannelCustomerWithoutBinding(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	ensureCustomerPortalERPBindingTestSchema(t, ctx, pool, schema)
	repo := NewRepository(pool, schema)

	var employeeID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.company_employees(name, phone, account_type, department_id, active)
		VALUES('未绑定渠道账号','13800138077','channel_customer',(SELECT id FROM %[1]s.company_departments WHERE name='销售' LIMIT 1),true)
		RETURNING id
	`, schema)).Scan(&employeeID); err != nil {
		t.Fatalf("insert employee: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.employee_login_passwords(employee_id, password_hash, login_disabled)
		VALUES($1,$2,false)
	`, schema), employeeID, customerPortalTestPasswordHash("secret")); err != nil {
		t.Fatalf("insert password: %v", err)
	}

	_, err := repo.CreatePasswordLoginSession(ctx, customerportalapp.CreatePasswordLoginSessionCommand{Login: "13800138077", Password: "secret"})
	if !errors.Is(err, customerportalapp.ErrCustomerBindingNotFound) {
		t.Fatalf("CreatePasswordLoginSession no binding err=%v, want ErrCustomerBindingNotFound", err)
	}
}

func TestCreatePasswordLoginSessionRejectsDisabledLoginAndWrongPassword(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	ensureCustomerPortalERPBindingTestSchema(t, ctx, pool, schema)
	repo := NewRepository(pool, schema)

	var customerID, disabledEmployeeID, wrongPasswordEmployeeID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customers(name, customer_type) VALUES('禁用密码登录客户','wholesale') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.company_employees(name, phone, account_type, department_id, active)
		VALUES('禁用渠道账号','13800138078','channel_customer',(SELECT id FROM %[1]s.company_departments WHERE name='销售' LIMIT 1),true)
		RETURNING id
	`, schema)).Scan(&disabledEmployeeID); err != nil {
		t.Fatalf("insert disabled employee: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.company_employees(name, phone, account_type, department_id, active)
		VALUES('错误密码渠道账号','13800138079','channel_customer',(SELECT id FROM %[1]s.company_departments WHERE name='销售' LIMIT 1),true)
		RETURNING id
	`, schema)).Scan(&wrongPasswordEmployeeID); err != nil {
		t.Fatalf("insert wrong password employee: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.employee_login_passwords(employee_id, password_hash, login_disabled)
		VALUES($1,$2,true),($3,$4,false)
	`, schema), disabledEmployeeID, customerPortalTestPasswordHash("secret"), wrongPasswordEmployeeID, customerPortalTestPasswordHash("secret")); err != nil {
		t.Fatalf("insert passwords: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_erp_user_bindings(customer_id, employee_id, role, status, updated_by)
		VALUES($1,$2,'owner','active','test')
	`, schema), customerID, disabledEmployeeID); err != nil {
		t.Fatalf("insert disabled binding: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_erp_user_bindings(customer_id, employee_id, role, status, updated_by)
		VALUES($1,$2,'owner','inactive','test')
	`, schema), customerID, wrongPasswordEmployeeID); err != nil {
		t.Fatalf("insert inactive binding: %v", err)
	}

	_, err := repo.CreatePasswordLoginSession(ctx, customerportalapp.CreatePasswordLoginSessionCommand{Login: "13800138078", Password: "secret"})
	if !errors.Is(err, customerportalapp.ErrMiniAccountLoginDisabled) {
		t.Fatalf("CreatePasswordLoginSession disabled err=%v, want ErrMiniAccountLoginDisabled", err)
	}
	_, err = repo.CreatePasswordLoginSession(ctx, customerportalapp.CreatePasswordLoginSessionCommand{Login: "13800138079", Password: "wrong"})
	if !errors.Is(err, customerportalapp.ErrMiniInvalidLogin) {
		t.Fatalf("CreatePasswordLoginSession wrong password err=%v, want ErrMiniInvalidLogin", err)
	}
}

func TestCreateFulfillmentOrderUsesSmallBatchWeightTierForNon454Spec(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID, productID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('公共SKU小批量客户') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(name, default_price, active)
		VALUES('小批量非454g商品', 999, true)
		RETURNING id
	`, schema)).Scan(&productID); err != nil {
		t.Fatalf("insert product: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_service_capabilities(customer_id, capability_code, enabled, config_json)
		VALUES($1,'direct_ship',true,'{"small_batch_price_rule":{"enabled":true,"threshold_lb":14,"tier_min_lb":15,"tier_max_lb":28}}'::jsonb)
	`, schema), customerID); err != nil {
		t.Fatalf("insert capability: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.product_price_tiers(product_id, spec_g, min_qty_lb, max_qty_lb, price_per_lb, active)
		VALUES
			($1,454,1,14,120,true),
			($1,454,15,28,90,true),
			($1,454,29,NULL,80,true)
	`, schema), productID); err != nil {
		t.Fatalf("insert tiers: %v", err)
	}

	_, err := repo.CreateFulfillmentOrder(ctx, customerportalapp.CreateFulfillmentOrderCommand{
		CustomerID:          customerID,
		PortalServiceCode:   customerportalapp.PortalServiceProductOrder,
		RecipientName:       "张三",
		RecipientPhone:      "13800138000",
		RecipientAddress:    "上海市测试路",
		ProductID:           productID,
		SpecG:               1000,
		Qty:                 1,
		ShippingAmount:      0,
		CreatedByMiniUserID: 1,
	})
	if err != nil {
		t.Fatalf("CreateFulfillmentOrder: %v", err)
	}

	var unitPrice, lineTotal float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(unit_price,0)::float8, COALESCE(line_total,0)::float8
		FROM %s.order_items
		WHERE product_id=$1
		ORDER BY id DESC
		LIMIT 1
	`, schema), productID).Scan(&unitPrice, &lineTotal); err != nil {
		t.Fatalf("query order item: %v", err)
	}
	if math.Abs(unitPrice-198) > 0.0001 || math.Abs(lineTotal-198) > 0.0001 {
		t.Fatalf("unit_price/line_total=%.2f/%.2f, want 198.00/198.00 from 15-28lb tier", unitPrice, lineTotal)
	}
}

func TestCreateFulfillmentOrderIgnoresClientSuppliedUnitPrice(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID, productID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('客户小程序后端定价客户') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(name, default_price, active)
		VALUES('后端定价商品', 88, true)
		RETURNING id
	`, schema)).Scan(&productID); err != nil {
		t.Fatalf("insert product: %v", err)
	}

	_, err := repo.CreateFulfillmentOrder(ctx, customerportalapp.CreateFulfillmentOrderCommand{
		CustomerID:          customerID,
		PortalServiceCode:   customerportalapp.PortalServiceProductOrder,
		RecipientName:       "张三",
		RecipientPhone:      "13800138000",
		RecipientAddress:    "上海市测试路",
		ProductID:           productID,
		SpecG:               454,
		Qty:                 2,
		UnitPrice:           0.01,
		ShippingAmount:      0,
		CreatedByMiniUserID: 1,
	})
	if err != nil {
		t.Fatalf("CreateFulfillmentOrder: %v", err)
	}

	var unitPrice, lineTotal, grandTotal float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(oi.unit_price,0)::float8,
		       COALESCE(oi.line_total,0)::float8,
		       COALESCE(o.grand_total,0)::float8
		FROM %s.order_items oi
		JOIN %s.orders o ON o.id=oi.order_id
		WHERE oi.product_id=$1
		ORDER BY oi.id DESC
		LIMIT 1
	`, schema, schema), productID).Scan(&unitPrice, &lineTotal, &grandTotal); err != nil {
		t.Fatalf("query order item: %v", err)
	}
	if math.Abs(unitPrice-88) > 0.0001 || math.Abs(lineTotal-176) > 0.0001 || math.Abs(grandTotal-176) > 0.0001 {
		t.Fatalf("unit_price/line_total/grand_total=%.2f/%.2f/%.2f, want server price 88.00/176.00/176.00", unitPrice, lineTotal, grandTotal)
	}
}

func TestLoadProductOrderServicePageFiltersCustomerOnlyProducts(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	repo := NewRepository(pool, schema)

	var customerAID, customerBID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('现货客户A') RETURNING id
	`, schema)).Scan(&customerAID); err != nil {
		t.Fatalf("insert customer A: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('现货客户B') RETURNING id
	`, schema)).Scan(&customerBID); err != nil {
		t.Fatalf("insert customer B: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(name, default_price, active, customer_id, visibility, custom_type)
		VALUES
			('公共商品中烘', 48, true, 0, 'public', ''),
			('客户A专属深烘', 58, true, $1, 'customer_only', 'custom_roast'),
			('客户B不应显示专属深烘', 59, true, $2, 'customer_only', 'custom_roast')
	`, schema), customerAID, customerBID); err != nil {
		t.Fatalf("insert products: %v", err)
	}

	page, err := repo.LoadServicePage(ctx, customerportalapp.ServicePageQuery{
		CustomerID: customerAID,
		Key:        customerportalapp.ServiceKeyProductOrder,
		Limit:      20,
	})
	if err != nil {
		t.Fatalf("LoadServicePage: %v", err)
	}
	names := make([]string, 0, len(page.Products))
	for _, product := range page.Products {
		names = append(names, product.Name)
	}
	got := strings.Join(names, ",")
	for _, want := range []string{"公共商品中烘", "客户A专属深烘"} {
		if !strings.Contains(got, want) {
			t.Fatalf("productOrder products=%q missing %q", got, want)
		}
	}
	if strings.Contains(got, "客户B不应显示专属深烘") {
		t.Fatalf("productOrder products leaked another customer product: %q", got)
	}
}

func TestLoadProductServicePageReplacesBaseProductWithCustomerAlias(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('岩师傅') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}

	var baseProductID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(name, default_price, active, customer_id, base_product_id, visibility, custom_type)
		VALUES('基础款曲奇', 48, true, 0, 0, 'public', '')
		RETURNING id
	`, schema)).Scan(&baseProductID); err != nil {
		t.Fatalf("insert base product: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(name, default_price, active, customer_id, base_product_id, visibility, custom_type)
		VALUES
			('公共保留款', 52, true, 0, 0, 'public', ''),
			('岩师傅兰卡', 58, true, $1, $2, 'customer_only', 'public_sku_alias')
	`, schema), customerID, baseProductID); err != nil {
		t.Fatalf("insert products: %v", err)
	}

	page, err := repo.LoadServicePage(ctx, customerportalapp.ServicePageQuery{
		CustomerID: customerID,
		Key:        customerportalapp.ServiceKeyProductOrder,
		Limit:      20,
	})
	if err != nil {
		t.Fatalf("LoadServicePage: %v", err)
	}
	names := make([]string, 0, len(page.Products))
	for _, product := range page.Products {
		names = append(names, product.Name)
	}
	got := strings.Join(names, ",")
	if strings.Contains(got, "基础款曲奇") {
		t.Fatalf("alias base product should be replaced for customer: %q", got)
	}
	if !strings.Contains(got, "岩师傅兰卡") || !strings.Contains(got, "公共保留款") {
		t.Fatalf("products=%q missing alias or unrelated public product", got)
	}
}

func TestLoadDirectShipAndProcessingServicePagesReturnSelectableProducts(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	repo := NewRepository(pool, schema)

	var customerAID, customerBID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('选择器客户A') RETURNING id
	`, schema)).Scan(&customerAID); err != nil {
		t.Fatalf("insert customer A: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('选择器客户B') RETURNING id
	`, schema)).Scan(&customerBID); err != nil {
		t.Fatalf("insert customer B: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(name, default_price, active, customer_id, visibility, custom_type)
		VALUES
			('选择器公共商品', 48, true, 0, 'public', ''),
			('选择器客户A专属商品', 58, true, $1, 'customer_only', 'custom_roast'),
			('选择器客户B不应显示商品', 59, true, $2, 'customer_only', 'custom_roast')
	`, schema), customerAID, customerBID); err != nil {
		t.Fatalf("insert products: %v", err)
	}

	for _, key := range []string{customerportalapp.ServiceKeyDirectShip, customerportalapp.ServiceKeyProcessing} {
		t.Run(key, func(t *testing.T) {
			page, err := repo.LoadServicePage(ctx, customerportalapp.ServicePageQuery{
				CustomerID: customerAID,
				Key:        key,
				Limit:      20,
			})
			if err != nil {
				t.Fatalf("LoadServicePage(%s): %v", key, err)
			}
			names := make([]string, 0, len(page.Products))
			for _, product := range page.Products {
				names = append(names, product.Name)
			}
			got := strings.Join(names, ",")
			for _, want := range []string{"选择器公共商品", "选择器客户A专属商品"} {
				if !strings.Contains(got, want) {
					t.Fatalf("%s products=%q missing %q", key, got, want)
				}
			}
			if strings.Contains(got, "选择器客户B不应显示商品") {
				t.Fatalf("%s products leaked another customer product: %q", key, got)
			}
		})
	}
}

func TestCreateFulfillmentOrderRejectsAnotherCustomerOnlyProduct(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	repo := NewRepository(pool, schema)

	var customerAID, customerBID, customerBProductID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('现货下单客户A') RETURNING id
	`, schema)).Scan(&customerAID); err != nil {
		t.Fatalf("insert customer A: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('现货下单客户B') RETURNING id
	`, schema)).Scan(&customerBID); err != nil {
		t.Fatalf("insert customer B: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(name, default_price, active, customer_id, visibility, custom_type)
		VALUES('客户B不应显示专属深烘', 59, true, $1, 'customer_only', 'custom_roast')
		RETURNING id
	`, schema), customerBID).Scan(&customerBProductID); err != nil {
		t.Fatalf("insert customer B product: %v", err)
	}

	_, err := repo.CreateFulfillmentOrder(ctx, customerportalapp.CreateFulfillmentOrderCommand{
		CustomerID:          customerAID,
		PortalServiceCode:   customerportalapp.PortalServiceProductOrder,
		RecipientName:       "张三",
		RecipientPhone:      "13800138000",
		RecipientAddress:    "上海市测试路",
		ProductID:           customerBProductID,
		SpecG:               454,
		Qty:                 1,
		CreatedByMiniUserID: 1,
	})
	if err == nil || !strings.Contains(err.Error(), "product unavailable") {
		t.Fatalf("CreateFulfillmentOrder error=%v, want product unavailable", err)
	}

	var rows int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %s.order_items WHERE product_id=$1
	`, schema), customerBProductID).Scan(&rows); err != nil {
		t.Fatalf("count order items: %v", err)
	}
	if rows != 0 {
		t.Fatalf("another customer product created order rows=%d", rows)
	}
}

func TestLoadMallPageFiltersCustomerOnlyMallProducts(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID, customerOnlyProductID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('零售商城客户') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	var publicProductID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(name, default_price, active, customer_id, visibility)
		VALUES('商城公共商品', 68, true, 0, 'public')
		RETURNING id
	`, schema)).Scan(&publicProductID); err != nil {
		t.Fatalf("insert public product: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(name, default_price, active, customer_id, visibility, custom_type)
		VALUES('商城客户专属不应展示', 88, true, $1, 'customer_only', 'custom_roast')
		RETURNING id
	`, schema), customerID).Scan(&customerOnlyProductID); err != nil {
		t.Fatalf("insert customer-only product: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.mall_products(product_id, title, spec_g, unit_price, status, sort_order)
		VALUES
			($1,'商城公共商品',250,68,'published',1),
			($2,'商城客户专属不应展示',250,88,'published',2)
	`, schema), publicProductID, customerOnlyProductID); err != nil {
		t.Fatalf("insert mall products: %v", err)
	}

	page, err := repo.LoadMallPage(ctx, customerID)
	if err != nil {
		t.Fatalf("LoadMallPage: %v", err)
	}
	names := make([]string, 0, len(page.Products))
	for _, product := range page.Products {
		names = append(names, product.Title)
	}
	got := strings.Join(names, ",")
	if !strings.Contains(got, "商城公共商品") {
		t.Fatalf("mall products=%q missing public product", got)
	}
	if strings.Contains(got, "商城客户专属不应展示") {
		t.Fatalf("mall page leaked customer-only product: %q", got)
	}
}

func TestCreateMallOrderRejectsCustomerOnlyMallProduct(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID, customerOnlyProductID, mallProductID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('零售商城下单客户') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(name, default_price, active, customer_id, visibility, custom_type)
		VALUES('商城客户专属不应展示', 88, true, $1, 'customer_only', 'custom_roast')
		RETURNING id
	`, schema), customerID).Scan(&customerOnlyProductID); err != nil {
		t.Fatalf("insert customer-only product: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.mall_products(product_id, title, spec_g, unit_price, status, sort_order)
		VALUES($1,'商城客户专属不应展示',250,88,'published',1)
		RETURNING id
	`, schema), customerOnlyProductID).Scan(&mallProductID); err != nil {
		t.Fatalf("insert mall product: %v", err)
	}

	_, err := repo.CreateMallOrder(ctx, customerportalapp.CreateMallOrderCommand{
		CustomerID:          customerID,
		CreatedByMiniUserID: 1,
		RecipientName:       "张三",
		RecipientPhone:      "13800138000",
		RecipientAddress:    "上海市测试路",
		Items:               []customerportalapp.MallOrderItemCommand{{MallProductID: mallProductID, Qty: 1}},
	})
	if err == nil || !strings.Contains(err.Error(), "mall product unavailable") {
		t.Fatalf("CreateMallOrder error=%v, want mall product unavailable", err)
	}

	var rows int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %s.order_items WHERE product_id=$1
	`, schema), customerOnlyProductID).Scan(&rows); err != nil {
		t.Fatalf("count order items: %v", err)
	}
	if rows != 0 {
		t.Fatalf("customer-only mall product created order rows=%d", rows)
	}
}

func TestListMallProductsExcludesCustomerOnlyOptions(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('商城选品客户') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(name, default_price, active, customer_id, visibility, custom_type)
		VALUES
			('商城公共商品',68,true,0,'public',''),
			('商城客户专属不应展示',88,true,$1,'customer_only','custom_roast')
	`, schema), customerID); err != nil {
		t.Fatalf("insert products: %v", err)
	}

	_, options, err := repo.ListMallProducts(ctx)
	if err != nil {
		t.Fatalf("ListMallProducts: %v", err)
	}
	names := make([]string, 0, len(options))
	for _, option := range options {
		names = append(names, option.Name)
	}
	got := strings.Join(names, ",")
	if !strings.Contains(got, "商城公共商品") {
		t.Fatalf("mall product options=%q missing public product", got)
	}
	if strings.Contains(got, "商城客户专属不应展示") {
		t.Fatalf("mall product options leaked customer-only product: %q", got)
	}
}

func TestUpsertPortalERPBindingRejectsDisabledLoginAccount(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	ensureCustomerPortalERPBindingTestSchema(t, ctx, pool, schema)
	repo := NewRepository(pool, schema)

	var customerID, employeeID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customers(name, customer_type) VALUES('门户停用账号客户','wholesale') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.company_employees(name, phone, account_type, department_id, active)
		VALUES('门户停用渠道账号','13900000008','channel_customer',(SELECT id FROM %[1]s.company_departments WHERE name='销售' LIMIT 1),true)
		RETURNING id
	`, schema)).Scan(&employeeID); err != nil {
		t.Fatalf("insert employee: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.employee_login_passwords(employee_id, password_hash, login_disabled)
		VALUES($1,'disabled-hash',true)
	`, schema), employeeID); err != nil {
		t.Fatalf("insert disabled login: %v", err)
	}

	_, err := repo.UpsertPortalERPBinding(ctx, customerportalapp.UpsertPortalERPBindingCommand{
		CustomerID: customerID,
		EmployeeID: employeeID,
		Status:     "active",
		UpdatedBy:  "Codex",
	})
	if err == nil || !strings.Contains(err.Error(), "login-enabled channel customer account required") {
		t.Fatalf("UpsertPortalERPBinding err=%v, want login-enabled channel account rejection", err)
	}

	var activeBindings int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %[1]s.customer_erp_user_bindings WHERE customer_id=$1 AND status='active'
	`, schema), customerID).Scan(&activeBindings); err != nil {
		t.Fatalf("count active bindings: %v", err)
	}
	if activeBindings != 0 {
		t.Fatalf("active bindings=%d, want 0 for disabled login account", activeBindings)
	}
}

func TestPortalAdminDetailHidesDisabledLoginERPBinding(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	ensureCustomerPortalERPBindingTestSchema(t, ctx, pool, schema)
	repo := NewRepository(pool, schema)

	var customerID, employeeID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customers(name, customer_type) VALUES('门户历史停用绑定客户','wholesale') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.company_employees(name, phone, account_type, department_id, active)
		VALUES('门户历史停用账号','13900000009','channel_customer',(SELECT id FROM %[1]s.company_departments WHERE name='销售' LIMIT 1),true)
		RETURNING id
	`, schema)).Scan(&employeeID); err != nil {
		t.Fatalf("insert employee: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.employee_login_passwords(employee_id, password_hash, login_disabled)
		VALUES($1,'disabled-hash',true)
	`, schema), employeeID); err != nil {
		t.Fatalf("insert disabled login: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_erp_user_bindings(customer_id, employee_id, role, status, updated_by)
		VALUES($1,$2,'customer','active','legacy')
	`, schema), customerID, employeeID); err != nil {
		t.Fatalf("insert legacy binding: %v", err)
	}

	detail, err := repo.PortalAdminDetail(ctx, customerID)
	if err != nil {
		t.Fatalf("PortalAdminDetail: %v", err)
	}
	if detail.Customer.ERPBinding != nil {
		t.Fatalf("PortalAdminDetail ERPBinding=%+v, want nil for disabled login account", detail.Customer.ERPBinding)
	}

	rows, err := repo.ListPortalAdminCustomers(ctx, customerportalapp.PortalAdminCustomerQuery{Limit: 20})
	if err != nil {
		t.Fatalf("ListPortalAdminCustomers: %v", err)
	}
	for _, row := range rows {
		if row.ID == customerID && row.ERPBinding != nil {
			t.Fatalf("ListPortalAdminCustomers ERPBinding=%+v, want nil for disabled login account", row.ERPBinding)
		}
	}
}

func TestSaveMallProductRejectsCustomerOnlyProduct(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID, customerOnlyProductID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('商城保存客户') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(name, default_price, active, customer_id, visibility, custom_type)
		VALUES('商城客户专属不应展示', 88, true, $1, 'customer_only', 'custom_roast')
		RETURNING id
	`, schema), customerID).Scan(&customerOnlyProductID); err != nil {
		t.Fatalf("insert customer-only product: %v", err)
	}

	_, err := repo.SaveMallProduct(ctx, customerportalapp.SaveMallProductCommand{
		ProductID:   customerOnlyProductID,
		Title:       "商城客户专属不应展示",
		SpecG:       250,
		UnitPrice:   88,
		Status:      "published",
		SortOrder:   1,
		TemplateKey: "standard",
		Actor:       "tester",
	})
	if err == nil || !strings.Contains(err.Error(), "mall product unavailable") {
		t.Fatalf("SaveMallProduct error=%v, want mall product unavailable", err)
	}
}

func TestCreateProcessingRequestRejectsAnotherCustomerInventory(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	repo := NewRepository(pool, schema)

	customerAID, customerBID := seedProcessingBoundaryCustomers(t, ctx, pool, schema)
	targetProductID := seedProcessingBoundaryProduct(t, ctx, pool, schema, 0, "public", "公共代加工成品")
	seedProcessingBoundaryInventory(t, ctx, pool, schema, customerBID, 7001, "客户B托管生豆不应使用", 50000)

	_, err := repo.CreateProcessingRequest(ctx, customerportalapp.CreateProcessingRequestCommand{
		CustomerID:          customerAID,
		CreatedByMiniUserID: 1,
		InputMaterialID:     7001,
		InputQtyG:           1000,
		TargetProductID:     targetProductID,
		TargetSpecG:         454,
		TargetQty:           2,
	})
	if err == nil || !strings.Contains(err.Error(), "input material unavailable") {
		t.Fatalf("CreateProcessingRequest error=%v, want input material unavailable", err)
	}
	assertNoProcessingRequestRows(t, ctx, pool, schema, customerAID, "another customer processing request created")
}

func TestCreateProcessingRequestRejectsInsufficientCustomerInventory(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	repo := NewRepository(pool, schema)

	customerAID, _ := seedProcessingBoundaryCustomers(t, ctx, pool, schema)
	targetProductID := seedProcessingBoundaryProduct(t, ctx, pool, schema, 0, "public", "公共代加工成品")
	seedProcessingBoundaryInventory(t, ctx, pool, schema, customerAID, 7002, "客户A托管生豆", 500)

	_, err := repo.CreateProcessingRequest(ctx, customerportalapp.CreateProcessingRequestCommand{
		CustomerID:          customerAID,
		CreatedByMiniUserID: 1,
		InputMaterialID:     7002,
		InputQtyG:           1000,
		TargetProductID:     targetProductID,
		TargetSpecG:         454,
		TargetQty:           2,
	})
	if err == nil || !strings.Contains(err.Error(), "input material unavailable") {
		t.Fatalf("CreateProcessingRequest error=%v, want input material unavailable", err)
	}
	assertNoProcessingRequestRows(t, ctx, pool, schema, customerAID, "insufficient inventory processing request created")
}

func TestCreateDirectShipBatchRejectsEmptyRows(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('空代发批次客户') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}

	_, err := repo.CreateDirectShipBatch(ctx, customerportalapp.CreateDirectShipBatchCommand{
		CustomerID:          customerID,
		CreatedByMiniUserID: 9,
		SourceName:          "空代发批次",
		TotalRows:           0,
	})
	if err == nil || err.Error() != "total_rows invalid" {
		t.Fatalf("CreateDirectShipBatch empty rows err=%v, want total_rows invalid", err)
	}

	var count int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %s.direct_ship_import_batches WHERE customer_id=$1
	`, schema), customerID).Scan(&count); err != nil {
		t.Fatalf("count direct ship batches: %v", err)
	}
	if count != 0 {
		t.Fatalf("empty direct ship batch inserted %d rows", count)
	}
}

func TestCreateProcessingRequestRejectsAnotherCustomerTargetProduct(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	repo := NewRepository(pool, schema)

	customerAID, customerBID := seedProcessingBoundaryCustomers(t, ctx, pool, schema)
	targetProductID := seedProcessingBoundaryProduct(t, ctx, pool, schema, customerBID, "customer_only", "客户B专属代加工成品")
	seedProcessingBoundaryInventory(t, ctx, pool, schema, customerAID, 7003, "客户A托管生豆", 50000)

	_, err := repo.CreateProcessingRequest(ctx, customerportalapp.CreateProcessingRequestCommand{
		CustomerID:          customerAID,
		CreatedByMiniUserID: 1,
		InputMaterialID:     7003,
		InputQtyG:           1000,
		TargetProductID:     targetProductID,
		TargetSpecG:         454,
		TargetQty:           2,
	})
	if err == nil || !strings.Contains(err.Error(), "target product unavailable") {
		t.Fatalf("CreateProcessingRequest error=%v, want target product unavailable", err)
	}
	assertNoProcessingRequestRows(t, ctx, pool, schema, customerAID, "another customer processing request created")
}

func seedProcessingBoundaryCustomers(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) (int64, int64) {
	t.Helper()
	var customerAID, customerBID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('代加工客户A') RETURNING id
	`, schema)).Scan(&customerAID); err != nil {
		t.Fatalf("insert customer A: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('代加工客户B') RETURNING id
	`, schema)).Scan(&customerBID); err != nil {
		t.Fatalf("insert customer B: %v", err)
	}
	return customerAID, customerBID
}

func seedProcessingBoundaryProduct(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string, customerID int64, visibility, name string) int64 {
	t.Helper()
	var productID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(name, default_price, active, customer_id, visibility)
		VALUES($1, 68, true, $2, $3)
		RETURNING id
	`, schema), name, customerID, visibility).Scan(&productID); err != nil {
		t.Fatalf("insert product %q: %v", name, err)
	}
	return productID
}

func seedProcessingBoundaryInventory(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string, customerID, itemID int64, name string, qtyG int64) {
	t.Helper()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_inventory_items(customer_id, item_type, item_id, item_name, spec_g, warehouse, qty_g, qty_units, status)
		VALUES($1, 'raw_bean', $2, $3, 0, 'cust_processing', $4, 0, 'available')
	`, schema), customerID, itemID, name, qtyG); err != nil {
		t.Fatalf("insert inventory %q: %v", name, err)
	}
}

func assertNoProcessingRequestRows(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string, customerID int64, message string) {
	t.Helper()
	var rows int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %s.processing_job_requests WHERE customer_id=$1
	`, schema), customerID).Scan(&rows); err != nil {
		t.Fatalf("count processing requests: %v", err)
	}
	if rows != 0 {
		t.Fatalf("%s rows=%d", message, rows)
	}
}

func TestLoadSettlementServicePageFiltersFinanceRowsByCustomer(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	repo := NewRepository(pool, schema)

	var customerAID, customerBID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('结算客户A') RETURNING id
	`, schema)).Scan(&customerAID); err != nil {
		t.Fatalf("insert customer A: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('结算客户B') RETURNING id
	`, schema)).Scan(&customerBID); err != nil {
		t.Fatalf("insert customer B: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_fee_items(customer_id, source_type, source_id, fee_type, amount, currency, occurred_at, settlement_batch_id, status, note)
		VALUES
			($1,'manual_adjustment',101,'shipping',12.34,'CNY','2026-05-13 09:00:00+08',0,'unsettled','客户A费用'),
			($2,'manual_adjustment',202,'shipping',56.78,'CNY','2026-05-13 10:00:00+08',0,'unsettled','客户B不应泄露')
	`, schema), customerAID, customerBID); err != nil {
		t.Fatalf("insert fee items: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_settlement_batches(customer_id, settlement_no, period_from, period_to, status, total_amount, confirmed_at, paid_at, created_by)
		VALUES
			($1,'客户A结算单','2026-05-01','2026-05-31','confirmed',12.34,'2026-05-31 10:00:00+08',NULL,'tester'),
			($2,'客户B不应泄露','2026-05-01','2026-05-31','confirmed',56.78,'2026-05-31 11:00:00+08',NULL,'tester')
	`, schema), customerAID, customerBID); err != nil {
		t.Fatalf("insert settlement batches: %v", err)
	}

	got, err := repo.LoadServicePage(ctx, customerportalapp.ServicePageQuery{
		CustomerID: customerAID,
		Key:        customerportalapp.ServiceKeySettlement,
		Limit:      20,
	})
	if err != nil {
		t.Fatalf("LoadServicePage(settlement): %v", err)
	}
	if got.Key != customerportalapp.ServiceKeySettlement {
		t.Fatalf("page key=%q, want settlement", got.Key)
	}
	if len(got.FeeItems) != 1 || got.FeeItems[0].Note != "客户A费用" || got.FeeItems[0].Amount != "12.34" {
		t.Fatalf("fee_items=%+v, want only customer A fee", got.FeeItems)
	}
	if len(got.SettlementBatches) != 1 || got.SettlementBatches[0].SettlementNo != "客户A结算单" || got.SettlementBatches[0].TotalAmount != "12.34" {
		t.Fatalf("settlement_batches=%+v, want only customer A batch", got.SettlementBatches)
	}
	if strings.Contains(fmt.Sprintf("%+v", got), "客户B不应泄露") {
		t.Fatalf("settlement service page leaked customer B finance rows: %+v", got)
	}
}

func TestEnsureSchemaRejectsNonObjectCapabilityConfig(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('客户') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	_, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_service_capabilities(customer_id, capability_code, config_json)
		VALUES($1,'bad_config','[]'::jsonb)
	`, schema), customerID)
	if err == nil {
		t.Fatal("insert array config_json err=nil, want check constraint error")
	}
}

func TestEnsureSchemaToleratesExistingNonObjectCapabilityConfig(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalCoreOnlyTestDB(t)

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('历史客户') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %[1]s.customer_service_capabilities (
			id BIGSERIAL PRIMARY KEY,
			customer_id BIGINT NOT NULL REFERENCES %[1]s.customers(id) ON DELETE CASCADE,
			capability_code TEXT NOT NULL,
			enabled BOOLEAN NOT NULL DEFAULT true,
			config_json JSONB NOT NULL DEFAULT '{}'::jsonb,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`, schema)); err != nil {
		t.Fatalf("create legacy capability table: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_service_capabilities(customer_id, capability_code, config_json)
		VALUES($1,'legacy_bad','[]'::jsonb)
	`, schema), customerID); err != nil {
		t.Fatalf("seed legacy capability config: %v", err)
	}

	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema with existing non-object config: %v", err)
	}
	_, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_service_capabilities(customer_id, capability_code, config_json)
		VALUES($1,'new_bad','[]'::jsonb)
	`, schema), customerID)
	if err == nil {
		t.Fatal("insert new array config_json err=nil, want NOT VALID check constraint to reject new bad data")
	}
}

func TestCreateLoginSessionDoesNotReactivateDisabledMiniUser(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	repo := NewRepository(pool, schema)

	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.mini_users(openid, unionid, phone, nickname, active)
		VALUES('disabled-openid','old-union','old-phone','old-name',false)
	`, schema)); err != nil {
		t.Fatalf("insert disabled mini user: %v", err)
	}

	_, err := repo.CreateLoginSession(ctx, customerportalapp.CreateLoginSessionCommand{
		OpenID:   " disabled-openid ",
		UnionID:  "new-union",
		Phone:    "new-phone",
		Nickname: "new-name",
	})
	if !errors.Is(err, customerportalapp.ErrMiniUserDisabled) {
		t.Fatalf("CreateLoginSession() err=%v, want ErrMiniUserDisabled", err)
	}

	var active bool
	var unionID, phone, nickname string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT active, unionid, phone, nickname
		FROM %[1]s.mini_users
		WHERE openid='disabled-openid'
	`, schema)).Scan(&active, &unionID, &phone, &nickname); err != nil {
		t.Fatalf("load disabled mini user: %v", err)
	}
	if active || unionID != "old-union" || phone != "old-phone" || nickname != "old-name" {
		t.Fatalf("disabled mini user changed active=%v union=%q phone=%q nickname=%q", active, unionID, phone, nickname)
	}
	var sessions int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT count(*) FROM %[1]s.mini_sessions s
		JOIN %[1]s.mini_users u ON u.id=s.mini_user_id
		WHERE u.openid='disabled-openid'
	`, schema)).Scan(&sessions); err != nil {
		t.Fatalf("count disabled user sessions: %v", err)
	}
	if sessions != 0 {
		t.Fatalf("disabled mini user sessions=%d, want 0", sessions)
	}
}

func TestNextCustomerPortalOrderNoIgnoresNonNumericSameDaySuffix(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)

	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.orders(order_no, order_date)
		VALUES ('SO-20260513-PAGE', '2026-05-13'),
		       ('SO-20260513-0007', '2026-05-13')
	`, schema)); err != nil {
		t.Fatalf("insert existing orders: %v", err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback(ctx)

	got, err := nextCustomerPortalOrderNo(ctx, tx, schema, time.Date(2026, 5, 13, 9, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatalf("nextCustomerPortalOrderNo: %v", err)
	}
	if got != "SO-20260513-0008" {
		t.Fatalf("next order no = %q, want SO-20260513-0008", got)
	}
}

func TestLoadBeanListPublicationRejectsAnotherCustomerPublication(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	ensureCustomerPortalCostingSchema(t, ctx, pool, schema)
	repo := NewRepository(pool, schema)

	var customerAID, customerBID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('客户A') RETURNING id
	`, schema)).Scan(&customerAID); err != nil {
		t.Fatalf("insert customer A: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('客户B') RETURNING id
	`, schema)).Scan(&customerBID); err != nil {
		t.Fatalf("insert customer B: %v", err)
	}
	publicationID := seedBeanListPublicationForCustomerPortalTest(t, ctx, pool, schema, "customer", fmt.Sprintf("%d", customerBID), "客户B专属豆单不应下载")

	_, err := repo.LoadBeanListPublication(ctx, customerAID, publicationID)
	if !errors.Is(err, customerportalapp.ErrBeanListPublicationNotFound) {
		t.Fatalf("LoadBeanListPublication err=%v, want ErrBeanListPublicationNotFound", err)
	}
}

func TestLoadBeanListPublicationAllowsOfficialPublication(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	ensureCustomerPortalCostingSchema(t, ctx, pool, schema)
	repo := NewRepository(pool, schema)

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('客户A') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	publicationID := seedBeanListPublicationForCustomerPortalTest(t, ctx, pool, schema, "official", "", "官方已发布豆单可下载")

	got, err := repo.LoadBeanListPublication(ctx, customerID, publicationID)
	if err != nil {
		t.Fatalf("LoadBeanListPublication official: %v", err)
	}
	wantCacheKey := fmt.Sprintf("bean-list:%d:V3.0.192", publicationID)
	if got.ID != publicationID || got.Title != "官方已发布豆单可下载" || got.CacheKey != wantCacheKey {
		t.Fatalf("official publication=%+v, want id=%d title/cache key", got, publicationID)
	}
}

func ensureCustomerPortalCostingSchema(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) {
	t.Helper()
	if err := postgrescosting.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("costing.EnsureSchema: %v", err)
	}
}

func ensureCustomerPortalERPBindingTestSchema(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) {
	t.Helper()
	if err := postgrescompany.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("company.EnsureSchema: %v", err)
	}
	if err := postgrescustomerfulfillment.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("customerfulfillment.EnsureSchema: %v", err)
	}
}

func customerPortalTestPasswordHash(raw string) string {
	sum := sha256.Sum256([]byte("orderapp-mobile-auth:" + raw))
	return fmt.Sprintf("%x", sum[:])
}

func seedBeanListPublicationForCustomerPortalTest(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema, ownerType, ownerKey, title string) int64 {
	t.Helper()
	var publicationID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.bean_list_publications(
			list_type, version_no, status, owner_type, owner_key, config_json, content_json, changelog, actor
		)
		VALUES('commercial','V3.0.192','published',$1,$2,'{}'::jsonb,$3::jsonb,'PR-192 tenant isolation','codex')
		RETURNING id
	`, schema), ownerType, ownerKey, fmt.Sprintf(`{"title":%q}`, title)).Scan(&publicationID); err != nil {
		t.Fatalf("insert bean list publication: %v", err)
	}
	return publicationID
}

func newCustomerPortalTestDB(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	pool, schema := newCustomerPortalCoreOnlyTestDB(t)
	if err := EnsureSchema(context.Background(), pool, schema); err != nil {
		t.Fatalf("customerportal.EnsureSchema: %v", err)
	}
	if err := postgresmaterials.EnsureSchema(context.Background(), pool, schema); err != nil {
		t.Fatalf("materials.EnsureSchema: %v", err)
	}
	return pool, schema
}

func newCustomerPortalCoreOnlyTestDB(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required for customer portal postgres tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	schema := fmt.Sprintf("test_customer_portal_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		pool.Close()
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	})
	if err := postgrescore.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("core.EnsureSchema: %v", err)
	}
	return pool, schema
}
