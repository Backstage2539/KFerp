package bom

import (
	"context"
	"fmt"
	"strings"

	bomapp "orderapp/internal/application/bom"
	bomdomain "orderapp/internal/domain/bom"
	catalogdomain "orderapp/internal/domain/catalog"
	productiondomain "orderapp/internal/domain/production"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool   *pgxpool.Pool
	schema string
}

func NewRepository(pool *pgxpool.Pool, schema string) Repository {
	return Repository{pool: pool, schema: schema}
}

func (r Repository) List(ctx context.Context) ([]bomapp.ListItem, error) {
	q := fmt.Sprintf(`
		SELECT
			p.id,
			COALESCE(p.customer_id,0),
			p.name,
			COALESCE(p.roast_level, ''),
			COALESCE(NULLIF(p.product_kind,''),'roasted_bean'),
			COALESCE(NULLIF(b.yield_rate,0), 0),
			COALESCE(NULLIF(b.status,''), CASE WHEN b.product_id IS NULL THEN 'missing' ELSE 'active' END),
			COALESCE((SELECT COUNT(*) FROM %[1]s.product_bom_items bi WHERE bi.product_id = p.id), 0),
			COALESCE((
				SELECT COUNT(*)
				FROM %[1]s.order_items oi
				JOIN %[1]s.orders o ON o.id=oi.order_id
				WHERE oi.product_id=p.id AND COALESCE(o.is_void,false)=false
			),0) AS order_usage_count,
			COALESCE(to_char(b.updated_at,'YYYY-MM-DD HH24:MI'), '-')
		FROM %[1]s.products p
		LEFT JOIN %[1]s.product_bom b ON b.product_id = p.id
		WHERE p.active = true
		ORDER BY p.name
	`, r.schema)
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]bomapp.ListItem, 0)
	for rows.Next() {
		var item bomapp.ListItem
		var fallback float64
		if err := rows.Scan(&item.ProductID, &item.CustomerID, &item.Product, &item.RoastLevel, &item.ProductKind, &fallback, &item.Status, &item.ItemCount, &item.OrderUsageCount, &item.UpdatedAt); err != nil {
			return nil, err
		}
		source, err := resolveEffectiveBomSource(ctx, r.pool, r.schema, item.ProductID)
		if err != nil {
			return nil, err
		}
		summary, err := loadBomSummaryForSource(ctx, r.pool, r.schema, source)
		if err != nil {
			return nil, err
		}
		if summary.YieldRate > 0 {
			fallback = summary.YieldRate
		}
		item.YieldRate = resolveBomYieldRate(item.RoastLevel, fallback)
		item.Status = summary.Status
		item.ItemCount = summary.ItemCount
		item.UpdatedAt = summary.UpdatedAt
		applyBomSourceToListItem(&item, source)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r Repository) Detail(ctx context.Context, productID int64) (bomapp.Detail, error) {
	source, err := resolveEffectiveBomSource(ctx, r.pool, r.schema, productID)
	if err != nil {
		return bomapp.Detail{}, err
	}

	summary, err := loadBomSummaryForSource(ctx, r.pool, r.schema, source)
	if err != nil {
		return bomapp.Detail{}, err
	}
	items, total, err := listBomItemsForSource(ctx, r.pool, r.schema, source)
	if err != nil {
		return bomapp.Detail{}, err
	}
	detail := bomapp.Detail{
		ProductID:   productID,
		ProductName: source.ProductName,
		RoastLevel:  source.RoastLevel,
		YieldRate:   resolveBomYieldRate(source.RoastLevel, summary.YieldRate),
		Status:      summary.Status,
		Items:       bomItemsToApp(items),
		TotalRatio:  total,
		UpdatedAt:   summary.UpdatedAt,
	}
	applyBomSourceToDetail(&detail, source)
	return detail, nil
}

func (r Repository) Products(ctx context.Context) ([]bomapp.Option, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT p.id, p.name, COALESCE(p.customer_id,0), COALESCE(p.roast_level,''), COALESCE(NULLIF(p.product_kind,''),'roasted_bean'), COALESCE(p.drip_bag_grams,10)::float8, COALESCE(p.drip_box_bag_count,10),
		COALESCE((
			SELECT COUNT(*)
			FROM %[1]s.order_items oi
			JOIN %[1]s.orders o ON o.id=oi.order_id
			WHERE oi.product_id=p.id AND COALESCE(o.is_void,false)=false
		),0) AS order_usage_count
		FROM %[1]s.products p WHERE p.active=true ORDER BY p.name`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]bomapp.Option, 0)
	for rows.Next() {
		var opt bomapp.Option
		if err := rows.Scan(&opt.ID, &opt.Name, &opt.CustomerID, &opt.RoastLevel, &opt.ProductKind, &opt.DripBagGrams, &opt.DripBoxBagCount, &opt.OrderUsageCount); err != nil {
			return nil, err
		}
		out = append(out, opt)
	}
	return out, rows.Err()
}

func (r Repository) Materials(ctx context.Context) ([]bomapp.Option, error) {
	opts, err := postgresinfra.FetchOptions(ctx, r.pool, "SELECT id, name FROM "+r.schema+".materials ORDER BY name")
	if err != nil {
		return nil, err
	}
	return bomOptionsToApp(opts), nil
}

func (r Repository) BagSpecMappings(ctx context.Context) ([]bomapp.BagSpecMapping, error) {
	rows, err := postgresinfra.ListBagSpecMappings(ctx, r.pool, r.schema)
	if err != nil {
		return nil, err
	}
	return bagSpecMappingsToApp(rows), nil
}

func (r Repository) SyncProductYield(ctx context.Context, cmd bomapp.SyncProductYieldCommand) error {
	if err := ensureBomEditable(ctx, r.pool, r.schema, cmd.ProductID); err != nil {
		return err
	}
	var roastLevel string
	if err := r.pool.QueryRow(ctx, "SELECT COALESCE(roast_level,'') FROM "+r.schema+".products WHERE id=$1", cmd.ProductID).Scan(&roastLevel); err != nil {
		return fmt.Errorf("product not found")
	}
	yieldRate := cmd.ExpectedYieldRate
	if yieldRate <= 0 {
		yieldRate = catalogdomain.ResolveYieldRate(roastLevel, 0.8)
	}
	yieldRate = productiondomain.NormalizeYieldRate(yieldRate)
	q := "INSERT INTO " + r.schema + ".product_bom(product_id,yield_rate,status,updated_at) VALUES($1,$2,'active',now()) ON CONFLICT (product_id) DO UPDATE SET yield_rate=excluded.yield_rate, status='active', updated_at=now()"
	_, err := r.pool.Exec(ctx, q, cmd.ProductID, yieldRate)
	if err == nil {
		action := "sync_yield"
		if cmd.ExpectedLossRate != nil {
			action = "save_expected_loss_rate"
		}
		postgresinfra.AuditInsert(ctx, r.pool, r.schema, cmd.Actor, "product_bom", &cmd.ProductID, action, postgresinfra.StrPtr("expected_loss_rate"), nil, postgresinfra.StrPtr(fmt.Sprintf("%.4f", productiondomain.ExpectedLossRate(yieldRate))), postgresinfra.AuditMeta{"product_id": cmd.ProductID, "status": "active", "yield_rate": yieldRate, "expected_loss_rate": productiondomain.ExpectedLossRate(yieldRate)})
	}
	return err
}

func (r Repository) DeactivateBom(ctx context.Context, cmd bomapp.DeactivateBomCommand) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := ensureBomEditable(ctx, tx, r.schema, cmd.ProductID); err != nil {
		return err
	}
	var roastLevel string
	if err := tx.QueryRow(ctx, "SELECT COALESCE(roast_level,'') FROM "+r.schema+".products WHERE id=$1", cmd.ProductID).Scan(&roastLevel); err != nil {
		return fmt.Errorf("product not found")
	}
	yieldRate := catalogdomain.ResolveYieldRate(roastLevel, 0.8)
	if _, err := tx.Exec(ctx, "INSERT INTO "+r.schema+".product_bom(product_id,yield_rate,status,updated_at) VALUES($1,$2,'inactive',now()) ON CONFLICT (product_id) DO UPDATE SET status='inactive', updated_at=now()", cmd.ProductID, yieldRate); err != nil {
		return err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product_bom", &cmd.ProductID, "deactivate", postgresinfra.StrPtr("status"), nil, postgresinfra.StrPtr("inactive"), postgresinfra.AuditMeta{"product_id": cmd.ProductID}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) SaveItem(ctx context.Context, cmd bomapp.SaveItemCommand) error {
	if err := r.SyncProductYield(ctx, bomapp.SyncProductYieldCommand{ProductID: cmd.ProductID, Actor: cmd.Actor}); err != nil {
		return err
	}
	_, total, err := listBomItems(ctx, r.pool, r.schema, cmd.ProductID)
	if err != nil {
		return err
	}

	if cmd.ComponentType == "material" && cmd.ConsumeUnit == "ratio_pct" {
		var oldRatio float64
		_ = r.pool.QueryRow(ctx,
			"SELECT COALESCE(ratio_pct,0) FROM "+r.schema+".product_bom_items WHERE product_id=$1 AND component_type='material' AND material_id=$2",
			cmd.ProductID, cmd.MaterialID).Scan(&oldRatio)
		if total-oldRatio+cmd.RatioPct > 100.0001 {
			return fmt.Errorf("ratio sum exceed 100%%")
		}
	}

	if cmd.ComponentType == "finished_product" {
		q := "INSERT INTO " + r.schema + ".product_bom_items(product_id,material_id,component_type,component_product_id,component_spec_g,consume_unit,qty_per_unit,ratio_pct,updated_at) VALUES($1,0,$2,$3,$4,$5,$6,0,now()) ON CONFLICT (product_id,component_product_id,component_spec_g,consume_unit) WHERE component_type='finished_product' DO UPDATE SET component_product_id=excluded.component_product_id, component_spec_g=excluded.component_spec_g, consume_unit=excluded.consume_unit, qty_per_unit=excluded.qty_per_unit, ratio_pct=excluded.ratio_pct, updated_at=now()"
		_, err = r.pool.Exec(ctx, q, cmd.ProductID, cmd.ComponentType, cmd.ComponentProductID, cmd.ComponentSpecG, cmd.ConsumeUnit, cmd.QtyPerUnit)
		if err == nil {
			auditBomItemSave(ctx, r.pool, r.schema, cmd)
		}
		return err
	}

	q := "INSERT INTO " + r.schema + ".product_bom_items(product_id,material_id,component_type,component_product_id,component_spec_g,consume_unit,qty_per_unit,ratio_pct,unit_cost_snapshot,updated_at) VALUES($1,$2,'material',0,$3,$4,$5,$6,COALESCE((SELECT purchase_price FROM " + r.schema + ".materials WHERE id=$2),0),now()) ON CONFLICT (product_id,material_id) WHERE component_type='material' DO UPDATE SET component_spec_g=excluded.component_spec_g, consume_unit=excluded.consume_unit, qty_per_unit=excluded.qty_per_unit, ratio_pct=excluded.ratio_pct, unit_cost_snapshot=excluded.unit_cost_snapshot, updated_at=now()"
	_, err = r.pool.Exec(ctx, q, cmd.ProductID, cmd.MaterialID, cmd.ComponentSpecG, cmd.ConsumeUnit, cmd.QtyPerUnit, cmd.RatioPct)
	if err == nil {
		auditBomItemSave(ctx, r.pool, r.schema, cmd)
	}
	return err
}

func (r Repository) DeleteItem(ctx context.Context, cmd bomapp.DeleteItemCommand) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := ensureBomEditable(ctx, tx, r.schema, cmd.ProductID); err != nil {
		return err
	}
	var row struct {
		ID                 int64
		ProductID          int64
		MaterialID         int64
		ComponentType      string
		ComponentProductID int64
		ComponentSpecG     int64
		ConsumeUnit        string
		QtyPerUnit         float64
		RatioPct           float64
	}
	err = tx.QueryRow(ctx, "SELECT id, product_id, material_id, COALESCE(NULLIF(component_type,''),'material'), COALESCE(component_product_id,0), COALESCE(component_spec_g,0), COALESCE(NULLIF(consume_unit,''),'ratio_pct'), COALESCE(qty_per_unit,0)::float8, COALESCE(ratio_pct,0)::float8 FROM "+r.schema+".product_bom_items WHERE id=$1 AND product_id=$2 FOR UPDATE", cmd.ID, cmd.ProductID).
		Scan(&row.ID, &row.ProductID, &row.MaterialID, &row.ComponentType, &row.ComponentProductID, &row.ComponentSpecG, &row.ConsumeUnit, &row.QtyPerUnit, &row.RatioPct)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("bom item not found")
		}
		return err
	}
	if _, err := tx.Exec(ctx, "DELETE FROM "+r.schema+".product_bom_items WHERE id=$1 AND product_id=$2", cmd.ID, cmd.ProductID); err != nil {
		return err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product_bom_item", &cmd.ProductID, "delete", postgresinfra.StrPtr("item_id"), postgresinfra.StrPtr(fmt.Sprintf("%d", row.ID)), nil, postgresinfra.AuditMeta{"product_id": row.ProductID, "material_id": row.MaterialID, "component_type": row.ComponentType, "component_product_id": row.ComponentProductID, "component_spec_g": row.ComponentSpecG, "consume_unit": row.ConsumeUnit, "qty_per_unit": row.QtyPerUnit, "ratio_pct": row.RatioPct}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) SaveBagSpecMapping(ctx context.Context, cmd bomapp.SaveBagSpecMappingCommand) error {
	return saveBagSpecMapping(ctx, r.pool, r.schema, cmd)
}

func (r Repository) DeleteBagSpecMapping(ctx context.Context, cmd bomapp.DeleteBagSpecMappingCommand) error {
	return deleteBagSpecMapping(ctx, r.pool, r.schema, cmd)
}

func (r Repository) ListVersions(ctx context.Context, productID int64) ([]bomapp.Version, error) {
	source, err := resolveEffectiveBomSource(ctx, r.pool, r.schema, productID)
	if err != nil {
		return nil, err
	}
	versionProductID := source.EffectiveProductID
	if versionProductID <= 0 {
		versionProductID = productID
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT v.id,v.product_id,v.version_no,v.status,COALESCE(v.yield_rate,0.8),
		       COALESCE((SELECT COUNT(*) FROM %s.bom_version_items i WHERE i.version_id=v.id),0),
		       COALESCE(v.note,''),to_char(v.created_at,'YYYY-MM-DD HH24:MI')
		FROM %s.bom_versions v
		WHERE v.product_id=$1
		ORDER BY v.created_at DESC, v.id DESC
	`, r.schema, r.schema), versionProductID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]bomapp.Version, 0)
	for rows.Next() {
		var v bomapp.Version
		if err := rows.Scan(&v.ID, &v.ProductID, &v.VersionNo, &v.Status, &v.YieldRate, &v.ItemCount, &v.Note, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r Repository) CreateVersion(ctx context.Context, cmd bomapp.CreateVersionCommand) (bomapp.Version, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return bomapp.Version{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := ensureBomEditable(ctx, tx, r.schema, cmd.ProductID); err != nil {
		return bomapp.Version{}, err
	}
	var yieldRate float64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(yield_rate,0.8) FROM %s.product_bom WHERE product_id=$1`, r.schema), cmd.ProductID).Scan(&yieldRate); err != nil {
		yieldRate = 0.8
	}
	var next int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(COUNT(*),0)+1 FROM %s.bom_versions WHERE product_id=$1`, r.schema), cmd.ProductID).Scan(&next); err != nil {
		return bomapp.Version{}, err
	}
	versionNo := fmt.Sprintf("V%03d", next)
	var versionID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.bom_versions(product_id,version_no,status,yield_rate,note,created_at)
		VALUES($1,$2,'draft',$3,$4,now())
		RETURNING id
	`, r.schema), cmd.ProductID, versionNo, yieldRate, strings.TrimSpace(cmd.Note)).Scan(&versionID); err != nil {
		return bomapp.Version{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.bom_version_items(version_id,material_id,component_type,component_product_id,component_spec_g,consume_unit,qty_per_unit,ratio_pct,unit_cost_snapshot)
		SELECT $1,material_id,component_type,component_product_id,component_spec_g,consume_unit,qty_per_unit,ratio_pct,unit_cost_snapshot
		FROM %s.product_bom_items
		WHERE product_id=$2
		ORDER BY id
	`, r.schema, r.schema), versionID, cmd.ProductID); err != nil {
		return bomapp.Version{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "bom_version", &versionID, "create", postgresinfra.StrPtr("version_no"), nil, postgresinfra.StrPtr(versionNo), postgresinfra.AuditMeta{"product_id": cmd.ProductID, "note": strings.TrimSpace(cmd.Note)}); err != nil {
		return bomapp.Version{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return bomapp.Version{}, err
	}
	versions, err := r.ListVersions(ctx, cmd.ProductID)
	if err != nil {
		return bomapp.Version{}, err
	}
	for _, v := range versions {
		if v.ID == versionID {
			return v, nil
		}
	}
	return bomapp.Version{ID: versionID, ProductID: cmd.ProductID, VersionNo: versionNo, Status: "draft", YieldRate: yieldRate}, nil
}

func (r Repository) ActivateVersion(ctx context.Context, cmd bomapp.ActivateVersionCommand) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var productID int64
	var yieldRate float64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT product_id,COALESCE(yield_rate,0.8) FROM %s.bom_versions WHERE id=$1 FOR UPDATE`, r.schema), cmd.VersionID).Scan(&productID, &yieldRate); err != nil {
		return fmt.Errorf("bom version not found")
	}
	if err := ensureBomEditable(ctx, tx, r.schema, productID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.bom_versions SET status='disabled' WHERE product_id=$1 AND id<>$2`, r.schema), productID, cmd.VersionID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.bom_versions SET status='active',activated_at=now() WHERE id=$1`, r.schema), cmd.VersionID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_bom(product_id,yield_rate,status,updated_at)
		VALUES($1,$2,'active',now())
		ON CONFLICT (product_id) DO UPDATE SET yield_rate=excluded.yield_rate, status='active', updated_at=now()
	`, r.schema), productID, yieldRate); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.product_bom_items WHERE product_id=$1`, r.schema), productID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_bom_items(product_id,material_id,component_type,component_product_id,component_spec_g,consume_unit,qty_per_unit,ratio_pct,unit_cost_snapshot,updated_at)
		SELECT $1,material_id,component_type,component_product_id,component_spec_g,consume_unit,qty_per_unit,ratio_pct,unit_cost_snapshot,now()
		FROM %s.bom_version_items
		WHERE version_id=$2
		ORDER BY id
	`, r.schema, r.schema), productID, cmd.VersionID); err != nil {
		return err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product_bom", &productID, "activate_version", postgresinfra.StrPtr("version_id"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", cmd.VersionID)), postgresinfra.AuditMeta{"product_id": productID, "version_id": cmd.VersionID}); err != nil {
		return err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "bom_version", &cmd.VersionID, "activate", postgresinfra.StrPtr("status"), nil, postgresinfra.StrPtr("active"), postgresinfra.AuditMeta{"product_id": productID, "version_id": cmd.VersionID}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) DeriveOwned(ctx context.Context, cmd bomapp.DeriveOwnedCommand) (bomapp.Detail, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return bomapp.Detail{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := deriveOwnedBomTx(ctx, tx, r.schema, cmd); err != nil {
		return bomapp.Detail{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return bomapp.Detail{}, err
	}
	return r.Detail(ctx, cmd.ProductID)
}

type bomItemRow struct {
	ID                   int64
	MaterialID           int64
	MaterialName         string
	ComponentType        string
	ComponentProductID   int64
	ComponentProductName string
	ComponentSpecG       int64
	ConsumeUnit          string
	QtyPerUnit           float64
	RatioPct             float64
	UnitCostSnapshot     float64
}

type bomQueryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type bomSourceInfo struct {
	ProductID             int64
	ProductName           string
	RoastLevel            string
	BaseProductID         int64
	BomSourceType         string
	EffectiveProductID    int64
	EffectiveBomVersionID int64
	SourceProductID       int64
	SourceProductCode     string
	SourceProductName     string
	SourceBomProductID    int64
	SourceBomVersionID    int64
	SourceBomVersionNo    string
	DerivedFromLabel      string
	CanEditBOM            bool
}

type bomSourceRow struct {
	SourceType                 string
	SourceProductID            int64
	SourceProductCodeSnapshot  string
	SourceProductNameSnapshot  string
	SourceBomProductID         int64
	SourceBomVersionID         int64
	SourceBomVersionNoSnapshot string
	DerivedFromProductID       int64
	DerivedFromBomVersionID    int64
}

type bomSummary struct {
	YieldRate float64
	Status    string
	ItemCount int
	UpdatedAt string
}

func resolveEffectiveBomSource(ctx context.Context, q bomQueryer, schema string, productID int64) (bomSourceInfo, error) {
	var info bomSourceInfo
	info.ProductID = productID
	info.EffectiveProductID = productID
	info.CanEditBOM = true
	if err := q.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(name,''), COALESCE(roast_level,''), COALESCE(base_product_id,0)
		FROM %s.products
		WHERE id=$1 AND active=true
	`, schema), productID).Scan(&info.ProductName, &info.RoastLevel, &info.BaseProductID); err != nil {
		return bomSourceInfo{}, err
	}

	var source bomSourceRow
	err := q.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(NULLIF(source_type,''),'owned'),
		       COALESCE(source_product_id,0),
		       COALESCE(source_product_code_snapshot,''),
		       COALESCE(source_product_name_snapshot,''),
		       COALESCE(source_bom_product_id,0),
		       COALESCE(source_bom_version_id,0),
		       COALESCE(source_bom_version_no_snapshot,''),
		       COALESCE(derived_from_product_id,0),
		       COALESCE(derived_from_bom_version_id,0)
		FROM %s.product_bom_sources
		WHERE product_id=$1
	`, schema), productID).Scan(&source.SourceType, &source.SourceProductID, &source.SourceProductCodeSnapshot, &source.SourceProductNameSnapshot, &source.SourceBomProductID, &source.SourceBomVersionID, &source.SourceBomVersionNoSnapshot, &source.DerivedFromProductID, &source.DerivedFromBomVersionID)
	if err != nil && err != pgx.ErrNoRows {
		return bomSourceInfo{}, err
	}
	hasExplicitSource := err != pgx.ErrNoRows
	hasOwn, err := hasOwnBomDefinition(ctx, q, schema, productID)
	if err != nil {
		return bomSourceInfo{}, err
	}

	sourceType := strings.TrimSpace(source.SourceType)
	if sourceType == "" {
		if info.BaseProductID > 0 && !hasOwn {
			sourceType = "inherit_current"
			source.SourceProductID = info.BaseProductID
		} else if hasOwn {
			sourceType = "owned"
		} else {
			sourceType = "missing"
		}
	} else if !hasExplicitSource && info.BaseProductID > 0 && !hasOwn {
		sourceType = "inherit_current"
		source.SourceProductID = info.BaseProductID
	}
	info.BomSourceType = sourceType

	switch sourceType {
	case "inherit_current", "inherit_version":
		info.CanEditBOM = false
		if source.SourceProductID <= 0 {
			source.SourceProductID = source.SourceBomProductID
		}
		if source.SourceProductID <= 0 {
			source.SourceProductID = info.BaseProductID
		}
		info.SourceProductID = source.SourceProductID
		info.SourceBomProductID = source.SourceProductID
		info.EffectiveProductID = source.SourceProductID
		info.SourceBomVersionID = source.SourceBomVersionID
		info.SourceBomVersionNo = source.SourceBomVersionNoSnapshot
		if sourceType == "inherit_current" || info.SourceBomVersionNo == "" {
			activeID, activeNo, err := activeBomVersionSnapshot(ctx, q, schema, info.EffectiveProductID)
			if err != nil {
				return bomSourceInfo{}, err
			}
			if sourceType == "inherit_current" {
				info.SourceBomVersionID = activeID
				info.SourceBomVersionNo = activeNo
			} else if info.SourceBomVersionNo == "" {
				info.SourceBomVersionNo = activeNo
			}
		}
		info.EffectiveBomVersionID = info.SourceBomVersionID
	case "derived_owned":
		info.CanEditBOM = true
		info.EffectiveProductID = productID
		info.SourceProductID = source.DerivedFromProductID
		if info.SourceProductID <= 0 {
			info.SourceProductID = source.SourceProductID
		}
		info.SourceBomProductID = source.SourceBomProductID
		if info.SourceBomProductID <= 0 {
			info.SourceBomProductID = info.SourceProductID
		}
		info.SourceBomVersionID = source.DerivedFromBomVersionID
		if info.SourceBomVersionID <= 0 {
			info.SourceBomVersionID = source.SourceBomVersionID
		}
		info.SourceBomVersionNo = source.SourceBomVersionNoSnapshot
		activeID, _, err := activeBomVersionSnapshot(ctx, q, schema, productID)
		if err != nil {
			return bomSourceInfo{}, err
		}
		info.EffectiveBomVersionID = activeID
	case "owned":
		info.CanEditBOM = true
		info.EffectiveProductID = productID
		activeID, _, err := activeBomVersionSnapshot(ctx, q, schema, productID)
		if err != nil {
			return bomSourceInfo{}, err
		}
		info.EffectiveBomVersionID = activeID
	default:
		info.BomSourceType = "missing"
		info.CanEditBOM = true
		info.EffectiveProductID = productID
	}

	if info.SourceProductID > 0 {
		info.SourceProductCode = strings.TrimSpace(source.SourceProductCodeSnapshot)
		if info.SourceProductCode == "" {
			info.SourceProductCode = skuCodeSnapshot(info.SourceProductID)
		}
		info.SourceProductName = strings.TrimSpace(source.SourceProductNameSnapshot)
		if info.SourceProductName == "" {
			_ = q.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(name,'') FROM %s.products WHERE id=$1`, schema), info.SourceProductID).Scan(&info.SourceProductName)
		}
		if info.SourceBomVersionNo == "" {
			_, versionNo, err := activeBomVersionSnapshot(ctx, q, schema, info.SourceProductID)
			if err != nil {
				return bomSourceInfo{}, err
			}
			info.SourceBomVersionNo = versionNo
		}
	}
	if info.SourceBomVersionNo == "" {
		info.SourceBomVersionNo = "当前BOM"
	}
	info.DerivedFromLabel = buildBomSourceLabel(info)
	return info, nil
}

func hasOwnBomDefinition(ctx context.Context, q bomQueryer, schema string, productID int64) (bool, error) {
	var has bool
	err := q.QueryRow(ctx, fmt.Sprintf(`
		SELECT EXISTS(SELECT 1 FROM %s.product_bom WHERE product_id=$1)
		    OR EXISTS(SELECT 1 FROM %s.product_bom_items WHERE product_id=$1)
	`, schema, schema), productID).Scan(&has)
	return has, err
}

func activeBomVersionSnapshot(ctx context.Context, q bomQueryer, schema string, productID int64) (int64, string, error) {
	var id int64
	var versionNo string
	err := q.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, COALESCE(version_no,'')
		FROM %s.bom_versions
		WHERE product_id=$1 AND status='active'
		ORDER BY activated_at DESC NULLS LAST, id DESC
		LIMIT 1
	`, schema), productID).Scan(&id, &versionNo)
	if err == pgx.ErrNoRows {
		return 0, "当前BOM", nil
	}
	if err != nil {
		return 0, "", err
	}
	if versionNo == "" {
		versionNo = "当前BOM"
	}
	return id, versionNo, nil
}

func loadBomSummaryForSource(ctx context.Context, q bomQueryer, schema string, source bomSourceInfo) (bomSummary, error) {
	if source.BomSourceType == "inherit_version" && source.EffectiveBomVersionID > 0 {
		var summary bomSummary
		err := q.QueryRow(ctx, fmt.Sprintf(`
			SELECT COALESCE(v.yield_rate,0.8)::float8,
			       COALESCE(NULLIF(v.status,''),'active'),
			       COALESCE((SELECT COUNT(*) FROM %s.bom_version_items i WHERE i.version_id=v.id),0),
			       COALESCE(to_char(v.created_at,'YYYY-MM-DD HH24:MI'),'-')
			FROM %s.bom_versions v
			WHERE v.id=$1
		`, schema, schema), source.EffectiveBomVersionID).Scan(&summary.YieldRate, &summary.Status, &summary.ItemCount, &summary.UpdatedAt)
		if err == nil {
			return summary, nil
		}
		if err != pgx.ErrNoRows {
			return bomSummary{}, err
		}
	}
	productID := source.EffectiveProductID
	if productID <= 0 {
		productID = source.ProductID
	}
	var summary bomSummary
	err := q.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(NULLIF(b.yield_rate,0),0)::float8,
		       COALESCE(NULLIF(b.status,''), CASE WHEN b.product_id IS NULL THEN 'missing' ELSE 'active' END),
		       COALESCE((SELECT COUNT(*) FROM %[1]s.product_bom_items bi WHERE bi.product_id=$1),0),
		       COALESCE(to_char(b.updated_at,'YYYY-MM-DD HH24:MI'),'-')
		FROM (SELECT $1::bigint AS product_id) p
		LEFT JOIN %[1]s.product_bom b ON b.product_id=p.product_id
	`, schema), productID).Scan(&summary.YieldRate, &summary.Status, &summary.ItemCount, &summary.UpdatedAt)
	return summary, err
}

func applyBomSourceToListItem(item *bomapp.ListItem, source bomSourceInfo) {
	item.BomSourceType = source.BomSourceType
	item.EffectiveProductID = source.EffectiveProductID
	item.EffectiveBomVersionID = source.EffectiveBomVersionID
	item.SourceProductID = source.SourceProductID
	item.SourceProductCode = source.SourceProductCode
	item.SourceProductName = source.SourceProductName
	item.SourceBomVersionID = source.SourceBomVersionID
	item.SourceBomVersionNo = source.SourceBomVersionNo
	item.DerivedFromLabel = source.DerivedFromLabel
	item.CanEditBOM = source.CanEditBOM
}

func applyBomSourceToDetail(detail *bomapp.Detail, source bomSourceInfo) {
	detail.BomSourceType = source.BomSourceType
	detail.EffectiveProductID = source.EffectiveProductID
	detail.EffectiveBomVersionID = source.EffectiveBomVersionID
	detail.SourceProductID = source.SourceProductID
	detail.SourceProductCode = source.SourceProductCode
	detail.SourceProductName = source.SourceProductName
	detail.SourceBomVersionID = source.SourceBomVersionID
	detail.SourceBomVersionNo = source.SourceBomVersionNo
	detail.DerivedFromLabel = source.DerivedFromLabel
	detail.CanEditBOM = source.CanEditBOM
}

func buildBomSourceLabel(source bomSourceInfo) string {
	target := strings.TrimSpace(strings.TrimSpace(source.SourceProductCode) + " " + strings.TrimSpace(source.SourceProductName))
	version := strings.TrimSpace(source.SourceBomVersionNo)
	if version == "" {
		version = "当前BOM"
	}
	switch source.BomSourceType {
	case "inherit_current":
		return fmt.Sprintf("继承：%s / BOM %s", target, version)
	case "inherit_version":
		return fmt.Sprintf("锁定：%s / BOM %s", target, version)
	case "derived_owned":
		return fmt.Sprintf("自有 BOM，派生自：%s / BOM %s", target, version)
	case "owned":
		return "自有 BOM"
	default:
		return "缺 BOM"
	}
}

func skuCodeSnapshot(productID int64) string {
	if productID <= 0 {
		return ""
	}
	return fmt.Sprintf("SKU-%d", productID)
}

func ensureBomEditable(ctx context.Context, q bomQueryer, schema string, productID int64) error {
	source, err := resolveEffectiveBomSource(ctx, q, schema, productID)
	if err != nil {
		return err
	}
	if !source.CanEditBOM {
		return fmt.Errorf("inherited BOM is read-only; derive owned BOM first")
	}
	return nil
}

func listBomItems(ctx context.Context, db bomQueryer, schema string, productID int64) ([]bomItemRow, float64, error) {
	query := fmt.Sprintf(`
		SELECT bi.id,
		       bi.material_id,
		       COALESCE(m.name,''),
		       COALESCE(NULLIF(bi.component_type,''),'material'),
		       COALESCE(bi.component_product_id,0),
		       COALESCE(cp.name,''),
		       COALESCE(bi.component_spec_g,0),
		       COALESCE(NULLIF(bi.consume_unit,''),'ratio_pct'),
		       COALESCE(bi.qty_per_unit,0)::float8,
		       bi.ratio_pct,
		       COALESCE(bi.unit_cost_snapshot,0)::float8
		FROM %s.product_bom_items bi
		LEFT JOIN %s.materials m ON m.id=bi.material_id
		LEFT JOIN %s.products cp ON cp.id=bi.component_product_id
		WHERE bi.product_id=$1
		ORDER BY COALESCE(m.name, cp.name, ''), bi.id
	`, schema, schema, schema)
	rows, err := db.Query(ctx, query, productID)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]bomItemRow, 0)
	total := 0.0
	for rows.Next() {
		var row bomItemRow
		if err := rows.Scan(&row.ID, &row.MaterialID, &row.MaterialName, &row.ComponentType, &row.ComponentProductID, &row.ComponentProductName, &row.ComponentSpecG, &row.ConsumeUnit, &row.QtyPerUnit, &row.RatioPct, &row.UnitCostSnapshot); err != nil {
			return nil, 0, err
		}
		if row.ComponentType == "material" && row.ConsumeUnit == "ratio_pct" {
			total += row.RatioPct
		}
		out = append(out, row)
	}
	return out, total, rows.Err()
}

func listBomVersionItems(ctx context.Context, q bomQueryer, schema string, versionID int64) ([]bomItemRow, float64, error) {
	rows, err := q.Query(ctx, fmt.Sprintf(`
		SELECT bi.id,
		       bi.material_id,
		       COALESCE(m.name,''),
		       COALESCE(NULLIF(bi.component_type,''),'material'),
		       COALESCE(bi.component_product_id,0),
		       COALESCE(cp.name,''),
		       COALESCE(bi.component_spec_g,0),
		       COALESCE(NULLIF(bi.consume_unit,''),'ratio_pct'),
		       COALESCE(bi.qty_per_unit,0)::float8,
		       bi.ratio_pct,
		       COALESCE(bi.unit_cost_snapshot,0)::float8
		FROM %s.bom_version_items bi
		LEFT JOIN %s.materials m ON m.id=bi.material_id
		LEFT JOIN %s.products cp ON cp.id=bi.component_product_id
		WHERE bi.version_id=$1
		ORDER BY COALESCE(m.name, cp.name, ''), bi.id
	`, schema, schema, schema), versionID)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]bomItemRow, 0)
	total := 0.0
	for rows.Next() {
		var row bomItemRow
		if err := rows.Scan(&row.ID, &row.MaterialID, &row.MaterialName, &row.ComponentType, &row.ComponentProductID, &row.ComponentProductName, &row.ComponentSpecG, &row.ConsumeUnit, &row.QtyPerUnit, &row.RatioPct, &row.UnitCostSnapshot); err != nil {
			return nil, 0, err
		}
		if row.ComponentType == "material" && row.ConsumeUnit == "ratio_pct" {
			total += row.RatioPct
		}
		out = append(out, row)
	}
	return out, total, rows.Err()
}

func listBomItemsForSource(ctx context.Context, q bomQueryer, schema string, source bomSourceInfo) ([]bomItemRow, float64, error) {
	if source.BomSourceType == "inherit_version" && source.EffectiveBomVersionID > 0 {
		return listBomVersionItems(ctx, q, schema, source.EffectiveBomVersionID)
	}
	productID := source.EffectiveProductID
	if productID <= 0 {
		productID = source.ProductID
	}
	return listBomItems(ctx, q, schema, productID)
}

func deriveOwnedBomTx(ctx context.Context, tx pgx.Tx, schema string, cmd bomapp.DeriveOwnedCommand) error {
	source, err := resolveEffectiveBomSource(ctx, tx, schema, cmd.ProductID)
	if err != nil {
		return err
	}
	if source.CanEditBOM && source.BomSourceType != "missing" {
		return nil
	}
	summary, err := loadBomSummaryForSource(ctx, tx, schema, source)
	if err != nil {
		return err
	}
	if summary.Status == "missing" && summary.ItemCount == 0 {
		return fmt.Errorf("source BOM not configured")
	}
	items, _, err := listBomItemsForSource(ctx, tx, schema, source)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.product_bom_items WHERE product_id=$1`, schema), cmd.ProductID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.product_bom WHERE product_id=$1`, schema), cmd.ProductID); err != nil {
		return err
	}
	status := strings.TrimSpace(summary.Status)
	if status == "" || status == "missing" {
		status = "active"
	}
	yieldRate := summary.YieldRate
	if yieldRate <= 0 {
		yieldRate = resolveBomYieldRate(source.RoastLevel, 0)
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_bom(product_id,yield_rate,status,updated_at)
		VALUES($1,$2,$3,now())
		ON CONFLICT (product_id) DO UPDATE SET yield_rate=excluded.yield_rate, status=excluded.status, updated_at=now()
	`, schema), cmd.ProductID, yieldRate, status); err != nil {
		return err
	}
	for _, item := range items {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.product_bom_items(product_id,material_id,component_type,component_product_id,component_spec_g,consume_unit,qty_per_unit,ratio_pct,unit_cost_snapshot,updated_at)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,now())
		`, schema), cmd.ProductID, item.MaterialID, item.ComponentType, item.ComponentProductID, item.ComponentSpecG, item.ConsumeUnit, item.QtyPerUnit, item.RatioPct, item.UnitCostSnapshot); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_bom_sources(
			product_id, source_type, source_product_id, source_product_code_snapshot, source_product_name_snapshot,
			source_bom_product_id, source_bom_version_id, source_bom_version_no_snapshot,
			derived_from_product_id, derived_from_bom_version_id, derived_at, derived_by, updated_at
		)
		VALUES($1,'derived_owned',$2,$3,$4,$5,$6,$7,$2,$6,now(),$8,now())
		ON CONFLICT (product_id) DO UPDATE SET
			source_type='derived_owned',
			source_product_id=excluded.source_product_id,
			source_product_code_snapshot=excluded.source_product_code_snapshot,
			source_product_name_snapshot=excluded.source_product_name_snapshot,
			source_bom_product_id=excluded.source_bom_product_id,
			source_bom_version_id=excluded.source_bom_version_id,
			source_bom_version_no_snapshot=excluded.source_bom_version_no_snapshot,
			derived_from_product_id=excluded.derived_from_product_id,
			derived_from_bom_version_id=excluded.derived_from_bom_version_id,
			derived_at=excluded.derived_at,
			derived_by=excluded.derived_by,
			updated_at=now()
	`, schema), cmd.ProductID, source.SourceProductID, source.SourceProductCode, source.SourceProductName, source.EffectiveProductID, source.SourceBomVersionID, source.SourceBomVersionNo, strings.TrimSpace(cmd.Actor)); err != nil {
		return err
	}
	return postgresinfra.AuditInsertTx(ctx, tx, schema, cmd.Actor, "product_bom", &cmd.ProductID, "derive_owned", postgresinfra.StrPtr("source_bom_version_id"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", source.SourceBomVersionID)), postgresinfra.AuditMeta{
		"source_product_id":     source.SourceProductID,
		"source_product_code":   source.SourceProductCode,
		"source_product_name":   source.SourceProductName,
		"source_bom_product_id": source.EffectiveProductID,
		"source_bom_version_id": source.SourceBomVersionID,
		"source_bom_version_no": source.SourceBomVersionNo,
		"target_product_id":     cmd.ProductID,
		"target_product_code":   skuCodeSnapshot(cmd.ProductID),
		"target_product_name":   source.ProductName,
		"copied_item_count":     len(items),
		"can_edit_bom":          true,
	})
}

func saveBagSpecMapping(ctx context.Context, pool *pgxpool.Pool, schema string, cmd bomapp.SaveBagSpecMappingCommand) error {
	if cmd.SpecG <= 0 {
		return fmt.Errorf("spec_g required")
	}
	if cmd.MaterialID <= 0 {
		return fmt.Errorf("material_id required")
	}

	q := fmt.Sprintf(`INSERT INTO %s.packaging_spec_material_map(spec_g, material_id, updated_at)
		VALUES($1,$2,now())
		ON CONFLICT (spec_g) DO UPDATE SET material_id=excluded.material_id, updated_at=now()`, schema)
	_, err := pool.Exec(ctx, q, cmd.SpecG, cmd.MaterialID)
	if err == nil {
		postgresinfra.AuditInsert(ctx, pool, schema, cmd.Actor, "packaging_spec_material_map", &cmd.SpecG, "save", postgresinfra.StrPtr("material_id"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", cmd.MaterialID)), postgresinfra.AuditMeta{"spec_g": cmd.SpecG, "material_id": cmd.MaterialID})
	}
	return err
}

func deleteBagSpecMapping(ctx context.Context, pool *pgxpool.Pool, schema string, cmd bomapp.DeleteBagSpecMappingCommand) error {
	if cmd.SpecG <= 0 {
		return fmt.Errorf("spec_g required")
	}
	_, err := pool.Exec(ctx, "DELETE FROM "+schema+".packaging_spec_material_map WHERE spec_g=$1", cmd.SpecG)
	if err == nil {
		postgresinfra.AuditInsert(ctx, pool, schema, cmd.Actor, "packaging_spec_material_map", &cmd.SpecG, "delete", postgresinfra.StrPtr("spec_g"), postgresinfra.StrPtr(fmt.Sprintf("%d", cmd.SpecG)), nil, postgresinfra.AuditMeta{"spec_g": cmd.SpecG})
	}
	return err
}

func auditBomItemSave(ctx context.Context, pool *pgxpool.Pool, schema string, cmd bomapp.SaveItemCommand) {
	componentID := cmd.MaterialID
	if cmd.ComponentType == "finished_product" {
		componentID = cmd.ComponentProductID
	}
	newValue := fmt.Sprintf("%s:%d:%s", cmd.ComponentType, componentID, cmd.ConsumeUnit)
	postgresinfra.AuditInsert(ctx, pool, schema, cmd.Actor, "product_bom_item", &cmd.ProductID, "save", postgresinfra.StrPtr("component"), nil, postgresinfra.StrPtr(newValue), postgresinfra.AuditMeta{
		"product_id":           cmd.ProductID,
		"material_id":          cmd.MaterialID,
		"component_type":       cmd.ComponentType,
		"component_product_id": cmd.ComponentProductID,
		"component_spec_g":     cmd.ComponentSpecG,
		"consume_unit":         cmd.ConsumeUnit,
		"qty_per_unit":         cmd.QtyPerUnit,
		"ratio_pct":            cmd.RatioPct,
	})
}

func bomItemsToApp(rows []bomItemRow) []bomapp.Item {
	out := make([]bomapp.Item, 0, len(rows))
	for _, row := range rows {
		out = append(out, bomapp.Item{
			ID:                   row.ID,
			MaterialID:           row.MaterialID,
			MaterialName:         row.MaterialName,
			ComponentType:        row.ComponentType,
			ComponentProductID:   row.ComponentProductID,
			ComponentProductName: row.ComponentProductName,
			ComponentSpecG:       row.ComponentSpecG,
			ConsumeUnit:          row.ConsumeUnit,
			QtyPerUnit:           row.QtyPerUnit,
			RatioPct:             row.RatioPct,
		})
	}
	return out
}

func bagSpecMappingsToApp(rows []bomdomain.BagSpecMapping) []bomapp.BagSpecMapping {
	out := make([]bomapp.BagSpecMapping, 0, len(rows))
	for _, row := range rows {
		out = append(out, bomapp.BagSpecMapping{SpecG: row.SpecG, MaterialID: row.MaterialID, MaterialName: row.MaterialName})
	}
	return out
}

func bomOptionsToApp(opts []postgresinfra.Option) []bomapp.Option {
	out := make([]bomapp.Option, 0, len(opts))
	for _, opt := range opts {
		out = append(out, bomapp.Option{ID: opt.ID, Name: opt.Name})
	}
	return out
}

func resolveBomYieldRate(roastLevel string, storedYieldRate float64) float64 {
	if storedYieldRate > 0 && storedYieldRate <= 1 {
		return productiondomain.NormalizeYieldRate(storedYieldRate)
	}
	return catalogdomain.ResolveYieldRate(roastLevel, 0.8)
}

func (r Repository) SetBomSource(ctx context.Context, cmd bomapp.SetBomSourceCommand) (bomapp.Detail, error) {
	var bomSourceProductID int64
	var sourceProductName string
	var sourceVersionID int64
	var sourceVersionNo string

	if cmd.SourceType == "inherit_version" {
		err := r.pool.QueryRow(ctx, fmt.Sprintf(`
			SELECT bv.product_id, COALESCE(p.name,''), bv.id, COALESCE(bv.version_no,'')
			FROM %s.bom_versions bv
			JOIN %s.products p ON p.id=bv.product_id
			WHERE bv.id=$1 AND bv.status='active'
		`, r.schema, r.schema), cmd.SourceBomVersionID).Scan(&bomSourceProductID, &sourceProductName, &sourceVersionID, &sourceVersionNo)
		if err != nil {
			return bomapp.Detail{}, fmt.Errorf("bom version not found: %w", err)
		}
	}

	_, err := r.pool.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.products
		SET bom_source_type=$2,
		    bom_source_bom_version_id=$3,
		    updated_at=now()
		WHERE id=$1
	`, r.schema), cmd.ProductID, cmd.SourceType, cmd.SourceBomVersionID)
	if err != nil {
		return bomapp.Detail{}, err
	}

	auditMeta := postgresinfra.AuditMeta{
		"bom_source_product_id":   bomSourceProductID,
		"bom_source_product_name": sourceProductName,
		"bom_source_version_id":   sourceVersionID,
		"bom_source_version_no":   sourceVersionNo,
	}
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, cmd.Actor, "product_bom", &cmd.ProductID,
		"set_bom_source", postgresinfra.StrPtr("bom_source_type"), nil,
		postgresinfra.StrPtr(cmd.SourceType), auditMeta)

	return r.Detail(ctx, cmd.ProductID)
}
