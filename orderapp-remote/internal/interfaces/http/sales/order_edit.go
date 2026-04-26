package sales

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OrderEditItem struct {
	ItemID      int64
	LineNo      int
	ProductID   int64
	Product     string
	Spec        string
	Qty         string
	Unit        string
	UnitPrice   string
	LineTotal   string
	PriceTierID int64
}

type OrderEditData struct {
	ID             int64
	OrderNo        string
	OrderDate      string
	CustomerID     int64
	SourceID       int64
	OrderTypeID    int64
	PayStatusID    int64
	ShipStatusID   int64
	ShipMethod     string
	ShipTrackingNo string
	Notes          string

	TotalAmount           string
	ShippingAmount        string
	DiscountAmount        string
	RoundToInt            bool
	RoundingAmount        string
	GrandTotal            string
	ExpressFee            string
	OutsourceMaterialFee  string
	OutsourceRoastFee     string
	OutsourcePackagingFee string
	OutsourceManualFee    string
	OutsourceTaxFee       string
	OutsourceOtherFee     string
	OutsourceTotalFee     string

	IsVoid     bool
	VoidedAt   *string
	VoidReason *string

	Items []OrderEditItem
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
			COALESCE(o.ship_method,'') as ship_method,
			COALESCE(o.ship_tracking_no,'') as ship_tracking_no,
			COALESCE(o.notes,'') as notes,
			COALESCE(o.total_amount,0) as total_amount,
			COALESCE(o.shipping_amount,0) as shipping_amount,
			COALESCE(o.discount_amount,0) as discount_amount,
			COALESCE(o.round_to_int,false) as round_to_int,
			COALESCE(o.rounding_amount,0) as rounding_amount,
			COALESCE(o.grand_total,0) as grand_total,
			COALESCE(o.express_fee,'') as express_fee,
			COALESCE(o.outsource_material_fee,0) as outsource_material_fee,
			COALESCE(o.outsource_roast_fee,0) as outsource_roast_fee,
			COALESCE(o.outsource_packaging_fee,0) as outsource_packaging_fee,
			COALESCE(o.outsource_manual_fee,0) as outsource_manual_fee,
			COALESCE(o.outsource_tax_fee,0) as outsource_tax_fee,
			COALESCE(o.outsource_other_fee,0) as outsource_other_fee,
			COALESCE(o.outsource_total_fee,0) as outsource_total_fee,
			o.is_void,
			CASE WHEN o.voided_at IS NULL THEN NULL ELSE to_char(o.voided_at, 'YYYY-MM-DD HH24:MI:SS') END AS voided_at,
			o.void_reason
		FROM %s.orders o
		WHERE o.id=$1
	`, schema)

	var d OrderEditData
	var totalAmt, shipAmt, discAmt, roundAmt, grandAmt float64
	var outsourceMaterial, outsourceRoast, outsourcePackaging, outsourceManual, outsourceTax, outsourceOther, outsourceTotal float64
	err := pool.QueryRow(ctx, q, id).Scan(
		&d.ID,
		&d.OrderNo,
		&d.OrderDate,
		&d.CustomerID,
		&d.SourceID,
		&d.OrderTypeID,
		&d.PayStatusID,
		&d.ShipStatusID,
		&d.ShipMethod,
		&d.ShipTrackingNo,
		&d.Notes,
		&totalAmt,
		&shipAmt,
		&discAmt,
		&d.RoundToInt,
		&roundAmt,
		&grandAmt,
		&d.ExpressFee,
		&outsourceMaterial,
		&outsourceRoast,
		&outsourcePackaging,
		&outsourceManual,
		&outsourceTax,
		&outsourceOther,
		&outsourceTotal,
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
	d.TotalAmount = fmt.Sprintf("%.2f", totalAmt)
	d.ShippingAmount = fmt.Sprintf("%.2f", shipAmt)
	d.DiscountAmount = fmt.Sprintf("%.2f", discAmt)
	d.RoundingAmount = fmt.Sprintf("%.2f", roundAmt)
	d.GrandTotal = fmt.Sprintf("%.2f", grandAmt)
	d.OutsourceMaterialFee = fmt.Sprintf("%.2f", outsourceMaterial)
	d.OutsourceRoastFee = fmt.Sprintf("%.2f", outsourceRoast)
	d.OutsourcePackagingFee = fmt.Sprintf("%.2f", outsourcePackaging)
	d.OutsourceManualFee = fmt.Sprintf("%.2f", outsourceManual)
	d.OutsourceTaxFee = fmt.Sprintf("%.2f", outsourceTax)
	d.OutsourceOtherFee = fmt.Sprintf("%.2f", outsourceOther)
	d.OutsourceTotalFee = fmt.Sprintf("%.2f", outsourceTotal)

	itemsQ := fmt.Sprintf(`
		SELECT oi.id, oi.line_no,
			COALESCE(oi.product_id,0),
			COALESCE(p.name,''),
			COALESCE(oi.spec,''),
			COALESCE(oi.qty,0),
			COALESCE(oi.unit,''),
			COALESCE(oi.unit_price,0),
			COALESCE(oi.line_total,0),
			COALESCE(oi.price_tier_id,0)
		FROM %s.order_items oi
		LEFT JOIN %s.products p ON p.id=oi.product_id
		WHERE oi.order_id=$1
		ORDER BY oi.line_no, oi.id
	`, schema, schema)
	rows, err := pool.Query(ctx, itemsQ, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	d.Items = make([]OrderEditItem, 0)
	for rows.Next() {
		var it OrderEditItem
		var qty, unitPrice, lineTotal float64
		if err := rows.Scan(&it.ItemID, &it.LineNo, &it.ProductID, &it.Product, &it.Spec, &qty, &it.Unit, &unitPrice, &lineTotal, &it.PriceTierID); err != nil {
			return nil, err
		}
		it.Qty = trimFloatZero(qty)
		it.UnitPrice = fmt.Sprintf("%.2f", unitPrice)
		it.LineTotal = fmt.Sprintf("%.2f", lineTotal)
		d.Items = append(d.Items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &d, nil
}

func parseSpecG(spec string) int64 {
	s := strings.TrimSpace(strings.ToLower(spec))
	s = strings.TrimSuffix(s, "g")
	n, _ := strconv.ParseInt(s, 10, 64)
	if n > 0 {
		return n
	}
	return 0
}

func trimFloatZero(v float64) string {
	s := strconv.FormatFloat(v, 'f', 4, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" {
		return "0"
	}
	return s
}
