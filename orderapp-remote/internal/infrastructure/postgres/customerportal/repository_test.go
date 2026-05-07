package customerportal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	customerportalapp "orderapp/internal/application/customerportal"
	postgrescore "orderapp/internal/infrastructure/postgres/core"

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

func newCustomerPortalTestDB(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	pool, schema := newCustomerPortalCoreOnlyTestDB(t)
	if err := EnsureSchema(context.Background(), pool, schema); err != nil {
		t.Fatalf("customerportal.EnsureSchema: %v", err)
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
