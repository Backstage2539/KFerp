package sales

import (
	"context"
	"github.com/jackc/pgx/v5"
	salesapp "orderapp/internal/application/sales"
	productionpg "orderapp/internal/infrastructure/postgres/production"
)

func (r Repository) OrderOperationOptions(ctx context.Context, productID, bomSpecID, bomVariantID int64) ([]salesapp.OrderOperationOption, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	return productionpg.OrderOperationOptionsTx(ctx, tx, r.schema, productID, bomSpecID, bomVariantID)
}
