package bom

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	bomapp "orderapp/internal/application/bom"
	productspecmigrationapp "orderapp/internal/application/productspecmigration"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestProductBOMCreationRequiresSpecTemplateForNewAndCutoverProductsPostgres(t *testing.T) {
	ctx, pool, schema := newPR600BomMaintenanceTestDB(t)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(id,name,active) VALUES
			(701,'PR600 新商品',true),
			(702,'PR600 迁移中历史商品',true),
			(703,'PR600 已切换商品',true),
			(704,'PR600 未准备历史商品',true),
			(705,'PR600 legacy 历史商品',true),
			(706,'PR600 ready 历史商品',true);
		INSERT INTO %s.product_bom_spec_migrations(product_id,state,legacy_catalog_product) VALUES
			(701,'preparing',false),
			(702,'preparing',true),
			(703,'cutover',true),
			(705,'legacy',true),
			(706,'ready',true);
	`, schema, schema)); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	createWithoutTemplate := func(productID int64) error {
		_, err := repo.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{
			Name: "无规格组商品 BOM", OutputType: "product", OutputID: productID,
			OutputProductID: productID, OutputQty: 1, OutputUnit: "袋", Actor: "pr600-authority-test",
		})
		return err
	}

	for _, productID := range []int64{701, 703} {
		err := createWithoutTemplate(productID)
		if err == nil || !strings.Contains(err.Error(), "published specification template") {
			t.Fatalf("product %d product-BOM without specification template error=%v", productID, err)
		}
	}
	for _, productID := range []int64{702, 705, 706} {
		if err := createWithoutTemplate(productID); err != nil {
			t.Fatalf("explicit legacy catalog product %d must retain single-recipe compatibility: %v", productID, err)
		}
	}
	if err := createWithoutTemplate(704); err != nil {
		t.Fatalf("product without migration row must retain legacy compatibility: %v", err)
	}
	var sourceMaterialID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.materials(code,name,kind,unit,cost_unit)
		VALUES('PR600-COPY-SOURCE','复制来源物料','bean','kg','kg') RETURNING id
	`, schema)).Scan(&sourceMaterialID); err != nil {
		t.Fatal(err)
	}
	sourceBOM, err := repo.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{
		Name: "物料复制来源", OutputType: "material", OutputID: sourceMaterialID, OutputMaterialID: sourceMaterialID,
		OutputQty: 1, OutputUnit: "kg", Actor: "pr600-authority-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CopyProductionBom(ctx, bomapp.CopyProductionBomCommand{
		ID: sourceBOM.ID, Name: "错误转换商品 BOM", OutputType: "product", OutputID: 701, OutputProductID: 701,
		Actor: "pr600-authority-test",
	}); err == nil || !strings.Contains(err.Error(), "published specification template") {
		t.Fatalf("copy material BOM to new product without template error=%v", err)
	}
	if _, err := repo.UpdateProductionBom(ctx, bomapp.UpdateProductionBomCommand{
		ID: sourceBOM.ID, Name: "错误改成商品 BOM", OutputType: "product", OutputID: 701, OutputProductID: 701,
		UpdateOutputBinding: true, Status: "active", Actor: "pr600-authority-test",
	}); err == nil || !strings.Contains(err.Error(), "specification") {
		t.Fatalf("edit material BOM to new product without specification group error=%v", err)
	}

	var rejectedCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %s.production_boms
		WHERE output_type='product' AND output_product_id IN (701,703)
	`, schema)).Scan(&rejectedCount); err != nil {
		t.Fatal(err)
	}
	if rejectedCount != 0 {
		t.Fatalf("rejected new/cutover product BOM writes persisted %d rows", rejectedCount)
	}
}

func TestReapplyProductBOMSpecTemplateKeepsStableIDsAndAuditsWholeGroupPostgres(t *testing.T) {
	ctx, pool, schema := newPR600BomMaintenanceTestDB(t)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_unit_definitions(code) VALUES('袋'),('盒') ON CONFLICT(code) DO NOTHING;
		INSERT INTO %s.products(id,name,active) VALUES(801,'模板重套商品',true);
		INSERT INTO %s.product_bom_spec_migrations(product_id,state,legacy_catalog_product) VALUES(801,'preparing',false);
	`, schema, schema, schema)); err != nil {
		t.Fatal(err)
	}
	var mainInputMaterialID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.materials(code,name,kind,unit,cost_unit,purchase_price)
		VALUES('PR600-MAIN','PR600 主投入','bean','kg','kg',50)
		RETURNING id
	`, schema)).Scan(&mainInputMaterialID); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	createdTemplate, err := repo.CreateProductionBomSpecTemplate(ctx, bomapp.CreateProductionBomSpecTemplateCommand{Name: "PR600 可维护规格", Actor: "template-owner"})
	if err != nil {
		t.Fatal(err)
	}
	if len(createdTemplate.Versions) != 1 {
		t.Fatalf("initial template versions=%+v", createdTemplate.Versions)
	}
	templateV1 := createdTemplate.Versions[0].ID
	variantsV1 := []bomapp.ProductionBomSpecTemplateVariant{
		{SpecKey: "bag-227", Name: "227g 袋", InventoryUnit: "袋", IsDefault: true, SortOrder: 1, Items: []bomapp.ProductionBomSpecTemplateVariantDraftItem{{IsMainInput: true, ProductionBomDraftItem: bomapp.ProductionBomDraftItem{ComponentType: "material", ConsumeUnit: "main_input_unit", QtyPerUnit: 0.227}}}},
		{SpecKey: "bag-454", Name: "454g 袋", InventoryUnit: "袋", SortOrder: 2, Items: []bomapp.ProductionBomSpecTemplateVariantDraftItem{{IsMainInput: true, ProductionBomDraftItem: bomapp.ProductionBomDraftItem{ComponentType: "material", ConsumeUnit: "main_input_unit", QtyPerUnit: 0.454}}}},
	}
	if _, err := repo.UpdateProductionBomSpecTemplateVersionDraft(ctx, bomapp.UpdateProductionBomSpecTemplateVersionDraftCommand{VersionID: templateV1, Variants: variantsV1, Actor: "template-owner"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.PublishProductionBomSpecTemplateVersion(ctx, bomapp.PublishProductionBomSpecTemplateVersionCommand{VersionID: templateV1, Actor: "template-owner"}); err != nil {
		t.Fatal(err)
	}
	createdBOM, err := repo.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{
		Name: "PR600 模板重套 BOM", OutputType: "product", OutputID: 801, OutputProductID: 801,
		OutputQty: 1, OutputUnit: "袋", SpecTemplateVersionID: templateV1, MainInputMaterialID: mainInputMaterialID, Actor: "bom-owner",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_versions SET status='published',published_at=now() WHERE id=$1`, schema), createdBOM.LatestVersionID); err != nil {
		t.Fatal(err)
	}
	var spec227ID, removedSpec454ID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_bom_specs WHERE bom_id=$1 AND spec_key='bag-227'`, schema), createdBOM.ID).Scan(&spec227ID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_bom_specs WHERE bom_id=$1 AND spec_key='bag-454'`, schema), createdBOM.ID).Scan(&removedSpec454ID); err != nil {
		t.Fatal(err)
	}

	templateV2Row, err := repo.CreateProductionBomSpecTemplateVersion(ctx, bomapp.CreateProductionBomSpecTemplateVersionCommand{TemplateID: createdTemplate.ID, SourceVersionID: templateV1, Note: "替换规格", Actor: "template-owner"})
	if err != nil {
		t.Fatal(err)
	}
	templateV2 := templateV2Row.ID
	variantsV2 := []bomapp.ProductionBomSpecTemplateVariant{
		variantsV1[0],
		{SpecKey: "bag-100", Name: "100g 袋", InventoryUnit: "袋", SortOrder: 2, Items: []bomapp.ProductionBomSpecTemplateVariantDraftItem{{IsMainInput: true, ProductionBomDraftItem: bomapp.ProductionBomDraftItem{ComponentType: "material", ConsumeUnit: "main_input_unit", QtyPerUnit: 0.1}}}},
	}
	if _, err := repo.UpdateProductionBomSpecTemplateVersionDraft(ctx, bomapp.UpdateProductionBomSpecTemplateVersionDraftCommand{VersionID: templateV2, Variants: variantsV2, Actor: "template-owner"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.PublishProductionBomSpecTemplateVersion(ctx, bomapp.PublishProductionBomSpecTemplateVersionCommand{VersionID: templateV2, Actor: "template-owner"}); err != nil {
		t.Fatal(err)
	}
	draft, err := repo.CreateProductionBomVersion(ctx, bomapp.CreateProductionBomVersionCommand{BomID: createdBOM.ID, SourceVersionID: createdBOM.LatestVersionID, Note: "重套 V2", Actor: "bom-owner"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ReapplyProductionBomSpecTemplateVersion(ctx, bomapp.ReapplyProductionBomSpecTemplateVersionCommand{
		VersionID: draft.ID, SpecTemplateVersionID: templateV2, MainInputMaterialID: mainInputMaterialID, Actor: "bom-owner",
	}); err != nil {
		t.Fatal(err)
	}

	type storedSpec struct {
		id  int64
		key string
	}
	rows, err := pool.Query(ctx, fmt.Sprintf(`
		SELECT variant.bom_spec_id,spec.spec_key
		FROM %s.production_bom_version_variants variant
		JOIN %s.production_bom_specs spec ON spec.id=variant.bom_spec_id
		WHERE variant.version_id=$1 ORDER BY variant.sort_order,variant.id
	`, schema, schema), draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	var stored []storedSpec
	for rows.Next() {
		var spec storedSpec
		if err := rows.Scan(&spec.id, &spec.key); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		stored = append(stored, spec)
	}
	rows.Close()
	if len(stored) != 2 || stored[0].key != "bag-227" || stored[0].id != spec227ID || stored[1].key != "bag-100" || stored[1].id == spec227ID || stored[1].id == removedSpec454ID {
		t.Fatalf("reapplied stable/new specifications=%+v want retained=%d removed=%d", stored, spec227ID, removedSpec454ID)
	}
	var retainedRemovedSpec, reapplyAudit int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.production_bom_specs WHERE id=$1`, schema), removedSpec454ID).Scan(&retainedRemovedSpec); err != nil {
		t.Fatal(err)
	}
	if retainedRemovedSpec != 1 {
		t.Fatal("removed draft specification identity must remain as a historical tombstone")
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %s.audit_logs
		WHERE entity_type='production_bom_version' AND entity_id=$1
		  AND action='reapply_spec_template' AND field='variants'
		  AND old_value IS NOT NULL AND new_value IS NOT NULL
	`, schema), draft.ID).Scan(&reapplyAudit); err != nil {
		t.Fatal(err)
	}
	if reapplyAudit != 1 {
		t.Fatalf("whole-group reapply audit count=%d want 1", reapplyAudit)
	}
	var sourceTemplateVersionID, savedMainInputMaterialID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT source_spec_template_version_id,main_input_material_id FROM %s.production_bom_versions WHERE id=$1`, schema), draft.ID).Scan(&sourceTemplateVersionID, &savedMainInputMaterialID); err != nil {
		t.Fatal(err)
	}
	if sourceTemplateVersionID != templateV2 || savedMainInputMaterialID != mainInputMaterialID {
		t.Fatalf("draft template/main input=%d/%d want %d/%d", sourceTemplateVersionID, savedMainInputMaterialID, templateV2, mainInputMaterialID)
	}

	templateV3Row, err := repo.CreateProductionBomSpecTemplateVersion(ctx, bomapp.CreateProductionBomSpecTemplateVersionCommand{TemplateID: createdTemplate.ID, SourceVersionID: templateV2, Note: "非法改单位", Actor: "template-owner"})
	if err != nil {
		t.Fatal(err)
	}
	variantsV3 := append([]bomapp.ProductionBomSpecTemplateVariant(nil), variantsV2...)
	variantsV3[0].InventoryUnit = "盒"
	if _, err := repo.UpdateProductionBomSpecTemplateVersionDraft(ctx, bomapp.UpdateProductionBomSpecTemplateVersionDraftCommand{VersionID: templateV3Row.ID, Variants: variantsV3, Actor: "template-owner"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.PublishProductionBomSpecTemplateVersion(ctx, bomapp.PublishProductionBomSpecTemplateVersionCommand{VersionID: templateV3Row.ID, Actor: "template-owner"}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ReapplyProductionBomSpecTemplateVersion(ctx, bomapp.ReapplyProductionBomSpecTemplateVersionCommand{
		VersionID: draft.ID, SpecTemplateVersionID: templateV3Row.ID, MainInputMaterialID: mainInputMaterialID, Actor: "bom-owner",
	}); err == nil || !strings.Contains(err.Error(), "inventory_unit cannot be changed") {
		t.Fatalf("reapply changing published specification unit error=%v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT source_spec_template_version_id FROM %s.production_bom_versions WHERE id=$1`, schema), draft.ID).Scan(&sourceTemplateVersionID); err != nil {
		t.Fatal(err)
	}
	if sourceTemplateVersionID != templateV2 {
		t.Fatalf("failed unit-changing reapply mutated source template to %d", sourceTemplateVersionID)
	}
}

func TestProductBOMOutputRebindingRejectsDraftSpecGroupWithoutPublishedTemplateProvenancePostgres(t *testing.T) {
	ctx, pool, schema := newPR600BomMaintenanceTestDB(t)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_unit_definitions(code) VALUES('袋') ON CONFLICT(code) DO NOTHING;
		INSERT INTO %s.products(id,name,active) VALUES
			(901,'PR600 新商品改绑目标',true),
			(902,'PR600 历史商品改绑目标',true),
			(903,'PR600 cutover 商品改绑目标',true);
		INSERT INTO %s.product_bom_spec_migrations(product_id,state,legacy_catalog_product) VALUES
			(901,'preparing',false),
			(902,'preparing',true),
			(903,'cutover',true);
	`, schema, schema, schema)); err != nil {
		t.Fatal(err)
	}
	var outputMaterialID, inputMaterialID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.materials(code,name,kind,unit,cost_unit)
		VALUES('PR600-REBIND-OUTPUT','改绑来源物料','bean','kg','kg') RETURNING id
	`, schema)).Scan(&outputMaterialID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.materials(code,name,kind,unit,cost_unit)
		VALUES('PR600-REBIND-INPUT','改绑主投入','bean','kg','kg') RETURNING id
	`, schema)).Scan(&inputMaterialID); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	unpublishedTemplate, err := repo.CreateProductionBomSpecTemplate(ctx, bomapp.CreateProductionBomSpecTemplateCommand{Name: "未发布来源模板", Actor: "rebind-auditor"})
	if err != nil {
		t.Fatal(err)
	}
	unpublishedTemplateVersionID := unpublishedTemplate.Versions[0].ID
	createSelfBuiltVariantBOM := func(name string) bomapp.ProductionBomSummary {
		t.Helper()
		row, err := repo.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{
			Name: name, OutputType: "material", OutputID: outputMaterialID, OutputMaterialID: outputMaterialID,
			OutputQty: 1, OutputUnit: "kg", Actor: "rebind-auditor",
		})
		if err != nil {
			t.Fatal(err)
		}
		_, err = repo.UpdateProductionBomVersionDraft(ctx, bomapp.UpdateProductionBomVersionDraftCommand{
			VersionID: row.LatestVersionID, Actor: "rebind-auditor",
			Variants: []bomapp.ProductionBomDraftVariant{{
				SpecKey: "self-built-bag", Name: "自建袋装", InventoryUnit: "袋", IsDefault: true,
				Items: []bomapp.ProductionBomDraftItem{{MaterialID: inputMaterialID, ComponentType: "material", ConsumeUnit: "kg", QtyPerUnit: 0.227}},
			}},
		})
		if err != nil {
			t.Fatal(err)
		}
		return row
	}

	for _, productID := range []int64{901, 903} {
		selfBuilt := createSelfBuiltVariantBOM(fmt.Sprintf("自建规格改绑 %d", productID))
		if productID == 903 {
			if _, err := pool.Exec(ctx, fmt.Sprintf(`
				UPDATE %s.production_bom_versions
				SET source_spec_template_version_id=$2,main_input_material_id=$3
				WHERE id=$1
			`, schema), selfBuilt.LatestVersionID, unpublishedTemplateVersionID, inputMaterialID); err != nil {
				t.Fatal(err)
			}
		}
		_, err := repo.UpdateProductionBom(ctx, bomapp.UpdateProductionBomCommand{
			ID: selfBuilt.ID, Name: "不应改绑", OutputType: "product", OutputID: productID, OutputProductID: productID,
			UpdateOutputBinding: true, Status: "active", Actor: "rebind-auditor",
		})
		if err == nil || !strings.Contains(err.Error(), "published specification template") {
			t.Fatalf("self-built draft variants rebound to governed product %d error=%v", productID, err)
		}
		var storedType string
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT output_type FROM %s.production_boms WHERE id=$1`, schema), selfBuilt.ID).Scan(&storedType); err != nil {
			t.Fatal(err)
		}
		if storedType != "material" {
			t.Fatalf("rejected rebound mutated BOM %d output_type=%q", selfBuilt.ID, storedType)
		}
	}

	legacyCompatible := createSelfBuiltVariantBOM("历史商品兼容自建规格")
	if _, err := repo.UpdateProductionBom(ctx, bomapp.UpdateProductionBomCommand{
		ID: legacyCompatible.ID, Name: "历史兼容改绑", OutputType: "product", OutputID: 902, OutputProductID: 902,
		UpdateOutputBinding: true, Status: "active", Actor: "rebind-auditor",
	}); err != nil {
		t.Fatalf("explicit legacy product must retain self-built draft compatibility: %v", err)
	}

	template, err := repo.CreateProductionBomSpecTemplate(ctx, bomapp.CreateProductionBomSpecTemplateCommand{Name: "合法改绑模板", Actor: "rebind-auditor"})
	if err != nil {
		t.Fatal(err)
	}
	templateVersionID := template.Versions[0].ID
	if _, err := repo.UpdateProductionBomSpecTemplateVersionDraft(ctx, bomapp.UpdateProductionBomSpecTemplateVersionDraftCommand{
		VersionID: templateVersionID, Actor: "rebind-auditor",
		Variants: []bomapp.ProductionBomSpecTemplateVariant{{
			SpecKey: "published-bag", Name: "模板袋装", InventoryUnit: "袋", IsDefault: true,
			Items: []bomapp.ProductionBomSpecTemplateVariantDraftItem{{IsMainInput: true, ProductionBomDraftItem: bomapp.ProductionBomDraftItem{ComponentType: "material", ConsumeUnit: "main_input_unit", QtyPerUnit: 0.227}}},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.PublishProductionBomSpecTemplateVersion(ctx, bomapp.PublishProductionBomSpecTemplateVersionCommand{VersionID: templateVersionID, Actor: "rebind-auditor"}); err != nil {
		t.Fatal(err)
	}
	invalidMainInput := createSelfBuiltVariantBOM("无效主投入来源")
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.production_bom_versions
		SET source_spec_template_version_id=$2,main_input_material_id=999999
		WHERE id=$1
	`, schema), invalidMainInput.LatestVersionID, templateVersionID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpdateProductionBom(ctx, bomapp.UpdateProductionBomCommand{
		ID: invalidMainInput.ID, Name: "不应接受无效主投入", OutputType: "product", OutputID: 901, OutputProductID: 901,
		UpdateOutputBinding: true, Status: "active", Actor: "rebind-auditor",
	}); err == nil || !strings.Contains(err.Error(), "active main input material") {
		t.Fatalf("published template with invalid main input rebound error=%v", err)
	}
	provenanced, err := repo.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{
		Name: "有模板来源的历史商品 BOM", OutputType: "product", OutputID: 902, OutputProductID: 902,
		OutputQty: 1, OutputUnit: "袋", SpecTemplateVersionID: templateVersionID, MainInputMaterialID: inputMaterialID, Actor: "rebind-auditor",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpdateProductionBom(ctx, bomapp.UpdateProductionBomCommand{
		ID: provenanced.ID, Name: "合法改绑新商品", OutputType: "product", OutputID: 901, OutputProductID: 901,
		UpdateOutputBinding: true, Status: "active", Actor: "rebind-auditor",
	}); err != nil {
		t.Fatalf("published-template-provenanced draft must be allowed to rebind: %v", err)
	}
}

func TestReapplyProductBOMSpecTemplatePreservesStableSpecBarcodePostgres(t *testing.T) {
	ctx, pool, schema := newPR600BomMaintenanceTestDB(t)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_unit_definitions(code) VALUES('袋') ON CONFLICT(code) DO NOTHING;
		INSERT INTO %s.products(id,name,active) VALUES(911,'PR600 条码保留商品',true);
		INSERT INTO %s.product_bom_spec_migrations(product_id,state,legacy_catalog_product)
		VALUES(911,'preparing',false);
	`, schema, schema, schema)); err != nil {
		t.Fatal(err)
	}
	var mainInputMaterialID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.materials(code,name,kind,unit,cost_unit)
		VALUES('PR600-BARCODE-MAIN','条码测试主投入','bean','kg','kg') RETURNING id
	`, schema)).Scan(&mainInputMaterialID); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	template, err := repo.CreateProductionBomSpecTemplate(ctx, bomapp.CreateProductionBomSpecTemplateCommand{Name: "条码保留模板", Actor: "barcode-auditor"})
	if err != nil {
		t.Fatal(err)
	}
	templateV1 := template.Versions[0].ID
	variants := []bomapp.ProductionBomSpecTemplateVariant{{
		SpecKey: "stable-bag", Name: "稳定袋装 V1", InventoryUnit: "袋", IsDefault: true,
		Items: []bomapp.ProductionBomSpecTemplateVariantDraftItem{{IsMainInput: true, ProductionBomDraftItem: bomapp.ProductionBomDraftItem{ComponentType: "material", ConsumeUnit: "main_input_unit", QtyPerUnit: 0.227}}},
	}}
	if _, err := repo.UpdateProductionBomSpecTemplateVersionDraft(ctx, bomapp.UpdateProductionBomSpecTemplateVersionDraftCommand{VersionID: templateV1, Variants: variants, Actor: "barcode-auditor"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.PublishProductionBomSpecTemplateVersion(ctx, bomapp.PublishProductionBomSpecTemplateVersionCommand{VersionID: templateV1, Actor: "barcode-auditor"}); err != nil {
		t.Fatal(err)
	}
	createdBOM, err := repo.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{
		Name: "条码保留 BOM", OutputType: "product", OutputID: 911, OutputProductID: 911,
		OutputQty: 1, OutputUnit: "袋", SpecTemplateVersionID: templateV1, MainInputMaterialID: mainInputMaterialID, Actor: "barcode-auditor",
	})
	if err != nil {
		t.Fatal(err)
	}
	var stableSpecID int64
	var stableSpecCode string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		UPDATE %s.production_bom_specs SET barcode='KEEP-ME-227'
		WHERE bom_id=$1 AND spec_key='stable-bag'
		RETURNING id,code
	`, schema), createdBOM.ID).Scan(&stableSpecID, &stableSpecCode); err != nil {
		t.Fatal(err)
	}
	templateV2Row, err := repo.CreateProductionBomSpecTemplateVersion(ctx, bomapp.CreateProductionBomSpecTemplateVersionCommand{
		TemplateID: template.ID, SourceVersionID: templateV1, Note: "名称升级", Actor: "barcode-auditor",
	})
	if err != nil {
		t.Fatal(err)
	}
	variants[0].Name = "稳定袋装 V2"
	if _, err := repo.UpdateProductionBomSpecTemplateVersionDraft(ctx, bomapp.UpdateProductionBomSpecTemplateVersionDraftCommand{VersionID: templateV2Row.ID, Variants: variants, Actor: "barcode-auditor"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.PublishProductionBomSpecTemplateVersion(ctx, bomapp.PublishProductionBomSpecTemplateVersionCommand{VersionID: templateV2Row.ID, Actor: "barcode-auditor"}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ReapplyProductionBomSpecTemplateVersion(ctx, bomapp.ReapplyProductionBomSpecTemplateVersionCommand{
		VersionID: createdBOM.LatestVersionID, SpecTemplateVersionID: templateV2Row.ID, MainInputMaterialID: mainInputMaterialID, Actor: "barcode-auditor",
	}); err != nil {
		t.Fatal(err)
	}
	var gotID int64
	var gotCode, gotKey, gotBarcode string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id,code,spec_key,barcode FROM %s.production_bom_specs
		WHERE bom_id=$1 AND spec_key='stable-bag'
	`, schema), createdBOM.ID).Scan(&gotID, &gotCode, &gotKey, &gotBarcode); err != nil {
		t.Fatal(err)
	}
	if gotID != stableSpecID || gotCode != stableSpecCode || gotKey != "stable-bag" || gotBarcode != "KEEP-ME-227" {
		t.Fatalf("stable specification changed after reapply: id/code/key/barcode=%d/%q/%q/%q want %d/%q/%q/%q", gotID, gotCode, gotKey, gotBarcode, stableSpecID, stableSpecCode, "stable-bag", "KEEP-ME-227")
	}
}

func TestGovernedProductBOMVersionRequiresPublishedTemplateProvenanceAcrossPublishAndBindingsPostgres(t *testing.T) {
	ctx, pool, schema := newPR600BomMaintenanceTestDB(t)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		ALTER TABLE %s.products ADD COLUMN IF NOT EXISTS parent_product_id BIGINT NOT NULL DEFAULT 0;
		CREATE TABLE %s.product_production_configs(
			product_id BIGINT PRIMARY KEY,
			production_bom_id BIGINT NOT NULL DEFAULT 0,
			production_bom_version_id BIGINT NOT NULL DEFAULT 0,
			process_route_id BIGINT NOT NULL DEFAULT 0,
			industry_field_template_id BIGINT NOT NULL DEFAULT 0,
			expected_loss_rate NUMERIC(10,4) NOT NULL DEFAULT 0,
			note TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			created_by TEXT NOT NULL DEFAULT '',
			updated_by TEXT NOT NULL DEFAULT ''
		);
		INSERT INTO %s.product_unit_definitions(code) VALUES('袋') ON CONFLICT(code) DO NOTHING;
		INSERT INTO %s.products(id,name,active) VALUES
			(921,'发布后切换商品',true),
			(922,'默认绑定新商品',true),
			(923,'通用绑定切换商品',true),
			(924,'明确 legacy 兼容商品',true);
		INSERT INTO %s.product_bom_spec_migrations(product_id,state,legacy_catalog_product) VALUES
			(921,'legacy',true),(922,'legacy',true),(923,'legacy',true),(924,'legacy',true);
	`, schema, schema, schema, schema, schema)); err != nil {
		t.Fatal(err)
	}
	var inputMaterialID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.materials(code,name,kind,unit,cost_unit)
		VALUES('PR600-LATE-GOVERNANCE-INPUT','后切换主投入','bean','kg','kg') RETURNING id
	`, schema)).Scan(&inputMaterialID); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	publishVersion := func(versionID int64) error {
		t.Helper()
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback(ctx) }()
		if err := repo.publishProductionBomVersionTx(ctx, tx, bomapp.PublishProductionBomVersionCommand{VersionID: versionID, Actor: "late-governance-auditor"}); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	createLegacyBOM := func(productID int64, name string) bomapp.ProductionBomSummary {
		t.Helper()
		row, err := repo.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{
			Name: name, OutputType: "product", OutputID: productID, OutputProductID: productID,
			OutputQty: 1, OutputUnit: "袋", Actor: "late-governance-auditor",
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := repo.UpdateProductionBomVersionDraft(ctx, bomapp.UpdateProductionBomVersionDraftCommand{
			VersionID: row.LatestVersionID, Actor: "late-governance-auditor",
			Variants: []bomapp.ProductionBomDraftVariant{{
				SpecKey: "legacy-bag", Name: "旧规格袋装", InventoryUnit: "袋", IsDefault: true,
				Items: []bomapp.ProductionBomDraftItem{{MaterialID: inputMaterialID, ComponentType: "material", ConsumeUnit: "kg", QtyPerUnit: 0.227}},
			}},
		}); err != nil {
			t.Fatal(err)
		}
		return row
	}

	publishCandidate := createLegacyBOM(921, "切换后待发布 BOM")
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.product_bom_spec_migrations SET state='cutover' WHERE product_id=921`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := publishVersion(publishCandidate.LatestVersionID); err == nil || !strings.Contains(err.Error(), "published specification template") {
		t.Errorf("cutover publish without template provenance error=%v", err)
	}

	productBindCandidate := createLegacyBOM(922, "新商品默认绑定候选")
	if err := publishVersion(productBindCandidate.LatestVersionID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.product_bom_spec_migrations SET state='preparing',legacy_catalog_product=false WHERE product_id=922`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.BindProductProductionBom(ctx, bomapp.BindProductProductionBomCommand{ProductID: 922, BomID: productBindCandidate.ID, Actor: "late-governance-auditor"}); err == nil || !strings.Contains(err.Error(), "published specification template") {
		t.Errorf("new product default bind without template provenance error=%v", err)
	}

	outputBindCandidate := createLegacyBOM(923, "切换商品通用绑定候选")
	if err := publishVersion(outputBindCandidate.LatestVersionID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.product_bom_spec_migrations SET state='cutover' WHERE product_id=923`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.BindProductionBomOutput(ctx, bomapp.BindProductionBomOutputCommand{OutputType: "product", OutputID: 923, BomID: outputBindCandidate.ID, Actor: "late-governance-auditor"}); err == nil || !strings.Contains(err.Error(), "published specification template") {
		t.Errorf("cutover generic output bind without template provenance error=%v", err)
	}

	legacyCandidate := createLegacyBOM(924, "明确 legacy 兼容 BOM")
	if err := publishVersion(legacyCandidate.LatestVersionID); err != nil {
		t.Fatalf("legacy publish compatibility: %v", err)
	}
	if _, err := repo.BindProductProductionBom(ctx, bomapp.BindProductProductionBomCommand{ProductID: 924, BomID: legacyCandidate.ID, Actor: "late-governance-auditor"}); err != nil {
		t.Fatalf("legacy product default bind compatibility: %v", err)
	}
	if _, err := repo.BindProductionBomOutput(ctx, bomapp.BindProductionBomOutputCommand{OutputType: "product", OutputID: 924, BomID: legacyCandidate.ID, Actor: "late-governance-auditor"}); err != nil {
		t.Fatalf("legacy generic output bind compatibility: %v", err)
	}

	for _, productID := range []int64{922, 923} {
		var bindingCount int
		if err := pool.QueryRow(ctx, fmt.Sprintf(`
			SELECT COUNT(*) FROM %s.production_bom_output_bindings
			WHERE output_type='product' AND output_id=$1 AND is_default=true
		`, schema), productID).Scan(&bindingCount); err != nil {
			t.Fatal(err)
		}
		if bindingCount != 0 {
			t.Errorf("rejected governed binding persisted for product %d", productID)
		}
	}
}

func TestArchivedPublishedTemplateProvenanceRemainsValidAcrossPublishAndBindingsPostgres(t *testing.T) {
	ctx, pool, schema := newPR600BomMaintenanceTestDB(t)
	ensurePR600ProductPublishTestTables(t, ctx, pool, schema)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_unit_definitions(code) VALUES('袋') ON CONFLICT(code) DO NOTHING;
		INSERT INTO %s.products(id,name,active) VALUES
			(931,'历史模板来源商品',true),(932,'草稿模板来源商品',true);
		INSERT INTO %s.product_bom_spec_migrations(product_id,state,legacy_catalog_product) VALUES
			(931,'preparing',false),(932,'preparing',false);
	`, schema, schema, schema)); err != nil {
		t.Fatal(err)
	}
	var mainInputMaterialID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.materials(code,name,kind,unit,cost_unit)
		VALUES('PR600-ARCHIVED-SOURCE-MAIN','历史来源主投入','bean','kg','kg') RETURNING id
	`, schema)).Scan(&mainInputMaterialID); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	template, err := repo.CreateProductionBomSpecTemplate(ctx, bomapp.CreateProductionBomSpecTemplateCommand{Name: "历史发布来源模板", Actor: "archive-source-auditor"})
	if err != nil {
		t.Fatal(err)
	}
	templateV1 := template.Versions[0].ID
	variants := []bomapp.ProductionBomSpecTemplateVariant{{
		SpecKey: "archived-source-bag", Name: "历史来源袋装", InventoryUnit: "袋", IsDefault: true,
		Items: []bomapp.ProductionBomSpecTemplateVariantDraftItem{{IsMainInput: true, ProductionBomDraftItem: bomapp.ProductionBomDraftItem{ComponentType: "material", ConsumeUnit: "main_input_unit", QtyPerUnit: 0.227}}},
	}}
	if _, err := repo.UpdateProductionBomSpecTemplateVersionDraft(ctx, bomapp.UpdateProductionBomSpecTemplateVersionDraftCommand{VersionID: templateV1, Variants: variants, Actor: "archive-source-auditor"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.PublishProductionBomSpecTemplateVersion(ctx, bomapp.PublishProductionBomSpecTemplateVersionCommand{VersionID: templateV1, Actor: "archive-source-auditor"}); err != nil {
		t.Fatal(err)
	}
	historicalBOM, err := repo.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{
		Name: "使用 V1 的 BOM", OutputType: "product", OutputID: 931, OutputProductID: 931,
		OutputQty: 1, OutputUnit: "袋", SpecTemplateVersionID: templateV1, MainInputMaterialID: mainInputMaterialID, Actor: "archive-source-auditor",
	})
	if err != nil {
		t.Fatal(err)
	}
	templateV2, err := repo.CreateProductionBomSpecTemplateVersion(ctx, bomapp.CreateProductionBomSpecTemplateVersionCommand{TemplateID: template.ID, SourceVersionID: templateV1, Note: "V2", Actor: "archive-source-auditor"})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.PublishProductionBomSpecTemplateVersion(ctx, bomapp.PublishProductionBomSpecTemplateVersionCommand{VersionID: templateV2.ID, Actor: "archive-source-auditor"}); err != nil {
		t.Fatal(err)
	}
	var v1Status string
	var v1Published bool
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT status,published_at IS NOT NULL FROM %s.production_bom_spec_template_versions WHERE id=$1`, schema), templateV1).Scan(&v1Status, &v1Published); err != nil {
		t.Fatal(err)
	}
	if v1Status != "archived" || !v1Published {
		t.Fatalf("template V1 history=%s/%v want archived/true", v1Status, v1Published)
	}
	if err := publishProductionBOMVersionForMaintenanceTest(ctx, pool, repo, historicalBOM.LatestVersionID, "archive-source-auditor"); err != nil {
		t.Fatalf("publish BOM copied from historically published V1: %v", err)
	}
	if _, err := repo.BindProductProductionBom(ctx, bomapp.BindProductProductionBomCommand{ProductID: 931, BomID: historicalBOM.ID, Actor: "archive-source-auditor"}); err != nil {
		t.Fatalf("BindProduct with archived published source: %v", err)
	}
	if _, err := repo.BindProductionBomOutput(ctx, bomapp.BindProductionBomOutputCommand{OutputType: "product", OutputID: 931, BomID: historicalBOM.ID, Actor: "archive-source-auditor"}); err != nil {
		t.Fatalf("generic output bind with archived published source: %v", err)
	}

	templateV3, err := repo.CreateProductionBomSpecTemplateVersion(ctx, bomapp.CreateProductionBomSpecTemplateVersionCommand{TemplateID: template.ID, SourceVersionID: templateV2.ID, Note: "未发布 V3", Actor: "archive-source-auditor"})
	if err != nil {
		t.Fatal(err)
	}
	draftSourceBOM, err := repo.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{
		Name: "错误草稿来源 BOM", OutputType: "product", OutputID: 932, OutputProductID: 932,
		OutputQty: 1, OutputUnit: "袋", SpecTemplateVersionID: templateV2.ID, MainInputMaterialID: mainInputMaterialID, Actor: "archive-source-auditor",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_versions SET source_spec_template_version_id=$2 WHERE id=$1`, schema), draftSourceBOM.LatestVersionID, templateV3.ID); err != nil {
		t.Fatal(err)
	}
	if err := publishProductionBOMVersionForMaintenanceTest(ctx, pool, repo, draftSourceBOM.LatestVersionID, "archive-source-auditor"); err == nil || !strings.Contains(err.Error(), "published specification template") {
		t.Fatalf("draft template provenance publish error=%v", err)
	}
}

func TestConcurrentProductBOMDraftMutationAndPublishAvoidsDeadlockPostgres(t *testing.T) {
	for _, operation := range []string{"save", "reapply"} {
		t.Run(operation, func(t *testing.T) {
			ctx, pool, schema := newPR600BomMaintenanceTestDB(t)
			ensurePR600ProductPublishTestTables(t, ctx, pool, schema)
			productID := int64(941)
			if operation == "reapply" {
				productID = 942
			}
			if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.product_unit_definitions(code) VALUES('袋') ON CONFLICT(code) DO NOTHING`, schema)); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.products(id,name,active) VALUES($1,$2,true)`, schema), productID, "并发 "+operation); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.product_bom_spec_migrations(product_id,state,legacy_catalog_product) VALUES($1,'preparing',false)`, schema), productID); err != nil {
				t.Fatal(err)
			}
			var mainInputMaterialID int64
			if err := pool.QueryRow(ctx, fmt.Sprintf(`
				INSERT INTO %s.materials(code,name,kind,unit,cost_unit)
				VALUES($1,$2,'bean','kg','kg') RETURNING id
			`, schema), fmt.Sprintf("PR600-CONCURRENT-%d", productID), "并发主投入").Scan(&mainInputMaterialID); err != nil {
				t.Fatal(err)
			}
			repo := NewRepository(pool, schema)
			template, err := repo.CreateProductionBomSpecTemplate(ctx, bomapp.CreateProductionBomSpecTemplateCommand{Name: "并发模板 " + operation, Actor: "concurrency-auditor"})
			if err != nil {
				t.Fatal(err)
			}
			templateVersionID := template.Versions[0].ID
			variant := bomapp.ProductionBomSpecTemplateVariant{
				SpecKey: "concurrent-bag", Name: "并发袋装", InventoryUnit: "袋", IsDefault: true,
				Items: []bomapp.ProductionBomSpecTemplateVariantDraftItem{{IsMainInput: true, ProductionBomDraftItem: bomapp.ProductionBomDraftItem{ComponentType: "material", ConsumeUnit: "main_input_unit", QtyPerUnit: 0.227}}},
			}
			if _, err := repo.UpdateProductionBomSpecTemplateVersionDraft(ctx, bomapp.UpdateProductionBomSpecTemplateVersionDraftCommand{VersionID: templateVersionID, Variants: []bomapp.ProductionBomSpecTemplateVariant{variant}, Actor: "concurrency-auditor"}); err != nil {
				t.Fatal(err)
			}
			if err := repo.PublishProductionBomSpecTemplateVersion(ctx, bomapp.PublishProductionBomSpecTemplateVersionCommand{VersionID: templateVersionID, Actor: "concurrency-auditor"}); err != nil {
				t.Fatal(err)
			}
			createdBOM, err := repo.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{
				Name: "并发 BOM " + operation, OutputType: "product", OutputID: productID, OutputProductID: productID,
				OutputQty: 1, OutputUnit: "袋", SpecTemplateVersionID: templateVersionID, MainInputMaterialID: mainInputMaterialID, Actor: "concurrency-auditor",
			})
			if err != nil {
				t.Fatal(err)
			}
			workerPoolConfig := pool.Config().Copy()
			applicationName := fmt.Sprintf("pr600-bom-%s-%d", operation, time.Now().UnixNano())
			workerPoolConfig.ConnConfig.RuntimeParams["application_name"] = applicationName
			workerPool, err := pgxpool.NewWithConfig(ctx, workerPoolConfig)
			if err != nil {
				t.Fatal(err)
			}
			defer workerPool.Close()
			workerRepo := NewRepository(workerPool, schema)

			publishTx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = publishTx.Rollback(ctx) }()
			if err := lockProductionBomDefaultGraphTx(ctx, publishTx, schema); err != nil {
				t.Fatal(err)
			}
			if _, err := productBOMRequiresSpecGroupTx(ctx, publishTx, schema, productID); err != nil {
				t.Fatal(err)
			}
			workerDone := make(chan error, 1)
			go func() {
				if operation == "reapply" {
					_, err := workerRepo.ReapplyProductionBomSpecTemplateVersion(context.Background(), bomapp.ReapplyProductionBomSpecTemplateVersionCommand{
						VersionID: createdBOM.LatestVersionID, SpecTemplateVersionID: templateVersionID, MainInputMaterialID: mainInputMaterialID, Actor: "concurrency-worker",
					})
					workerDone <- err
					return
				}
				_, err := workerRepo.UpdateProductionBomVersionDraft(context.Background(), bomapp.UpdateProductionBomVersionDraftCommand{
					VersionID: createdBOM.LatestVersionID, Actor: "concurrency-worker",
					Variants: []bomapp.ProductionBomDraftVariant{{
						SpecKey: "concurrent-bag", Name: "并发袋装已保存", InventoryUnit: "袋", IsDefault: true,
						Items: []bomapp.ProductionBomDraftItem{{MaterialID: mainInputMaterialID, ComponentType: "material", ConsumeUnit: "kg", QtyPerUnit: 0.227}},
					}},
				})
				workerDone <- err
			}()
			waitForPR600BOMWorkerAdvisoryLock(t, ctx, pool, applicationName)

			publishCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			publishErr := repo.publishProductionBomVersionTx(publishCtx, publishTx, bomapp.PublishProductionBomVersionCommand{VersionID: createdBOM.LatestVersionID, Actor: "concurrency-publisher"})
			cancel()
			if publishErr == nil {
				publishErr = publishTx.Commit(ctx)
			} else {
				_ = publishTx.Rollback(ctx)
			}
			workerErr := <-workerDone
			for label, gotErr := range map[string]error{"publish": publishErr, "worker": workerErr} {
				if gotErr != nil && (strings.Contains(gotErr.Error(), "40P01") || strings.Contains(strings.ToLower(gotErr.Error()), "deadlock") || strings.Contains(strings.ToLower(gotErr.Error()), "timeout")) {
					t.Fatalf("%s %s deadlock/timeout error=%v", operation, label, gotErr)
				}
			}
			if publishErr != nil {
				t.Fatalf("%s publish error=%v", operation, publishErr)
			}
			if workerErr == nil || (!strings.Contains(workerErr.Error(), "read-only") && !strings.Contains(workerErr.Error(), "not found")) {
				t.Fatalf("%s worker must observe published read-only version, got %v", operation, workerErr)
			}
			var finalStatus string
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.production_bom_versions WHERE id=$1`, schema), createdBOM.LatestVersionID).Scan(&finalStatus); err != nil {
				t.Fatal(err)
			}
			if finalStatus != "published" {
				t.Fatalf("%s final version status=%q want published", operation, finalStatus)
			}
		})
	}
}

func TestCreateProductionBomVersionPreservesSpecTemplateProvenancePostgres(t *testing.T) {
	ctx, pool, schema := newPR600BomMaintenanceTestDB(t)
	ensurePR600ProductPublishTestTables(t, ctx, pool, schema)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_unit_definitions(code) VALUES('袋') ON CONFLICT(code) DO NOTHING;
		INSERT INTO %s.products(id,name,active) VALUES
			(951,'新模型版本复制商品',true),
			(952,'历史兼容版本复制商品',true);
		INSERT INTO %s.product_bom_spec_migrations(product_id,state,legacy_catalog_product) VALUES
			(951,'preparing',false),
			(952,'legacy',true);
	`, schema, schema, schema)); err != nil {
		t.Fatal(err)
	}
	var mainInputMaterialID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.materials(code,name,kind,unit,cost_unit)
		VALUES('PR600-VERSION-PROVENANCE-MAIN','版本复制主投入','bean','kg','kg')
		RETURNING id
	`, schema)).Scan(&mainInputMaterialID); err != nil {
		t.Fatal(err)
	}

	repo := NewRepository(pool, schema)
	template, err := repo.CreateProductionBomSpecTemplate(ctx, bomapp.CreateProductionBomSpecTemplateCommand{Name: "版本复制规格模板", Actor: "version-provenance-auditor"})
	if err != nil {
		t.Fatal(err)
	}
	templateVersionID := template.Versions[0].ID
	if _, err := repo.UpdateProductionBomSpecTemplateVersionDraft(ctx, bomapp.UpdateProductionBomSpecTemplateVersionDraftCommand{
		VersionID: templateVersionID,
		Variants: []bomapp.ProductionBomSpecTemplateVariant{{
			SpecKey: "bag-227", Name: "227g 袋", InventoryUnit: "袋", IsDefault: true,
			Items: []bomapp.ProductionBomSpecTemplateVariantDraftItem{{
				IsMainInput:            true,
				ProductionBomDraftItem: bomapp.ProductionBomDraftItem{ComponentType: "material", ConsumeUnit: "main_input_unit", QtyPerUnit: 0.227},
			}},
		}},
		Actor: "version-provenance-auditor",
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.PublishProductionBomSpecTemplateVersion(ctx, bomapp.PublishProductionBomSpecTemplateVersionCommand{VersionID: templateVersionID, Actor: "version-provenance-auditor"}); err != nil {
		t.Fatal(err)
	}
	governedBOM, err := repo.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{
		Name: "新模型版本复制 BOM", OutputType: "product", OutputID: 951, OutputProductID: 951,
		OutputQty: 1, OutputUnit: "袋", SpecTemplateVersionID: templateVersionID, MainInputMaterialID: mainInputMaterialID, Actor: "version-provenance-auditor",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := publishProductionBOMVersionForMaintenanceTest(ctx, pool, repo, governedBOM.LatestVersionID, "version-provenance-auditor"); err != nil {
		t.Fatalf("publish governed BOM V1: %v", err)
	}
	governedV2, err := repo.CreateProductionBomVersion(ctx, bomapp.CreateProductionBomVersionCommand{
		BomID: governedBOM.ID, SourceVersionID: governedBOM.LatestVersionID, Note: "V2", Actor: "version-provenance-auditor",
	})
	if err != nil {
		t.Fatal(err)
	}
	var copiedTemplateVersionID, copiedMainInputMaterialID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(source_spec_template_version_id,0),COALESCE(main_input_material_id,0)
		FROM %s.production_bom_versions WHERE id=$1
	`, schema), governedV2.ID).Scan(&copiedTemplateVersionID, &copiedMainInputMaterialID); err != nil {
		t.Fatal(err)
	}
	if copiedTemplateVersionID != templateVersionID || copiedMainInputMaterialID != mainInputMaterialID {
		t.Errorf("governed BOM V2 provenance=(%d,%d), want (%d,%d)", copiedTemplateVersionID, copiedMainInputMaterialID, templateVersionID, mainInputMaterialID)
	}
	if err := publishProductionBOMVersionForMaintenanceTest(ctx, pool, repo, governedV2.ID, "version-provenance-auditor"); err != nil {
		t.Errorf("publish governed BOM V2 copied from valid V1: %v", err)
	}

	legacyBOM, err := repo.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{
		Name: "历史兼容版本复制 BOM", OutputType: "product", OutputID: 952, OutputProductID: 952,
		OutputQty: 1, OutputUnit: "袋", Actor: "version-provenance-auditor",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := publishProductionBOMVersionForMaintenanceTest(ctx, pool, repo, legacyBOM.LatestVersionID, "version-provenance-auditor"); err != nil {
		t.Fatalf("publish legacy BOM V1: %v", err)
	}
	legacyV2, err := repo.CreateProductionBomVersion(ctx, bomapp.CreateProductionBomVersionCommand{
		BomID: legacyBOM.ID, SourceVersionID: legacyBOM.LatestVersionID, Note: "V2", Actor: "version-provenance-auditor",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := publishProductionBOMVersionForMaintenanceTest(ctx, pool, repo, legacyV2.ID, "version-provenance-auditor"); err != nil {
		t.Fatalf("legacy BOM V2 without template provenance must remain compatible: %v", err)
	}
}

func TestPublishAlternateNonDefaultProductBOMSkipsDefaultSwitchGuardPostgres(t *testing.T) {
	fixture := newPR600DefaultSwitchFixture(t)
	if err := publishProductionBOMVersionForMaintenanceTest(fixture.ctx, fixture.pool, fixture.repo, fixture.first.LatestVersionID, "alternate-publish-auditor"); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.repo.BindProductionBomOutput(fixture.ctx, bomapp.BindProductionBomOutputCommand{
		OutputType: "product", OutputID: fixture.productID, BomID: fixture.first.ID, Actor: "alternate-publish-auditor",
	}); err != nil {
		t.Fatal(err)
	}
	fixture.insertOldDefaultBlockers(t)

	if err := publishProductionBOMVersionForMaintenanceTest(fixture.ctx, fixture.pool, fixture.repo, fixture.second.LatestVersionID, "alternate-publish-auditor"); err != nil {
		t.Fatalf("publishing a non-default alternate BOM must not attempt a default switch: %v", err)
	}
	var defaultBOMID int64
	if err := fixture.pool.QueryRow(fixture.ctx, fmt.Sprintf(`
		SELECT bom_id FROM %s.production_bom_output_bindings
		WHERE output_type='product' AND output_id=$1 AND is_default=true
	`, fixture.schema), fixture.productID).Scan(&defaultBOMID); err != nil {
		t.Fatal(err)
	}
	if defaultBOMID != fixture.first.ID {
		t.Fatalf("alternate publish changed default BOM to %d, want %d", defaultBOMID, fixture.first.ID)
	}
}

func TestBindProductionBomOutputEnforcesInventoryAndOrderDefaultSwitchBlockersPostgres(t *testing.T) {
	fixture := newPR600DefaultSwitchFixture(t)
	for _, versionID := range []int64{fixture.first.LatestVersionID, fixture.second.LatestVersionID} {
		if err := publishProductionBOMVersionForMaintenanceTest(fixture.ctx, fixture.pool, fixture.repo, versionID, "generic-bind-auditor"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := fixture.repo.BindProductionBomOutput(fixture.ctx, bomapp.BindProductionBomOutputCommand{
		OutputType: "product", OutputID: fixture.productID, BomID: fixture.first.ID, Actor: "generic-bind-auditor",
	}); err != nil {
		t.Fatal(err)
	}
	fixture.insertOldDefaultBlockers(t)

	_, err := fixture.repo.BindProductionBomOutput(fixture.ctx, bomapp.BindProductionBomOutputCommand{
		OutputType: "product", OutputID: fixture.productID, BomID: fixture.second.ID, Actor: "generic-bind-auditor",
	})
	var blocked *productspecmigrationapp.DefaultBOMSwitchBlockedError
	if !errors.As(err, &blocked) {
		t.Fatalf("generic output bind error=%v, want default BOM switch blocker", err)
	}
	blockerCodes := make(map[string]bool, len(blocked.Blockers))
	for _, blocker := range blocked.Blockers {
		blockerCodes[blocker.Code] = true
	}
	for _, code := range []string{"old_bom_stock", "old_bom_unfinished_orders"} {
		if !blockerCodes[code] {
			t.Errorf("generic output bind blockers=%+v, want %s", blocked.Blockers, code)
		}
	}
	var defaultBOMID int64
	if err := fixture.pool.QueryRow(fixture.ctx, fmt.Sprintf(`
		SELECT bom_id FROM %s.production_bom_output_bindings
		WHERE output_type='product' AND output_id=$1 AND is_default=true
	`, fixture.schema), fixture.productID).Scan(&defaultBOMID); err != nil {
		t.Fatal(err)
	}
	if defaultBOMID != fixture.first.ID {
		t.Fatalf("blocked generic bind changed default BOM to %d, want %d", defaultBOMID, fixture.first.ID)
	}
}

func TestUpdateProductionBomUnchangedPublishedOutputSkipsDraftProvenancePostgres(t *testing.T) {
	ctx, pool, schema := newPR600BomMaintenanceTestDB(t)
	ensurePR600ProductPublishTestTables(t, ctx, pool, schema)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_unit_definitions(code) VALUES('袋') ON CONFLICT(code) DO NOTHING;
		INSERT INTO %s.products(id,name,active) VALUES(971,'已发布可改名商品',true);
		INSERT INTO %s.product_bom_spec_migrations(product_id,state,legacy_catalog_product)
		VALUES(971,'preparing',false);
	`, schema, schema, schema)); err != nil {
		t.Fatal(err)
	}
	var mainInputMaterialID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.materials(code,name,kind,unit,cost_unit)
		VALUES('PR600-UPDATE-MAIN','已发布改名主投入','bean','kg','kg') RETURNING id
	`, schema)).Scan(&mainInputMaterialID); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	template, err := repo.CreateProductionBomSpecTemplate(ctx, bomapp.CreateProductionBomSpecTemplateCommand{Name: "已发布改名规格模板", Actor: "published-update-auditor"})
	if err != nil {
		t.Fatal(err)
	}
	templateVersionID := template.Versions[0].ID
	if _, err := repo.UpdateProductionBomSpecTemplateVersionDraft(ctx, bomapp.UpdateProductionBomSpecTemplateVersionDraftCommand{
		VersionID: templateVersionID,
		Variants: []bomapp.ProductionBomSpecTemplateVariant{{
			SpecKey: "bag", Name: "袋装", InventoryUnit: "袋", IsDefault: true,
			Items: []bomapp.ProductionBomSpecTemplateVariantDraftItem{{
				IsMainInput:            true,
				ProductionBomDraftItem: bomapp.ProductionBomDraftItem{ComponentType: "material", ConsumeUnit: "main_input_unit", QtyPerUnit: 1},
			}},
		}},
		Actor: "published-update-auditor",
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.PublishProductionBomSpecTemplateVersion(ctx, bomapp.PublishProductionBomSpecTemplateVersionCommand{VersionID: templateVersionID, Actor: "published-update-auditor"}); err != nil {
		t.Fatal(err)
	}
	created, err := repo.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{
		Name: "已发布原名称", OutputType: "product", OutputID: 971, OutputProductID: 971,
		OutputQty: 1, OutputUnit: "袋", SpecTemplateVersionID: templateVersionID, MainInputMaterialID: mainInputMaterialID, Actor: "published-update-auditor",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := publishProductionBOMVersionForMaintenanceTest(ctx, pool, repo, created.LatestVersionID, "published-update-auditor"); err != nil {
		t.Fatal(err)
	}
	updated, err := repo.UpdateProductionBom(ctx, bomapp.UpdateProductionBomCommand{
		ID: created.ID, Name: "已发布新名称", OutputType: "product", OutputID: 971, OutputProductID: 971,
		UpdateOutputBinding: true, Status: "inactive", Actor: "published-update-auditor",
	})
	if err != nil {
		t.Fatalf("unchanged published output identity name/status update: %v", err)
	}
	if updated.Name != "已发布新名称" || updated.Status != "inactive" {
		t.Fatalf("updated published BOM=%+v, want changed name/status", updated)
	}
}

type pr600DefaultSwitchFixture struct {
	ctx       context.Context
	pool      *pgxpool.Pool
	schema    string
	repo      Repository
	productID int64
	first     bomapp.ProductionBomSummary
	second    bomapp.ProductionBomSummary
	firstSpec int64
}

func newPR600DefaultSwitchFixture(t *testing.T) pr600DefaultSwitchFixture {
	t.Helper()
	ctx, pool, schema := newPR600BomMaintenanceTestDB(t)
	ensurePR600ProductPublishTestTables(t, ctx, pool, schema)
	const productID int64 = 961
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.product_unit_definitions(code) VALUES('袋') ON CONFLICT(code) DO NOTHING`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.products(id,name,active) VALUES($1,'默认 BOM 切换商品',true)`, schema), productID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_bom_spec_migrations(product_id,state,legacy_catalog_product)
		VALUES($1,'cutover',true)
	`, schema), productID); err != nil {
		t.Fatal(err)
	}
	var mainInputMaterialID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.materials(code,name,kind,unit,cost_unit)
		VALUES('PR600-SWITCH-MAIN','默认切换主投入','bean','kg','kg') RETURNING id
	`, schema)).Scan(&mainInputMaterialID); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	template, err := repo.CreateProductionBomSpecTemplate(ctx, bomapp.CreateProductionBomSpecTemplateCommand{Name: "默认切换规格模板", Actor: "default-switch-fixture"})
	if err != nil {
		t.Fatal(err)
	}
	templateVersionID := template.Versions[0].ID
	if _, err := repo.UpdateProductionBomSpecTemplateVersionDraft(ctx, bomapp.UpdateProductionBomSpecTemplateVersionDraftCommand{
		VersionID: templateVersionID,
		Variants: []bomapp.ProductionBomSpecTemplateVariant{{
			SpecKey: "bag", Name: "袋装", InventoryUnit: "袋", IsDefault: true,
			Items: []bomapp.ProductionBomSpecTemplateVariantDraftItem{{
				IsMainInput:            true,
				ProductionBomDraftItem: bomapp.ProductionBomDraftItem{ComponentType: "material", ConsumeUnit: "main_input_unit", QtyPerUnit: 1},
			}},
		}},
		Actor: "default-switch-fixture",
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.PublishProductionBomSpecTemplateVersion(ctx, bomapp.PublishProductionBomSpecTemplateVersionCommand{VersionID: templateVersionID, Actor: "default-switch-fixture"}); err != nil {
		t.Fatal(err)
	}
	createBOM := func(name string) bomapp.ProductionBomSummary {
		created, err := repo.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{
			Name: name, OutputType: "product", OutputID: productID, OutputProductID: productID,
			OutputQty: 1, OutputUnit: "袋", SpecTemplateVersionID: templateVersionID, MainInputMaterialID: mainInputMaterialID, Actor: "default-switch-fixture",
		})
		if err != nil {
			t.Fatal(err)
		}
		return created
	}
	first := createBOM("默认 BOM A")
	second := createBOM("备用 BOM B")
	var firstSpec int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_bom_specs WHERE bom_id=$1 AND spec_key='bag'`, schema), first.ID).Scan(&firstSpec); err != nil {
		t.Fatal(err)
	}
	return pr600DefaultSwitchFixture{ctx: ctx, pool: pool, schema: schema, repo: repo, productID: productID, first: first, second: second, firstSpec: firstSpec}
}

func (fixture pr600DefaultSwitchFixture) insertOldDefaultBlockers(t *testing.T) {
	t.Helper()
	if _, err := fixture.pool.Exec(fixture.ctx, fmt.Sprintf(`
		CREATE TABLE %s.finished_inventory(
			product_id BIGINT NOT NULL,bom_spec_id BIGINT NOT NULL,bom_variant_id BIGINT NOT NULL DEFAULT 0,
			spec_g BIGINT NOT NULL DEFAULT 0,warehouse TEXT NOT NULL DEFAULT 'finished_goods',
			onhand_units BIGINT NOT NULL DEFAULT 0,onhand_loose_g BIGINT NOT NULL DEFAULT 0
		);
		CREATE TABLE %s.orders(
			id BIGINT PRIMARY KEY,ship_status_id BIGINT NOT NULL DEFAULT 0,is_void BOOLEAN NOT NULL DEFAULT false
		);
		CREATE TABLE %s.order_items(
			order_id BIGINT NOT NULL,product_id BIGINT NOT NULL,bom_spec_id BIGINT NOT NULL
		);
	`, fixture.schema, fixture.schema, fixture.schema)); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(fixture.ctx, fmt.Sprintf(`
		INSERT INTO %s.finished_inventory(product_id,bom_spec_id,onhand_units)
		VALUES($1,$2,1)
	`, fixture.schema), fixture.productID, fixture.firstSpec); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(fixture.ctx, fmt.Sprintf(`INSERT INTO %s.orders(id) VALUES(1)`, fixture.schema)); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.pool.Exec(fixture.ctx, fmt.Sprintf(`
		INSERT INTO %s.order_items(order_id,product_id,bom_spec_id)
		VALUES(1,$1,$2)
	`, fixture.schema), fixture.productID, fixture.firstSpec); err != nil {
		t.Fatal(err)
	}
}

func ensurePR600ProductPublishTestTables(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) {
	t.Helper()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		ALTER TABLE %s.products ADD COLUMN IF NOT EXISTS parent_product_id BIGINT NOT NULL DEFAULT 0;
		CREATE TABLE IF NOT EXISTS %s.product_production_configs(
			product_id BIGINT PRIMARY KEY,
			production_bom_id BIGINT NOT NULL DEFAULT 0,
			production_bom_version_id BIGINT NOT NULL DEFAULT 0,
			process_route_id BIGINT NOT NULL DEFAULT 0,
			industry_field_template_id BIGINT NOT NULL DEFAULT 0,
			expected_loss_rate NUMERIC(10,4) NOT NULL DEFAULT 0,
			note TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			created_by TEXT NOT NULL DEFAULT '',
			updated_by TEXT NOT NULL DEFAULT ''
		);
	`, schema, schema)); err != nil {
		t.Fatal(err)
	}
}

func publishProductionBOMVersionForMaintenanceTest(ctx context.Context, pool *pgxpool.Pool, repo Repository, versionID int64, actor string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := repo.publishProductionBomVersionTx(ctx, tx, bomapp.PublishProductionBomVersionCommand{VersionID: versionID, Actor: actor}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func waitForPR600BOMWorkerAdvisoryLock(t *testing.T, ctx context.Context, pool *pgxpool.Pool, applicationName string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var waiting bool
		if err := pool.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM pg_stat_activity
				WHERE application_name=$1 AND wait_event_type='Lock' AND wait_event='advisory'
			)
		`, applicationName).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("worker %s did not reach product advisory lock", applicationName)
}

func newPR600BomMaintenanceTestDB(t *testing.T) (context.Context, *pgxpool.Pool, string) {
	t.Helper()
	ctx, pool, schema := newPR600SpecTemplatePublishTestDB(t)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %s.product_bom_spec_migrations(
			product_id BIGINT PRIMARY KEY,
			state TEXT NOT NULL,
			legacy_catalog_product BOOLEAN NOT NULL
		);
		CREATE TABLE %s.business_group_assignments(
			id BIGSERIAL PRIMARY KEY,
			group_id BIGINT NOT NULL DEFAULT 0,
			group_item_id BIGINT NOT NULL DEFAULT 0,
			usage_key TEXT NOT NULL DEFAULT '',
			object_key TEXT NOT NULL DEFAULT '',
			object_id BIGINT NOT NULL DEFAULT 0,
			object_ref TEXT NOT NULL DEFAULT '',
			sort_order INT NOT NULL DEFAULT 0,
			created_by TEXT NOT NULL DEFAULT '',
			updated_by TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE %s.business_groups(id BIGINT PRIMARY KEY,name TEXT NOT NULL DEFAULT '');
		CREATE TABLE %s.business_group_items(id BIGINT PRIMARY KEY,name TEXT NOT NULL DEFAULT '');
		CREATE TABLE %s.process_routes(id BIGINT PRIMARY KEY,name TEXT NOT NULL DEFAULT '');
		CREATE TABLE %s.product_unit_definitions(
			code TEXT PRIMARY KEY,name TEXT NOT NULL DEFAULT '',active BOOLEAN NOT NULL DEFAULT true,deleted_at TIMESTAMPTZ
		);
	`, schema, schema, schema, schema, schema, schema)); err != nil {
		t.Fatal(err)
	}
	return ctx, pool, schema
}
