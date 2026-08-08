package catalog

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	catalogapp "orderapp/internal/application/catalog"
)

func TestFetchProductsUsesNeutralLegacyYieldProjection(t *testing.T) {
	queryBytes, err := os.ReadFile("../catalog_queries.go")
	if err != nil {
		t.Fatal(err)
	}
	query := string(queryBytes)
	for _, forbidden := range []string{
		"CASE WHEN ppc.product_id IS NOT NULL THEN 1 - COALESCE(NULLIF(ppc.expected_loss_rate,0), 0)",
		"COALESCE(ppc.expected_loss_rate, GREATEST(0, 1 - COALESCE(pbv.yield_rate, b.yield_rate, 0.8)))",
	} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("current product query must not derive overall yield/loss: %q", forbidden)
		}
	}
	if !strings.Contains(query, "1::float8 AS neutral_yield_rate") || !strings.Contains(query, "0::float8 AS neutral_expected_loss_rate") {
		t.Fatalf("current product query must project neutral compatibility yield/loss values")
	}
}

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

func TestBusinessGroupItemTreeKeepsChildrenWhenParentRowsComeFirst(t *testing.T) {
	items := []catalogapp.BusinessGroupItem{
		{ID: 10, GroupID: 3, ParentID: 0, Name: "大类", Active: true, SortOrder: 10},
		{ID: 11, GroupID: 3, ParentID: 10, Name: "小类", Active: true, SortOrder: 10},
	}

	tree := businessGroupItemTree(items)
	if len(tree) != 1 {
		t.Fatalf("businessGroupItemTree() roots = %d, want 1: %+v", len(tree), tree)
	}
	if len(tree[0].Children) != 1 || tree[0].Children[0].ID != 11 {
		t.Fatalf("businessGroupItemTree() parent children = %+v, want child 11", tree[0].Children)
	}
}

func TestDeleteBusinessGroupItemPhysicallyDeletesTreeAndUncategorizesAssignments(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	body := catalogRepositoryFunctionForTest(t, string(repository), "func (r Repository) DeleteBusinessGroupItem", "func (r Repository) MoveBusinessGroupItem")
	for _, want := range []string{
		"WITH RECURSIVE targets AS",
		"SET group_item_id=0",
		"DELETE FROM %s.business_group_items WHERE id=ANY($1)",
		`"delete_business_group_item"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("DeleteBusinessGroupItem must delete the category tree and move references to uncategorized; missing %q", want)
		}
	}
	if strings.Contains(body, "UPDATE %s.business_group_items SET active=false") {
		t.Fatalf("DeleteBusinessGroupItem must physically delete categories instead of deactivating them")
	}
}

func TestProductMarginOverrideRemainsReadableButProductUpdateDoesNotWrite(t *testing.T) {
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
	} {
		if !strings.Contains(tc.src, tc.want) {
			t.Fatalf("catalog product margin override compatibility missing %s marker %q", tc.name, tc.want)
		}
	}
	for _, banned := range []string{
		"margin_rate_override=$",
		`"margin_rate_override":           cmd.MarginRateOverride`,
	} {
		if strings.Contains(string(repository), banned) {
			t.Fatalf("product basics update should not write legacy margin override marker %q", banned)
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
		{name: "unit template column", src: string(schema), want: "ALTER TABLE %[1]s.product_categories ADD COLUMN IF NOT EXISTS unit_template_id BIGINT NOT NULL DEFAULT 0"},
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

func TestProductsReferenceUnitTemplatesAsPrimaryUOMMasterData(t *testing.T) {
	schemaBytes, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	repositoryBytes, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	queryBytes, err := os.ReadFile("../catalog_queries.go")
	if err != nil {
		t.Fatal(err)
	}
	schema := string(schemaBytes)
	repository := string(repositoryBytes)
	queries := string(queryBytes)
	for _, want := range []string{
		"ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS unit_template_id BIGINT NOT NULL DEFAULT 0",
	} {
		if !strings.Contains(schema, want) {
			t.Fatalf("products must directly reference product_unit_templates; schema missing %q", want)
		}
	}
	for _, want := range []string{
		"product_direct_unit_template",
		"COALESCE(NULLIF(product_direct_unit_template.inventory_unit,''), NULLIF(p.unit_rule_override_json->>'inventory_unit',''",
		"COALESCE(p.unit_template_id,0)",
		"AS unit_rule_source",
	} {
		if !strings.Contains(queries, want) {
			t.Fatalf("catalog product query must expose direct unit-template effective UOM; missing %q", want)
		}
	}
	for _, want := range []string{
		"unit_template_id=$18",
		`"old_unit_template_id"`,
		`"new_unit_template_id"`,
		`"unit_template_id":`,
		`cmd.UnitTemplateID`,
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("product create/update must persist and audit direct unit template references; missing %q", want)
		}
	}
}

func TestSalesSpecTemplatesDriveDerivedSKUsAndParentInventoryUnit(t *testing.T) {
	schemaBytes, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	repositoryBytes, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	queryBytes, err := os.ReadFile("../catalog_queries.go")
	if err != nil {
		t.Fatal(err)
	}
	schema := string(schemaBytes)
	repository := string(repositoryBytes)
	queries := string(queryBytes)
	for _, want := range []string{
		"ALTER TABLE %[1]s.product_unit_templates ADD COLUMN IF NOT EXISTS sales_specs_json JSONB NOT NULL DEFAULT '[]'::jsonb",
		"ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS auto_derived_sku BOOLEAN NOT NULL DEFAULT false",
		"ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS derived_unit_template_id BIGINT NOT NULL DEFAULT 0",
		"ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS derived_spec_key TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS derived_sales_unit TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS derived_spec_status TEXT NOT NULL DEFAULT ''",
	} {
		if !strings.Contains(schema, want) {
			t.Fatalf("sales spec derived SKU schema missing %q", want)
		}
	}
	for _, want := range []string{
		"syncDerivedSKUsForParentTx(ctx, tx, r.schema, cmd.Actor, cmd.ProductID)",
		"syncDerivedSKUsForParentTx(ctx, tx, r.schema, cmd.Actor, productID)",
		"syncDerivedSKUsForTemplateTx(ctx, tx, r.schema, cmd.Actor, id)",
		"WHERE auto_derived_sku=true AND derived_unit_template_id=$1 AND derived_spec_status<>'template_removed'",
		"derived_spec_status",
		"template_removed",
		"template_disabled",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("sales spec derived SKU repository behavior missing %q", want)
		}
	}
	for _, want := range []string{
		"parent_product_direct_unit_template",
		"CASE WHEN COALESCE(p.parent_product_id,0)>0 THEN parent_units.parent_product_inventory_unit",
		"COALESCE(NULLIF(parent_product_direct_unit_template.inventory_unit,''), NULLIF(parent_product.unit_rule_override_json->>'inventory_unit',''",
		"COALESCE(NULLIF(p.derived_sales_unit,''),",
		"COALESCE(p.auto_derived_sku,false) AS auto_derived_sku",
		"COALESCE(p.derived_spec_key,'') AS derived_spec_key",
	} {
		if !strings.Contains(queries, want) {
			t.Fatalf("catalog product query must expose derived SKU metadata and parent inventory unit; missing %q", want)
		}
	}
}

func TestCreateSKUSyncsDerivedSpecsForTopLevelProduct(t *testing.T) {
	repositoryBytes, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	createSKUFn := catalogRepositoryFunctionForTest(t, string(repositoryBytes), "func (r Repository) CreateSKU", "type derivedSKUParent")
	for _, want := range []string{
		"if cmd.ParentProductID == 0",
		"syncDerivedSKUsForParentTx(ctx, tx, r.schema, cmd.Actor, productID)",
	} {
		if !strings.Contains(createSKUFn, want) {
			t.Fatalf("CreateSKU must derive child SKUs for top-level product archives; missing %q", want)
		}
	}
}

func TestProductCategoriesSchemaBackfillsActiveForLegacyTables(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"ALTER TABLE %[1]s.product_categories ADD COLUMN IF NOT EXISTS active BOOLEAN NOT NULL DEFAULT true",
		"ALTER TABLE %[1]s.production_bom_group_categories ADD COLUMN IF NOT EXISTS active BOOLEAN NOT NULL DEFAULT true",
	} {
		if !strings.Contains(string(schema), want) {
			t.Fatalf("legacy classification/group tables must backfill active columns before PR-442 migration; missing %q", want)
		}
	}
}

func TestWarehouseBusinessGroupMigrationUsesStaticItemRows(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"item_rows AS (",
		"UNION ALL SELECT '客户仓库', 'customer_warehouses', 20",
		"UNION ALL SELECT '损耗/报废', 'loss_scrap_warehouses', 30",
		"CROSS JOIN item_rows ir",
	} {
		if !strings.Contains(string(schema), want) {
			t.Fatalf("warehouse migration must create static group item rows before assignment; missing %q", want)
		}
	}
	if strings.Contains(string(schema), "GROUP BY tg.id, 1, 2, 3, 4, 5, 6") {
		t.Fatalf("warehouse migration must not group by SELECT ordinals from warehouses; it fails on w.kind")
	}
}

func TestProductConfigOverridesRemainReadableButProductUpdateOnlyWritesUnitRule(t *testing.T) {
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
		{name: "product fetch reads overrides", src: string(repository), want: "unit_rule_override_json::text"},
	} {
		if !strings.Contains(tc.src, tc.want) {
			t.Fatalf("product config override compatibility missing %s marker %q", tc.name, tc.want)
		}
	}
	for _, banned := range []string{
		"gradient_template_id_override=$",
		"operation_template_id_override=$",
	} {
		if strings.Contains(string(repository), banned) {
			t.Fatalf("product basics update should not write legacy config override marker %q", banned)
		}
	}
	updateFn := catalogRepositoryFunctionForTest(t, string(repository), "func (r Repository) UpdateProductBasics", "func (r Repository) DeactivateProducts")
	for _, want := range []string{
		"unit_rule_override_json=$",
		"old_inventory_unit",
		"new_inventory_unit",
		"old_integer_inventory_unit",
		"new_integer_inventory_unit",
		"old_default_sales_unit",
		"new_default_sales_unit",
		"old_unit_conversion_json",
		"new_unit_conversion_json",
		"old_sales_unit_rules",
		"new_sales_unit_rules",
	} {
		if !strings.Contains(updateFn, want) {
			t.Fatalf("product basics update must persist and audit product inventory unit; missing %q", want)
		}
	}
	for _, createMarker := range []string{
		"func (r Repository) CreateProduct",
		"func (r Repository) CreateSKU",
	} {
		createFn := catalogRepositoryFunctionForTest(t, string(repository), createMarker, "if err := tx.Commit")
		for _, want := range []string{
			`"default_sales_unit"`,
			`"unit_conversion_json"`,
			`"sales_unit_rules"`,
		} {
			if !strings.Contains(createFn, want) {
				t.Fatalf("%s audit must include product sales unit master data; missing %q", createMarker, want)
			}
		}
	}
}

func TestProductConfigTemplateDoesNotBackfillFromCategoryOnStartup(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{
		"SET product_config_template_id=COALESCE(pc.product_config_template_id,0)",
		"COALESCE(p.product_category_id,0)=pc.id",
		"COALESCE(pc.product_config_template_id,0)>0",
	} {
		if strings.Contains(string(schema), banned) {
			t.Fatalf("startup schema must not backfill product template state from categories; found %q", banned)
		}
	}
}

func TestBusinessGroupAssignmentsSupportStringObjectRefsAndAudit(t *testing.T) {
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
		"ALTER TABLE %[1]s.business_group_assignments ADD COLUMN IF NOT EXISTS object_ref TEXT NOT NULL DEFAULT ''",
		"business_group_assignments_object_ref_idx",
		"lower(object_ref)",
		"func (r Repository) ListBusinessGroupAssignments",
		"func (r Repository) SaveBusinessGroupAssignment",
		"func (r Repository) DeleteBusinessGroupAssignment",
		"business group is not referenced by active usage",
		"object_ref",
		"save_business_group_assignment",
		"delete_business_group_assignment",
		"usage_key",
		"object_key",
		"warehouse_inventory",
	} {
		if !strings.Contains(combined, want) {
			t.Fatalf("business group assignment implementation missing marker %q", want)
		}
	}
}

func TestBusinessGroupAssignmentsKeepOneCurrentAssignmentAcrossBootstrap(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(schema)
	for _, want := range []string{
		"ON %[1]s.business_group_assignments(lower(usage_key), lower(object_key), object_id, lower(object_ref));",
		"ranked_business_group_assignments AS",
		"PARTITION BY lower(bga.usage_key), lower(bga.object_key), bga.object_id, lower(bga.object_ref)",
		"COALESCE(bg.code,'') LIKE 'default_%%'",
		"NOT EXISTS (",
		"lower(existing.usage_key)='product_catalog'",
		"lower(existing.object_key)='product'",
		"existing.object_id=p.id",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("business group assignments must stay single-current across schema bootstrap; missing %q", want)
		}
	}
	if strings.Contains(src, "ON %[1]s.business_group_assignments(group_id, lower(usage_key), lower(object_key), object_id, lower(object_ref));") {
		t.Fatalf("business group assignment unique key must not include group_id; deploy bootstrap can otherwise recreate legacy default assignments beside user moves")
	}
}

func TestLegacyBusinessGroupUsageMigrationsDoNotReactivateExplicitlyDisabledUsages(t *testing.T) {
	catalogSchema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	stockSchema, err := os.ReadFile("../stock/schema.go")
	if err != nil {
		t.Fatal(err)
	}
	assertHistoryGuard := func(name, src, usageKey string) {
		t.Helper()
		marker := "WHERE existing.group_id=tg.id AND lower(existing.usage_key)='" + usageKey + "'"
		index := strings.Index(src, marker)
		if index < 0 {
			t.Fatalf("%s migration must check active and inactive usage history before initializing %s", name, usageKey)
		}
		start := index - 180
		if start < 0 {
			start = 0
		}
		guard := src[start : index+len(marker)]
		if !strings.Contains(guard, "WHERE NOT EXISTS (") {
			t.Fatalf("%s migration history check for %s must be inside WHERE NOT EXISTS", name, usageKey)
		}
		if strings.Contains(guard, "existing.active") {
			t.Fatalf("%s migration must treat inactive %s rows as explicit unbind history", name, usageKey)
		}
	}
	for _, usageKey := range []string{"product_catalog", "production_bom", "warehouse_inventory", "material_catalog"} {
		assertHistoryGuard("catalog", string(catalogSchema), usageKey)
	}
	assertHistoryGuard("stock", string(stockSchema), "warehouse_inventory")
}

func TestMaterialClassificationMigratesToBusinessGroupAssignments(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(schema)
	for _, want := range []string{
		"migrateMaterialClassificationsToBusinessGroups",
		"material_catalog_migrated",
		"material_catalog",
		"物料档案归组",
		"material_classification_groups",
		"material_classification_group_categories",
		"material_classification_assignments",
		"'material'",
		"legacy_material_classification_group_",
		"legacy_material_classification_category_",
		"lower(existing.usage_key)='material_catalog'",
		"lower(existing.object_key)='material'",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("material classifications must migrate to generic business groups; missing %q", want)
		}
	}
}

func TestProductWritesUseBusinessGroupAssignmentsInsteadOfLegacyCategoryColumns(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(repository)
	for _, want := range []string{
		"LEFT JOIN %[1]s.business_group_assignments bga",
		"lower(bga.usage_key)='product_catalog'",
		"lower(bga.object_key)='product'",
		"func (r Repository) SaveBusinessGroupAssignment",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("product repository must use generic business group assignments; missing %q", want)
		}
	}
	for _, fn := range []string{"func (r Repository) UpdateProductBasics", "func (r Repository) CreateProduct", "func (r Repository) CreateSKU"} {
		start := strings.Index(src, fn)
		if start == -1 {
			t.Fatalf("cannot locate %s", fn)
		}
		next := strings.Index(src[start+len(fn):], "\nfunc ")
		body := src[start:]
		if next >= 0 {
			body = src[start : start+len(fn)+next]
		}
		for _, forbidden := range []string{
			"product_category_id=$",
			"classification_template_id=$",
			"ProductCategoryID",
			"ClassificationTemplateID",
		} {
			if strings.Contains(body, forbidden) {
				t.Fatalf("%s should not write legacy product category/classification state; found %q", fn, forbidden)
			}
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
		{name: "product update writes special attrs", src: string(repository), want: "special_attrs_json=$16::jsonb"},
		{name: "template save writes special attrs schema", src: string(repository), want: "special_attrs_schema_json=$13::jsonb"},
		{name: "derived config copies special attrs schema", src: string(repository), want: "source.SpecialAttrsSchemaJSON"},
		{name: "derived config returns existing copy", src: string(repository), want: "findProductConfigTemplateBySourceTx"},
	} {
		if !strings.Contains(tc.src, tc.want) {
			t.Fatalf("product config special attrs persistence missing %s marker %q", tc.name, tc.want)
		}
	}
}

func TestProductProductionConfigSchemaBackfillsLegacyBOMAndCleansIndustryFields(t *testing.T) {
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
		"CREATE TABLE IF NOT EXISTS %[1]s.product_production_config_industry_templates",
		"expected_loss_rate NUMERIC(10,4) NOT NULL DEFAULT 0",
		"process_route_id BIGINT NOT NULL DEFAULT 0",
		"show_in_price_list BOOLEAN NOT NULL DEFAULT true",
		"backfillProductProductionConfigs",
		"1 - COALESCE(NULLIF(pbv.yield_rate,0)",
		"ListProductProductionConfigs",
		"SaveProductProductionConfig",
		`"product_production_config"`,
		`"save_product_production_config"`,
		"LEFT JOIN %[1]s.product_production_configs ppc ON ppc.product_id=p.id",
		"1::float8 AS neutral_yield_rate",
		"0::float8 AS neutral_expected_loss_rate",
	} {
		if !strings.Contains(combined, want) {
			t.Fatalf("product production config implementation missing marker %q", want)
		}
	}

	schemaSource := string(schema)
	backfill := catalogRepositoryFunctionForTest(t, schemaSource, "func backfillProductProductionConfigs", "func cleanupProductProductionConfigIndustryFields")
	if !strings.Contains(backfill, "return cleanupProductProductionConfigIndustryFields(ctx, pool, schema)") {
		t.Fatalf("product production config backfill must call industry field cleanup")
	}
	if strings.Contains(backfill, "jsonb_each_text") {
		t.Fatalf("legacy special_attrs_json must not create product industry fields")
	}

	cleanup := catalogSourceFunctionForTest(t, schemaSource, "func cleanupProductProductionConfigIndustryFields")
	for _, want := range []string{
		"DELETE FROM %[1]s.product_production_config_fields",
		"product_production_config_industry_templates",
		"industry_field_templates",
		"industry_field_definitions",
		"to_regclass",
	} {
		if !strings.Contains(cleanup, want) {
			t.Fatalf("product industry field cleanup missing %q", want)
		}
	}
	const exactFieldKeyMatch = "lower(btrim(d.field_key))=lower(COALESCE(NULLIF(btrim(f.template_field_key),''),btrim(f.field_key)))"
	if !strings.Contains(cleanup, exactFieldKeyMatch) {
		t.Fatalf("product industry field cleanup must follow the case-insensitive product field key uniqueness %q", exactFieldKeyMatch)
	}
}

func TestPR584ProductIndustryTemplatesAndBusinessGroupUsageRepositoryContracts(t *testing.T) {
	schemaBytes, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	repositoryBytes, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	schemaSource := string(schemaBytes)
	repositorySource := string(repositoryBytes)
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS %[1]s.product_production_config_industry_templates",
		"PRIMARY KEY(product_id, template_id)",
		"SELECT product_id, industry_field_template_id, 1, 'system-backfill', 'system-backfill'",
		"ON CONFLICT (product_id, template_id) DO NOTHING",
	} {
		if !strings.Contains(schemaSource, want) {
			t.Fatalf("PR-584 product industry template schema missing %q", want)
		}
	}

	normalize := catalogRepositoryFunctionForTest(t, repositorySource, "func normalizeProductProductionConfigFieldsAgainstTemplatesTx", "func (r Repository) ListProductClassificationTemplates")
	for _, want := range []string{
		"WHERE id=ANY($1::bigint[]) AND status='active'",
		"ORDER BY id",
		"FOR SHARE",
		"ORDER BY array_position($1::bigint[], template_id), sort_order, id",
		"if _, exists := defs[key]; exists",
		"definitionOrder++",
		"seenFields",
	} {
		if !strings.Contains(normalize, want) {
			t.Fatalf("PR-584 ordered industry field union missing %q", want)
		}
	}
	if strings.Contains(normalize, "ORDER BY array_position($1::bigint[], id)\n\t\tFOR SHARE") {
		t.Fatal("template locks must use canonical id order rather than request order to avoid deadlocks")
	}

	saveConfig := catalogRepositoryFunctionForTest(t, repositorySource, "func (r Repository) SaveProductProductionConfig", "func normalizeIndustryFieldTemplateIDsForSave")
	for _, want := range []string{
		"oldTemplateIDs",
		"DELETE FROM %s.product_production_config_industry_templates WHERE product_id=$1",
		"industry_field_template_ids",
		"postgresinfra.StrPtr(fmt.Sprint(oldTemplateIDs))",
	} {
		if !strings.Contains(saveConfig, want) {
			t.Fatalf("PR-584 product config persistence/audit missing %q", want)
		}
	}

	saveGroup := catalogRepositoryFunctionForTest(t, repositorySource, "func (r Repository) SaveBusinessGroup", "func businessGroupUsageKeysTx")
	for _, want := range []string{
		"if cmd.Usages != nil",
		"auditField := postgresinfra.StrPtr(\"name\")",
		"auditField = postgresinfra.StrPtr(\"usages\")",
		`"group_id": id`,
		`"usage_keys": newUsageKeys`,
	} {
		if !strings.Contains(saveGroup, want) {
			t.Fatalf("PR-584 business group usage replacement/audit missing %q", want)
		}
	}

	saveAssignment := catalogRepositoryFunctionForTest(t, repositorySource, "func (r Repository) SaveBusinessGroupAssignment", "func (r Repository) DeleteBusinessGroupAssignment")
	if strings.Contains(saveAssignment, "ensureBusinessGroupUsageForAssignmentTx") {
		t.Fatal("assignment save must not implicitly create or reactivate a group usage")
	}
	for _, want := range []string{"active=true", "business group is not referenced by active usage"} {
		if !strings.Contains(saveAssignment, want) {
			t.Fatalf("assignment save must require an explicit active group usage; missing %q", want)
		}
	}
}

func TestCleanupProductProductionConfigIndustryFields(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required for catalog postgres tests")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)

	schema := fmt.Sprintf("test_catalog_industry_cleanup_%d", time.Now().UnixNano())
	firstBootSchema := schema + "_first_boot"
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+firstBootSchema+" CASCADE")
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
	})
	mustExec := func(query string) {
		t.Helper()
		if _, err := pool.Exec(ctx, query); err != nil {
			t.Fatalf("exec catalog cleanup test SQL: %v", err)
		}
	}

	mustExec(fmt.Sprintf(`
CREATE SCHEMA %[1]s;
CREATE TABLE %[1]s.product_production_configs (
	product_id BIGINT PRIMARY KEY,
	industry_field_template_id BIGINT NOT NULL DEFAULT 0
);
CREATE TABLE %[1]s.product_production_config_fields (
	id BIGINT PRIMARY KEY,
	product_id BIGINT NOT NULL,
	field_key TEXT NOT NULL DEFAULT '',
	template_field_key TEXT NOT NULL DEFAULT ''
);
CREATE TABLE %[1]s.product_production_config_industry_templates (
	product_id BIGINT NOT NULL,
	template_id BIGINT NOT NULL,
	sort_order INT NOT NULL DEFAULT 1,
	PRIMARY KEY(product_id, template_id)
);
CREATE TABLE %[1]s.industry_field_templates (
	id BIGINT PRIMARY KEY
);
CREATE TABLE %[1]s.industry_field_definitions (
	id BIGINT PRIMARY KEY,
	template_id BIGINT NOT NULL,
	field_key TEXT NOT NULL DEFAULT ''
);

INSERT INTO %[1]s.product_production_configs(product_id, industry_field_template_id) VALUES
	(2,0),
	(3,30),
	(4,40),
	(5,50),
	(6,60),
	(7,70);
INSERT INTO %[1]s.product_production_config_industry_templates(product_id, template_id, sort_order) VALUES
	(3,30,1),
	(4,40,1),
	(5,50,1),
	(6,60,1),
	(7,70,1);
INSERT INTO %[1]s.industry_field_templates(id) VALUES (40),(50),(60),(70);
INSERT INTO %[1]s.industry_field_definitions(id, template_id, field_key) VALUES
	(501,50,'exact-key'),
	(601,60,'fallback-key'),
	(701,70,'CaseKey');
INSERT INTO %[1]s.product_production_config_fields(id, product_id, field_key, template_field_key) VALUES
	(1,1,'orphan-key','orphan-key'),
	(2,2,'template-zero-key','template-zero-key'),
	(3,3,'missing-template-key','missing-template-key'),
	(4,4,'zero-definition-key','zero-definition-key'),
	(5,5,'ignored-exact-key','exact-key'),
	(6,5,'exact-key','external-key'),
	(7,6,' fallback-key ','   '),
	(8,7,'ignored-case-key','casekey');
`, schema))

	if err := cleanupProductProductionConfigIndustryFields(ctx, pool, schema); err != nil {
		t.Fatalf("cleanupProductProductionConfigIndustryFields: %v", err)
	}
	assertProductProductionConfigFieldIDs(t, ctx, pool, schema, []int64{5, 7, 8})
	if err := cleanupProductProductionConfigIndustryFields(ctx, pool, schema); err != nil {
		t.Fatalf("cleanupProductProductionConfigIndustryFields second run: %v", err)
	}
	assertProductProductionConfigFieldIDs(t, ctx, pool, schema, []int64{5, 7, 8})

	mustExec(fmt.Sprintf(`
CREATE SCHEMA %[1]s;
CREATE TABLE %[1]s.product_production_configs (
	product_id BIGINT PRIMARY KEY,
	industry_field_template_id BIGINT NOT NULL DEFAULT 0
);
CREATE TABLE %[1]s.product_production_config_fields (
	id BIGINT PRIMARY KEY,
	product_id BIGINT NOT NULL,
	field_key TEXT NOT NULL DEFAULT '',
	template_field_key TEXT NOT NULL DEFAULT ''
);
CREATE TABLE %[1]s.product_production_config_industry_templates (
	product_id BIGINT NOT NULL,
	template_id BIGINT NOT NULL,
	sort_order INT NOT NULL DEFAULT 1,
	PRIMARY KEY(product_id, template_id)
);
INSERT INTO %[1]s.product_production_configs(product_id, industry_field_template_id) VALUES (101,99),(102,0);
INSERT INTO %[1]s.product_production_config_industry_templates(product_id, template_id, sort_order) VALUES (101,99,1);
INSERT INTO %[1]s.product_production_config_fields(id, product_id, field_key, template_field_key) VALUES
	(101,101,'first-boot-key','first-boot-key'),
	(102,102,'template-zero-key','template-zero-key');
`, firstBootSchema))
	if err := cleanupProductProductionConfigIndustryFields(ctx, pool, firstBootSchema); err != nil {
		t.Fatalf("cleanup without industry template tables: %v", err)
	}
	assertProductProductionConfigFieldIDs(t, ctx, pool, firstBootSchema, []int64{101})
}

func assertProductProductionConfigFieldIDs(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string, want []int64) {
	t.Helper()
	rows, err := pool.Query(ctx, fmt.Sprintf(`SELECT id FROM %s.product_production_config_fields ORDER BY id`, schema))
	if err != nil {
		t.Fatalf("query product production config field ids: %v", err)
	}
	defer rows.Close()
	got := make([]int64, 0, len(want))
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan product production config field id: %v", err)
		}
		got = append(got, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate product production config field ids: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("product production config field ids = %v, want %v", got, want)
	}
}

func TestProductProductionConfigFieldsRequireIndustryTemplate(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	fn := catalogRepositoryFunctionForTest(t, string(repository), "func normalizeProductProductionConfigFieldsAgainstTemplatesTx", "func (r Repository) ListProductClassificationTemplates")
	if !strings.Contains(fn, "if len(templateIDs) == 0 {\n\t\treturn []catalogapp.ProductProductionConfigField{}, nil\n\t}") {
		t.Fatalf("product production fields must be empty without an industry template")
	}
}

func TestProductProductionConfigLocksIndustryTemplateBeforeReadingDefinitions(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(repository)
	save := catalogRepositoryFunctionForTest(t, src, "func (r Repository) SaveProductProductionConfig", "func normalizeIndustryFieldTemplateIDsForSave")
	normalizeCall := strings.Index(save, "normalizeProductProductionConfigFieldsAgainstTemplatesTx")
	configWrite := strings.Index(save, "INSERT INTO %s.product_production_configs")
	if normalizeCall < 0 || configWrite < 0 || normalizeCall >= configWrite {
		t.Fatal("SaveProductProductionConfig must normalize and lock the template before writing product config")
	}

	fn := catalogRepositoryFunctionForTest(t, src, "func normalizeProductProductionConfigFieldsAgainstTemplatesTx", "func (r Repository) ListProductClassificationTemplates")
	noTemplateReturn := strings.Index(fn, "if len(templateIDs) == 0")
	templateLock := strings.Index(fn, "FROM %s.industry_field_templates")
	definitionRead := strings.Index(fn, "FROM %s.industry_field_definitions")
	if definitionRead < 0 {
		t.Fatal("product production config normalization must read industry field definitions")
	}
	if templateLock < 0 {
		t.Fatal("product production config normalization must take a shared template-row lock")
	}
	if !strings.Contains(fn[templateLock:definitionRead], "FOR SHARE") {
		t.Fatal("product production config normalization must take a shared template-row lock")
	}
	if noTemplateReturn < 0 || noTemplateReturn >= templateLock {
		t.Fatal("no-template product fields must return [] before attempting a template-row lock")
	}
	if templateLock >= definitionRead {
		t.Fatal("product production config normalization must lock the template row before reading definitions")
	}
	lockSQL := fn[templateLock:definitionRead]
	if !strings.Contains(lockSQL, "WHERE id=ANY($1::bigint[])") {
		t.Fatal("product production config normalization must lock only the referenced templates")
	}
	if !strings.Contains(lockSQL, "status='active'") {
		t.Fatal("product production config normalization must accept only active templates")
	}
}

func TestNormalizeProductProductionConfigFieldsWaitsForTemplateUpdate(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required for catalog template-lock tests")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	schema := fmt.Sprintf("test_catalog_template_lock_%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
	})
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE SCHEMA %[1]s;
		CREATE TABLE %[1]s.industry_field_templates (
			id BIGINT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'active'
		);
		CREATE TABLE %[1]s.industry_field_definitions (
			id BIGINT PRIMARY KEY,
			template_id BIGINT NOT NULL,
			field_key TEXT NOT NULL DEFAULT '',
			label TEXT NOT NULL DEFAULT '',
			field_type TEXT NOT NULL DEFAULT 'text',
			unit TEXT NOT NULL DEFAULT '',
			required BOOLEAN NOT NULL DEFAULT false,
			options_json JSONB NOT NULL DEFAULT '[]'::jsonb,
			sort_order INT NOT NULL DEFAULT 0
		);
		INSERT INTO %[1]s.industry_field_templates(id,name,status) VALUES (10,'before','active');
		INSERT INTO %[1]s.industry_field_definitions(id,template_id,field_key,label)
		VALUES (1,10,'old-key','Old field');
	`, schema)); err != nil {
		t.Fatalf("create catalog template-lock schema: %v", err)
	}

	templateTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin template transaction: %v", err)
	}
	defer func() { _ = templateTx.Rollback(context.Background()) }()
	if _, err := templateTx.Exec(ctx, fmt.Sprintf(`UPDATE %s.industry_field_templates SET name='after' WHERE id=10`, schema)); err != nil {
		t.Fatalf("lock template row for update: %v", err)
	}
	if _, err := templateTx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.industry_field_definitions WHERE template_id=10`, schema)); err != nil {
		t.Fatalf("delete old template definition: %v", err)
	}
	if _, err := templateTx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.industry_field_definitions(id,template_id,field_key,label)
		VALUES (2,10,'new-key','New field')
	`, schema)); err != nil {
		t.Fatalf("insert new template definition: %v", err)
	}

	productTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin product transaction: %v", err)
	}
	defer func() { _ = productTx.Rollback(context.Background()) }()
	var productBackendPID int
	if err := productTx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&productBackendPID); err != nil {
		t.Fatalf("load product transaction backend pid: %v", err)
	}
	type normalizationResult struct {
		fields []catalogapp.ProductProductionConfigField
		err    error
	}
	resultCh := make(chan normalizationResult, 1)
	started := make(chan struct{})
	productCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	go func() {
		close(started)
		fields, err := normalizeProductProductionConfigFieldsAgainstTemplatesTx(productCtx, productTx, schema, []int64{10}, []catalogapp.ProductProductionConfigField{{
			FieldKey:         "old-key",
			TemplateFieldKey: "old-key",
			ValueText:        "legacy value",
		}})
		resultCh <- normalizationResult{fields: fields, err: err}
	}()
	<-started
	waitDeadline := time.Now().Add(time.Second)
	for {
		select {
		case result := <-resultCh:
			t.Fatalf("product normalization returned before template commit: fields=%+v err=%v", result.fields, result.err)
		default:
		}
		var waitEventType string
		if err := pool.QueryRow(ctx, `SELECT COALESCE(wait_event_type,'') FROM pg_stat_activity WHERE pid=$1`, productBackendPID).Scan(&waitEventType); err != nil {
			t.Fatalf("read product transaction wait state: %v", err)
		}
		if waitEventType == "Lock" {
			break
		}
		if time.Now().After(waitDeadline) {
			t.Fatal("product normalization did not wait on the template row lock")
		}
		time.Sleep(10 * time.Millisecond)
	}

	if err := templateTx.Commit(ctx); err != nil {
		t.Fatalf("commit template update: %v", err)
	}
	select {
	case result := <-resultCh:
		if result.err == nil || !strings.Contains(result.err.Error(), "field old-key is not defined by selected industry field templates") {
			t.Fatalf("product normalization after template commit err = %v, want old-key rejection", result.err)
		}
	case <-productCtx.Done():
		t.Fatalf("product normalization did not resume after template commit: %v", productCtx.Err())
	}
	if err := productTx.Rollback(ctx); err != nil {
		t.Fatalf("rollback rejected product config: %v", err)
	}

	inactiveTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin inactive template product transaction: %v", err)
	}
	defer func() { _ = inactiveTx.Rollback(context.Background()) }()
	if _, err := inactiveTx.Exec(ctx, fmt.Sprintf(`UPDATE %s.industry_field_templates SET status='inactive' WHERE id=10`, schema)); err != nil {
		t.Fatalf("deactivate industry template: %v", err)
	}
	_, err = normalizeProductProductionConfigFieldsAgainstTemplatesTx(ctx, inactiveTx, schema, []int64{10}, []catalogapp.ProductProductionConfigField{{
		FieldKey:         "new-key",
		TemplateFieldKey: "new-key",
		ValueText:        "current value",
	}})
	if err == nil || !strings.Contains(err.Error(), "industry field template not found") {
		t.Fatalf("normalize referenced inactive template err=%v, want rejection", err)
	}
}

func TestPR584NormalizeProductProductionConfigFieldsUsesOrderedTemplateUnion(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required for catalog template-union tests")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	schema := fmt.Sprintf("test_catalog_pr584_template_union_%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
	})
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE SCHEMA %[1]s;
		CREATE TABLE %[1]s.industry_field_templates (
			id BIGINT PRIMARY KEY,
			status TEXT NOT NULL DEFAULT 'active'
		);
		CREATE TABLE %[1]s.industry_field_definitions (
			id BIGINT PRIMARY KEY,
			template_id BIGINT NOT NULL,
			field_key TEXT NOT NULL DEFAULT '',
			label TEXT NOT NULL DEFAULT '',
			field_type TEXT NOT NULL DEFAULT 'text',
			unit TEXT NOT NULL DEFAULT '',
			required BOOLEAN NOT NULL DEFAULT false,
			options_json JSONB NOT NULL DEFAULT '[]'::jsonb,
			sort_order INT NOT NULL DEFAULT 0
		);
		INSERT INTO %[1]s.industry_field_templates(id,status) VALUES (10,'active'),(20,'active');
		INSERT INTO %[1]s.industry_field_definitions(id,template_id,field_key,label,field_type,unit,required,options_json,sort_order) VALUES
			(1,10,'shared_key','Shared A','text','A',false,'[]',1),
			(2,10,'a_only','A only','number','kg',true,'[]',2),
			(3,20,'SHARED_KEY','Shared B','select','B',true,'["B1"]',1),
			(4,20,'b_only','B only','boolean','',false,'[]',2);
	`, schema)); err != nil {
		t.Fatalf("create template union schema: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin template union transaction: %v", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	fields, err := normalizeProductProductionConfigFieldsAgainstTemplatesTx(ctx, tx, schema, []int64{20, 10}, []catalogapp.ProductProductionConfigField{
		{FieldKey: "shared_key", ValueText: "kept"},
		{FieldKey: "b_only", ValueBool: func() *bool { value := true; return &value }()},
		{FieldKey: "a_only", ValueNumber: func() *float64 { value := 1.25; return &value }()},
		{FieldKey: "SHARED_KEY", ValueText: "duplicate payload"},
	})
	if err != nil {
		t.Fatalf("normalize ordered template union: %v", err)
	}
	if len(fields) != 3 {
		t.Fatalf("normalized fields=%+v, want three unique keys", fields)
	}
	if fields[0].FieldKey != "SHARED_KEY" || fields[0].Label != "Shared B" || fields[0].FieldType != "select" || fields[0].SortOrder != 1 || fields[0].ValueText != "kept" {
		t.Fatalf("first-wins shared field=%+v, want template 20 definition with preserved value", fields[0])
	}
	if fields[1].FieldKey != "b_only" || fields[1].SortOrder != 2 {
		t.Fatalf("template 20 unique field=%+v", fields[1])
	}
	if fields[2].FieldKey != "a_only" || fields[2].SortOrder != 3 {
		t.Fatalf("template 10 union field=%+v", fields[2])
	}
}

func TestProductProductionConfigListInitializesEmptyFields(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	fn := catalogRepositoryFunctionForTest(t, string(repository), "func (r Repository) ListProductProductionConfigs", "func (r Repository) GetProductProductionConfig")
	if !strings.Contains(fn, "row.Fields = []catalogapp.ProductProductionConfigField{}") {
		t.Fatalf("product production config responses must encode empty fields as []")
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
		{name: "product update", src: string(repository), want: "remark=$14"},
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
		{name: "product update writes name", src: string(repository), want: "name=COALESCE(NULLIF($15,''), name)"},
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

func TestCopyProductArchiveCopiesOnlyMasterDataNotPriceOrBomTemplates(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(repository)
	fn := catalogRepositoryFunctionForTest(t, src, "func (r Repository) CopyProduct", "func fetchProductForCopyTx")
	for _, want := range []string{
		"nextProductArchiveCopyNameTx",
		"copy_product_archive",
		"unit_template_id",
		"unit_rule_override_json",
	} {
		if !strings.Contains(fn, want) {
			t.Fatalf("CopyProduct must copy product master data; missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"product_config_template_id",
		"classification_template_id",
		"product_production_config_fields",
		"product_price_tiers",
		"product_production_configs",
		"product_production_bom_bindings",
		"margin_rate_override",
		"gradient_template_id_override",
		"operation_template_id_override",
	} {
		if strings.Contains(fn, forbidden) {
			t.Fatalf("CopyProduct must not copy legacy template, price, or BOM state; found %q", forbidden)
		}
	}
}

func TestUpdateProductBasicsDoesNotWriteLegacyTemplateColumns(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(repository)
	fn := catalogRepositoryFunctionForTest(t, src, "func (r Repository) UpdateProductBasics", "func (r Repository) DeactivateProducts")
	for _, forbidden := range []string{
		"product_config_template_id=$",
		"classification_template_id=$",
		"cmd.ProductConfigTemplateID",
		"cmd.ClassificationTemplateID",
	} {
		if strings.Contains(fn, forbidden) {
			t.Fatalf("UpdateProductBasics must not write legacy template columns; found %q", forbidden)
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

func TestSyncDerivedSKUsForTemplateBuffersParentIDsBeforeNestedQueries(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	fn := catalogRepositoryFunctionForTest(t, string(repository), "func syncDerivedSKUsForTemplateTx", "func syncDerivedSKUsForParentTx")
	if strings.Contains(fn, "defer rows.Close()") {
		t.Fatalf("syncDerivedSKUsForTemplateTx must close parent rows before nested queries on the same transaction")
	}
	rowsErr := strings.Index(fn, "if err := rows.Err(); err != nil")
	nestedQuery := strings.Index(fn, "syncDerivedSKUsForParentTx")
	if rowsErr < 0 || nestedQuery < 0 {
		t.Fatalf("syncDerivedSKUsForTemplateTx missing rows.Err or nested sync markers")
	}
	if nestedQuery < rowsErr {
		t.Fatalf("syncDerivedSKUsForTemplateTx runs nested queries before parent rows are fully consumed; this can trigger pgx conn busy")
	}
	if !strings.Contains(fn, "parentIDs := make([]int64, 0)") {
		t.Fatalf("syncDerivedSKUsForTemplateTx should buffer parent ids before syncing derived SKUs")
	}
}

func TestSyncDerivedSKUsForParentBuffersChildrenBeforeStatusUpdates(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	fn := catalogRepositoryFunctionForTest(t, string(repository), "func syncDerivedSKUsForParentTx", "func upsertDerivedSKUForSpecTx")
	childQuery := strings.Index(fn, "SELECT id, COALESCE(derived_spec_key,'')")
	if childQuery < 0 {
		t.Fatal("syncDerivedSKUsForParentTx missing child SKU query")
	}
	childSync := fn[childQuery:]
	if strings.Contains(childSync, "defer rows.Close()") {
		t.Fatalf("syncDerivedSKUsForParentTx must close child rows before status updates on the same transaction")
	}
	rowsErr := strings.Index(childSync, "if err := rows.Err(); err != nil")
	statusUpdate := strings.Index(childSync, "SET derived_spec_status=$2")
	if rowsErr < 0 || statusUpdate < 0 {
		t.Fatalf("syncDerivedSKUsForParentTx missing rows.Err or status update markers")
	}
	if statusUpdate < rowsErr {
		t.Fatalf("syncDerivedSKUsForParentTx updates child status before child rows are fully consumed; this can trigger pgx conn busy")
	}
	if !strings.Contains(childSync, "childStatuses := make([]derivedSKUChildStatus, 0)") {
		t.Fatalf("syncDerivedSKUsForParentTx should buffer child status updates before executing them")
	}
}

func TestSaveProductUnitTemplateSyncsDerivedSKUsWithoutBusyConnection(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required for catalog postgres tests")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)

	schema := fmt.Sprintf("test_catalog_sales_spec_sync_%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
	})
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create catalog sales-spec schema: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
CREATE TABLE %s.product_unit_templates (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL,
	inventory_unit TEXT NOT NULL DEFAULT 'kg',
	quote_unit TEXT NOT NULL DEFAULT 'kg',
	order_unit TEXT NOT NULL DEFAULT 'kg',
	unit_conversion_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	sales_specs_json JSONB NOT NULL DEFAULT '[]'::jsonb,
	integer_unit BOOLEAN NOT NULL DEFAULT false,
	active BOOLEAN NOT NULL DEFAULT true,
	deleted_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.products (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL DEFAULT '',
	remark TEXT NOT NULL DEFAULT '',
	product_kind TEXT NOT NULL DEFAULT 'roasted_bean',
	roast_level TEXT NOT NULL DEFAULT '',
	default_price NUMERIC NOT NULL DEFAULT 0,
	active BOOLEAN NOT NULL DEFAULT true,
	retail_price_100g NUMERIC NOT NULL DEFAULT 0,
	retail_price_200g NUMERIC NOT NULL DEFAULT 0,
	retail_price_227g NUMERIC NOT NULL DEFAULT 0,
	retail_price_250g NUMERIC NOT NULL DEFAULT 0,
	drip_bag_grams NUMERIC NOT NULL DEFAULT 10,
	drip_box_bag_count INT NOT NULL DEFAULT 10,
	allow_fulfillment_order BOOLEAN NOT NULL DEFAULT true,
	allow_mall_order BOOLEAN NOT NULL DEFAULT false,
	customer_id BIGINT NOT NULL DEFAULT 0,
	base_product_id BIGINT NOT NULL DEFAULT 0,
	visibility TEXT NOT NULL DEFAULT 'public',
	custom_type TEXT NOT NULL DEFAULT '',
	green_bean_type TEXT NOT NULL DEFAULT '',
	green_bean_bom_product_id BIGINT NOT NULL DEFAULT 0,
	special_attrs_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	unit_rule_override_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	unit_template_id BIGINT NOT NULL DEFAULT 0,
	parent_product_id BIGINT NOT NULL DEFAULT 0,
	sku_name TEXT NOT NULL DEFAULT '',
	sku_code TEXT NOT NULL DEFAULT '',
	barcode TEXT NOT NULL DEFAULT '',
	spec_label TEXT NOT NULL DEFAULT '',
	net_content_qty NUMERIC NOT NULL DEFAULT 0,
	net_content_unit TEXT NOT NULL DEFAULT '',
	is_default_sku BOOLEAN NOT NULL DEFAULT false,
	default_sku_id BIGINT NOT NULL DEFAULT 0,
	product_category_id BIGINT NOT NULL DEFAULT 0,
	auto_derived_sku BOOLEAN NOT NULL DEFAULT false,
	derived_unit_template_id BIGINT NOT NULL DEFAULT 0,
	derived_spec_key TEXT NOT NULL DEFAULT '',
	derived_spec_name TEXT NOT NULL DEFAULT '',
	derived_sales_unit TEXT NOT NULL DEFAULT '',
	derived_spec_status TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.audit_logs (
	id BIGSERIAL PRIMARY KEY,
	ts TIMESTAMPTZ NOT NULL DEFAULT now(),
	actor TEXT NOT NULL DEFAULT '',
	entity_type TEXT NOT NULL DEFAULT '',
	entity_id BIGINT,
	action TEXT NOT NULL DEFAULT '',
	field TEXT,
	old_value TEXT,
	new_value TEXT,
	meta JSONB
)`, schema, schema, schema)); err != nil {
		t.Fatalf("create audit fixture: %v", err)
	}

	active := true
	repository := NewRepository(pool, schema)
	template, err := repository.SaveProductUnitTemplate(ctx, catalogapp.SaveProductUnitTemplateCommand{
		Actor:              "pr550-test",
		Name:               "PR550 销售规格模板",
		InventoryUnit:      "kg",
		QuoteUnit:          "kg",
		OrderUnit:          "kg",
		UnitConversionJSON: "{}",
		Active:             &active,
	})
	if err != nil {
		t.Fatalf("create product unit template: %v", err)
	}

	var parentID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
INSERT INTO %s.products(name, product_kind, unit_template_id, active)
VALUES ('PR550 父商品','roasted_bean',$1,true)
RETURNING id
`, schema), template.ID).Scan(&parentID); err != nil {
		t.Fatalf("create parent product: %v", err)
	}

	specs := []catalogapp.ProductSalesSpec{{
		SpecKey:        "1kg",
		SpecName:       "1Kg",
		SalesUnit:      "1Kg",
		NetContentQty:  1,
		NetContentUnit: "kg",
		Default:        true,
		Active:         true,
	}}
	if _, err := repository.SaveProductUnitTemplate(ctx, catalogapp.SaveProductUnitTemplateCommand{
		Actor:              "pr550-test",
		ID:                 template.ID,
		Name:               template.Name,
		InventoryUnit:      template.InventoryUnit,
		QuoteUnit:          template.QuoteUnit,
		OrderUnit:          template.OrderUnit,
		UnitConversionJSON: "{}",
		SalesSpecs:         specs,
		Active:             &active,
	}); err != nil {
		t.Fatalf("add sales spec must not leave parent rows busy: %v", err)
	}

	var derivedID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
SELECT id
FROM %s.products
WHERE parent_product_id=$1 AND auto_derived_sku=true
  AND derived_unit_template_id=$2 AND derived_spec_key='1kg'
`, schema), parentID, template.ID).Scan(&derivedID); err != nil {
		t.Fatalf("load derived SKU: %v", err)
	}
	var defaultSKUID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT default_sku_id FROM %s.products WHERE id=$1`, schema), parentID).Scan(&defaultSKUID); err != nil {
		t.Fatalf("load parent default SKU: %v", err)
	}
	if defaultSKUID != derivedID {
		t.Fatalf("parent default SKU = %d, want derived SKU %d", defaultSKUID, derivedID)
	}

	var staleID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
INSERT INTO %s.products(
	name, product_kind, parent_product_id, unit_template_id, auto_derived_sku,
	derived_unit_template_id, derived_spec_key, derived_spec_status, active
)
VALUES ('PR550 历史规格','roasted_bean',$1,$2,true,$2,'removed-spec','active',true)
RETURNING id
`, schema), parentID, template.ID).Scan(&staleID); err != nil {
		t.Fatalf("create stale derived SKU: %v", err)
	}
	if _, err := repository.SaveProductUnitTemplate(ctx, catalogapp.SaveProductUnitTemplateCommand{
		Actor:              "pr550-test",
		ID:                 template.ID,
		Name:               template.Name,
		InventoryUnit:      template.InventoryUnit,
		QuoteUnit:          template.QuoteUnit,
		OrderUnit:          template.OrderUnit,
		UnitConversionJSON: "{}",
		SalesSpecs:         specs,
		Active:             &active,
	}); err != nil {
		t.Fatalf("resave sales spec must not update child status while rows are busy: %v", err)
	}
	var staleStatus string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT derived_spec_status FROM %s.products WHERE id=$1`, schema), staleID).Scan(&staleStatus); err != nil {
		t.Fatalf("load stale derived SKU status: %v", err)
	}
	if staleStatus != "template_removed" {
		t.Fatalf("stale derived SKU status = %q, want template_removed", staleStatus)
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

func catalogSourceFunctionForTest(t *testing.T, src string, startMarker string) string {
	t.Helper()
	start := strings.Index(src, startMarker)
	if start < 0 {
		t.Fatalf("source missing function marker %q", startMarker)
	}
	remainder := src[start+len(startMarker):]
	if end := strings.Index(remainder, "\nfunc "); end >= 0 {
		return src[start : start+len(startMarker)+end]
	}
	return src[start:]
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

func TestProductUnitDeletesSoftDisableAndAudit(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(repository)
	for _, want := range []string{
		"DeleteProductUnitDefinition",
		"DeleteProductUnitTemplate",
		"UPDATE %s.product_unit_definitions",
		"UPDATE %s.product_unit_templates",
		"active=false",
		`"delete_product_unit_definition"`,
		`"delete_product_unit_template"`,
		"AuditInsertTx(ctx, tx, r.schema, cmd.Actor, \"product_unit_definition\"",
		"AuditInsertTx(ctx, tx, r.schema, cmd.Actor, \"product_unit_template\"",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("product unit delete soft-disable/audit implementation missing marker %q", want)
		}
	}
}

func TestProductUnitTemplateInventoryUnitIsLockedAfterCreate(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(repository)
	for _, want := range []string{
		"assertProductUnitTemplateInventoryUnitUnchanged",
		"库存单位保存后不能修改",
		"SELECT COALESCE(NULLIF(inventory_unit,''),'kg')",
		"product_unit_template_inventory_unit_locked",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("product unit template inventory unit lock missing marker %q", want)
		}
	}
}

func TestTemplateDeletesUseDeletedStateAndHideFromLists(t *testing.T) {
	schemaBytes, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	repositoryBytes, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	schema := string(schemaBytes)
	repository := string(repositoryBytes)
	for _, want := range []string{
		"ALTER TABLE %[1]s.product_unit_definitions ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ",
		"ALTER TABLE %[1]s.product_unit_templates ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ",
		"ALTER TABLE %[1]s.product_classification_templates ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ",
		"ALTER TABLE %[1]s.product_config_templates ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ",
	} {
		if !strings.Contains(schema, want) {
			t.Fatalf("schema missing deleted-state column marker %q", want)
		}
	}
	for _, want := range []string{
		"WHERE deleted_at IS NULL",
		"SET active=false, deleted_at=now(), updated_at=now()",
		"func (r Repository) DeleteProductConfigTemplate",
		"delete_product_config_template",
		"delete_product_classification_template",
		"delete_product_unit_definition",
		"delete_product_unit_template",
		"deleted_at=NULL",
		"cmd.Active != nil && !*cmd.Active",
		"unit_template_inactive_skipped_for_deactivate",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("repository missing delete-state/deactivate marker %q", want)
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

func TestProductPriceMasterSchemaPersistsFinalRecordsAndReferenceSchemes(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	schemaSrc := string(schema)
	repositorySrc := string(repository)
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS %[1]s.product_price_groups",
		"CREATE TABLE IF NOT EXISTS %[1]s.product_price_records",
		"final_unit_price NUMERIC(14,4) NOT NULL",
		"price_unit TEXT NOT NULL",
		"inventory_conversion_json JSONB NOT NULL DEFAULT '{}'::jsonb",
		"CREATE TABLE IF NOT EXISTS %[1]s.product_tier_price_schemes",
		"CREATE TABLE IF NOT EXISTS %[1]s.product_tier_price_scheme_tiers",
		"source_price_record_id BIGINT NOT NULL",
	} {
		if !strings.Contains(schemaSrc, want) {
			t.Fatalf("product price master schema missing marker %q", want)
		}
	}
	for _, want := range []string{
		"func (r Repository) SaveProductPriceRecord",
		"save_product_price_record",
		"func (r Repository) SaveProductTierPriceScheme",
		"save_product_tier_price_scheme",
		"source_price_record_id, final_unit_price, price_unit, currency",
	} {
		if !strings.Contains(repositorySrc, want) {
			t.Fatalf("product price master repository missing marker %q", want)
		}
	}
	schemeTableStart := strings.Index(schemaSrc, "CREATE TABLE IF NOT EXISTS %[1]s.product_tier_price_scheme_tiers")
	schemeTableEnd := strings.Index(schemaSrc[schemeTableStart:], ");")
	if schemeTableStart < 0 || schemeTableEnd < 0 {
		t.Fatalf("product tier scheme table definition not found")
	}
	schemeTable := schemaSrc[schemeTableStart : schemeTableStart+schemeTableEnd]
	if strings.Contains(schemeTable, "margin_rate") || strings.Contains(schemeTable, "cost_plus") {
		t.Fatalf("tier price scheme tiers must reference final price records, not store calculation fields: %s", schemeTable)
	}
}

func TestPricingRuleMarkupOnlyMigrationIsSafeAndIdempotent(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(schema)
	start := strings.Index(src, "-- PR-539 pricing rules use markup only.")
	end := strings.Index(src, "-- PR-539 pricing rules use markup only end.")
	if start < 0 || end <= start {
		t.Fatalf("PR-539 pricing-rule migration block missing")
	}
	block := src[start:end]
	for _, want := range []string{
		"margin_rate / 100",
		"jsonb_typeof(calculation_json)",
		"'profit_method', 'markup'",
		"'legacy_profit_method'",
		"'legacy_margin_rate'",
		"'system-pr539-migration'",
		"active=false",
	} {
		if !strings.Contains(block, want) {
			t.Fatalf("PR-539 migration missing %q: %s", want, block)
		}
	}
	for _, forbidden := range []string{"bean_list_publications", "order_items", "orders", "final_unit_price"} {
		if strings.Contains(block, forbidden) {
			t.Fatalf("PR-539 migration must not rewrite frozen prices; found %q in %s", forbidden, block)
		}
	}
}

func TestProductSettingsRepositoryAttachesPublishedPriceSummaries(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(repository)
	for _, want := range []string{
		"attachProductPriceSummaries",
		"attachCustomerProductAliasPriceSummaries",
		"loadPublishedPriceSummaries",
		"bean_list_publications",
		"commercial_wholesale_tiers",
		"source_price_record_id",
		"price_table_version",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("product settings repository price summary missing marker %q", want)
		}
	}
}

func TestPerProductDefaultSKUSchemaAndRepositoryContract(t *testing.T) {
	schemaBytes, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	repositoryBytes, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	queryBytes, err := os.ReadFile("../catalog_queries.go")
	if err != nil {
		t.Fatal(err)
	}
	schema := string(schemaBytes)
	repository := string(repositoryBytes)
	queries := string(queryBytes)
	for _, want := range []string{
		"default_sku_id BIGINT NOT NULL DEFAULT 0",
		"backfillProductDefaultSKUs(ctx, pool, schema)",
		"func backfillProductDefaultSKUs",
		"IN ('', 'active')",
		"COALESCE(legacy_flags.legacy_default_count,0) = 1",
		"COALESCE(current_child.id, unique_legacy_child.id, template_default_child.id, first_valid_child.id, parent.id)",
		"COUNT(*) FILTER (WHERE child.is_default_sku=true) AS legacy_default_count",
		"derived_spec_key",
		"UPDATE %[1]s.products child",
	} {
		if !strings.Contains(schema, want) {
			t.Fatalf("default SKU migration missing marker %q", want)
		}
	}
	if strings.Contains(schema, "ORDER BY child.is_default_sku DESC") {
		t.Fatalf("multiple legacy default flags must not select an arbitrary child ahead of the template default")
	}
	for _, want := range []string{
		"func (r Repository) SetProductDefaultSKU",
		"setProductDefaultSKUTx",
		"reconcileProductDefaultSKUTx",
		"derived_spec_status",
		"template_removed",
		"sku does not belong to parent product",
		`"set_default_sku"`,
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("default SKU repository missing marker %q", want)
		}
	}
	if strings.Contains(repository, "netContentUnit, spec.Default, parent.CategoryID") {
		t.Fatalf("derived SKU upsert must not copy template-global default into per-product compatibility flag")
	}
	for _, want := range []string{"default_sku_id", "effective_default_sku_id", "default_spec_label"} {
		if !strings.Contains(queries, want) {
			t.Fatalf("catalog projection missing %q", want)
		}
	}
}

func TestDeactivateProductsReconcilesAffectedDefaultSKUParents(t *testing.T) {
	repositoryBytes, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	repository := string(repositoryBytes)
	start := strings.Index(repository, "func (r Repository) DeactivateProducts")
	end := strings.Index(repository[start:], "func (r Repository) CreateProduct")
	if start < 0 || end < 0 {
		t.Fatal("DeactivateProducts implementation not found")
	}
	deactivate := repository[start : start+end]
	for _, want := range []string{
		"parent_product_id",
		"reconcileProductDefaultSKUTx(ctx, tx, r.schema, cmd.Actor, parentID)",
	} {
		if !strings.Contains(deactivate, want) {
			t.Fatalf("deactivating a SKU must reconcile its parent default; missing %q", want)
		}
	}
}
