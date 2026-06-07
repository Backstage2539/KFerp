package bom

import (
	"context"
	"fmt"
	"strings"
	"time"

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

var ErrLegacyProductionBomGroupsReadonly = fmt.Errorf("production BOM groups are legacy readonly")

func NewRepository(pool *pgxpool.Pool, schema string) Repository {
	return Repository{pool: pool, schema: schema}
}

func (r Repository) List(ctx context.Context) ([]bomapp.ListItem, error) {
	if err := repairLegacyProductionBomBindings(ctx, r.pool, r.schema); err != nil {
		return nil, err
	}
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
	if err := repairLegacyProductionBomBindings(ctx, r.pool, r.schema); err != nil {
		return bomapp.Detail{}, err
	}
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
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT p.id, ('SKU-' || lpad(p.id::text,6,'0')), p.name, COALESCE(p.customer_id,0), COALESCE(p.roast_level,''), COALESCE(NULLIF(p.product_kind,''),'roasted_bean'), COALESCE(p.drip_bag_grams,10)::float8, COALESCE(p.drip_box_bag_count,10),
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
		if err := rows.Scan(&opt.ID, &opt.ProductCode, &opt.Name, &opt.CustomerID, &opt.RoastLevel, &opt.ProductKind, &opt.DripBagGrams, &opt.DripBoxBagCount, &opt.OrderUsageCount); err != nil {
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
	var active bool
	if err := tx.QueryRow(ctx, "SELECT COALESCE(roast_level,''), active FROM "+r.schema+".products WHERE id=$1", cmd.ProductID).Scan(&roastLevel, &active); err != nil {
		return fmt.Errorf("product not found")
	}
	if active {
		return fmt.Errorf("商品仍为启用状态，需先停用商品后再失效 BOM")
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
	ProductID              int64
	ProductName            string
	RoastLevel             string
	BaseProductID          int64
	BomSourceType          string
	EffectiveProductID     int64
	EffectiveBomVersionID  int64
	SourceProductID        int64
	SourceProductCode      string
	SourceProductName      string
	SourceBomProductID     int64
	SourceBomVersionID     int64
	SourceBomVersionNo     string
	DerivedFromLabel       string
	CanEditBOM             bool
	ProductionBomID        int64
	ProductionBomCode      string
	ProductionBomName      string
	ProductionBomVersionID int64
	ProductionBomVersionNo string
	LatestBomVersionID     int64
	LatestBomVersionNo     string
	IsLatestBomVersion     bool
	ProductionBomGroupID   int64
	ProductionBomGroupName string
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
	if binding, ok, err := productionBomBindingForProduct(ctx, q, schema, productID); err != nil {
		return bomSourceInfo{}, err
	} else if ok {
		info.BomSourceType = "owned"
		info.CanEditBOM = true
		info.EffectiveProductID = productID
		info.EffectiveBomVersionID = binding.BomVersionID
		info.ProductionBomID = binding.BomID
		info.ProductionBomCode = binding.BomCode
		info.ProductionBomName = binding.BomName
		info.ProductionBomVersionID = binding.BomVersionID
		info.ProductionBomVersionNo = binding.BomVersionNo
		info.LatestBomVersionID = binding.LatestBomVersionID
		info.LatestBomVersionNo = binding.LatestBomVersionNo
		info.IsLatestBomVersion = binding.IsLatestBomVersion
		info.ProductionBomGroupID = binding.ProductionBomGroupID
		info.ProductionBomGroupName = binding.ProductionBomGroup
		info.DerivedFromLabel = "默认生产 BOM"
		return info, nil
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
	if source.ProductionBomVersionID > 0 {
		var summary bomSummary
		err := q.QueryRow(ctx, fmt.Sprintf(`
			SELECT COALESCE(v.yield_rate,0.8)::float8,
			       CASE WHEN pb.status='inactive' THEN 'inactive' ELSE 'active' END,
			       COALESCE((SELECT COUNT(*) FROM %s.production_bom_version_items i WHERE i.version_id=v.id),0),
			       COALESCE(to_char(COALESCE(v.published_at, v.created_at),'YYYY-MM-DD HH24:MI'),'-')
			FROM %s.production_bom_versions v
			JOIN %s.production_boms pb ON pb.id=v.bom_id
			WHERE v.id=$1
		`, schema, schema, schema), source.ProductionBomVersionID).Scan(&summary.YieldRate, &summary.Status, &summary.ItemCount, &summary.UpdatedAt)
		if err == nil {
			return summary, nil
		}
		if err != pgx.ErrNoRows {
			return bomSummary{}, err
		}
	}
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
	item.ProductionBomID = source.ProductionBomID
	item.ProductionBomCode = source.ProductionBomCode
	item.ProductionBomName = source.ProductionBomName
	item.ProductionBomVersionID = source.ProductionBomVersionID
	item.ProductionBomVersionNo = source.ProductionBomVersionNo
	item.LatestBomVersionID = source.LatestBomVersionID
	item.LatestBomVersionNo = source.LatestBomVersionNo
	item.IsLatestBomVersion = source.IsLatestBomVersion
	item.ProductionBomGroupID = source.ProductionBomGroupID
	item.ProductionBomGroupName = source.ProductionBomGroupName
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
	detail.ProductionBomID = source.ProductionBomID
	detail.ProductionBomCode = source.ProductionBomCode
	detail.ProductionBomName = source.ProductionBomName
	detail.ProductionBomVersionID = source.ProductionBomVersionID
	detail.ProductionBomVersionNo = source.ProductionBomVersionNo
	detail.LatestBomVersionID = source.LatestBomVersionID
	detail.LatestBomVersionNo = source.LatestBomVersionNo
	detail.IsLatestBomVersion = source.IsLatestBomVersion
	detail.ProductionBomGroupID = source.ProductionBomGroupID
	detail.ProductionBomGroupName = source.ProductionBomGroupName
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

func listProductionBomVersionItems(ctx context.Context, q bomQueryer, schema string, versionID int64) ([]bomItemRow, float64, error) {
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
		FROM %s.production_bom_version_items bi
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
	if source.ProductionBomVersionID > 0 {
		return listProductionBomVersionItems(ctx, q, schema, source.ProductionBomVersionID)
	}
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
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return bomapp.Detail{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Validate the version exists and capture snapshots for inherit_version
	var bomSourceProductID int64
	var sourceProductName string
	var sourceVersionID int64
	var sourceVersionNo string
	var sourceProductCode string
	if cmd.SourceType == "owned" {
		// Unlock: remove the source restriction, go back to owned
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			DELETE FROM %s.product_bom_sources
			WHERE product_id=$1
		`, r.schema), cmd.ProductID); err != nil {
			return bomapp.Detail{}, fmt.Errorf("remove bom source: %w", err)
		}
		if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product_bom", &cmd.ProductID,
			"unlock_bom_source", postgresinfra.StrPtr("source_type"), nil,
			postgresinfra.StrPtr("owned"), postgresinfra.AuditMeta{
				"target_product_id": cmd.ProductID,
			}); err != nil {
			return bomapp.Detail{}, fmt.Errorf("audit unlock_bom_source: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return bomapp.Detail{}, err
		}
		return r.Detail(ctx, cmd.ProductID)
	}
	if cmd.SourceType == "inherit_version" {
		err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT bv.product_id, COALESCE(p.name,''), bv.id, COALESCE(bv.version_no,''), COALESCE(p.code,'')
			FROM %s.bom_versions bv
			JOIN %s.products p ON p.id=bv.product_id
			WHERE bv.id=$1 AND bv.status='active'
		`, r.schema, r.schema), cmd.SourceBomVersionID).Scan(&bomSourceProductID, &sourceProductName, &sourceVersionID, &sourceVersionNo, &sourceProductCode)
		if err != nil {
			return bomapp.Detail{}, fmt.Errorf("bom version not found: %w", err)
		}
	}

	// Read current effective source to capture source product info
	currentSource, err := resolveEffectiveBomSource(ctx, tx, r.schema, cmd.ProductID)
	if err != nil {
		return bomapp.Detail{}, err
	}
	srcProductID := currentSource.SourceProductID
	if srcProductID <= 0 {
		srcProductID = currentSource.EffectiveProductID
	}
	srcProductName := currentSource.SourceProductName
	srcProductCode := currentSource.SourceProductCode
	srcBomProductID := currentSource.EffectiveProductID
	if srcBomProductID <= 0 {
		srcBomProductID = currentSource.SourceBomProductID
	}

	// For inherit_current, set version to 0 (follow current active version)
	versionID := cmd.SourceBomVersionID
	versionNo := sourceVersionNo
	if cmd.SourceType == "inherit_current" {
		versionID = 0
		versionNo = ""
	}

	// Upsert into product_bom_sources
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_bom_sources(
			product_id, source_type, source_product_id, source_product_code_snapshot, source_product_name_snapshot,
			source_bom_product_id, source_bom_version_id, source_bom_version_no_snapshot,
			derived_from_product_id, derived_from_bom_version_id,
			updated_at
		)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,0,0,now())
		ON CONFLICT (product_id) DO UPDATE SET
			source_type=excluded.source_type,
			source_product_id=excluded.source_product_id,
			source_product_code_snapshot=excluded.source_product_code_snapshot,
			source_product_name_snapshot=excluded.source_product_name_snapshot,
			source_bom_product_id=excluded.source_bom_product_id,
			source_bom_version_id=excluded.source_bom_version_id,
			source_bom_version_no_snapshot=excluded.source_bom_version_no_snapshot,
			updated_at=now()
	`, r.schema), cmd.ProductID, cmd.SourceType, srcProductID, srcProductCode, srcProductName, srcBomProductID, versionID, versionNo); err != nil {
		return bomapp.Detail{}, fmt.Errorf("write bom source: %w", err)
	}

	// Audit log
	auditMeta := postgresinfra.AuditMeta{
		"bom_source_product_id":     srcProductID,
		"bom_source_product_code":   srcProductCode,
		"bom_source_product_name":   srcProductName,
		"bom_source_bom_product_id": srcBomProductID,
		"bom_source_bom_version_id": versionID,
		"bom_source_bom_version_no": versionNo,
		"target_product_id":         cmd.ProductID,
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product_bom", &cmd.ProductID,
		"set_bom_source", postgresinfra.StrPtr("source_type"), nil,
		postgresinfra.StrPtr(cmd.SourceType), auditMeta); err != nil {
		return bomapp.Detail{}, fmt.Errorf("audit set_bom_source: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return bomapp.Detail{}, err
	}
	return r.Detail(ctx, cmd.ProductID)
}

func productionBomBindingForProduct(ctx context.Context, q bomQueryer, schema string, productID int64) (bomapp.ProductProductionBomBinding, bool, error) {
	var row bomapp.ProductProductionBomBinding
	err := q.QueryRow(ctx, fmt.Sprintf(`
		SELECT b.product_id,
		       pb.id,
		       COALESCE(pb.code,''),
		       COALESCE(pb.name,''),
		       v.id,
		       COALESCE(v.version_no,''),
		       COALESCE(latest.id,0),
		       COALESCE(latest.version_no,''),
		       v.id=COALESCE(latest.id,0),
		       COALESCE(g.id,0),
		       COALESCE(g.name,'')
		FROM %s.product_production_bom_bindings b
		JOIN %s.production_boms pb ON pb.id=b.bom_id
		JOIN %s.production_bom_versions v ON v.id=b.bom_version_id
		LEFT JOIN %s.production_bom_groups g ON g.id=pb.group_id
		LEFT JOIN LATERAL (
			SELECT id, version_no
			FROM %s.production_bom_versions lv
			WHERE lv.bom_id=pb.id AND lv.status='published'
			ORDER BY lv.published_at DESC NULLS LAST, lv.id DESC
			LIMIT 1
		) latest ON true
		WHERE b.product_id=$1
	`, schema, schema, schema, schema, schema), productID).Scan(
		&row.ProductID,
		&row.BomID,
		&row.BomCode,
		&row.BomName,
		&row.BomVersionID,
		&row.BomVersionNo,
		&row.LatestBomVersionID,
		&row.LatestBomVersionNo,
		&row.IsLatestBomVersion,
		&row.ProductionBomGroupID,
		&row.ProductionBomGroup,
	)
	if err == pgx.ErrNoRows {
		return bomapp.ProductProductionBomBinding{}, false, nil
	}
	if err != nil {
		return bomapp.ProductProductionBomBinding{}, false, err
	}
	return row, true, nil
}

func (r Repository) ListProductionBomGroups(ctx context.Context, _ bool) ([]bomapp.ProductionBomGroup, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, name, sort_order, active
		FROM %s.production_bom_groups
		WHERE active=true
		  AND name NOT IN ('默认分组','默认配方组')
		ORDER BY sort_order, name, id
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]bomapp.ProductionBomGroup, 0)
	for rows.Next() {
		var row bomapp.ProductionBomGroup
		if err := rows.Scan(&row.ID, &row.Name, &row.SortOrder, &row.Active); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}
	categoryRows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, group_id, name, sort_order
		FROM %s.production_bom_group_categories
		ORDER BY group_id, sort_order, name, id
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer categoryRows.Close()
	byGroup := map[int64][]bomapp.ProductionBomGroupCategory{}
	for categoryRows.Next() {
		var row bomapp.ProductionBomGroupCategory
		if err := categoryRows.Scan(&row.ID, &row.GroupID, &row.Name, &row.SortOrder); err != nil {
			return nil, err
		}
		byGroup[row.GroupID] = append(byGroup[row.GroupID], row)
	}
	if err := categoryRows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Categories = byGroup[out[i].ID]
	}
	return out, nil
}

func (r Repository) CreateProductionBomGroup(ctx context.Context, cmd bomapp.CreateProductionBomGroupCommand) (bomapp.ProductionBomGroup, error) {
	return bomapp.ProductionBomGroup{}, ErrLegacyProductionBomGroupsReadonly
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return bomapp.ProductionBomGroup{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var id int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_groups(name, sort_order, active, created_by, updated_by)
		VALUES($1,$2,true,$3,$3)
		RETURNING id
	`, r.schema), strings.TrimSpace(cmd.Name), cmd.SortOrder, strings.TrimSpace(cmd.Actor)).Scan(&id); err != nil {
		return bomapp.ProductionBomGroup{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "production_bom_group", &id, "create", postgresinfra.StrPtr("name"), nil, postgresinfra.StrPtr(strings.TrimSpace(cmd.Name)), postgresinfra.AuditMeta{"name": strings.TrimSpace(cmd.Name), "sort_order": cmd.SortOrder}); err != nil {
		return bomapp.ProductionBomGroup{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return bomapp.ProductionBomGroup{}, err
	}
	return bomapp.ProductionBomGroup{ID: id, Name: strings.TrimSpace(cmd.Name), SortOrder: cmd.SortOrder, Active: true}, nil
}

func (r Repository) UpdateProductionBomGroup(ctx context.Context, cmd bomapp.UpdateProductionBomGroupCommand) (bomapp.ProductionBomGroup, error) {
	return bomapp.ProductionBomGroup{}, ErrLegacyProductionBomGroupsReadonly
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return bomapp.ProductionBomGroup{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.production_bom_groups
		SET name=$2, sort_order=$3, updated_at=now(), updated_by=$4
		WHERE id=$1
	`, r.schema), cmd.ID, strings.TrimSpace(cmd.Name), cmd.SortOrder, strings.TrimSpace(cmd.Actor)); err != nil {
		return bomapp.ProductionBomGroup{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "production_bom_group", &cmd.ID, "update", postgresinfra.StrPtr("name"), nil, postgresinfra.StrPtr(strings.TrimSpace(cmd.Name)), postgresinfra.AuditMeta{"group_id": cmd.ID, "name": strings.TrimSpace(cmd.Name), "sort_order": cmd.SortOrder}); err != nil {
		return bomapp.ProductionBomGroup{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return bomapp.ProductionBomGroup{}, err
	}
	var row bomapp.ProductionBomGroup
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT id, name, sort_order, active FROM %s.production_bom_groups WHERE id=$1`, r.schema), cmd.ID).Scan(&row.ID, &row.Name, &row.SortOrder, &row.Active); err != nil {
		return bomapp.ProductionBomGroup{}, err
	}
	return row, nil
}

func (r Repository) DeleteProductionBomGroup(ctx context.Context, cmd bomapp.DeleteProductionBomGroupCommand) error {
	return ErrLegacyProductionBomGroupsReadonly
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var existingID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_bom_groups WHERE id=$1 FOR UPDATE`, r.schema), cmd.ID).Scan(&existingID); err != nil {
		return err
	}
	if existingID <= 0 {
		return pgx.ErrNoRows
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.production_boms
		SET group_id=0,
		    group_category_id=0,
		    updated_at=now(), updated_by=$2
		WHERE group_id=$1
	`, r.schema), cmd.ID, strings.TrimSpace(cmd.Actor)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.production_bom_group_categories WHERE group_id=$1`, r.schema), cmd.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.production_bom_groups WHERE id=$1`, r.schema), cmd.ID); err != nil {
		return err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "production_bom_group", &cmd.ID, "delete_production_bom_group", postgresinfra.StrPtr("group_id"), postgresinfra.StrPtr(fmt.Sprint(cmd.ID)), nil, postgresinfra.AuditMeta{"group_id": cmd.ID, "moved_to": "unclassified"}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) MoveProductionBomGroup(ctx context.Context, cmd bomapp.MoveProductionBomGroupCommand) error {
	return ErrLegacyProductionBomGroupsReadonly
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.production_bom_groups
		SET sort_order=$2, updated_at=now(), updated_by=$3
		WHERE id=$1
	`, r.schema), cmd.ID, cmd.SortOrder, strings.TrimSpace(cmd.Actor)); err != nil {
		return err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "production_bom_group", &cmd.ID, "move_production_bom_group", postgresinfra.StrPtr("sort_order"), nil, postgresinfra.StrPtr(fmt.Sprint(cmd.SortOrder)), postgresinfra.AuditMeta{"group_id": cmd.ID, "sort_order": cmd.SortOrder}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) CreateProductionBomGroupCategory(ctx context.Context, cmd bomapp.CreateProductionBomGroupCategoryCommand) (bomapp.ProductionBomGroupCategory, error) {
	return bomapp.ProductionBomGroupCategory{}, ErrLegacyProductionBomGroupsReadonly
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return bomapp.ProductionBomGroupCategory{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	groupID, err := ensureProductionBomGroupTx(ctx, tx, r.schema, cmd.GroupID)
	if err != nil {
		return bomapp.ProductionBomGroupCategory{}, err
	}
	if groupID <= 0 {
		return bomapp.ProductionBomGroupCategory{}, fmt.Errorf("group_id required")
	}
	var id int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_group_categories(group_id, name, sort_order, created_by, updated_by)
		VALUES($1,$2,$3,$4,$4)
		RETURNING id
	`, r.schema), groupID, strings.TrimSpace(cmd.Name), cmd.SortOrder, strings.TrimSpace(cmd.Actor)).Scan(&id); err != nil {
		return bomapp.ProductionBomGroupCategory{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "production_bom_group_category", &id, "create_production_bom_group_category", postgresinfra.StrPtr("name"), nil, postgresinfra.StrPtr(strings.TrimSpace(cmd.Name)), postgresinfra.AuditMeta{"group_id": groupID, "category_id": id, "sort_order": cmd.SortOrder}); err != nil {
		return bomapp.ProductionBomGroupCategory{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return bomapp.ProductionBomGroupCategory{}, err
	}
	return bomapp.ProductionBomGroupCategory{ID: id, GroupID: groupID, Name: strings.TrimSpace(cmd.Name), SortOrder: cmd.SortOrder}, nil
}

func (r Repository) UpdateProductionBomGroupCategory(ctx context.Context, cmd bomapp.UpdateProductionBomGroupCategoryCommand) (bomapp.ProductionBomGroupCategory, error) {
	return bomapp.ProductionBomGroupCategory{}, ErrLegacyProductionBomGroupsReadonly
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return bomapp.ProductionBomGroupCategory{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.production_bom_group_categories
		SET name=$2, sort_order=$3, updated_at=now(), updated_by=$4
		WHERE id=$1
	`, r.schema), cmd.ID, strings.TrimSpace(cmd.Name), cmd.SortOrder, strings.TrimSpace(cmd.Actor)); err != nil {
		return bomapp.ProductionBomGroupCategory{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "production_bom_group_category", &cmd.ID, "update_production_bom_group_category", postgresinfra.StrPtr("name"), nil, postgresinfra.StrPtr(strings.TrimSpace(cmd.Name)), postgresinfra.AuditMeta{"category_id": cmd.ID, "sort_order": cmd.SortOrder}); err != nil {
		return bomapp.ProductionBomGroupCategory{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return bomapp.ProductionBomGroupCategory{}, err
	}
	var row bomapp.ProductionBomGroupCategory
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT id, group_id, name, sort_order FROM %s.production_bom_group_categories WHERE id=$1`, r.schema), cmd.ID).Scan(&row.ID, &row.GroupID, &row.Name, &row.SortOrder); err != nil {
		return bomapp.ProductionBomGroupCategory{}, err
	}
	return row, nil
}

func (r Repository) DeleteProductionBomGroupCategory(ctx context.Context, cmd bomapp.DeleteProductionBomGroupCategoryCommand) error {
	return ErrLegacyProductionBomGroupsReadonly
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var groupID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT group_id FROM %s.production_bom_group_categories WHERE id=$1 FOR UPDATE`, r.schema), cmd.ID).Scan(&groupID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.production_boms
		SET group_category_id=0, updated_at=now(), updated_by=$2
		WHERE group_category_id=$1
	`, r.schema), cmd.ID, strings.TrimSpace(cmd.Actor)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.production_bom_group_categories WHERE id=$1`, r.schema), cmd.ID); err != nil {
		return err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "production_bom_group_category", &cmd.ID, "delete_production_bom_group_category", postgresinfra.StrPtr("category_id"), postgresinfra.StrPtr(fmt.Sprint(cmd.ID)), nil, postgresinfra.AuditMeta{"group_id": groupID, "category_id": cmd.ID, "moved_to": "group_unclassified"}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) ListProductionBoms(ctx context.Context) ([]bomapp.ProductionBomSummary, error) {
	if err := repairLegacyProductionBomBindings(ctx, r.pool, r.schema); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, productionBomSummarySQL(r.schema, "1=1")+" ORDER BY COALESCE(g.sort_order,100), pb.name, pb.id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProductionBomSummaries(rows)
}

func (r Repository) GetProductionBomDetail(ctx context.Context, id int64, versionID int64) (bomapp.ProductionBomDetail, error) {
	rows, err := r.pool.Query(ctx, productionBomSummarySQL(r.schema, "pb.id=$1"), id)
	if err != nil {
		return bomapp.ProductionBomDetail{}, err
	}
	summaries, err := scanProductionBomSummaries(rows)
	if err != nil {
		return bomapp.ProductionBomDetail{}, err
	}
	if len(summaries) == 0 {
		return bomapp.ProductionBomDetail{}, pgx.ErrNoRows
	}
	versions, err := r.listProductionBomVersions(ctx, id)
	if err != nil {
		return bomapp.ProductionBomDetail{}, err
	}
	detailVersionID := int64(0)
	if versionID > 0 {
		for _, version := range versions {
			if version.ID == versionID {
				detailVersionID = version.ID
				break
			}
		}
	}
	if detailVersionID <= 0 {
		detailVersionID = summaries[0].LatestVersionID
		for _, version := range versions {
			if version.Status == "draft" {
				detailVersionID = version.ID
				break
			}
		}
	}
	items := []bomapp.Item{}
	selectedVersion := bomapp.ProductionBomVersion{}
	if detailVersionID > 0 {
		for _, version := range versions {
			if version.ID == detailVersionID {
				selectedVersion = version
				break
			}
		}
		itemRows, _, err := listProductionBomVersionItems(ctx, r.pool, r.schema, detailVersionID)
		if err != nil {
			return bomapp.ProductionBomDetail{}, err
		}
		items = bomItemsToApp(itemRows)
	}
	summary := summaries[0]
	if selectedVersion.ID > 0 {
		summary.LatestVersionID = selectedVersion.ID
		summary.LatestVersionNo = selectedVersion.VersionNo
		summary.LatestVersionStatus = selectedVersion.Status
		summary.ExpectedYieldRate = selectedVersion.ExpectedYieldRate
		summary.ExpectedLossRate = selectedVersion.ExpectedLossRate
	}
	referencedProducts, err := r.listProductionBomReferencedProducts(ctx, id)
	if err != nil {
		return bomapp.ProductionBomDetail{}, err
	}
	usedByBoms, err := r.listProductionBomComponentUsedByBoms(ctx, summary.OutputProductID)
	if err != nil {
		return bomapp.ProductionBomDetail{}, err
	}
	return bomapp.ProductionBomDetail{ProductionBomSummary: summary, Versions: versions, Items: items, ReferencedProducts: referencedProducts, UsedByBoms: usedByBoms}, nil
}

func (r Repository) ListProductionBomUsageByProduct(ctx context.Context, productID int64) ([]bomapp.ProductionBomUsedByBom, error) {
	return r.listProductionBomUsageByProduct(ctx, productID)
}

func (r Repository) CreateProductionBom(ctx context.Context, cmd bomapp.CreateProductionBomCommand) (bomapp.ProductionBomSummary, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return bomapp.ProductionBomSummary{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	groupID := cmd.GroupID
	groupCategoryID := cmd.GroupCategoryID
	tempCode := fmt.Sprintf("PENDING-%d", time.Now().UnixNano())
	var bomID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_boms(code, name, output_product_id, status, created_by, updated_by)
		VALUES($1,$2,$3,'active',$4,$4)
		RETURNING id
	`, r.schema), tempCode, strings.TrimSpace(cmd.Name), cmd.OutputProductID, strings.TrimSpace(cmd.Actor)).Scan(&bomID); err != nil {
		return bomapp.ProductionBomSummary{}, err
	}
	code := fmt.Sprintf("BOM-%06d", bomID)
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_boms SET code=$1 WHERE id=$2`, r.schema), code, bomID); err != nil {
		return bomapp.ProductionBomSummary{}, err
	}
	yieldRate := 0.8
	if cmd.ExpectedLossRate != nil {
		yieldRate = productiondomain.NormalizeYieldRate(1 - *cmd.ExpectedLossRate)
	}
	var versionID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_versions(bom_id, version_no, status, yield_rate, output_qty, output_unit, note, created_at, created_by)
		VALUES($1,'V001','draft',$2,$3,$4,'初始版本',now(),$5)
		RETURNING id
	`, r.schema), bomID, yieldRate, cmd.OutputQty, strings.TrimSpace(cmd.OutputUnit), strings.TrimSpace(cmd.Actor)).Scan(&versionID); err != nil {
		return bomapp.ProductionBomSummary{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "production_bom", &bomID, "create", postgresinfra.StrPtr("code"), nil, postgresinfra.StrPtr(code), postgresinfra.AuditMeta{"bom_id": bomID, "bom_version_id": versionID, "name": strings.TrimSpace(cmd.Name), "output_product_id": cmd.OutputProductID, "output_qty": cmd.OutputQty, "output_unit": strings.TrimSpace(cmd.OutputUnit), "group_id": groupID, "group_category_id": groupCategoryID}); err != nil {
		return bomapp.ProductionBomSummary{}, err
	}
	if err := saveBusinessGroupAssignmentForProductionBomTx(ctx, tx, r.schema, strings.TrimSpace(cmd.Actor), bomID, groupID, groupCategoryID); err != nil {
		return bomapp.ProductionBomSummary{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return bomapp.ProductionBomSummary{}, err
	}
	return r.productionBomSummaryByID(ctx, bomID)
}

func (r Repository) UpdateProductionBom(ctx context.Context, cmd bomapp.UpdateProductionBomCommand) (bomapp.ProductionBomSummary, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return bomapp.ProductionBomSummary{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	groupID := cmd.GroupID
	groupCategoryID := cmd.GroupCategoryID
	status := strings.TrimSpace(cmd.Status)
	if status == "" {
		status = "active"
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.production_boms
		SET name=COALESCE(NULLIF($2,''), name),
		    output_product_id=COALESCE(NULLIF($3,0), output_product_id),
		    status=$4, updated_at=now(), updated_by=$5
		WHERE id=$1
	`, r.schema), cmd.ID, strings.TrimSpace(cmd.Name), cmd.OutputProductID, status, strings.TrimSpace(cmd.Actor)); err != nil {
		return bomapp.ProductionBomSummary{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "production_bom", &cmd.ID, "update", postgresinfra.StrPtr("status"), nil, postgresinfra.StrPtr(status), postgresinfra.AuditMeta{"bom_id": cmd.ID, "name": strings.TrimSpace(cmd.Name), "group_id": groupID, "group_category_id": groupCategoryID, "status": status}); err != nil {
		return bomapp.ProductionBomSummary{}, err
	}
	if err := saveBusinessGroupAssignmentForProductionBomTx(ctx, tx, r.schema, strings.TrimSpace(cmd.Actor), cmd.ID, groupID, groupCategoryID); err != nil {
		return bomapp.ProductionBomSummary{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return bomapp.ProductionBomSummary{}, err
	}
	return r.productionBomSummaryByID(ctx, cmd.ID)
}

func (r Repository) CopyProductionBom(ctx context.Context, cmd bomapp.CopyProductionBomCommand) (bomapp.ProductionBomSummary, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return bomapp.ProductionBomSummary{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	groupID := cmd.GroupID
	groupCategoryID := cmd.GroupCategoryID
	var sourceName string
	var sourceOutputProductID int64
	var sourceVersionID int64
	var sourceYield float64
	var sourceOutputQty float64
	var sourceOutputUnit string
	var sourceSpecialAttrsSchemaJSON string
	var sourceSpecialAttrsJSON string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT pb.name, COALESCE(pb.output_product_id,0), v.id, COALESCE(v.yield_rate,0.8)::float8, COALESCE(v.output_qty,1)::float8, COALESCE(NULLIF(v.output_unit,''),'unit'), COALESCE(v.special_attrs_schema_json::text,'[]'), COALESCE(v.special_attrs_json::text,'{}')
		FROM %s.production_boms pb
		JOIN LATERAL (
			SELECT id, yield_rate, output_qty, output_unit, special_attrs_schema_json, special_attrs_json
			FROM %s.production_bom_versions
			WHERE bom_id=pb.id AND status IN ('draft','published')
			ORDER BY CASE WHEN status='draft' THEN 0 ELSE 1 END, published_at DESC NULLS LAST, created_at DESC, id DESC
			LIMIT 1
		) v ON true
		WHERE pb.id=$1
	`, r.schema, r.schema), cmd.ID).Scan(&sourceName, &sourceOutputProductID, &sourceVersionID, &sourceYield, &sourceOutputQty, &sourceOutputUnit, &sourceSpecialAttrsSchemaJSON, &sourceSpecialAttrsJSON); err != nil {
		return bomapp.ProductionBomSummary{}, fmt.Errorf("source production BOM not found")
	}
	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		name = sourceName + " 副本"
	}
	tempCode := fmt.Sprintf("PENDING-%d", time.Now().UnixNano())
	var newBomID int64
	outputProductID := sourceOutputProductID
	if cmd.OutputProductID > 0 {
		outputProductID = cmd.OutputProductID
	}
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_boms(code, name, output_product_id, status, source_bom_id, source_bom_version_id, created_by, updated_by)
		VALUES($1,$2,$3,'active',$4,$5,$6,$6)
		RETURNING id
	`, r.schema), tempCode, name, outputProductID, cmd.ID, sourceVersionID, strings.TrimSpace(cmd.Actor)).Scan(&newBomID); err != nil {
		return bomapp.ProductionBomSummary{}, err
	}
	code := fmt.Sprintf("BOM-%06d", newBomID)
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_boms SET code=$1 WHERE id=$2`, r.schema), code, newBomID); err != nil {
		return bomapp.ProductionBomSummary{}, err
	}
	var newVersionID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_versions(bom_id, version_no, status, yield_rate, output_qty, output_unit, note, special_attrs_schema_json, special_attrs_json, created_at, created_by)
		VALUES($1,'V001','draft',$2,$3,$4,'复制来源 BOM',$5::jsonb,$6::jsonb,now(),$7)
		RETURNING id
	`, r.schema), newBomID, sourceYield, sourceOutputQty, sourceOutputUnit, sourceSpecialAttrsSchemaJSON, sourceSpecialAttrsJSON, strings.TrimSpace(cmd.Actor)).Scan(&newVersionID); err != nil {
		return bomapp.ProductionBomSummary{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_version_items(version_id, material_id, component_type, component_product_id, component_spec_g, consume_unit, qty_per_unit, ratio_pct, unit_cost_snapshot)
		SELECT $1, material_id, component_type, component_product_id, component_spec_g, consume_unit, qty_per_unit, ratio_pct, unit_cost_snapshot
		FROM %s.production_bom_version_items
		WHERE version_id=$2
		ORDER BY id
	`, r.schema, r.schema), newVersionID, sourceVersionID); err != nil {
		return bomapp.ProductionBomSummary{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "production_bom", &newBomID, "copy_production_bom", postgresinfra.StrPtr("source_bom_id"), postgresinfra.StrPtr(fmt.Sprintf("%d", cmd.ID)), postgresinfra.StrPtr(fmt.Sprintf("%d", newBomID)), postgresinfra.AuditMeta{"source_bom_id": cmd.ID, "source_bom_version_id": sourceVersionID, "target_bom_id": newBomID, "target_bom_version_id": newVersionID, "group_id": groupID, "group_category_id": groupCategoryID}); err != nil {
		return bomapp.ProductionBomSummary{}, err
	}
	if err := saveBusinessGroupAssignmentForProductionBomTx(ctx, tx, r.schema, strings.TrimSpace(cmd.Actor), newBomID, groupID, groupCategoryID); err != nil {
		return bomapp.ProductionBomSummary{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return bomapp.ProductionBomSummary{}, err
	}
	return r.productionBomSummaryByID(ctx, newBomID)
}

func saveBusinessGroupAssignmentForProductionBomTx(ctx context.Context, tx pgx.Tx, schema string, actor string, bomID int64, groupID int64, groupItemID int64) error {
	if bomID <= 0 {
		return nil
	}
	if groupID <= 0 || groupItemID <= 0 {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			DELETE FROM %s.business_group_assignments
			WHERE lower(usage_key)='production_bom' AND lower(object_key)='production_bom' AND object_id=$1 AND object_ref=''
		`, schema), bomID); err != nil {
			return err
		}
		return nil
	}
	var ok bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT EXISTS(
			SELECT 1
			FROM %s.business_groups bg
			JOIN %s.business_group_usages bgu ON bgu.group_id=bg.id
			JOIN %s.business_group_items item ON item.group_id=bg.id
			WHERE bg.id=$1
			  AND item.id=$2
			  AND bg.active=true
			  AND item.active=true
			  AND bgu.active=true
			  AND lower(bgu.usage_key)='production_bom'
		)
	`, schema, schema, schema), groupID, groupItemID).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("business group item mismatch")
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		DELETE FROM %s.business_group_assignments
		WHERE lower(usage_key)='production_bom' AND lower(object_key)='production_bom' AND object_id=$1 AND object_ref=''
	`, schema), bomID); err != nil {
		return err
	}
	var assignmentID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.business_group_assignments(group_id, group_item_id, usage_key, object_key, object_id, object_ref, sort_order, created_by, updated_by)
		VALUES($1,$2,'production_bom','production_bom',$3,'',100,$4,$4)
		RETURNING id
	`, schema), groupID, groupItemID, bomID, actor).Scan(&assignmentID); err != nil {
		return err
	}
	return postgresinfra.AuditInsertTx(ctx, tx, schema, actor, "business_group_assignment", &assignmentID, "save_business_group_assignment", postgresinfra.StrPtr("production_bom"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", groupItemID)), postgresinfra.AuditMeta{"bom_id": bomID, "group_id": groupID, "group_item_id": groupItemID, "usage_key": "production_bom", "object_key": "production_bom"})
}

func (r Repository) CreateProductionBomVersion(ctx context.Context, cmd bomapp.CreateProductionBomVersionCommand) (bomapp.ProductionBomVersion, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return bomapp.ProductionBomVersion{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var sourceVersionID int64
	var yieldRate float64
	var outputQty float64
	var outputUnit string
	var specialAttrsSchemaJSON string
	var specialAttrsJSON string
	if cmd.SourceVersionID > 0 {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT id, COALESCE(yield_rate,0.8)::float8, COALESCE(output_qty,1)::float8, COALESCE(NULLIF(output_unit,''),'unit'), COALESCE(special_attrs_schema_json::text,'[]'), COALESCE(special_attrs_json::text,'{}')
			FROM %s.production_bom_versions
			WHERE bom_id=$1 AND id=$2 AND status IN ('published','draft')
		`, r.schema), cmd.BomID, cmd.SourceVersionID).Scan(&sourceVersionID, &yieldRate, &outputQty, &outputUnit, &specialAttrsSchemaJSON, &specialAttrsJSON); err != nil {
			return bomapp.ProductionBomVersion{}, fmt.Errorf("source production BOM version not found")
		}
	} else if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT id, COALESCE(yield_rate,0.8)::float8, COALESCE(output_qty,1)::float8, COALESCE(NULLIF(output_unit,''),'unit'), COALESCE(special_attrs_schema_json::text,'[]'), COALESCE(special_attrs_json::text,'{}')
			FROM %s.production_bom_versions
			WHERE bom_id=$1 AND status='published'
			ORDER BY published_at DESC NULLS LAST, id DESC
			LIMIT 1
		`, r.schema), cmd.BomID).Scan(&sourceVersionID, &yieldRate, &outputQty, &outputUnit, &specialAttrsSchemaJSON, &specialAttrsJSON); err != nil {
		return bomapp.ProductionBomVersion{}, fmt.Errorf("published production BOM version not found")
	}
	var next int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(COUNT(*),0)+1 FROM %s.production_bom_versions WHERE bom_id=$1`, r.schema), cmd.BomID).Scan(&next); err != nil {
		return bomapp.ProductionBomVersion{}, err
	}
	versionNo := fmt.Sprintf("V%03d", next)
	var versionID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_versions(bom_id, version_no, status, yield_rate, output_qty, output_unit, note, special_attrs_schema_json, special_attrs_json, created_at, created_by)
		VALUES($1,$2,'draft',$3,$4,$5,$6,$7::jsonb,$8::jsonb,now(),$9)
		RETURNING id
	`, r.schema), cmd.BomID, versionNo, yieldRate, outputQty, outputUnit, strings.TrimSpace(cmd.Note), specialAttrsSchemaJSON, specialAttrsJSON, strings.TrimSpace(cmd.Actor)).Scan(&versionID); err != nil {
		return bomapp.ProductionBomVersion{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_version_items(version_id, material_id, component_type, component_product_id, component_spec_g, consume_unit, qty_per_unit, ratio_pct, unit_cost_snapshot)
		SELECT $1, material_id, component_type, component_product_id, component_spec_g, consume_unit, qty_per_unit, ratio_pct, unit_cost_snapshot
		FROM %s.production_bom_version_items
		WHERE version_id=$2
		ORDER BY id
	`, r.schema, r.schema), versionID, sourceVersionID); err != nil {
		return bomapp.ProductionBomVersion{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "production_bom_version", &versionID, "create", postgresinfra.StrPtr("version_no"), nil, postgresinfra.StrPtr(versionNo), postgresinfra.AuditMeta{"bom_id": cmd.BomID, "source_version_id": sourceVersionID}); err != nil {
		return bomapp.ProductionBomVersion{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return bomapp.ProductionBomVersion{}, err
	}
	return r.productionBomVersionByID(ctx, versionID)
}

func (r Repository) UpdateProductionBomVersionDraft(ctx context.Context, cmd bomapp.UpdateProductionBomVersionDraftCommand) (bomapp.ProductionBomVersion, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return bomapp.ProductionBomVersion{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var status string
	var yieldRate float64
	var outputQty float64
	var outputUnit string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT status, COALESCE(yield_rate,0.8)::float8, COALESCE(output_qty,1)::float8, COALESCE(NULLIF(output_unit,''),'unit') FROM %s.production_bom_versions WHERE id=$1 FOR UPDATE`, r.schema), cmd.VersionID).Scan(&status, &yieldRate, &outputQty, &outputUnit); err != nil {
		return bomapp.ProductionBomVersion{}, fmt.Errorf("production BOM version not found")
	}
	if status != "draft" {
		return bomapp.ProductionBomVersion{}, fmt.Errorf("published production BOM version is read-only")
	}
	if cmd.ExpectedLossRate != nil {
		yieldRate = productiondomain.NormalizeYieldRate(1 - *cmd.ExpectedLossRate)
	}
	if cmd.OutputQty > 0 {
		outputQty = cmd.OutputQty
	}
	if strings.TrimSpace(cmd.OutputUnit) != "" {
		outputUnit = strings.TrimSpace(cmd.OutputUnit)
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.production_bom_versions
		SET yield_rate=$2,
		    output_qty=$3,
		    output_unit=$4,
		    special_attrs_schema_json=CASE WHEN $5<>'' THEN $5::jsonb ELSE special_attrs_schema_json END,
		    special_attrs_json=CASE WHEN $6<>'' THEN $6::jsonb ELSE special_attrs_json END
		WHERE id=$1
	`, r.schema), cmd.VersionID, yieldRate, outputQty, outputUnit, cmd.SpecialAttrsSchemaJSON, cmd.SpecialAttrsJSON); err != nil {
		return bomapp.ProductionBomVersion{}, err
	}
	if cmd.Items != nil {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.production_bom_version_items WHERE version_id=$1`, r.schema), cmd.VersionID); err != nil {
			return bomapp.ProductionBomVersion{}, err
		}
		for _, item := range cmd.Items {
			componentType := strings.TrimSpace(item.ComponentType)
			if componentType == "" {
				componentType = "material"
			}
			if _, err := tx.Exec(ctx, fmt.Sprintf(`
				INSERT INTO %s.production_bom_version_items(version_id, material_id, component_type, component_product_id, component_spec_g, consume_unit, qty_per_unit, ratio_pct, unit_cost_snapshot)
				VALUES($1,$2,$3,$4,$5,$6,$7,$8,COALESCE((SELECT purchase_price FROM %s.materials WHERE id=$2),0))
			`, r.schema, r.schema), cmd.VersionID, item.MaterialID, componentType, item.ComponentProductID, item.ComponentSpecG, item.ConsumeUnit, item.QtyPerUnit, item.RatioPct); err != nil {
				return bomapp.ProductionBomVersion{}, err
			}
		}
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "production_bom_version", &cmd.VersionID, "update_draft", postgresinfra.StrPtr("version_id"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", cmd.VersionID)), postgresinfra.AuditMeta{"version_id": cmd.VersionID, "item_count": len(cmd.Items)}); err != nil {
		return bomapp.ProductionBomVersion{}, err
	}
	if cmd.SpecialAttrsSchemaJSON != "" || cmd.SpecialAttrsJSON != "" {
		if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "production_bom_version", &cmd.VersionID, "update_special_attrs", postgresinfra.StrPtr("special_attrs_json"), nil, postgresinfra.StrPtr(cmd.SpecialAttrsJSON), postgresinfra.AuditMeta{"version_id": cmd.VersionID, "special_attrs_schema_json": cmd.SpecialAttrsSchemaJSON}); err != nil {
			return bomapp.ProductionBomVersion{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return bomapp.ProductionBomVersion{}, err
	}
	return r.productionBomVersionByID(ctx, cmd.VersionID)
}

func (r Repository) ValidateProductionBomVersionForPublish(ctx context.Context, cmd bomapp.PublishProductionBomVersionCommand) error {
	var bomID int64
	var outputProductID int64
	var itemCount int64
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT v.bom_id,
		       COALESCE(pb.output_product_id,0),
		       COALESCE((SELECT COUNT(*) FROM %s.production_bom_version_items i WHERE i.version_id=v.id),0)
		FROM %s.production_bom_versions v
		JOIN %s.production_boms pb ON pb.id=v.bom_id
		WHERE v.id=$1 AND v.status='draft'
	`, r.schema, r.schema, r.schema), cmd.VersionID).Scan(&bomID, &outputProductID, &itemCount); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("production BOM version not found")
		}
		return err
	}
	if outputProductID <= 0 {
		return fmt.Errorf("output_product_id required")
	}
	if itemCount <= 0 {
		return fmt.Errorf("components required")
	}
	var hasCycle bool
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		WITH RECURSIVE selected_versions AS (
			SELECT pb.id AS bom_id,
			       pb.output_product_id,
			       v.id AS version_id
			FROM %[1]s.production_boms pb
			JOIN LATERAL (
				SELECT id, status, published_at, created_at
				FROM %[1]s.production_bom_versions pv
				WHERE pv.bom_id=pb.id
				  AND pv.status IN ('draft','published')
				ORDER BY CASE WHEN pv.id=$1 THEN 0 WHEN pv.status='published' THEN 1 ELSE 2 END,
				         pv.published_at DESC NULLS LAST,
				         pv.created_at DESC,
				         pv.id DESC
				LIMIT 1
			) v ON true
			WHERE pb.output_product_id > 0
		),
		edges AS (
			SELECT sv.output_product_id AS parent_product_id,
			       i.component_product_id AS component_product_id
			FROM selected_versions sv
			JOIN %[1]s.production_bom_version_items i ON i.version_id=sv.version_id
			WHERE i.component_type IN ('product','finished_product')
			  AND i.component_product_id > 0
		),
		walk(product_id, path) AS (
			SELECT $2::bigint, ARRAY[$2::bigint]
			UNION ALL
			SELECT e.component_product_id, walk.path || e.component_product_id
			FROM walk
			JOIN edges e ON e.parent_product_id=walk.product_id
			WHERE NOT e.component_product_id = ANY(walk.path)
		)
		SELECT EXISTS(
			SELECT 1
			FROM walk
			JOIN edges e ON e.parent_product_id=walk.product_id
			WHERE e.component_product_id=$2
		)
	`, r.schema), cmd.VersionID, outputProductID).Scan(&hasCycle); err != nil {
		return err
	}
	if hasCycle {
		return fmt.Errorf("cycle detected")
	}
	_ = bomID
	return nil
}

func (r Repository) PublishProductionBomVersion(ctx context.Context, cmd bomapp.PublishProductionBomVersionCommand) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var bomID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT bom_id FROM %s.production_bom_versions WHERE id=$1 FOR UPDATE`, r.schema), cmd.VersionID).Scan(&bomID); err != nil {
		return fmt.Errorf("production BOM version not found")
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_versions SET status='published', published_at=now(), published_by=$2 WHERE id=$1`, r.schema), cmd.VersionID, strings.TrimSpace(cmd.Actor)); err != nil {
		return err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "production_bom_version", &cmd.VersionID, "publish_production_bom_version", postgresinfra.StrPtr("status"), nil, postgresinfra.StrPtr("published"), postgresinfra.AuditMeta{"bom_id": bomID, "version_id": cmd.VersionID}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) BindProductProductionBom(ctx context.Context, cmd bomapp.BindProductProductionBomCommand) (bomapp.ProductProductionBomBinding, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return bomapp.ProductProductionBomBinding{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var versionBomID int64
	var versionNo string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT bom_id, version_no FROM %s.production_bom_versions WHERE id=$1 AND status='published'`, r.schema), cmd.BomVersionID).Scan(&versionBomID, &versionNo); err != nil {
		return bomapp.ProductProductionBomBinding{}, fmt.Errorf("published production BOM version not found")
	}
	if versionBomID != cmd.BomID {
		return bomapp.ProductProductionBomBinding{}, fmt.Errorf("bom_version_id does not belong to bom_id")
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_production_bom_bindings(product_id, bom_id, bom_version_id, bound_at, bound_by)
		VALUES($1,$2,$3,now(),$4)
		ON CONFLICT (product_id) DO UPDATE SET bom_id=excluded.bom_id, bom_version_id=excluded.bom_version_id, bound_at=now(), bound_by=excluded.bound_by
	`, r.schema), cmd.ProductID, cmd.BomID, cmd.BomVersionID, strings.TrimSpace(cmd.Actor)); err != nil {
		return bomapp.ProductProductionBomBinding{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product", &cmd.ProductID, "bind_production_bom", postgresinfra.StrPtr("production_bom_version_id"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", cmd.BomVersionID)), postgresinfra.AuditMeta{"product_id": cmd.ProductID, "production_bom_id": cmd.BomID, "production_bom_version_id": cmd.BomVersionID, "production_bom_version_no": versionNo}); err != nil {
		return bomapp.ProductProductionBomBinding{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return bomapp.ProductProductionBomBinding{}, err
	}
	binding, ok, err := productionBomBindingForProduct(ctx, r.pool, r.schema, cmd.ProductID)
	if err != nil {
		return bomapp.ProductProductionBomBinding{}, err
	}
	if !ok {
		return bomapp.ProductProductionBomBinding{}, fmt.Errorf("production BOM binding not found")
	}
	return binding, nil
}

func productionBomSummarySQL(schema, where string) string {
	return fmt.Sprintf(`
		SELECT pb.id,
		       COALESCE(pb.code,''),
		       COALESCE(pb.name,''),
		       COALESCE(pb.output_product_id,0),
		       COALESCE(op.name,''),
		       CASE WHEN COALESCE(pb.output_product_id,0)>0 THEN 'SKU-' || lpad(pb.output_product_id::text,6,'0') ELSE '' END,
		       COALESCE(bga.group_id,0),
		       COALESCE(bg.name,''),
		       COALESCE(bga.group_item_id,0),
		       COALESCE(item.name,''),
		       COALESCE(NULLIF(pb.status,''),'active'),
		       COALESCE(latest.id,0),
		       COALESCE(latest.version_no,''),
		       COALESCE(latest.status,'') AS latest_version_status,
		       COALESCE(latest.yield_rate,0)::float8,
		       COALESCE((
		           SELECT COUNT(*)
		           FROM %[1]s.product_production_bom_bindings b
		           WHERE b.bom_id=pb.id
		       ),0),
		       COALESCE(to_char(pb.updated_at,'YYYY-MM-DD HH24:MI'),'-')
		FROM %[1]s.production_boms pb
		LEFT JOIN %[1]s.products op ON op.id=pb.output_product_id
		LEFT JOIN %[1]s.business_group_assignments bga ON bga.object_id=pb.id AND lower(bga.usage_key)='production_bom' AND lower(bga.object_key)='production_bom'
		LEFT JOIN %[1]s.business_groups bg ON bg.id=bga.group_id
		LEFT JOIN %[1]s.business_group_items item ON item.id=bga.group_item_id
		LEFT JOIN LATERAL (
			SELECT id, version_no, status, yield_rate
			FROM %[1]s.production_bom_versions v
			WHERE v.bom_id=pb.id AND v.status IN ('draft','published')
			ORDER BY CASE WHEN v.status='draft' THEN 0 ELSE 1 END,
			         v.published_at DESC NULLS LAST, v.created_at DESC, v.id DESC
			LIMIT 1
		) latest ON true
		WHERE %s
	`, schema, where)
}

func scanProductionBomSummaries(rows pgx.Rows) ([]bomapp.ProductionBomSummary, error) {
	defer rows.Close()
	out := make([]bomapp.ProductionBomSummary, 0)
	for rows.Next() {
		var row bomapp.ProductionBomSummary
		if err := rows.Scan(
			&row.ID,
			&row.Code,
			&row.Name,
			&row.OutputProductID,
			&row.OutputProductName,
			&row.OutputProductCode,
			&row.GroupID,
			&row.GroupName,
			&row.GroupCategoryID,
			&row.GroupCategoryName,
			&row.Status,
			&row.LatestVersionID,
			&row.LatestVersionNo,
			&row.LatestVersionStatus,
			&row.ExpectedYieldRate,
			&row.ReferenceProductCount,
			&row.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) productionBomSummaryByID(ctx context.Context, id int64) (bomapp.ProductionBomSummary, error) {
	rows, err := r.pool.Query(ctx, productionBomSummarySQL(r.schema, "pb.id=$1"), id)
	if err != nil {
		return bomapp.ProductionBomSummary{}, err
	}
	rowsOut, err := scanProductionBomSummaries(rows)
	if err != nil {
		return bomapp.ProductionBomSummary{}, err
	}
	if len(rowsOut) == 0 {
		return bomapp.ProductionBomSummary{}, pgx.ErrNoRows
	}
	return rowsOut[0], nil
}

func (r Repository) listProductionBomVersions(ctx context.Context, bomID int64) ([]bomapp.ProductionBomVersion, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		WITH latest AS (
			SELECT id
			FROM %s.production_bom_versions
			WHERE bom_id=$1 AND status='published'
			ORDER BY published_at DESC NULLS LAST, id DESC
			LIMIT 1
		)
		SELECT v.id, v.bom_id, COALESCE(v.version_no,''), COALESCE(v.status,'draft'), COALESCE(v.yield_rate,0.8)::float8,
		       COALESCE(v.output_qty,1)::float8,
		       COALESCE(NULLIF(v.output_unit,''),'unit'),
		       COALESCE((SELECT COUNT(*) FROM %s.production_bom_version_items i WHERE i.version_id=v.id),0),
		       COALESCE(v.note,''),
		       COALESCE(v.special_attrs_schema_json::text,'[]'),
		       COALESCE(v.special_attrs_json::text,'{}'),
		       COALESCE(to_char(v.created_at,'YYYY-MM-DD HH24:MI'),'-'),
		       COALESCE(to_char(v.published_at,'YYYY-MM-DD HH24:MI'),''),
		       v.id=COALESCE((SELECT id FROM latest),0)
		FROM %s.production_bom_versions v
		WHERE v.bom_id=$1
		ORDER BY v.created_at DESC, v.id DESC
	`, r.schema, r.schema, r.schema), bomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]bomapp.ProductionBomVersion, 0)
	for rows.Next() {
		var row bomapp.ProductionBomVersion
		if err := rows.Scan(&row.ID, &row.BomID, &row.VersionNo, &row.Status, &row.YieldRate, &row.OutputQty, &row.OutputUnit, &row.ItemCount, &row.Note, &row.SpecialAttrsSchemaJSON, &row.SpecialAttrsJSON, &row.CreatedAt, &row.PublishedAt, &row.IsLatest); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) listProductionBomReferencedProducts(ctx context.Context, bomID int64) ([]bomapp.ProductionBomReferencedProduct, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT b.product_id,
		       COALESCE(p.name,''),
		       ('SKU-' || lpad(b.product_id::text,6,'0')),
		       COALESCE(p.active,true),
		       b.bom_version_id,
		       COALESCE(v.version_no,'')
		FROM %[1]s.product_production_bom_bindings b
		JOIN %[1]s.products p ON p.id=b.product_id
		LEFT JOIN %[1]s.production_bom_versions v ON v.id=b.bom_version_id
		WHERE b.bom_id=$1
		ORDER BY COALESCE(p.active,true) DESC, p.name, b.product_id
	`, r.schema), bomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]bomapp.ProductionBomReferencedProduct, 0)
	for rows.Next() {
		var row bomapp.ProductionBomReferencedProduct
		if err := rows.Scan(&row.ProductID, &row.ProductName, &row.ProductCode, &row.Active, &row.BomVersionID, &row.BomVersionNo); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) listProductionBomUsageByProduct(ctx context.Context, productID int64) ([]bomapp.ProductionBomUsedByBom, error) {
	if productID <= 0 {
		return []bomapp.ProductionBomUsedByBom{}, nil
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		WITH output_usage AS (
			SELECT pb.id AS bom_id,
			       COALESCE(pb.code,'') AS bom_code,
			       COALESCE(pb.name,'') AS bom_name,
			       COALESCE(v.id,0) AS bom_version_id,
			       COALESCE(v.version_no,'') AS bom_version_no,
			       COALESCE(pb.output_product_id,0) AS output_product_id,
			       COALESCE(op.name,'') AS output_product_name,
			       'output' AS relation_type,
			       '' AS consume_unit,
			       0::float8 AS qty_per_unit,
			       0 AS relation_sort,
			       0 AS sort_item_id
			FROM %[1]s.production_boms pb
			JOIN %[1]s.products op ON op.id=pb.output_product_id AND op.active=true
			LEFT JOIN LATERAL (
				SELECT id, version_no
				FROM %[1]s.production_bom_versions v
				WHERE v.bom_id=pb.id AND v.status IN ('draft','published')
				ORDER BY CASE WHEN v.status='draft' THEN 0 ELSE 1 END,
				         v.published_at DESC NULLS LAST,
				         v.created_at DESC,
				         v.id DESC
				LIMIT 1
			) v ON true
			WHERE pb.output_product_id=$1
			  AND COALESCE(NULLIF(pb.status,''),'active')='active'
		),
		current_usage_versions AS (
			SELECT pb.id AS bom_id,
			       v.id AS bom_version_id,
			       COALESCE(v.version_no,'') AS bom_version_no
			FROM %[1]s.production_boms pb
			JOIN LATERAL (
				SELECT id, version_no, status, published_at, created_at
				FROM %[1]s.production_bom_versions v
				WHERE v.bom_id=pb.id AND v.status IN ('draft','published')
				ORDER BY CASE WHEN v.status='draft' THEN 0 ELSE 1 END,
				         v.published_at DESC NULLS LAST,
				         v.created_at DESC,
				         v.id DESC
				LIMIT 1
			) v ON true
			WHERE COALESCE(NULLIF(pb.status,''),'active')='active'
		),
		component_usage AS (
			SELECT DISTINCT ON (pb.id)
			       pb.id AS bom_id,
			       COALESCE(pb.code,'') AS bom_code,
			       COALESCE(pb.name,'') AS bom_name,
			       cv.bom_version_id AS bom_version_id,
			       cv.bom_version_no AS bom_version_no,
			       COALESCE(pb.output_product_id,0) AS output_product_id,
			       COALESCE(op.name,'') AS output_product_name,
			       'component' AS relation_type,
			       COALESCE(i.consume_unit,'') AS consume_unit,
			       COALESCE(i.qty_per_unit,0)::float8 AS qty_per_unit,
			       1 AS relation_sort,
			       i.id AS sort_item_id
			FROM %[1]s.production_boms pb
			JOIN current_usage_versions cv ON cv.bom_id=pb.id
			JOIN %[1]s.production_bom_version_items i ON i.version_id=cv.bom_version_id
			JOIN %[1]s.products cp ON cp.id=i.component_product_id AND cp.active=true
			LEFT JOIN %[1]s.products op ON op.id=pb.output_product_id
			WHERE i.component_type IN ('product','finished_product')
			  AND i.component_product_id=$1
			  AND COALESCE(NULLIF(pb.status,''),'active')='active'
			ORDER BY pb.id,
			         i.id
		),
		usage AS (
			SELECT * FROM output_usage
			UNION ALL
			SELECT * FROM component_usage
		)
		SELECT bom_id,
		       bom_code,
		       bom_name,
		       bom_version_id,
		       bom_version_no,
		       output_product_id,
		       output_product_name,
		       relation_type,
		       consume_unit,
		       qty_per_unit
		FROM usage
		ORDER BY bom_name, relation_sort, bom_version_id DESC, sort_item_id
	`, r.schema), productID)
	if err != nil {
		return nil, err
	}
	return scanProductionBomUsedByBomRows(rows)
}

func (r Repository) listProductionBomComponentUsedByBoms(ctx context.Context, componentProductID int64) ([]bomapp.ProductionBomUsedByBom, error) {
	if componentProductID <= 0 {
		return []bomapp.ProductionBomUsedByBom{}, nil
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		WITH current_component_versions AS (
			SELECT pb.id AS bom_id,
			       v.id AS bom_version_id,
			       COALESCE(v.version_no,'') AS bom_version_no
			FROM %[1]s.production_boms pb
			JOIN LATERAL (
				SELECT id, version_no, status, published_at, created_at
				FROM %[1]s.production_bom_versions v
				WHERE v.bom_id=pb.id AND v.status IN ('draft','published')
				ORDER BY CASE WHEN v.status='draft' THEN 0 ELSE 1 END,
				         v.published_at DESC NULLS LAST,
				         v.created_at DESC,
				         v.id DESC
				LIMIT 1
			) v ON true
			WHERE COALESCE(NULLIF(pb.status,''),'active')='active'
		),
		usage AS (
			SELECT DISTINCT ON (pb.id)
			       pb.id AS bom_id,
			       COALESCE(pb.code,'') AS bom_code,
			       COALESCE(pb.name,'') AS bom_name,
			       cv.bom_version_id AS bom_version_id,
			       cv.bom_version_no AS bom_version_no,
			       COALESCE(pb.output_product_id,0) AS output_product_id,
			       COALESCE(op.name,'') AS output_product_name,
			       'component' AS relation_type,
			       COALESCE(i.consume_unit,'') AS consume_unit,
			       COALESCE(i.qty_per_unit,0)::float8 AS qty_per_unit,
			       i.id AS sort_item_id
			FROM %[1]s.production_boms pb
			JOIN current_component_versions cv ON cv.bom_id=pb.id
			JOIN %[1]s.production_bom_version_items i ON i.version_id=cv.bom_version_id
			JOIN %[1]s.products cp ON cp.id=i.component_product_id AND cp.active=true
			LEFT JOIN %[1]s.products op ON op.id=pb.output_product_id
			WHERE i.component_type IN ('product','finished_product')
			  AND i.component_product_id=$1
			  AND COALESCE(NULLIF(pb.status,''),'active')='active'
			  AND COALESCE(pb.output_product_id,0)<>$1
			ORDER BY pb.id,
			         i.id
		)
		SELECT bom_id,
		       bom_code,
		       bom_name,
		       bom_version_id,
		       bom_version_no,
		       output_product_id,
		       output_product_name,
		       relation_type,
		       consume_unit,
		       qty_per_unit
		FROM usage
		ORDER BY bom_name, bom_version_id DESC, sort_item_id
	`, r.schema), componentProductID)
	if err != nil {
		return nil, err
	}
	return scanProductionBomUsedByBomRows(rows)
}

func scanProductionBomUsedByBomRows(rows pgx.Rows) ([]bomapp.ProductionBomUsedByBom, error) {
	defer rows.Close()
	out := make([]bomapp.ProductionBomUsedByBom, 0)
	for rows.Next() {
		var row bomapp.ProductionBomUsedByBom
		if err := rows.Scan(&row.BomID, &row.BomCode, &row.BomName, &row.BomVersionID, &row.BomVersionNo, &row.OutputProductID, &row.OutputProductName, &row.RelationType, &row.ConsumeUnit, &row.QtyPerUnit); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) productionBomVersionByID(ctx context.Context, id int64) (bomapp.ProductionBomVersion, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT v.id, v.bom_id, COALESCE(v.version_no,''), COALESCE(v.status,'draft'), COALESCE(v.yield_rate,0.8)::float8,
		       COALESCE(v.output_qty,1)::float8,
		       COALESCE(NULLIF(v.output_unit,''),'unit'),
		       COALESCE((SELECT COUNT(*) FROM %s.production_bom_version_items i WHERE i.version_id=v.id),0),
		       COALESCE(v.note,''),
		       COALESCE(v.special_attrs_schema_json::text,'[]'),
		       COALESCE(v.special_attrs_json::text,'{}'),
		       COALESCE(to_char(v.created_at,'YYYY-MM-DD HH24:MI'),'-'),
		       COALESCE(to_char(v.published_at,'YYYY-MM-DD HH24:MI'),''),
		       v.id=COALESCE((
		           SELECT latest.id
		           FROM %s.production_bom_versions latest
		           WHERE latest.bom_id=v.bom_id AND latest.status='published'
		           ORDER BY latest.published_at DESC NULLS LAST, latest.id DESC
		           LIMIT 1
		       ),0)
		FROM %s.production_bom_versions v
		WHERE v.id=$1
	`, r.schema, r.schema, r.schema), id)
	if err != nil {
		return bomapp.ProductionBomVersion{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		return bomapp.ProductionBomVersion{}, pgx.ErrNoRows
	}
	var row bomapp.ProductionBomVersion
	if err := rows.Scan(&row.ID, &row.BomID, &row.VersionNo, &row.Status, &row.YieldRate, &row.OutputQty, &row.OutputUnit, &row.ItemCount, &row.Note, &row.SpecialAttrsSchemaJSON, &row.SpecialAttrsJSON, &row.CreatedAt, &row.PublishedAt, &row.IsLatest); err != nil {
		return bomapp.ProductionBomVersion{}, err
	}
	return row, rows.Err()
}

func ensureProductionBomGroupTx(ctx context.Context, tx pgx.Tx, schema string, groupID int64) (int64, error) {
	if groupID > 0 {
		return groupID, nil
	}
	return 0, nil
}

func validateProductionBomGroupCategoryTx(ctx context.Context, tx pgx.Tx, schema string, groupID int64, categoryID int64) (int64, error) {
	if groupID <= 0 || categoryID <= 0 {
		return 0, nil
	}
	var exists bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT EXISTS(
			SELECT 1 FROM %s.production_bom_group_categories
			WHERE id=$1 AND group_id=$2
		)
	`, schema), categoryID, groupID).Scan(&exists); err != nil {
		return 0, err
	}
	if !exists {
		return 0, fmt.Errorf("group_category_id does not belong to group_id")
	}
	return categoryID, nil
}

func repairLegacyProductionBomBindings(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	// PR-403: repair legacy product BOM rows that still have items but no production BOM binding.
	q := fmt.Sprintf(`
WITH missing_legacy_bindings AS (
	SELECT p.id AS product_id,
	       COALESCE(NULLIF(p.name,''), '商品 ' || p.id::text) AS product_name,
	       COALESCE(NULLIF(pb.status,''), 'active') AS status,
	       COALESCE(NULLIF(pb.yield_rate,0), 0.8) AS yield_rate,
	       COALESCE(pb.updated_at, now()) AS updated_at
	FROM %[1]s.products p
	LEFT JOIN %[1]s.product_bom pb ON pb.product_id=p.id
	LEFT JOIN %[1]s.product_production_bom_bindings existing_binding ON existing_binding.product_id=p.id
	WHERE COALESCE(p.active,true)=true
	  AND existing_binding.product_id IS NULL
	  AND (
	    pb.product_id IS NOT NULL
	    OR EXISTS (SELECT 1 FROM %[1]s.product_bom_items bi WHERE bi.product_id=p.id)
	    OR EXISTS (SELECT 1 FROM %[1]s.bom_versions bv WHERE bv.product_id=p.id)
	  )
),
inserted_boms AS (
	INSERT INTO %[1]s.production_boms(code, name, output_product_id, group_id, status, legacy_product_id, created_by, updated_by)
	SELECT 'BOM-' || LPAD(product_id::text, 6, '0'),
	       product_name || ' 生产 BOM',
	       product_id,
	       0,
	       CASE WHEN status='inactive' THEN 'inactive' ELSE 'active' END,
	       product_id,
	       'system-pr403-legacy-binding-repair',
	       'system-pr403-legacy-binding-repair'
	FROM missing_legacy_bindings
	ON CONFLICT DO NOTHING
	RETURNING id
),
version_source AS (
	SELECT pbom.id AS bom_id,
	       mlb.product_id,
	       mlb.yield_rate,
	       mlb.updated_at
	FROM missing_legacy_bindings mlb
	JOIN %[1]s.production_boms pbom ON pbom.legacy_product_id=mlb.product_id
	WHERE NOT EXISTS (
		SELECT 1 FROM %[1]s.production_bom_versions existing_version WHERE existing_version.bom_id=pbom.id
	)
),
inserted_versions AS (
	INSERT INTO %[1]s.production_bom_versions(bom_id, version_no, status, yield_rate, output_qty, output_unit, note, legacy_product_id, legacy_bom_version_id, created_at, published_at, created_by, published_by)
	SELECT bom_id, 'V001', 'published', yield_rate, 1, 'kg', '旧 BOM 绑定修复', product_id, 0, updated_at, updated_at, 'system-pr403-legacy-binding-repair', 'system-pr403-legacy-binding-repair'
	FROM version_source
	ON CONFLICT DO NOTHING
	RETURNING id
),
item_source AS (
	SELECT v.id AS version_id,
	       i.material_id,
	       COALESCE(NULLIF(i.component_type,''),'material') AS component_type,
	       COALESCE(i.component_product_id,0) AS component_product_id,
	       COALESCE(i.component_spec_g,0) AS component_spec_g,
	       COALESCE(NULLIF(i.consume_unit,''),'ratio_pct') AS consume_unit,
	       COALESCE(i.qty_per_unit,0) AS qty_per_unit,
	       COALESCE(i.ratio_pct,0) AS ratio_pct,
	       COALESCE(i.unit_cost_snapshot,0) AS unit_cost_snapshot
	FROM %[1]s.production_bom_versions v
	JOIN missing_legacy_bindings mlb ON mlb.product_id=v.legacy_product_id
	JOIN %[1]s.product_bom_items i ON i.product_id=mlb.product_id
	WHERE NOT EXISTS (
		SELECT 1 FROM %[1]s.production_bom_version_items existing_item WHERE existing_item.version_id=v.id
	)
),
inserted_items AS (
	INSERT INTO %[1]s.production_bom_version_items(version_id, material_id, component_type, component_product_id, component_spec_g, consume_unit, qty_per_unit, ratio_pct, unit_cost_snapshot)
	SELECT version_id, material_id, component_type, component_product_id, component_spec_g, consume_unit, qty_per_unit, ratio_pct, unit_cost_snapshot
	FROM item_source
	RETURNING id
),
binding_rows AS (
	SELECT mlb.product_id, pbom.id AS bom_id, v.id AS bom_version_id
	FROM missing_legacy_bindings mlb
	JOIN %[1]s.production_boms pbom ON pbom.legacy_product_id=mlb.product_id
	JOIN LATERAL (
		SELECT id
		FROM %[1]s.production_bom_versions pv
		WHERE pv.bom_id=pbom.id AND pv.status='published'
		ORDER BY pv.published_at DESC NULLS LAST, pv.id DESC
		LIMIT 1
	) v ON true
)
INSERT INTO %[1]s.product_production_bom_bindings(product_id, bom_id, bom_version_id, bound_by)
SELECT product_id, bom_id, bom_version_id, 'system-pr403-legacy-binding-repair'
FROM binding_rows
WHERE bom_version_id > 0
ON CONFLICT DO NOTHING;
`, schema)
	_, err := pool.Exec(ctx, q)
	return err
}
