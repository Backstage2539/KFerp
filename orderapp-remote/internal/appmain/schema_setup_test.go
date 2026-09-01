package appmain

import (
	"os"
	"strings"
	"testing"
)

func TestSchemaSetupInitializesCoreCustomerAndCompanyDependenciesInOrder(t *testing.T) {
	body, err := os.ReadFile("internal/appmain/schema_setup.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(body)
	company := strings.Index(src, `Name: "company"`)
	core := strings.Index(src, `Name: "core"`)
	customer := strings.Index(src, `Name: "customer"`)
	customerPortal := strings.Index(src, `Name: "customerportal"`)
	support := strings.Index(src, `Name: "support"`)
	authz := strings.Index(src, `Name: "authz"`)
	if company < 0 || core < 0 || customer < 0 || customerPortal < 0 || support < 0 || authz < 0 {
		t.Fatalf("schema steps missing company/core/customer/customerportal/support/authz: company=%d core=%d customer=%d customerportal=%d support=%d authz=%d", company, core, customer, customerPortal, support, authz)
	}
	if !(company < support && company < authz) {
		t.Fatalf("company schema must run before support/authz because both reference employees")
	}
	if !(core < customer && customer < customerPortal) {
		t.Fatalf("customer schema must run after core customers and before customer portal: core=%d customer=%d customerportal=%d", core, customer, customerPortal)
	}
}

func TestSchemaSetupSynchronizesSerialIDSequencesLast(t *testing.T) {
	body, err := os.ReadFile("internal/appmain/schema_setup.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(body)
	contracts := strings.Index(src, `Name: "contracts"`)
	sequences := strings.Index(src, `Name: "serial-id-sequences"`)
	wiring := strings.Index(src, "postgresinfra.SyncSerialIDSequences")
	if contracts < 0 || sequences < 0 || wiring < 0 {
		t.Fatalf("schema setup must wire final serial ID sequence synchronization: contracts=%d sequences=%d wiring=%d", contracts, sequences, wiring)
	}
	if sequences < contracts {
		t.Fatalf("serial ID sequences must be synchronized after every module schema: contracts=%d sequences=%d", contracts, sequences)
	}
}

func TestSchemaSetupCreatesBOMAuthorityBeforeDependentModulesAndReinstallsGuardsAfterThem(t *testing.T) {
	body, err := os.ReadFile("internal/appmain/schema_setup.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(body)
	bom := strings.Index(src, `Name: "bom"`)
	catalog := strings.Index(src, `Name: "catalog"`)
	base := strings.Index(src, `Name: "product-bom-spec-authority-base"`)
	portal := strings.Index(src, `Name: "customerportal"`)
	fulfillment := strings.Index(src, `Name: "customerfulfillment"`)
	guards := strings.Index(src, `Name: "product-bom-spec-authority-guards"`)
	if !(bom >= 0 && catalog > bom && base > catalog && portal > base && fulfillment > base && guards > fulfillment) {
		t.Fatalf("BOM authority schema order invalid: bom=%d catalog=%d base=%d portal=%d fulfillment=%d guards=%d", bom, catalog, base, portal, fulfillment, guards)
	}
}
