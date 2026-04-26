package postgres

import (
	"context"
	"fmt"

	bomdomain "orderapp/internal/domain/bom"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ListBagSpecMappings(ctx context.Context, pool *pgxpool.Pool, schema string) ([]bomdomain.BagSpecMapping, error) {
	q := fmt.Sprintf(`
		SELECT m.spec_g, m.material_id, COALESCE(mat.name,'')
		FROM %s.packaging_spec_material_map m
		LEFT JOIN %s.materials mat ON mat.id = m.material_id
		ORDER BY m.spec_g
	`, schema, schema)
	rows, err := pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]bomdomain.BagSpecMapping, 0)
	for rows.Next() {
		var r bomdomain.BagSpecMapping
		if err := rows.Scan(&r.SpecG, &r.MaterialID, &r.MaterialName); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
