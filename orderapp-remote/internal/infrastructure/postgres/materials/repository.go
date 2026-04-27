package materials

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	materialsapp "orderapp/internal/application/materials"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool   *pgxpool.Pool
	schema string
}

type materialRow struct {
	ID                int64
	Code              string
	Name              string
	Kind              string
	Unit              string
	BatchNo           string
	PurchasePrice     float64
	SalePrice         float64
	OnhandG           int64
	OnhandUnits       int64
	MinLevelG         int64
	MinLevelUnits     int64
	Origin            string
	ProcessingStation string
	Variety           string
	ProcessMethod     string
	Grade             string
	Altitude          string
	Flavor            string
	BeanListNote      string
	UpdatedAt         string
}

type materialInput struct {
	Code              string
	Name              string
	Kind              string
	Unit              string
	BatchNo           string
	PurchasePrice     float64
	SalePrice         float64
	OnhandG           int64
	OnhandUnits       int64
	MinLevelG         int64
	MinLevelUnits     int64
	Origin            string
	ProcessingStation string
	Variety           string
	ProcessMethod     string
	Grade             string
	Altitude          string
	Flavor            string
	BeanListNote      string
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
		       COALESCE(batch_no, ''),
		       COALESCE(purchase_price,0), COALESCE(sale_price,0),
		       onhand_g, onhand_units, min_level_g, min_level_units,
		       COALESCE(origin, ''), COALESCE(processing_station, ''), COALESCE(variety, ''),
		       COALESCE(process_method, ''), COALESCE(grade, ''), COALESCE(altitude, ''),
		       COALESCE(flavor, ''), COALESCE(bean_list_note, ''),
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
		if err := rows.Scan(&r.ID, &r.Code, &r.Name, &r.Kind, &r.Unit, &r.BatchNo, &r.PurchasePrice, &r.SalePrice, &r.OnhandG, &r.OnhandUnits, &r.MinLevelG, &r.MinLevelUnits, &r.Origin, &r.ProcessingStation, &r.Variety, &r.ProcessMethod, &r.Grade, &r.Altitude, &r.Flavor, &r.BeanListNote, &r.UpdatedAt); err != nil {
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
		       COALESCE(batch_no, ''),
		       COALESCE(purchase_price,0), COALESCE(sale_price,0),
		       onhand_g, onhand_units, min_level_g, min_level_units,
		       COALESCE(origin, ''), COALESCE(processing_station, ''), COALESCE(variety, ''),
		       COALESCE(process_method, ''), COALESCE(grade, ''), COALESCE(altitude, ''),
		       COALESCE(flavor, ''), COALESCE(bean_list_note, ''),
		       to_char(updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.materials WHERE id=$1 FOR UPDATE`, schema)
	if err := tx.QueryRow(ctx, qOld, id).Scan(&old.ID, &old.Code, &old.Name, &old.Kind, &old.Unit, &old.BatchNo, &old.PurchasePrice, &old.SalePrice, &old.OnhandG, &old.OnhandUnits, &old.MinLevelG, &old.MinLevelUnits, &old.Origin, &old.ProcessingStation, &old.Variety, &old.ProcessMethod, &old.Grade, &old.Altitude, &old.Flavor, &old.BeanListNote, &old.UpdatedAt); err != nil {
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
			batch_no=$6,
			purchase_price=$7,
			sale_price=$8,
			onhand_g=$9,
			onhand_units=$10,
			min_level_g=$11,
			min_level_units=$12,
			origin=$13,
			processing_station=$14,
			variety=$15,
			process_method=$16,
			grade=$17,
			altitude=$18,
			flavor=$19,
			bean_list_note=$20,
			updated_at=now()
		WHERE id=$1`, schema)
	if _, err := tx.Exec(ctx, q, id, next.Code, next.Name, next.Kind, next.Unit, next.BatchNo, next.PurchasePrice, next.SalePrice, next.OnhandG, next.OnhandUnits, next.MinLevelG, next.MinLevelUnits, next.Origin, next.ProcessingStation, next.Variety, next.ProcessMethod, next.Grade, next.Altitude, next.Flavor, next.BeanListNote); err != nil {
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
	in.BatchNo = strings.TrimSpace(in.BatchNo)
	in.Origin = strings.TrimSpace(in.Origin)
	in.ProcessingStation = strings.TrimSpace(in.ProcessingStation)
	in.Variety = strings.TrimSpace(in.Variety)
	in.ProcessMethod = strings.TrimSpace(in.ProcessMethod)
	in.Grade = strings.TrimSpace(in.Grade)
	in.Altitude = strings.TrimSpace(in.Altitude)
	in.Flavor = strings.TrimSpace(in.Flavor)
	in.BeanListNote = strings.TrimSpace(in.BeanListNote)
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
	if in.BatchNo == "" {
		in.BatchNo = time.Now().Format("20060102")
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
		{"batch_no", old.BatchNo, next.BatchNo},
		{"purchase_price", fmt.Sprintf("%.2f", old.PurchasePrice), fmt.Sprintf("%.2f", next.PurchasePrice)},
		{"sale_price", fmt.Sprintf("%.2f", old.SalePrice), fmt.Sprintf("%.2f", next.SalePrice)},
		{"onhand_g", fmt.Sprintf("%d", old.OnhandG), fmt.Sprintf("%d", next.OnhandG)},
		{"onhand_units", fmt.Sprintf("%d", old.OnhandUnits), fmt.Sprintf("%d", next.OnhandUnits)},
		{"min_level_g", fmt.Sprintf("%d", old.MinLevelG), fmt.Sprintf("%d", next.MinLevelG)},
		{"min_level_units", fmt.Sprintf("%d", old.MinLevelUnits), fmt.Sprintf("%d", next.MinLevelUnits)},
		{"origin", old.Origin, next.Origin},
		{"processing_station", old.ProcessingStation, next.ProcessingStation},
		{"variety", old.Variety, next.Variety},
		{"process_method", old.ProcessMethod, next.ProcessMethod},
		{"grade", old.Grade, next.Grade},
		{"altitude", old.Altitude, next.Altitude},
		{"flavor", old.Flavor, next.Flavor},
		{"bean_list_note", old.BeanListNote, next.BeanListNote},
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
		ID:                row.ID,
		Code:              row.Code,
		Name:              row.Name,
		Kind:              row.Kind,
		Unit:              row.Unit,
		BatchNo:           row.BatchNo,
		PurchasePrice:     row.PurchasePrice,
		SalePrice:         row.SalePrice,
		OnhandG:           row.OnhandG,
		OnhandUnits:       row.OnhandUnits,
		MinLevelG:         row.MinLevelG,
		MinLevelUnits:     row.MinLevelUnits,
		Origin:            row.Origin,
		ProcessingStation: row.ProcessingStation,
		Variety:           row.Variety,
		ProcessMethod:     row.ProcessMethod,
		Grade:             row.Grade,
		Altitude:          row.Altitude,
		Flavor:            row.Flavor,
		BeanListNote:      row.BeanListNote,
		UpdatedAt:         row.UpdatedAt,
	}
}

func materialInputFromApp(in materialsapp.MaterialInput) materialInput {
	return materialInput{
		Code:              in.Code,
		Name:              in.Name,
		Kind:              in.Kind,
		Unit:              in.Unit,
		BatchNo:           in.BatchNo,
		PurchasePrice:     in.PurchasePrice,
		SalePrice:         in.SalePrice,
		OnhandG:           in.OnhandG,
		OnhandUnits:       in.OnhandUnits,
		MinLevelG:         in.MinLevelG,
		MinLevelUnits:     in.MinLevelUnits,
		Origin:            in.Origin,
		ProcessingStation: in.ProcessingStation,
		Variety:           in.Variety,
		ProcessMethod:     in.ProcessMethod,
		Grade:             in.Grade,
		Altitude:          in.Altitude,
		Flavor:            in.Flavor,
		BeanListNote:      in.BeanListNote,
	}
}
