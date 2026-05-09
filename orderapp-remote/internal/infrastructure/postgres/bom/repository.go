package bom

import (
	"context"
	"fmt"
	"strings"

	bomapp "orderapp/internal/application/bom"
	bomdomain "orderapp/internal/domain/bom"
	catalogdomain "orderapp/internal/domain/catalog"
	postgresinfra "orderapp/internal/infrastructure/postgres"

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
		if err := rows.Scan(&item.ProductID, &item.CustomerID, &item.Product, &item.RoastLevel, &fallback, &item.Status, &item.ItemCount, &item.UpdatedAt); err != nil {
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
	rows, err := r.pool.Query(ctx, "SELECT id, name, COALESCE(customer_id,0) FROM "+r.schema+".products WHERE active=true ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]bomapp.Option, 0)
	for rows.Next() {
		var opt bomapp.Option
		if err := rows.Scan(&opt.ID, &opt.Name, &opt.CustomerID); err != nil {
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

func (r Repository) SyncProductYield(ctx context.Context, productID int64) error {
	var roastLevel string
	if err := r.pool.QueryRow(ctx, "SELECT COALESCE(roast_level,'') FROM "+r.schema+".products WHERE id=$1", productID).Scan(&roastLevel); err != nil {
		return fmt.Errorf("product not found")
	}
	yieldRate := catalogdomain.ResolveYieldRate(roastLevel, 0.8)
	q := "INSERT INTO " + r.schema + ".product_bom(product_id,yield_rate,status,updated_at) VALUES($1,$2,'active',now()) ON CONFLICT (product_id) DO UPDATE SET yield_rate=excluded.yield_rate, status='active', updated_at=now()"
	_, err := r.pool.Exec(ctx, q, productID, yieldRate)
	return err
}

func (r Repository) DeactivateBom(ctx context.Context, productID int64) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var roastLevel string
	if err := tx.QueryRow(ctx, "SELECT COALESCE(roast_level,'') FROM "+r.schema+".products WHERE id=$1", productID).Scan(&roastLevel); err != nil {
		return fmt.Errorf("product not found")
	}
	yieldRate := catalogdomain.ResolveYieldRate(roastLevel, 0.8)
	if _, err := tx.Exec(ctx, "INSERT INTO "+r.schema+".product_bom(product_id,yield_rate,status,updated_at) VALUES($1,$2,'inactive',now()) ON CONFLICT (product_id) DO UPDATE SET status='inactive', updated_at=now()", productID, yieldRate); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) SaveItem(ctx context.Context, cmd bomapp.SaveItemCommand) error {
	if err := r.SyncProductYield(ctx, cmd.ProductID); err != nil {
		return err
	}
	_, total, err := listBomItems(ctx, r.pool, r.schema, cmd.ProductID)
	if err != nil {
		return err
	}

	var oldRatio float64
	_ = r.pool.QueryRow(ctx,
		"SELECT COALESCE(ratio_pct,0) FROM "+r.schema+".product_bom_items WHERE product_id=$1 AND material_id=$2",
		cmd.ProductID, cmd.MaterialID).Scan(&oldRatio)
	if total-oldRatio+cmd.RatioPct > 100.0001 {
		return fmt.Errorf("ratio sum exceed 100%%")
	}

	q := "INSERT INTO " + r.schema + ".product_bom_items(product_id,material_id,ratio_pct,updated_at) VALUES($1,$2,$3,now()) ON CONFLICT (product_id,material_id) DO UPDATE SET ratio_pct=excluded.ratio_pct, updated_at=now()"
	_, err = r.pool.Exec(ctx, q, cmd.ProductID, cmd.MaterialID, cmd.RatioPct)
	return err
}

func (r Repository) DeleteItem(ctx context.Context, cmd bomapp.DeleteItemCommand) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM "+r.schema+".product_bom_items WHERE id=$1", cmd.ID)
	return err
}

func (r Repository) SaveBagSpecMapping(ctx context.Context, cmd bomapp.SaveBagSpecMappingCommand) error {
	return saveBagSpecMapping(ctx, r.pool, r.schema, cmd.SpecG, cmd.MaterialID)
}

func (r Repository) DeleteBagSpecMapping(ctx context.Context, specG int64) error {
	return deleteBagSpecMapping(ctx, r.pool, r.schema, specG)
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
		INSERT INTO %s.bom_version_items(version_id,material_id,ratio_pct)
		SELECT $1,material_id,ratio_pct
		FROM %s.product_bom_items
		WHERE product_id=$2
		ORDER BY id
	`, r.schema, r.schema), versionID, cmd.ProductID); err != nil {
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

func (r Repository) ActivateVersion(ctx context.Context, versionID int64) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var productID int64
	var yieldRate float64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT product_id,COALESCE(yield_rate,0.8) FROM %s.bom_versions WHERE id=$1 FOR UPDATE`, r.schema), versionID).Scan(&productID, &yieldRate); err != nil {
		return fmt.Errorf("bom version not found")
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.bom_versions SET status='disabled' WHERE product_id=$1 AND id<>$2`, r.schema), productID, versionID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.bom_versions SET status='active',activated_at=now() WHERE id=$1`, r.schema), versionID); err != nil {
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
		INSERT INTO %s.product_bom_items(product_id,material_id,ratio_pct,updated_at)
		SELECT $1,material_id,ratio_pct,now()
		FROM %s.bom_version_items
		WHERE version_id=$2
		ORDER BY id
	`, r.schema, r.schema), productID, versionID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type bomItemRow struct {
	ID           int64
	MaterialID   int64
	MaterialName string
	RatioPct     float64
}

func listBomItems(ctx context.Context, pool *pgxpool.Pool, schema string, productID int64) ([]bomItemRow, float64, error) {
	q := fmt.Sprintf(`
		SELECT bi.id, bi.material_id, COALESCE(m.name,''), bi.ratio_pct
		FROM %s.product_bom_items bi
		LEFT JOIN %s.materials m ON m.id=bi.material_id
		WHERE bi.product_id=$1
		ORDER BY m.name, bi.id
	`, schema, schema)
	rows, err := pool.Query(ctx, q, productID)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]bomItemRow, 0)
	total := 0.0
	for rows.Next() {
		var row bomItemRow
		if err := rows.Scan(&row.ID, &row.MaterialID, &row.MaterialName, &row.RatioPct); err != nil {
			return nil, 0, err
		}
		total += row.RatioPct
		out = append(out, row)
	}
	return out, total, rows.Err()
}

func saveBagSpecMapping(ctx context.Context, pool *pgxpool.Pool, schema string, specG, materialID int64) error {
	if specG <= 0 {
		return fmt.Errorf("spec_g required")
	}
	if materialID <= 0 {
		return fmt.Errorf("material_id required")
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

func bomItemsToApp(rows []bomItemRow) []bomapp.Item {
	out := make([]bomapp.Item, 0, len(rows))
	for _, row := range rows {
		out = append(out, bomapp.Item{
			ID:           row.ID,
			MaterialID:   row.MaterialID,
			MaterialName: row.MaterialName,
			RatioPct:     row.RatioPct,
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
