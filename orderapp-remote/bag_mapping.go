package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type BagSpecMapping struct {
	SpecG        int64
	MaterialID   int64
	MaterialName string
}

func ensureBagSpecMappingTable(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.packaging_spec_material_map (
		spec_g BIGINT PRIMARY KEY,
		material_id BIGINT NOT NULL,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`, schema)
	_, err := pool.Exec(ctx, q)
	return err
}

func listBagSpecMappings(ctx context.Context, pool *pgxpool.Pool, schema string) ([]BagSpecMapping, error) {
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

	out := make([]BagSpecMapping, 0)
	for rows.Next() {
		var r BagSpecMapping
		if err := rows.Scan(&r.SpecG, &r.MaterialID, &r.MaterialName); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func validBagSpecMappingInputs(specG, materialID int64) error {
	if specG <= 0 {
		return fmt.Errorf("spec_g required")
	}
	if materialID <= 0 {
		return fmt.Errorf("material_id required")
	}
	return nil
}

func saveBagSpecMapping(ctx context.Context, pool *pgxpool.Pool, schema string, specG, materialID int64) error {
	if err := validBagSpecMappingInputs(specG, materialID); err != nil {
		return err
	}

	q := fmt.Sprintf(`INSERT INTO %s.packaging_spec_material_map(spec_g, material_id, updated_at)
		VALUES($1,$2,now())
		ON CONFLICT (spec_g) DO UPDATE SET material_id=excluded.material_id, updated_at=now()`, schema)
	_, err := pool.Exec(ctx, q, specG, materialID)
	return err
}

func deleteBagSpecMapping(ctx context.Context, pool *pgxpool.Pool, schema string, specG int64) error {
	if specG <= 0 {
		return fmt.Errorf("spec_g required")
	}
	_, err := pool.Exec(ctx, "DELETE FROM "+schema+".packaging_spec_material_map WHERE spec_g=$1", specG)
	return err
}

func mappingNameBySpec(mappings []BagSpecMapping) map[int64]string {
	out := map[int64]string{}
	for _, m := range mappings {
		name := strings.TrimSpace(m.MaterialName)
		if m.SpecG > 0 && name != "" {
			out[m.SpecG] = name
		}
	}
	return out
}
