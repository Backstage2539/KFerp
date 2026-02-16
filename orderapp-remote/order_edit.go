package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OrderEditData struct {
	PageData PageData

	ID          int64
	OrderNo     string
	OrderDate   string
	CustomerID  int64
	SourceID    int64
	OrderTypeID int64
	PayStatusID int64
	ShipStatusID int64
	Notes       string
	ShippingAmount string
	DiscountAmount string
	RoundToInt bool
	ExpressFee string

	IsVoid     bool
	VoidedAt   *string
	VoidReason *string

	Error string
}

func fetchOrderEdit(ctx context.Context, pool *pgxpool.Pool, schema string, id int64) (*OrderEditData, error) {
	q := fmt.Sprintf(`
		SELECT
			o.id,
			o.order_no,
			to_char(o.order_date,'YYYY-MM-DD') as order_date,
			COALESCE(o.customer_id,0) as customer_id,
			COALESCE(o.source_id,0) as source_id,
			COALESCE(o.order_type_id,0) as order_type_id,
			COALESCE(o.pay_status_id,0) as pay_status_id,
			COALESCE(o.ship_status_id,0) as ship_status_id,
			COALESCE(o.notes,'') as notes,
			COALESCE(o.shipping_amount,0) as shipping_amount,
			COALESCE(o.discount_amount,0) as discount_amount,
			COALESCE(o.round_to_int,false) as round_to_int,
			COALESCE(o.express_fee,'') as express_fee,
			o.is_void,
			CASE WHEN o.voided_at IS NULL THEN NULL ELSE to_char(o.voided_at, 'YYYY-MM-DD HH24:MI:SS') END AS voided_at,
			o.void_reason
		FROM %s.orders o
		WHERE o.id=$1
	`, schema)

	var d OrderEditData
	var shipAmt, discAmt float64
	err := pool.QueryRow(ctx, q, id).Scan(
		&d.ID,
		&d.OrderNo,
		&d.OrderDate,
		&d.CustomerID,
		&d.SourceID,
		&d.OrderTypeID,
		&d.PayStatusID,
		&d.ShipStatusID,
		&d.Notes,
		&shipAmt,
		&discAmt,
		&d.RoundToInt,
		&d.ExpressFee,
		&d.IsVoid,
		&d.VoidedAt,
		&d.VoidReason,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	// store as string for inputs
	d.ShippingAmount = fmt.Sprintf("%.2f", shipAmt)
	d.DiscountAmount = fmt.Sprintf("%.2f", discAmt)
	d.PageData.Today = d.OrderDate
	return &d, nil
}

func updateOrderHeader(ctx context.Context, pool *pgxpool.Pool, schema string, id int64, req *UpdateOrderRequest) error {
	orderDate := strings.TrimSpace(req.OrderDate)
	if orderDate == "" {
		return fmt.Errorf("order_date required")
	}
	if _, err := time.Parse("2006-01-02", orderDate); err != nil {
		return fmt.Errorf("invalid order_date")
	}
	if req.CustomerID <= 0 {
		return fmt.Errorf("customer required")
	}

	ship := 0.0
	if v := strings.TrimSpace(req.ShippingAmount); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return fmt.Errorf("invalid shipping_amount")
		}
		ship = f
	}
	disc := 0.0
	if v := strings.TrimSpace(req.DiscountAmount); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return fmt.Errorf("invalid discount_amount")
		}
		disc = f
	}
	round := strings.TrimSpace(req.RoundToInt) != ""

	q := fmt.Sprintf(`
		UPDATE %s.orders
		SET order_date=$2,
			customer_id=$3,
			source_id=$4,
			order_type_id=$5,
			pay_status_id=$6,
			ship_status_id=$7,
			notes=$8,
			shipping_amount=$9,
			discount_amount=$10,
			round_to_int=$11,
			express_fee=$12
		WHERE id=$1
	`, schema)

	_, err := pool.Exec(ctx, q,
		id,
		orderDate,
		req.CustomerID,
		nullInt(req.SourceID),
		nullInt(req.OrderTypeID),
		nullInt(req.PayStatusID),
		nullInt(req.ShipStatusID),
		nullText(req.Notes),
		ship,
		disc,
		round,
		nullText(req.ExpressFee),
	)
	return err
}
