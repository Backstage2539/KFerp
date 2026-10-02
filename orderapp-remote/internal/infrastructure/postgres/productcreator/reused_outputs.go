package productcreator

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/productcreator"
)

type referenceReader interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (e BusinessExecutor) referenceReader(ctx context.Context) referenceReader {
	if tx := queryWithTransaction(ctx); tx != nil {
		return tx
	}
	if e.pool != nil {
		return e.pool
	}
	return nil
}
func (e BusinessExecutor) inspectReusedOutputs(ctx context.Context, run app.Run) (map[string]map[string]any, []app.ValidationIssue) {
	details := map[string]map[string]any{}
	issues := []app.ValidationIssue{}
	q := e.referenceReader(ctx)
	for _, n := range run.Workflow.Nodes {
		if !app.IsReusedOutput(run.Workflow, n, run.Inputs) {
			continue
		}
		values := run.Inputs[n.ID]
		key := "material_id"
		if n.Kind == app.ModuleProduct {
			key = "product_id"
		}
		id := positiveNumber(values[key])
		if id <= 0 {
			continue
		}
		fail := func(message string) {
			issues = append(issues, app.ValidationIssue{NodeID: n.ID, Field: key, Code: "reference_unavailable", Message: message})
		}
		if q == nil {
			fail("已有档案暂时无法读取")
			continue
		}
		var name, unit, specs string
		var owner, bomID, versionID int64
		var active bool
		if n.Kind == app.ModuleMaterial {
			var manufactured bool
			err := q.QueryRow(ctx, fmt.Sprintf(`SELECT name,COALESCE(unit,''),COALESCE(owner_customer_id,0),COALESCE(is_semi_finished,false),deprecated_at IS NULL FROM %s.materials WHERE id=$1 FOR SHARE`, e.schema), id).Scan(&name, &unit, &owner, &manufactured, &active)
			if err != nil || !active || !manufactured {
				fail("请选择有效的已有自制物料")
				continue
			}
			if owner != positiveNumber(values["owner_customer_id"]) {
				fail("已有物料归属已变化或不匹配，请重新选择")
				continue
			}
		} else {
			err := q.QueryRow(ctx, fmt.Sprintf(`SELECT name,COALESCE(customer_id,0),COALESCE(active,true) FROM %s.products WHERE id=$1 FOR SHARE`, e.schema), id).Scan(&name, &owner, &active)
			if err != nil || !active {
				fail("请选择有效的已有商品")
				continue
			}
			if owner != positiveNumber(values["customer_id"]) {
				fail("已有商品归属已变化或不匹配，请重新选择")
				continue
			}
			err = q.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(jsonb_agg(jsonb_build_array(s.id,v.id,x.id,x.inventory_unit) ORDER BY s.id,x.id),'[]'::jsonb)::text
    FROM %[1]s.production_boms b JOIN %[1]s.production_bom_specs s ON s.bom_id=b.id
    JOIN LATERAL(SELECT id FROM %[1]s.production_bom_versions WHERE bom_id=b.id AND status='published' ORDER BY published_at DESC NULLS LAST,created_at DESC,id DESC LIMIT 1)v ON true
    JOIN %[1]s.production_bom_version_variants x ON x.version_id=v.id AND x.bom_spec_id=s.id
    WHERE b.output_type='product' AND b.output_product_id=$1 AND COALESCE(b.status,'active')='active'`, e.schema), id).Scan(&specs)
			if err != nil || specs == "[]" {
				fail("已有商品没有有效的已发布规格")
				continue
			}
		}
		// Lock the existing default binding during commit. Reuse never changes it.
		err := q.QueryRow(ctx, fmt.Sprintf(`SELECT ob.bom_id,ob.bom_version_id FROM %[1]s.production_bom_output_bindings ob
    JOIN %[1]s.production_boms b ON b.id=ob.bom_id AND b.status='active'
    JOIN %[1]s.production_bom_versions v ON v.id=ob.bom_version_id AND v.bom_id=b.id AND v.status='published'
    WHERE ob.output_type=$1 AND ob.output_id=$2 AND ob.is_default=true FOR SHARE OF ob,b,v`, e.schema), string(n.Kind), id).Scan(&bomID, &versionID)
		if err != nil && err != pgx.ErrNoRows {
			fail("无法读取已有制造配置")
			continue
		}
		if n.Kind == app.ModuleMaterial && bomID == 0 {
			fail("已有自制物料缺少默认已发布制造 BOM，请在物料原业务页面补齐")
			continue
		}
		details[n.ID] = map[string]any{"type": n.Kind, "id": id, "name": name, "unit": unit, "owner_customer_id": owner, "bom_id": bomID, "bom_version_id": versionID, "specifications": specs}
	}
	return details, issues
}
func (e BusinessExecutor) validateReusedSnapshots(ctx context.Context, run app.Run) error {
	if run.Workflow.Version < 8 {
		return nil
	}
	active := run
	active.Workflow = app.ActiveWorkflow(run.Workflow, run.Inputs)
	details, issues := e.inspectReusedOutputs(ctx, active)
	if len(issues) > 0 {
		return app.ExecutionError{Issues: issues}
	}
	// The persisted preview is the review boundary, not data supplied in a commit request.
	for id, current := range details {
		var previous any
		if run.Preview != nil {
			for _, step := range run.Preview.Steps {
				if step.NodeID == id {
					previous = step.Details["reference_snapshot"]
				}
			}
		}
		before, _ := json.Marshal(previous)
		after, _ := json.Marshal(current)
		if string(before) != string(after) {
			return app.ExecutionError{Issues: []app.ValidationIssue{{NodeID: id, Code: "reference_changed", Message: "所选已有档案或制造配置已变化，请重新预览后提交"}}}
		}
	}
	return nil
}
func (e BusinessExecutor) validExistingProductSpec(ctx context.Context, productID, specID int64) bool {
	q := e.referenceReader(ctx)
	if q == nil || productID <= 0 || specID <= 0 {
		return false
	}
	var valid bool
	err := q.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.production_bom_specs s
  JOIN %[1]s.production_boms b ON b.id=s.bom_id AND b.output_type='product' AND b.output_product_id=$1 AND b.status='active'
  JOIN LATERAL(SELECT id FROM %[1]s.production_bom_versions WHERE bom_id=b.id AND status='published' ORDER BY published_at DESC NULLS LAST,created_at DESC,id DESC LIMIT 1)v ON true
  JOIN %[1]s.production_bom_version_variants x ON x.version_id=v.id AND x.bom_spec_id=s.id WHERE s.id=$2)`, e.schema), productID, specID).Scan(&valid)
	return err == nil && valid
}
