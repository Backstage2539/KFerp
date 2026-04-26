package materials

import (
	"context"
	"fmt"
	"math"
	"strings"

	materialsapp "orderapp/internal/application/materials"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool   *pgxpool.Pool
	schema string
}

type materialRow struct {
	ID            int64
	Code          string
	Name          string
	Kind          string
	Unit          string
	PurchasePrice float64
	SalePrice     float64
	OnhandG       int64
	OnhandUnits   int64
	MinLevelG     int64
	MinLevelUnits int64
	UpdatedAt     string
}

type materialInput struct {
	Code          string
	Name          string
	Kind          string
	Unit          string
	PurchasePrice float64
	SalePrice     float64
	OnhandG       int64
	OnhandUnits   int64
	MinLevelG     int64
	MinLevelUnits int64
}

func NewRepository(pool *pgxpool.Pool, schema string) Repository {
	return Repository{pool: pool, schema: schema}
}

func (r Repository) List(ctx context.Context, cmd materialsapp.ListCommand) ([]materialsapp.Material, error) {
	rows, err := listMaterials(ctx, r.pool, r.schema, cmd.Query, cmd.Limit)
	if err != nil {
		return nil, err
	}
	return materialsToApp(rows), nil
}

func (r Repository) Update(ctx context.Context, cmd materialsapp.UpdateCommand) (materialsapp.Material, error) {
	row, err := updateMaterialInline(ctx, r.pool, r.schema, cmd.Actor, cmd.ID, materialInputFromApp(cmd.Input))
	if err != nil {
		return materialsapp.Material{}, err
	}
	return materialToApp(row), nil
}

func listMaterials(ctx context.Context, pool *pgxpool.Pool, schema, q string, limit int) ([]materialRow, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	where := ""
	args := []any{}
	argn := 1
	if s := strings.TrimSpace(q); s != "" {
		where = fmt.Sprintf("WHERE (name ILIKE $%d OR code ILIKE $%d)", argn, argn)
		args = append(args, "%"+s+"%")
		argn++
	}
	args = append(args, limit)
	limitArg := argn

	sql := fmt.Sprintf(`
		SELECT id, code, name, kind, unit,
		       COALESCE(purchase_price,0), COALESCE(sale_price,0),
		       onhand_g, onhand_units, min_level_g, min_level_units,
		       to_char(updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.materials
		%s
		ORDER BY kind, name, id DESC
		LIMIT $%d
	`, schema, where, limitArg)
	rows, err := pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]materialRow, 0)
	for rows.Next() {
		var r materialRow
		if err := rows.Scan(&r.ID, &r.Code, &r.Name, &r.Kind, &r.Unit, &r.PurchasePrice, &r.SalePrice, &r.OnhandG, &r.OnhandUnits, &r.MinLevelG, &r.MinLevelUnits, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func updateMaterialInline(ctx context.Context, pool *pgxpool.Pool, schema, actor string, id int64, in materialInput) (materialRow, error) {
	if id <= 0 {
		return materialRow{}, fmt.Errorf("invalid id")
	}
	next, err := normalizeMaterialInput(in)
	if err != nil {
		return materialRow{}, err
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return materialRow{}, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return materialRow{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var old materialRow
	qOld := fmt.Sprintf(`SELECT id, code, name, kind, unit,
		       COALESCE(purchase_price,0), COALESCE(sale_price,0),
		       onhand_g, onhand_units, min_level_g, min_level_units,
		       to_char(updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.materials WHERE id=$1 FOR UPDATE`, schema)
	if err := tx.QueryRow(ctx, qOld, id).Scan(&old.ID, &old.Code, &old.Name, &old.Kind, &old.Unit, &old.PurchasePrice, &old.SalePrice, &old.OnhandG, &old.OnhandUnits, &old.MinLevelG, &old.MinLevelUnits, &old.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return materialRow{}, fmt.Errorf("not found")
		}
		return materialRow{}, err
	}

	q := fmt.Sprintf(`UPDATE %s.materials SET
			code=$2,
			name=$3,
			kind=$4,
			unit=$5,
			purchase_price=$6,
			sale_price=$7,
			onhand_g=$8,
			onhand_units=$9,
			min_level_g=$10,
			min_level_units=$11,
			updated_at=now()
		WHERE id=$1`, schema)
	if _, err := tx.Exec(ctx, q, id, next.Code, next.Name, next.Kind, next.Unit, next.PurchasePrice, next.SalePrice, next.OnhandG, next.OnhandUnits, next.MinLevelG, next.MinLevelUnits); err != nil {
		return materialRow{}, err
	}
	if err := logMaterialDiffsTx(ctx, tx, schema, actor, old, next); err != nil {
		return materialRow{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return materialRow{}, err
	}

	rows, err := listMaterials(ctx, pool, schema, next.Code, 1)
	if err != nil {
		return materialRow{}, err
	}
	if len(rows) == 0 {
		return materialRow{}, fmt.Errorf("not found")
	}
	return rows[0], nil
}

func normalizeMaterialInput(in materialInput) (materialInput, error) {
	in.Code = strings.TrimSpace(in.Code)
	in.Name = strings.TrimSpace(in.Name)
	in.Kind = strings.TrimSpace(in.Kind)
	in.Unit = strings.TrimSpace(in.Unit)
	if in.Code == "" {
		return materialInput{}, fmt.Errorf("code required")
	}
	if in.Name == "" {
		return materialInput{}, fmt.Errorf("name required")
	}
	if in.Kind == "" {
		in.Kind = "other"
	}
	if in.Unit == "" {
		in.Unit = "g"
	}
	switch in.Kind {
	case "bean", "pack", "other":
	default:
		return materialInput{}, fmt.Errorf("invalid kind")
	}
	if in.PurchasePrice < 0 || in.SalePrice < 0 || math.IsNaN(in.PurchasePrice) || math.IsNaN(in.SalePrice) || math.IsInf(in.PurchasePrice, 0) || math.IsInf(in.SalePrice, 0) {
		return materialInput{}, fmt.Errorf("negative price")
	}
	if in.OnhandG < 0 || in.OnhandUnits < 0 || in.MinLevelG < 0 || in.MinLevelUnits < 0 {
		return materialInput{}, fmt.Errorf("negative qty")
	}
	return in, nil
}

func logMaterialDiffsTx(ctx context.Context, tx pgx.Tx, schema, actor string, old materialRow, next materialInput) error {
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "unknown"
	}
	id := old.ID
	log := func(field, oldValue, newValue string) error {
		if oldValue == newValue {
			return nil
		}
		q := fmt.Sprintf(`INSERT INTO %s.audit_logs(actor, entity_type, entity_id, action, field, old_value, new_value, meta)
			VALUES($1,'material',$2,'update',$3,$4,$5,jsonb_build_object('material_id',$2::bigint,'code',$6::text))`, schema)
		_, err := tx.Exec(ctx, q, actor, id, field, oldValue, newValue, next.Code)
		return err
	}
	for _, item := range []struct {
		field string
		old   string
		next  string
	}{
		{"code", old.Code, next.Code},
		{"name", old.Name, next.Name},
		{"kind", old.Kind, next.Kind},
		{"unit", old.Unit, next.Unit},
		{"purchase_price", fmt.Sprintf("%.2f", old.PurchasePrice), fmt.Sprintf("%.2f", next.PurchasePrice)},
		{"sale_price", fmt.Sprintf("%.2f", old.SalePrice), fmt.Sprintf("%.2f", next.SalePrice)},
		{"onhand_g", fmt.Sprintf("%d", old.OnhandG), fmt.Sprintf("%d", next.OnhandG)},
		{"onhand_units", fmt.Sprintf("%d", old.OnhandUnits), fmt.Sprintf("%d", next.OnhandUnits)},
		{"min_level_g", fmt.Sprintf("%d", old.MinLevelG), fmt.Sprintf("%d", next.MinLevelG)},
		{"min_level_units", fmt.Sprintf("%d", old.MinLevelUnits), fmt.Sprintf("%d", next.MinLevelUnits)},
	} {
		if err := log(item.field, item.old, item.next); err != nil {
			return err
		}
	}
	return nil
}

func materialsToApp(rows []materialRow) []materialsapp.Material {
	out := make([]materialsapp.Material, 0, len(rows))
	for _, row := range rows {
		out = append(out, materialToApp(row))
	}
	return out
}

func materialToApp(row materialRow) materialsapp.Material {
	return materialsapp.Material{
		ID:            row.ID,
		Code:          row.Code,
		Name:          row.Name,
		Kind:          row.Kind,
		Unit:          row.Unit,
		PurchasePrice: row.PurchasePrice,
		SalePrice:     row.SalePrice,
		OnhandG:       row.OnhandG,
		OnhandUnits:   row.OnhandUnits,
		MinLevelG:     row.MinLevelG,
		MinLevelUnits: row.MinLevelUnits,
		UpdatedAt:     row.UpdatedAt,
	}
}

func materialInputFromApp(in materialsapp.MaterialInput) materialInput {
	return materialInput{
		Code:          in.Code,
		Name:          in.Name,
		Kind:          in.Kind,
		Unit:          in.Unit,
		PurchasePrice: in.PurchasePrice,
		SalePrice:     in.SalePrice,
		OnhandG:       in.OnhandG,
		OnhandUnits:   in.OnhandUnits,
		MinLevelG:     in.MinLevelG,
		MinLevelUnits: in.MinLevelUnits,
	}
}
