package bomgraph

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type Queryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// ValidateCandidate rejects a published or draft BOM version when selecting it
// as the default for its typed output would close a cycle in the current default
// product/material BOM graph. Callers must hold the shared default-graph
// advisory lock for the surrounding transaction.
func ValidateCandidate(ctx context.Context, q Queryer, schema string, candidateVersionID int64) error {
	var hasCycle bool
	if err := q.QueryRow(ctx, fmt.Sprintf(`
		WITH RECURSIVE candidate AS (
			SELECT pb.id AS bom_id,
			       COALESCE(NULLIF(pb.output_type,''),'product') AS output_type,
			       CASE WHEN pb.output_type='material' THEN pb.output_material_id ELSE pb.output_product_id END AS output_id,
			       v.id AS version_id
			FROM %[1]s.production_bom_versions v
			JOIN %[1]s.production_boms pb ON pb.id=v.bom_id
			WHERE v.id=$1
		),
		selected_versions AS (
			SELECT bom_id, output_type, output_id, version_id, true AS is_output_default FROM candidate
			UNION ALL
			SELECT pb.id,
			       binding.output_type,
			       binding.output_id,
			       binding.bom_version_id,
			       true
			FROM %[1]s.production_bom_output_bindings binding
			JOIN %[1]s.production_boms pb ON pb.id=binding.bom_id
			JOIN %[1]s.production_bom_versions version
			  ON version.id=binding.bom_version_id
			 AND version.bom_id=pb.id
			 AND version.status='published'
			WHERE binding.is_default=true
			  AND COALESCE(NULLIF(pb.status,''),'active')='active'
			  AND pb.output_type=binding.output_type
			  AND CASE WHEN pb.output_type='material' THEN pb.output_material_id ELSE pb.output_product_id END=binding.output_id
			  AND NOT EXISTS(
				SELECT 1
				FROM candidate
				WHERE candidate.output_type=binding.output_type
				  AND candidate.output_id=binding.output_id
			  )
			UNION ALL
			SELECT DISTINCT pb.id,
			       COALESCE(NULLIF(pb.output_type,''),'product'),
			       CASE WHEN pb.output_type='material' THEN pb.output_material_id ELSE pb.output_product_id END,
			       version.id,
			       false
			FROM %[1]s.production_bom_versions version
			JOIN %[1]s.production_boms pb ON pb.id=version.bom_id
			JOIN %[1]s.production_bom_version_variants variant ON variant.version_id=version.id
			WHERE version.status='published'
			  AND COALESCE(NULLIF(pb.status,''),'active')='active'
			  AND NOT EXISTS(SELECT 1 FROM candidate WHERE candidate.version_id=version.id)
			  AND NOT EXISTS(
			      SELECT 1 FROM %[1]s.production_bom_output_bindings binding
			      WHERE binding.bom_version_id=version.id AND binding.is_default=true
			  )
		),
		item_edges AS (
			SELECT CASE WHEN parent_variant.id IS NOT NULL THEN 'product_spec' ELSE selected.output_type END AS parent_type,
			       CASE WHEN parent_variant.id IS NOT NULL THEN parent_variant.bom_spec_id ELSE selected.output_id END AS parent_id,
			       CASE
			         WHEN COALESCE(item.component_bom_spec_id,0)>0 THEN 'product_spec'
			         WHEN item.component_type IN ('product','finished_product') THEN 'product'
			         ELSE 'material'
			       END AS component_type,
			       CASE
			         WHEN COALESCE(item.component_bom_spec_id,0)>0 THEN item.component_bom_spec_id
			         WHEN item.component_type IN ('product','finished_product') THEN item.component_product_id
			         ELSE item.material_id
			       END AS component_id
			FROM selected_versions selected
			JOIN %[1]s.production_bom_version_items item ON item.version_id=selected.version_id
			LEFT JOIN %[1]s.production_bom_version_variants parent_variant
			  ON parent_variant.id=item.variant_id AND parent_variant.version_id=selected.version_id
			WHERE CASE
			        WHEN COALESCE(item.component_bom_spec_id,0)>0 THEN item.component_bom_spec_id
			        WHEN item.component_type IN ('product','finished_product') THEN item.component_product_id
			        ELSE item.material_id
			      END > 0
		),
		alias_edges AS (
			SELECT 'product'::text AS parent_type,
			       selected.output_id AS parent_id,
			       'product_spec'::text AS component_type,
			       default_variant.bom_spec_id AS component_id
			FROM selected_versions selected
			JOIN %[1]s.production_bom_version_variants default_variant
			  ON default_variant.version_id=selected.version_id AND default_variant.is_default=true
			WHERE selected.output_type='product' AND selected.is_output_default=true
		),
		edges AS (
			SELECT parent_type,parent_id,component_type,component_id FROM item_edges
			UNION ALL
			SELECT parent_type,parent_id,component_type,component_id FROM alias_edges
		),
		candidate_nodes AS (
			SELECT 'product_spec'::text AS node_type, variant.bom_spec_id AS node_id
			FROM candidate
			JOIN %[1]s.production_bom_version_variants variant ON variant.version_id=candidate.version_id
			UNION ALL
			SELECT candidate.output_type,candidate.output_id
			FROM candidate
			WHERE NOT EXISTS(
				SELECT 1 FROM %[1]s.production_bom_version_variants variant WHERE variant.version_id=candidate.version_id
			)
		),
		walk(node_type, node_id, path) AS (
			SELECT candidate_nodes.node_type,
			       candidate_nodes.node_id,
			       ARRAY[candidate_nodes.node_type || ':' || candidate_nodes.node_id::text]
			FROM candidate_nodes
			UNION ALL
			SELECT edge.component_type,
			       edge.component_id,
			       walk.path || (edge.component_type || ':' || edge.component_id::text)
			FROM walk
			JOIN edges edge ON edge.parent_type=walk.node_type AND edge.parent_id=walk.node_id
			WHERE NOT (edge.component_type || ':' || edge.component_id::text) = ANY(walk.path)
		)
		SELECT EXISTS(
			SELECT 1
			FROM walk
			JOIN edges edge ON edge.parent_type=walk.node_type AND edge.parent_id=walk.node_id
			JOIN candidate_nodes ON candidate_nodes.node_type=edge.component_type AND candidate_nodes.node_id=edge.component_id
		)
	`, schema), candidateVersionID).Scan(&hasCycle); err != nil {
		return err
	}
	if hasCycle {
		return fmt.Errorf("cycle detected")
	}
	return nil
}
