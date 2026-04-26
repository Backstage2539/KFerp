package bom

import (
	"context"
	"fmt"

	bomapp "orderapp/internal/application/bom"
	catalogdomain "orderapp/internal/domain/catalog"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5/pgxpool"
)

type postgresBomRepository struct {
	pool   *pgxpool.Pool
	schema string
}

func (r postgresBomRepository) List(ctx context.Context) ([]bomapp.ListItem, error) {
	q := fmt.Sprintf(`
		SELECT
			p.id,
			p.name,
			COALESCE(p.roast_level, ''),
			COALESCE(b.yield_rate, 0.8),
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
		if err := rows.Scan(&item.ProductID, &item.Product, &item.RoastLevel, &fallback, &item.ItemCount, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.YieldRate = catalogdomain.ResolveYieldRate(item.RoastLevel, fallback)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r postgresBomRepository) Detail(ctx context.Context, productID int64) (bomapp.Detail, error) {
	var productName string
	var roastLevel string
	var yieldRate float64
	var updatedAt string
	err := r.pool.QueryRow(ctx,
		"SELECT COALESCE(p.name,''), COALESCE(p.roast_level,''), COALESCE(b.yield_rate,0.8), COALESCE(to_char(b.updated_at,'YYYY-MM-DD HH24:MI'),'-') "+
			"FROM "+r.schema+".products p LEFT JOIN "+r.schema+".product_bom b ON b.product_id=p.id "+
			"WHERE p.id=$1", productID).Scan(&productName, &roastLevel, &yieldRate, &updatedAt)
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
		Items:       bomItemsToApp(items),
		TotalRatio:  total,
		UpdatedAt:   updatedAt,
	}, nil
}

func (r postgresBomRepository) Products(ctx context.Context) ([]bomapp.Option, error) {
	opts, err := postgresinfra.FetchOptions(ctx, r.pool, "SELECT id, name FROM "+r.schema+".products WHERE active=true ORDER BY name")
	if err != nil {
		return nil, err
	}
	return bomOptionsToApp(opts), nil
}

func (r postgresBomRepository) Materials(ctx context.Context) ([]bomapp.Option, error) {
	opts, err := postgresinfra.FetchOptions(ctx, r.pool, "SELECT id, name FROM "+r.schema+".materials ORDER BY name")
	if err != nil {
		return nil, err
	}
	return bomOptionsToApp(opts), nil
}

func (r postgresBomRepository) BagSpecMappings(ctx context.Context) ([]bomapp.BagSpecMapping, error) {
	rows, err := listBagSpecMappings(ctx, r.pool, r.schema)
	if err != nil {
		return nil, err
	}
	out := make([]bomapp.BagSpecMapping, 0, len(rows))
	for _, row := range rows {
		out = append(out, bomapp.BagSpecMapping{SpecG: row.SpecG, MaterialID: row.MaterialID, MaterialName: row.MaterialName})
	}
	return out, nil
}

func (r postgresBomRepository) SyncProductYield(ctx context.Context, productID int64) error {
	var roastLevel string
	if err := r.pool.QueryRow(ctx, "SELECT COALESCE(roast_level,'') FROM "+r.schema+".products WHERE id=$1", productID).Scan(&roastLevel); err != nil {
		return fmt.Errorf("product not found")
	}
	yieldRate := catalogdomain.ResolveYieldRate(roastLevel, 0.8)
	q := "INSERT INTO " + r.schema + ".product_bom(product_id,yield_rate,updated_at) VALUES($1,$2,now()) ON CONFLICT (product_id) DO UPDATE SET yield_rate=excluded.yield_rate, updated_at=now()"
	_, err := r.pool.Exec(ctx, q, productID, yieldRate)
	return err
}

func (r postgresBomRepository) SaveItem(ctx context.Context, cmd bomapp.SaveItemCommand) error {
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

func (r postgresBomRepository) DeleteItem(ctx context.Context, cmd bomapp.DeleteItemCommand) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM "+r.schema+".product_bom_items WHERE id=$1", cmd.ID)
	return err
}

func (r postgresBomRepository) SaveBagSpecMapping(ctx context.Context, cmd bomapp.SaveBagSpecMappingCommand) error {
	return saveBagSpecMapping(ctx, r.pool, r.schema, cmd.SpecG, cmd.MaterialID)
}

func (r postgresBomRepository) DeleteBagSpecMapping(ctx context.Context, specG int64) error {
	return deleteBagSpecMapping(ctx, r.pool, r.schema, specG)
}

func bomItemsToApp(rows []BomItemRow) []bomapp.Item {
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

func bomOptionsToApp(opts []postgresinfra.Option) []bomapp.Option {
	out := make([]bomapp.Option, 0, len(opts))
	for _, opt := range opts {
		out = append(out, bomapp.Option{ID: opt.ID, Name: opt.Name})
	}
	return out
}
