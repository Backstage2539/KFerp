package main

import (
	"context"
	"fmt"

	catalogapp "orderapp/internal/application/catalog"

	"github.com/jackc/pgx/v5/pgxpool"
)

type postgresCatalogRepository struct {
	pool   *pgxpool.Pool
	schema string
}

func (r postgresCatalogRepository) ListProducts(ctx context.Context) ([]catalogapp.Product, error) {
	ps, err := fetchProducts(ctx, r.pool, r.schema)
	if err != nil {
		return nil, err
	}
	return catalogProductsFromOptions(ps), nil
}

func (r postgresCatalogRepository) GetProduct(ctx context.Context, id int64) (*catalogapp.Product, error) {
	p, err := fetchProductByID(ctx, r.pool, r.schema, id)
	if err != nil || p == nil {
		return nil, err
	}
	out := catalogProductFromOption(*p)
	return &out, nil
}

func (r postgresCatalogRepository) ReplacePriceTiers(ctx context.Context, cmd catalogapp.ReplacePriceTiersCommand) error {
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

	if _, err := tx.Exec(ctx, fmt.Sprintf("DELETE FROM %s.product_price_tiers WHERE product_id=$1", r.schema), cmd.ProductID); err != nil {
		return err
	}
	ins := fmt.Sprintf("INSERT INTO %s.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) VALUES ($1,$2,$3,$4,true)", r.schema)
	for _, tier := range cmd.Tiers {
		if _, err := tx.Exec(ctx, ins, cmd.ProductID, tier.MinLb, tier.MaxLb, tier.PriceLb); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	auditInsert(ctx, r.pool, r.schema, cmd.Actor, "product", &cmd.ProductID, "update", strPtrStr("price_tiers"), nil, strPtrStr(fmt.Sprintf("%d", len(cmd.Tiers))), AuditMeta{"product_id": cmd.ProductID, "tier_count": len(cmd.Tiers)})
	return nil
}

func catalogProductFromOption(p ProductOption) catalogapp.Product {
	out := catalogapp.Product{ID: p.ID, Name: p.Name, DefaultPrice: p.DefaultPrice}
	out.Tiers = make([]catalogapp.PriceTier, 0, len(p.Tiers))
	for _, t := range p.Tiers {
		out.Tiers = append(out.Tiers, catalogapp.PriceTier{ID: t.ID, MinLb: t.MinLb, MaxLb: t.MaxLb, PriceLb: t.PriceLb})
	}
	return out
}

func catalogProductsFromOptions(products []ProductOption) []catalogapp.Product {
	out := make([]catalogapp.Product, 0, len(products))
	for _, p := range products {
		out = append(out, catalogProductFromOption(p))
	}
	return out
}
