package production

import (
	"context"
	"fmt"

	productionapp "orderapp/internal/application/production"
)

func (r Repository) ListMachines(ctx context.Context, activeOnly bool) ([]productionapp.RoastMachine, error) {
	where := ""
	order := "id"
	if activeOnly {
		where = " WHERE active=true"
		order = "capacity_g DESC,id ASC"
	}
	q := fmt.Sprintf(`SELECT id,COALESCE(name,''),COALESCE(capacity_g,0),COALESCE(allowed_specs,''),COALESCE(min_roast_g,0),COALESCE(active,true) FROM %s.roast_machines%s ORDER BY %s`, r.schema, where, order)
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]productionapp.RoastMachine, 0)
	for rows.Next() {
		var row productionapp.RoastMachine
		if err := rows.Scan(&row.ID, &row.Name, &row.CapacityG, &row.AllowedSpecs, &row.MinRoastG, &row.Active); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) SaveMachine(ctx context.Context, cmd productionapp.RoastMachineCommand) error {
	if cmd.ID > 0 {
		tag, err := r.pool.Exec(ctx, "UPDATE "+r.schema+".roast_machines SET name=$2,capacity_g=$3,allowed_specs=$4,min_roast_g=$5,active=$6,updated_at=now() WHERE id=$1", cmd.ID, cmd.Name, cmd.CapacityG, cmd.AllowedSpecs, cmd.MinRoastG, cmd.Active)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("machine not found")
		}
		return nil
	}
	_, err := r.pool.Exec(ctx, "INSERT INTO "+r.schema+".roast_machines(name,capacity_g,allowed_specs,min_roast_g,active,updated_at) VALUES($1,$2,$3,$4,$5,now())", cmd.Name, cmd.CapacityG, cmd.AllowedSpecs, cmd.MinRoastG, cmd.Active)
	return err
}
