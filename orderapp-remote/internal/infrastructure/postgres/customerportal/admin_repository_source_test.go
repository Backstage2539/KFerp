package customerportal

import (
	"os"
	"strings"
	"testing"
)

func TestCustomerPortalAdminRepositoryPersistsProfilesCapabilitiesAndBindings(t *testing.T) {
	body, err := os.ReadFile("admin_repository.go")
	if err != nil {
		t.Fatalf("read admin_repository.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"ListPortalAdminCustomers",
		"PortalAdminDetail",
		"UpdatePortalVisibility",
		"customer_portal_profiles",
		"processing_warehouse_code",
		"default_sender_id",
		"customer_processing",
		"warehouses",
		"customer_portal_user_bindings",
		"customer_service_capabilities",
		"theme_key",
		"miniapp_entry_mode",
		"NormalizePortalThemeKey",
		"NormalizeMiniappEntryMode",
		"ON CONFLICT(customer_id, capability_code) DO UPDATE",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("admin repository missing %q", want)
		}
	}
}

func TestCustomerPortalAdminRepositoryShowsOnlyEnabledPortalCustomersAndERPBinding(t *testing.T) {
	body, err := os.ReadFile("admin_repository.go")
	if err != nil {
		t.Fatalf("read admin_repository.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"p.enabled=true",
		"customer_erp_user_bindings",
		"company_employees",
		"account_type='channel_customer'",
		"employee_login_passwords",
		"login_disabled",
		"UpsertPortalERPBinding",
		"ERPBinding",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("admin repository missing wholesale/ERP binding guard %q", want)
		}
	}
	if strings.Contains(text, "COALESCE(NULLIF(c.customer_type,''),'retail')='wholesale'") {
		t.Fatalf("admin repository should not hardcode portal configuration list to wholesale customers")
	}
}

func TestCustomerPortalAdminRepositoryAuditsPortalProfileFields(t *testing.T) {
	body, err := os.ReadFile("admin_repository.go")
	if err != nil {
		t.Fatalf("read admin_repository.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		`auditPortalProfileVisibilityTx(ctx, tx, r.schema, cmd, oldProfile)`,
		`auditPortalProfileBoolField(ctx, tx, schema, actor, cmd.CustomerID, old.exists, "enabled"`,
		`auditPortalProfileTextField(ctx, tx, schema, actor, cmd.CustomerID, old.exists, "capability_template_key"`,
		`auditPortalProfileTextField(ctx, tx, schema, actor, cmd.CustomerID, old.exists, "display_name"`,
		`auditPortalProfileTextField(ctx, tx, schema, actor, cmd.CustomerID, old.exists, "processing_warehouse_code"`,
		`auditPortalProfileIntField(ctx, tx, schema, actor, cmd.CustomerID, old.exists, "default_sender_id"`,
		`auditPortalProfileTextField(ctx, tx, schema, actor, cmd.CustomerID, old.exists, "bean_list_version"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("admin repository missing portal profile audit marker %q", want)
		}
	}
}

func TestCustomerPortalAdminRepositoryDoesNotAutoCreateProcessingWarehouseFromCapabilities(t *testing.T) {
	body, err := os.ReadFile("admin_repository.go")
	if err != nil {
		t.Fatalf("read admin_repository.go: %v", err)
	}
	text := string(body)
	if strings.Contains(text, `warehouseCode == "" && capabilityEnabled`) {
		t.Fatalf("admin repository should not auto-default processing warehouses from portal capability settings")
	}
	if !strings.Contains(text, "portalCustomerWarehouses") || !strings.Contains(text, "customer_id=$1") {
		t.Fatalf("admin repository should expose customer-bound warehouses from the stock warehouse binding")
	}
}

func TestCustomerPortalAdminRepositoryPersistsMallProducts(t *testing.T) {
	body, err := os.ReadFile("admin_repository.go")
	if err != nil {
		t.Fatalf("read admin_repository.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"ListMallProducts",
		"SaveMallProduct",
		"UpdateMallProductImage",
		"mall_products",
		"product_options",
		"NormalizeMallTemplateKey",
		"NormalizeMallProductStatus",
		"ON CONFLICT(id) DO UPDATE",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("mall admin repository missing %q", want)
		}
	}
}
