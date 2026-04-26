package support

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func OperationLogMiddleware(pool *pgxpool.Pool, schema string) echo.MiddlewareFunc {
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
	for _, prefix := range []string{"/vue-shell/assets/"} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return path == "/favicon.ico"
}

type operationLogExecer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

type OperationLogEntry struct {
	Actor      string
	EmployeeID *int64
	Method     string
	Path       string
	Route      string
	Query      string
	Status     int
	DurationMS int64
	IP         string
	UserAgent  string
	Referer    string
	Error      string
}

type OperationLogger struct {
	exec   operationLogExecer
	schema string
}

func NewOperationLogger(exec operationLogExecer, schema string) OperationLogger {
	return OperationLogger{exec: exec, schema: schema}
}

func (l OperationLogger) Log(ctx context.Context, entry OperationLogEntry) error {
	q := fmt.Sprintf(`INSERT INTO %s.operation_logs(actor, employee_id, method, path, route, query, status, duration_ms, ip, user_agent, referer, error)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, l.schema)
	_, err := l.exec.Exec(ctx, q,
		entry.Actor,
		entry.EmployeeID,
		entry.Method,
		entry.Path,
		entry.Route,
		entry.Query,
		entry.Status,
		entry.DurationMS,
		entry.IP,
		entry.UserAgent,
		entry.Referer,
		entry.Error,
	)
	return err
}

func writeOperationLog(c echo.Context, pool *pgxpool.Pool, schema string, status int, duration time.Duration, handlerErr error) {
	req := c.Request()
	actor := ActorOf(c)
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

	_ = NewOperationLogger(pool, schema).Log(req.Context(), OperationLogEntry{
		Actor:      actor,
		EmployeeID: employeeID,
		Method:     method,
		Path:       path,
		Route:      route,
		Query:      query,
		Status:     status,
		DurationMS: durationMS,
		IP:         c.RealIP(),
		UserAgent:  req.UserAgent(),
		Referer:    req.Referer(),
		Error:      errText,
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
