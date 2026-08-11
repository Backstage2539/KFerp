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
			SELECT bom_id, output_type, output_id, version_id FROM candidate
			UNION ALL
			SELECT pb.id,
			       binding.output_type,
			       binding.output_id,
			       binding.bom_version_id
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
		),
		edges AS (
			SELECT selected.output_type AS parent_type,
			       selected.output_id AS parent_id,
			       CASE WHEN item.component_type IN ('product','finished_product') THEN 'product' ELSE 'material' END AS component_type,
			       CASE WHEN item.component_type IN ('product','finished_product') THEN item.component_product_id ELSE item.material_id END AS component_id
			FROM selected_versions selected
			JOIN %[1]s.production_bom_version_items item ON item.version_id=selected.version_id
			WHERE CASE WHEN item.component_type IN ('product','finished_product') THEN item.component_product_id ELSE item.material_id END > 0
		),
		walk(node_type, node_id, path) AS (
			SELECT candidate.output_type,
			       candidate.output_id,
			       ARRAY[candidate.output_type || ':' || candidate.output_id::text]
			FROM candidate
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
			JOIN candidate ON candidate.output_type=edge.component_type AND candidate.output_id=edge.component_id
		)
	`, schema), candidateVersionID).Scan(&hasCycle); err != nil {
		return err
	}
	if hasCycle {
		return fmt.Errorf("cycle detected")
	}
	return nil
}
