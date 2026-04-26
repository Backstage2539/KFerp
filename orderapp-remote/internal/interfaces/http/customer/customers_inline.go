package customer

import (
	"context"
	"fmt"
	support "orderapp/internal/interfaces/http/support"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CustomerInlineReq struct {
	Name               string `form:"name"`
	Contact            string `form:"contact"`
	Phone              string `form:"phone"`
	Address            string `form:"address"`
	DefaultSourceID    string `form:"default_source_id"`
	DefaultOrderTypeID string `form:"default_order_type_id"`
	Active             string `form:"active"`
}

func inlineUpdateCustomer(ctx context.Context, pool *pgxpool.Pool, schema, actor string, id int64, req *CustomerInlineReq) error {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return fmt.Errorf("name required")
	}
	contact := strings.TrimSpace(req.Contact)
	phone := strings.TrimSpace(req.Phone)
	address := strings.TrimSpace(req.Address)
	active := strings.TrimSpace(req.Active) != ""

	var ds *int
	if strings.TrimSpace(req.DefaultSourceID) != "" {
		v, err := strconv.Atoi(strings.TrimSpace(req.DefaultSourceID))
		if err == nil && v > 0 {
			ds = &v
		}
	}
	var dt *int
	if strings.TrimSpace(req.DefaultOrderTypeID) != "" {
		v, err := strconv.Atoi(strings.TrimSpace(req.DefaultOrderTypeID))
		if err == nil && v > 0 {
			dt = &v
		}
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// old values
	var oName, oContact, oPhone, oAddr string
	var oActive bool
	var oDS, oDT *int
	q0 := fmt.Sprintf(`SELECT name, COALESCE(contact,''), COALESCE(phone,''), COALESCE(address,''), active, default_source_id, default_order_type_id
		FROM %s.customers WHERE id=$1`, schema)
	if err := tx.QueryRow(ctx, q0, id).Scan(&oName, &oContact, &oPhone, &oAddr, &oActive, &oDS, &oDT); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("not found")
		}
		return err
	}

	q := fmt.Sprintf(`UPDATE %s.customers SET name=$2, contact=$3, phone=$4, address=$5, active=$6,
		default_source_id=$7, default_order_type_id=$8, updated_at=$9 WHERE id=$1`, schema)
	if _, err := tx.Exec(ctx, q, id, name, nullText(contact), nullText(phone), nullText(address), active, ds, dt, time.Now()); err != nil {
		return err
	}

	// audit diffs
	if oName != name {
		support.AuditInsert(ctx, pool, schema, actor, "customer", &id, "update", support.StrPtr("name"), support.StrPtr(oName), support.StrPtr(name), nil)
	}
	if oContact != contact {
		support.AuditInsert(ctx, pool, schema, actor, "customer", &id, "update", support.StrPtr("contact"), support.StrPtr(oContact), support.StrPtr(contact), nil)
	}
	if oPhone != phone {
		support.AuditInsert(ctx, pool, schema, actor, "customer", &id, "update", support.StrPtr("phone"), support.StrPtr(oPhone), support.StrPtr(phone), nil)
	}
	if oAddr != address {
		support.AuditInsert(ctx, pool, schema, actor, "customer", &id, "update", support.StrPtr("address"), support.StrPtr(oAddr), support.StrPtr(address), nil)
	}
	if oActive != active {
		support.AuditInsert(ctx, pool, schema, actor, "customer", &id, "update", support.StrPtr("active"), support.StrPtr(fmt.Sprintf("%v", oActive)), support.StrPtr(fmt.Sprintf("%v", active)), nil)
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
		support.AuditInsert(ctx, pool, schema, actor, "customer", &id, "update", support.StrPtr("default_source_id"), support.StrPtr(ods), support.StrPtr(nds), nil)
	}
	if odt != ndt {
		support.AuditInsert(ctx, pool, schema, actor, "customer", &id, "update", support.StrPtr("default_order_type_id"), support.StrPtr(odt), support.StrPtr(ndt), nil)
	}

	return tx.Commit(ctx)
}
