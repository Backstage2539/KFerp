package appmain

import (
	"bufio"
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func parseRequirementsToOldPR(md string) []string {
	// Minimal parser: treat markdown headings and list items as candidates.
	// We prefer stable output over perfect semantics.
	lines := strings.Split(md, "\n")
	out := make([]string, 0)
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		// skip code fences
		if strings.HasPrefix(ln, "```") {
			continue
		}
		// headings and list items
		if strings.HasPrefix(ln, "#") {
			t := strings.TrimSpace(strings.TrimLeft(ln, "#"))
			if t != "" {
				out = append(out, t)
			}
			continue
		}
		if strings.HasPrefix(ln, "-") || strings.HasPrefix(ln, "*") {
			t := strings.TrimSpace(strings.TrimLeft(ln, "-*"))
			if t != "" {
				out = append(out, t)
			}
			continue
		}
	}
	// de-dup while preserving order
	seen := map[string]bool{}
	uniq := make([]string, 0, len(out))
	for _, t := range out {
		if seen[t] {
			continue
		}
		seen[t] = true
		uniq = append(uniq, t)
	}
	return uniq
}

func parseAcceptanceTestsToOldUT(md string) []string {
	// Parse markdown checkboxes: - [ ] xxx / - [x] xxx
	s := bufio.NewScanner(strings.NewReader(md))
	out := make([]string, 0)
	re := regexp.MustCompile(`^[-*]\s*\[[ xX]\]\s*(.+)$`)
	for s.Scan() {
		ln := strings.TrimSpace(s.Text())
		m := re.FindStringSubmatch(ln)
		if m == nil {
			continue
		}
		t := strings.TrimSpace(m[1])
		if t != "" {
			out = append(out, t)
		}
	}
	// de-dup
	seen := map[string]bool{}
	uniq := make([]string, 0, len(out))
	for _, t := range out {
		if seen[t] {
			continue
		}
		seen[t] = true
		uniq = append(uniq, t)
	}
	return uniq
}

func insertOldRows(ctx context.Context, pool *pgxpool.Pool, schema, table, prefix string, titles []string) (inserted int, err error) {
	// Insert as done; code like old-001 / OLD-001
	for i, t := range titles {
		code := fmt.Sprintf("%s-%03d", prefix, i+1)
		q := fmt.Sprintf(`INSERT INTO %s.%s(code,title,status,assignee)
			VALUES($1,$2,'done','')
			ON CONFLICT (code) DO NOTHING`, schema, table)
		ct, e := pool.Exec(ctx, q, code, t)
		if e != nil {
			return inserted, e
		}
		inserted += int(ct.RowsAffected())
	}
	return inserted, nil
}

func migrateRequirements(ctx context.Context, pool *pgxpool.Pool, schema string, md string) (int, error) {
	items := parseRequirementsToOldPR(md)
	return insertOldRows(ctx, pool, schema, "req_product", "old", items)
}

func migrateAcceptanceTests(ctx context.Context, pool *pgxpool.Pool, schema string, md string) (int, error) {
	items := parseAcceptanceTestsToOldUT(md)
	return insertOldRows(ctx, pool, schema, "req_unit", "OLD", items)
}
