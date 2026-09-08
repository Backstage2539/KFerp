package sales

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	salesdomain "orderapp/internal/domain/sales"
)

func requireCourierOrderTx(ctx context.Context, tx pgx.Tx, schema string, orderID int64) error {
	var method string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(to_jsonb(o)->>'ship_method','') FROM %s.orders o WHERE id=$1 FOR UPDATE`, schema), orderID).Scan(&method); err != nil {
		return err
	}
	if salesdomain.IsNonCourierShipMethod(method) {
		return fmt.Errorf("自提或本地送货订单不能使用快递发货或回填单号")
	}
	return nil
}
