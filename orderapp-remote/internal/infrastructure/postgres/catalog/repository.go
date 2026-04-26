package catalog

import (
	"context"
	"fmt"

	catalogapp "orderapp/internal/application/catalog"
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

func (r Repository) ListProducts(ctx context.Context) ([]catalogapp.Product, error) {
	ps, err := postgresinfra.FetchProducts(ctx, r.pool, r.schema)
	if err != nil {
		return nil, err
	}
	return catalogProductsFromOptions(ps), nil
}

func (r Repository) GetProduct(ctx context.Context, id int64) (*catalogapp.Product, error) {
	p, err := fetchProductByID(ctx, r.pool, r.schema, id)
	if err != nil || p == nil {
		return nil, err
	}
	out := catalogProductFromOption(*p)
	return &out, nil
}

func (r Repository) ReplacePriceTiers(ctx context.Context, cmd catalogapp.ReplacePriceTiersCommand) error {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	roastLevel := catalogdomain.NormalizeRoastLevel(cmd.RoastLevel)
	yieldRate := catalogdomain.ResolveYieldRate(roastLevel, 0.8)
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.products
		SET roast_level=$2, retail_price_100g=$3, retail_price_200g=$4, retail_price_227g=$5, retail_price_250g=$6
		WHERE id=$1`, r.schema), cmd.ProductID, roastLevel, cmd.RetailPrice100G, cmd.RetailPrice200G, cmd.RetailPrice227G, cmd.RetailPrice250G); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.product_bom(product_id,yield_rate,updated_at)
		VALUES($1,$2,now())
		ON CONFLICT (product_id) DO UPDATE SET yield_rate=excluded.yield_rate, updated_at=now()`, r.schema), cmd.ProductID, yieldRate); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf("DELETE FROM %s.product_price_tiers WHERE product_id=$1", r.schema), cmd.ProductID); err != nil {
		return err
	}
	ins := fmt.Sprintf(`INSERT INTO %s.product_price_tiers
		(product_id, spec_g, min_qty_units, max_qty_units, price_per_unit, min_qty_lb, max_qty_lb, price_per_lb, active)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,true)`, r.schema)
	for _, tier := range cmd.Tiers {
		specG := tier.SpecG
		if specG <= 0 {
			specG = 454
		}
		minLb := tier.MinQty * float64(specG) / 454.0
		var maxLb *float64
		if tier.MaxQty != nil {
			v := *tier.MaxQty * float64(specG) / 454.0
			maxLb = &v
		}
		priceLb := tier.UnitPrice * 454.0 / float64(specG)
		if _, err := tx.Exec(ctx, ins, cmd.ProductID, specG, tier.MinQty, tier.MaxQty, tier.UnitPrice, minLb, maxLb, priceLb); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, cmd.Actor, "product", &cmd.ProductID, "update", postgresinfra.StrPtr("price_tiers"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", len(cmd.Tiers))), postgresinfra.AuditMeta{
		"product_id":        cmd.ProductID,
		"roast_level":       roastLevel,
		"yield_rate":        yieldRate,
		"tier_count":        len(cmd.Tiers),
		"retail_price_100g": cmd.RetailPrice100G,
		"retail_price_200g": cmd.RetailPrice200G,
		"retail_price_227g": cmd.RetailPrice227G,
		"retail_price_250g": cmd.RetailPrice250G,
	})
	return nil
}

func fetchProductByID(ctx context.Context, pool *pgxpool.Pool, schema string, id int64) (*postgresinfra.ProductOption, error) {
	ps, err := postgresinfra.FetchProducts(ctx, pool, schema)
	if err != nil {
		return nil, err
	}
	for i := range ps {
		if ps[i].ID == id {
			return &ps[i], nil
		}
	}
	var p postgresinfra.ProductOption
	err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT id, name, COALESCE(roast_level,''), default_price,
		COALESCE(retail_price_100g, 0), COALESCE(retail_price_200g, 0),
		COALESCE(retail_price_227g, default_price, 0), COALESCE(retail_price_250g, 0)
		FROM %s.products WHERE id=$1`, schema), id).Scan(&p.ID, &p.Name, &p.RoastLevel, &p.DefaultPrice, &p.RetailPrice100G, &p.RetailPrice200G, &p.RetailPrice227G, &p.RetailPrice250G)
	if err != nil {
		return nil, nil
	}
	return &p, nil
}

func catalogProductFromOption(p postgresinfra.ProductOption) catalogapp.Product {
	out := catalogapp.Product{ID: p.ID, Name: p.Name, RoastLevel: p.RoastLevel, DefaultPrice: p.DefaultPrice, RetailPrice100G: p.RetailPrice100G, RetailPrice200G: p.RetailPrice200G, RetailPrice227G: p.RetailPrice227G, RetailPrice250G: p.RetailPrice250G}
	out.Tiers = make([]catalogapp.PriceTier, 0, len(p.Tiers))
	for _, t := range p.Tiers {
		out.Tiers = append(out.Tiers, catalogapp.PriceTier{ID: t.ID, SpecG: t.SpecG, MinQty: t.MinQty, MaxQty: t.MaxQty, UnitPrice: t.UnitPrice})
	}
	return out
}

func catalogProductsFromOptions(products []postgresinfra.ProductOption) []catalogapp.Product {
	out := make([]catalogapp.Product, 0, len(products))
	for _, p := range products {
		out = append(out, catalogProductFromOption(p))
	}
	return out
}
