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
	ID            int64
	Code          string
	Name          string
	Kind          string
	Unit          string
	BatchNo       string
	PurchasePrice float64
	SalePrice     float64
	OnhandG       int64
	OnhandUnits   int64
	MinLevelG     int64
	MinLevelUnits int64
	Profile       *beanProfileInput
	PackProfile   *packProfileInput
	UpdatedAt     string
	DeprecatedAt  string
}

type materialInput struct {
	Code          string
	Name          string
	Kind          string
	Unit          string
	BatchNo       string
	PurchasePrice float64
	SalePrice     float64
	OnhandG       int64
	OnhandUnits   int64
	MinLevelG     int64
	MinLevelUnits int64
	Profile       *beanProfileInput
	PackProfile   *packProfileInput
}

type beanProfileInput struct {
	Origin            string
	ProcessingStation string
	Variety           string
	ProcessMethod     string
	Grade             string
	Altitude          string
	Flavor            string
	BeanListNote      string
}

type packProfileInput struct {
	SizeSpec   string
	Dimensions string
	Material   string
	Capacity   string
	Color      string
	Note       string
}

func NewRepository(pool *pgxpool.Pool, schema string) Repository {
	return Repository{pool: pool, schema: schema}
}

func (r Repository) List(ctx context.Context, cmd materialsapp.ListCommand) ([]materialsapp.Material, error) {
	rows, err := listMaterials(ctx, r.pool, r.schema, cmd.Query, cmd.Limit, cmd.IncludeDeprecated)
	if err != nil {
		return nil, err
	}
	return materialsToApp(rows), nil
}

func (r Repository) Create(ctx context.Context, cmd materialsapp.CreateCommand) (materialsapp.Material, error) {
	row, err := createMaterialInline(ctx, r.pool, r.schema, cmd.Actor, materialInputFromApp(cmd.Input))
	if err != nil {
		return materialsapp.Material{}, err
	}
	return materialToApp(row), nil
}

func (r Repository) Update(ctx context.Context, cmd materialsapp.UpdateCommand) (materialsapp.Material, error) {
	row, err := updateMaterialInline(ctx, r.pool, r.schema, cmd.Actor, cmd.ID, materialInputFromApp(cmd.Input))
	if err != nil {
		return materialsapp.Material{}, err
	}
	return materialToApp(row), nil
}

func (r Repository) Deprecate(ctx context.Context, cmd materialsapp.DeprecateCommand) (materialsapp.Material, error) {
	row, err := deprecateMaterialInline(ctx, r.pool, r.schema, cmd.Actor, cmd.ID)
	if err != nil {
		return materialsapp.Material{}, err
	}
	return materialToApp(row), nil
}

func listMaterials(ctx context.Context, pool *pgxpool.Pool, schema, q string, limit int, includeDeprecated bool) ([]materialRow, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	whereParts := []string{}
	args := []any{}
	argn := 1
	if !includeDeprecated {
		whereParts = append(whereParts, "m.deprecated_at IS NULL")
	}
	if s := strings.TrimSpace(q); s != "" {
		whereParts = append(whereParts, fmt.Sprintf("(m.name ILIKE $%d OR m.code ILIKE $%d OR m.batch_no ILIKE $%d)", argn, argn, argn))
		args = append(args, "%"+s+"%")
		argn++
	}
	where := ""
	if len(whereParts) > 0 {
		where = "WHERE " + strings.Join(whereParts, " AND ")
	}
	args = append(args, limit)
	limitArg := argn

	sql := fmt.Sprintf(`
		SELECT m.id, m.code, m.name, m.kind, m.unit,
		       COALESCE(m.batch_no, ''),
		       COALESCE(m.purchase_price,0), COALESCE(m.sale_price,0),
		       m.onhand_g, m.onhand_units, m.min_level_g, m.min_level_units,
		       COALESCE(bp.origin, ''), COALESCE(bp.processing_station, ''), COALESCE(bp.variety, ''),
		       COALESCE(bp.process_method, ''), COALESCE(bp.grade, ''), COALESCE(bp.altitude, ''),
		       COALESCE(bp.flavor, ''), COALESCE(bp.bean_list_note, ''),
		       COALESCE(pp.size_spec, ''), COALESCE(pp.dimensions, ''), COALESCE(pp.material_texture, ''),
		       COALESCE(pp.capacity, ''), COALESCE(pp.color, ''), COALESCE(pp.note, ''),
		       to_char(m.updated_at,'YYYY-MM-DD HH24:MI'),
		       COALESCE(to_char(m.deprecated_at,'YYYY-MM-DD HH24:MI'), '')
		FROM %s.materials m
		LEFT JOIN %s.material_bean_profiles bp ON bp.material_id = m.id
		LEFT JOIN %s.material_pack_profiles pp ON pp.material_id = m.id
		%s
		ORDER BY m.kind, m.name, m.id DESC
		LIMIT $%d
	`, schema, schema, schema, where, limitArg)
	rows, err := pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]materialRow, 0)
	for rows.Next() {
		var r materialRow
		var profile beanProfileInput
		var packProfile packProfileInput
		if err := rows.Scan(&r.ID, &r.Code, &r.Name, &r.Kind, &r.Unit, &r.BatchNo, &r.PurchasePrice, &r.SalePrice, &r.OnhandG, &r.OnhandUnits, &r.MinLevelG, &r.MinLevelUnits, &profile.Origin, &profile.ProcessingStation, &profile.Variety, &profile.ProcessMethod, &profile.Grade, &profile.Altitude, &profile.Flavor, &profile.BeanListNote, &packProfile.SizeSpec, &packProfile.Dimensions, &packProfile.Material, &packProfile.Capacity, &packProfile.Color, &packProfile.Note, &r.UpdatedAt, &r.DeprecatedAt); err != nil {
			return nil, err
		}
		if r.Kind == "bean" || !profile.empty() {
			r.Profile = &profile
		}
		if r.Kind == "pack" || !packProfile.empty() {
			r.PackProfile = &packProfile
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
	qOld := fmt.Sprintf(`SELECT m.id, m.code, m.name, m.kind, m.unit,
		       COALESCE(m.batch_no, ''),
		       COALESCE(m.purchase_price,0), COALESCE(m.sale_price,0),
		       m.onhand_g, m.onhand_units, m.min_level_g, m.min_level_units,
		       COALESCE(bp.origin, ''), COALESCE(bp.processing_station, ''), COALESCE(bp.variety, ''),
		       COALESCE(bp.process_method, ''), COALESCE(bp.grade, ''), COALESCE(bp.altitude, ''),
		       COALESCE(bp.flavor, ''), COALESCE(bp.bean_list_note, ''),
		       COALESCE(pp.size_spec, ''), COALESCE(pp.dimensions, ''), COALESCE(pp.material_texture, ''),
		       COALESCE(pp.capacity, ''), COALESCE(pp.color, ''), COALESCE(pp.note, ''),
		       to_char(m.updated_at,'YYYY-MM-DD HH24:MI'),
		       COALESCE(to_char(m.deprecated_at,'YYYY-MM-DD HH24:MI'), '')
		FROM %s.materials m
		LEFT JOIN %s.material_bean_profiles bp ON bp.material_id = m.id
		LEFT JOIN %s.material_pack_profiles pp ON pp.material_id = m.id
		WHERE m.id=$1
		FOR UPDATE OF m`, schema, schema, schema)
	var oldProfile beanProfileInput
	var oldPackProfile packProfileInput
	if err := tx.QueryRow(ctx, qOld, id).Scan(&old.ID, &old.Code, &old.Name, &old.Kind, &old.Unit, &old.BatchNo, &old.PurchasePrice, &old.SalePrice, &old.OnhandG, &old.OnhandUnits, &old.MinLevelG, &old.MinLevelUnits, &oldProfile.Origin, &oldProfile.ProcessingStation, &oldProfile.Variety, &oldProfile.ProcessMethod, &oldProfile.Grade, &oldProfile.Altitude, &oldProfile.Flavor, &oldProfile.BeanListNote, &oldPackProfile.SizeSpec, &oldPackProfile.Dimensions, &oldPackProfile.Material, &oldPackProfile.Capacity, &oldPackProfile.Color, &oldPackProfile.Note, &old.UpdatedAt, &old.DeprecatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return materialRow{}, fmt.Errorf("not found")
		}
		return materialRow{}, err
	}
	if old.Kind == "bean" || !oldProfile.empty() {
		old.Profile = &oldProfile
	}
	if old.Kind == "pack" || !oldPackProfile.empty() {
		old.PackProfile = &oldPackProfile
	}
	if old.DeprecatedAt != "" {
		return materialRow{}, fmt.Errorf("material deprecated")
	}
	if err := assertImmutableMaterialFields(old, next); err != nil {
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
			updated_at=now()
		WHERE id=$1`, schema)
	if _, err := tx.Exec(ctx, q, id, next.Code, next.Name, next.Kind, next.Unit, next.BatchNo, next.PurchasePrice, next.SalePrice, next.OnhandG, next.OnhandUnits, next.MinLevelG, next.MinLevelUnits); err != nil {
		return materialRow{}, err
	}
	if err := writeBeanProfileTx(ctx, tx, schema, id, next); err != nil {
		return materialRow{}, err
	}
	if err := writePackProfileTx(ctx, tx, schema, id, next); err != nil {
		return materialRow{}, err
	}
	if err := logMaterialDiffsTx(ctx, tx, schema, actor, old, next); err != nil {
		return materialRow{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return materialRow{}, err
	}

	rows, err := listMaterials(ctx, pool, schema, next.Code, 1, false)
	if err != nil {
		return materialRow{}, err
	}
	if len(rows) == 0 {
		return materialRow{}, fmt.Errorf("not found")
	}
	return rows[0], nil
}

func createMaterialInline(ctx context.Context, pool *pgxpool.Pool, schema, actor string, in materialInput) (materialRow, error) {
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

	q := fmt.Sprintf(`INSERT INTO %s.materials(
			code, name, kind, unit, batch_no, purchase_price, sale_price,
			onhand_g, onhand_units, min_level_g, min_level_units, updated_at
		)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,now())
		RETURNING id`, schema)
	var id int64
	if err := tx.QueryRow(ctx, q, next.Code, next.Name, next.Kind, next.Unit, next.BatchNo, next.PurchasePrice, next.SalePrice, next.OnhandG, next.OnhandUnits, next.MinLevelG, next.MinLevelUnits).Scan(&id); err != nil {
		return materialRow{}, err
	}
	if err := writeBeanProfileTx(ctx, tx, schema, id, next); err != nil {
		return materialRow{}, err
	}
	if err := writePackProfileTx(ctx, tx, schema, id, next); err != nil {
		return materialRow{}, err
	}
	if err := logMaterialCreateTx(ctx, tx, schema, actor, id, next); err != nil {
		return materialRow{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return materialRow{}, err
	}
	return getMaterialByID(ctx, pool, schema, id)
}

func deprecateMaterialInline(ctx context.Context, pool *pgxpool.Pool, schema, actor string, id int64) (materialRow, error) {
	if id <= 0 {
		return materialRow{}, fmt.Errorf("invalid id")
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

	q := fmt.Sprintf(`UPDATE %s.materials
		SET deprecated_at=COALESCE(deprecated_at, now()), updated_at=now()
		WHERE id=$1
		RETURNING code`, schema)
	var code string
	if err := tx.QueryRow(ctx, q, id).Scan(&code); err != nil {
		if err == pgx.ErrNoRows {
			return materialRow{}, fmt.Errorf("not found")
		}
		return materialRow{}, err
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "unknown"
	}
	logQ := fmt.Sprintf(`INSERT INTO %s.audit_logs(actor, entity_type, entity_id, action, field, old_value, new_value, meta)
		VALUES($1,'material',$2,'deprecate','deprecated_at','',to_char(now(),'YYYY-MM-DD HH24:MI'),jsonb_build_object('material_id',$2::bigint,'code',$3::text))`, schema)
	if _, err := tx.Exec(ctx, logQ, actor, id, code); err != nil {
		return materialRow{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return materialRow{}, err
	}
	return getMaterialByID(ctx, pool, schema, id)
}

func getMaterialByID(ctx context.Context, pool *pgxpool.Pool, schema string, id int64) (materialRow, error) {
	rows, err := listMaterials(ctx, pool, schema, "", 500, true)
	if err != nil {
		return materialRow{}, err
	}
	for _, row := range rows {
		if row.ID == id {
			return row, nil
		}
	}
	return materialRow{}, fmt.Errorf("not found")
}

func normalizeMaterialInput(in materialInput) (materialInput, error) {
	in.Code = strings.TrimSpace(in.Code)
	in.Name = strings.TrimSpace(in.Name)
	in.Kind = strings.TrimSpace(in.Kind)
	in.Unit = strings.TrimSpace(in.Unit)
	in.BatchNo = strings.TrimSpace(in.BatchNo)
	if in.Profile != nil {
		in.Profile.normalize()
	}
	if in.PackProfile != nil {
		in.PackProfile.normalize()
	}
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
	if in.Kind == "bean" {
		if in.Profile == nil {
			in.Profile = &beanProfileInput{}
		}
		in.PackProfile = nil
	} else if in.Kind == "pack" {
		if in.PackProfile == nil {
			in.PackProfile = &packProfileInput{}
		}
		in.Profile = nil
	} else {
		in.Profile = nil
		in.PackProfile = nil
	}
	if in.PurchasePrice < 0 || in.SalePrice < 0 || math.IsNaN(in.PurchasePrice) || math.IsNaN(in.SalePrice) || math.IsInf(in.PurchasePrice, 0) || math.IsInf(in.SalePrice, 0) {
		return materialInput{}, fmt.Errorf("negative price")
	}
	if in.OnhandG < 0 || in.OnhandUnits < 0 || in.MinLevelG < 0 || in.MinLevelUnits < 0 {
		return materialInput{}, fmt.Errorf("negative qty")
	}
	return in, nil
}

func assertImmutableMaterialFields(old materialRow, next materialInput) error {
	if old.Code != next.Code ||
		old.Name != next.Name ||
		old.Kind != next.Kind ||
		old.Unit != next.Unit ||
		old.BatchNo != next.BatchNo ||
		fmt.Sprintf("%.2f", old.PurchasePrice) != fmt.Sprintf("%.2f", next.PurchasePrice) ||
		fmt.Sprintf("%.2f", old.SalePrice) != fmt.Sprintf("%.2f", next.SalePrice) {
		return fmt.Errorf("base fields are immutable; copy material to create a new version")
	}
	return nil
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
		{"bean_profile.origin", profileValue(old.Profile, func(p *beanProfileInput) string { return p.Origin }), profileValue(next.Profile, func(p *beanProfileInput) string { return p.Origin })},
		{"bean_profile.processing_station", profileValue(old.Profile, func(p *beanProfileInput) string { return p.ProcessingStation }), profileValue(next.Profile, func(p *beanProfileInput) string { return p.ProcessingStation })},
		{"bean_profile.variety", profileValue(old.Profile, func(p *beanProfileInput) string { return p.Variety }), profileValue(next.Profile, func(p *beanProfileInput) string { return p.Variety })},
		{"bean_profile.process_method", profileValue(old.Profile, func(p *beanProfileInput) string { return p.ProcessMethod }), profileValue(next.Profile, func(p *beanProfileInput) string { return p.ProcessMethod })},
		{"bean_profile.grade", profileValue(old.Profile, func(p *beanProfileInput) string { return p.Grade }), profileValue(next.Profile, func(p *beanProfileInput) string { return p.Grade })},
		{"bean_profile.altitude", profileValue(old.Profile, func(p *beanProfileInput) string { return p.Altitude }), profileValue(next.Profile, func(p *beanProfileInput) string { return p.Altitude })},
		{"bean_profile.flavor", profileValue(old.Profile, func(p *beanProfileInput) string { return p.Flavor }), profileValue(next.Profile, func(p *beanProfileInput) string { return p.Flavor })},
		{"bean_profile.bean_list_note", profileValue(old.Profile, func(p *beanProfileInput) string { return p.BeanListNote }), profileValue(next.Profile, func(p *beanProfileInput) string { return p.BeanListNote })},
		{"pack_profile.size_spec", packProfileValue(old.PackProfile, func(p *packProfileInput) string { return p.SizeSpec }), packProfileValue(next.PackProfile, func(p *packProfileInput) string { return p.SizeSpec })},
		{"pack_profile.dimensions", packProfileValue(old.PackProfile, func(p *packProfileInput) string { return p.Dimensions }), packProfileValue(next.PackProfile, func(p *packProfileInput) string { return p.Dimensions })},
		{"pack_profile.material", packProfileValue(old.PackProfile, func(p *packProfileInput) string { return p.Material }), packProfileValue(next.PackProfile, func(p *packProfileInput) string { return p.Material })},
		{"pack_profile.capacity", packProfileValue(old.PackProfile, func(p *packProfileInput) string { return p.Capacity }), packProfileValue(next.PackProfile, func(p *packProfileInput) string { return p.Capacity })},
		{"pack_profile.color", packProfileValue(old.PackProfile, func(p *packProfileInput) string { return p.Color }), packProfileValue(next.PackProfile, func(p *packProfileInput) string { return p.Color })},
		{"pack_profile.note", packProfileValue(old.PackProfile, func(p *packProfileInput) string { return p.Note }), packProfileValue(next.PackProfile, func(p *packProfileInput) string { return p.Note })},
	} {
		if err := log(item.field, item.old, item.next); err != nil {
			return err
		}
	}
	return nil
}

func logMaterialCreateTx(ctx context.Context, tx pgx.Tx, schema, actor string, id int64, next materialInput) error {
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "unknown"
	}
	q := fmt.Sprintf(`INSERT INTO %s.audit_logs(actor, entity_type, entity_id, action, field, old_value, new_value, meta)
		VALUES($1,'material',$2,'create','material','',$3,jsonb_build_object('material_id',$2::bigint,'code',$4::text))`, schema)
	_, err := tx.Exec(ctx, q, actor, id, next.Name, next.Code)
	return err
}

func writeBeanProfileTx(ctx context.Context, tx pgx.Tx, schema string, materialID int64, in materialInput) error {
	if in.Kind != "bean" {
		_, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.material_bean_profiles WHERE material_id=$1`, schema), materialID)
		return err
	}
	profile := in.Profile
	if profile == nil {
		profile = &beanProfileInput{}
	}
	q := fmt.Sprintf(`INSERT INTO %s.material_bean_profiles(
			material_id, origin, processing_station, variety, process_method,
			grade, altitude, flavor, bean_list_note, updated_at
		)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,now())
		ON CONFLICT(material_id) DO UPDATE SET
			origin=excluded.origin,
			processing_station=excluded.processing_station,
			variety=excluded.variety,
			process_method=excluded.process_method,
			grade=excluded.grade,
			altitude=excluded.altitude,
			flavor=excluded.flavor,
			bean_list_note=excluded.bean_list_note,
			updated_at=now()`, schema)
	_, err := tx.Exec(ctx, q, materialID, profile.Origin, profile.ProcessingStation, profile.Variety, profile.ProcessMethod, profile.Grade, profile.Altitude, profile.Flavor, profile.BeanListNote)
	return err
}

func writePackProfileTx(ctx context.Context, tx pgx.Tx, schema string, materialID int64, in materialInput) error {
	if in.Kind != "pack" {
		_, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.material_pack_profiles WHERE material_id=$1`, schema), materialID)
		return err
	}
	profile := in.PackProfile
	if profile == nil {
		profile = &packProfileInput{}
	}
	q := fmt.Sprintf(`INSERT INTO %s.material_pack_profiles(
			material_id, size_spec, dimensions, material_texture, capacity, color, note, updated_at
		)
		VALUES($1,$2,$3,$4,$5,$6,$7,now())
		ON CONFLICT(material_id) DO UPDATE SET
			size_spec=excluded.size_spec,
			dimensions=excluded.dimensions,
			material_texture=excluded.material_texture,
			capacity=excluded.capacity,
			color=excluded.color,
			note=excluded.note,
			updated_at=now()`, schema)
	_, err := tx.Exec(ctx, q, materialID, profile.SizeSpec, profile.Dimensions, profile.Material, profile.Capacity, profile.Color, profile.Note)
	return err
}

func (p *beanProfileInput) normalize() {
	p.Origin = strings.TrimSpace(p.Origin)
	p.ProcessingStation = strings.TrimSpace(p.ProcessingStation)
	p.Variety = strings.TrimSpace(p.Variety)
	p.ProcessMethod = strings.TrimSpace(p.ProcessMethod)
	p.Grade = strings.TrimSpace(p.Grade)
	p.Altitude = strings.TrimSpace(p.Altitude)
	p.Flavor = strings.TrimSpace(p.Flavor)
	p.BeanListNote = strings.TrimSpace(p.BeanListNote)
}

func (p *beanProfileInput) empty() bool {
	return p == nil ||
		(p.Origin == "" &&
			p.ProcessingStation == "" &&
			p.Variety == "" &&
			p.ProcessMethod == "" &&
			p.Grade == "" &&
			p.Altitude == "" &&
			p.Flavor == "" &&
			p.BeanListNote == "")
}

func (p *packProfileInput) normalize() {
	p.SizeSpec = strings.TrimSpace(p.SizeSpec)
	p.Dimensions = strings.TrimSpace(p.Dimensions)
	p.Material = strings.TrimSpace(p.Material)
	p.Capacity = strings.TrimSpace(p.Capacity)
	p.Color = strings.TrimSpace(p.Color)
	p.Note = strings.TrimSpace(p.Note)
}

func (p *packProfileInput) empty() bool {
	return p == nil ||
		(p.SizeSpec == "" &&
			p.Dimensions == "" &&
			p.Material == "" &&
			p.Capacity == "" &&
			p.Color == "" &&
			p.Note == "")
}

func profileValue(p *beanProfileInput, get func(*beanProfileInput) string) string {
	if p == nil {
		return ""
	}
	return get(p)
}

func packProfileValue(p *packProfileInput, get func(*packProfileInput) string) string {
	if p == nil {
		return ""
	}
	return get(p)
}

func beanProfileToApp(profile *beanProfileInput) *materialsapp.BeanProfile {
	if profile == nil {
		return nil
	}
	return &materialsapp.BeanProfile{
		Origin:            profile.Origin,
		ProcessingStation: profile.ProcessingStation,
		Variety:           profile.Variety,
		ProcessMethod:     profile.ProcessMethod,
		Grade:             profile.Grade,
		Altitude:          profile.Altitude,
		Flavor:            profile.Flavor,
		BeanListNote:      profile.BeanListNote,
	}
}

func beanProfileFromApp(profile *materialsapp.BeanProfile) *beanProfileInput {
	if profile == nil {
		return nil
	}
	return &beanProfileInput{
		Origin:            profile.Origin,
		ProcessingStation: profile.ProcessingStation,
		Variety:           profile.Variety,
		ProcessMethod:     profile.ProcessMethod,
		Grade:             profile.Grade,
		Altitude:          profile.Altitude,
		Flavor:            profile.Flavor,
		BeanListNote:      profile.BeanListNote,
	}
}

func packProfileToApp(profile *packProfileInput) *materialsapp.PackProfile {
	if profile == nil {
		return nil
	}
	return &materialsapp.PackProfile{
		SizeSpec:   profile.SizeSpec,
		Dimensions: profile.Dimensions,
		Material:   profile.Material,
		Capacity:   profile.Capacity,
		Color:      profile.Color,
		Note:       profile.Note,
	}
}

func packProfileFromApp(profile *materialsapp.PackProfile) *packProfileInput {
	if profile == nil {
		return nil
	}
	return &packProfileInput{
		SizeSpec:   profile.SizeSpec,
		Dimensions: profile.Dimensions,
		Material:   profile.Material,
		Capacity:   profile.Capacity,
		Color:      profile.Color,
		Note:       profile.Note,
	}
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
		BatchNo:       row.BatchNo,
		PurchasePrice: row.PurchasePrice,
		SalePrice:     row.SalePrice,
		OnhandG:       row.OnhandG,
		OnhandUnits:   row.OnhandUnits,
		MinLevelG:     row.MinLevelG,
		MinLevelUnits: row.MinLevelUnits,
		BeanProfile:   beanProfileToApp(row.Profile),
		PackProfile:   packProfileToApp(row.PackProfile),
		UpdatedAt:     row.UpdatedAt,
		DeprecatedAt:  row.DeprecatedAt,
	}
}

func materialInputFromApp(in materialsapp.MaterialInput) materialInput {
	return materialInput{
		Code:          in.Code,
		Name:          in.Name,
		Kind:          in.Kind,
		Unit:          in.Unit,
		BatchNo:       in.BatchNo,
		PurchasePrice: in.PurchasePrice,
		SalePrice:     in.SalePrice,
		OnhandG:       in.OnhandG,
		OnhandUnits:   in.OnhandUnits,
		MinLevelG:     in.MinLevelG,
		MinLevelUnits: in.MinLevelUnits,
		Profile:       beanProfileFromApp(in.BeanProfile),
		PackProfile:   packProfileFromApp(in.PackProfile),
	}
}
