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
		{name: "audit metadata", src: string(repository), want: `"margin_rate_override":           cmd.MarginRateOverride`},
	} {
		if !strings.Contains(tc.src, tc.want) {
			t.Fatalf("catalog product margin override persistence missing %s marker %q", tc.name, tc.want)
		}
	}
}

func TestProductSubtypeConfigAndUnitRulesPersistOnCategories(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		src  string
		want string
	}{
		{name: "operation template column", src: string(schema), want: "ALTER TABLE %[1]s.product_categories ADD COLUMN IF NOT EXISTS operation_template_id BIGINT NOT NULL DEFAULT 0"},
		{name: "price list rule column", src: string(schema), want: "ALTER TABLE %[1]s.product_categories ADD COLUMN IF NOT EXISTS price_list_rule_json JSONB NOT NULL DEFAULT '{}'::jsonb"},
		{name: "inventory unit column", src: string(schema), want: "ALTER TABLE %[1]s.product_categories ADD COLUMN IF NOT EXISTS inventory_unit TEXT NOT NULL DEFAULT 'kg'"},
		{name: "quote unit column", src: string(schema), want: "ALTER TABLE %[1]s.product_categories ADD COLUMN IF NOT EXISTS quote_unit TEXT NOT NULL DEFAULT 'kg'"},
		{name: "order unit column", src: string(schema), want: "ALTER TABLE %[1]s.product_categories ADD COLUMN IF NOT EXISTS order_unit TEXT NOT NULL DEFAULT 'kg'"},
		{name: "unit conversion column", src: string(schema), want: "ALTER TABLE %[1]s.product_categories ADD COLUMN IF NOT EXISTS unit_conversion_json JSONB NOT NULL DEFAULT '{}'::jsonb"},
		{name: "integer unit column", src: string(schema), want: "ALTER TABLE %[1]s.product_categories ADD COLUMN IF NOT EXISTS integer_unit BOOLEAN NOT NULL DEFAULT false"},
		{name: "list categories selects operation template", src: string(repository), want: "COALESCE(operation_template_id,0)"},
		{name: "list categories selects unit rule", src: string(repository), want: "COALESCE(inventory_unit,'kg')"},
		{name: "save category writes unit rule", src: string(repository), want: "unit_conversion_json=$13::jsonb"},
	} {
		if !strings.Contains(tc.src, tc.want) {
			t.Fatalf("product subtype config persistence missing %s marker %q", tc.name, tc.want)
		}
	}
}

func TestProductConfigOverridesPersistOnProducts(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		src  string
		want string
	}{
		{name: "gradient override column", src: string(schema), want: "ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS gradient_template_id_override BIGINT NOT NULL DEFAULT 0"},
		{name: "operation override column", src: string(schema), want: "ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS operation_template_id_override BIGINT NOT NULL DEFAULT 0"},
		{name: "unit override column", src: string(schema), want: "ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS unit_rule_override_json JSONB NOT NULL DEFAULT '{}'::jsonb"},
		{name: "product update writes overrides", src: string(repository), want: "gradient_template_id_override=$17"},
		{name: "product fetch reads overrides", src: string(repository), want: "unit_rule_override_json::text"},
	} {
		if !strings.Contains(tc.src, tc.want) {
			t.Fatalf("product config override persistence missing %s marker %q", tc.name, tc.want)
		}
	}
}

func TestCustomerProductRuleTemplateSchemaPersistsTemplatesAndOverrides(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(schema)
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS %[1]s.customer_product_rule_templates",
		"CREATE TABLE IF NOT EXISTS %[1]s.customer_product_rule_template_items",
		"CREATE TABLE IF NOT EXISTS %[1]s.customer_product_rule_overrides",
		"product_subtype_category_id BIGINT NOT NULL DEFAULT 0",
		"gradient_template_id BIGINT NOT NULL DEFAULT 0",
		"operation_template_id BIGINT NOT NULL DEFAULT 0",
		"unit_rule_json JSONB NOT NULL DEFAULT '{}'::jsonb",
		"customer_product_rule_overrides_customer_subtype_uniq",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("customer product rule schema missing marker %q", want)
		}
	}
}

func TestProductConfigTemplateSchemaPersistsIndependentTemplates(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		src  string
		want string
	}{
		{name: "schema table", src: string(schema), want: "CREATE TABLE IF NOT EXISTS %[1]s.product_config_templates"},
		{name: "category binding column", src: string(schema), want: "ALTER TABLE %[1]s.product_categories ADD COLUMN IF NOT EXISTS product_config_template_id BIGINT NOT NULL DEFAULT 0"},
		{name: "template customer source uniq", src: string(schema), want: "product_config_templates_customer_source_active_uniq"},
		{name: "template list method", src: string(repository), want: "func (r Repository) ListProductConfigTemplates"},
		{name: "template save method", src: string(repository), want: "func (r Repository) SaveProductConfigTemplate"},
		{name: "template derive method", src: string(repository), want: "func (r Repository) DeriveProductConfigTemplate"},
		{name: "category select binding", src: string(repository), want: "COALESCE(product_config_template_id,0)"},
		{name: "category save binding", src: string(repository), want: "product_config_template_id=$15"},
		{name: "template materializes category fields", src: string(repository), want: "materializeProductConfigTemplateToCategoriesTx"},
		{name: "derive category clones config", src: string(repository), want: "deriveProductConfigTemplateTx"},
	} {
		if !strings.Contains(tc.src, tc.want) {
			t.Fatalf("product config template persistence missing %s marker %q", tc.name, tc.want)
		}
	}
}

func TestProductConfigSpecialAttrsPersistAndCopyIdempotently(t *testing.T) {
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
		{name: "product special attrs column", src: string(schema), want: "ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS special_attrs_json JSONB NOT NULL DEFAULT '{}'::jsonb"},
		{name: "template special schema column", src: string(schema), want: "ALTER TABLE %[1]s.product_config_templates ADD COLUMN IF NOT EXISTS special_attrs_schema_json JSONB NOT NULL DEFAULT '[]'::jsonb"},
		{name: "legacy roast migration to special attrs", src: string(schema), want: "jsonb_set(COALESCE(special_attrs_json"},
		{name: "product list query reads special attrs", src: string(queries), want: "COALESCE(p.special_attrs_json::text,'{}')"},
		{name: "template list reads special attrs schema", src: string(repository), want: "COALESCE(special_attrs_schema_json::text,'[]')"},
		{name: "product update writes special attrs", src: string(repository), want: "special_attrs_json=$20::jsonb"},
		{name: "template save writes special attrs schema", src: string(repository), want: "special_attrs_schema_json=$13::jsonb"},
		{name: "derived config copies special attrs schema", src: string(repository), want: "source.SpecialAttrsSchemaJSON"},
		{name: "derived config returns existing copy", src: string(repository), want: "findProductConfigTemplateBySourceTx"},
	} {
		if !strings.Contains(tc.src, tc.want) {
			t.Fatalf("product config special attrs persistence missing %s marker %q", tc.name, tc.want)
		}
	}
}

func TestLegacyProductKindMigrationBackfillsDefaultProductTypeSubtypes(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(schema)
	for _, want := range []string{
		"name='熟豆'",
		"name='生豆'",
		"name='挂耳'",
		"name='速溶咖啡'",
		"name='默认熟豆'",
		"name='默认生豆'",
		"name='默认挂耳'",
		"name='默认速溶咖啡'",
		"SET product_category_id = subtype.id",
		"p.product_kind='roasted_bean'",
		"p.product_kind='green_bean'",
		"p.product_kind='drip_bag'",
		"p.product_kind='instant_coffee'",
		"COALESCE(p.product_category_id,0)=0",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("legacy product_kind migration missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"UPDATE %[1]s.order_items",
		"UPDATE %[1]s.bean_list_publications SET content_json",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("legacy product_kind migration must not rewrite historical snapshots or order items; found %q", forbidden)
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

func TestProductNameAndOrderUsageAreExposedForCustomerSkuSorting(t *testing.T) {
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
		{name: "product update writes name", src: string(repository), want: "name=COALESCE(NULLIF($16,''), name)"},
		{name: "audit metadata includes name", src: string(repository), want: `"name":`},
		{name: "product fetch counts order items", src: string(queries), want: "order_usage_count"},
		{name: "product fetch joins order items", src: string(queries), want: "FROM %[1]s.order_items oi"},
	} {
		if !strings.Contains(tc.src, tc.want) {
			t.Fatalf("catalog product name/order usage missing %s marker %q", tc.name, tc.want)
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
		"INSERT INTO %s.product_bom_items(product_id,material_id,component_type,component_product_id,component_spec_g,consume_unit,qty_per_unit,ratio_pct,unit_cost_snapshot,updated_at)",
		"SELECT $1,material_id,component_type,component_product_id,component_spec_g,consume_unit,qty_per_unit,ratio_pct,unit_cost_snapshot,now()",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("custom product BOM copy missing component marker %q", want)
		}
	}
}

func TestCopySKUsRewritesCrossCustomerProductReferences(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(repository)
	for _, want := range []string{
		"type skuCopyPlan struct",
		"sourceToTarget := map[int64]int64{}",
		"resolveSKUCopyProductReferenceTx",
		"updateCopiedSKUGreenBeanReferenceTx",
		"componentType == \"finished_product\"",
		"componentProductID, err = resolveSKUCopyProductReferenceTx",
		"SELECT id, name, COALESCE(customer_id,0), COALESCE(product_category_id,0)",
		"belongs to customer",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("SKU copy product reference rewrite missing marker %q", want)
		}
	}
	copyBOM := catalogRepositoryFunctionForTest(t, src, "func copyProductBOMTx", "func copyProductPriceTiersTx")
	if strings.Contains(copyBOM, "SELECT $1,material_id,component_type,component_product_id") {
		t.Fatalf("copyProductBOMTx must not copy component_product_id verbatim")
	}
}

func TestCopySKUsUsesBomInheritanceInsteadOfCopyingBomItems(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	fn := catalogRepositoryFunctionForTest(t, string(repository), "func (r Repository) CopySKUs", "func replaceProductPriceTiersTx")
	for _, want := range []string{
		"setProductBOMSourceToInheritTx",
		`"bom_source_type":    "inherit_current"`,
		`"source_product_id":   plan.source.ID`,
	} {
		if !strings.Contains(fn, want) {
			t.Fatalf("CopySKUs must write inherited BOM source metadata, missing %q", want)
		}
	}
	if strings.Contains(fn, "copyProductBOMTx") {
		t.Fatalf("CopySKUs must not copy BOM items by default; use inherited BOM source instead")
	}
}

func TestListSKUCopyOptionsBuffersRowsBeforeTargetLookups(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	fn := catalogRepositoryFunctionForTest(t, string(repository), "func (r Repository) ListSKUCopyOptions", "func (r Repository) CopySKUs")
	if strings.Contains(fn, "defer rows.Close()") {
		t.Fatalf("ListSKUCopyOptions must close source rows before running target lookups on the same transaction")
	}
	rowsErr := strings.Index(fn, "if err := rows.Err(); err != nil")
	targetLookup := strings.Index(fn, "findEquivalentCategoryForTargetTx")
	if rowsErr < 0 || targetLookup < 0 {
		t.Fatalf("ListSKUCopyOptions missing rows.Err or target lookup markers")
	}
	if targetLookup < rowsErr {
		t.Fatalf("ListSKUCopyOptions runs target lookups before source rows are fully consumed; this can trigger pgx conn busy")
	}
	if !strings.Contains(fn, "sourceOptions := make([]catalogapp.SKUCopyOption, 0)") {
		t.Fatalf("ListSKUCopyOptions should buffer source SKU rows before annotating overwrite state")
	}
}

func TestCopyProductBOMBuffersRowsBeforeReferenceLookups(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	fn := catalogRepositoryFunctionForTest(t, string(repository), "func copyProductBOMTx", "func copyProductPriceTiersTx")
	if strings.Contains(fn, "defer rows.Close()") {
		t.Fatalf("copyProductBOMTx must close source rows before resolving copied product references on the same transaction")
	}
	rowsErr := strings.Index(fn, "if err := rows.Err(); err != nil")
	referenceLookup := strings.Index(fn, "resolveSKUCopyProductReferenceTx")
	if rowsErr < 0 || referenceLookup < 0 {
		t.Fatalf("copyProductBOMTx missing rows.Err or reference lookup markers")
	}
	if referenceLookup < rowsErr {
		t.Fatalf("copyProductBOMTx resolves product references before source rows are fully consumed; this can trigger pgx conn busy")
	}
	if !strings.Contains(fn, "bomItems := make([]skuCopyBOMItem, 0)") {
		t.Fatalf("copyProductBOMTx should buffer BOM item rows before copying them")
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
		`cmd.BaseProductID > 0 && customType != "custom_roast"`,
		"baseProductID = 0",
		"cmd.CopyPriceTiers && baseProductID > 0",
		`"base_product_id":           baseProductID`,
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("custom green product without base product missing marker %q", want)
		}
	}
}

func TestCreateCustomProductAllowsCustomRoastWithoutBaseProduct(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(repository)
	for _, want := range []string{
		`customType := strings.TrimSpace(cmd.CustomType)`,
		`cmd.BaseProductID > 0 && customType != "custom_roast"`,
		`productKind != catalogdomain.ProductKindGreenBean && customType != "custom_roast"`,
		`baseProductID = 0`,
		`copyBOM := cmd.CopyBOM && productKind == catalogdomain.ProductKindRoasted && baseProductID > 0`,
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("custom roast product without base product missing marker %q", want)
		}
	}
}

func catalogRepositoryFunctionForTest(t *testing.T, src string, startMarker string, endMarker string) string {
	t.Helper()
	start := strings.Index(src, startMarker)
	if start < 0 {
		t.Fatalf("repository.go missing function marker %q", startMarker)
	}
	end := strings.Index(src[start+len(startMarker):], endMarker)
	if end < 0 {
		t.Fatalf("repository.go missing next function marker %q", endMarker)
	}
	return src[start : start+len(startMarker)+end]
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

func TestCustomerPublicUsageCleanupKeepsOwnedParentsWithActiveChildren(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(repository)
	for _, want := range []string{
		"FROM %[1]s.product_categories child",
		"child.active=true",
		"child.parent_id=c.id",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("customer public usage cleanup must keep owned parent categories with active children; missing %q", want)
		}
	}
}

func TestDeactivateProductsDisablesActiveBomVersions(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(repository)
	for _, want := range []string{
		"UPDATE %s.bom_versions SET status='disabled'",
		"WHERE product_id = ANY($1) AND status='active'",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("deactivating SKU must disable active BOM versions; missing %q", want)
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

func TestProductCategoryNameUniquenessIgnoresSoftDeletedRows(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(schema)
	for _, want := range []string{
		"DROP INDEX IF EXISTS %[1]s.product_categories_customer_parent_name_uniq",
		"CREATE UNIQUE INDEX product_categories_customer_parent_name_uniq",
		"ON %[1]s.product_categories (customer_id, COALESCE(parent_id,0), lower(name))",
		"WHERE active=true",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("product category name uniqueness must be rebuilt as active-only partial index, missing %q", want)
		}
	}
}

func TestProductCategorySchemaRepairsActiveChildrenWithInactiveParents(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(schema)
	for _, want := range []string{
		"WITH duplicate_source_parents AS",
		"MIN(id) OVER (PARTITION BY customer_id, source_category_id) AS keeper_id",
		"child.parent_id=duplicate_source_parents.id",
		"UPDATE %[1]s.product_categories parent",
		"SET active=true",
		"child.parent_id=parent.id",
		"child.active=true",
		"active_source_parent.source_category_id=parent.source_category_id",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("schema must repair inactive parent categories that still have active children; missing %q", want)
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
