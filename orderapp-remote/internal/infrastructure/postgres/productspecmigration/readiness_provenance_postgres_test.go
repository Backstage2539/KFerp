package productspecmigration

import (
	"errors"
	"fmt"
	"testing"
	"time"

	productspecmigrationapp "orderapp/internal/application/productspecmigration"
)

func TestCutoverRequiresPublishedTemplateProvenanceAndActiveMainInputPostgres(t *testing.T) {
	ctx, pool, schema := newLegacyChildGuardTestDB(t)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.products(id,name) VALUES(80,'legacy parent');
		INSERT INTO %[1]s.products(
			id,name,parent_product_id,spec_label,sku_code,sku_name,net_content_qty,net_content_unit
		) VALUES(81,'manual legacy child',80,'default','LEGACY-DEFAULT','默认规格',1,'袋');
		INSERT INTO %[1]s.production_boms(id,output_type,output_product_id,status)
		VALUES(90,'product',80,'active');
		INSERT INTO %[1]s.production_bom_versions(
			id,bom_id,version_no,status,published_at,source_spec_template_version_id,main_input_material_id
		) VALUES(91,90,'V001','published',now(),0,0);
		INSERT INTO %[1]s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default)
		VALUES('product',80,90,91,true);
		INSERT INTO %[1]s.production_bom_specs(id,bom_id,spec_key,name,inventory_unit)
		VALUES(180,90,'default','默认规格','袋');
		INSERT INTO %[1]s.production_bom_version_variants(
			id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order
		) VALUES(280,91,180,'默认规格','袋',true,1);
	`, schema)); err != nil {
		t.Fatal(err)
	}

	repo := NewRepository(pool, schema)
	if _, err := repo.Prepare(ctx, productspecmigrationapp.PrepareCommand{ProductID: 80, Actor: "provenance-test"}); err != nil {
		t.Fatal(err)
	}

	manual, err := repo.Assess(ctx, productspecmigrationapp.AssessCommand{ProductID: 80, Actor: "provenance-test"})
	if err != nil {
		t.Fatal(err)
	}
	assertReadinessBlocker(t, manual.Readiness, "missing_published_spec_template_provenance")
	assertReadinessBlocker(t, manual.Readiness, "inactive_main_input_material")
	if manual.State != productspecmigrationapp.StatePreparing {
		t.Fatalf("manual variants migration state=%q, want preparing", manual.State)
	}
	if _, err := repo.Cutover(ctx, productspecmigrationapp.CutoverCommand{ProductID: 80, Actor: "provenance-test"}); err == nil {
		t.Fatal("manual variants without template provenance unexpectedly cut over")
	} else {
		var blocked *productspecmigrationapp.CutoverBlockedError
		if !errors.As(err, &blocked) {
			t.Fatalf("manual variants cutover error=%v, want CutoverBlockedError", err)
		}
	}

	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.production_bom_spec_template_versions(id,status,published_at)
		VALUES(902,'archived',NULL);
		UPDATE %[1]s.production_bom_versions
		SET source_spec_template_version_id=902,main_input_material_id=901
		WHERE id=91;
	`, schema)); err != nil {
		t.Fatal(err)
	}
	neverPublished, err := repo.Assess(ctx, productspecmigrationapp.AssessCommand{ProductID: 80, Actor: "provenance-test"})
	if err != nil {
		t.Fatal(err)
	}
	assertReadinessBlocker(t, neverPublished.Readiness, "missing_published_spec_template_provenance")
	assertNoReadinessBlocker(t, neverPublished.Readiness, "inactive_main_input_material")

	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		UPDATE %[1]s.production_bom_versions
		SET source_spec_template_version_id=900,main_input_material_id=901
		WHERE id=91;
	`, schema)); err != nil {
		t.Fatal(err)
	}
	ready, err := repo.Assess(ctx, productspecmigrationapp.AssessCommand{ProductID: 80, Actor: "provenance-test"})
	if err != nil || !ready.Readiness.Ready || ready.State != productspecmigrationapp.StateReady {
		t.Fatalf("historically published template readiness=%+v state=%q err=%v", ready.Readiness, ready.State, err)
	}

	deprecateTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := deprecateTx.Exec(ctx, fmt.Sprintf(`UPDATE %s.materials SET deprecated_at=now() WHERE id=901`, schema)); err != nil {
		_ = deprecateTx.Rollback(ctx)
		t.Fatal(err)
	}
	cutoverDone := make(chan error, 1)
	go func() {
		_, cutoverErr := repo.Cutover(ctx, productspecmigrationapp.CutoverCommand{ProductID: 80, Actor: "provenance-test"})
		cutoverDone <- cutoverErr
	}()
	select {
	case cutoverErr := <-cutoverDone:
		_ = deprecateTx.Rollback(ctx)
		t.Fatalf("cutover completed while main-input validity change was uncommitted: %v", cutoverErr)
	case <-time.After(150 * time.Millisecond):
	}
	if err := deprecateTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case cutoverErr := <-cutoverDone:
		var blocked *productspecmigrationapp.CutoverBlockedError
		if !errors.As(cutoverErr, &blocked) {
			t.Fatalf("deprecated main-input cutover error=%v, want CutoverBlockedError", cutoverErr)
		}
		assertReadinessBlocker(t, blocked.Readiness, "inactive_main_input_material")
	case <-time.After(5 * time.Second):
		t.Fatal("cutover remained blocked after main-input validity change committed")
	}
	var state string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT state FROM %s.product_bom_spec_migrations WHERE product_id=80`, schema)).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != string(productspecmigrationapp.StatePreparing) {
		t.Fatalf("migration state after concurrent blocker=%q, want preparing", state)
	}

	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.materials SET deprecated_at=NULL WHERE id=901`, schema)); err != nil {
		t.Fatal(err)
	}
	cutover, err := repo.Cutover(ctx, productspecmigrationapp.CutoverCommand{ProductID: 80, Actor: "provenance-test"})
	if err != nil || cutover.State != productspecmigrationapp.StateCutover {
		t.Fatalf("valid provenance cutover=%+v err=%v", cutover, err)
	}
	if _, err := repo.Cutover(ctx, productspecmigrationapp.CutoverCommand{ProductID: 80, Actor: "provenance-test"}); err != nil {
		t.Fatalf("idempotent cutover: %v", err)
	}
}

func assertReadinessBlocker(t *testing.T, readiness productspecmigrationapp.Readiness, code string) {
	t.Helper()
	for _, blocker := range readiness.Blockers {
		if blocker.Code == code && blocker.Count > 0 {
			return
		}
	}
	t.Fatalf("readiness blockers=%+v, want %q", readiness.Blockers, code)
}

func assertNoReadinessBlocker(t *testing.T, readiness productspecmigrationapp.Readiness, code string) {
	t.Helper()
	for _, blocker := range readiness.Blockers {
		if blocker.Code == code {
			t.Fatalf("readiness blockers=%+v, do not want %q", readiness.Blockers, code)
		}
	}
}
