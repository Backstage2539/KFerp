package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ReqRow struct {
	ID        int64
	Code      string
	PRCode    string
	Title     string
	Status    string
	Assignee  string
	Evidence  string
	CreatedAt time.Time
}

func ensureReqTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	ddl := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.req_product (
			id BIGSERIAL PRIMARY KEY,
			code TEXT NOT NULL UNIQUE,
			title TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'todo',
			assignee TEXT NOT NULL DEFAULT '',
			evidence TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.req_dev (
			id BIGSERIAL PRIMARY KEY,
			code TEXT NOT NULL UNIQUE,
			title TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'todo',
			assignee TEXT NOT NULL DEFAULT '',
			evidence TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.req_unit (
			id BIGSERIAL PRIMARY KEY,
			code TEXT NOT NULL UNIQUE,
			title TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'todo',
			assignee TEXT NOT NULL DEFAULT '',
			evidence TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.req_api (
			id BIGSERIAL PRIMARY KEY,
			code TEXT NOT NULL UNIQUE,
			title TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'todo',
			assignee TEXT NOT NULL DEFAULT '',
			evidence TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.req_review (
			id BIGSERIAL PRIMARY KEY,
			code TEXT NOT NULL UNIQUE,
			pr_code TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'todo',
			assignee TEXT NOT NULL DEFAULT '',
			evidence TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`, schema),
	}
	for _, q := range ddl {
		if _, err := pool.Exec(ctx, q); err != nil {
			return err
		}
	}
	// Schema migration (safe, idempotent)
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.req_review ADD COLUMN IF NOT EXISTS pr_code TEXT NOT NULL DEFAULT ''`, schema))
	return nil
}

func listReqRows(ctx context.Context, pool *pgxpool.Pool, schema, table string, limit, offset int) (rowsOut []ReqRow, hasNext bool, err error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}
	cols := "id, code, title, status, assignee, evidence, created_at"
	if table == "req_review" {
		cols = "id, code, pr_code, title, status, assignee, evidence, created_at"
	}

	orderBy := "id DESC"
	// Archive ordering: put imported/archived items last.
	if table == "req_product" {
		orderBy = "(code ILIKE 'old-%') ASC, id DESC"
	}
	if table == "req_unit" {
		orderBy = "(code ILIKE 'old-%' OR code ILIKE 'OLD-%') ASC, id DESC"
	}

	q := fmt.Sprintf(`SELECT %s
		FROM %s.%s
		ORDER BY %s
		LIMIT %d OFFSET %d`, cols, schema, table, orderBy, limit+1, offset)
	r, e := pool.Query(ctx, q)
	if e != nil {
		return nil, false, e
	}
	defer r.Close()

	out := make([]ReqRow, 0)
	for r.Next() {
		var row ReqRow
		if table == "req_review" {
			if e := r.Scan(&row.ID, &row.Code, &row.PRCode, &row.Title, &row.Status, &row.Assignee, &row.Evidence, &row.CreatedAt); e != nil {
				return nil, false, e
			}
		} else {
			if e := r.Scan(&row.ID, &row.Code, &row.Title, &row.Status, &row.Assignee, &row.Evidence, &row.CreatedAt); e != nil {
				return nil, false, e
			}
		}
		out = append(out, row)
	}
	if e := r.Err(); e != nil {
		return nil, false, e
	}
	if len(out) > limit {
		hasNext = true
		out = out[:limit]
	}
	return out, hasNext, nil
}

func createReqRow(ctx context.Context, pool *pgxpool.Pool, schema, table, code, title, status, assignee string) error {
	code = strings.TrimSpace(code)
	title = strings.TrimSpace(title)
	status = strings.TrimSpace(status)
	assignee = strings.TrimSpace(assignee)
	if title == "" {
		return fmt.Errorf("title required")
	}
	if status == "" {
		status = "todo"
	}
	if code == "" {
		n, err := nextReqCodeForTable(ctx, pool, schema, table)
		if err != nil {
			return err
		}
		code = n
	}
	q := fmt.Sprintf(`INSERT INTO %s.%s (code, title, status, assignee)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (code) DO NOTHING`, schema, table)
	ct, err := pool.Exec(ctx, q, code, title, status, assignee)
	if err != nil {
		return err
	}
	// if conflict (no insert), surface as error to the UI
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("code already exists: %s", code)
	}
	return nil
}

func seedReqWorkflowA(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	// One-time seed for the "A" small requirement: add top shortcuts on req pages.
	// Safe: uses ON CONFLICT DO NOTHING.
	if err := createReqRow(ctx, pool, schema, "req_product", "PR-001", "需求管理5页面增加统一顶部快捷入口/互相跳转", "todo", "VA"); err != nil {
		// ignore if already exists
		if !strings.Contains(err.Error(), "already exists") {
			return err
		}
	}
	for _, it := range []struct{
		table, code, title, status, assignee string
	}{
		{"req_dev", "DEV-001", "req_*.html 顶部增加按钮组：产品/开发/单测/API/审核互相跳转", "todo", "JJ"},
		{"req_dev", "DEV-002", "统一样式（复用现有 btn/pill 样式，移动端可用）", "todo", "JJ"},
		{"req_unit", "UT-001", "打开5个页面返回200（无500），模板渲染不报错", "todo", "JJ"},
		{"req_api", "API-001", "GET /app/req/product 返回200", "todo", "JJ"},
		{"req_api", "API-002", "GET /app/req/dev 返回200", "todo", "JJ"},
		{"req_api", "API-003", "GET /app/req/unit 返回200", "todo", "JJ"},
		{"req_api", "API-004", "GET /app/req/api 返回200", "todo", "JJ"},
		{"req_api", "API-005", "GET /app/req/review 返回200", "todo", "JJ"},
		{"req_review", "REV-001", "需求管理页面顶部入口可点击跳转；无404/500", "todo", "VA"},
	} {
		if err := createReqRow(ctx, pool, schema, it.table, it.code, it.title, it.status, it.assignee); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return err
			}
		}
	}
	return nil
}
