package customerportal

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	salesapp "orderapp/internal/application/sales"
	productionpg "orderapp/internal/infrastructure/postgres/production"
)

func (r Repository) LoadMallOperationOptions(ctx context.Context, customerID, mallProductID int64) ([]salesapp.OrderOperationOption, error) {
	if customerID <= 0 || mallProductID <= 0 {
		return nil, fmt.Errorf("商城商品不可用")
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var productID, specID int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT m.product_id,m.bom_spec_id FROM %[1]s.mall_products m JOIN %[1]s.products p ON p.id=m.product_id WHERE m.id=$1 AND m.status='published' AND p.active=true AND %s`, r.schema, mallProductPublicCatalogSQL("p")), mallProductID).Scan(&productID, &specID)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("商城商品不可用")
	}
	if err != nil {
		return nil, err
	}
	identity, err := r.resolveMallOutputIdentity(ctx, tx, productID, specID, 0)
	if err != nil {
		return nil, err
	}
	return productionpg.OrderOperationOptionsTx(ctx, tx, r.schema, identity.ProductID, identity.BomSpecID, identity.BomVariantID)
}
