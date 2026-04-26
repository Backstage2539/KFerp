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

type CustomersPageData struct {
	Q          string
	Rows       []CustomerRow
	Sources    []Option
	OrderTypes []Option
	Limit      int
	Offset     int
	Page       int
	HasPrev    bool
	HasNext    bool
	Error      string
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
	Sources            []Option
	OrderTypes         []Option
	Assets             []CustomerAsset
	Dash               CustomerDashboard
	From               string
	Ok                 bool
	Active             bool
	Error              string
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

func upsertCustomer(ctx context.Context, pool *pgxpool.Pool, schema string, actor string, id *int64, req *CustomerUpsertRequest) (newID int64, err error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return 0, fmt.Errorf("name required")
	}
	raw := strings.TrimSpace(req.RawName)
	contact := strings.TrimSpace(req.Contact)
	phone := strings.TrimSpace(req.Phone)
	address := strings.TrimSpace(req.Address)
	active := strings.TrimSpace(req.Active) != ""

	var ds *int
	if strings.TrimSpace(req.DefaultSourceID) != "" {
		if v, err := strconv.Atoi(strings.TrimSpace(req.DefaultSourceID)); err == nil && v > 0 {
			ds = &v
		}
	}
	var dt *int
	if strings.TrimSpace(req.DefaultOrderTypeID) != "" {
		if v, err := strconv.Atoi(strings.TrimSpace(req.DefaultOrderTypeID)); err == nil && v > 0 {
			dt = &v
		}
	}

	if raw == "" {
		raw = name
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return 0, err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if id == nil {
		q := fmt.Sprintf(`INSERT INTO %s.customers(name, raw_name, contact, phone, address, active, default_source_id, default_order_type_id, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8, now(), now()) RETURNING id`, schema)
		if err := tx.QueryRow(ctx, q, name, raw, nullText(contact), nullText(phone), nullText(address), active, ds, dt).Scan(&newID); err != nil {
			return 0, err
		}
		auditInsert(ctx, pool, schema, actor, "customer", &newID, "create", nil, nil, nil, AuditMeta{"name": name})
	} else {
		newID = *id
		// load old
		var oName, oRaw, oContact, oPhone, oAddr string
		var oActive bool
		var oDS, oDT *int
		q0 := fmt.Sprintf(`SELECT name, COALESCE(raw_name,''), COALESCE(contact,''), COALESCE(phone,''), COALESCE(address,''), active, default_source_id, default_order_type_id FROM %s.customers WHERE id=$1`, schema)
		if err := tx.QueryRow(ctx, q0, newID).Scan(&oName, &oRaw, &oContact, &oPhone, &oAddr, &oActive, &oDS, &oDT); err != nil {
			return 0, err
		}

		q := fmt.Sprintf(`UPDATE %s.customers SET name=$2, raw_name=$3, contact=$4, phone=$5, address=$6, active=$7,
			default_source_id=$8, default_order_type_id=$9, updated_at=$10 WHERE id=$1`, schema)
		if _, err := tx.Exec(ctx, q, newID, name, raw, nullText(contact), nullText(phone), nullText(address), active, ds, dt, time.Now()); err != nil {
			return 0, err
		}

		// audit diffs
		if oName != name {
			auditInsert(ctx, pool, schema, actor, "customer", &newID, "update", strPtrStr("name"), strPtrStr(oName), strPtrStr(name), nil)
		}
		if oContact != contact {
			auditInsert(ctx, pool, schema, actor, "customer", &newID, "update", strPtrStr("contact"), strPtrStr(oContact), strPtrStr(contact), nil)
		}
		if oPhone != phone {
			auditInsert(ctx, pool, schema, actor, "customer", &newID, "update", strPtrStr("phone"), strPtrStr(oPhone), strPtrStr(phone), nil)
		}
		if oAddr != address {
			auditInsert(ctx, pool, schema, actor, "customer", &newID, "update", strPtrStr("address"), strPtrStr(oAddr), strPtrStr(address), nil)
		}
		if oActive != active {
			auditInsert(ctx, pool, schema, actor, "customer", &newID, "update", strPtrStr("active"), strPtrStr(fmt.Sprintf("%v", oActive)), strPtrStr(fmt.Sprintf("%v", active)), nil)
		}
		ods := ""
		odt := ""
		nds := ""
		ndt := ""
		if oDS != nil {
			ods = fmt.Sprintf("%d", *oDS)
		}
		if oDT != nil {
			odt = fmt.Sprintf("%d", *oDT)
		}
		if ds != nil {
			nds = fmt.Sprintf("%d", *ds)
		}
		if dt != nil {
			ndt = fmt.Sprintf("%d", *dt)
		}
		if ods != nds {
			auditInsert(ctx, pool, schema, actor, "customer", &newID, "update", strPtrStr("default_source_id"), strPtrStr(ods), strPtrStr(nds), nil)
		}
		if odt != ndt {
			auditInsert(ctx, pool, schema, actor, "customer", &newID, "update", strPtrStr("default_order_type_id"), strPtrStr(odt), strPtrStr(ndt), nil)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return newID, nil
}
