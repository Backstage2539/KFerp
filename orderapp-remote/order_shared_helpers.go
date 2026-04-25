package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	salesdomain "orderapp/internal/domain/sales"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func parseNum(s string) (*string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	if _, err := strconv.ParseFloat(s, 64); err != nil {
		return nil, err
	}
	return &s, nil
}

func parseFee(v string) (float64, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, err
	}
	return f, nil
}

func calcOutsourceTotal(req interface {
	GetMaterial() string
	GetRoast() string
	GetPackaging() string
	GetManual() string
	GetTax() string
	GetOther() string
}) (float64, [6]float64, error) {
	material, err := parseFee(req.GetMaterial())
	if err != nil {
		return 0, [6]float64{}, fmt.Errorf("invalid outsource_material_fee")
	}
	roast, err := parseFee(req.GetRoast())
	if err != nil {
		return 0, [6]float64{}, fmt.Errorf("invalid outsource_roast_fee")
	}
	packaging, err := parseFee(req.GetPackaging())
	if err != nil {
		return 0, [6]float64{}, fmt.Errorf("invalid outsource_packaging_fee")
	}
	manual, err := parseFee(req.GetManual())
	if err != nil {
		return 0, [6]float64{}, fmt.Errorf("invalid outsource_manual_fee")
	}
	tax, err := parseFee(req.GetTax())
	if err != nil {
		return 0, [6]float64{}, fmt.Errorf("invalid outsource_tax_fee")
	}
	other, err := parseFee(req.GetOther())
	if err != nil {
		return 0, [6]float64{}, fmt.Errorf("invalid outsource_other_fee")
	}
	fees := [6]float64{material, roast, packaging, manual, tax, other}
	return material + roast + packaging + manual + tax + other, fees, nil
}

func loadOptions(ctx context.Context, pool *pgxpool.Pool, schema string, data *PageData) error {
	var err error
	if data.Customers, err = fetchOptions(ctx, pool, fmt.Sprintf("SELECT id, name FROM %s.customers WHERE active=true ORDER BY name", schema)); err != nil {
		return err
	}
	if data.Sources, err = fetchOptions(ctx, pool, fmt.Sprintf("SELECT id, name FROM %s.sources ORDER BY id", schema)); err != nil {
		return err
	}
	if data.ShipStatuses, err = fetchOptions(ctx, pool, fmt.Sprintf("SELECT id, name FROM %s.ship_statuses ORDER BY id", schema)); err != nil {
		return err
	}
	if data.PayStatuses, err = fetchOptions(ctx, pool, fmt.Sprintf("SELECT id, name FROM %s.pay_statuses ORDER BY id", schema)); err != nil {
		return err
	}
	if data.OrderTypes, err = fetchOptions(ctx, pool, fmt.Sprintf("SELECT id, name FROM %s.order_types ORDER BY id", schema)); err != nil {
		return err
	}
	if data.Products, err = fetchProducts(ctx, pool, schema); err != nil {
		return err
	}
	return nil
}

func fetchProducts(ctx context.Context, pool *pgxpool.Pool, schema string) ([]ProductOption, error) {
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

	// Load tiers for all products
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
		return out, nil // products without tiers still ok
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

func fetchOptions(ctx context.Context, pool *pgxpool.Pool, sqlstr string) ([]Option, error) {
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

func nextOrderNo(ctx context.Context, tx pgx.Tx, schema string, od time.Time) (string, error) {
	ymd := od.Format("20060102")
	prefix := "SO-" + ymd + "-"

	var maxNo int
	q := fmt.Sprintf(`
		SELECT COALESCE(MAX(CAST(right(order_no,4) AS INT)), 0)
		FROM %s.orders
		WHERE order_no LIKE $1
	`, schema)

	if err := tx.QueryRow(ctx, q, prefix+"%").Scan(&maxNo); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%04d", prefix, maxNo+1), nil
}

func getStr(s []string, i int) string {
	if i < 0 || i >= len(s) {
		return ""
	}
	return s[i]
}

func nullText(v string) any {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return v
}

func nullInt(v int64) any {
	if v <= 0 {
		return nil
	}
	return v
}
