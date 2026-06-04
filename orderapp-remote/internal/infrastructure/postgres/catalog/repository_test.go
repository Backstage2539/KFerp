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

func TestCustomerProductAliasSchemaPersistsCustomerFacingNamesAndAudits(t *testing.T) {
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
		{name: "alias table", src: string(schema), want: "CREATE TABLE IF NOT EXISTS %[1]s.customer_product_aliases"},
		{name: "customer id", src: string(schema), want: "customer_id BIGINT NOT NULL"},
		{name: "product id", src: string(schema), want: "product_id BIGINT NOT NULL"},
		{name: "display name", src: string(schema), want: "display_name TEXT NOT NULL"},
		{name: "customer code", src: string(schema), want: "customer_item_code TEXT NOT NULL DEFAULT ''"},
		{name: "alias product config template", src: string(schema), want: "product_config_template_id BIGINT NOT NULL DEFAULT 0"},
		{name: "alias gradient template", src: string(schema), want: "gradient_template_id BIGINT NOT NULL DEFAULT 0"},
		{name: "alias unit template", src: string(schema), want: "unit_template_id BIGINT NOT NULL DEFAULT 0"},
		{name: "price list flag", src: string(schema), want: "include_in_price_list BOOLEAN NOT NULL DEFAULT true"},
		{name: "factory self seed", src: string(schema), want: "工厂自营"},
		{name: "list method", src: string(repository), want: "func (r Repository) ListCustomerProductAliases"},
		{name: "save method", src: string(repository), want: "func (r Repository) SaveCustomerProductAlias"},
		{name: "disable method", src: string(repository), want: "func (r Repository) DisableCustomerProductAlias"},
		{name: "factory customer method", src: string(repository), want: "func (r Repository) EnsureFactoryCustomer"},
		{name: "audit entity", src: string(repository), want: `"customer_product_alias"`},
		{name: "audit product config template", src: string(repository), want: `"product_config_template_id": cmd.ProductConfigTemplateID`},
		{name: "audit pricing template", src: string(repository), want: `"gradient_template_id":       cmd.GradientTemplateID`},
		{name: "audit unit template", src: string(repository), want: `"unit_template_id":           cmd.UnitTemplateID`},
		{name: "create audit", src: string(repository), want: `"create_customer_product_alias"`},
		{name: "update audit", src: string(repository), want: `"update_customer_product_alias"`},
		{name: "disable audit", src: string(repository), want: `"disable_customer_product_alias"`},
	} {
		if !strings.Contains(tc.src, tc.want) {
			t.Fatalf("customer product alias persistence missing %s marker %q", tc.name, tc.want)
		}
	}
}

func TestCustomerProductAliasBatchDisableAndIndustryFieldOverridesPersist(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	combined := string(schema) + "\n" + string(repository)
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS %[1]s.customer_product_alias_industry_field_values",
		"customer_product_alias_industry_field_values_alias_key_uq",
		"func (r Repository) BatchDisableCustomerProductAliases",
		"func (r Repository) ListCustomerProductAliasIndustryFields",
		"func (r Repository) SaveCustomerProductAliasIndustryFields",
		"customer_product_alias_industry_fields",
		"save_customer_product_alias_industry_fields",
		"disable_customer_product_alias",
	} {
		if !strings.Contains(combined, want) {
			t.Fatalf("customer product alias batch/industry field implementation missing marker %q", want)
		}
	}
}

func TestProductClassificationTemplatesPersistProductConfigTemplateReferences(t *testing.T) {
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
		{name: "classification template product config column", src: string(schema), want: "product_config_template_id BIGINT NOT NULL DEFAULT 0"},
		{name: "classification template product config migration", src: string(schema), want: "ALTER TABLE %[1]s.product_classification_templates ADD COLUMN IF NOT EXISTS product_config_template_id BIGINT NOT NULL DEFAULT 0"},
		{name: "classification category product config migration", src: string(schema), want: "ALTER TABLE %[1]s.product_classification_template_categories ADD COLUMN IF NOT EXISTS product_config_template_id BIGINT NOT NULL DEFAULT 0"},
		{name: "template select product config", src: string(repository), want: "COALESCE(product_config_template_id,0), COALESCE(gradient_template_id,0), COALESCE(unit_template_id,0)"},
		{name: "category select product config", src: string(repository), want: "COALESCE(product_config_template_id,0), COALESCE(gradient_template_id,0), COALESCE(unit_template_id,0)"},
		{name: "template update product config", src: string(repository), want: "product_config_template_id=$6, gradient_template_id=$7, unit_template_id=$8"},
		{name: "category update product config", src: string(repository), want: "product_config_template_id=$7, gradient_template_id=$8, unit_template_id=$9"},
		{name: "audit product config template", src: string(repository), want: `"product_config_template_id": cmd.ProductConfigTemplateID`},
	} {
		if !strings.Contains(tc.src, tc.want) {
			t.Fatalf("classification template persistence missing %s marker %q", tc.name, tc.want)
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

func TestProductProductionConfigSchemaBackfillsLegacyBOMAndAttributes(t *testing.T) {
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
	combined := string(schema) + "\n" + string(repository) + "\n" + string(queries)
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS %[1]s.product_production_configs",
		"CREATE TABLE IF NOT EXISTS %[1]s.product_production_config_fields",
		"expected_loss_rate NUMERIC(10,4) NOT NULL DEFAULT 0",
		"process_route_id BIGINT NOT NULL DEFAULT 0",
		"show_in_price_list BOOLEAN NOT NULL DEFAULT true",
		"backfillProductProductionConfigs",
		"1 - COALESCE(NULLIF(pbv.yield_rate,0)",
		"jsonb_each_text",
		"ListProductProductionConfigs",
		"SaveProductProductionConfig",
		`"product_production_config"`,
		`"save_product_production_config"`,
		"LEFT JOIN %[1]s.product_production_configs ppc ON ppc.product_id=p.id",
		"COALESCE(ppc.expected_loss_rate",
	} {
		if !strings.Contains(combined, want) {
			t.Fatalf("product production config implementation missing marker %q", want)
		}
	}
}

func TestProductProductionConfigLegacyFieldsCanSaveWithoutIndustryTemplate(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(repository)
	if strings.Contains(src, "industry_field_template_id required for product information fields") {
		t.Fatalf("changing BOM bindings must not fail existing legacy product information fields without a template")
	}
	if !strings.Contains(src, "if templateID <= 0 {\n\t\treturn fields, nil\n\t}") {
		t.Fatalf("legacy product production fields should pass through when no industry template is selected")
	}
}

func TestClassificationCategoryDeleteMovesAssignmentsBackToTemplateUnclassified(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(repository)
	fn := catalogRepositoryFunctionForTest(t, src, "func (r Repository) DeleteProductClassificationCategory", "func (r Repository) SaveProductClassificationAssignment")
	for _, want := range []string{
		"UPDATE",
		"product_classification_assignments",
		"customer_product_alias_classification_assignments",
		"category_id=0",
		"delete_product_classification_category",
	} {
		if !strings.Contains(fn, want) {
			t.Fatalf("classification category delete must move assignments to virtual unclassified; missing %q", want)
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

func TestCopyProductArchiveCopiesConfigurationSnapshots(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(repository)
	fn := catalogRepositoryFunctionForTest(t, src, "func (r Repository) CopyProduct", "func fetchProductForCopyTx")
	for _, want := range []string{
		"nextProductArchiveCopyNameTx",
		"product_config_template_id",
		"product_price_tiers",
		"product_production_configs",
		"product_production_bom_bindings",
		"product_production_config_fields",
		"copy_product_archive",
	} {
		if !strings.Contains(fn, want) {
			t.Fatalf("CopyProduct must copy product archive configuration; missing %q", want)
		}
	}
}

func TestLegacySKUCopyRepositoryCodeIsRemoved(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(repository)
	for _, forbidden := range []string{
		"func (r Repository) CopySKUs",
		"func (r Repository) ListSKUCopyOptions",
		"func copyProductBOMTx",
		"resolveSKUCopyProductReferenceTx",
		"skuCopyPlan",
		"skuCopyBOMItem",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("legacy SKU copy repository code must be removed, found %q", forbidden)
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
