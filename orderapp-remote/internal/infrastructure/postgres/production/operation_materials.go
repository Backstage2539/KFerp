package production

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/production"
	infra "orderapp/internal/infrastructure/postgres"
	"strings"
)

func (r Repository) GetOperationMaterialConfig(ctx context.Context, id, variantID int64) (app.OperationMaterialConfig, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.OperationMaterialConfig{}, err
	}
	defer tx.Rollback(ctx)
	return r.operationMaterialConfigTx(ctx, tx, id, variantID)
}
func (r Repository) operationMaterialConfigTx(ctx context.Context, tx pgx.Tx, id, variantID int64) (app.OperationMaterialConfig, error) {
	out := app.OperationMaterialConfig{Rows: []app.OperationMaterialConfigRow{}, Nodes: []app.OperationMaterialConfigNode{}}
	var routeID int64
	var raw string
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(x.process_route_id,v.process_route_id),COALESCE(b.bindings,'{}'::jsonb)::text FROM %[1]s.production_bom_versions v LEFT JOIN %[1]s.production_bom_version_variants x ON x.version_id=v.id AND x.id=$2 LEFT JOIN %[1]s.production_bom_operation_bindings b ON b.bom_version_id=v.id AND b.bom_variant_id=$2 WHERE v.id=$1 AND ($2=0 OR x.id IS NOT NULL)`, r.schema), id, variantID).Scan(&routeID, &raw)
	if err != nil {
		return out, err
	}
	bindings := map[string]app.OperationMaterialBinding{}
	if err = json.Unmarshal([]byte(raw), &bindings); err != nil {
		return out, err
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT seq,operation,COALESCE(NULLIF(execution_json->>'node_key',''),'route-'||route_id||'-step-'||seq) FROM %s.process_route_operations WHERE route_id=$1 ORDER BY seq,id`, r.schema), routeID)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var node app.OperationMaterialConfigNode
		if err = rows.Scan(&node.Sequence, &node.Name, &node.Key); err != nil {
			rows.Close()
			return out, err
		}
		out.Nodes = append(out.Nodes, node)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	rows, err = tx.Query(ctx, fmt.Sprintf(`SELECT COALESCE(NULLIF(i.component_type,''),'material'),i.material_id,i.component_product_id,i.component_bom_spec_id,i.component_spec_g,COALESCE(m.name,p.name,'') FROM %[1]s.production_bom_version_items i LEFT JOIN %[1]s.materials m ON m.id=i.material_id LEFT JOIN %[1]s.products p ON p.id=i.component_product_id WHERE i.version_id=$1 AND i.variant_id=$2 ORDER BY i.id`, r.schema), id, variantID)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var kind, name string
		var material, product, spec, grams int64
		if err = rows.Scan(&kind, &material, &product, &spec, &grams, &name); err != nil {
			rows.Close()
			return out, err
		}
		component := material
		if kind == "product" || kind == "finished_product" {
			component = product
			kind = "product"
		}
		key := operationMaterialKey(kind, component, spec, grams)
		b := bindings[key]
		if b.NodeKey == "" && len(out.Nodes) == 1 {
			b.NodeKey = out.Nodes[0].Key
		}
		out.Rows = append(out.Rows, app.OperationMaterialConfigRow{Key: key, Name: name, Binding: b})
	}
	err = rows.Err()
	rows.Close()
	encoded, _ := json.Marshal([]any{raw, out.Rows, out.Nodes})
	out.Version = fmt.Sprintf("%x", sha256.Sum256(encoded))
	return out, err
}
func (r Repository) SaveOperationMaterialConfig(ctx context.Context, c app.SaveOperationMaterialConfig) (app.OperationMaterialConfig, error) {
	var out app.OperationMaterialConfig
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, fmt.Sprintf("%s:operation_materials:%d:%d", r.schema, c.VersionID, c.VariantID)); err != nil {
		return out, err
	}
	out, err = r.operationMaterialConfigTx(ctx, tx, c.VersionID, c.VariantID)
	if err != nil {
		return out, err
	}
	if out.Version != c.ExpectedVersion {
		return out, fmt.Errorf("BOM或工艺路线已变化，请刷新")
	}
	nodes := map[string]bool{}
	for _, n := range out.Nodes {
		nodes[n.Key] = true
	}
	seen := map[string]bool{}
	for _, row := range out.Rows {
		b, ok := c.Bindings[row.Key]
		if !ok || !nodes[b.NodeKey] {
			return out, fmt.Errorf("请指定 %s 的使用工序", row.Name)
		}
		seen[row.Key] = true
		if b.SourceWarehouse != "" {
			var active bool
			if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.warehouses WHERE code=$1 AND active)`, r.schema), b.SourceWarehouse).Scan(&active); err != nil {
				return out, err
			}
			if !active {
				return out, fmt.Errorf("来源仓不存在或已停用")
			}
		}
	}
	for k := range c.Bindings {
		if !seen[k] {
			return out, fmt.Errorf("组件已变化，请刷新")
		}
	}
	if strings.TrimSpace(c.Actor) == "" {
		return out, fmt.Errorf("操作者不能为空")
	}
	raw, _ := json.Marshal(c.Bindings)
	if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_operation_bindings(bom_version_id,bom_variant_id,bindings,updated_by) VALUES($1,$4,$2::jsonb,$3) ON CONFLICT(bom_version_id,bom_variant_id) DO UPDATE SET bindings=EXCLUDED.bindings,updated_by=EXCLUDED.updated_by,updated_at=now()`, r.schema), c.VersionID, string(raw), c.Actor, c.VariantID); err != nil {
		return out, err
	}
	if err = infra.AuditInsertTx(ctx, tx, r.schema, c.Actor, "production_bom_version", &c.VersionID, "operation_materials", nil, nil, nil, infra.AuditMeta{"old": out.Rows, "new": c.Bindings}); err != nil {
		return out, err
	}
	out, err = r.operationMaterialConfigTx(ctx, tx, c.VersionID, c.VariantID)
	if err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}

func validateFrozenOperationBindingsTx(ctx context.Context, tx pgx.Tx, schema string, workID int64) error {
	var snapshot, raw string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT process_snapshot_json::text,operation_material_bindings::text FROM %s.work_orders WHERE id=$1`, schema), workID).Scan(&snapshot, &raw); err != nil {
		return err
	}
	var route processTemplateSnapshot
	if err := json.Unmarshal([]byte(snapshot), &route); err != nil {
		return err
	}
	nodes := map[string]bool{}
	for _, op := range route.Operations {
		var cfg app.OperationExecution
		_ = json.Unmarshal([]byte(op.ExecutionJSON), &cfg)
		if cfg.Version != 1 {
			return nil
		}
		nodes[cfg.NodeKey] = true
	}
	if len(nodes) < 2 {
		return nil
	}
	bindings := map[string]app.OperationMaterialBinding{}
	if err := json.Unmarshal([]byte(raw), &bindings); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT component_type,CASE WHEN component_id>0 THEN component_id ELSE material_id END,component_bom_spec_id,component_spec_g,material_name FROM %s.work_order_material_reservations WHERE work_order_id=$1`, schema), workID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var kind, name string
		var id, spec, g int64
		if err = rows.Scan(&kind, &id, &spec, &g, &name); err != nil {
			return err
		}
		if kind == "" {
			kind = "material"
		}
		if kind == "finished_product" {
			kind = "product"
		}
		if !nodes[bindings[operationMaterialKey(kind, id, spec, g)].NodeKey] {
			return fmt.Errorf("请先在BOM中指定 %s 的使用工序，再释放工单", name)
		}
	}
	return rows.Err()
}
