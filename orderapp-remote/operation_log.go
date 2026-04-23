package main

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func ensureAuditTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.audit_logs (
	id BIGSERIAL PRIMARY KEY,
	ts TIMESTAMPTZ NOT NULL DEFAULT now(),
	actor TEXT NOT NULL DEFAULT '',
	entity_type TEXT NOT NULL DEFAULT '',
	entity_id BIGINT NULL,
	action TEXT NOT NULL DEFAULT '',
	field TEXT NULL,
	old_value TEXT NULL,
	new_value TEXT NULL,
	meta JSONB NULL
);
CREATE INDEX IF NOT EXISTS audit_logs_ts_idx ON %s.audit_logs(ts DESC);
CREATE INDEX IF NOT EXISTS audit_logs_entity_idx ON %s.audit_logs(entity_type, entity_id, ts DESC);
CREATE INDEX IF NOT EXISTS audit_logs_actor_idx ON %s.audit_logs(actor, ts DESC);

CREATE TABLE IF NOT EXISTS %s.order_audit_logs (
	id BIGSERIAL PRIMARY KEY,
	order_id BIGINT NOT NULL,
	actor TEXT NOT NULL DEFAULT '',
	field TEXT NOT NULL DEFAULT '',
	old_value TEXT NULL,
	new_value TEXT NULL,
	changed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS order_audit_logs_order_idx ON %s.order_audit_logs(order_id, id DESC);

CREATE TABLE IF NOT EXISTS %s.operation_logs (
	id BIGSERIAL PRIMARY KEY,
	ts TIMESTAMPTZ NOT NULL DEFAULT now(),
	actor TEXT NOT NULL DEFAULT '',
	employee_id BIGINT NULL,
	method TEXT NOT NULL DEFAULT '',
	path TEXT NOT NULL DEFAULT '',
	route TEXT NOT NULL DEFAULT '',
	query TEXT NOT NULL DEFAULT '',
	status INTEGER NOT NULL DEFAULT 0,
	duration_ms BIGINT NOT NULL DEFAULT 0,
	ip TEXT NOT NULL DEFAULT '',
	user_agent TEXT NOT NULL DEFAULT '',
	referer TEXT NOT NULL DEFAULT '',
	error TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS operation_logs_ts_idx ON %s.operation_logs(ts DESC);
CREATE INDEX IF NOT EXISTS operation_logs_actor_idx ON %s.operation_logs(actor, ts DESC);
CREATE INDEX IF NOT EXISTS operation_logs_route_idx ON %s.operation_logs(method, route, ts DESC);
`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema)
	_, err := pool.Exec(ctx, q)
	return err
}

func operationLogMiddleware(pool *pgxpool.Pool, schema string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if shouldSkipOperationLog(c) {
				return next(c)
			}

			start := time.Now()
			err := next(c)
			status := c.Response().Status
			if err != nil {
				if httpErr, ok := err.(*echo.HTTPError); ok {
					status = httpErr.Code
				} else if status < 400 {
					status = 500
				}
			}
			if status == 0 {
				status = 200
			}

			writeOperationLog(c, pool, schema, status, time.Since(start), err)
			return err
		}
	}
}

func shouldSkipOperationLog(c echo.Context) bool {
	path := strings.TrimSpace(c.Request().URL.Path)
	if path == "" {
		return false
	}
	for _, prefix := range []string{"/bom-react/assets/", "/vue-shell/assets/"} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return path == "/favicon.ico"
}

func writeOperationLog(c echo.Context, pool *pgxpool.Pool, schema string, status int, duration time.Duration, handlerErr error) {
	req := c.Request()
	actor := actorOf(c)
	employeeID := contextEmployeeID(c)
	method := req.Method
	path := req.URL.Path
	route := c.Path()
	query := sanitizedRawQuery(req.URL.Query())
	durationMS := duration.Milliseconds()
	errText := ""
	if handlerErr != nil {
		errText = handlerErr.Error()
	}

	q := fmt.Sprintf(`INSERT INTO %s.operation_logs(actor, employee_id, method, path, route, query, status, duration_ms, ip, user_agent, referer, error)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, schema)
	_, _ = pool.Exec(req.Context(), q,
		actor,
		employeeID,
		method,
		path,
		route,
		query,
		status,
		durationMS,
		c.RealIP(),
		req.UserAgent(),
		req.Referer(),
		errText,
	)

	field := strings.TrimSpace(method + " " + path)
	auditInsert(req.Context(), pool, schema, actor, "operation", nil, "request", strPtrStr(field), nil, strPtrStr(strconv.Itoa(status)), AuditMeta{
		"duration_ms": durationMS,
		"employee_id": employeeID,
		"error":       errText,
		"ip":          c.RealIP(),
		"query":       query,
		"route":       route,
		"user_agent":  req.UserAgent(),
	})
}

func contextEmployeeID(c echo.Context) *int64 {
	v := c.Get("employee_id")
	switch n := v.(type) {
	case int64:
		if n > 0 {
			return &n
		}
	case int:
		if n > 0 {
			out := int64(n)
			return &out
		}
	}
	return nil
}

func sanitizedRawQuery(values url.Values) string {
	if len(values) == 0 {
		return ""
	}
	out := url.Values{}
	for k, vs := range values {
		key := strings.ToLower(strings.TrimSpace(k))
		redact := strings.Contains(key, "password") || strings.Contains(key, "pass") || strings.Contains(key, "token") || strings.Contains(key, "code")
		for _, v := range vs {
			if redact {
				out.Add(k, "REDACTED")
			} else {
				out.Add(k, v)
			}
		}
	}
	return out.Encode()
}
