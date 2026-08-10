package bom

import (
	"os"
	"strings"
	"testing"
)

func TestRepositoryDeleteItemScopesByProductAndAuditsActualRow(t *testing.T) {
	src := readRepositorySource(t)
	for _, want := range []string{
		"WHERE id=$1 AND product_id=$2",
		"bom item not found",
		"row.ComponentType",
		"row.ComponentProductID",
		"product_bom_item",
		"AuditInsertTx(ctx, tx",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("repository delete item missing marker %q", want)
		}
	}
}

func TestRepositoryWritesAuditForBomWritePaths(t *testing.T) {
	src := readRepositorySource(t)
	for _, want := range []string{
		`AuditInsert(ctx, r.pool, r.schema, cmd.Actor, "product_bom",`,
		`AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product_bom",`,
		`AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "bom_version",`,
		`AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product_bom_item",`,
		`AuditInsert(ctx, pool, schema, cmd.Actor, "packaging_spec_material_map",`,
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("repository audit coverage missing marker %q", want)
		}
	}
}

func TestBomRepositoryExposesProductKindForOutputAndComponentFiltering(t *testing.T) {
	src := readRepositorySource(t)
	for _, want := range []string{
		"COALESCE(NULLIF(p.product_kind,''),'roasted_bean')",
		"&item.ProductKind",
		"&opt.ProductKind",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("BOM repository must expose product_kind so output and component candidates can apply their own rules; missing %q", want)
		}
	}
}

func TestBomRepositoryExposesOrderUsageForCustomerSkuSorting(t *testing.T) {
	src := readRepositorySource(t)
	for _, want := range []string{
		"order_usage_count",
		"FROM %[1]s.order_items oi",
		"&item.OrderUsageCount",
		"&opt.OrderUsageCount",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("BOM repository must expose order usage for customer/BOM sorting; missing %q", want)
		}
	}
}

func TestBomRepositoryProductsUseParentInventoryUnitForDerivedSKUs(t *testing.T) {
	src := readRepositorySource(t)
	for _, want := range []string{
		"parent_product_direct_unit_template",
		"CASE WHEN COALESCE(p.parent_product_id,0)>0 THEN parent_units.parent_product_inventory_unit",
		"CASE WHEN COALESCE(p.parent_product_id,0)>0 THEN parent_units.parent_product_inventory_unit_explicit",
		"product_direct_unit_template",
		"product_direct_unit_template.id=COALESCE(p.unit_template_id,0)",
		"NULLIF(product_direct_unit_template.inventory_unit,'')",
		"COALESCE(NULLIF(p.unit_rule_override_json->>'inventory_unit',''), NULLIF(product_direct_unit_template.inventory_unit,'')) IS NOT NULL END AS inventory_unit_explicit",
		"NULLIF(product_config.inventory_unit,'')",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("BOM products must resolve child SKU inventory from parent and preserve legacy parent fallback; missing %q", want)
		}
	}
	overrideIdx := strings.Index(src, "NULLIF(p.unit_rule_override_json->>'inventory_unit','')")
	directTemplateIdx := strings.Index(src, "NULLIF(product_direct_unit_template.inventory_unit,'')")
	legacyConfigIdx := strings.Index(src, "NULLIF(product_config.inventory_unit,'')")
	if overrideIdx < 0 || directTemplateIdx < 0 || legacyConfigIdx < 0 || !(overrideIdx < directTemplateIdx && directTemplateIdx < legacyConfigIdx) {
		t.Fatalf("BOM product inventory unit priority must be product override -> direct unit template -> legacy config")
	}
}

func TestBomRepositoryProductsHideTemplateRemovedDerivedSKUs(t *testing.T) {
	src := readRepositorySource(t)
	for _, want := range []string{
		"COALESCE(p.auto_derived_sku,false)",
		"derived_spec_status",
		"template_removed",
		"COALESCE(NULLIF(p.derived_spec_status,''),'active')<>'template_removed'",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("BOM product candidates must hide template-removed derived SKUs; missing %q", want)
		}
	}
}

func TestBomRepositoryPersistsSourceMetadataAndDeriveAudit(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	repository := readRepositorySource(t)
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS %[1]s.product_bom_sources",
		"source_product_code_snapshot",
		"source_bom_version_no_snapshot",
		"derived_at TIMESTAMPTZ",
		"deriveOwnedBomTx",
		`"derive_owned"`,
		`"source_product_code"`,
		`"source_bom_version_no"`,
		"can_edit_bom",
	} {
		if !strings.Contains(string(schema)+"\n"+repository, want) {
			t.Fatalf("BOM source metadata or derive audit missing marker %q", want)
		}
	}
}

func TestProductionBomLibrarySchemaBackfillAndBindingMarkers(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	repository := readRepositorySource(t)
	combined := string(schema) + "\n" + repository
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS %[1]s.production_bom_groups",
		"CREATE TABLE IF NOT EXISTS %[1]s.production_boms",
		"CREATE TABLE IF NOT EXISTS %[1]s.production_bom_versions",
		"CREATE TABLE IF NOT EXISTS %[1]s.production_bom_version_items",
		"CREATE TABLE IF NOT EXISTS %[1]s.product_production_bom_bindings",
		"output_product_id BIGINT NOT NULL DEFAULT 0",
		"output_qty NUMERIC(14,6) NOT NULL DEFAULT 1",
		"output_unit TEXT NOT NULL DEFAULT 'unit'",
		"UPDATE %[1]s.production_boms SET output_product_id=legacy_product_id",
		"backfillProductionBomLibrary",
		"inherit_current",
		"inherit_version",
		"derived_owned",
		"product_production_bom_bindings",
		`"set_default_production_bom"`,
		`"copy_production_bom"`,
		`"publish_production_bom_version"`,
	} {
		if !strings.Contains(combined, want) {
			t.Fatalf("production BOM library implementation missing marker %q", want)
		}
	}
}

func TestCopiedProductionBomVersionNormalizesLegacyYieldToOne(t *testing.T) {
	src := readRepositorySource(t)
	start := strings.Index(src, "func (r Repository) CreateProductionBomVersion")
	if start < 0 {
		t.Fatal("CreateProductionBomVersion source block not found")
	}
	end := strings.Index(src[start:], "\nfunc (r Repository) UpdateProductionBomVersionDraft")
	if end < 0 {
		t.Fatal("CreateProductionBomVersion source block not found")
	}
	block := src[start : start+end]
	if !strings.Contains(block, "yieldRate := 1.0") {
		t.Fatal("copied production BOM versions must normalize legacy overall yield to 1")
	}
	if strings.Contains(block, "COALESCE(yield_rate") {
		t.Fatal("copied production BOM versions must not inherit legacy yield_rate")
	}
}

func TestCopiedProductionBomNormalizesLegacyYieldToOne(t *testing.T) {
	src := readRepositorySource(t)
	start := strings.Index(src, "func (r Repository) CopyProductionBom")
	if start < 0 {
		t.Fatal("CopyProductionBom source block not found")
	}
	end := strings.Index(src[start:], "\nfunc saveBusinessGroupAssignmentForProductionBomTx")
	if end < 0 {
		t.Fatal("CopyProductionBom source block not found")
	}
	block := src[start : start+end]
	if strings.Contains(block, "sourceYield") || strings.Contains(block, "COALESCE(v.yield_rate") {
		t.Fatal("copied production BOM must not inherit legacy yield_rate")
	}
	if !strings.Contains(block, "newBomID, 1.0, sourceOutputQty") {
		t.Fatal("copied production BOM must initialize V001 yield_rate to 1")
	}
	if !strings.Contains(block, "sourceMaterialLossRate") {
		t.Fatal("copied production BOM must preserve the configured material loss rate")
	}
	if !strings.Contains(block, "bomapp.NormalizeProductionBomName(sourceName)") {
		t.Fatal("default copied production BOM name must use the normalized source business name")
	}
}

func TestProductionBomLibraryBackfillDoesNotPublishOrBindEmptyLegacyShells(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(schema)
	start := strings.Index(source, "func backfillProductionBomLibrary")
	if start == -1 {
		t.Fatal("backfillProductionBomLibrary source not found")
	}
	endOffset := strings.Index(source[start:], "\ntype bomVersionSpecialAttrCandidate")
	if endOffset == -1 {
		t.Fatal("backfillProductionBomLibrary source not found")
	}
	backfill := source[start : start+endOffset]

	versionStart := strings.Index(backfill, "WITH version_rows AS (")
	versionEnd := strings.Index(backfill, "\nINSERT INTO %[1]s.production_bom_versions")
	if versionStart == -1 || versionEnd == -1 || versionEnd <= versionStart {
		t.Fatal("legacy version_rows source not found")
	}
	versionRows := backfill[versionStart:versionEnd]
	if strings.Contains(versionRows, "CASE WHEN bv.status='active' THEN 'published'") {
		t.Fatal("empty legacy bom_versions must remain draft instead of becoming published shells")
	}
	for _, want := range []string{"%[1]s.bom_version_items", "%[1]s.product_bom_items", "'published'", "'draft'"} {
		if !strings.Contains(versionRows, want) {
			t.Fatalf("legacy bom_version publication must depend on a real item source; missing %q", want)
		}
	}

	fallbackStart := strings.Index(backfill, "WITH fallback_products AS (")
	if fallbackStart == -1 {
		t.Fatal("fallback_products source not found")
	}
	fallbackTail := backfill[fallbackStart:]
	fallbackEnd := strings.Index(fallbackTail, "ON CONFLICT DO NOTHING;")
	if fallbackEnd == -1 {
		t.Fatal("fallback_products source not found")
	}
	fallback := fallbackTail[:fallbackEnd]
	if strings.Contains(fallback, "SELECT bom_id, 'V001', 'published'") {
		t.Fatal("empty product_bom shells must create draft V001 instead of published V001")
	}
	for _, want := range []string{"%[1]s.product_bom_items", "'published'", "'draft'"} {
		if !strings.Contains(fallback, want) {
			t.Fatalf("fallback V001 publication must depend on real product_bom_items; missing %q", want)
		}
	}

	bindingStart := strings.Index(backfill, "binding_rows AS (")
	if bindingStart == -1 {
		t.Fatal("backfill binding_rows source not found")
	}
	bindingTail := backfill[bindingStart:]
	bindingEnd := strings.Index(bindingTail, ")\nINSERT INTO %[1]s.product_production_bom_bindings")
	if bindingEnd == -1 {
		t.Fatal("backfill binding_rows source not found")
	}
	bindingRows := bindingTail[:bindingEnd]
	latestStart := strings.Index(bindingRows, "LEFT JOIN LATERAL (")
	if latestStart == -1 {
		t.Fatal("latest published BOM binding source not found")
	}
	lockedBinding := bindingRows[:latestStart]
	latestBinding := bindingRows[latestStart:]
	if !strings.Contains(lockedBinding, "%[1]s.production_bom_version_items") {
		t.Fatal("locked legacy BOM binding must require production BOM version items")
	}
	if !strings.Contains(latestBinding, "%[1]s.production_bom_version_items") {
		t.Fatal("latest published BOM binding must require production BOM version items")
	}
}

func TestProductionBomVersionsPersistBomLevelMaterialLossRate(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	repository := readRepositorySource(t)
	combined := string(schema) + "\n" + repository
	for _, want := range []string{
		"material_loss_rate NUMERIC(10,4) NOT NULL DEFAULT 0",
		"ALTER TABLE %[1]s.production_bom_versions ADD COLUMN IF NOT EXISTS material_loss_rate",
		"MAX(COALESCE(i.material_loss_rate,0))",
		"COALESCE(v.material_loss_rate,0)::float8",
		"MaterialLossRate",
		`"material_loss_rate"`,
	} {
		if !strings.Contains(combined, want) {
			t.Fatalf("production BOM version material loss implementation missing marker %q", want)
		}
	}
}

func TestProductionBomVersionItemsKeepMaterialLossSnapshotForRuntimeConsumption(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	repository := readRepositorySource(t)
	combined := string(schema) + "\n" + repository
	for _, want := range []string{
		"material_loss_rate NUMERIC(10,4) NOT NULL DEFAULT 0",
		"ALTER TABLE %[1]s.production_bom_version_items ADD COLUMN IF NOT EXISTS material_loss_rate",
		"COALESCE(bi.material_loss_rate,0)::float8",
		"material_loss_rate, unit_cost_snapshot",
		"itemMaterialLossRate",
		`"material_loss_rate"`,
	} {
		if !strings.Contains(combined, want) {
			t.Fatalf("production BOM material loss implementation missing marker %q", want)
		}
	}
}

func TestProductionBomDraftAllowsFixedPackagingAlongsideRatioMaterialLoss(t *testing.T) {
	repository := readRepositorySource(t)
	start := strings.Index(repository, "func (r Repository) UpdateProductionBomVersionDraft")
	if start < 0 {
		t.Fatal("missing UpdateProductionBomVersionDraft")
	}
	rest := repository[start:]
	end := strings.Index(rest[1:], "\nfunc ")
	if end >= 0 {
		rest = rest[:end+1]
	}
	for _, forbidden := range []string{
		"原料损耗比开启后，组件消耗单位只能使用比例",
		"COALESCE(consume_unit,'')<>'ratio_pct'",
	} {
		if strings.Contains(rest, forbidden) {
			t.Fatalf("BOM loss must not reject fixed packaging; found %q", forbidden)
		}
	}
	if !strings.Contains(rest, `componentType == "material" && item.ConsumeUnit == "ratio_pct"`) {
		t.Fatal("BOM loss snapshot must still apply only to ratio material rows")
	}
}

func TestProductionBomOutputProductAndMultiLevelPublishValidationMarkers(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	repository := readRepositorySource(t)
	combined := string(schema) + "\n" + repository
	for _, want := range []string{
		"OutputProductID",
		"OutputProductName",
		"OutputQty",
		"OutputUnit",
		"ValidateProductionBomVersionForPublish",
		"output_product_id required",
		"components required",
		"cycle detected",
		"component_type IN ('product','finished_product')",
		"ListProductionBomUsageByProduct",
		"listProductionBomUsageByProduct",
		"listProductionBomComponentUsedByBoms",
		"UsedByBoms: usedByBoms",
		"'output' AS relation_type",
		"'component' AS relation_type",
		"WHERE pb.output_product_id=$1",
		"COALESCE(NULLIF(pb.status,''),'active') AS bom_status",
		"AS is_default",
		"COALESCE(pb.output_product_id,0)<>$1",
		"SELECT DISTINCT ON (pb.id)",
	} {
		if !strings.Contains(combined, want) {
			t.Fatalf("production BOM output/multi-level implementation missing marker %q", want)
		}
	}
	if !strings.Contains(repository, "usedByBoms, err := r.listProductionBomComponentUsedByBoms(ctx, summary.OutputProductID)") {
		t.Fatalf("production BOM detail must keep upper-BOM component lookup separate from product archive output usage")
	}
}

func TestProductionBomUsageLookupsUseCurrentVersionOnly(t *testing.T) {
	repository := readRepositorySource(t)
	for _, want := range []string{
		"current_usage_versions AS (",
		"current_component_versions AS (",
		"JOIN current_usage_versions cv ON cv.bom_id=pb.id",
		"JOIN current_component_versions cv ON cv.bom_id=pb.id",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("production BOM usage lookup must inspect only each BOM current draft/published version; missing marker %q", want)
		}
	}
}

func TestProductionBomVersionSpecialAttrsSchemaBackfillAndAuditMarkers(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	repository := readRepositorySource(t)
	combined := string(schema) + "\n" + repository
	for _, want := range []string{
		"ALTER TABLE %[1]s.production_bom_versions ADD COLUMN IF NOT EXISTS special_attrs_schema_json JSONB NOT NULL DEFAULT '[]'::jsonb",
		"ALTER TABLE %[1]s.production_bom_versions ADD COLUMN IF NOT EXISTS special_attrs_json JSONB NOT NULL DEFAULT '{}'::jsonb",
		"backfillProductionBomVersionSpecialAttrs",
		"copyProductionBomForSpecialAttrsConflict",
		"source_bom_version_id",
		"special_attrs_schema_json",
		"special_attrs_json",
		"CASE WHEN $7<>'' THEN $7::jsonb ELSE special_attrs_schema_json END",
		"CASE WHEN $8<>'' THEN $8::jsonb ELSE special_attrs_json END",
		`"update_special_attrs"`,
	} {
		if !strings.Contains(combined, want) {
			t.Fatalf("production BOM version special attrs implementation missing marker %q", want)
		}
	}
}

func TestProductionBomVersionsOwnRouteAndSinglePublishedVersion(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	repository := readRepositorySource(t)
	service, err := os.ReadFile("../../../application/bom/service.go")
	if err != nil {
		t.Fatal(err)
	}
	combined := string(schema) + "\n" + repository + "\n" + string(service)
	for _, want := range []string{
		"ALTER TABLE %[1]s.production_bom_versions ADD COLUMN IF NOT EXISTS process_route_id BIGINT NOT NULL DEFAULT 0",
		"CREATE UNIQUE INDEX IF NOT EXISTS production_bom_versions_one_published_uq",
		"archiveNonLatestPublishedProductionBomVersions",
		"ProcessRouteID",
		"`json:\"process_route_id\"`",
		"process_route_name",
		"IsLatestUsable",
		"is_latest_usable",
		"SET status='archived'",
		"UPDATE %s.production_bom_versions SET status='archived'",
		"process_route_id=$",
		"sourceProcessRouteID",
	} {
		if !strings.Contains(combined, want) {
			t.Fatalf("production BOM version route/single-active implementation missing marker %q", want)
		}
	}
}

func TestProductionBomVersionOperationCostSnapshots(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	repository := readRepositorySource(t)
	combined := string(schema) + "\n" + repository
	for _, want := range []string{
		"production_bom_version_operation_costs",
		"workstation_capacity_id",
		"hourly_rate_snapshot",
		"standard_minutes_snapshot",
		"batch_size_qty_snapshot",
		"operation_unit_cost",
		"operation_cost_unit",
		"refreshProductionBomVersionOperationCostSnapshotsTx",
		"standard_cost_capacity_id",
		"工艺路线工序缺少标准成本产能档",
	} {
		if !strings.Contains(combined, want) {
			t.Fatalf("production BOM operation cost snapshot implementation missing marker %q", want)
		}
	}
}

func TestProductionBomOperationCostSnapshotDoesNotExecWhileRowsOpen(t *testing.T) {
	repository := readRepositorySource(t)
	start := strings.Index(repository, "func refreshProductionBomVersionOperationCostSnapshotsTx")
	if start == -1 {
		t.Fatalf("cannot locate refreshProductionBomVersionOperationCostSnapshotsTx")
	}
	end := strings.Index(repository[start+len("func refreshProductionBomVersionOperationCostSnapshotsTx"):], "\nfunc ")
	body := repository[start:]
	if end >= 0 {
		body = repository[start : start+len("func refreshProductionBomVersionOperationCostSnapshotsTx")+end]
	}
	rowsNextIdx := strings.Index(body, "for rows.Next()")
	rowsErrIdx := strings.Index(body, "rows.Err()")
	if rowsNextIdx == -1 || rowsErrIdx == -1 {
		t.Fatalf("snapshot refresh must still read route operation rows and check rows.Err()")
	}
	if strings.Contains(body[rowsNextIdx:rowsErrIdx], "tx.Exec(ctx") {
		t.Fatalf("snapshot refresh must not call tx.Exec while route operation rows are still open; pgx returns conn busy")
	}
}

func TestProductionBomGroupsArePureUIFoldersWithDeleteAndSort(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	repository := readRepositorySource(t)
	combined := string(schema) + "\n" + repository
	for _, want := range []string{
		"DeleteProductionBomGroup",
		"MoveProductionBomGroup",
		"move_production_bom_group",
		"delete_production_bom_group",
		"SET group_id=0",
		"group_category_id=0",
		"sort_order=$2",
	} {
		if !strings.Contains(combined, want) {
			t.Fatalf("production BOM group folder behavior missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"DEFAULT_PRODUCTION_BOM_GROUP_NAME",
		"VALUES('默认分组'",
		"VALUES($1,100,true,'system','system')",
		"DisableProductionBomGroup",
		"disable_production_bom_group",
		"include_inactive",
		"ON CONFLICT DO NOTHING;\n\nWITH default_group",
	} {
		if strings.Contains(combined, forbidden) {
			t.Fatalf("production BOM groups should not use inactive/disable model; found %q", forbidden)
		}
	}
}

func TestProductionBomGroupCategoriesAndDraftInitialVersionMarkers(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	repository := readRepositorySource(t)
	combined := string(schema) + "\n" + repository
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS %[1]s.production_bom_group_categories",
		"ALTER TABLE %[1]s.production_boms ADD COLUMN IF NOT EXISTS group_category_id BIGINT NOT NULL DEFAULT 0",
		"production_bom_group_categories_group_sort_idx",
		"resetInvalidProductionBomGroupCategories",
		"CreateProductionBomGroupCategory",
		"UpdateProductionBomGroupCategory",
		"DeleteProductionBomGroupCategory",
		"delete_production_bom_group_category",
		"SET group_category_id=0",
		"validateProductionBomGroupCategoryTx",
		"GroupCategoryID",
		"GroupCategoryName",
		"latest_version_status",
		"VALUES($1,'V001','draft'",
		"repairEmptyInitialPublishedProductionBomVersions",
	} {
		if !strings.Contains(combined, want) {
			t.Fatalf("production BOM group category or initial draft behavior missing marker %q", want)
		}
	}
}

func TestProductionBomBackfillRepairsLegacyItemsWithoutBindings(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	combined := string(schema) + "\n" + readRepositorySource(t)
	for _, want := range []string{
		"PR-403: repair legacy product BOM rows that still have items but no production BOM binding",
		"missing_legacy_bindings",
		"LEFT JOIN %[1]s.product_production_bom_bindings existing_binding",
		"existing_binding.product_id IS NULL",
		"EXISTS (SELECT 1 FROM %[1]s.product_bom_items bi WHERE bi.product_id=p.id)",
		"INSERT INTO %[1]s.product_production_bom_bindings(product_id, bom_id, bom_version_id, bound_by)",
		"'system-pr403-legacy-binding-repair'",
	} {
		if !strings.Contains(combined, want) {
			t.Fatalf("legacy BOM binding repair missing marker %q", want)
		}
	}
}

func TestProductionBomLegacyBindingRepairRequiresItemBackedSource(t *testing.T) {
	repository := readRepositorySource(t)
	start := strings.Index(repository, "func repairLegacyProductionBomBindings")
	if start == -1 {
		t.Fatal("repairLegacyProductionBomBindings not found")
	}
	repair := repository[start:]
	candidateStart := strings.Index(repair, "WITH missing_legacy_bindings AS (")
	candidateEnd := strings.Index(repair, "),\ninserted_boms AS (")
	if candidateStart == -1 || candidateEnd == -1 || candidateEnd <= candidateStart {
		t.Fatal("missing_legacy_bindings source not found")
	}
	candidates := repair[candidateStart:candidateEnd]
	for _, forbidden := range []string{
		"pb.product_id IS NOT NULL",
		"EXISTS (SELECT 1 FROM %[1]s.bom_versions bv WHERE bv.product_id=p.id)",
		"JOIN %[1]s.bom_version_items bvi ON bvi.version_id=bv.id",
	} {
		if strings.Contains(candidates, forbidden) {
			t.Fatalf("legacy binding repair must not treat an empty BOM/version shell as repairable; found %q", forbidden)
		}
	}
	for _, want := range []string{
		"%[1]s.product_bom_items",
		"%[1]s.production_bom_versions",
		"%[1]s.production_bom_version_items",
	} {
		if !strings.Contains(candidates, want) {
			t.Fatalf("legacy binding repair candidates must be backed by real BOM items; missing %q", want)
		}
	}
	for _, want := range []string{
		"pg_advisory_xact_lock",
		"target_boms AS",
		"RETURNING id, bom_id, legacy_product_id, published_at",
		"RETURNING version_id",
		"binding_version_candidates AS",
	} {
		if !strings.Contains(repair, want) {
			t.Fatalf("legacy binding repair must serialize and sequence inserted BOMs, versions, items, and bindings; missing %q", want)
		}
	}
	bindingStart := strings.Index(repair, "binding_rows AS (")
	bindingEnd := strings.Index(repair, ")\nINSERT INTO %[1]s.product_production_bom_bindings")
	if bindingStart == -1 || bindingEnd == -1 || bindingEnd <= bindingStart {
		t.Fatal("binding_rows source not found")
	}
	bindingRows := repair[bindingStart:bindingEnd]
	if !strings.Contains(bindingRows, "binding_version_candidates") {
		t.Fatal("legacy binding repair must bind only an item-backed version candidate")
	}
}

func TestProductionBomBackfillPreservesExplicitItemBackedHistoricalVersion(t *testing.T) {
	schema := readBomSchemaSource(t)
	start := strings.Index(schema, "binding_rows AS (")
	if start == -1 {
		t.Fatal("production BOM backfill binding_rows not found")
	}
	end := strings.Index(schema[start:], ")\nINSERT INTO %[1]s.product_production_bom_bindings")
	if end == -1 {
		t.Fatal("production BOM backfill binding insert not found")
	}
	bindingRows := schema[start : start+end]
	if !strings.Contains(schema, "source_type='inherit_version' AS locks_bom_version") {
		t.Fatal("explicit inherit_version intent must survive a zero or missing legacy version id")
	}
	for _, want := range []string{
		"CASE WHEN tr.locks_bom_version THEN locked.id ELSE latest.id END",
		"locked.legacy_bom_version_id=tr.source_bom_version_id",
		"locked.bom_id=pbom.id",
		"tr.locks_bom_version",
		"production_bom_version_items locked_item",
	} {
		if !strings.Contains(bindingRows, want) {
			t.Fatalf("explicit historical binding guard missing %q", want)
		}
	}
	if strings.Contains(bindingRows, "locked.status='published'") {
		t.Fatal("an explicit item-backed historical version must not silently fall back to the latest published version")
	}
}

func TestProductionBomSpecialAttrsBackfillRequiresSourceComponents(t *testing.T) {
	schema := readBomSchemaSource(t)
	start := strings.Index(schema, "func backfillProductionBomVersionSpecialAttrs")
	if start == -1 {
		t.Fatal("backfillProductionBomVersionSpecialAttrs not found")
	}
	backfill := schema[start:]
	for _, want := range []string{
		"production_bom_version_items source_item",
		"insertedItems.RowsAffected() == 0",
		"has no components",
	} {
		if !strings.Contains(backfill, want) {
			t.Fatalf("special attrs backfill must reject empty published copies; missing %q", want)
		}
	}
}

func TestProductionBomDetailListsReferencedProducts(t *testing.T) {
	repository := readRepositorySource(t)
	for _, want := range []string{
		"listProductionBomReferencedProducts",
		"JOIN %[1]s.products p ON p.id=b.product_id",
		"WHERE b.bom_id=$1",
		"ReferencedProducts: referencedProducts",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("production BOM detail referenced product implementation missing marker %q", want)
		}
	}
}

func TestProductionBomCanDeactivateWhenActiveProductsReferenceIt(t *testing.T) {
	repository := readRepositorySource(t)
	start := strings.Index(repository, "func (r Repository) UpdateProductionBom")
	end := strings.Index(repository, "func (r Repository) CopyProductionBom")
	if start == -1 || end == -1 || end <= start {
		t.Fatalf("cannot locate UpdateProductionBom source")
	}
	updateProductionBom := repository[start:end]
	for _, forbidden := range []string{
		"production BOM is used by active products",
		"deactivate products first",
		"activeReferences",
		"p.active=true",
	} {
		if strings.Contains(updateProductionBom, forbidden) {
			t.Fatalf("production BOM deactivation should not block active product references; found %q", forbidden)
		}
	}
}

func TestProductionBomGroupingUsesBusinessGroupAssignments(t *testing.T) {
	repository := readRepositorySource(t)
	for _, want := range []string{
		"LEFT JOIN %[1]s.business_group_assignments bga",
		"lower(bga.usage_key)='production_bom'",
		"lower(bga.object_key)='production_bom'",
		"ORDER BY COALESCE(bga.sort_order,100), pb.name, pb.id",
		"saveBusinessGroupAssignmentForProductionBomTx",
		`"save_business_group_assignment"`,
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("production BOM generic group assignment implementation missing marker %q", want)
		}
	}
	if strings.Contains(repository, "ORDER BY COALESCE(g.sort_order,100), pb.name, pb.id") {
		t.Fatalf("production BOM list must not order by legacy group alias g after PR-442 generic grouping")
	}
	for _, fn := range []string{"func (r Repository) CreateProductionBom", "func (r Repository) UpdateProductionBom", "func (r Repository) CopyProductionBom"} {
		start := strings.Index(repository, fn)
		if start == -1 {
			t.Fatalf("cannot locate %s", fn)
		}
		next := strings.Index(repository[start+len(fn):], "\nfunc ")
		body := repository[start:]
		if next >= 0 {
			body = repository[start : start+len(fn)+next]
		}
		for _, forbidden := range []string{
			"group_id=$",
			"group_category_id=$",
			"INSERT INTO %s.production_boms(code, name, output_product_id, group_id, group_category_id",
		} {
			if strings.Contains(body, forbidden) {
				t.Fatalf("%s should not write legacy production_boms group columns; found %q", fn, forbidden)
			}
		}
	}
}

func TestProductionBomLegacyGroupWritesAreReadonlyCompatibility(t *testing.T) {
	repository := readRepositorySource(t)
	for _, want := range []string{
		"production BOM groups are legacy readonly",
		"ErrLegacyProductionBomGroupsReadonly",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("legacy production BOM group write guard missing marker %q", want)
		}
	}
}

func TestCreateProductionBomDefaultsToNoHiddenOverallLoss(t *testing.T) {
	repository := readRepositorySource(t)
	start := strings.Index(repository, "func (r Repository) CreateProductionBom(ctx")
	end := strings.Index(repository[start:], "func (r Repository) UpdateProductionBom(ctx")
	if start < 0 || end < 0 {
		t.Fatal("cannot locate CreateProductionBom")
	}
	body := repository[start : start+end]
	if !strings.Contains(body, "yieldRate := 1.0") {
		t.Fatal("new production BOM must default yield_rate to 1.0")
	}
	if strings.Contains(body, "yieldRate := 0.8") {
		t.Fatal("new production BOM must not hide a default 20% overall loss")
	}
}

func TestProductionBomVersionSchemaDefaultsNewRowsToFullYield(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(schema)
	for _, want := range []string{
		"yield_rate NUMERIC(10,4) NOT NULL DEFAULT 1.0000",
		"ALTER TABLE %[1]s.production_bom_versions ALTER COLUMN yield_rate SET DEFAULT 1.0000",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("new production BOM version schema must default to no hidden overall loss; missing %q", want)
		}
	}
}

func TestLegacyBomCompatibilityTablesDefaultNewRowsToFullYieldWithoutBackfill(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(schema)
	for _, want := range []string{
		"yield_rate NUMERIC(10,4) NOT NULL DEFAULT 1.0000",
		"ALTER TABLE %s.product_bom ALTER COLUMN yield_rate SET DEFAULT 1.0000",
		"ALTER TABLE %[1]s.bom_versions ALTER COLUMN yield_rate SET DEFAULT 1.0000",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("legacy BOM compatibility tables must use a neutral default for new rows; missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"yield_rate NUMERIC(10,4) NOT NULL DEFAULT 0.8000",
		"UPDATE %s.product_bom SET yield_rate",
		"UPDATE %[1]s.bom_versions SET yield_rate",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("legacy BOM compatibility defaults must be neutral without rewriting historical rows; found %q", forbidden)
		}
	}
}

func readRepositorySource(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func readBomSchemaSource(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
