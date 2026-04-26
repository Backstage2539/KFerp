package support

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func updateReqStatus(ctx context.Context, pool *pgxpool.Pool, schema, table, code, status string) error {
	code = strings.TrimSpace(code)
	status = strings.TrimSpace(status)
	if code == "" {
		return fmt.Errorf("code required")
	}
	if status == "" {
		return fmt.Errorf("status required")
	}
	switch status {
	case "todo", "doing", "done":
	default:
		return fmt.Errorf("invalid status")
	}
	q := fmt.Sprintf(`UPDATE %s.%s SET status=$2 WHERE code=$1`, schema, table)
	ct, err := pool.Exec(ctx, q, code, status)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("not found")
	}
	return nil
}

func updateReviewStatusAndCascade(ctx context.Context, pool *pgxpool.Pool, schema, actor, reviewCode, status string) error {
	// NOTE: git auto-push on acceptance has been removed (per Van request).
	// actor is currently unused but kept for future audit improvements.
	_ = actor

	reviewCode = strings.TrimSpace(reviewCode)
	status = strings.TrimSpace(status)
	if reviewCode == "" {
		return fmt.Errorf("code required")
	}

	// update review row status
	if err := updateReqStatus(ctx, pool, schema, "req_review", reviewCode, status); err != nil {
		return err
	}
	if status != "done" {
		return nil
	}

	// read pr_code and cascade PR status to done
	var prCode string
	q := fmt.Sprintf(`SELECT pr_code FROM %s.req_review WHERE code=$1`, schema)
	if err := pool.QueryRow(ctx, q, reviewCode).Scan(&prCode); err != nil {
		return err
	}
	prCode = strings.TrimSpace(prCode)
	if prCode == "" {
		return nil
	}
	q2 := fmt.Sprintf(`UPDATE %s.req_product SET status='done' WHERE code=$1`, schema)
	_, err := pool.Exec(ctx, q2, prCode)
	return err
}

func createReviewRow(ctx context.Context, pool *pgxpool.Pool, schema, code, prCode, title, status, assignee string) error {
	code = strings.TrimSpace(code)
	prCode = strings.TrimSpace(prCode)
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
		n, err := nextReqCodeForTable(ctx, pool, schema, "req_review")
		if err != nil {
			return err
		}
		code = n
	}
	// Ensure title includes pr_code prefix if provided
	displayTitle := title
	if prCode != "" && !strings.HasPrefix(displayTitle, prCode) {
		displayTitle = prCode + " " + displayTitle
	}
	q := fmt.Sprintf(`INSERT INTO %s.req_review (code, pr_code, title, status, assignee)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (code) DO NOTHING`, schema)
	ct, err := pool.Exec(ctx, q, code, prCode, displayTitle, status, assignee)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("code already exists: %s", code)
	}
	return nil
}
