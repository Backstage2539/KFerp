package customer

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CustomerRow struct {
	ID                 int64   `json:"id"`
	Name               string  `json:"name"`
	Contact            *string `json:"contact"`
	Phone              *string `json:"phone"`
	Address            *string `json:"address"`
	Active             bool    `json:"active"`
	DefaultSourceID    *int    `json:"default_source_id"`
	DefaultOrderTypeID *int    `json:"default_order_type_id"`
	Updated            string  `json:"updated"`
}

type CustomerDashboard struct {
	TotalOrders     int
	UnpaidOrders    int
	UnshippedOrders int
	InProduction    int
	InShipping      int
	Completed       int
}

type CustomerEditData struct {
	ID                 int64
	Name               string
	RawName            string
	Contact            string
	Phone              string
	Address            string
	DefaultSourceID    string
	DefaultOrderTypeID string
	Active             bool
}

type CustomerUpsertRequest struct {
	Name               string `form:"name"`
	RawName            string `form:"raw_name"`
	Contact            string `form:"contact"`
	Phone              string `form:"phone"`
	Address            string `form:"address"`
	DefaultSourceID    string `form:"default_source_id"`
	DefaultOrderTypeID string `form:"default_order_type_id"`
	Active             string `form:"active"`
}

func fetchCustomers(ctx context.Context, pool *pgxpool.Pool, schema, q string, limit, offset int) (rows []CustomerRow, hasNext bool, err error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	args := make([]any, 0)
	w := ""
	if strings.TrimSpace(q) != "" {
		w = "WHERE name ILIKE $1 OR COALESCE(contact,'') ILIKE $1 OR COALESCE(phone,'') ILIKE $1 OR COALESCE(address,'') ILIKE $1"
		args = append(args, "%"+strings.TrimSpace(q)+"%")
	}
	// fetch one more row to determine hasNext
	args = append(args, limit+1, offset)
	limArg := len(args) - 1
	offArg := len(args)

	sql := fmt.Sprintf(`
		SELECT id, name, contact, phone, address, active, default_source_id, default_order_type_id,
			to_char(updated_at,'YYYY-MM-DD HH24:MI') AS updated
		FROM %s.customers
		%s
		ORDER BY active DESC, name ASC
		LIMIT $%d OFFSET $%d
	`, schema, w, limArg, offArg)

	qr, err := pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, false, err
	}
	defer qr.Close()

	out := make([]CustomerRow, 0)
	for qr.Next() {
		var r CustomerRow
		if err := qr.Scan(&r.ID, &r.Name, &r.Contact, &r.Phone, &r.Address, &r.Active, &r.DefaultSourceID, &r.DefaultOrderTypeID, &r.Updated); err != nil {
			return nil, false, err
		}
		out = append(out, r)
	}
	if err := qr.Err(); err != nil {
		return nil, false, err
	}

	if len(out) > limit {
		hasNext = true
		out = out[:limit]
	}
	return out, hasNext, nil
}

func fetchCustomerByID(ctx context.Context, pool *pgxpool.Pool, schema string, id int64) (*CustomerEditData, error) {
	q := fmt.Sprintf(`SELECT id, name, COALESCE(raw_name,''), COALESCE(contact,''), COALESCE(phone,''), COALESCE(address,''),
		COALESCE(default_source_id::text,''), COALESCE(default_order_type_id::text,''), active
		FROM %s.customers WHERE id=$1`, schema)
	var d CustomerEditData
	err := pool.QueryRow(ctx, q, id).Scan(&d.ID, &d.Name, &d.RawName, &d.Contact, &d.Phone, &d.Address, &d.DefaultSourceID, &d.DefaultOrderTypeID, &d.Active)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &d, nil
}
