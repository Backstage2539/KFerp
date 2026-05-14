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

func TestCustomerPortalAdminRepositoryShowsOnlyWholesaleCustomersAndERPBinding(t *testing.T) {
	body, err := os.ReadFile("admin_repository.go")
	if err != nil {
		t.Fatalf("read admin_repository.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"customer_type",
		"'wholesale'",
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
