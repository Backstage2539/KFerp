package production

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	salesapp "orderapp/internal/application/sales"
)

// OrderOperationOptionsTx follows only the selected output's manufacturing
// dependencies. It reads published BOMs and never creates a plan or work order.
func OrderOperationOptionsTx(ctx context.Context, tx pgx.Tx, schema string, productID, bomSpecID, bomVariantID int64) ([]salesapp.OrderOperationOption, error) {
	if productID <= 0 || bomSpecID < 0 || bomVariantID < 0 || (bomSpecID == 0 && bomVariantID > 0) {
		return nil, fmt.Errorf("商品或 BOM 规格无效")
	}
	root, err := resolveProductionBomForDemandProductSpecTx(ctx, tx, schema, productID, 0, "", bomSpecID, bomVariantID, false)
	if err != nil {
		if isProductionBomNotConfiguredError(err) {
			return []salesapp.OrderOperationOption{}, nil
		}
		return nil, err
	}
	type output struct {
		kind                              string
		id, spec, variant, version, route int64
		name                              string
	}
	start := output{kind: "product", id: productID, spec: bomSpecID, variant: bomVariantID, version: root.BomVersionID, route: root.ProcessRouteID, name: root.ProductName}
	result := []salesapp.OrderOperationOption{}
	visiting, visited, seen := map[string]bool{}, map[string]bool{}, map[string]bool{}
	var walk func(output) error
	walk = func(node output) error {
		key := fmt.Sprintf("%s:%d:%d", node.kind, node.id, node.spec)
		if visiting[key] {
			return fmt.Errorf("制造依赖存在循环，请先核对 BOM：%s", node.name)
		}
		if visited[key] {
			return nil
		}
		if len(visited)+len(visiting) > 500 {
			return fmt.Errorf("制造依赖层级过多，请核对 BOM")
		}
		visiting[key] = true
		if node.route > 0 {
			rows, e := tx.Query(ctx, fmt.Sprintf(`SELECT pro.id,pro.operation_id,COALESCE(NULLIF(mo.name,''),pro.operation) FROM %[1]s.process_route_operations pro JOIN %[1]s.process_routes route ON route.id=pro.route_id AND route.status='active' LEFT JOIN %[1]s.manufacturing_operations mo ON mo.id=pro.operation_id WHERE pro.route_id=$1 AND pro.operation_id>0 ORDER BY pro.seq,pro.id`, schema), node.route)
			if e != nil {
				return e
			}
			for rows.Next() {
				o := salesapp.OrderOperationOption{OutputType: node.kind, OutputID: node.id, BomSpecID: node.spec, BomVariantID: node.variant, OutputName: node.name, RouteID: node.route}
				if e = rows.Scan(&o.StepID, &o.OperationID, &o.OperationName); e != nil {
					rows.Close()
					return e
				}
				identity := fmt.Sprintf("%s:%d:%d:%d:%d:%d", o.OutputType, o.OutputID, o.BomSpecID, o.BomVariantID, o.RouteID, o.StepID)
				if !seen[identity] {
					result = append(result, o)
					seen[identity] = true
				}
			}
			e = rows.Err()
			rows.Close()
			if e != nil {
				return e
			}
		}
		rows, e := tx.Query(ctx, fmt.Sprintf(`SELECT CASE WHEN component_type IN ('product','finished_product') THEN 'product' ELSE 'material' END,CASE WHEN component_type IN ('product','finished_product') THEN component_product_id ELSE material_id END,COALESCE(component_bom_spec_id,0) FROM %s.production_bom_version_items WHERE version_id=$1 AND COALESCE(variant_id,0)=$2 AND (qty_per_unit>0 OR ratio_pct>0) ORDER BY id`, schema), node.version, node.variant)
		if e != nil {
			return e
		}
		children := []output{}
		for rows.Next() {
			var child output
			if e = rows.Scan(&child.kind, &child.id, &child.spec); e != nil {
				rows.Close()
				return e
			}
			if child.kind == "material" {
				child.spec = 0
			}
			if child.id > 0 {
				children = append(children, child)
			}
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		for _, child := range children {
			// A component without a published manufacturing binding is a purchased
			// input and has no operation candidates. Never include unrelated routes.
			e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT version.id,COALESCE(variant.id,0),COALESCE(NULLIF(variant.process_route_id,0),version.process_route_id,0),CASE WHEN binding.output_type='product' THEN COALESCE(p.name,'') ELSE COALESCE(m.name,'') END
    FROM %[1]s.production_bom_output_bindings binding
    JOIN %[1]s.production_boms bom ON bom.id=binding.bom_id AND bom.status='active' AND bom.output_type=binding.output_type AND ((binding.output_type='product' AND bom.output_product_id=binding.output_id) OR (binding.output_type='material' AND bom.output_material_id=binding.output_id))
    JOIN %[1]s.production_bom_versions version ON version.id=binding.bom_version_id AND version.bom_id=bom.id AND version.status='published'
    LEFT JOIN %[1]s.production_bom_specs spec ON $3::bigint>0 AND spec.id=$3 AND spec.bom_id=bom.id
    LEFT JOIN %[1]s.production_bom_version_variants variant ON variant.version_id=version.id AND variant.bom_spec_id=spec.id
    LEFT JOIN %[1]s.products p ON binding.output_type='product' AND p.id=binding.output_id
    LEFT JOIN %[1]s.materials m ON binding.output_type='material' AND m.id=binding.output_id
    WHERE binding.output_type=$1 AND binding.output_id=$2 AND binding.is_default=true AND ($3::bigint=0 OR variant.id IS NOT NULL)
    AND (binding.output_type<>'product' OR p.active=true) AND (binding.output_type<>'material' OR m.deprecated_at IS NULL)
    ORDER BY version.id,variant.id LIMIT 1`, schema), child.kind, child.id, child.spec).Scan(&child.version, &child.variant, &child.route, &child.name)
			if e == pgx.ErrNoRows {
				continue
			}
			if e != nil {
				return e
			}
			if e = walk(child); e != nil {
				return e
			}
		}
		delete(visiting, key)
		visited[key] = true
		return nil
	}
	if err = walk(start); err != nil {
		return nil, err
	}
	return result, nil
}
