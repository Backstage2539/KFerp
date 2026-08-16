package bom

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	bomapp "orderapp/internal/application/bom"
	catalogapp "orderapp/internal/application/catalog"
	postgrescatalog "orderapp/internal/infrastructure/postgres/catalog"
	postgresmaterials "orderapp/internal/infrastructure/postgres/materials"
	supporthttp "orderapp/internal/interfaces/http/support"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPR598MaterialOutputRepositoryAndCompatibilityMigrationPostgres(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	schema := fmt.Sprintf("pr598_bom_%d_%d", os.Getpid(), time.Now().UnixNano())
	mustPR598SQL(t, ctx, pool, "CREATE SCHEMA "+schema)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })

	mustPR598SQL(t, ctx, pool, fmt.Sprintf(`
		CREATE TABLE %[1]s.products(
			id BIGINT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			active BOOLEAN NOT NULL DEFAULT true,
			base_product_id BIGINT NOT NULL DEFAULT 0
		);
	`, schema))
	if err := supporthttp.EnsureAuditTables(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if err := postgresmaterials.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	var outputConstraintOID int64
	var outputConstraintValidated bool
	if err := pool.QueryRow(ctx, `
		SELECT c.oid::bigint, c.convalidated
		FROM pg_constraint c
		JOIN pg_class t ON t.oid=c.conrelid
		JOIN pg_namespace n ON n.oid=t.relnamespace
		WHERE n.nspname=$1 AND t.relname='production_boms' AND c.conname='production_boms_output_binding_check'
	`, schema).Scan(&outputConstraintOID, &outputConstraintValidated); err != nil {
		t.Fatal(err)
	}
	if !outputConstraintValidated {
		t.Fatal("production_boms_output_binding_check must be validated")
	}
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	var outputConstraintOIDAfter int64
	if err := pool.QueryRow(ctx, `
		SELECT c.oid::bigint
		FROM pg_constraint c
		JOIN pg_class t ON t.oid=c.conrelid
		JOIN pg_namespace n ON n.oid=t.relnamespace
		WHERE n.nspname=$1 AND t.relname='production_boms' AND c.conname='production_boms_output_binding_check'
	`, schema).Scan(&outputConstraintOIDAfter); err != nil {
		t.Fatal(err)
	}
	if outputConstraintOIDAfter != outputConstraintOID {
		t.Fatalf("output binding constraint was recreated: oid %d -> %d", outputConstraintOID, outputConstraintOIDAfter)
	}
	mustPR598SQL(t, ctx, pool, fmt.Sprintf(`
		CREATE TABLE %[1]s.business_groups(id BIGINT PRIMARY KEY, name TEXT NOT NULL DEFAULT '');
		CREATE TABLE %[1]s.business_group_items(id BIGINT PRIMARY KEY, name TEXT NOT NULL DEFAULT '');
		CREATE TABLE %[1]s.business_group_assignments(
			id BIGSERIAL PRIMARY KEY,
			group_id BIGINT NOT NULL DEFAULT 0,
			group_item_id BIGINT NOT NULL DEFAULT 0,
			usage_key TEXT NOT NULL DEFAULT '',
			object_key TEXT NOT NULL DEFAULT '',
			object_id BIGINT NOT NULL DEFAULT 0,
			object_ref TEXT NOT NULL DEFAULT '',
			sort_order INTEGER NOT NULL DEFAULT 100,
			created_by TEXT NOT NULL DEFAULT '',
			updated_by TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE %[1]s.process_routes(id BIGINT PRIMARY KEY, name TEXT NOT NULL DEFAULT '');
	`, schema))

	var inputMaterialID, outputMaterialID, alternateMaterialID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.materials(code,name,kind,unit,cost_unit,is_semi_finished) VALUES('RAW-1','鲜豆','bean','kg','kg',false) RETURNING id`, schema)).Scan(&inputMaterialID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.materials(code,name,kind,unit,cost_unit,is_semi_finished) VALUES('WIP-1','湿豆','bean','kg','kg',false) RETURNING id`, schema)).Scan(&outputMaterialID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.materials(code,name,kind,unit,cost_unit,is_semi_finished) VALUES('WIP-2','生豆','bean','kg','kg',true) RETURNING id`, schema)).Scan(&alternateMaterialID); err != nil {
		t.Fatal(err)
	}

	repo := NewRepository(pool, schema)
	first := createAndPublishPR598MaterialBom(t, ctx, repo, outputMaterialID, inputMaterialID, "湿豆主配方")
	var firstDefaultAuditCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.audit_logs WHERE entity_type='material' AND entity_id=$1 AND action='set_default_production_bom'`, schema), outputMaterialID).Scan(&firstDefaultAuditCount); err != nil {
		t.Fatal(err)
	}
	if firstDefaultAuditCount != 1 {
		t.Fatalf("first material publish default audit count = %d, want 1", firstDefaultAuditCount)
	}
	resolved, err := repo.ResolveDefaultPublishedOutputBom(ctx, "material", outputMaterialID)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.BomID != first.ID || resolved.BomVersionID != first.LatestVersionID || !resolved.IsDefault {
		t.Fatalf("first default binding = %+v, bom=%+v", resolved, first)
	}

	filtered, err := repo.ListProductionBomsFiltered(ctx, bomapp.ProductionBomFilter{OutputType: "material", OutputID: outputMaterialID, ComponentType: "material", ComponentID: inputMaterialID})
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 1 || filtered[0].OutputType != "material" || filtered[0].OutputID != outputMaterialID || filtered[0].OutputMaterialName != "湿豆" || filtered[0].OutputUnit != "kg" {
		t.Fatalf("filtered material BOMs = %+v", filtered)
	}

	second := createAndPublishPR598MaterialBom(t, ctx, repo, outputMaterialID, inputMaterialID, "湿豆备用配方")
	switched, err := repo.BindProductionBomOutput(ctx, bomapp.BindProductionBomOutputCommand{OutputType: "material", OutputID: outputMaterialID, BomID: second.ID, Actor: "pr598-test"})
	if err != nil {
		t.Fatal(err)
	}
	if switched.BomID != second.ID || switched.BomVersionID != second.LatestVersionID {
		t.Fatalf("switched binding = %+v", switched)
	}
	resolved, err = repo.ResolveDefaultPublishedOutputBom(ctx, "material", outputMaterialID)
	if err != nil || resolved.BomID != second.ID {
		t.Fatalf("resolved switched default = %+v err=%v", resolved, err)
	}
	var bindingCount, auditCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.production_bom_output_bindings WHERE output_type='material' AND output_id=$1 AND is_default=true`, schema), outputMaterialID).Scan(&bindingCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.audit_logs WHERE entity_type='material' AND entity_id=$1 AND action='set_default_production_bom'`, schema), outputMaterialID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if bindingCount != 1 || auditCount != 2 {
		t.Fatalf("default/audit count = %d/%d, want 1/2 (automatic first default plus explicit switch)", bindingCount, auditCount)
	}

	_, err = repo.UpdateProductionBom(ctx, bomapp.UpdateProductionBomCommand{ID: first.ID, Name: first.Name, OutputType: "material", OutputID: alternateMaterialID, OutputMaterialID: alternateMaterialID, UpdateOutputBinding: true, Status: "active", Actor: "pr598-test"})
	if err == nil || !strings.Contains(err.Error(), "output identity is immutable") {
		t.Fatalf("published output identity update error = %v", err)
	}

	// Legacy product bindings remain available and are backfilled into the unified resolver.
	mustPR598SQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.products(id,name,active,base_product_id) VALUES(7001,'旧商品',true,0);
		INSERT INTO %[1]s.production_boms(code,name,output_type,output_product_id,output_material_id,status,legacy_product_id)
		VALUES('BOM-LEGACY-7001','旧商品配方','product',7001,0,'active',7001);
		INSERT INTO %[1]s.production_bom_versions(bom_id,version_no,status,output_qty,output_unit,published_at)
		SELECT id,'V001','published',1,'kg',now() FROM %[1]s.production_boms WHERE code='BOM-LEGACY-7001';
		INSERT INTO %[1]s.product_production_bom_bindings(product_id,bom_id,bom_version_id,bound_by)
		SELECT 7001,pb.id,v.id,'legacy-test' FROM %[1]s.production_boms pb JOIN %[1]s.production_bom_versions v ON v.bom_id=pb.id WHERE pb.code='BOM-LEGACY-7001';
		DELETE FROM %[1]s.production_bom_output_bindings WHERE output_type='product' AND output_id=7001;
	`, schema))
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	legacy, err := repo.ResolveDefaultPublishedOutputBom(ctx, "product", 7001)
	if err != nil || legacy.OutputID != 7001 || legacy.BomID <= 0 || legacy.BomVersionID <= 0 {
		t.Fatalf("legacy product binding migration = %+v err=%v", legacy, err)
	}

	// Product production settings must keep the legacy and typed default
	// resolvers in sync in the same save transaction.
	mustPR598SQL(t, ctx, pool, fmt.Sprintf(`
		CREATE TABLE %[1]s.product_production_configs(
			product_id BIGINT PRIMARY KEY, production_bom_id BIGINT NOT NULL DEFAULT 0,
			production_bom_version_id BIGINT NOT NULL DEFAULT 0, process_route_id BIGINT NOT NULL DEFAULT 0,
			industry_field_template_id BIGINT NOT NULL DEFAULT 0, expected_loss_rate NUMERIC(10,4) NOT NULL DEFAULT 0,
			note TEXT NOT NULL DEFAULT '', created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			created_by TEXT NOT NULL DEFAULT '', updated_by TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE %[1]s.product_production_config_industry_templates(
			product_id BIGINT NOT NULL, template_id BIGINT NOT NULL, sort_order INT NOT NULL DEFAULT 1,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			created_by TEXT NOT NULL DEFAULT '', updated_by TEXT NOT NULL DEFAULT '', PRIMARY KEY(product_id,template_id)
		);
		CREATE TABLE %[1]s.product_production_config_fields(
			id BIGSERIAL PRIMARY KEY, product_id BIGINT NOT NULL, field_key TEXT NOT NULL DEFAULT '', label TEXT NOT NULL DEFAULT '',
			field_type TEXT NOT NULL DEFAULT 'text', unit TEXT NOT NULL DEFAULT '', value_text TEXT NOT NULL DEFAULT '',
			value_number NUMERIC(14,4), value_bool BOOLEAN, template_field_key TEXT NOT NULL DEFAULT '', required BOOLEAN NOT NULL DEFAULT false,
			options_json JSONB NOT NULL DEFAULT '[]'::jsonb, show_in_price_list BOOLEAN NOT NULL DEFAULT true, sort_order INT NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		DELETE FROM %[1]s.production_bom_output_bindings WHERE output_type='product' AND output_id=7001;
	`, schema))
	if _, err := postgrescatalog.NewRepository(pool, schema).SaveProductProductionConfig(ctx, catalogapp.SaveProductProductionConfigCommand{
		Actor: "pr598-catalog-test", ProductID: 7001, ProductionBomID: legacy.BomID, ProductionBomVersionID: legacy.BomVersionID,
	}); err != nil {
		t.Fatal(err)
	}
	var catalogUnifiedCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %s.production_bom_output_bindings
		WHERE output_type='product' AND output_id=7001 AND bom_id=$1 AND bom_version_id=$2 AND is_default=true
	`, schema), legacy.BomID, legacy.BomVersionID).Scan(&catalogUnifiedCount); err != nil {
		t.Fatal(err)
	}
	if catalogUnifiedCount != 1 {
		t.Fatalf("catalog save unified binding count = %d, want 1", catalogUnifiedCount)
	}

	// Publish validates the current output target and its inventory unit; the
	// semi-finished display flag is deliberately not part of either rule.
	validationBom, err := repo.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{Name: "生豆发布校验", OutputType: "material", OutputID: alternateMaterialID, OutputMaterialID: alternateMaterialID, OutputQty: 1, OutputUnit: "g", Actor: "pr598-test"})
	if err != nil {
		t.Fatal(err)
	}
	validationItems := []bomapp.ProductionBomDraftItem{{ComponentType: "material", MaterialID: inputMaterialID, ConsumeUnit: "ratio_pct", RatioPct: 100}}
	if _, err := repo.UpdateProductionBomVersionDraft(ctx, bomapp.UpdateProductionBomVersionDraftCommand{VersionID: validationBom.LatestVersionID, OutputQty: 1, OutputUnit: "g", Items: validationItems, Actor: "pr598-test"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.ValidateProductionBomVersionForPublish(ctx, bomapp.PublishProductionBomVersionCommand{VersionID: validationBom.LatestVersionID}); err == nil || !strings.Contains(err.Error(), "output_unit must match") {
		t.Fatalf("material output unit validation error = %v", err)
	}
	if _, err := repo.UpdateProductionBomVersionDraft(ctx, bomapp.UpdateProductionBomVersionDraftCommand{VersionID: validationBom.LatestVersionID, OutputQty: 1, OutputUnit: "kg", Items: validationItems, Actor: "pr598-test"}); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.materials SET deprecated_at=now() WHERE id=$1`, schema), alternateMaterialID); err != nil {
		t.Fatal(err)
	}
	if err := repo.ValidateProductionBomVersionForPublish(ctx, bomapp.PublishProductionBomVersionCommand{VersionID: validationBom.LatestVersionID}); err == nil || !strings.Contains(err.Error(), "output material is inactive") {
		t.Fatalf("inactive material output validation error = %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.materials SET deprecated_at=NULL WHERE id=$1`, schema), alternateMaterialID); err != nil {
		t.Fatal(err)
	}

	// A draft cannot publish when a referenced component has been deprecated or
	// deleted, even when its consume unit does not require unit matching.
	componentBom, err := repo.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{Name: "组件有效性校验", OutputType: "material", OutputID: alternateMaterialID, OutputMaterialID: alternateMaterialID, OutputQty: 1, OutputUnit: "kg", Actor: "pr598-test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpdateProductionBomVersionDraft(ctx, bomapp.UpdateProductionBomVersionDraftCommand{VersionID: componentBom.LatestVersionID, OutputQty: 1, OutputUnit: "kg", Items: []bomapp.ProductionBomDraftItem{{ComponentType: "material", MaterialID: inputMaterialID, ConsumeUnit: "ratio_pct", RatioPct: 100}}, Actor: "pr598-test"}); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.materials SET deprecated_at=now() WHERE id=$1`, schema), inputMaterialID); err != nil {
		t.Fatal(err)
	}
	if err := repo.ValidateProductionBomVersionForPublish(ctx, bomapp.PublishProductionBomVersionCommand{VersionID: componentBom.LatestVersionID}); err == nil || !strings.Contains(err.Error(), "component material is inactive") {
		t.Fatalf("inactive component validation error = %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.materials SET deprecated_at=NULL WHERE id=$1`, schema), inputMaterialID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_version_items SET material_id=999999999 WHERE version_id=$1`, schema), componentBom.LatestVersionID); err != nil {
		t.Fatal(err)
	}
	if err := repo.ValidateProductionBomVersionForPublish(ctx, bomapp.PublishProductionBomVersionCommand{VersionID: componentBom.LatestVersionID}); err == nil || !strings.Contains(err.Error(), "component material not found") {
		t.Fatalf("missing component validation error = %v", err)
	}
	mustPR598SQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.products(id,name,active,base_product_id) VALUES(7003,'停用组件商品',false,0);
		UPDATE %[1]s.production_bom_version_items
		SET component_type='product', material_id=0, component_product_id=7003, consume_unit='fixed_qty', qty_per_unit=1, ratio_pct=0
		WHERE version_id=%[2]d;
	`, schema, componentBom.LatestVersionID))
	if err := repo.ValidateProductionBomVersionForPublish(ctx, bomapp.PublishProductionBomVersionCommand{VersionID: componentBom.LatestVersionID}); err == nil || !strings.Contains(err.Error(), "component product is inactive") {
		t.Fatalf("inactive product component validation error = %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_version_items SET component_product_id=999999998 WHERE version_id=$1`, schema), componentBom.LatestVersionID); err != nil {
		t.Fatal(err)
	}
	if err := repo.ValidateProductionBomVersionForPublish(ctx, bomapp.PublishProductionBomVersionCommand{VersionID: componentBom.LatestVersionID}); err == nil || !strings.Contains(err.Error(), "component product not found") {
		t.Fatalf("missing product component validation error = %v", err)
	}

	// Typed self-reference is rejected for material outputs.
	selfBom, err := repo.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{Name: "生豆自引用", OutputType: "material", OutputID: alternateMaterialID, OutputMaterialID: alternateMaterialID, OutputQty: 1, OutputUnit: "kg", Actor: "pr598-test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpdateProductionBomVersionDraft(ctx, bomapp.UpdateProductionBomVersionDraftCommand{VersionID: selfBom.LatestVersionID, OutputQty: 1, OutputUnit: "kg", Items: []bomapp.ProductionBomDraftItem{{ComponentType: "material", MaterialID: alternateMaterialID, ConsumeUnit: "ratio_pct", RatioPct: 100}}, Actor: "pr598-test"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.ValidateProductionBomVersionForPublish(ctx, bomapp.PublishProductionBomVersionCommand{VersionID: selfBom.LatestVersionID}); err == nil || !strings.Contains(err.Error(), "typed self-reference") {
		t.Fatalf("typed self-reference validation error = %v", err)
	}

	// Cross-type cycle: material -> product -> the same material.
	mustPR598SQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.products(id,name,active,base_product_id) VALUES(7002,'循环商品',true,0);
		INSERT INTO %[1]s.production_boms(code,name,output_type,output_product_id,output_material_id,status)
		VALUES('BOM-CYCLE-PRODUCT','循环商品配方','product',7002,0,'active');
		INSERT INTO %[1]s.production_bom_versions(bom_id,version_no,status,output_qty,output_unit,published_at)
		SELECT id,'V001','published',1,'kg',now() FROM %[1]s.production_boms WHERE code='BOM-CYCLE-PRODUCT';
		INSERT INTO %[1]s.production_bom_version_items(version_id,material_id,component_type,consume_unit,ratio_pct)
		SELECT v.id,%[2]d,'material','ratio_pct',100 FROM %[1]s.production_bom_versions v JOIN %[1]s.production_boms pb ON pb.id=v.bom_id WHERE pb.code='BOM-CYCLE-PRODUCT';
		INSERT INTO %[1]s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default,updated_by)
		SELECT 'product',7002,pb.id,v.id,true,'cycle-test' FROM %[1]s.production_boms pb JOIN %[1]s.production_bom_versions v ON v.bom_id=pb.id WHERE pb.code='BOM-CYCLE-PRODUCT';
	`, schema, alternateMaterialID))
	cycleBom, err := repo.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{Name: "跨类型循环", OutputType: "material", OutputID: alternateMaterialID, OutputMaterialID: alternateMaterialID, OutputQty: 1, OutputUnit: "kg", Actor: "pr598-test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpdateProductionBomVersionDraft(ctx, bomapp.UpdateProductionBomVersionDraftCommand{VersionID: cycleBom.LatestVersionID, OutputQty: 1, OutputUnit: "kg", Items: []bomapp.ProductionBomDraftItem{{ComponentType: "product", ComponentProductID: 7002, ConsumeUnit: "fixed_qty", QtyPerUnit: 1}}, Actor: "pr598-test"}); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_boms SET status='inactive' WHERE code='BOM-CYCLE-PRODUCT'`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := repo.ValidateProductionBomVersionForPublish(ctx, bomapp.PublishProductionBomVersionCommand{VersionID: cycleBom.LatestVersionID}); err != nil {
		t.Fatalf("inactive default BOM must not participate in cycle graph: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_boms SET status='active' WHERE code='BOM-CYCLE-PRODUCT'`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := repo.ValidateProductionBomVersionForPublish(ctx, bomapp.PublishProductionBomVersionCommand{VersionID: cycleBom.LatestVersionID}); err == nil || !strings.Contains(err.Error(), "cycle detected") {
		t.Fatalf("cross typed cycle validation error = %v", err)
	}

	// Two individually-valid opposite drafts must not both publish. The atomic
	// repository operation serializes graph validation and publication so the
	// second transaction observes the first transaction's new default edge.
	var concurrentAID, concurrentBID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.materials(code,name,kind,unit,cost_unit) VALUES('CONCURRENT-A','并发产出A','bean','kg','kg') RETURNING id`, schema)).Scan(&concurrentAID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.materials(code,name,kind,unit,cost_unit) VALUES('CONCURRENT-B','并发产出B','bean','kg','kg') RETURNING id`, schema)).Scan(&concurrentBID); err != nil {
		t.Fatal(err)
	}
	concurrentA, err := repo.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{Name: "并发A到B", OutputType: "material", OutputID: concurrentAID, OutputMaterialID: concurrentAID, OutputQty: 1, OutputUnit: "kg", Actor: "pr598-concurrent"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpdateProductionBomVersionDraft(ctx, bomapp.UpdateProductionBomVersionDraftCommand{VersionID: concurrentA.LatestVersionID, OutputQty: 1, OutputUnit: "kg", Items: []bomapp.ProductionBomDraftItem{{ComponentType: "material", MaterialID: concurrentBID, ConsumeUnit: "ratio_pct", RatioPct: 100}}, Actor: "pr598-concurrent"}); err != nil {
		t.Fatal(err)
	}
	concurrentB, err := repo.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{Name: "并发B到A", OutputType: "material", OutputID: concurrentBID, OutputMaterialID: concurrentBID, OutputQty: 1, OutputUnit: "kg", Actor: "pr598-concurrent"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpdateProductionBomVersionDraft(ctx, bomapp.UpdateProductionBomVersionDraftCommand{VersionID: concurrentB.LatestVersionID, OutputQty: 1, OutputUnit: "kg", Items: []bomapp.ProductionBomDraftItem{{ComponentType: "material", MaterialID: concurrentAID, ConsumeUnit: "ratio_pct", RatioPct: 100}}, Actor: "pr598-concurrent"}); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	errs := make(chan error, 2)
	var workers sync.WaitGroup
	for _, versionID := range []int64{concurrentA.LatestVersionID, concurrentB.LatestVersionID} {
		workers.Add(1)
		go func(versionID int64) {
			defer workers.Done()
			<-start
			errs <- repo.ValidateAndPublishProductionBomVersion(ctx, bomapp.PublishProductionBomVersionCommand{VersionID: versionID, Actor: "pr598-concurrent"})
		}(versionID)
	}
	close(start)
	workers.Wait()
	close(errs)
	successes := 0
	cycles := 0
	for publishErr := range errs {
		if publishErr == nil {
			successes++
		} else if strings.Contains(publishErr.Error(), "cycle detected") {
			cycles++
		} else {
			t.Fatalf("unexpected concurrent publish error: %v", publishErr)
		}
	}
	if successes != 1 || cycles != 1 {
		t.Fatalf("concurrent opposite publish results successes/cycles = %d/%d, want 1/1", successes, cycles)
	}

	// Publish must lock its output/component master rows in the same transaction
	// as validation. Otherwise an uncommitted deprecation can race between the
	// active check and the published status update.
	var lockedOutputID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.materials(code,name,kind,unit,cost_unit) VALUES('LOCKED-OUTPUT','并发停用产出','bean','kg','kg') RETURNING id`, schema)).Scan(&lockedOutputID); err != nil {
		t.Fatal(err)
	}
	lockedOutputBom, err := repo.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{Name: "并发停用产出校验", OutputType: "material", OutputID: lockedOutputID, OutputMaterialID: lockedOutputID, OutputQty: 1, OutputUnit: "kg", Actor: "pr598-lock"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpdateProductionBomVersionDraft(ctx, bomapp.UpdateProductionBomVersionDraftCommand{VersionID: lockedOutputBom.LatestVersionID, OutputQty: 1, OutputUnit: "kg", Items: []bomapp.ProductionBomDraftItem{{ComponentType: "material", MaterialID: inputMaterialID, ConsumeUnit: "ratio_pct", RatioPct: 100}}, Actor: "pr598-lock"}); err != nil {
		t.Fatal(err)
	}
	deprecateTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	deprecateCommitted := false
	defer func() {
		if !deprecateCommitted {
			_ = deprecateTx.Rollback(context.Background())
		}
	}()
	if _, err := deprecateTx.Exec(ctx, fmt.Sprintf(`UPDATE %s.materials SET deprecated_at=now() WHERE id=$1`, schema), lockedOutputID); err != nil {
		t.Fatal(err)
	}
	publishDone := make(chan error, 1)
	go func() {
		publishDone <- repo.ValidateAndPublishProductionBomVersion(ctx, bomapp.PublishProductionBomVersionCommand{VersionID: lockedOutputBom.LatestVersionID, Actor: "pr598-lock"})
	}()
	select {
	case publishErr := <-publishDone:
		t.Fatalf("publish bypassed concurrent target lock, err=%v", publishErr)
	case <-time.After(150 * time.Millisecond):
	}
	if err := deprecateTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	deprecateCommitted = true
	if publishErr := <-publishDone; publishErr == nil || !strings.Contains(publishErr.Error(), "output material is inactive") {
		t.Fatalf("publish after concurrent output deprecation error = %v", publishErr)
	}

	var editedOutputID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.materials(code,name,kind,unit,cost_unit) VALUES('EDITED-OUTPUT','并发编辑产出','bean','kg','kg') RETURNING id`, schema)).Scan(&editedOutputID); err != nil {
		t.Fatal(err)
	}
	editedBom, err := repo.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{Name: "并发草稿编辑校验", OutputType: "material", OutputID: editedOutputID, OutputMaterialID: editedOutputID, OutputQty: 1, OutputUnit: "kg", Actor: "pr598-edit"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpdateProductionBomVersionDraft(ctx, bomapp.UpdateProductionBomVersionDraftCommand{VersionID: editedBom.LatestVersionID, OutputQty: 1, OutputUnit: "kg", Items: []bomapp.ProductionBomDraftItem{{ComponentType: "material", MaterialID: inputMaterialID, ConsumeUnit: "ratio_pct", RatioPct: 100}}, Actor: "pr598-edit"}); err != nil {
		t.Fatal(err)
	}
	editTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	editCommitted := false
	defer func() {
		if !editCommitted {
			_ = editTx.Rollback(context.Background())
		}
	}()
	if _, err := editTx.Exec(ctx, fmt.Sprintf(`SELECT id FROM %s.production_bom_versions WHERE id=$1 FOR UPDATE`, schema), editedBom.LatestVersionID); err != nil {
		t.Fatal(err)
	}
	if _, err := editTx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_version_items SET material_id=999999997 WHERE version_id=$1`, schema), editedBom.LatestVersionID); err != nil {
		t.Fatal(err)
	}
	editPublishDone := make(chan error, 1)
	go func() {
		editPublishDone <- repo.ValidateAndPublishProductionBomVersion(ctx, bomapp.PublishProductionBomVersionCommand{VersionID: editedBom.LatestVersionID, Actor: "pr598-edit"})
	}()
	select {
	case publishErr := <-editPublishDone:
		t.Fatalf("publish bypassed concurrent draft lock, err=%v", publishErr)
	case <-time.After(150 * time.Millisecond):
	}
	if err := editTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	editCommitted = true
	if publishErr := <-editPublishDone; publishErr == nil || !strings.Contains(publishErr.Error(), "component material not found") {
		t.Fatalf("publish after concurrent draft edit error = %v", publishErr)
	}
}

func TestPR598DefaultBindingSwitchRejectsTypedGraphCyclesPostgres(t *testing.T) {
	ctx, pool, schema := newPR598BomBindingTestDB(t)
	mustPR598SQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.products(id,name,active,base_product_id) VALUES
			(8101,'商品默认切换',true,0),
			(8102,'物料默认切换依赖商品',true,0),
			(8103,'商品配置入口默认切换',true,0);
		INSERT INTO %[1]s.materials(id,code,name,kind,unit,cost_unit) VALUES
			(8201,'MAT-P-CYCLE','商品切换成环物料','bean','kg','kg'),
			(8202,'MAT-P-LEAF','商品安全叶子','bean','kg','kg'),
			(8203,'MAT-M-CYCLE','物料切换成环产出','bean','kg','kg'),
			(8204,'MAT-M-LEAF','物料安全叶子','bean','kg','kg'),
			(8205,'MAT-CATALOG-CYCLE','商品配置成环物料','bean','kg','kg'),
			(8206,'MAT-CATALOG-LEAF','商品配置安全叶子','bean','kg','kg');

		INSERT INTO %[1]s.production_boms(id,code,name,output_type,output_product_id,output_material_id,status) VALUES
			(9101,'BOM-P-SAFE','商品安全默认','product',8101,0,'active'),
			(9102,'BOM-P-CYCLE','商品成环候选','product',8101,0,'active'),
			(9103,'BOM-M-TO-P','物料依赖商品','material',0,8201,'active'),
			(9111,'BOM-P-TO-M','商品依赖物料','product',8102,0,'active'),
			(9112,'BOM-M-SAFE','物料安全默认','material',0,8203,'active'),
			(9113,'BOM-M-CYCLE','物料成环候选','material',0,8203,'active'),
			(9121,'BOM-CATALOG-P-SAFE','商品配置安全默认','product',8103,0,'active'),
			(9122,'BOM-CATALOG-P-CYCLE','商品配置成环候选','product',8103,0,'active'),
			(9123,'BOM-CATALOG-M-TO-P','商品配置物料依赖商品','material',0,8205,'active');
		INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,output_qty,output_unit,published_at,created_at) VALUES
			(9201,9101,'V001','published',1,'kg',now(),now()),
			(9202,9102,'V001','published',1,'kg',now(),now()),
			(9203,9103,'V001','published',1,'kg',now(),now()),
			(9211,9111,'V001','published',1,'kg',now(),now()),
			(9212,9112,'V001','published',1,'kg',now(),now()),
			(9213,9113,'V001','published',1,'kg',now(),now()),
			(9221,9121,'V001','published',1,'kg',now(),now()),
			(9222,9122,'V001','published',1,'kg',now(),now()),
			(9223,9123,'V001','published',1,'kg',now(),now());
		INSERT INTO %[1]s.production_bom_version_items(
			version_id,material_id,component_type,component_product_id,consume_unit,qty_per_unit,ratio_pct
		) VALUES
			(9201,8202,'material',0,'ratio_pct',0,100),
			(9202,8201,'material',0,'ratio_pct',0,100),
			(9203,0,'product',8101,'fixed_qty',1,0),
			(9211,8203,'material',0,'ratio_pct',0,100),
			(9212,8204,'material',0,'ratio_pct',0,100),
			(9213,0,'product',8102,'fixed_qty',1,0),
			(9221,8206,'material',0,'ratio_pct',0,100),
			(9222,8205,'material',0,'ratio_pct',0,100),
			(9223,0,'product',8103,'fixed_qty',1,0);

		INSERT INTO %[1]s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default,updated_by) VALUES
			('product',8101,9101,9201,true,'seed'),
			('material',8201,9103,9203,true,'seed'),
			('product',8102,9111,9211,true,'seed'),
			('material',8203,9112,9212,true,'seed'),
			('product',8103,9121,9221,true,'seed'),
			('material',8205,9123,9223,true,'seed');
		INSERT INTO %[1]s.product_production_bom_bindings(product_id,bom_id,bom_version_id,bound_by) VALUES
			(8101,9101,9201,'seed'),
			(8102,9111,9211,'seed'),
			(8103,9121,9221,'seed');
		INSERT INTO %[1]s.product_production_configs(
			product_id,production_bom_id,production_bom_version_id,created_by,updated_by
		) VALUES
			(8101,9101,0,'seed','seed'),
			(8102,9111,0,'seed','seed'),
			(8103,9121,0,'seed','seed');
	`, schema))

	repo := NewRepository(pool, schema)
	if _, err := repo.BindProductProductionBom(ctx, bomapp.BindProductProductionBomCommand{
		ProductID: 8101, BomID: 9102, Actor: "cycle-product-switch",
	}); err == nil || !strings.Contains(err.Error(), "cycle detected") {
		t.Fatalf("product default switch cycle error = %v", err)
	}
	assertPR598DefaultBindingIDs(t, ctx, pool, schema, "product", 8101, 9101, 9201)
	var legacyProductBomID, configProductBomID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT binding.bom_id, config.production_bom_id
		FROM %[1]s.product_production_bom_bindings binding
		JOIN %[1]s.product_production_configs config ON config.product_id=binding.product_id
		WHERE binding.product_id=8101
	`, schema)).Scan(&legacyProductBomID, &configProductBomID); err != nil {
		t.Fatal(err)
	}
	if legacyProductBomID != 9101 || configProductBomID != 9101 {
		t.Fatalf("product binding changed after rejected cycle: legacy=%d config=%d", legacyProductBomID, configProductBomID)
	}

	if _, err := repo.BindProductionBomOutput(ctx, bomapp.BindProductionBomOutputCommand{
		OutputType: "material", OutputID: 8203, BomID: 9113, Actor: "cycle-material-switch",
	}); err == nil || !strings.Contains(err.Error(), "cycle detected") {
		t.Fatalf("material default switch cycle error = %v", err)
	}
	assertPR598DefaultBindingIDs(t, ctx, pool, schema, "material", 8203, 9112, 9212)

	if _, err := postgrescatalog.NewRepository(pool, schema).SaveProductProductionConfig(ctx, catalogapp.SaveProductProductionConfigCommand{
		Actor: "cycle-catalog-switch", ProductID: 8103, ProductionBomID: 9122, ProductionBomVersionID: 9222,
	}); err == nil || !strings.Contains(err.Error(), "cycle detected") {
		t.Fatalf("catalog product default switch cycle error = %v", err)
	}
	assertPR598DefaultBindingIDs(t, ctx, pool, schema, "product", 8103, 9121, 9221)

	// Opposite default switches are each valid against the original graph, but
	// cannot both commit. The shared advisory graph lock makes the second switch
	// validate against the first switch's committed default.
	mustPR598SQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.materials(id,code,name,kind,unit,cost_unit) VALUES
			(8301,'MAT-CONCURRENT-A','并发默认A','bean','kg','kg'),
			(8302,'MAT-CONCURRENT-B','并发默认B','bean','kg','kg'),
			(8303,'MAT-CONCURRENT-LEAF','并发安全叶子','bean','kg','kg');
		INSERT INTO %[1]s.production_boms(id,code,name,output_type,output_product_id,output_material_id,status) VALUES
			(9131,'BOM-CONCURRENT-A-SAFE','并发A安全默认','material',0,8301,'active'),
			(9132,'BOM-CONCURRENT-A-TO-B','并发A依赖B','material',0,8301,'active'),
			(9133,'BOM-CONCURRENT-B-SAFE','并发B安全默认','material',0,8302,'active'),
			(9134,'BOM-CONCURRENT-B-TO-A','并发B依赖A','material',0,8302,'active');
		INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,output_qty,output_unit,published_at,created_at) VALUES
			(9231,9131,'V001','published',1,'kg',now(),now()),
			(9232,9132,'V001','published',1,'kg',now(),now()),
			(9233,9133,'V001','published',1,'kg',now(),now()),
			(9234,9134,'V001','published',1,'kg',now(),now());
		INSERT INTO %[1]s.production_bom_version_items(version_id,material_id,component_type,consume_unit,ratio_pct) VALUES
			(9231,8303,'material','ratio_pct',100),
			(9232,8302,'material','ratio_pct',100),
			(9233,8303,'material','ratio_pct',100),
			(9234,8301,'material','ratio_pct',100);
		INSERT INTO %[1]s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default,updated_by) VALUES
			('material',8301,9131,9231,true,'seed'),
			('material',8302,9133,9233,true,'seed');
	`, schema))
	start := make(chan struct{})
	errs := make(chan error, 2)
	var workers sync.WaitGroup
	for _, cmd := range []bomapp.BindProductionBomOutputCommand{
		{OutputType: "material", OutputID: 8301, BomID: 9132, Actor: "concurrent-a"},
		{OutputType: "material", OutputID: 8302, BomID: 9134, Actor: "concurrent-b"},
	} {
		workers.Add(1)
		go func(cmd bomapp.BindProductionBomOutputCommand) {
			defer workers.Done()
			<-start
			_, err := repo.BindProductionBomOutput(ctx, cmd)
			errs <- err
		}(cmd)
	}
	close(start)
	workers.Wait()
	close(errs)
	successes, cycles := 0, 0
	for err := range errs {
		switch {
		case err == nil:
			successes++
		case strings.Contains(err.Error(), "cycle detected"):
			cycles++
		default:
			t.Fatalf("unexpected concurrent default switch error: %v", err)
		}
	}
	if successes != 1 || cycles != 1 {
		t.Fatalf("concurrent opposite default switches successes/cycles = %d/%d, want 1/1", successes, cycles)
	}
}

func newPR598BomBindingTestDB(t *testing.T) (context.Context, *pgxpool.Pool, string) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	schema := fmt.Sprintf("pr598_bom_binding_%d_%d", os.Getpid(), time.Now().UnixNano())
	mustPR598SQL(t, ctx, pool, "CREATE SCHEMA "+schema)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })
	mustPR598SQL(t, ctx, pool, fmt.Sprintf(`
		CREATE TABLE %[1]s.products(
			id BIGINT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			active BOOLEAN NOT NULL DEFAULT true,
			base_product_id BIGINT NOT NULL DEFAULT 0
		);
	`, schema))
	if err := supporthttp.EnsureAuditTables(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if err := postgresmaterials.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	mustPR598SQL(t, ctx, pool, fmt.Sprintf(`
		CREATE TABLE %[1]s.product_bom_spec_migrations(
			product_id BIGINT PRIMARY KEY,
			state TEXT NOT NULL,
			legacy_catalog_product BOOLEAN NOT NULL DEFAULT true
		);
		CREATE TABLE %[1]s.product_production_configs(
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
		CREATE TABLE %[1]s.product_production_config_industry_templates(
			product_id BIGINT NOT NULL,
			template_id BIGINT NOT NULL,
			sort_order INT NOT NULL DEFAULT 1,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			created_by TEXT NOT NULL DEFAULT '',
			updated_by TEXT NOT NULL DEFAULT '',
			PRIMARY KEY(product_id,template_id)
		);
		CREATE TABLE %[1]s.product_production_config_fields(
			id BIGSERIAL PRIMARY KEY,
			product_id BIGINT NOT NULL,
			field_key TEXT NOT NULL DEFAULT '',
			label TEXT NOT NULL DEFAULT '',
			field_type TEXT NOT NULL DEFAULT 'text',
			unit TEXT NOT NULL DEFAULT '',
			value_text TEXT NOT NULL DEFAULT '',
			value_number NUMERIC(14,4),
			value_bool BOOLEAN,
			template_field_key TEXT NOT NULL DEFAULT '',
			required BOOLEAN NOT NULL DEFAULT false,
			options_json JSONB NOT NULL DEFAULT '[]'::jsonb,
			show_in_price_list BOOLEAN NOT NULL DEFAULT true,
			sort_order INT NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
	`, schema))
	return ctx, pool, schema
}

func TestPR598LegacyOrphanBomKeepsOutputConstraintPendingUntilRepaired(t *testing.T) {
	ctx, pool, schema := newPR598LegacyBomTestDB(t)

	// This is the production_boms shape deployed before PR-598. Historical
	// rows with no legacy product or default binding cannot be assigned a typed
	// output without a business decision, so the migration must preserve them.
	mustPR598SQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.production_boms(code,name,status)
		VALUES('LEGACY-ORPHAN','历史待识别 BOM','inactive');
	`, schema))

	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema must preserve an unmapped legacy BOM: %v", err)
	}
	var constraintOID int64
	var validated bool
	if err := pool.QueryRow(ctx, `
		SELECT c.oid::bigint,c.convalidated
		FROM pg_constraint c
		JOIN pg_class t ON t.oid=c.conrelid
		JOIN pg_namespace n ON n.oid=t.relnamespace
		WHERE n.nspname=$1 AND t.relname='production_boms' AND c.conname='production_boms_output_binding_check'
	`, schema).Scan(&constraintOID, &validated); err != nil {
		t.Fatal(err)
	}
	if validated {
		t.Fatal("typed output constraint must remain pending while a legacy orphan exists")
	}
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("repeated EnsureSchema must preserve an unmapped legacy BOM: %v", err)
	}
	var repeatedConstraintOID int64
	if err := pool.QueryRow(ctx, `
		SELECT c.oid::bigint,c.convalidated
		FROM pg_constraint c
		JOIN pg_class t ON t.oid=c.conrelid
		JOIN pg_namespace n ON n.oid=t.relnamespace
		WHERE n.nspname=$1 AND t.relname='production_boms' AND c.conname='production_boms_output_binding_check'
	`, schema).Scan(&repeatedConstraintOID, &validated); err != nil {
		t.Fatal(err)
	}
	if repeatedConstraintOID != constraintOID || validated {
		t.Fatalf("pending constraint changed across EnsureSchema: oid %d -> %d, validated=%v", constraintOID, repeatedConstraintOID, validated)
	}
	var legacyStatus, legacyOutputType string
	var legacyOutputProductID, legacyOutputMaterialID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT status,output_type,output_product_id,output_material_id
		FROM %s.production_boms
		WHERE code='LEGACY-ORPHAN'
	`, schema)).Scan(&legacyStatus, &legacyOutputType, &legacyOutputProductID, &legacyOutputMaterialID); err != nil {
		t.Fatal(err)
	}
	if legacyStatus != "inactive" || legacyOutputType != "product" || legacyOutputProductID != 0 || legacyOutputMaterialID != 0 {
		t.Fatalf("legacy orphan was rewritten: status=%s output=%s:%d/%d", legacyStatus, legacyOutputType, legacyOutputProductID, legacyOutputMaterialID)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_boms(code,name,output_type,output_product_id,output_material_id)
		VALUES('NEW-INVALID','invalid','product',0,0)
	`, schema)); err == nil {
		t.Fatal("NOT VALID constraint must still reject new untyped BOM writes")
	}
	mustPR598SQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %s.production_boms
		SET output_type='product', output_product_id=42, output_material_id=0
		WHERE code='LEGACY-ORPHAN'
	`, schema))
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema after legacy output repair: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT c.convalidated
		FROM pg_constraint c
		JOIN pg_class t ON t.oid=c.conrelid
		JOIN pg_namespace n ON n.oid=t.relnamespace
		WHERE n.nspname=$1 AND t.relname='production_boms' AND c.conname='production_boms_output_binding_check'
	`, schema).Scan(&validated); err != nil {
		t.Fatal(err)
	}
	if !validated {
		t.Fatal("typed output constraint must validate after all legacy rows are repaired")
	}
}

func TestPR598LegacyOutputConstraintRejectsUnsupportedInvalidRows(t *testing.T) {
	tests := []struct {
		name  string
		setup string
	}{
		{
			name: "active orphan",
			setup: `INSERT INTO %[1]s.production_boms(code,name,status)
				VALUES('ACTIVE-ORPHAN','active orphan','active')`,
		},
		{
			name: "both output ids",
			setup: `ALTER TABLE %[1]s.production_boms ADD COLUMN output_type TEXT NOT NULL DEFAULT 'product';
				ALTER TABLE %[1]s.production_boms ADD COLUMN output_material_id BIGINT NOT NULL DEFAULT 0;
				INSERT INTO %[1]s.production_boms(code,name,status,output_type,output_product_id,output_material_id)
				VALUES('BOTH-OUTPUTS','both outputs','inactive','product',9,7)`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, pool, schema := newPR598LegacyBomTestDB(t)
			mustPR598SQL(t, ctx, pool, fmt.Sprintf(test.setup, schema))
			if err := EnsureSchema(ctx, pool, schema); err == nil {
				t.Fatal("EnsureSchema must reject unsupported invalid typed-output history")
			}
		})
	}
}

func newPR598LegacyBomTestDB(t *testing.T) (context.Context, *pgxpool.Pool, string) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	schema := fmt.Sprintf("pr598_legacy_output_%d_%d", os.Getpid(), time.Now().UnixNano())
	mustPR598SQL(t, ctx, pool, "CREATE SCHEMA "+schema)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })
	if err := postgresmaterials.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	mustPR598SQL(t, ctx, pool, fmt.Sprintf(`
		CREATE TABLE %[1]s.production_boms (
			id BIGSERIAL PRIMARY KEY,
			code TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL DEFAULT '',
			output_product_id BIGINT NOT NULL DEFAULT 0,
			group_id BIGINT NOT NULL DEFAULT 0,
			group_category_id BIGINT NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'active',
			source_bom_id BIGINT NOT NULL DEFAULT 0,
			source_bom_version_id BIGINT NOT NULL DEFAULT 0,
			source_product_id BIGINT NOT NULL DEFAULT 0,
			source_product_code_snapshot TEXT NOT NULL DEFAULT '',
			source_product_name_snapshot TEXT NOT NULL DEFAULT '',
			legacy_product_id BIGINT NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			created_by TEXT NOT NULL DEFAULT '',
			updated_by TEXT NOT NULL DEFAULT ''
		);
	`, schema))
	return ctx, pool, schema
}

func assertPR598DefaultBindingIDs(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema, outputType string, outputID, wantBomID, wantVersionID int64) {
	t.Helper()
	var bomID, versionID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT bom_id,bom_version_id
		FROM %s.production_bom_output_bindings
		WHERE output_type=$1 AND output_id=$2 AND is_default=true
	`, schema), outputType, outputID).Scan(&bomID, &versionID); err != nil {
		t.Fatal(err)
	}
	if bomID != wantBomID || versionID != wantVersionID {
		t.Fatalf("%s:%d default binding = %d/%d, want %d/%d", outputType, outputID, bomID, versionID, wantBomID, wantVersionID)
	}
}

func createAndPublishPR598MaterialBom(t *testing.T, ctx context.Context, repo Repository, outputMaterialID, inputMaterialID int64, name string) bomapp.ProductionBomSummary {
	t.Helper()
	row, err := repo.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{Name: name, OutputType: "material", OutputID: outputMaterialID, OutputMaterialID: outputMaterialID, OutputQty: 1, OutputUnit: "kg", Actor: "pr598-test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpdateProductionBomVersionDraft(ctx, bomapp.UpdateProductionBomVersionDraftCommand{VersionID: row.LatestVersionID, OutputQty: 1, OutputUnit: "kg", Items: []bomapp.ProductionBomDraftItem{{ComponentType: "material", MaterialID: inputMaterialID, ConsumeUnit: "ratio_pct", RatioPct: 100}}, Actor: "pr598-test"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.ValidateProductionBomVersionForPublish(ctx, bomapp.PublishProductionBomVersionCommand{VersionID: row.LatestVersionID}); err != nil {
		t.Fatal(err)
	}
	if err := repo.PublishProductionBomVersion(ctx, bomapp.PublishProductionBomVersionCommand{VersionID: row.LatestVersionID, Actor: "pr598-test"}); err != nil {
		t.Fatal(err)
	}
	row.LatestVersionStatus = "published"
	return row
}

func mustPR598SQL(t *testing.T, ctx context.Context, pool *pgxpool.Pool, query string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(ctx, query, args...); err != nil {
		t.Fatal(err)
	}
}
