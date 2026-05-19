package catalog

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestInsertIDAtPositionReordersWithoutDuplicatePositionTie(t *testing.T) {
	cases := []struct {
		name     string
		ids      []int64
		movedID  int64
		position int
		want     []int64
	}{
		{name: "move to first", ids: []int64{1, 3}, movedID: 2, position: 1, want: []int64{2, 1, 3}},
		{name: "move to middle", ids: []int64{1, 3}, movedID: 2, position: 2, want: []int64{1, 2, 3}},
		{name: "append when position is too large", ids: []int64{1, 3}, movedID: 2, position: 99, want: []int64{1, 3, 2}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := insertIDAtPosition(tc.ids, tc.movedID, tc.position)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("insertIDAtPosition() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestProductMarginOverridePersistsOnProducts(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	queries, err := os.ReadFile("../catalog_queries.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		src  string
		want string
	}{
		{name: "schema column", src: string(schema), want: "ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS margin_rate_override NUMERIC(14,6)"},
		{name: "product fetch", src: string(queries), want: "p.margin_rate_override::float8"},
		{name: "product get fallback", src: string(repository), want: "margin_rate_override::float8"},
		{name: "product update", src: string(repository), want: "margin_rate_override=$9"},
		{name: "audit metadata", src: string(repository), want: `"margin_rate_override": cmd.MarginRateOverride`},
	} {
		if !strings.Contains(tc.src, tc.want) {
			t.Fatalf("catalog product margin override persistence missing %s marker %q", tc.name, tc.want)
		}
	}
}

func TestProductRemarkPersistsOnProducts(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	queries, err := os.ReadFile("../catalog_queries.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		src  string
		want string
	}{
		{name: "schema column", src: string(schema), want: "ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS remark TEXT NOT NULL DEFAULT ''"},
		{name: "product list query", src: string(queries), want: "COALESCE(p.remark,'')"},
		{name: "product get query", src: string(repository), want: "COALESCE(remark,'')"},
		{name: "product create insert", src: string(repository), want: "name, remark, product_kind"},
		{name: "product update", src: string(repository), want: "remark=$15"},
		{name: "custom product insert", src: string(repository), want: "name, remark, product_kind, roast_level"},
		{name: "audit metadata", src: string(repository), want: `"remark":`},
	} {
		if !strings.Contains(tc.src, tc.want) {
			t.Fatalf("catalog product remark persistence missing %s marker %q", tc.name, tc.want)
		}
	}
}

func TestProductKindSchemaRepairsPartiallyCreatedColumns(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(schema)
	for _, want := range []string{
		"UPDATE %[1]s.products SET product_kind='roasted_bean' WHERE COALESCE(product_kind,'')=''",
		"UPDATE %[1]s.products SET drip_bag_grams = 10 WHERE drip_bag_grams IS NULL",
		"UPDATE %[1]s.products SET drip_box_bag_count = 10 WHERE drip_box_bag_count IS NULL",
		"UPDATE %[1]s.products SET allow_fulfillment_order = true WHERE allow_fulfillment_order IS NULL",
		"UPDATE %[1]s.products SET allow_mall_order = false WHERE allow_mall_order IS NULL",
		"ALTER TABLE %[1]s.products ALTER COLUMN product_kind SET DEFAULT 'roasted_bean'",
		"ALTER TABLE %[1]s.products ALTER COLUMN drip_bag_grams SET DEFAULT 10",
		"ALTER TABLE %[1]s.products ALTER COLUMN drip_box_bag_count SET DEFAULT 10",
		"ALTER TABLE %[1]s.products ALTER COLUMN allow_fulfillment_order SET DEFAULT true",
		"ALTER TABLE %[1]s.products ALTER COLUMN allow_mall_order SET DEFAULT false",
		"ALTER TABLE %[1]s.products ALTER COLUMN product_kind SET NOT NULL",
		"ALTER TABLE %[1]s.products ALTER COLUMN drip_bag_grams SET NOT NULL",
		"ALTER TABLE %[1]s.products ALTER COLUMN drip_box_bag_count SET NOT NULL",
		"ALTER TABLE %[1]s.products ALTER COLUMN allow_fulfillment_order SET NOT NULL",
		"ALTER TABLE %[1]s.products ALTER COLUMN allow_mall_order SET NOT NULL",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("schema missing product kind repair marker %q", want)
		}
	}
}

func TestProductQueriesReturnGreenBeanMetadata(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	queries, err := os.ReadFile("../catalog_queries.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		src  string
		want string
	}{
		{name: "list green type", src: string(queries), want: "p.green_bean_type"},
		{name: "list green bom", src: string(queries), want: "p.green_bean_bom_product_id"},
		{name: "detail green type", src: string(repository), want: "green_bean_type"},
		{name: "detail green bom", src: string(repository), want: "green_bean_bom_product_id"},
	} {
		if !strings.Contains(tc.src, tc.want) {
			t.Fatalf("product query missing %s marker %q", tc.name, tc.want)
		}
	}
}

func TestCreateCustomProductCopiesDripProductMetadata(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(repository)
	for _, want := range []string{
		"COALESCE(NULLIF(product_kind,''), 'roasted_bean')",
		"COALESCE(drip_bag_grams,10)",
		"COALESCE(drip_box_bag_count,10)",
		"COALESCE(allow_fulfillment_order,true)",
		"COALESCE(allow_mall_order,false)",
		"drip_bag_grams, drip_box_bag_count, allow_fulfillment_order, allow_mall_order,",
		"productKind, roastLevel, base.DefaultPrice",
		"dripBagGrams, dripBoxBagCount, base.AllowFulfillmentOrder, base.AllowMallOrder",
		"product_kind,price_basis,sales_unit,unit_bag_count,price_source_json",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("custom product drip metadata copy missing marker %q", want)
		}
	}
}

func TestCreateCustomProductCopyBOMPreservesComponentFields(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(repository)
	for _, want := range []string{
		"INSERT INTO %s.product_bom_items(product_id,material_id,component_type,component_product_id,component_spec_g,consume_unit,qty_per_unit,ratio_pct,updated_at)",
		"SELECT $1,material_id,component_type,component_product_id,component_spec_g,consume_unit,qty_per_unit,ratio_pct,now()",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("custom product BOM copy missing component marker %q", want)
		}
	}
}

func TestCreateCustomProductInsertDoesNotDuplicateProductKindColumn(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(repository)
	if strings.Contains(src, "product_kind, drip_bag_grams, drip_box_bag_count, allow_fulfillment_order, allow_mall_order,\n\t\t\tproduct_category_id") {
		t.Fatalf("custom product insert still duplicates product_kind and shifts insert values")
	}
}

func TestCreateCustomProductAllowsGreenBeanWithoutBaseProduct(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(repository)
	for _, want := range []string{
		"baseProductID := cmd.BaseProductID",
		"if cmd.BaseProductID > 0 {",
		"baseProductID = 0",
		"cmd.CopyPriceTiers && baseProductID > 0",
		`"base_product_id":           baseProductID`,
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("custom green product without base product missing marker %q", want)
		}
	}
}

func TestCustomerPublicUsagePersistsReferenceSwitchesAndAudits(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(repository)
	for _, want := range []string{
		"func (r Repository) SaveCustomerPublicUsage",
		"customer_sku_public_usage",
		"update_public_usage",
		"cleanupLegacyPublicCopiesTx",
		"AuditInsertTx(ctx, tx, r.schema, cmd.Actor, \"customer_product_catalog\"",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("customer public usage repository missing marker %q", want)
		}
	}
}

func TestCustomerPublicUsageDoesNotInsertCopiedPublicProductsOrCategories(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(repository)
	for _, forbidden := range []string{
		"func insertCustomerProductCopyTx",
		"func ensureCustomerCategoryCopyTx",
		"func fetchPublicProductCopyRowsTx",
		"func fetchPublicCategoryCopyRowsTx",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("public usage should reference public catalog without copy helper %q", forbidden)
		}
	}
}

func TestProductSchemaDropsLegacyGlobalProductNameUniqueness(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(schema)
	for _, want := range []string{
		"ALTER TABLE %[1]s.products DROP CONSTRAINT IF EXISTS products_name_key",
		"DROP INDEX IF EXISTS %[1]s.products_name_key",
		"CREATE TABLE IF NOT EXISTS %[1]s.customer_sku_public_usage",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("schema must remove legacy global product name uniqueness so customer SKU copies can keep public names, missing %q", want)
		}
	}
}

func TestTemplateOwnershipSchemaPersistsSourceAndUsageSwitches(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(schema)
	for _, want := range []string{
		"ALTER TABLE %[1]s.product_categories ADD COLUMN IF NOT EXISTS source_category_id BIGINT NOT NULL DEFAULT 0",
		"ALTER TABLE %[1]s.product_categories ADD COLUMN IF NOT EXISTS template_state TEXT NOT NULL DEFAULT 'customer_owned'",
		"ALTER TABLE %[1]s.pricing_gradient_templates ADD COLUMN IF NOT EXISTS customer_id BIGINT NOT NULL DEFAULT 0",
		"ALTER TABLE %[1]s.pricing_gradient_templates ADD COLUMN IF NOT EXISTS source_template_id BIGINT NOT NULL DEFAULT 0",
		"ALTER TABLE %[1]s.pricing_gradient_templates ADD COLUMN IF NOT EXISTS template_state TEXT NOT NULL DEFAULT 'customer_owned'",
		"use_public_gradient_templates BOOLEAN NOT NULL DEFAULT false",
		"product_categories_customer_source_active_uniq",
		"pricing_gradient_templates_customer_source_active_uniq",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("template ownership schema missing marker %q", want)
		}
	}
}

func TestTemplateDerivationRepositoryAuditsAndCopiesPublicSources(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(repository)
	for _, want := range []string{
		"func (r Repository) DeriveProductCategory",
		"func (r Repository) DeriveCustomerProduct",
		"func (r Repository) DeriveGradientTemplate",
		"derive_public_category",
		"derive_public_sku",
		"derive_public_gradient_template",
		"source_category_id",
		"source_template_id",
		"public category requires derivation",
		"public product requires derivation",
		"targetTemplateID = derived.ID",
		"derived_public_template",
		"if existingID > 0 {\n\t\treturn fetchCatalogProductByIDTx",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("template derivation repository missing marker %q", want)
		}
	}
	if strings.Contains(src, "UPDATE %s.products SET name=$2, product_category_id=NULLIF($3,0), product_category_position=$4") {
		t.Fatalf("re-copying an existing public SKU alias must not overwrite the customer copy name or category")
	}
}
