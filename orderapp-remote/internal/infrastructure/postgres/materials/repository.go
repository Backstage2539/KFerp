package materials

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	materialsapp "orderapp/internal/application/materials"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool   *pgxpool.Pool
	schema string
}

type materialRow struct {
	ID                         int64
	Code                       string
	Name                       string
	Kind                       string
	IsSemiFinished             bool
	CanManufacture             bool
	Unit                       string
	CostUnit                   string
	BatchNo                    string
	PurchasePrice              float64
	SalePrice                  float64
	OnhandG                    int64
	OnhandUnits                int64
	StockQty                   float64
	MinLevelG                  int64
	MinLevelUnits              int64
	MinLevelQty                float64
	IndustryFieldTemplateID    int64
	IndustryFields             []materialIndustryFieldInput
	ClassificationGroupID      int64
	ClassificationGroupName    string
	ClassificationCategoryID   int64
	ClassificationCategoryName string
	Profile                    *beanProfileInput
	PackProfile                *packProfileInput
	UpdatedAt                  string
	DeprecatedAt               string
}

type materialInput struct {
	Code                    string
	Name                    string
	Kind                    string
	IsSemiFinished          bool
	IsSemiFinishedSet       bool
	Unit                    string
	CostUnit                string
	BatchNo                 string
	PurchasePrice           float64
	SalePrice               float64
	OnhandG                 int64
	OnhandUnits             int64
	MinLevelG               int64
	MinLevelUnits           int64
	MinLevelQty             float64
	IndustryFieldTemplateID int64
	IndustryFields          []materialIndustryFieldInput
	Profile                 *beanProfileInput
	PackProfile             *packProfileInput
}

type materialIndustryFieldInput struct {
	FieldKey  string
	ValueText string
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
	rows, err := listMaterials(ctx, r.pool, r.schema, cmd.Query, cmd.Active, cmd.Limit, cmd.IncludeDeprecated)
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

func (r Repository) ListClassificationGroups(ctx context.Context) ([]materialsapp.MaterialClassificationGroup, error) {
	return listMaterialClassificationGroups(ctx, r.pool, r.schema)
}

func (r Repository) SaveClassificationGroup(ctx context.Context, cmd materialsapp.SaveClassificationGroupCommand) (materialsapp.MaterialClassificationGroup, error) {
	return saveMaterialClassificationGroup(ctx, r.pool, r.schema, cmd)
}

func (r Repository) DeleteClassificationGroup(ctx context.Context, cmd materialsapp.DeleteClassificationGroupCommand) error {
	return deleteMaterialClassificationGroup(ctx, r.pool, r.schema, cmd)
}

func (r Repository) SaveClassificationCategory(ctx context.Context, cmd materialsapp.SaveClassificationCategoryCommand) (materialsapp.MaterialClassificationCategory, error) {
	return saveMaterialClassificationCategory(ctx, r.pool, r.schema, cmd)
}

func (r Repository) DeleteClassificationCategory(ctx context.Context, cmd materialsapp.DeleteClassificationCategoryCommand) error {
	return deleteMaterialClassificationCategory(ctx, r.pool, r.schema, cmd)
}

func (r Repository) AssignClassification(ctx context.Context, cmd materialsapp.AssignClassificationCommand) error {
	return assignMaterialClassification(ctx, r.pool, r.schema, cmd)
}

func listMaterials(ctx context.Context, pool *pgxpool.Pool, schema, q, active string, limit int, includeDeprecated bool) ([]materialRow, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	whereParts := []string{}
	args := []any{}
	argn := 1
	switch strings.TrimSpace(active) {
	case "inactive":
		whereParts = append(whereParts, "m.deprecated_at IS NOT NULL")
	case "all":
	case "":
		if includeDeprecated {
			break
		}
		whereParts = append(whereParts, "m.deprecated_at IS NULL")
	default:
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

	canManufactureSQL, err := materialCanManufactureSQL(ctx, pool, schema)
	if err != nil {
		return nil, err
	}
	sql := fmt.Sprintf(`
		SELECT m.id, m.code, m.name, m.kind, COALESCE(m.is_semi_finished,false), %s, m.unit, m.cost_unit,
		       COALESCE(m.batch_no, ''),
		       COALESCE(m.purchase_price,0), COALESCE(m.sale_price,0),
		       m.onhand_g, m.onhand_units, m.min_level_g, m.min_level_units,
		       COALESCE(m.industry_field_template_id,0),
		       COALESCE(a.group_id,0), COALESCE(g.name,''),
		       COALESCE(a.category_id,0), COALESCE(gc.name,''),
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
		LEFT JOIN %s.material_classification_assignments a ON a.material_id = m.id
		LEFT JOIN %s.material_classification_groups g ON g.id = a.group_id
		LEFT JOIN %s.material_classification_group_categories gc ON gc.id = a.category_id
		%s
		ORDER BY COALESCE(g.sort_order,999999), COALESCE(gc.sort_order,999999), m.name, m.id DESC
		LIMIT $%d
	`, canManufactureSQL, schema, schema, schema, schema, schema, schema, where, limitArg)
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
		if err := rows.Scan(&r.ID, &r.Code, &r.Name, &r.Kind, &r.IsSemiFinished, &r.CanManufacture, &r.Unit, &r.CostUnit, &r.BatchNo, &r.PurchasePrice, &r.SalePrice, &r.OnhandG, &r.OnhandUnits, &r.MinLevelG, &r.MinLevelUnits, &r.IndustryFieldTemplateID, &r.ClassificationGroupID, &r.ClassificationGroupName, &r.ClassificationCategoryID, &r.ClassificationCategoryName, &profile.Origin, &profile.ProcessingStation, &profile.Variety, &profile.ProcessMethod, &profile.Grade, &profile.Altitude, &profile.Flavor, &profile.BeanListNote, &packProfile.SizeSpec, &packProfile.Dimensions, &packProfile.Material, &packProfile.Capacity, &packProfile.Color, &packProfile.Note, &r.UpdatedAt, &r.DeprecatedAt); err != nil {
			return nil, err
		}
		r.Kind = normalizeMaterialKind(r.Kind)
		r.StockQty = materialQtyForUnit(r.Unit, r.OnhandG, r.OnhandUnits)
		r.MinLevelQty = materialQtyForUnit(r.Unit, r.MinLevelG, r.MinLevelUnits)
		if r.Kind == "bean" || !profile.empty() {
			r.Profile = &profile
		}
		if r.Kind == "pack" || !packProfile.empty() {
			r.PackProfile = &packProfile
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := attachMaterialIndustryFields(ctx, pool, schema, out); err != nil {
		return nil, err
	}
	return out, nil
}

func materialCanManufactureSQL(ctx context.Context, pool *pgxpool.Pool, schema string) (string, error) {
	for _, table := range []string{"production_boms", "production_bom_versions", "production_bom_output_bindings"} {
		var exists bool
		if err := pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, schema+"."+table).Scan(&exists); err != nil {
			return "", err
		}
		if !exists {
			return "false", nil
		}
	}
	return fmt.Sprintf(`EXISTS (
		SELECT 1
		FROM %[1]s.production_bom_output_bindings ob
		JOIN %[1]s.production_boms pb ON pb.id=ob.bom_id
		JOIN %[1]s.production_bom_versions v ON v.id=ob.bom_version_id AND v.bom_id=pb.id
		WHERE ob.output_type='material'
		  AND ob.output_id=m.id
		  AND ob.is_default=true
		  AND pb.output_type='material'
		  AND pb.output_material_id=m.id
		  AND COALESCE(NULLIF(pb.status,''),'active')='active'
		  AND v.status='published'
		  AND m.deprecated_at IS NULL
	)`, schema), nil
}

func updateMaterialInline(ctx context.Context, pool *pgxpool.Pool, schema, actor string, id int64, in materialInput) (materialRow, error) {
	if id <= 0 {
		return materialRow{}, fmt.Errorf("invalid id")
	}
	requestedInventoryUnit := strings.TrimSpace(in.Unit)
	requestedCostUnit := strings.TrimSpace(in.CostUnit)
	in.CostUnit = ""
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
	qOld := fmt.Sprintf(`SELECT m.id, m.code, m.name, m.kind, COALESCE(m.is_semi_finished,false), m.unit, m.cost_unit,
		       COALESCE(m.batch_no, ''),
		       COALESCE(m.purchase_price,0), COALESCE(m.sale_price,0),
		       m.onhand_g, m.onhand_units, m.min_level_g, m.min_level_units,
		       COALESCE(m.industry_field_template_id,0),
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
	if err := tx.QueryRow(ctx, qOld, id).Scan(&old.ID, &old.Code, &old.Name, &old.Kind, &old.IsSemiFinished, &old.Unit, &old.CostUnit, &old.BatchNo, &old.PurchasePrice, &old.SalePrice, &old.OnhandG, &old.OnhandUnits, &old.MinLevelG, &old.MinLevelUnits, &old.IndustryFieldTemplateID, &oldProfile.Origin, &oldProfile.ProcessingStation, &oldProfile.Variety, &oldProfile.ProcessMethod, &oldProfile.Grade, &oldProfile.Altitude, &oldProfile.Flavor, &oldProfile.BeanListNote, &oldPackProfile.SizeSpec, &oldPackProfile.Dimensions, &oldPackProfile.Material, &oldPackProfile.Capacity, &oldPackProfile.Color, &oldPackProfile.Note, &old.UpdatedAt, &old.DeprecatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return materialRow{}, fmt.Errorf("not found")
		}
		return materialRow{}, err
	}
	old.Kind = normalizeMaterialKind(old.Kind)
	if old.Kind == "bean" || !oldProfile.empty() {
		old.Profile = &oldProfile
	}
	if old.Kind == "pack" || !oldPackProfile.empty() {
		old.PackProfile = &oldPackProfile
	}
	if old.DeprecatedAt != "" {
		return materialRow{}, fmt.Errorf("material deprecated")
	}
	old.IndustryFields = loadMaterialIndustryFieldsForTx(ctx, tx, schema, id)
	if !next.IsSemiFinishedSet {
		next.IsSemiFinished = old.IsSemiFinished
	}
	unitChanged := materialInventoryUnitChanged(old, next, requestedInventoryUnit)
	if unitChanged {
		inUse, err := materialInventoryUnitInUseTx(ctx, tx, schema, old)
		if err != nil {
			return materialRow{}, fmt.Errorf("库存单位使用情况无法确认，拒绝修改: %w", err)
		}
		if err := assertMaterialInventoryUnitReadOnly(old, next, requestedInventoryUnit, inUse); err != nil {
			return materialRow{}, err
		}
	} else {
		next.Unit = old.Unit
	}
	if err := assertMaterialCostUnitReadOnly(old, next, requestedCostUnit, unitChanged); err != nil {
		return materialRow{}, err
	}
	next.CostUnit = old.CostUnit
	if err := assertMaterialStockFieldsReadOnly(old, next); err != nil {
		return materialRow{}, err
	}

	q := fmt.Sprintf(`UPDATE %s.materials SET
			code=$2,
			name=$3,
			kind=$4,
			is_semi_finished=$5,
			unit=$6,
			cost_unit=$7,
			batch_no=$8,
			purchase_price=$9,
			sale_price=$10,
			onhand_g=$11,
			onhand_units=$12,
			min_level_g=$13,
			min_level_units=$14,
			industry_field_template_id=$15,
			updated_at=now()
		WHERE id=$1`, schema)
	if _, err := tx.Exec(ctx, q, id, next.Code, next.Name, next.Kind, next.IsSemiFinished, next.Unit, next.CostUnit, next.BatchNo, next.PurchasePrice, next.SalePrice, next.OnhandG, next.OnhandUnits, next.MinLevelG, next.MinLevelUnits, next.IndustryFieldTemplateID); err != nil {
		return materialRow{}, err
	}
	if err := writeBeanProfileTx(ctx, tx, schema, id, next); err != nil {
		return materialRow{}, err
	}
	if err := writePackProfileTx(ctx, tx, schema, id, next); err != nil {
		return materialRow{}, err
	}
	if err := writeMaterialIndustryFieldsTx(ctx, tx, schema, actor, id, next); err != nil {
		return materialRow{}, err
	}
	if err := logMaterialDiffsTx(ctx, tx, schema, actor, old, next); err != nil {
		return materialRow{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return materialRow{}, err
	}

	rows, err := listMaterials(ctx, pool, schema, next.Code, "active", 1, false)
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
			code, name, kind, is_semi_finished, unit, cost_unit, batch_no, purchase_price, sale_price,
			onhand_g, onhand_units, min_level_g, min_level_units, industry_field_template_id, updated_at
		)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,now())
		RETURNING id`, schema)
	var id int64
	if err := tx.QueryRow(ctx, q, next.Code, next.Name, next.Kind, next.IsSemiFinished, next.Unit, next.CostUnit, next.BatchNo, next.PurchasePrice, next.SalePrice, next.OnhandG, next.OnhandUnits, next.MinLevelG, next.MinLevelUnits, next.IndustryFieldTemplateID).Scan(&id); err != nil {
		return materialRow{}, err
	}
	if err := writeBeanProfileTx(ctx, tx, schema, id, next); err != nil {
		return materialRow{}, err
	}
	if err := writePackProfileTx(ctx, tx, schema, id, next); err != nil {
		return materialRow{}, err
	}
	if err := writeMaterialIndustryFieldsTx(ctx, tx, schema, actor, id, next); err != nil {
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
	rows, err := listMaterials(ctx, pool, schema, "", "all", 500, true)
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
	in.CostUnit = normalizeMaterialCostUnit(in.CostUnit)
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
	in.Kind = normalizeMaterialKind(in.Kind)
	if in.Unit == "" {
		in.Unit = "g"
	}
	if isMaterialWeightUnit(in.Unit) {
		if in.CostUnit == "" {
			in.CostUnit = "kg"
		}
		if in.CostUnit != "kg" {
			return materialInput{}, fmt.Errorf("重量物料成本计价单位必须为 kg")
		}
	} else {
		if in.CostUnit == "" {
			in.CostUnit = in.Unit
		}
		if in.CostUnit != in.Unit {
			return materialInput{}, fmt.Errorf("非重量物料成本计价单位必须与库存单位一致")
		}
	}
	if in.MinLevelQty > 0 && in.MinLevelG == 0 && in.MinLevelUnits == 0 {
		in.MinLevelG, in.MinLevelUnits = quantityToLegacy(in.Unit, in.MinLevelQty)
	}
	in.IndustryFields = normalizeMaterialIndustryFields(in.IndustryFields)
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

func normalizeMaterialIndustryFields(fields []materialIndustryFieldInput) []materialIndustryFieldInput {
	seen := map[string]bool{}
	out := make([]materialIndustryFieldInput, 0, len(fields))
	for _, field := range fields {
		key := strings.TrimSpace(field.FieldKey)
		if key == "" {
			continue
		}
		lookup := strings.ToLower(key)
		if seen[lookup] {
			continue
		}
		seen[lookup] = true
		out = append(out, materialIndustryFieldInput{
			FieldKey:  key,
			ValueText: strings.TrimSpace(field.ValueText),
		})
	}
	return out
}

func normalizeMaterialKind(kind string) string {
	switch strings.TrimSpace(kind) {
	case "raw_bean", "raw-bean", "green_bean", "green-bean":
		return "bean"
	case "packaging", "package", "packing":
		return "pack"
	default:
		return strings.TrimSpace(kind)
	}
}

func normalizeMaterialCostUnit(unit string) string {
	unit = strings.TrimSpace(unit)
	switch strings.ToLower(unit) {
	case "kg", "千克":
		return "kg"
	default:
		return unit
	}
}

func isMaterialWeightUnit(unit string) bool {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "g", "kg", "lb", "oz", "克", "千克":
		return true
	default:
		return false
	}
}

func materialQtyForUnit(unit string, qtyG, qtyUnits int64) float64 {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "kg":
		return float64(qtyG) / 1000
	case "lb":
		return float64(qtyG) / 453.59237
	case "oz":
		return float64(qtyG) / 28.349523125
	case "g", "克":
		return float64(qtyG)
	default:
		if qtyUnits != 0 {
			return float64(qtyUnits)
		}
		return float64(qtyG)
	}
}

func quantityToLegacy(unit string, qty float64) (int64, int64) {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "kg":
		return int64(math.Round(qty * 1000)), 0
	case "lb":
		return int64(math.Round(qty * 453.59237)), 0
	case "oz":
		return int64(math.Round(qty * 28.349523125)), 0
	case "g", "克":
		return int64(math.Round(qty)), 0
	default:
		return 0, int64(math.Round(qty))
	}
}

func assertMaterialStockFieldsReadOnly(old materialRow, next materialInput) error {
	if old.OnhandG != next.OnhandG || old.OnhandUnits != next.OnhandUnits {
		return fmt.Errorf("stock fields are read-only; use stock adjustment")
	}
	return nil
}

func materialInventoryUnitChanged(old materialRow, next materialInput, requestedInventoryUnit string) bool {
	if strings.TrimSpace(requestedInventoryUnit) == "" {
		return false
	}
	oldUnit := strings.TrimSpace(old.Unit)
	if oldUnit == "" {
		oldUnit = "g"
	}
	nextUnit := strings.TrimSpace(next.Unit)
	if nextUnit == "" {
		nextUnit = oldUnit
	}
	return oldUnit != nextUnit
}

func assertMaterialInventoryUnitReadOnly(old materialRow, next materialInput, requestedInventoryUnit string, inUse bool) error {
	if materialInventoryUnitChanged(old, next, requestedInventoryUnit) && inUse {
		return fmt.Errorf("库存单位保存后不能修改；如需调整，请新建物料档案")
	}
	return nil
}

func materialInventoryUnitInUseTx(ctx context.Context, tx pgx.Tx, schema string, old materialRow) (bool, error) {
	if old.OnhandG != 0 || old.OnhandUnits != 0 {
		return true, nil
	}
	checks := []struct {
		tables []string
		query  string
	}{
		{[]string{"material_batch_locations"}, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.material_batch_locations WHERE material_id=$1 AND (qty_g<>0 OR qty_units<>0))`, schema)},
		{[]string{"material_batches"}, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.material_batches WHERE material_id=$1 AND (remaining_g<>0 OR remaining_units<>0))`, schema)},
		{[]string{"production_boms", "production_bom_versions", "production_bom_version_items"}, fmt.Sprintf(`SELECT EXISTS(
			SELECT 1 FROM %[1]s.production_boms pb JOIN %[1]s.production_bom_versions v ON v.bom_id=pb.id
			WHERE v.status='published' AND pb.output_type='material' AND pb.output_material_id=$1
			UNION ALL
			SELECT 1 FROM %[1]s.production_bom_version_items i JOIN %[1]s.production_bom_versions v ON v.id=i.version_id
			WHERE v.status='published' AND COALESCE(NULLIF(i.component_type,''),'material')='material' AND i.material_id=$1
		)`, schema)},
		{[]string{"work_order_material_reservations", "work_orders"}, fmt.Sprintf(`SELECT EXISTS(
			SELECT 1 FROM %[1]s.work_order_material_reservations r
			JOIN %[1]s.work_orders wo ON wo.id=r.work_order_id
			WHERE r.material_id=$1 AND lower(COALESCE(wo.status,'')) NOT IN ('completed','cancelled','canceled','closed')
		)`, schema)},
	}
	for _, check := range checks {
		allExist := true
		for _, table := range check.tables {
			var exists bool
			if err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, schema+"."+table).Scan(&exists); err != nil {
				return false, err
			}
			if !exists {
				allExist = false
				break
			}
		}
		if !allExist {
			continue
		}
		var used bool
		if err := tx.QueryRow(ctx, check.query, old.ID).Scan(&used); err != nil {
			return false, err
		}
		if used {
			return true, nil
		}
	}
	workOrderOutputColumns := []string{"output_type", "output_material_id", "status"}
	workOrderOutputAvailable := true
	for _, column := range workOrderOutputColumns {
		exists, err := materialSchemaColumnExistsTx(ctx, tx, schema, "work_orders", column)
		if err != nil {
			return false, err
		}
		if !exists {
			workOrderOutputAvailable = false
			break
		}
	}
	if workOrderOutputAvailable {
		var used bool
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT EXISTS(
				SELECT 1 FROM %s.work_orders
				WHERE output_type='material'
				  AND output_material_id=$1
				  AND lower(COALESCE(status,'')) NOT IN ('completed','cancelled','canceled','closed')
			)
		`, schema), old.ID).Scan(&used); err != nil {
			return false, err
		}
		if used {
			return true, nil
		}
	}
	return false, nil
}

func materialSchemaColumnExistsTx(ctx context.Context, tx pgx.Tx, schema, table, column string) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema=$1 AND table_name=$2 AND column_name=$3
		)
	`, schema, table, column).Scan(&exists)
	return exists, err
}

func assertMaterialCostUnitReadOnly(old materialRow, next materialInput, requestedCostUnit string, inventoryUnitChanged bool) error {
	requestedCostUnit = normalizeMaterialCostUnit(requestedCostUnit)
	oldCostUnit := normalizeMaterialCostUnit(old.CostUnit)
	if oldCostUnit == "" {
		oldCostUnit = normalizeMaterialCostUnit(next.CostUnit)
	}
	if requestedCostUnit != "" && oldCostUnit != requestedCostUnit {
		return fmt.Errorf("成本计价单位保存后不能修改；如需调整，请新建物料档案")
	}
	if inventoryUnitChanged && oldCostUnit != normalizeMaterialCostUnit(next.CostUnit) {
		return fmt.Errorf("成本计价单位保存后不能修改；如需调整，请新建物料档案")
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
		{"is_semi_finished", fmt.Sprintf("%t", old.IsSemiFinished), fmt.Sprintf("%t", next.IsSemiFinished)},
		{"unit", old.Unit, next.Unit},
		{"cost_unit", old.CostUnit, next.CostUnit},
		{"batch_no", old.BatchNo, next.BatchNo},
		{"purchase_price", fmt.Sprintf("%.2f", old.PurchasePrice), fmt.Sprintf("%.2f", next.PurchasePrice)},
		{"sale_price", fmt.Sprintf("%.2f", old.SalePrice), fmt.Sprintf("%.2f", next.SalePrice)},
		{"onhand_g", fmt.Sprintf("%d", old.OnhandG), fmt.Sprintf("%d", next.OnhandG)},
		{"onhand_units", fmt.Sprintf("%d", old.OnhandUnits), fmt.Sprintf("%d", next.OnhandUnits)},
		{"min_level_g", fmt.Sprintf("%d", old.MinLevelG), fmt.Sprintf("%d", next.MinLevelG)},
		{"min_level_units", fmt.Sprintf("%d", old.MinLevelUnits), fmt.Sprintf("%d", next.MinLevelUnits)},
		{"industry_field_template_id", fmt.Sprintf("%d", old.IndustryFieldTemplateID), fmt.Sprintf("%d", next.IndustryFieldTemplateID)},
		{"industry_fields", materialIndustryFieldsString(old.IndustryFields), materialIndustryFieldsString(next.IndustryFields)},
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
	if _, err := tx.Exec(ctx, q, actor, id, next.Name, next.Code); err != nil {
		return err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, schema, actor, "material", &id, "create", postgresinfra.StrPtr("is_semi_finished"), postgresinfra.StrPtr(""), postgresinfra.StrPtr(fmt.Sprintf("%t", next.IsSemiFinished)), postgresinfra.AuditMeta{"material_id": id, "code": next.Code}); err != nil {
		return err
	}
	costUnitQ := fmt.Sprintf(`INSERT INTO %s.audit_logs(actor, entity_type, entity_id, action, field, old_value, new_value, meta)
		VALUES($1,'material',$2,'create','cost_unit','',$3,jsonb_build_object('material_id',$2::bigint,'code',$4::text))`, schema)
	_, err := tx.Exec(ctx, costUnitQ, actor, id, next.CostUnit, next.Code)
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

func loadMaterialIndustryFieldsForTx(ctx context.Context, tx pgx.Tx, schema string, materialID int64) []materialIndustryFieldInput {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT field_key, value_text
		FROM %s.material_industry_field_values
		WHERE material_id=$1
		ORDER BY field_key`, schema), materialID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []materialIndustryFieldInput{}
	for rows.Next() {
		var row materialIndustryFieldInput
		if err := rows.Scan(&row.FieldKey, &row.ValueText); err == nil {
			out = append(out, row)
		}
	}
	return out
}

func attachMaterialIndustryFields(ctx context.Context, pool *pgxpool.Pool, schema string, rows []materialRow) error {
	if len(rows) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(rows))
	byID := map[int64]int{}
	for idx := range rows {
		ids = append(ids, rows[idx].ID)
		byID[rows[idx].ID] = idx
	}
	q := fmt.Sprintf(`
		SELECT material_id, field_key, value_text
		FROM %s.material_industry_field_values
		WHERE material_id = ANY($1)
		ORDER BY field_key`, schema)
	fieldRows, err := pool.Query(ctx, q, ids)
	if err != nil {
		return err
	}
	defer fieldRows.Close()
	for fieldRows.Next() {
		var materialID int64
		var field materialIndustryFieldInput
		if err := fieldRows.Scan(&materialID, &field.FieldKey, &field.ValueText); err != nil {
			return err
		}
		if idx, ok := byID[materialID]; ok {
			rows[idx].IndustryFields = append(rows[idx].IndustryFields, field)
		}
	}
	return fieldRows.Err()
}

func writeMaterialIndustryFieldsTx(ctx context.Context, tx pgx.Tx, schema, actor string, materialID int64, in materialInput) error {
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.material_industry_field_values WHERE material_id=$1`, schema), materialID); err != nil {
		return err
	}
	for _, field := range in.IndustryFields {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.material_industry_field_values(material_id, field_key, value_text, updated_at, updated_by)
			VALUES($1,$2,$3,now(),$4)`, schema), materialID, field.FieldKey, field.ValueText, actor); err != nil {
			return err
		}
	}
	return nil
}

func listMaterialClassificationGroups(ctx context.Context, pool *pgxpool.Pool, schema string) ([]materialsapp.MaterialClassificationGroup, error) {
	groupRows, err := pool.Query(ctx, fmt.Sprintf(`
		SELECT id, name, sort_order
		FROM %s.material_classification_groups
		ORDER BY sort_order, id`, schema))
	if err != nil {
		return nil, err
	}
	defer groupRows.Close()
	groups := make([]materialsapp.MaterialClassificationGroup, 0)
	groupIndex := map[int64]int{}
	for groupRows.Next() {
		var group materialsapp.MaterialClassificationGroup
		if err := groupRows.Scan(&group.ID, &group.Name, &group.SortOrder); err != nil {
			return nil, err
		}
		group.Categories = []materialsapp.MaterialClassificationCategory{}
		groupIndex[group.ID] = len(groups)
		groups = append(groups, group)
	}
	if err := groupRows.Err(); err != nil {
		return nil, err
	}
	categoryRows, err := pool.Query(ctx, fmt.Sprintf(`
		SELECT id, group_id, name, sort_order
		FROM %s.material_classification_group_categories
		ORDER BY group_id, sort_order, id`, schema))
	if err != nil {
		return nil, err
	}
	defer categoryRows.Close()
	for categoryRows.Next() {
		var category materialsapp.MaterialClassificationCategory
		if err := categoryRows.Scan(&category.ID, &category.GroupID, &category.Name, &category.SortOrder); err != nil {
			return nil, err
		}
		if idx, ok := groupIndex[category.GroupID]; ok {
			groups[idx].Categories = append(groups[idx].Categories, category)
		}
	}
	return groups, categoryRows.Err()
}

func saveMaterialClassificationGroup(ctx context.Context, pool *pgxpool.Pool, schema string, cmd materialsapp.SaveClassificationGroupCommand) (materialsapp.MaterialClassificationGroup, error) {
	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return materialsapp.MaterialClassificationGroup{}, fmt.Errorf("name required")
	}
	if cmd.SortOrder <= 0 {
		cmd.SortOrder = 100
	}
	actor := strings.TrimSpace(cmd.Actor)
	if actor == "" {
		actor = "materials"
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return materialsapp.MaterialClassificationGroup{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var id int64
	action := "create"
	if cmd.ID > 0 {
		action = "update"
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			UPDATE %s.material_classification_groups
			SET name=$2, sort_order=$3, updated_at=now()
			WHERE id=$1
			RETURNING id`, schema), cmd.ID, name, cmd.SortOrder).Scan(&id); err != nil {
			return materialsapp.MaterialClassificationGroup{}, err
		}
	} else {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.material_classification_groups(name, sort_order, created_at, updated_at)
			VALUES($1,$2,now(),now())
			RETURNING id`, schema), name, cmd.SortOrder).Scan(&id); err != nil {
			return materialsapp.MaterialClassificationGroup{}, err
		}
	}
	if err := logMaterialClassificationTx(ctx, tx, schema, actor, "material_classification_group", id, action, "group", "", name); err != nil {
		return materialsapp.MaterialClassificationGroup{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return materialsapp.MaterialClassificationGroup{}, err
	}
	groups, err := listMaterialClassificationGroups(ctx, pool, schema)
	if err != nil {
		return materialsapp.MaterialClassificationGroup{}, err
	}
	for _, group := range groups {
		if group.ID == id {
			return group, nil
		}
	}
	return materialsapp.MaterialClassificationGroup{}, fmt.Errorf("not found")
}

func deleteMaterialClassificationGroup(ctx context.Context, pool *pgxpool.Pool, schema string, cmd materialsapp.DeleteClassificationGroupCommand) error {
	if cmd.ID <= 0 {
		return fmt.Errorf("invalid id")
	}
	actor := strings.TrimSpace(cmd.Actor)
	if actor == "" {
		actor = "materials"
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var name string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT name FROM %s.material_classification_groups WHERE id=$1`, schema), cmd.ID).Scan(&name); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.material_classification_assignments WHERE group_id=$1`, schema), cmd.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.material_classification_groups WHERE id=$1`, schema), cmd.ID); err != nil {
		return err
	}
	if err := logMaterialClassificationTx(ctx, tx, schema, actor, "material_classification_group", cmd.ID, "delete", "group", name, ""); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func saveMaterialClassificationCategory(ctx context.Context, pool *pgxpool.Pool, schema string, cmd materialsapp.SaveClassificationCategoryCommand) (materialsapp.MaterialClassificationCategory, error) {
	if cmd.GroupID <= 0 {
		return materialsapp.MaterialClassificationCategory{}, fmt.Errorf("group required")
	}
	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return materialsapp.MaterialClassificationCategory{}, fmt.Errorf("name required")
	}
	if cmd.SortOrder <= 0 {
		cmd.SortOrder = 100
	}
	actor := strings.TrimSpace(cmd.Actor)
	if actor == "" {
		actor = "materials"
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return materialsapp.MaterialClassificationCategory{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var exists bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.material_classification_groups WHERE id=$1)`, schema), cmd.GroupID).Scan(&exists); err != nil {
		return materialsapp.MaterialClassificationCategory{}, err
	}
	if !exists {
		return materialsapp.MaterialClassificationCategory{}, fmt.Errorf("group not found")
	}
	var id int64
	action := "create"
	if cmd.ID > 0 {
		action = "update"
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			UPDATE %s.material_classification_group_categories
			SET group_id=$2, name=$3, sort_order=$4, updated_at=now()
			WHERE id=$1
			RETURNING id`, schema), cmd.ID, cmd.GroupID, name, cmd.SortOrder).Scan(&id); err != nil {
			return materialsapp.MaterialClassificationCategory{}, err
		}
	} else {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.material_classification_group_categories(group_id, name, sort_order, created_at, updated_at)
			VALUES($1,$2,$3,now(),now())
			RETURNING id`, schema), cmd.GroupID, name, cmd.SortOrder).Scan(&id); err != nil {
			return materialsapp.MaterialClassificationCategory{}, err
		}
	}
	if err := logMaterialClassificationTx(ctx, tx, schema, actor, "material_classification_category", id, action, "category", "", name); err != nil {
		return materialsapp.MaterialClassificationCategory{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return materialsapp.MaterialClassificationCategory{}, err
	}
	return materialsapp.MaterialClassificationCategory{ID: id, GroupID: cmd.GroupID, Name: name, SortOrder: cmd.SortOrder}, nil
}

func deleteMaterialClassificationCategory(ctx context.Context, pool *pgxpool.Pool, schema string, cmd materialsapp.DeleteClassificationCategoryCommand) error {
	if cmd.ID <= 0 {
		return fmt.Errorf("invalid id")
	}
	actor := strings.TrimSpace(cmd.Actor)
	if actor == "" {
		actor = "materials"
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var name string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT name FROM %s.material_classification_group_categories WHERE id=$1`, schema), cmd.ID).Scan(&name); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.material_classification_assignments
		SET category_id=0, updated_at=now()
		WHERE category_id=$1`, schema), cmd.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.material_classification_group_categories WHERE id=$1`, schema), cmd.ID); err != nil {
		return err
	}
	if err := logMaterialClassificationTx(ctx, tx, schema, actor, "material_classification_category", cmd.ID, "delete", "category", name, ""); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func assignMaterialClassification(ctx context.Context, pool *pgxpool.Pool, schema string, cmd materialsapp.AssignClassificationCommand) error {
	if len(cmd.MaterialIDs) == 0 {
		return fmt.Errorf("material_ids required")
	}
	actor := strings.TrimSpace(cmd.Actor)
	if actor == "" {
		actor = "materials"
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if cmd.GroupID > 0 {
		var exists bool
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.material_classification_groups WHERE id=$1)`, schema), cmd.GroupID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("group not found")
		}
	}
	if cmd.CategoryID > 0 {
		var exists bool
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.material_classification_group_categories WHERE id=$1 AND group_id=$2)`, schema), cmd.CategoryID, cmd.GroupID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("category not found")
		}
	}
	for _, materialID := range cmd.MaterialIDs {
		if materialID <= 0 {
			continue
		}
		if cmd.GroupID <= 0 {
			if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.material_classification_assignments WHERE material_id=$1`, schema), materialID); err != nil {
				return err
			}
		} else {
			if _, err := tx.Exec(ctx, fmt.Sprintf(`
				INSERT INTO %s.material_classification_assignments(material_id, group_id, category_id, updated_at, updated_by)
				VALUES($1,$2,$3,now(),$4)
				ON CONFLICT(material_id) DO UPDATE SET
					group_id=excluded.group_id,
					category_id=excluded.category_id,
					updated_at=now(),
					updated_by=excluded.updated_by`, schema), materialID, cmd.GroupID, cmd.CategoryID, actor); err != nil {
				return err
			}
		}
		if err := logMaterialClassificationTx(ctx, tx, schema, actor, "material", materialID, "classify", "material_classification", "", fmt.Sprintf("%d/%d", cmd.GroupID, cmd.CategoryID)); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func logMaterialClassificationTx(ctx context.Context, tx pgx.Tx, schema, actor, entityType string, entityID int64, action, field, oldValue, newValue string) error {
	q := fmt.Sprintf(`INSERT INTO %s.audit_logs(actor, entity_type, entity_id, action, field, old_value, new_value, meta)
		VALUES($1,$2,$3,$4,$5,$6,$7,jsonb_build_object('entity_type',$2::text,'entity_id',$3::bigint))`, schema)
	_, err := tx.Exec(ctx, q, actor, entityType, entityID, action, field, oldValue, newValue)
	return err
}

func materialIndustryFieldsString(fields []materialIndustryFieldInput) string {
	if len(fields) == 0 {
		return ""
	}
	parts := make([]string, 0, len(fields))
	for _, field := range normalizeMaterialIndustryFields(fields) {
		parts = append(parts, field.FieldKey+"="+field.ValueText)
	}
	return strings.Join(parts, ";")
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
		ID:                         row.ID,
		Code:                       row.Code,
		Name:                       row.Name,
		Kind:                       row.Kind,
		IsSemiFinished:             row.IsSemiFinished,
		CanManufacture:             row.CanManufacture,
		Unit:                       row.Unit,
		CostUnit:                   row.CostUnit,
		BatchNo:                    row.BatchNo,
		PurchasePrice:              row.PurchasePrice,
		SalePrice:                  row.SalePrice,
		OnhandG:                    row.OnhandG,
		OnhandUnits:                row.OnhandUnits,
		StockQty:                   row.StockQty,
		MinLevelG:                  row.MinLevelG,
		MinLevelUnits:              row.MinLevelUnits,
		MinLevelQty:                row.MinLevelQty,
		IndustryFieldTemplateID:    row.IndustryFieldTemplateID,
		IndustryFields:             materialIndustryFieldsToApp(row.IndustryFields),
		ClassificationGroupID:      row.ClassificationGroupID,
		ClassificationGroupName:    row.ClassificationGroupName,
		ClassificationCategoryID:   row.ClassificationCategoryID,
		ClassificationCategoryName: row.ClassificationCategoryName,
		BeanProfile:                beanProfileToApp(row.Profile),
		PackProfile:                packProfileToApp(row.PackProfile),
		UpdatedAt:                  row.UpdatedAt,
		DeprecatedAt:               row.DeprecatedAt,
	}
}

func materialInputFromApp(in materialsapp.MaterialInput) materialInput {
	return materialInput{
		Code:                    in.Code,
		Name:                    in.Name,
		Kind:                    in.Kind,
		IsSemiFinished:          in.IsSemiFinished,
		IsSemiFinishedSet:       in.IsSemiFinishedSet,
		Unit:                    in.Unit,
		CostUnit:                in.CostUnit,
		BatchNo:                 in.BatchNo,
		PurchasePrice:           in.PurchasePrice,
		SalePrice:               in.SalePrice,
		OnhandG:                 in.OnhandG,
		OnhandUnits:             in.OnhandUnits,
		MinLevelG:               in.MinLevelG,
		MinLevelUnits:           in.MinLevelUnits,
		MinLevelQty:             in.MinLevelQty,
		IndustryFieldTemplateID: in.IndustryFieldTemplateID,
		IndustryFields:          materialIndustryFieldsFromApp(in.IndustryFields),
		Profile:                 beanProfileFromApp(in.BeanProfile),
		PackProfile:             packProfileFromApp(in.PackProfile),
	}
}

func materialIndustryFieldsToApp(fields []materialIndustryFieldInput) []materialsapp.MaterialIndustryFieldValue {
	out := make([]materialsapp.MaterialIndustryFieldValue, 0, len(fields))
	for _, field := range fields {
		out = append(out, materialsapp.MaterialIndustryFieldValue{
			FieldKey:  field.FieldKey,
			ValueText: field.ValueText,
		})
	}
	return out
}

func materialIndustryFieldsFromApp(fields []materialsapp.MaterialIndustryFieldValue) []materialIndustryFieldInput {
	out := make([]materialIndustryFieldInput, 0, len(fields))
	for _, field := range fields {
		out = append(out, materialIndustryFieldInput{
			FieldKey:  field.FieldKey,
			ValueText: field.ValueText,
		})
	}
	return out
}
