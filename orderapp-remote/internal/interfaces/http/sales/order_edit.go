package sales

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

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id, COALESCE(spec,''), COALESCE(qty,0), COALESCE(unit_price,0)
		FROM %s.order_items
		WHERE order_id=$1
		ORDER BY line_no, id
	`, schema), id)
	if err != nil {
		return err
	}
	defer rows.Close()

	type rowItem struct {
		id        int64
		specG     int64
		qty       float64
		unitPrice float64
	}
	old := make([]rowItem, 0)
	for rows.Next() {
		var rid int64
		var spec string
		var qty, up float64
		if err := rows.Scan(&rid, &spec, &qty, &up); err != nil {
			return err
		}
		old = append(old, rowItem{id: rid, specG: parseSpecG(spec), qty: qty, unitPrice: up})
	}
	if err := rows.Err(); err != nil {
		return err
	}

	qtyMap := map[int64]float64{}
	upMap := map[int64]float64{}
	for i := 0; i < len(req.ItemID); i++ {
		rid, err := strconv.ParseInt(strings.TrimSpace(req.ItemID[i]), 10, 64)
		if err != nil || rid <= 0 {
			continue
		}
		q, _ := strconv.ParseFloat(strings.TrimSpace(getStr(req.Qty, i)), 64)
		if q < 0 {
			q = 0
		}
		u, _ := strconv.ParseFloat(strings.TrimSpace(getStr(req.UnitPrice, i)), 64)
		if u < 0 {
			u = 0
		}
		qtyMap[rid] = q
		upMap[rid] = u
	}

	totalAmt := 0.0
	for _, it := range old {
		q := it.qty
		if v, ok := qtyMap[it.id]; ok {
			q = v
		}
		u := it.unitPrice
		if v, ok := upMap[it.id]; ok {
			u = v
		}
		if q <= 0 {
			q = 0
		}
		if u < 0 {
			u = 0
		}
		qtyLb := (float64(it.specG) * q) / 454.0
		line := qtyLb * u
		totalAmt += line
		if _, err := tx.Exec(ctx, fmt.Sprintf("UPDATE %s.order_items SET qty=$2,unit_price=$3,line_total=$4,price_overridden=true WHERE id=$1", schema), it.id, q, u, line); err != nil {
			return err
		}
	}

	outsourceTotal, outsourceFees, err := calcOutsourceTotal(req)
	if err != nil {
		return err
	}
	grand0 := totalAmt + ship - disc + outsourceTotal
	grandTotal, roundingAmt := applyRoundToInt(grand0, round)

	q := fmt.Sprintf(`
		UPDATE %s.orders
		SET order_date=$2,
			customer_id=$3,
			source_id=$4,
			order_type_id=$5,
			pay_status_id=$6,
			ship_status_id=$7,
			notes=$8,
			total_amount=$9,
			shipping_amount=$10,
			discount_amount=$11,
			round_to_int=$12,
			rounding_amount=$13,
			grand_total=$14,
			express_fee=$15,
			outsource_material_fee=$16,
			outsource_roast_fee=$17,
			outsource_packaging_fee=$18,
			outsource_manual_fee=$19,
			outsource_tax_fee=$20,
			outsource_other_fee=$21,
			outsource_total_fee=$22
		WHERE id=$1
	`, schema)

	if _, err := tx.Exec(ctx, q,
		id,
		orderDate,
		req.CustomerID,
		nullInt(req.SourceID),
		nullInt(req.OrderTypeID),
		nullInt(req.PayStatusID),
		nullInt(req.ShipStatusID),
		nullText(req.Notes),
		totalAmt,
		ship,
		disc,
		round,
		roundingAmt,
		grandTotal,
		nullText(req.ExpressFee),
		outsourceFees[0],
		outsourceFees[1],
		outsourceFees[2],
		outsourceFees[3],
		outsourceFees[4],
		outsourceFees[5],
		outsourceTotal,
	); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
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
