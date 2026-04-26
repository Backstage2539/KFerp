package appmain

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type InlineUpdateRequest struct {
	OrderTypeID     string `form:"order_type_id"`
	PayStatusID     string `form:"pay_status_id"`
	ShipStatusID    string `form:"ship_status_id"`
	ProcessStatusID string `form:"process_status_id"`
	Notes           string `form:"notes"`
}

func inlineUpdateOrder(ctx context.Context, pool *pgxpool.Pool, schema string, orderID int64, actor string, req *InlineUpdateRequest) error {
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "unknown"
	}

	// Load current values
	var curOrderType *int64
	var curPay *int64
	var curShip *int64
	var curProc *int64
	var curNotes *string
	q := fmt.Sprintf("SELECT order_type_id, pay_status_id, ship_status_id, process_status_id, notes FROM %s.orders WHERE id=$1", schema)
	if err := pool.QueryRow(ctx, q, orderID).Scan(&curOrderType, &curPay, &curShip, &curProc, &curNotes); err != nil {
		return err
	}

	parseID := func(s string) (*int64, error) {
		s = strings.TrimSpace(s)
		if s == "" {
			return nil, nil
		}
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, err
		}
		if id <= 0 {
			return nil, nil
		}
		return &id, nil
	}

	nextOrderType, err := parseID(req.OrderTypeID)
	if err != nil {
		return fmt.Errorf("invalid order_type_id")
	}
	nextPay, err := parseID(req.PayStatusID)
	if err != nil {
		return fmt.Errorf("invalid pay_status_id")
	}
	nextShip, err := parseID(req.ShipStatusID)
	if err != nil {
		return fmt.Errorf("invalid ship_status_id")
	}
	nextProc, err := parseID(req.ProcessStatusID)
	if err != nil {
		return fmt.Errorf("invalid process_status_id")
	}
	nextNotes := strings.TrimSpace(req.Notes)
	var nextNotesPtr *string
	if nextNotes != "" {
		nextNotesPtr = &nextNotes
	}

	changed := false

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	// (no-op)

	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Apply updates (only if changed)
	upd := fmt.Sprintf(`UPDATE %s.orders SET order_type_id=$2, pay_status_id=$3, ship_status_id=$4, process_status_id=$5, notes=$6 WHERE id=$1`, schema)
	if !eqIntPtr(curOrderType, nextOrderType) || !eqIntPtr(curPay, nextPay) || !eqIntPtr(curShip, nextShip) || !eqIntPtr(curProc, nextProc) || !eqStrPtr(curNotes, nextNotesPtr) {
		if _, err := tx.Exec(ctx, upd, orderID, nextOrderType, nextPay, nextShip, nextProc, nextNotesPtr); err != nil {
			return err
		}
		changed = true
	}

	// Audit logs (legacy + unified)
	ins := fmt.Sprintf(`INSERT INTO %s.order_audit_logs(order_id, actor, field, old_value, new_value) VALUES ($1,$2,$3,$4,$5)`, schema)
	if !eqIntPtr(curOrderType, nextOrderType) {
		oldS, newS := intPtrToStr(curOrderType), intPtrToStr(nextOrderType)
		if _, err := tx.Exec(ctx, ins, orderID, actor, "order_type_id", oldS, newS); err != nil {
			return err
		}
		if err := auditInsertTx(ctx, tx, schema, actor, "order", &orderID, "update", strPtrStr("order_type_id"), oldS, newS, AuditMeta{"order_id": orderID}); err != nil {
			return err
		}
	}
	if !eqIntPtr(curPay, nextPay) {
		oldS, newS := intPtrToStr(curPay), intPtrToStr(nextPay)
		if _, err := tx.Exec(ctx, ins, orderID, actor, "pay_status_id", oldS, newS); err != nil {
			return err
		}
		if err := auditInsertTx(ctx, tx, schema, actor, "order", &orderID, "update", strPtrStr("pay_status_id"), oldS, newS, AuditMeta{"order_id": orderID}); err != nil {
			return err
		}
	}
	if !eqIntPtr(curShip, nextShip) {
		oldS, newS := intPtrToStr(curShip), intPtrToStr(nextShip)
		if _, err := tx.Exec(ctx, ins, orderID, actor, "ship_status_id", oldS, newS); err != nil {
			return err
		}
		if err := auditInsertTx(ctx, tx, schema, actor, "order", &orderID, "update", strPtrStr("ship_status_id"), oldS, newS, AuditMeta{"order_id": orderID}); err != nil {
			return err
		}
	}
	if !eqIntPtr(curProc, nextProc) {
		oldS, newS := intPtrToStr(curProc), intPtrToStr(nextProc)
		if _, err := tx.Exec(ctx, ins, orderID, actor, "process_status_id", oldS, newS); err != nil {
			return err
		}
		if err := auditInsertTx(ctx, tx, schema, actor, "order", &orderID, "update", strPtrStr("process_status_id"), oldS, newS, AuditMeta{"order_id": orderID}); err != nil {
			return err
		}
	}
	if !eqStrPtr(curNotes, nextNotesPtr) {
		oldS, newS := strPtr(curNotes), strPtr(nextNotesPtr)
		if _, err := tx.Exec(ctx, ins, orderID, actor, "notes", oldS, newS); err != nil {
			return err
		}
		if err := auditInsertTx(ctx, tx, schema, actor, "order", &orderID, "update", strPtrStr("notes"), oldS, newS, AuditMeta{"order_id": orderID}); err != nil {
			return err
		}
	}

	if !changed {
		return nil
	}
	return tx.Commit(ctx)
}

func eqIntPtr(a, b *int64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func eqStrPtr(a, b *string) bool {
	av := ""
	if a != nil {
		av = *a
	}
	bv := ""
	if b != nil {
		bv = *b
	}
	return av == bv
}

func intPtrToStr(p *int64) *string {
	if p == nil {
		return nil
	}
	s := fmt.Sprintf("%d", *p)
	return &s
}

func strPtr(p *string) *string {
	if p == nil {
		return nil
	}
	s := *p
	return &s
}

var _ pgx.Tx
