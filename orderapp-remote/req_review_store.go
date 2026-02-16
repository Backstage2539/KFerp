package main

import (
	"context"
	"fmt"
	"strings"
	"time"

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
	// update review row status
	if err := updateReqStatus(ctx, pool, schema, "req_review", reviewCode, status); err != nil {
		return err
	}
	if status != "done" {
		return nil
	}
	// read pr_code/title and cascade
	var prCode, title string
	q := fmt.Sprintf(`SELECT pr_code, title FROM %s.req_review WHERE code=$1`, schema)
	if err := pool.QueryRow(ctx, q, reviewCode).Scan(&prCode, &title); err != nil {
		return err
	}
	prCode = strings.TrimSpace(prCode)
	title = strings.TrimSpace(title)
	if prCode != "" {
		// cascade PR status to done
		q2 := fmt.Sprintf(`UPDATE %s.req_product SET status='done' WHERE code=$1`, schema)
		if _, err := pool.Exec(ctx, q2, prCode); err != nil {
			return err
		}
	}

	// after acceptance done: append log + push to git (best-effort but surface errors)
	if err := gitSyncAcceptance(ctx, GitAcceptanceEntry{
		TimeUnix:   time.Now().Unix(),
		ReviewCode: reviewCode,
		PRCode:     prCode,
		Title:      title,
		Actor:      actor,
	}); err != nil {
		return err
	}
	return nil
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
