package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev379ChannelPortalWorkbenchSwitchRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-379-CHANNEL-PORTAL-WORKBENCH-SWITCH",
		"DEV-379-CUSTOMER-PORTAL-SWITCH",
		"DEV-379-CHANNEL-FULFILLMENT-CUSTOMER",
		"DEV-379-ORDER-ENTRY-CATEGORY-PRICELIST",
		"UT-379-CHANNEL-PORTAL-WORKBENCH-SWITCH",
		"API-379-CHANNEL-PORTAL-WORKBENCH-SWITCH",
		"REV-379-CHANNEL-PORTAL-WORKBENCH-SWITCH",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("PR-376 requirement seed missing %q", want)
		}
	}
}

func TestDev379ChannelPortalWorkbenchSwitchSourceMarkers(t *testing.T) {
	customerService := string(readOrderAppFileForTest(t, filepath.Join("internal", "application", "customer", "service.go")))
	for _, want := range []string{
		"CustomerTypeChannel",
		`"channel"`,
		"PortalEnabled",
		"CapabilityTemplateKey",
	} {
		if !strings.Contains(customerService, want) {
			t.Fatalf("customer service missing PR-376 marker %q", want)
		}
	}
	if strings.Contains(customerService, "请维护客户门户/工作台：能力模板") {
		t.Fatal("customer profile service should not require capability template when portal switch is enabled")
	}

	customerRoutes := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "customer", "customer_routes.go")))
	for _, want := range []string{`json:"portal_enabled"`, `json:"capability_template_key"`} {
		if !strings.Contains(customerRoutes, want) {
			t.Fatalf("customer routes missing PR-376 marker %q", want)
		}
	}

	customerRepo := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customer", "repository.go")))
	for _, want := range []string{
		"syncCustomerPortalProfileTx",
		"customer_portal_profiles",
		"portal_enabled",
		"capability_template_key",
	} {
		if !strings.Contains(customerRepo, want) {
			t.Fatalf("customer repository missing PR-376 marker %q", want)
		}
	}

	portalService := string(readOrderAppFileForTest(t, filepath.Join("internal", "application", "customerportal", "service.go")))
	for _, want := range []string{
		"CapabilityTemplateChannelDirectShip",
		`"channel_direct_ship"`,
		"渠道代发/现货下单",
		"external_recipients",
		"customerProcessingPortal",
	} {
		if !strings.Contains(portalService, want) {
			t.Fatalf("customer portal service missing PR-376 marker %q", want)
		}
	}

	fulfillmentAPI := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "customerfulfillment", "api.go")))
	if strings.Contains(fulfillmentAPI, `customer_type = 'wholesale'`) || strings.Contains(fulfillmentAPI, `CustomerTypeWholesale`) {
		t.Fatalf("customer fulfillment candidates must not hardcode wholesale customer type")
	}
	if !strings.Contains(fulfillmentAPI, "CustomerERPWorkbenchAvailable") {
		t.Fatalf("customer fulfillment candidates should use capability/workbench availability")
	}

	portalAdminRepo := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerportal", "admin_repository.go")))
	for _, want := range []string{"customer_portal_profiles p", "p.enabled=true"} {
		if !strings.Contains(portalAdminRepo, want) {
			t.Fatalf("customer portal admin repository missing enabled-profile marker %q", want)
		}
	}

	customersView := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "CustomersView.vue")))
	for _, want := range []string{
		"开通客户门户/工作台",
		"portal_enabled",
	} {
		if !strings.Contains(customersView, want) {
			t.Fatalf("CustomersView.vue missing PR-376 marker %q", want)
		}
	}
	for _, forbidden := range []string{"capability_template_key", "defaultCapabilityTemplateForCustomerType", "请选择能力模板"} {
		if strings.Contains(customersView, forbidden) {
			t.Fatalf("CustomersView.vue should not bind capability templates in customer profile: found %q", forbidden)
		}
	}

	customerTypes := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "customer-types.js")))
	for _, want := range []string{"channel", "渠道客户"} {
		if !strings.Contains(customerTypes, want) {
			t.Fatalf("customer-types.js missing PR-376 marker %q", want)
		}
	}
	if strings.Contains(customerTypes, "channel_direct_ship") {
		t.Fatal("customer-types.js should not map customer type to capability template")
	}

	orderEntryView := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "OrderEntryView.vue")))
	for _, want := range []string{
		"beanListVersionGroups",
		"beanListGroupKeyForProduct",
		"selectedBeanListPublicationIDs",
		"product_type_category_id",
	} {
		if !strings.Contains(orderEntryView, want) {
			t.Fatalf("OrderEntryView.vue missing PR-376 marker %q", want)
		}
	}

	for _, rel := range []string{
		filepath.Join("frontend-vue-shell", "src", "views", "CustomerFulfillmentView.vue"),
		filepath.Join("frontend-vue-shell", "src", "views", "CustomerProcessingPortalView.vue"),
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{
			"历史收件信息",
			"recipientOptions",
			"receiver_name",
			"receiver_address",
		} {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-376 recipient marker %q", rel, want)
			}
		}
	}
}

func TestDev379ChannelPortalWorkbenchSwitchDocs(t *testing.T) {
	for _, rel := range []string{
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_FULFILLMENT.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "acceptance", "2026-05-26-channel-portal-workbench-switch.md"),
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{
			"PR-379-CHANNEL-PORTAL-WORKBENCH-SWITCH",
			"开通客户门户/工作台",
			"渠道客户",
			"能力模板",
		} {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-376 documentation marker %q", rel, want)
			}
		}
	}
}
