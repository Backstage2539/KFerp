package appmain

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AuditRow struct {
	ChangedAt string  `json:"changed_at"`
	Actor     string  `json:"actor"`
	Field     string  `json:"field"`
	OldValue  *string `json:"old_value"`
	NewValue  *string `json:"new_value"`
}

func fetchAuditLogs(ctx context.Context, pool *pgxpool.Pool, schema string, orderID int64, limit int) ([]AuditRow, error) {
	if limit <= 0 {
		limit = 50
	}

	payMap, _ := fetchIDNameMap(ctx, pool, fmt.Sprintf("SELECT id, name FROM %s.pay_statuses", schema))
	shipMap, _ := fetchIDNameMap(ctx, pool, fmt.Sprintf("SELECT id, name FROM %s.ship_statuses", schema))

	sql := fmt.Sprintf(`
		SELECT
			to_char(changed_at,'YYYY-MM-DD HH24:MI:SS') AS changed_at,
			actor, field, old_value, new_value
		FROM %s.order_audit_logs
		WHERE order_id=$1
		ORDER BY id DESC
		LIMIT $2
	`, schema)
	rows, err := pool.Query(ctx, sql, orderID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]AuditRow, 0)
	for rows.Next() {
		var r AuditRow
		if err := rows.Scan(&r.ChangedAt, &r.Actor, &r.Field, &r.OldValue, &r.NewValue); err != nil {
			return nil, err
		}

		switch r.Field {
		case "pay_status_id":
			r.OldValue = idTextToLabel(r.OldValue, payMap)
			r.NewValue = idTextToLabel(r.NewValue, payMap)
		case "ship_status_id":
			r.OldValue = idTextToLabel(r.OldValue, shipMap)
			r.NewValue = idTextToLabel(r.NewValue, shipMap)
		}

		out = append(out, r)
	}
	return out, rows.Err()
}

func fetchIDNameMap(ctx context.Context, pool *pgxpool.Pool, sqlstr string) (map[int64]string, error) {
	rows, err := pool.Query(ctx, sqlstr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := map[int64]string{}
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		m[id] = name
	}
	return m, rows.Err()
}

func idTextToLabel(v *string, m map[int64]string) *string {
	if v == nil {
		return nil
	}
	s := strings.TrimSpace(*v)
	if s == "" {
		return nil
	}
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return v
	}
	name, ok := m[id]
	if !ok {
		return v
	}
	out := fmt.Sprintf("%s(%d)", name, id)
	return &out
}
