// A guarded one-record correction through the normal material domain service.
// Preview is read-only; apply requires its exact snapshot digest.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	app "orderapp/internal/application/materials"
	pc "orderapp/internal/application/productcreator"
	infra "orderapp/internal/infrastructure/postgres"
	materials "orderapp/internal/infrastructure/postgres/materials"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	runID := flag.Int64("run", 0, "committed creator run")
	materialID := flag.Int64("material", 0, "input material id")
	name := flag.String("name", "", "correct purchased material name")
	actor := flag.String("actor", "", "audit operator")
	confirm := flag.String("confirm", "", "preview digest; omit for read-only preview")
	flag.Parse()
	if *runID <= 0 || *materialID <= 0 || strings.TrimSpace(*name) == "" {
		return fmt.Errorf("run, material and name are required")
	}
	schema := os.Getenv("DB_SCHEMA")
	if schema == "" {
		schema = "p2rms15pepb5ciz"
	}
	schema = pgx.Identifier{schema}.Sanitize()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	apply := *confirm != ""
	if apply && strings.TrimSpace(*actor) == "" {
		return fmt.Errorf("actor required for apply")
	}
	// Lock the material and all transactional tables referencing material ids,
	// preventing new receipts/reservations while checking and correcting it.
	refs, err := tx.Query(ctx, `SELECT table_name,column_name FROM information_schema.columns WHERE table_schema=$1 AND column_name IN ('material_id','output_material_id') ORDER BY table_name,column_name`, strings.Trim(schema, `"`))
	if err != nil {
		return err
	}
	type ref struct{ Table, Column string }
	links := []ref{}
	for refs.Next() {
		var r ref
		if err := refs.Scan(&r.Table, &r.Column); err != nil {
			refs.Close()
			return err
		}
		links = append(links, r)
	}
	err = refs.Err()
	refs.Close()
	if err != nil {
		return err
	}
	if apply {
		tables := map[string]bool{"materials": true, "product_creator_runs": true, "audit_logs": true}
		for _, r := range links {
			tables[r.Table] = true
		}
		names := []string{}
		for table := range tables {
			names = append(names, schema+"."+pgx.Identifier{table}.Sanitize())
		}
		sort.Strings(names)
		if _, err := tx.Exec(ctx, "SET LOCAL lock_timeout='5s'"); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "LOCK TABLE "+strings.Join(names, ",")+" IN SHARE ROW EXCLUSIVE MODE"); err != nil {
			return err
		}
	}
	var workflowBytes, resultBytes, materialBytes []byte
	var status string
	var version int64
	if err := tx.QueryRow(ctx, "SELECT status,template_version,workflow_snapshot,commit_result FROM "+schema+".product_creator_runs WHERE id=$1", *runID).Scan(&status, &version, &workflowBytes, &resultBytes); err != nil {
		return err
	}
	if status != "config_committed" && status != "in_progress" {
		return fmt.Errorf("run is not committed")
	}
	var workflow pc.Workflow
	var result struct {
		Objects map[string][]struct {
			Type string `json:"type"`
			ID   int64  `json:"id"`
		}
	}
	if err := json.Unmarshal(workflowBytes, &workflow); err != nil {
		return err
	}
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		return err
	}
	nodeID := ""
	for _, node := range workflow.Nodes {
		if node.Kind == pc.ModuleMaterial && node.Config["data_role"] == "input" {
			for _, obj := range result.Objects[node.ID] {
				if obj.Type == "material" && obj.ID == *materialID {
					nodeID = node.ID
				}
			}
		}
	}
	if nodeID == "" {
		return fmt.Errorf("material is not a committed input of this run")
	}
	if err := tx.QueryRow(ctx, "SELECT to_jsonb(m) FROM "+schema+".materials m WHERE id=$1 AND deprecated_at IS NULL", *materialID).Scan(&materialBytes); err != nil {
		return err
	}
	var input app.MaterialInput
	if err := json.Unmarshal(materialBytes, &input); err != nil {
		return err
	}
	if input.OnhandG != 0 || input.OnhandUnits != 0 || input.PurchasePrice != 0 {
		return fmt.Errorf("stock or purchase cost changed; correction stopped")
	}
	evidence := map[string]any{"run_id": *runID, "template_version": version, "node_id": nodeID, "material": json.RawMessage(materialBytes), "new_name": *name, "new_supply_mode": "purchase"}
	counts := map[string]int64{}
	profiles := map[string]json.RawMessage{}
	safe := map[string]bool{"material_bean_profiles": true, "material_pack_profiles": true, "material_industry_field_values": true, "material_classification_assignments": true, "material_customer_references": true, "production_bom_version_items": true}
	for _, r := range links {
		var data []byte
		query := fmt.Sprintf("SELECT COALESCE(jsonb_agg(to_jsonb(t) ORDER BY to_jsonb(t)::text),'[]') FROM %s.%s t WHERE %s=$1", schema, pgx.Identifier{r.Table}.Sanitize(), pgx.Identifier{r.Column}.Sanitize())
		if err := tx.QueryRow(ctx, query, *materialID).Scan(&data); err != nil {
			return err
		}
		var rows []json.RawMessage
		if err := json.Unmarshal(data, &rows); err != nil {
			return err
		}
		counts[r.Table+"."+r.Column] = int64(len(rows))
		profiles[r.Table] = data
		if len(rows) > 0 && !safe[r.Table] {
			return fmt.Errorf("new/existing business dependency %s: %d; correction stopped", r.Table, len(rows))
		}
	}
	// Consumer BOMs must belong to this creator run; preserve their recipe rows.
	allowed := []int64{}
	for _, objs := range result.Objects {
		for _, obj := range objs {
			if obj.Type == "bom" {
				allowed = append(allowed, obj.ID)
			}
		}
	}
	var external int
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM "+schema+".production_bom_version_items i JOIN "+schema+".production_bom_versions v ON v.id=i.version_id WHERE i.material_id=$1 AND NOT(v.bom_id=ANY($2))", *materialID, allowed).Scan(&external); err != nil {
		return err
	}
	if external > 0 {
		return fmt.Errorf("material now used outside this creator run")
	}
	evidence["references"] = profiles
	evidence["reference_counts"] = counts
	raw, _ := json.Marshal(evidence)
	digest := fmt.Sprintf("%x", sha256.Sum256(raw))
	if !apply {
		evidence["confirmation_digest"] = digest
		out, _ := json.MarshalIndent(evidence, "", "  ")
		fmt.Println(string(out))
		return nil
	}
	if *confirm != digest {
		return fmt.Errorf("snapshot changed; obtain a fresh preview before applying")
	}
	var beans []app.BeanProfile
	_ = json.Unmarshal(profiles["material_bean_profiles"], &beans)
	if len(beans) > 0 {
		input.BeanProfile = &beans[0]
	}
	var packs []struct {
		app.PackProfile
		Texture string `json:"material_texture"`
	}
	_ = json.Unmarshal(profiles["material_pack_profiles"], &packs)
	if len(packs) > 0 {
		input.PackProfile = &packs[0].PackProfile
		input.PackProfile.Material = packs[0].Texture
	}
	_ = json.Unmarshal(profiles["material_industry_field_values"], &input.IndustryFields)
	input.Name = *name
	input.SupplyMode = "purchase"
	input.IsSemiFinished = false
	input.IsSemiFinishedSet = true
	txCtx := infra.WithBusinessProvenance(infra.WithTransaction(ctx, tx), *runID, nodeID)
	// The domain service retains ordinary stock/unit/BOM protection and audits.
	svc := app.NewService(materials.NewRepository(pool, strings.Trim(schema, `"`)))
	updated, err := svc.Update(txCtx, app.UpdateCommand{ID: *materialID, Actor: *actor, Input: input})
	if err != nil {
		return err
	}
	id := *runID
	if err := infra.AuditInsertTx(txCtx, tx, strings.Trim(schema, `"`), *actor, "product_creator_run", &id, "correct_input_material", infra.StrPtr("material"), infra.StrPtr(string(materialBytes)), infra.StrPtr(*name), infra.AuditMeta{"run_id": *runID, "material_id": *materialID, "node_id": nodeID, "template_version": version, "snapshot_digest": digest}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	out, _ := json.Marshal(map[string]any{"status": "corrected", "run_id": *runID, "material": updated, "snapshot_digest": digest})
	fmt.Println(string(out))
	return nil
}
