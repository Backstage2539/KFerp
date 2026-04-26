package postgres

import (
	"context"
	"fmt"

	salesdomain "orderapp/internal/domain/sales"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Option struct {
	ID   int64
	Name string
}

type ProductTierOption struct {
	ID        int64
	SpecG     int64
	MinQty    float64
	MaxQty    *float64
	UnitPrice float64
}

type ProductOption struct {
	ID              int64
	Name            string
	RoastLevel      string
	DefaultPrice    float64
	RetailPrice100G float64
	RetailPrice200G float64
	RetailPrice227G float64
	RetailPrice250G float64
	RetailSpecs     []int64
	Tiers           []ProductTierOption
}

func FetchOptions(ctx context.Context, pool *pgxpool.Pool, sqlstr string) ([]Option, error) {
	rows, err := pool.Query(ctx, sqlstr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Option, 0)
	for rows.Next() {
		var o Option
		if err := rows.Scan(&o.ID, &o.Name); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func FetchProducts(ctx context.Context, pool *pgxpool.Pool, schema string) ([]ProductOption, error) {
	sqlstr := fmt.Sprintf(`SELECT id, name, COALESCE(roast_level,''), default_price,
		COALESCE(retail_price_100g, 0),
		COALESCE(retail_price_200g, 0),
		COALESCE(retail_price_227g, default_price, 0),
		COALESCE(retail_price_250g, 0)
		FROM %s.products WHERE active=true ORDER BY name`, schema)
	rows, err := pool.Query(ctx, sqlstr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ProductOption, 0)
	for rows.Next() {
		var p ProductOption
		if err := rows.Scan(&p.ID, &p.Name, &p.RoastLevel, &p.DefaultPrice, &p.RetailPrice100G, &p.RetailPrice200G, &p.RetailPrice227G, &p.RetailPrice250G); err != nil {
			return nil, err
		}
		p.RetailSpecs = salesdomain.RetailAvailableSpecs(salesdomain.RetailSpecPrices{
			Price100G: p.RetailPrice100G,
			Price200G: p.RetailPrice200G,
			Price227G: p.RetailPrice227G,
			Price250G: p.RetailPrice250G,
		})
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	tierSQL := fmt.Sprintf(`
		SELECT id, product_id,
		       COALESCE(NULLIF(spec_g,0), 454),
		       COALESCE(min_qty_units, min_qty_lb),
		       COALESCE(max_qty_units, max_qty_lb),
		       COALESCE(price_per_unit, price_per_lb)
		FROM %s.product_price_tiers
		WHERE active=true
		ORDER BY product_id, COALESCE(NULLIF(spec_g,0), 454), COALESCE(min_qty_units, min_qty_lb)
	`, schema)
	trs, err := pool.Query(ctx, tierSQL)
	if err != nil {
		return out, nil
	}
	defer trs.Close()

	tierMap := map[int64][]ProductTierOption{}
	for trs.Next() {
		var tid, pid int64
		var specG int64
		var min float64
		var max *float64
		var price float64
		if err := trs.Scan(&tid, &pid, &specG, &min, &max, &price); err != nil {
			return nil, err
		}
		tierMap[pid] = append(tierMap[pid], ProductTierOption{ID: tid, SpecG: specG, MinQty: min, MaxQty: max, UnitPrice: price})
	}
	if err := trs.Err(); err != nil {
		return nil, err
	}

	for i := range out {
		out[i].Tiers = tierMap[out[i].ID]
	}
	return out, nil
}
