package bom

import (
	"context"
	"fmt"
	"strings"

	bomapp "orderapp/internal/application/bom"
	bomdomain "orderapp/internal/domain/bom"
	catalogdomain "orderapp/internal/domain/catalog"
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
			COALESCE(b.yield_rate, 0.8),
			COALESCE(NULLIF(b.status,''), CASE WHEN b.product_id IS NULL THEN 'missing' ELSE 'active' END),
			COALESCE((SELECT COUNT(*) FROM %s.product_bom_items bi WHERE bi.product_id = p.id), 0),
			COALESCE(to_char(b.updated_at,'YYYY-MM-DD HH24:MI'), '-')
		FROM %s.products p
		LEFT JOIN %s.product_bom b ON b.product_id = p.id
		WHERE p.active = true
		ORDER BY p.name
	`, r.schema, r.schema, r.schema)
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]bomapp.ListItem, 0)
	for rows.Next() {
		var item bomapp.ListItem
		var fallback float64
		if err := rows.Scan(&item.ProductID, &item.CustomerID, &item.Product, &item.RoastLevel, &item.ProductKind, &fallback, &item.Status, &item.ItemCount, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.YieldRate = catalogdomain.ResolveYieldRate(item.RoastLevel, fallback)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r Repository) Detail(ctx context.Context, productID int64) (bomapp.Detail, error) {
	var productName string
	var roastLevel string
	var yieldRate float64
	var status string
	var updatedAt string
	err := r.pool.QueryRow(ctx,
		"SELECT COALESCE(p.name,''), COALESCE(p.roast_level,''), COALESCE(b.yield_rate,0.8), COALESCE(NULLIF(b.status,''), CASE WHEN b.product_id IS NULL THEN 'missing' ELSE 'active' END), COALESCE(to_char(b.updated_at,'YYYY-MM-DD HH24:MI'),'-') "+
			"FROM "+r.schema+".products p LEFT JOIN "+r.schema+".product_bom b ON b.product_id=p.id "+
			"WHERE p.id=$1", productID).Scan(&productName, &roastLevel, &yieldRate, &status, &updatedAt)
	if err != nil {
		return bomapp.Detail{}, err
	}

	items, total, err := listBomItems(ctx, r.pool, r.schema, productID)
	if err != nil {
		return bomapp.Detail{}, err
	}
	return bomapp.Detail{
		ProductID:   productID,
		ProductName: productName,
		RoastLevel:  roastLevel,
		YieldRate:   catalogdomain.ResolveYieldRate(roastLevel, yieldRate),
		Status:      status,
		Items:       bomItemsToApp(items),
		TotalRatio:  total,
		UpdatedAt:   updatedAt,
	}, nil
}

func (r Repository) Products(ctx context.Context) ([]bomapp.Option, error) {
	rows, err := r.pool.Query(ctx, "SELECT p.id, p.name, COALESCE(p.customer_id,0), COALESCE(p.roast_level,''), COALESCE(NULLIF(p.product_kind,''),'roasted_bean'), COALESCE(p.drip_bag_grams,10)::float8, COALESCE(p.drip_box_bag_count,10) FROM "+r.schema+".products p WHERE p.active=true ORDER BY p.name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]bomapp.Option, 0)
	for rows.Next() {
		var opt bomapp.Option
		if err := rows.Scan(&opt.ID, &opt.Name, &opt.CustomerID, &opt.RoastLevel, &opt.ProductKind, &opt.DripBagGrams, &opt.DripBoxBagCount); err != nil {
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
	var roastLevel string
	if err := r.pool.QueryRow(ctx, "SELECT COALESCE(roast_level,'') FROM "+r.schema+".products WHERE id=$1", cmd.ProductID).Scan(&roastLevel); err != nil {
		return fmt.Errorf("product not found")
	}
	yieldRate := catalogdomain.ResolveYieldRate(roastLevel, 0.8)
	q := "INSERT INTO " + r.schema + ".product_bom(product_id,yield_rate,status,updated_at) VALUES($1,$2,'active',now()) ON CONFLICT (product_id) DO UPDATE SET yield_rate=excluded.yield_rate, status='active', updated_at=now()"
	_, err := r.pool.Exec(ctx, q, cmd.ProductID, yieldRate)
	if err == nil {
		postgresinfra.AuditInsert(ctx, r.pool, r.schema, cmd.Actor, "product_bom", &cmd.ProductID, "sync_yield", postgresinfra.StrPtr("yield_rate"), nil, postgresinfra.StrPtr(fmt.Sprintf("%.4f", yieldRate)), postgresinfra.AuditMeta{"product_id": cmd.ProductID, "status": "active"})
	}
	return err
}

func (r Repository) DeactivateBom(ctx context.Context, cmd bomapp.DeactivateBomCommand) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

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
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT v.id,v.product_id,v.version_no,v.status,COALESCE(v.yield_rate,0.8),
		       COALESCE((SELECT COUNT(*) FROM %s.bom_version_items i WHERE i.version_id=v.id),0),
		       COALESCE(v.note,''),to_char(v.created_at,'YYYY-MM-DD HH24:MI')
		FROM %s.bom_versions v
		WHERE v.product_id=$1
		ORDER BY v.created_at DESC, v.id DESC
	`, r.schema, r.schema), productID)
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

func listBomItems(ctx context.Context, pool *pgxpool.Pool, schema string, productID int64) ([]bomItemRow, float64, error) {
	q := fmt.Sprintf(`
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
	rows, err := pool.Query(ctx, q, productID)
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
