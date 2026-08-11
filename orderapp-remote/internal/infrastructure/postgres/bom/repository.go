package bom

import (
	"context"
	"fmt"
	"strings"
	"time"

	bomapp "orderapp/internal/application/bom"
	bomdomain "orderapp/internal/domain/bom"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	postgresbomgraph "orderapp/internal/infrastructure/postgres/bomgraph"

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
	return listProductionBomProductOptions(ctx, r.pool, r.schema, nil)
}

func listProductionBomProductOptions(ctx context.Context, q bomQueryer, schema string, productIDs []int64) ([]bomapp.Option, error) {
	rows, err := q.Query(ctx, fmt.Sprintf(`SELECT p.id, ('SKU-' || lpad(p.id::text,6,'0')), p.name, COALESCE(p.customer_id,0),
		CASE WHEN COALESCE(p.parent_product_id,0)>0 THEN parent_units.parent_product_inventory_unit ELSE COALESCE(NULLIF(p.unit_rule_override_json->>'inventory_unit',''), NULLIF(product_direct_unit_template.inventory_unit,''), NULLIF(product_config.inventory_unit,''), NULLIF(category_config.inventory_unit,''), NULLIF(product_unit_template.inventory_unit,''), NULLIF(category_unit_template.inventory_unit,''), 'kg') END AS inventory_unit,
		CASE WHEN COALESCE(p.parent_product_id,0)>0 THEN parent_units.parent_product_inventory_unit_explicit ELSE COALESCE(NULLIF(p.unit_rule_override_json->>'inventory_unit',''), NULLIF(product_direct_unit_template.inventory_unit,'')) IS NOT NULL END AS inventory_unit_explicit,
		COALESCE(p.roast_level,''), COALESCE(NULLIF(p.product_kind,''),'roasted_bean'), COALESCE(p.drip_bag_grams,10)::float8, COALESCE(p.drip_box_bag_count,10),
		COALESCE((
			SELECT COUNT(*)
			FROM %[1]s.order_items oi
			JOIN %[1]s.orders o ON o.id=oi.order_id
			WHERE oi.product_id=p.id AND COALESCE(o.is_void,false)=false
		),0) AS order_usage_count
		FROM %[1]s.products p
		LEFT JOIN %[1]s.products parent_product ON parent_product.id=CASE WHEN COALESCE(p.parent_product_id,0)>0 THEN p.parent_product_id ELSE p.id END
		LEFT JOIN %[1]s.product_unit_templates parent_product_direct_unit_template ON parent_product_direct_unit_template.id=COALESCE(parent_product.unit_template_id,0) AND parent_product_direct_unit_template.active=true AND parent_product_direct_unit_template.deleted_at IS NULL
		LEFT JOIN %[1]s.product_config_templates parent_product_config ON parent_product_config.id=COALESCE(parent_product.product_config_template_id,0) AND parent_product_config.deleted_at IS NULL
		LEFT JOIN %[1]s.product_unit_templates parent_product_unit_template ON parent_product_unit_template.id=COALESCE(parent_product_config.unit_template_id,0) AND parent_product_unit_template.deleted_at IS NULL
		LEFT JOIN %[1]s.product_categories parent_category_config ON parent_category_config.id=COALESCE(parent_product.product_category_id,0)
		LEFT JOIN %[1]s.product_unit_templates parent_category_unit_template ON parent_category_unit_template.id=COALESCE(parent_category_config.unit_template_id,0) AND parent_category_unit_template.deleted_at IS NULL
		LEFT JOIN LATERAL (
			SELECT
				COALESCE(NULLIF(parent_product.unit_rule_override_json->>'inventory_unit',''), NULLIF(parent_product_direct_unit_template.inventory_unit,''), NULLIF(parent_product_config.inventory_unit,''), NULLIF(parent_category_config.inventory_unit,''), NULLIF(parent_product_unit_template.inventory_unit,''), NULLIF(parent_category_unit_template.inventory_unit,''), 'kg') AS parent_product_inventory_unit,
				COALESCE(NULLIF(parent_product.unit_rule_override_json->>'inventory_unit',''), NULLIF(parent_product_direct_unit_template.inventory_unit,'')) IS NOT NULL AS parent_product_inventory_unit_explicit
		) parent_units ON true
		LEFT JOIN %[1]s.product_unit_templates product_direct_unit_template ON product_direct_unit_template.id=COALESCE(p.unit_template_id,0) AND product_direct_unit_template.active=true AND product_direct_unit_template.deleted_at IS NULL
		LEFT JOIN %[1]s.product_config_templates product_config ON product_config.id=COALESCE(p.product_config_template_id,0) AND product_config.deleted_at IS NULL
		LEFT JOIN %[1]s.product_unit_templates product_unit_template ON product_unit_template.id=COALESCE(product_config.unit_template_id,0) AND product_unit_template.deleted_at IS NULL
		LEFT JOIN %[1]s.product_categories category_config ON category_config.id=COALESCE(p.product_category_id,0)
		LEFT JOIN %[1]s.product_unit_templates category_unit_template ON category_unit_template.id=COALESCE(category_config.unit_template_id,0) AND category_unit_template.deleted_at IS NULL
		WHERE p.active=true
		  AND (NOT COALESCE(p.auto_derived_sku,false) OR COALESCE(NULLIF(p.derived_spec_status,''),'active')<>'template_removed')
		  AND ($1::bigint[] IS NULL OR p.id=ANY($1))
		ORDER BY p.name`, schema), productIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]bomapp.Option, 0)
	for rows.Next() {
		var opt bomapp.Option
		if err := rows.Scan(&opt.ID, &opt.ProductCode, &opt.Name, &opt.CustomerID, &opt.InventoryUnit, &opt.InventoryUnitExplicit, &opt.RoastLevel, &opt.ProductKind, &opt.DripBagGrams, &opt.DripBoxBagCount, &opt.OrderUsageCount); err != nil {
			return nil, err
		}
		out = append(out, opt)
	}
	return out, rows.Err()
}

func (r Repository) Materials(ctx context.Context) ([]bomapp.Option, error) {
	return listProductionBomMaterialOptions(ctx, r.pool, r.schema, nil)
}

func listProductionBomMaterialOptions(ctx context.Context, q bomQueryer, schema string, materialIDs []int64) ([]bomapp.Option, error) {
	rows, err := q.Query(ctx, "SELECT id, name, COALESCE(NULLIF(unit,''),'kg') FROM "+schema+".materials WHERE ($1::bigint[] IS NULL OR id=ANY($1)) ORDER BY name", materialIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]bomapp.Option, 0)
	for rows.Next() {
		var opt bomapp.Option
		if err := rows.Scan(&opt.ID, &opt.Name, &opt.InventoryUnit); err != nil {
			return nil, err
		}
		out = append(out, opt)
	}
	return out, rows.Err()
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
	yieldRate := 1.0
	q := "INSERT INTO " + r.schema + ".product_bom(product_id,yield_rate,status,updated_at) VALUES($1,$2,'active',now()) ON CONFLICT (product_id) DO UPDATE SET yield_rate=excluded.yield_rate, status='active', updated_at=now()"
	_, err := r.pool.Exec(ctx, q, cmd.ProductID, yieldRate)
	if err == nil {
		action := "sync_yield"
		if cmd.ExpectedLossRate != nil {
			action = "save_expected_loss_rate"
		}
		postgresinfra.AuditInsert(ctx, r.pool, r.schema, cmd.Actor, "product_bom", &cmd.ProductID, action, postgresinfra.StrPtr("expected_loss_rate"), nil, postgresinfra.StrPtr("0.0000"), postgresinfra.AuditMeta{"product_id": cmd.ProductID, "status": "active", "yield_rate": yieldRate, "expected_loss_rate": 0})
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
	var active bool
	if err := tx.QueryRow(ctx, "SELECT active FROM "+r.schema+".products WHERE id=$1", cmd.ProductID).Scan(&active); err != nil {
		return fmt.Errorf("product not found")
	}
	if active {
		return fmt.Errorf("商品仍为启用状态，需先停用商品后再失效 BOM")
	}
	yieldRate := 1.0
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
	yieldRate := 1.0
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
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT product_id FROM %s.bom_versions WHERE id=$1 FOR UPDATE`, r.schema), cmd.VersionID).Scan(&productID); err != nil {
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
	`, r.schema), productID, 1.0); err != nil {
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
	MaterialLossRate     float64
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
		       0::float8 AS material_loss_rate,
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
		if err := rows.Scan(&row.ID, &row.MaterialID, &row.MaterialName, &row.ComponentType, &row.ComponentProductID, &row.ComponentProductName, &row.ComponentSpecG, &row.ConsumeUnit, &row.QtyPerUnit, &row.RatioPct, &row.MaterialLossRate, &row.UnitCostSnapshot); err != nil {
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
		       0::float8 AS material_loss_rate,
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
		if err := rows.Scan(&row.ID, &row.MaterialID, &row.MaterialName, &row.ComponentType, &row.ComponentProductID, &row.ComponentProductName, &row.ComponentSpecG, &row.ConsumeUnit, &row.QtyPerUnit, &row.RatioPct, &row.MaterialLossRate, &row.UnitCostSnapshot); err != nil {
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
		       COALESCE(bi.material_loss_rate,0)::float8,
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
		if err := rows.Scan(&row.ID, &row.MaterialID, &row.MaterialName, &row.ComponentType, &row.ComponentProductID, &row.ComponentProductName, &row.ComponentSpecG, &row.ConsumeUnit, &row.QtyPerUnit, &row.RatioPct, &row.MaterialLossRate, &row.UnitCostSnapshot); err != nil {
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
	yieldRate := 1.0
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
			MaterialLossRate:     row.MaterialLossRate,
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

func resolveBomYieldRate(_ string, _ float64) float64 {
	return 1
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
	return r.ListProductionBomsFiltered(ctx, bomapp.ProductionBomFilter{})
}

func (r Repository) ListProductionBomsFiltered(ctx context.Context, filter bomapp.ProductionBomFilter) ([]bomapp.ProductionBomSummary, error) {
	if err := repairLegacyProductionBomBindings(ctx, r.pool, r.schema); err != nil {
		return nil, err
	}
	where := []string{"1=1"}
	args := []any{}
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	outputType := strings.ToLower(strings.TrimSpace(filter.OutputType))
	if outputType != "" {
		where = append(where, "pb.output_type="+addArg(outputType))
	}
	if filter.OutputID > 0 {
		where = append(where, "CASE WHEN pb.output_type='material' THEN pb.output_material_id ELSE pb.output_product_id END="+addArg(filter.OutputID))
	}
	componentType := strings.ToLower(strings.TrimSpace(filter.ComponentType))
	if componentType == "finished_product" {
		componentType = "product"
	}
	if componentType != "" || filter.ComponentID > 0 {
		componentWhere := []string{"i.version_id IN (SELECT id FROM " + r.schema + ".production_bom_versions WHERE bom_id=pb.id AND status IN ('draft','published'))"}
		if componentType != "" {
			componentWhere = append(componentWhere, "CASE WHEN i.component_type IN ('product','finished_product') THEN 'product' ELSE 'material' END="+addArg(componentType))
		}
		if filter.ComponentID > 0 {
			componentWhere = append(componentWhere, "CASE WHEN i.component_type IN ('product','finished_product') THEN i.component_product_id ELSE i.material_id END="+addArg(filter.ComponentID))
		}
		where = append(where, "EXISTS(SELECT 1 FROM "+r.schema+".production_bom_version_items i WHERE "+strings.Join(componentWhere, " AND ")+")")
	}
	rows, err := r.pool.Query(ctx, productionBomSummarySQL(r.schema, strings.Join(where, " AND "))+" ORDER BY COALESCE(bga.sort_order,100), pb.name, pb.id", args...)
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
		summary.ProcessRouteID = selectedVersion.ProcessRouteID
		summary.ProcessRouteName = selectedVersion.ProcessRouteName
		summary.IsLatestUsable = selectedVersion.IsLatestUsable
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
		INSERT INTO %s.production_boms(code, name, output_type, output_product_id, output_material_id, status, created_by, updated_by)
		VALUES($1,$2,$3,$4,$5,'active',$6,$6)
		RETURNING id
	`, r.schema), tempCode, strings.TrimSpace(cmd.Name), cmd.OutputType, cmd.OutputProductID, cmd.OutputMaterialID, strings.TrimSpace(cmd.Actor)).Scan(&bomID); err != nil {
		return bomapp.ProductionBomSummary{}, err
	}
	code := fmt.Sprintf("BOM-%06d", bomID)
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_boms SET code=$1 WHERE id=$2`, r.schema), code, bomID); err != nil {
		return bomapp.ProductionBomSummary{}, err
	}
	yieldRate := 1.0
	var versionID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_versions(bom_id, version_no, status, yield_rate, output_qty, output_unit, note, created_at, created_by)
		VALUES($1,'V001','draft',$2,$3,$4,'初始版本',now(),$5)
		RETURNING id
	`, r.schema), bomID, yieldRate, cmd.OutputQty, strings.TrimSpace(cmd.OutputUnit), strings.TrimSpace(cmd.Actor)).Scan(&versionID); err != nil {
		return bomapp.ProductionBomSummary{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "production_bom", &bomID, "create", postgresinfra.StrPtr("code"), nil, postgresinfra.StrPtr(code), postgresinfra.AuditMeta{"bom_id": bomID, "bom_version_id": versionID, "name": strings.TrimSpace(cmd.Name), "output_type": cmd.OutputType, "output_id": cmd.OutputID, "output_product_id": cmd.OutputProductID, "output_material_id": cmd.OutputMaterialID, "output_qty": cmd.OutputQty, "output_unit": strings.TrimSpace(cmd.OutputUnit), "group_id": groupID, "group_category_id": groupCategoryID}); err != nil {
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
	if cmd.UpdateOutputBinding {
		var currentOutputType string
		var currentOutputProductID int64
		var currentOutputMaterialID int64
		var hasPublished bool
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COALESCE(NULLIF(output_type,''),'product'),
			       COALESCE(output_product_id,0),
			       COALESCE(output_material_id,0),
			       EXISTS(SELECT 1 FROM %s.production_bom_versions v WHERE v.bom_id=production_boms.id AND v.status='published')
			FROM %s.production_boms
			WHERE id=$1
			FOR UPDATE
		`, r.schema, r.schema), cmd.ID).Scan(&currentOutputType, &currentOutputProductID, &currentOutputMaterialID, &hasPublished); err != nil {
			return bomapp.ProductionBomSummary{}, err
		}
		identityChanged := currentOutputType != cmd.OutputType || currentOutputProductID != cmd.OutputProductID || currentOutputMaterialID != cmd.OutputMaterialID
		if identityChanged && hasPublished {
			return bomapp.ProductionBomSummary{}, fmt.Errorf("published production BOM output identity is immutable")
		}
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.production_boms
		SET name=COALESCE(NULLIF($2,''), name),
		    output_type=CASE WHEN $3 THEN $4 ELSE output_type END,
		    output_product_id=CASE WHEN $3 THEN $5 ELSE output_product_id END,
		    output_material_id=CASE WHEN $3 THEN $6 ELSE output_material_id END,
		    status=$7, updated_at=now(), updated_by=$8
		WHERE id=$1
	`, r.schema), cmd.ID, strings.TrimSpace(cmd.Name), cmd.UpdateOutputBinding, cmd.OutputType, cmd.OutputProductID, cmd.OutputMaterialID, status, strings.TrimSpace(cmd.Actor)); err != nil {
		return bomapp.ProductionBomSummary{}, err
	}
	outputUnit := strings.TrimSpace(cmd.OutputUnit)
	if outputUnit != "" {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.production_bom_versions
			SET output_unit=$2
			WHERE bom_id=$1 AND status='draft'
		`, r.schema), cmd.ID, outputUnit); err != nil {
			return bomapp.ProductionBomSummary{}, err
		}
	}
	auditMeta := postgresinfra.AuditMeta{"bom_id": cmd.ID, "name": strings.TrimSpace(cmd.Name), "status": status, "output_type": cmd.OutputType, "output_id": cmd.OutputID, "output_product_id": cmd.OutputProductID, "output_material_id": cmd.OutputMaterialID, "output_unit": outputUnit}
	if cmd.UpdateGroupAssignment {
		auditMeta["group_id"] = groupID
		auditMeta["group_category_id"] = groupCategoryID
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "production_bom", &cmd.ID, "update", postgresinfra.StrPtr("status"), nil, postgresinfra.StrPtr(status), auditMeta); err != nil {
		return bomapp.ProductionBomSummary{}, err
	}
	if cmd.UpdateGroupAssignment {
		if err := saveBusinessGroupAssignmentForProductionBomTx(ctx, tx, r.schema, strings.TrimSpace(cmd.Actor), cmd.ID, groupID, groupCategoryID); err != nil {
			return bomapp.ProductionBomSummary{}, err
		}
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
	var sourceOutputType string
	var sourceOutputProductID int64
	var sourceOutputMaterialID int64
	var sourceVersionID int64
	var sourceOutputQty float64
	var sourceOutputUnit string
	var sourceMaterialLossRate float64
	var sourceSpecialAttrsSchemaJSON string
	var sourceSpecialAttrsJSON string
	var sourceProcessRouteID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT pb.name, COALESCE(NULLIF(pb.output_type,''),'product'), COALESCE(pb.output_product_id,0), COALESCE(pb.output_material_id,0), v.id, COALESCE(v.output_qty,1)::float8, COALESCE(NULLIF(v.output_unit,''),'unit'), COALESCE(v.material_loss_rate,0)::float8, COALESCE(v.special_attrs_schema_json::text,'[]'), COALESCE(v.special_attrs_json::text,'{}'), COALESCE(v.process_route_id,0)
		FROM %s.production_boms pb
		JOIN LATERAL (
			SELECT id, output_qty, output_unit, material_loss_rate, special_attrs_schema_json, special_attrs_json, process_route_id
			FROM %s.production_bom_versions
			WHERE bom_id=pb.id AND status IN ('draft','published')
			ORDER BY CASE WHEN status='draft' THEN 0 ELSE 1 END, published_at DESC NULLS LAST, created_at DESC, id DESC
			LIMIT 1
		) v ON true
		WHERE pb.id=$1
	`, r.schema, r.schema), cmd.ID).Scan(&sourceName, &sourceOutputType, &sourceOutputProductID, &sourceOutputMaterialID, &sourceVersionID, &sourceOutputQty, &sourceOutputUnit, &sourceMaterialLossRate, &sourceSpecialAttrsSchemaJSON, &sourceSpecialAttrsJSON, &sourceProcessRouteID); err != nil {
		return bomapp.ProductionBomSummary{}, fmt.Errorf("source production BOM not found")
	}
	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		name = bomapp.NormalizeProductionBomName(sourceName)
		if name == "" {
			name = "未命名 BOM"
		}
		name += " 副本"
	}
	tempCode := fmt.Sprintf("PENDING-%d", time.Now().UnixNano())
	var newBomID int64
	outputType := sourceOutputType
	outputProductID := sourceOutputProductID
	outputMaterialID := sourceOutputMaterialID
	if cmd.OutputType != "" {
		outputType = cmd.OutputType
		outputProductID = cmd.OutputProductID
		outputMaterialID = cmd.OutputMaterialID
	}
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_boms(code, name, output_type, output_product_id, output_material_id, status, source_bom_id, source_bom_version_id, created_by, updated_by)
		VALUES($1,$2,$3,$4,$5,'active',$6,$7,$8,$8)
		RETURNING id
	`, r.schema), tempCode, name, outputType, outputProductID, outputMaterialID, cmd.ID, sourceVersionID, strings.TrimSpace(cmd.Actor)).Scan(&newBomID); err != nil {
		return bomapp.ProductionBomSummary{}, err
	}
	code := fmt.Sprintf("BOM-%06d", newBomID)
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_boms SET code=$1 WHERE id=$2`, r.schema), code, newBomID); err != nil {
		return bomapp.ProductionBomSummary{}, err
	}
	var newVersionID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_versions(bom_id, version_no, status, yield_rate, output_qty, output_unit, material_loss_rate, note, special_attrs_schema_json, special_attrs_json, process_route_id, created_at, created_by)
		VALUES($1,'V001','draft',$2,$3,$4,$5,'复制来源 BOM',$6::jsonb,$7::jsonb,$8,now(),$9)
		RETURNING id
	`, r.schema), newBomID, 1.0, sourceOutputQty, sourceOutputUnit, sourceMaterialLossRate, sourceSpecialAttrsSchemaJSON, sourceSpecialAttrsJSON, sourceProcessRouteID, strings.TrimSpace(cmd.Actor)).Scan(&newVersionID); err != nil {
		return bomapp.ProductionBomSummary{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_version_items(version_id, material_id, component_type, component_product_id, component_spec_g, consume_unit, qty_per_unit, ratio_pct, material_loss_rate, unit_cost_snapshot)
		SELECT $1, material_id, component_type, component_product_id, component_spec_g, consume_unit, qty_per_unit, ratio_pct, material_loss_rate, unit_cost_snapshot
		FROM %s.production_bom_version_items
		WHERE version_id=$2
		ORDER BY id
	`, r.schema, r.schema), newVersionID, sourceVersionID); err != nil {
		return bomapp.ProductionBomSummary{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "production_bom", &newBomID, "copy_production_bom", postgresinfra.StrPtr("source_bom_id"), postgresinfra.StrPtr(fmt.Sprintf("%d", cmd.ID)), postgresinfra.StrPtr(fmt.Sprintf("%d", newBomID)), postgresinfra.AuditMeta{"source_bom_id": cmd.ID, "source_bom_version_id": sourceVersionID, "target_bom_id": newBomID, "target_bom_version_id": newVersionID, "output_type": outputType, "output_product_id": outputProductID, "output_material_id": outputMaterialID, "group_id": groupID, "group_category_id": groupCategoryID}); err != nil {
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
	yieldRate := 1.0
	var outputQty float64
	var outputUnit string
	var materialLossRate float64
	var specialAttrsSchemaJSON string
	var specialAttrsJSON string
	var sourceProcessRouteID int64
	if cmd.SourceVersionID > 0 {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT id, COALESCE(output_qty,1)::float8, COALESCE(NULLIF(output_unit,''),'unit'), COALESCE(material_loss_rate,0)::float8, COALESCE(special_attrs_schema_json::text,'[]'), COALESCE(special_attrs_json::text,'{}'), COALESCE(process_route_id,0)
			FROM %s.production_bom_versions
			WHERE bom_id=$1 AND id=$2 AND status IN ('published','draft')
		`, r.schema), cmd.BomID, cmd.SourceVersionID).Scan(&sourceVersionID, &outputQty, &outputUnit, &materialLossRate, &specialAttrsSchemaJSON, &specialAttrsJSON, &sourceProcessRouteID); err != nil {
			return bomapp.ProductionBomVersion{}, fmt.Errorf("source production BOM version not found")
		}
	} else if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT id, COALESCE(output_qty,1)::float8, COALESCE(NULLIF(output_unit,''),'unit'), COALESCE(material_loss_rate,0)::float8, COALESCE(special_attrs_schema_json::text,'[]'), COALESCE(special_attrs_json::text,'{}'), COALESCE(process_route_id,0)
			FROM %s.production_bom_versions
			WHERE bom_id=$1 AND status='published'
			ORDER BY published_at DESC NULLS LAST, id DESC
			LIMIT 1
		`, r.schema), cmd.BomID).Scan(&sourceVersionID, &outputQty, &outputUnit, &materialLossRate, &specialAttrsSchemaJSON, &specialAttrsJSON, &sourceProcessRouteID); err != nil {
		return bomapp.ProductionBomVersion{}, fmt.Errorf("published production BOM version not found")
	}
	var next int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(COUNT(*),0)+1 FROM %s.production_bom_versions WHERE bom_id=$1`, r.schema), cmd.BomID).Scan(&next); err != nil {
		return bomapp.ProductionBomVersion{}, err
	}
	versionNo := fmt.Sprintf("V%03d", next)
	var versionID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_versions(bom_id, version_no, status, yield_rate, output_qty, output_unit, material_loss_rate, note, special_attrs_schema_json, special_attrs_json, process_route_id, created_at, created_by)
		VALUES($1,$2,'draft',$3,$4,$5,$6,$7,$8::jsonb,$9::jsonb,$10,now(),$11)
		RETURNING id
	`, r.schema), cmd.BomID, versionNo, yieldRate, outputQty, outputUnit, materialLossRate, strings.TrimSpace(cmd.Note), specialAttrsSchemaJSON, specialAttrsJSON, sourceProcessRouteID, strings.TrimSpace(cmd.Actor)).Scan(&versionID); err != nil {
		return bomapp.ProductionBomVersion{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_version_items(version_id, material_id, component_type, component_product_id, component_spec_g, consume_unit, qty_per_unit, ratio_pct, material_loss_rate, unit_cost_snapshot)
		SELECT $1, material_id, component_type, component_product_id, component_spec_g, consume_unit, qty_per_unit, ratio_pct, material_loss_rate, unit_cost_snapshot
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
	yieldRate := 1.0
	var outputQty float64
	var outputUnit string
	var materialLossRate float64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT status, COALESCE(output_qty,1)::float8, COALESCE(NULLIF(output_unit,''),'unit'), COALESCE(material_loss_rate,0)::float8 FROM %s.production_bom_versions WHERE id=$1 FOR UPDATE`, r.schema), cmd.VersionID).Scan(&status, &outputQty, &outputUnit, &materialLossRate); err != nil {
		return bomapp.ProductionBomVersion{}, fmt.Errorf("production BOM version not found")
	}
	if status != "draft" {
		return bomapp.ProductionBomVersion{}, fmt.Errorf("published production BOM version is read-only")
	}
	if cmd.MaterialLossRate != nil {
		materialLossRate = *cmd.MaterialLossRate
		if materialLossRate < 0 || materialLossRate >= 1 {
			return bomapp.ProductionBomVersion{}, fmt.Errorf("material_loss_rate must be >= 0 and < 1")
		}
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
		    material_loss_rate=$5,
		    process_route_id=$6,
		    special_attrs_schema_json=CASE WHEN $7<>'' THEN $7::jsonb ELSE special_attrs_schema_json END,
		    special_attrs_json=CASE WHEN $8<>'' THEN $8::jsonb ELSE special_attrs_json END
		WHERE id=$1
	`, r.schema), cmd.VersionID, yieldRate, outputQty, outputUnit, materialLossRate, cmd.ProcessRouteID, cmd.SpecialAttrsSchemaJSON, cmd.SpecialAttrsJSON); err != nil {
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
			itemMaterialLossRate := 0.0
			if materialLossRate > 0 && componentType == "material" && item.ConsumeUnit == "ratio_pct" {
				itemMaterialLossRate = materialLossRate
			}
			if _, err := tx.Exec(ctx, fmt.Sprintf(`
				INSERT INTO %s.production_bom_version_items(version_id, material_id, component_type, component_product_id, component_spec_g, consume_unit, qty_per_unit, ratio_pct, material_loss_rate, unit_cost_snapshot)
				VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,COALESCE((SELECT purchase_price FROM %s.materials WHERE id=$2),0))
			`, r.schema, r.schema), cmd.VersionID, item.MaterialID, componentType, item.ComponentProductID, item.ComponentSpecG, item.ConsumeUnit, item.QtyPerUnit, item.RatioPct, itemMaterialLossRate); err != nil {
				return bomapp.ProductionBomVersion{}, err
			}
		}
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "production_bom_version", &cmd.VersionID, "update_draft", postgresinfra.StrPtr("version_id"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", cmd.VersionID)), postgresinfra.AuditMeta{"version_id": cmd.VersionID, "item_count": len(cmd.Items), "material_loss_rate": materialLossRate}); err != nil {
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

func (r Repository) validateProductionBomVersionItemInventoryUnits(ctx context.Context, q bomQueryer, versionID int64) error {
	rows, err := q.Query(ctx, fmt.Sprintf(`
		SELECT COALESCE(NULLIF(component_type,''),'material'),
		       COALESCE(material_id,0),
		       COALESCE(component_product_id,0),
		       COALESCE(consume_unit,'')
		FROM %s.production_bom_version_items
		WHERE version_id=$1
	`, r.schema), versionID)
	if err != nil {
		return err
	}
	defer rows.Close()
	items := make([]bomapp.ProductionBomDraftItem, 0)
	needMaterials := false
	needProducts := false
	materialIDs := make([]int64, 0)
	productIDs := make([]int64, 0)
	for rows.Next() {
		var item bomapp.ProductionBomDraftItem
		if err := rows.Scan(&item.ComponentType, &item.MaterialID, &item.ComponentProductID, &item.ConsumeUnit); err != nil {
			return err
		}
		items = append(items, item)
		if !bomapp.ProductionBomConsumeUnitRequiresInventoryMatch(item.ConsumeUnit) {
			continue
		}
		if item.ComponentType == "product" || item.ComponentType == "finished_product" {
			needProducts = true
			productIDs = append(productIDs, item.ComponentProductID)
		} else {
			needMaterials = true
			materialIDs = append(materialIDs, item.MaterialID)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rows.Close()
	if !needMaterials && !needProducts {
		return nil
	}
	var materials []bomapp.Option
	var products []bomapp.Option
	if needMaterials {
		materials, err = listProductionBomMaterialOptions(ctx, q, r.schema, materialIDs)
		if err != nil {
			return err
		}
	}
	if needProducts {
		products, err = listProductionBomProductOptions(ctx, q, r.schema, productIDs)
		if err != nil {
			return err
		}
	}
	return bomapp.ValidateProductionBomDraftItemInventoryUnits(items, materials, products)
}

func (r Repository) ValidateProductionBomVersionForPublish(ctx context.Context, cmd bomapp.PublishProductionBomVersionCommand) error {
	return r.validateProductionBomVersionForPublish(ctx, r.pool, cmd)
}

func (r Repository) validateProductionBomVersionForPublish(ctx context.Context, q bomQueryer, cmd bomapp.PublishProductionBomVersionCommand) error {
	var bomID int64
	var outputType string
	var outputProductID int64
	var outputMaterialID int64
	var outputUnit string
	var itemCount int64
	if err := q.QueryRow(ctx, fmt.Sprintf(`
		SELECT v.bom_id,
		       COALESCE(NULLIF(pb.output_type,''),'product'),
		       COALESCE(pb.output_product_id,0),
		       COALESCE(pb.output_material_id,0),
		       COALESCE(NULLIF(v.output_unit,''),'unit'),
		       COALESCE((SELECT COUNT(*) FROM %s.production_bom_version_items i WHERE i.version_id=v.id),0)
		FROM %s.production_bom_versions v
		JOIN %s.production_boms pb ON pb.id=v.bom_id
		WHERE v.id=$1 AND v.status='draft'
	`, r.schema, r.schema, r.schema), cmd.VersionID).Scan(&bomID, &outputType, &outputProductID, &outputMaterialID, &outputUnit, &itemCount); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("production BOM version not found")
		}
		return err
	}
	outputID := outputProductID
	if outputType == "material" {
		outputID = outputMaterialID
	}
	if outputType == "product" && outputProductID <= 0 {
		return fmt.Errorf("output_product_id required")
	}
	if outputType == "material" && outputMaterialID <= 0 {
		return fmt.Errorf("output_material_id required")
	}
	if (outputType != "product" && outputType != "material") || outputID <= 0 || (outputType == "product" && outputMaterialID > 0) || (outputType == "material" && outputProductID > 0) {
		return fmt.Errorf("invalid production BOM output binding")
	}
	if itemCount <= 0 {
		return fmt.Errorf("components required")
	}
	if err := validateProductionBomComponentsForPublish(ctx, q, r.schema, cmd.VersionID); err != nil {
		return err
	}
	if err := r.validateProductionBomVersionItemInventoryUnits(ctx, q, cmd.VersionID); err != nil {
		return err
	}
	if err := r.validateProductionBomOutputTarget(ctx, q, outputType, outputID, outputUnit); err != nil {
		return err
	}
	var hasSelfReference bool
	if err := q.QueryRow(ctx, fmt.Sprintf(`
		SELECT EXISTS(
			SELECT 1
			FROM %s.production_bom_version_items i
			WHERE i.version_id=$1
			  AND CASE WHEN i.component_type IN ('product','finished_product') THEN 'product' ELSE 'material' END=$2
			  AND CASE WHEN i.component_type IN ('product','finished_product') THEN i.component_product_id ELSE i.material_id END=$3
		)
	`, r.schema), cmd.VersionID, outputType, outputID).Scan(&hasSelfReference); err != nil {
		return err
	}
	if hasSelfReference {
		return fmt.Errorf("typed self-reference detected")
	}
	if err := validateProductionBomDefaultGraphCandidate(ctx, q, r.schema, cmd.VersionID); err != nil {
		return err
	}
	if err := validateProductionBomRouteStandardCostCapacity(ctx, q, r.schema, cmd.VersionID); err != nil {
		return err
	}
	_ = bomID
	return nil
}

func validateProductionBomDefaultGraphCandidate(ctx context.Context, q bomQueryer, schema string, candidateVersionID int64) error {
	return postgresbomgraph.ValidateCandidate(ctx, q, schema, candidateVersionID)
}

func validateProductionBomComponentsForPublish(ctx context.Context, q bomQueryer, schema string, versionID int64) error {
	var componentType string
	var componentID int64
	var exists bool
	var active bool
	err := q.QueryRow(ctx, fmt.Sprintf(`
		SELECT invalid.component_type, invalid.component_id, invalid.target_exists, invalid.target_active
		FROM (
			SELECT i.id,
			       CASE WHEN i.component_type IN ('product','finished_product') THEN 'product' ELSE 'material' END AS component_type,
			       CASE WHEN i.component_type IN ('product','finished_product') THEN COALESCE(i.component_product_id,0) ELSE COALESCE(i.material_id,0) END AS component_id,
			       CASE WHEN i.component_type IN ('product','finished_product') THEN p.id IS NOT NULL ELSE m.id IS NOT NULL END AS target_exists,
			       CASE WHEN i.component_type IN ('product','finished_product') THEN COALESCE(p.active,false) ELSE m.id IS NOT NULL AND m.deprecated_at IS NULL END AS target_active
			FROM %[1]s.production_bom_version_items i
			LEFT JOIN %[1]s.materials m ON i.component_type NOT IN ('product','finished_product') AND m.id=i.material_id
			LEFT JOIN %[1]s.products p ON i.component_type IN ('product','finished_product') AND p.id=i.component_product_id
			WHERE i.version_id=$1
		) invalid
		WHERE NOT invalid.target_exists OR NOT invalid.target_active
		ORDER BY invalid.id
		LIMIT 1
	`, schema), versionID).Scan(&componentType, &componentID, &exists, &active)
	if err == pgx.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("component %s not found: %d", componentType, componentID)
	}
	if !active {
		return fmt.Errorf("component %s is inactive: %d", componentType, componentID)
	}
	return nil
}

func (r Repository) validateProductionBomOutputTarget(ctx context.Context, q bomQueryer, outputType string, outputID int64, outputUnit string) error {
	outputUnit = strings.TrimSpace(outputUnit)
	if outputType == "material" {
		var inventoryUnit string
		var active bool
		if err := q.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(NULLIF(unit,''),'unit'), deprecated_at IS NULL FROM %s.materials WHERE id=$1`, r.schema), outputID).Scan(&inventoryUnit, &active); err != nil {
			if err == pgx.ErrNoRows {
				return fmt.Errorf("output material not found")
			}
			return err
		}
		if !active {
			return fmt.Errorf("output material is inactive")
		}
		if strings.TrimSpace(inventoryUnit) != outputUnit {
			return fmt.Errorf("output_unit must match output material inventory unit")
		}
		return nil
	}
	products, err := listProductionBomProductOptions(ctx, q, r.schema, []int64{outputID})
	if err != nil {
		return err
	}
	for _, product := range products {
		if product.ID != outputID {
			continue
		}
		if strings.TrimSpace(product.InventoryUnit) != outputUnit {
			return fmt.Errorf("output_unit must match output product inventory unit")
		}
		return nil
	}
	return fmt.Errorf("output product not found or inactive")
}

func validateProductionBomRouteStandardCostCapacity(ctx context.Context, q bomQueryer, schema string, versionID int64) error {
	var processRouteID int64
	if err := q.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(process_route_id,0)
		FROM %s.production_bom_versions
		WHERE id=$1
	`, schema), versionID).Scan(&processRouteID); err != nil {
		return err
	}
	if processRouteID <= 0 {
		return nil
	}
	var missingCount int64
	if err := q.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*)
		FROM %s.process_route_operations
		WHERE route_id=$1 AND COALESCE(standard_cost_capacity_id,0)<=0
	`, schema), processRouteID).Scan(&missingCount); err != nil {
		return err
	}
	if missingCount > 0 {
		return fmt.Errorf("工艺路线工序缺少标准成本产能档")
	}
	var invalidCount int64
	if err := q.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*)
		FROM %[1]s.process_route_operations pro
		LEFT JOIN %[1]s.manufacturing_workstation_capacities c ON c.id=pro.standard_cost_capacity_id AND c.status='active'
		LEFT JOIN %[1]s.manufacturing_workstations w ON w.id=c.workstation_id AND w.status='active'
		LEFT JOIN %[1]s.manufacturing_workstation_operations wo ON wo.workstation_id=w.id AND wo.operation_id=pro.operation_id
		WHERE pro.route_id=$1
		  AND (c.id IS NULL OR w.id IS NULL OR wo.operation_id IS NULL)
	`, schema), processRouteID).Scan(&invalidCount); err != nil {
		return err
	}
	if invalidCount > 0 {
		return fmt.Errorf("标准成本产能档必须来自启用工位且适用当前工序")
	}
	return nil
}

func (r Repository) PublishProductionBomVersion(ctx context.Context, cmd bomapp.PublishProductionBomVersionCommand) error {
	return r.ValidateAndPublishProductionBomVersion(ctx, cmd)
}

func (r Repository) ValidateAndPublishProductionBomVersion(ctx context.Context, cmd bomapp.PublishProductionBomVersionCommand) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockProductionBomDefaultGraphTx(ctx, tx, r.schema); err != nil {
		return err
	}
	var lockedVersionID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT v.id
		FROM %s.production_bom_versions v
		JOIN %s.production_boms pb ON pb.id=v.bom_id
		WHERE v.id=$1 AND v.status='draft'
		FOR UPDATE OF v, pb
	`, r.schema, r.schema), cmd.VersionID).Scan(&lockedVersionID); err != nil {
		return fmt.Errorf("production BOM version not found")
	}
	if err := lockProductionBomPublishTargetsTx(ctx, tx, r.schema, lockedVersionID); err != nil {
		return err
	}
	if err := r.validateProductionBomVersionForPublish(ctx, tx, cmd); err != nil {
		return err
	}
	if err := r.publishProductionBomVersionTx(ctx, tx, cmd); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func lockProductionBomDefaultGraphTx(ctx context.Context, tx pgx.Tx, schema string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, schema+":production-bom-default-graph")
	return err
}

func lockProductionBomPublishTargetsTx(ctx context.Context, tx pgx.Tx, schema string, versionID int64) error {
	lockRows := func(query string) error {
		rows, err := tx.Query(ctx, query, versionID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return err
			}
		}
		return rows.Err()
	}
	if err := lockRows(fmt.Sprintf(`
		WITH target_ids AS (
			SELECT pb.output_material_id AS id
			FROM %[1]s.production_bom_versions v
			JOIN %[1]s.production_boms pb ON pb.id=v.bom_id
			WHERE v.id=$1 AND pb.output_type='material' AND pb.output_material_id>0
			UNION
			SELECT i.material_id
			FROM %[1]s.production_bom_version_items i
			WHERE i.version_id=$1
			  AND i.component_type NOT IN ('product','finished_product')
			  AND i.material_id>0
		)
		SELECT m.id
		FROM %[1]s.materials m
		JOIN target_ids target ON target.id=m.id
		ORDER BY m.id
		FOR UPDATE OF m
	`, schema)); err != nil {
		return err
	}
	return lockRows(fmt.Sprintf(`
		WITH target_ids AS (
			SELECT pb.output_product_id AS id
			FROM %[1]s.production_bom_versions v
			JOIN %[1]s.production_boms pb ON pb.id=v.bom_id
			WHERE v.id=$1 AND COALESCE(NULLIF(pb.output_type,''),'product')='product' AND pb.output_product_id>0
			UNION
			SELECT i.component_product_id
			FROM %[1]s.production_bom_version_items i
			WHERE i.version_id=$1
			  AND i.component_type IN ('product','finished_product')
			  AND i.component_product_id>0
		)
		SELECT p.id
		FROM %[1]s.products p
		JOIN target_ids target ON target.id=p.id
		ORDER BY p.id
		FOR UPDATE OF p
	`, schema))
}

func (r Repository) publishProductionBomVersionTx(ctx context.Context, tx pgx.Tx, cmd bomapp.PublishProductionBomVersionCommand) error {
	var bomID int64
	var outputType string
	var outputID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT v.bom_id,
		       COALESCE(NULLIF(pb.output_type,''),'product'),
		       CASE WHEN pb.output_type='material' THEN pb.output_material_id ELSE pb.output_product_id END
		FROM %s.production_bom_versions v
		JOIN %s.production_boms pb ON pb.id=v.bom_id
		WHERE v.id=$1 AND v.status='draft'
		FOR UPDATE OF v, pb
	`, r.schema, r.schema), cmd.VersionID).Scan(&bomID, &outputType, &outputID); err != nil {
		return fmt.Errorf("production BOM version not found")
	}
	snapshotCount, err := refreshProductionBomVersionOperationCostSnapshotsTx(ctx, tx, r.schema, cmd.VersionID)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_versions SET status='archived' WHERE bom_id=$1 AND status='published' AND id<>$2`, r.schema), bomID, cmd.VersionID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_versions SET status='published', published_at=now(), published_by=$2 WHERE id=$1`, r.schema), cmd.VersionID, strings.TrimSpace(cmd.Actor)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.product_production_bom_bindings
		SET bom_version_id=$1, bound_at=now(), bound_by=$3
		WHERE bom_id=$2
	`, r.schema), cmd.VersionID, bomID, strings.TrimSpace(cmd.Actor)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.production_bom_output_bindings
		SET bom_version_id=$1, updated_at=now(), updated_by=$3
		WHERE bom_id=$2
	`, r.schema), cmd.VersionID, bomID, strings.TrimSpace(cmd.Actor)); err != nil {
		return err
	}
	if outputType == "material" {
		var inserted bool
		err := tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.production_bom_output_bindings(output_type, output_id, bom_id, bom_version_id, is_default, updated_at, updated_by)
			VALUES('material',$1,$2,$3,true,now(),$4)
			ON CONFLICT(output_type, output_id) DO NOTHING
			RETURNING true
		`, r.schema), outputID, bomID, cmd.VersionID, strings.TrimSpace(cmd.Actor)).Scan(&inserted)
		if err != nil && err != pgx.ErrNoRows {
			return err
		}
		if inserted {
			if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "material", &outputID, "set_default_production_bom", postgresinfra.StrPtr("default_production_bom_id"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", bomID)), postgresinfra.AuditMeta{"output_type": outputType, "output_id": outputID, "production_bom_id": bomID, "production_bom_version_id": cmd.VersionID, "automatic": true, "reason": "first_published_material_bom"}); err != nil {
				return err
			}
		}
	}
	if outputType == "product" {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.product_production_configs
			SET production_bom_version_id=0, updated_at=now(), updated_by=$2
			WHERE production_bom_id=$1
		`, r.schema), bomID, strings.TrimSpace(cmd.Actor)); err != nil {
			return err
		}
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "production_bom_version", &cmd.VersionID, "publish_production_bom_version", postgresinfra.StrPtr("status"), nil, postgresinfra.StrPtr("published"), postgresinfra.AuditMeta{"bom_id": bomID, "version_id": cmd.VersionID, "output_type": outputType, "output_id": outputID, "operation_cost_snapshot_count": snapshotCount}); err != nil {
		return err
	}
	return nil
}

func refreshProductionBomVersionOperationCostSnapshotsTx(ctx context.Context, tx pgx.Tx, schema string, versionID int64) (int, error) {
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.production_bom_version_operation_costs WHERE version_id=$1`, schema), versionID); err != nil {
		return 0, err
	}
	var processRouteID int64
	var outputUnit string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(process_route_id,0), COALESCE(NULLIF(output_unit,''),'unit')
		FROM %s.production_bom_versions
		WHERE id=$1
	`, schema), versionID).Scan(&processRouteID, &outputUnit); err != nil {
		return 0, err
	}
	if processRouteID <= 0 {
		return 0, nil
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT pro.seq,
		       pro.operation_id,
		       COALESCE(NULLIF(pro.operation,''), NULLIF(o.name,''), '工序') AS operation_name,
		       COALESCE(w.id,0) AS workstation_id,
		       COALESCE(w.name,'') AS workstation_name,
		       COALESCE(c.id,0) AS workstation_capacity_id,
		       COALESCE(c.name,'') AS capacity_name,
		       COALESCE(NULLIF(w.hourly_rate,0), c.hourly_rate, 0)::float8 AS hourly_rate,
		       COALESCE(c.standard_minutes,0)::float8 AS standard_minutes,
		       COALESCE(c.batch_size_qty,0)::float8 AS batch_size_qty,
		       COALESCE(c.batch_size_unit,'') AS batch_size_unit,
		       COALESCE(NULLIF(c.cost_method,''),'time') AS cost_method,
		       COALESCE(c.piece_rate,0)::float8 AS piece_rate,
		       COALESCE(wo.operation_id,0) AS applicable_operation_id,
		       COALESCE(pro.standard_cost_capacity_id,0) AS standard_cost_capacity_id
		FROM %[1]s.process_route_operations pro
		LEFT JOIN %[1]s.manufacturing_operations o ON o.id=pro.operation_id
		LEFT JOIN %[1]s.manufacturing_workstation_capacities c ON c.id=pro.standard_cost_capacity_id AND c.status='active'
		LEFT JOIN %[1]s.manufacturing_workstations w ON w.id=c.workstation_id AND w.status='active'
		LEFT JOIN %[1]s.manufacturing_workstation_operations wo ON wo.workstation_id=w.id AND wo.operation_id=pro.operation_id
		WHERE pro.route_id=$1
		ORDER BY pro.seq, pro.id
	`, schema), processRouteID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	type operationCostSnapshot struct {
		seq               int
		operationID       int64
		operationName     string
		workstationID     int64
		workstationName   string
		capacityID        int64
		capacityName      string
		hourlyRate        float64
		standardMinutes   float64
		batchSizeQty      float64
		batchSizeUnit     string
		costMethod        string
		pieceRate         float64
		rateUnit          string
		unitCost          float64
		operationCostUnit string
	}
	var snapshots []operationCostSnapshot
	for rows.Next() {
		var seq int
		var operationID int64
		var operationName string
		var workstationID int64
		var workstationName string
		var capacityID int64
		var capacityName string
		var hourlyRate float64
		var standardMinutes float64
		var batchSizeQty float64
		var batchSizeUnit string
		var costMethod string
		var pieceRate float64
		var applicableOperationID int64
		var standardCostCapacityID int64
		if err := rows.Scan(&seq, &operationID, &operationName, &workstationID, &workstationName, &capacityID, &capacityName, &hourlyRate, &standardMinutes, &batchSizeQty, &batchSizeUnit, &costMethod, &pieceRate, &applicableOperationID, &standardCostCapacityID); err != nil {
			return 0, err
		}
		if standardCostCapacityID <= 0 {
			return 0, fmt.Errorf("工艺路线工序缺少标准成本产能档")
		}
		if capacityID <= 0 || workstationID <= 0 || applicableOperationID <= 0 {
			return 0, fmt.Errorf("标准成本产能档必须来自启用工位且适用当前工序")
		}
		unitCost, operationCostUnit, ok := calculateBomOperationSnapshotCost(costMethod, pieceRate, hourlyRate, standardMinutes, batchSizeQty, batchSizeUnit, outputUnit)
		if !ok {
			if normalizeBomOperationCostMethod(costMethod) == "piece" {
				return 0, fmt.Errorf("计件成本必须大于 0")
			}
			return 0, fmt.Errorf("工序成本批量单位 %s 不能换算为 BOM 产出库存单位 %s", strings.TrimSpace(batchSizeUnit), strings.TrimSpace(outputUnit))
		}
		snapshots = append(snapshots, operationCostSnapshot{
			seq:               seq,
			operationID:       operationID,
			operationName:     operationName,
			workstationID:     workstationID,
			workstationName:   workstationName,
			capacityID:        capacityID,
			capacityName:      capacityName,
			hourlyRate:        hourlyRate,
			standardMinutes:   standardMinutes,
			batchSizeQty:      batchSizeQty,
			batchSizeUnit:     batchSizeUnit,
			costMethod:        normalizeBomOperationCostMethod(costMethod),
			pieceRate:         pieceRate,
			rateUnit:          bomOperationCostRateUnit(costMethod),
			unitCost:          unitCost,
			operationCostUnit: operationCostUnit,
		})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	for _, snapshot := range snapshots {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.production_bom_version_operation_costs(
				version_id,operation_id,operation_name,workstation_id,workstation_name,
				workstation_capacity_id,capacity_name,hourly_rate_snapshot,standard_minutes_snapshot,
				batch_size_qty_snapshot,batch_size_unit_snapshot,cost_method,piece_rate_snapshot,rate_unit_snapshot,
				operation_unit_cost,operation_cost_unit,sort_order,created_at
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,now())
		`, schema), versionID, snapshot.operationID, snapshot.operationName, snapshot.workstationID, snapshot.workstationName, snapshot.capacityID, snapshot.capacityName, snapshot.hourlyRate, snapshot.standardMinutes, snapshot.batchSizeQty, snapshot.batchSizeUnit, snapshot.costMethod, snapshot.pieceRate, snapshot.rateUnit, snapshot.unitCost, snapshot.operationCostUnit, snapshot.seq); err != nil {
			return 0, err
		}
	}
	return len(snapshots), nil
}

func calculateBomOperationSnapshotCost(costMethod string, pieceRate float64, hourlyRate float64, standardMinutes float64, batchSizeQty float64, batchSizeUnit string, outputUnit string) (float64, string, bool) {
	if normalizeBomOperationCostMethod(costMethod) == "piece" {
		if pieceRate <= 0 {
			return 0, "", false
		}
		return pieceRate, "sales_spec_count", true
	}
	outputBatchQty, ok := convertBomOperationBatchQty(batchSizeQty, batchSizeUnit, outputUnit)
	if !ok || outputBatchQty <= 0 {
		return 0, "", false
	}
	unitCost := 0.0
	if hourlyRate > 0 && standardMinutes > 0 {
		unitCost = hourlyRate * standardMinutes / 60 / outputBatchQty
	}
	return unitCost, strings.TrimSpace(outputUnit), true
}

func normalizeBomOperationCostMethod(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "piece") {
		return "piece"
	}
	return "time"
}

func bomOperationCostRateUnit(costMethod string) string {
	if normalizeBomOperationCostMethod(costMethod) == "piece" {
		return "sales_spec_count"
	}
	return "hour"
}

func convertBomOperationBatchQty(qty float64, sourceUnit string, targetUnit string) (float64, bool) {
	sourceUnit = normalizeBomOperationUnit(sourceUnit)
	targetUnit = normalizeBomOperationUnit(targetUnit)
	if qty <= 0 || sourceUnit == "" || targetUnit == "" {
		return 0, false
	}
	if sourceUnit == targetUnit {
		return qty, true
	}
	sourceFactor := bomOperationWeightKgFactor(sourceUnit)
	targetFactor := bomOperationWeightKgFactor(targetUnit)
	if sourceFactor <= 0 || targetFactor <= 0 {
		return 0, false
	}
	return qty * sourceFactor / targetFactor, true
}

func normalizeBomOperationUnit(unit string) string {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "公斤", "千克", "kg":
		return "kg"
	case "克", "g":
		return "g"
	case "磅", "lb", "lbs":
		return "lb"
	default:
		return strings.TrimSpace(unit)
	}
}

func bomOperationWeightKgFactor(unit string) float64 {
	switch normalizeBomOperationUnit(unit) {
	case "kg":
		return 1
	case "g":
		return 0.001
	case "lb":
		return 0.453592
	default:
		return 0
	}
}

func (r Repository) BindProductProductionBom(ctx context.Context, cmd bomapp.BindProductProductionBomCommand) (bomapp.ProductProductionBomBinding, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return bomapp.ProductProductionBomBinding{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockProductionBomDefaultGraphTx(ctx, tx, r.schema); err != nil {
		return bomapp.ProductProductionBomBinding{}, err
	}
	var latestVersionID int64
	var versionNo string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT v.id, COALESCE(v.version_no,'')
		FROM %s.production_boms pb
		JOIN LATERAL (
			SELECT id, version_no, published_at, created_at
			FROM %s.production_bom_versions
			WHERE bom_id=pb.id AND status='published'
			ORDER BY published_at DESC NULLS LAST, created_at DESC, id DESC
			LIMIT 1
		) v ON true
		WHERE pb.id=$1
		  AND COALESCE(NULLIF(pb.output_type,''),'product')='product'
		  AND pb.output_product_id=$2
		  AND (pb.status='active' OR COALESCE(NULLIF(pb.status,''),'active')='active')
	`, r.schema, r.schema), cmd.BomID, cmd.ProductID).Scan(&latestVersionID, &versionNo); err != nil {
		return bomapp.ProductProductionBomBinding{}, fmt.Errorf("published production BOM version not found")
	}
	if cmd.BomVersionID > 0 && cmd.BomVersionID != latestVersionID {
		return bomapp.ProductProductionBomBinding{}, fmt.Errorf("default production BOM always uses latest published version")
	}
	if err := validateProductionBomDefaultGraphCandidate(ctx, tx, r.schema, latestVersionID); err != nil {
		return bomapp.ProductProductionBomBinding{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_production_configs(
			product_id,
			production_bom_id,
			production_bom_version_id,
			process_route_id,
			industry_field_template_id,
			expected_loss_rate,
			note,
			created_by,
			updated_by
		)
		VALUES($1,$2,$3,0,0,0,'',$4,$4)
		ON CONFLICT (product_id) DO UPDATE SET
			production_bom_id=excluded.production_bom_id,
			production_bom_version_id=excluded.production_bom_version_id,
			updated_at=now(),
			updated_by=excluded.updated_by
	`, r.schema), cmd.ProductID, cmd.BomID, int64(0), strings.TrimSpace(cmd.Actor)); err != nil {
		return bomapp.ProductProductionBomBinding{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_production_bom_bindings(product_id, bom_id, bom_version_id, bound_at, bound_by)
		VALUES($1,$2,$3,now(),$4)
		ON CONFLICT (product_id) DO UPDATE SET bom_id=excluded.bom_id, bom_version_id=excluded.bom_version_id, bound_at=now(), bound_by=excluded.bound_by
	`, r.schema), cmd.ProductID, cmd.BomID, latestVersionID, strings.TrimSpace(cmd.Actor)); err != nil {
		return bomapp.ProductProductionBomBinding{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_output_bindings(output_type, output_id, bom_id, bom_version_id, is_default, updated_at, updated_by)
		VALUES('product',$1,$2,$3,true,now(),$4)
		ON CONFLICT(output_type, output_id) DO UPDATE SET
			bom_id=excluded.bom_id,
			bom_version_id=excluded.bom_version_id,
			is_default=true,
			updated_at=now(),
			updated_by=excluded.updated_by
	`, r.schema), cmd.ProductID, cmd.BomID, latestVersionID, strings.TrimSpace(cmd.Actor)); err != nil {
		return bomapp.ProductProductionBomBinding{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product", &cmd.ProductID, "set_default_production_bom", postgresinfra.StrPtr("default_production_bom_id"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", cmd.BomID)), postgresinfra.AuditMeta{"product_id": cmd.ProductID, "production_bom_id": cmd.BomID, "production_bom_version_id": latestVersionID, "production_bom_version_no": versionNo}); err != nil {
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

func (r Repository) ResolveDefaultPublishedOutputBom(ctx context.Context, outputType string, outputID int64) (bomapp.ProductionBomOutputBinding, error) {
	outputType = strings.ToLower(strings.TrimSpace(outputType))
	if (outputType != "product" && outputType != "material") || outputID <= 0 {
		return bomapp.ProductionBomOutputBinding{}, fmt.Errorf("invalid output binding")
	}
	var row bomapp.ProductionBomOutputBinding
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT ob.output_type, ob.output_id, ob.bom_id, ob.bom_version_id, ob.is_default
		FROM %[1]s.production_bom_output_bindings ob
		JOIN %[1]s.production_boms pb ON pb.id=ob.bom_id
		JOIN %[1]s.production_bom_versions v ON v.id=ob.bom_version_id AND v.bom_id=pb.id
		LEFT JOIN %[1]s.materials om ON ob.output_type='material' AND om.id=ob.output_id
		LEFT JOIN %[1]s.products op ON ob.output_type='product' AND op.id=ob.output_id
		WHERE ob.output_type=$1
		  AND ob.output_id=$2
		  AND ob.is_default=true
		  AND pb.output_type=ob.output_type
		  AND CASE WHEN pb.output_type='material' THEN pb.output_material_id ELSE pb.output_product_id END=ob.output_id
		  AND COALESCE(NULLIF(pb.status,''),'active')='active'
		  AND v.status='published'
		  AND ((ob.output_type='material' AND om.id IS NOT NULL AND om.deprecated_at IS NULL)
		       OR (ob.output_type='product' AND op.id IS NOT NULL AND COALESCE(op.active,true)=true))
	`, r.schema), outputType, outputID).Scan(&row.OutputType, &row.OutputID, &row.BomID, &row.BomVersionID, &row.IsDefault); err != nil {
		return bomapp.ProductionBomOutputBinding{}, err
	}
	return row, nil
}

func (r Repository) BindProductionBomOutput(ctx context.Context, cmd bomapp.BindProductionBomOutputCommand) (bomapp.ProductionBomOutputBinding, error) {
	outputType := strings.ToLower(strings.TrimSpace(cmd.OutputType))
	if (outputType != "product" && outputType != "material") || cmd.OutputID <= 0 || cmd.BomID <= 0 {
		return bomapp.ProductionBomOutputBinding{}, fmt.Errorf("invalid output binding")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return bomapp.ProductionBomOutputBinding{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockProductionBomDefaultGraphTx(ctx, tx, r.schema); err != nil {
		return bomapp.ProductionBomOutputBinding{}, err
	}
	var latestVersionID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT v.id
		FROM %[1]s.production_boms pb
		JOIN LATERAL (
			SELECT id
			FROM %[1]s.production_bom_versions
			WHERE bom_id=pb.id AND status='published'
			ORDER BY published_at DESC NULLS LAST, created_at DESC, id DESC
			LIMIT 1
		) v ON true
		WHERE pb.id=$1
		  AND pb.output_type=$2
		  AND CASE WHEN pb.output_type='material' THEN pb.output_material_id ELSE pb.output_product_id END=$3
		  AND COALESCE(NULLIF(pb.status,''),'active')='active'
		FOR UPDATE OF pb
	`, r.schema), cmd.BomID, outputType, cmd.OutputID).Scan(&latestVersionID); err != nil {
		return bomapp.ProductionBomOutputBinding{}, fmt.Errorf("published production BOM version not found")
	}
	if cmd.BomVersionID > 0 && cmd.BomVersionID != latestVersionID {
		return bomapp.ProductionBomOutputBinding{}, fmt.Errorf("default production BOM always uses latest published version")
	}
	if err := validateProductionBomDefaultGraphCandidate(ctx, tx, r.schema, latestVersionID); err != nil {
		return bomapp.ProductionBomOutputBinding{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_bom_output_bindings(output_type, output_id, bom_id, bom_version_id, is_default, updated_at, updated_by)
		VALUES($1,$2,$3,$4,true,now(),$5)
		ON CONFLICT(output_type, output_id) DO UPDATE SET
			bom_id=excluded.bom_id,
			bom_version_id=excluded.bom_version_id,
			is_default=true,
			updated_at=now(),
			updated_by=excluded.updated_by
	`, r.schema), outputType, cmd.OutputID, cmd.BomID, latestVersionID, strings.TrimSpace(cmd.Actor)); err != nil {
		return bomapp.ProductionBomOutputBinding{}, err
	}
	if outputType == "product" {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.product_production_bom_bindings(product_id, bom_id, bom_version_id, bound_at, bound_by)
			VALUES($1,$2,$3,now(),$4)
			ON CONFLICT(product_id) DO UPDATE SET bom_id=excluded.bom_id, bom_version_id=excluded.bom_version_id, bound_at=now(), bound_by=excluded.bound_by
		`, r.schema), cmd.OutputID, cmd.BomID, latestVersionID, strings.TrimSpace(cmd.Actor)); err != nil {
			return bomapp.ProductionBomOutputBinding{}, err
		}
	}
	entityType := outputType
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, entityType, &cmd.OutputID, "set_default_production_bom", postgresinfra.StrPtr("default_production_bom_id"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", cmd.BomID)), postgresinfra.AuditMeta{"output_type": outputType, "output_id": cmd.OutputID, "production_bom_id": cmd.BomID, "production_bom_version_id": latestVersionID}); err != nil {
		return bomapp.ProductionBomOutputBinding{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return bomapp.ProductionBomOutputBinding{}, err
	}
	return bomapp.ProductionBomOutputBinding{OutputType: outputType, OutputID: cmd.OutputID, BomID: cmd.BomID, BomVersionID: latestVersionID, IsDefault: true}, nil
}

func productionBomSummarySQL(schema, where string) string {
	return fmt.Sprintf(`
		SELECT pb.id,
		       COALESCE(pb.code,''),
		       COALESCE(pb.name,''),
		       COALESCE(NULLIF(pb.output_type,''),'product'),
		       CASE WHEN pb.output_type='material' THEN COALESCE(pb.output_material_id,0) ELSE COALESCE(pb.output_product_id,0) END,
		       CASE WHEN pb.output_type='material' THEN COALESCE(om.name,'') ELSE COALESCE(op.name,'') END,
		       CASE WHEN pb.output_type='material' THEN COALESCE(om.code,'') ELSE CASE WHEN COALESCE(pb.output_product_id,0)>0 THEN 'SKU-' || lpad(pb.output_product_id::text,6,'0') ELSE '' END END,
		       COALESCE(NULLIF(latest.output_unit,''),'unit'),
		       COALESCE(pb.output_product_id,0),
		       COALESCE(op.name,''),
		       CASE WHEN COALESCE(pb.output_product_id,0)>0 THEN 'SKU-' || lpad(pb.output_product_id::text,6,'0') ELSE '' END,
		       COALESCE(pb.output_material_id,0),
		       COALESCE(om.name,''),
		       COALESCE(om.code,''),
		       COALESCE(bga.group_id,0),
		       COALESCE(bg.name,''),
		       COALESCE(bga.group_item_id,0),
		       COALESCE(item.name,''),
		       COALESCE(NULLIF(pb.status,''),'active'),
		       COALESCE(latest.id,0),
		       COALESCE(latest.version_no,''),
			       COALESCE(latest.status,'') AS latest_version_status,
			       COALESCE(usable.process_route_id,0) AS process_route_id,
			       COALESCE(route.name,'') AS process_route_name,
			       COALESCE(usable.id,0)>0 AS is_latest_usable,
			       COALESCE(latest.yield_rate,0)::float8,
		       COALESCE((
		           SELECT COUNT(*)
		           FROM %[1]s.production_bom_output_bindings b
		           WHERE b.bom_id=pb.id
		       ),0),
		       COALESCE(to_char(pb.updated_at,'YYYY-MM-DD HH24:MI'),'-')
		FROM %[1]s.production_boms pb
		LEFT JOIN %[1]s.products op ON op.id=pb.output_product_id
		LEFT JOIN %[1]s.materials om ON om.id=pb.output_material_id
		LEFT JOIN %[1]s.business_group_assignments bga ON bga.object_id=pb.id AND lower(bga.usage_key)='production_bom' AND lower(bga.object_key)='production_bom'
		LEFT JOIN %[1]s.business_groups bg ON bg.id=bga.group_id
		LEFT JOIN %[1]s.business_group_items item ON item.id=bga.group_item_id
			LEFT JOIN LATERAL (
				SELECT id, version_no, status, yield_rate, output_unit
			FROM %[1]s.production_bom_versions v
			WHERE v.bom_id=pb.id AND v.status IN ('draft','published')
			ORDER BY CASE WHEN v.status='draft' THEN 0 ELSE 1 END,
			         v.published_at DESC NULLS LAST, v.created_at DESC, v.id DESC
				LIMIT 1
			) latest ON true
			LEFT JOIN LATERAL (
				SELECT id, version_no, status, yield_rate, process_route_id
				FROM %[1]s.production_bom_versions v
				WHERE v.bom_id=pb.id AND v.status='published'
				ORDER BY v.published_at DESC NULLS LAST, v.created_at DESC, v.id DESC
				LIMIT 1
			) usable ON true
			LEFT JOIN %[1]s.process_routes route ON route.id=usable.process_route_id
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
			&row.OutputType,
			&row.OutputID,
			&row.OutputName,
			&row.OutputCode,
			&row.OutputUnit,
			&row.OutputProductID,
			&row.OutputProductName,
			&row.OutputProductCode,
			&row.OutputMaterialID,
			&row.OutputMaterialName,
			&row.OutputMaterialCode,
			&row.BusinessGroupID,
			&row.BusinessGroupName,
			&row.GroupItemID,
			&row.GroupItemName,
			&row.Status,
			&row.LatestVersionID,
			&row.LatestVersionNo,
			&row.LatestVersionStatus,
			&row.ProcessRouteID,
			&row.ProcessRouteName,
			&row.IsLatestUsable,
			&row.ExpectedYieldRate,
			&row.ReferenceProductCount,
			&row.UpdatedAt,
		); err != nil {
			return nil, err
		}
		row.GroupID = row.BusinessGroupID
		row.GroupName = row.BusinessGroupName
		row.GroupCategoryID = row.GroupItemID
		row.GroupCategoryName = row.GroupItemName
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
		       COALESCE(v.material_loss_rate,0)::float8,
		       COALESCE((SELECT COUNT(*) FROM %s.production_bom_version_items i WHERE i.version_id=v.id),0),
		       COALESCE(v.note,''),
		       COALESCE(v.special_attrs_schema_json::text,'[]'),
		       COALESCE(v.special_attrs_json::text,'{}'),
		       COALESCE(v.process_route_id,0),
		       COALESCE(route.name,''),
		       COALESCE(to_char(v.created_at,'YYYY-MM-DD HH24:MI'),'-'),
		       COALESCE(to_char(v.published_at,'YYYY-MM-DD HH24:MI'),''),
		       v.id=COALESCE((SELECT id FROM latest),0),
		       v.status='published' AND v.id=COALESCE((SELECT id FROM latest),0)
		FROM %s.production_bom_versions v
		LEFT JOIN %s.process_routes route ON route.id=v.process_route_id
		WHERE v.bom_id=$1
		ORDER BY v.created_at DESC, v.id DESC
	`, r.schema, r.schema, r.schema, r.schema), bomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]bomapp.ProductionBomVersion, 0)
	for rows.Next() {
		var row bomapp.ProductionBomVersion
		if err := rows.Scan(&row.ID, &row.BomID, &row.VersionNo, &row.Status, &row.YieldRate, &row.OutputQty, &row.OutputUnit, &row.MaterialLossRate, &row.ItemCount, &row.Note, &row.SpecialAttrsSchemaJSON, &row.SpecialAttrsJSON, &row.ProcessRouteID, &row.ProcessRouteName, &row.CreatedAt, &row.PublishedAt, &row.IsLatest, &row.IsLatestUsable); err != nil {
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
			       COALESCE(NULLIF(pb.status,''),'active') AS bom_status,
			       COALESCE(NULLIF(ppc.production_bom_id,0), pbb.bom_id, 0)=pb.id AS is_default,
			       COALESCE(NULLIF(pb.status,''),'active')='active' AND COALESCE(published_v.id,0)>0 AS can_set_default,
			       COALESCE(published_v.id,0) AS current_published_version_id,
			       COALESCE(published_v.version_no,'') AS current_published_version_no,
			       'output' AS relation_type,
			       '' AS consume_unit,
			       0::float8 AS qty_per_unit,
			       0 AS relation_sort,
			       0 AS sort_item_id
			FROM %[1]s.production_boms pb
			JOIN %[1]s.products op ON op.id=pb.output_product_id AND op.active=true
			LEFT JOIN %[1]s.product_production_configs ppc ON ppc.product_id=$1
			LEFT JOIN %[1]s.product_production_bom_bindings pbb ON pbb.product_id=$1
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
			LEFT JOIN LATERAL (
				SELECT id, version_no
				FROM %[1]s.production_bom_versions v
				WHERE v.bom_id=pb.id AND v.status='published'
				ORDER BY v.published_at DESC NULLS LAST,
				         v.created_at DESC,
				         v.id DESC
				LIMIT 1
			) published_v ON true
			WHERE pb.output_product_id=$1
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
			       COALESCE(NULLIF(pb.status,''),'active') AS bom_status,
			       EXISTS(
			           SELECT 1 FROM %[1]s.product_production_bom_bindings b
			           WHERE b.product_id=COALESCE(pb.output_product_id,0) AND b.bom_id=pb.id
			       ) AS is_default,
			       false AS can_set_default,
			       0::bigint AS current_published_version_id,
			       '' AS current_published_version_no,
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
		       bom_status,
		       is_default,
		       can_set_default,
		       current_published_version_id,
		       current_published_version_no,
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
			       COALESCE(NULLIF(pb.status,''),'active') AS bom_status,
			       EXISTS(
			           SELECT 1 FROM %[1]s.product_production_bom_bindings b
			           WHERE b.product_id=COALESCE(pb.output_product_id,0) AND b.bom_id=pb.id
			       ) AS is_default,
			       false AS can_set_default,
			       0::bigint AS current_published_version_id,
			       '' AS current_published_version_no,
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
		       bom_status,
		       is_default,
		       can_set_default,
		       current_published_version_id,
		       current_published_version_no,
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
		if err := rows.Scan(&row.BomID, &row.BomCode, &row.BomName, &row.BomVersionID, &row.BomVersionNo, &row.OutputProductID, &row.OutputProductName, &row.BomStatus, &row.IsDefault, &row.CanSetDefault, &row.CurrentPublishedVersionID, &row.CurrentPublishedVersionNo, &row.RelationType, &row.ConsumeUnit, &row.QtyPerUnit); err != nil {
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
		       COALESCE(v.material_loss_rate,0)::float8,
		       COALESCE((SELECT COUNT(*) FROM %s.production_bom_version_items i WHERE i.version_id=v.id),0),
		       COALESCE(v.note,''),
		       COALESCE(v.special_attrs_schema_json::text,'[]'),
		       COALESCE(v.special_attrs_json::text,'{}'),
		       COALESCE(v.process_route_id,0),
		       COALESCE(route.name,''),
		       COALESCE(to_char(v.created_at,'YYYY-MM-DD HH24:MI'),'-'),
		       COALESCE(to_char(v.published_at,'YYYY-MM-DD HH24:MI'),''),
		       v.id=COALESCE((
		           SELECT latest.id
		           FROM %s.production_bom_versions latest
		           WHERE latest.bom_id=v.bom_id AND latest.status='published'
		           ORDER BY latest.published_at DESC NULLS LAST, latest.id DESC
		           LIMIT 1
		       ),0),
		       v.status='published' AND v.id=COALESCE((
		           SELECT latest.id
		           FROM %s.production_bom_versions latest
		           WHERE latest.bom_id=v.bom_id AND latest.status='published'
		           ORDER BY latest.published_at DESC NULLS LAST, latest.id DESC
		           LIMIT 1
		       ),0)
		FROM %s.production_bom_versions v
		LEFT JOIN %s.process_routes route ON route.id=v.process_route_id
		WHERE v.id=$1
	`, r.schema, r.schema, r.schema, r.schema, r.schema), id)
	if err != nil {
		return bomapp.ProductionBomVersion{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		return bomapp.ProductionBomVersion{}, pgx.ErrNoRows
	}
	var row bomapp.ProductionBomVersion
	if err := rows.Scan(&row.ID, &row.BomID, &row.VersionNo, &row.Status, &row.YieldRate, &row.OutputQty, &row.OutputUnit, &row.MaterialLossRate, &row.ItemCount, &row.Note, &row.SpecialAttrsSchemaJSON, &row.SpecialAttrsJSON, &row.ProcessRouteID, &row.ProcessRouteName, &row.CreatedAt, &row.PublishedAt, &row.IsLatest, &row.IsLatestUsable); err != nil {
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
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockProductionBomDefaultGraphTx(ctx, tx, schema); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, schema+":pr403-legacy-bom-binding-repair"); err != nil {
		return err
	}
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
	    EXISTS (SELECT 1 FROM %[1]s.product_bom_items bi WHERE bi.product_id=p.id)
	    OR EXISTS (
	      SELECT 1
	      FROM %[1]s.production_boms source_bom
	      JOIN %[1]s.production_bom_versions source_version ON source_version.bom_id=source_bom.id AND source_version.status='published'
	      JOIN %[1]s.production_bom_version_items source_item ON source_item.version_id=source_version.id
	      WHERE source_bom.legacy_product_id=p.id
	    )
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
	RETURNING id, legacy_product_id
),
target_boms AS (
	SELECT pbom.id AS bom_id,
	       mlb.product_id,
	       mlb.yield_rate,
	       mlb.updated_at
	FROM missing_legacy_bindings mlb
	JOIN %[1]s.production_boms pbom ON pbom.legacy_product_id=mlb.product_id
	UNION ALL
	SELECT inserted.id AS bom_id,
	       mlb.product_id,
	       mlb.yield_rate,
	       mlb.updated_at
	FROM missing_legacy_bindings mlb
	JOIN inserted_boms inserted ON inserted.legacy_product_id=mlb.product_id
),
version_source AS (
	SELECT target.bom_id,
	       target.product_id,
	       target.yield_rate,
	       target.updated_at
	FROM target_boms target
	WHERE EXISTS (
		SELECT 1 FROM %[1]s.product_bom_items source_item WHERE source_item.product_id=target.product_id
	)
	  AND NOT EXISTS (
		SELECT 1 FROM %[1]s.production_bom_versions existing_version WHERE existing_version.bom_id=target.bom_id
	)
),
inserted_versions AS (
	INSERT INTO %[1]s.production_bom_versions(bom_id, version_no, status, yield_rate, output_qty, output_unit, note, legacy_product_id, legacy_bom_version_id, created_at, published_at, created_by, published_by)
	SELECT bom_id, 'V001', 'published', yield_rate, 1, 'kg', '旧 BOM 绑定修复', product_id, 0, updated_at, updated_at, 'system-pr403-legacy-binding-repair', 'system-pr403-legacy-binding-repair'
	FROM version_source
	ON CONFLICT DO NOTHING
	RETURNING id, bom_id, legacy_product_id, published_at
),
target_versions AS (
	SELECT existing.id AS version_id,
	       existing.bom_id,
	       existing.legacy_product_id AS product_id,
	       existing.published_at
	FROM target_boms target
	JOIN %[1]s.production_bom_versions existing ON existing.bom_id=target.bom_id AND existing.status='published'
	UNION ALL
	SELECT inserted.id AS version_id,
	       inserted.bom_id,
	       inserted.legacy_product_id AS product_id,
	       inserted.published_at
	FROM inserted_versions inserted
),
item_source AS (
	SELECT target.version_id,
	       i.material_id,
	       COALESCE(NULLIF(i.component_type,''),'material') AS component_type,
	       COALESCE(i.component_product_id,0) AS component_product_id,
	       COALESCE(i.component_spec_g,0) AS component_spec_g,
	       COALESCE(NULLIF(i.consume_unit,''),'ratio_pct') AS consume_unit,
	       COALESCE(i.qty_per_unit,0) AS qty_per_unit,
	       COALESCE(i.ratio_pct,0) AS ratio_pct,
	       0 AS material_loss_rate,
	       COALESCE(i.unit_cost_snapshot,0) AS unit_cost_snapshot
	FROM target_versions target
	JOIN %[1]s.product_bom_items i ON i.product_id=target.product_id
	WHERE NOT EXISTS (
		SELECT 1 FROM %[1]s.production_bom_version_items existing_item WHERE existing_item.version_id=target.version_id
	)
),
inserted_items AS (
	INSERT INTO %[1]s.production_bom_version_items(version_id, material_id, component_type, component_product_id, component_spec_g, consume_unit, qty_per_unit, ratio_pct, material_loss_rate, unit_cost_snapshot)
	SELECT version_id, material_id, component_type, component_product_id, component_spec_g, consume_unit, qty_per_unit, ratio_pct, material_loss_rate, unit_cost_snapshot
	FROM item_source
	RETURNING version_id
),
binding_version_candidates AS (
	SELECT existing.id AS version_id,
	       existing.bom_id,
	       existing.published_at
	FROM %[1]s.production_bom_versions existing
	WHERE existing.status='published'
	  AND EXISTS (
	    SELECT 1 FROM %[1]s.production_bom_version_items existing_item WHERE existing_item.version_id=existing.id
	  )
	UNION ALL
	SELECT target.version_id,
	       target.bom_id,
	       target.published_at
	FROM target_versions target
	WHERE EXISTS (
		SELECT 1 FROM inserted_items inserted_item WHERE inserted_item.version_id=target.version_id
	)
),
binding_rows AS (
	SELECT target.product_id, target.bom_id, selected.version_id AS bom_version_id
	FROM target_boms target
	JOIN LATERAL (
		SELECT candidate.version_id
		FROM binding_version_candidates candidate
		WHERE candidate.bom_id=target.bom_id
		ORDER BY candidate.published_at DESC NULLS LAST, candidate.version_id DESC
		LIMIT 1
	) selected ON true
)
INSERT INTO %[1]s.product_production_bom_bindings(product_id, bom_id, bom_version_id, bound_by)
SELECT product_id, bom_id, bom_version_id, 'system-pr403-legacy-binding-repair'
FROM binding_rows
WHERE bom_version_id > 0
ON CONFLICT DO NOTHING;
`, schema)
	if _, err := tx.Exec(ctx, q); err != nil {
		return err
	}
	var hasTypedOutputBindings bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, schema+`.production_bom_output_bindings`).Scan(&hasTypedOutputBindings); err != nil {
		return err
	}
	if hasTypedOutputBindings {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %[1]s.production_bom_output_bindings(output_type, output_id, bom_id, bom_version_id, is_default, updated_at, updated_by)
			SELECT 'product', b.product_id, b.bom_id, b.bom_version_id, true, now(), 'system-pr403-legacy-binding-repair'
			FROM %[1]s.product_production_bom_bindings b
			JOIN %[1]s.production_boms pb ON pb.id=b.bom_id
			  AND COALESCE(NULLIF(pb.output_type,''),'product')='product'
			  AND pb.output_product_id=b.product_id
			  AND COALESCE(NULLIF(pb.status,''),'active')='active'
			JOIN %[1]s.production_bom_versions v ON v.id=b.bom_version_id AND v.bom_id=b.bom_id AND v.status='published'
			WHERE b.product_id>0
			ON CONFLICT(output_type, output_id) DO NOTHING
		`, schema)); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
