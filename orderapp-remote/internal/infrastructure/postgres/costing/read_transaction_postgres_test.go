package costing

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestQueryCostingReadRowsUsesLocalJITSettingAndReleasesSingleConnection(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required")
	}

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	repo := NewRepository(pool, "public")

	if _, err := pool.Exec(context.Background(), "SET jit = on"); err != nil {
		t.Fatalf("enable session JIT: %v", err)
	}
	assertSettings := func(ctx context.Context) error {
		return repo.queryCostingReadRows(ctx, "SELECT current_setting('jit'), current_setting('transaction_read_only')", nil, func(rows pgx.Rows) error {
			if !rows.Next() {
				return rows.Err()
			}
			var jit, readOnly string
			if err := rows.Scan(&jit, &readOnly); err != nil {
				return err
			}
			if jit != "off" || readOnly != "on" {
				return fmt.Errorf("transaction settings jit=%s read_only=%s", jit, readOnly)
			}
			return nil
		})
	}

	for attempt := 0; attempt < 3; attempt++ {
		if err := assertSettings(context.Background()); err != nil {
			t.Fatalf("read attempt %d: %v", attempt+1, err)
		}
	}
	var sessionJIT string
	if err := pool.QueryRow(context.Background(), "SHOW jit").Scan(&sessionJIT); err != nil {
		t.Fatal(err)
	}
	if sessionJIT != "on" {
		t.Fatalf("session JIT = %s, want on after local read transaction", sessionJIT)
	}

	if err := repo.queryCostingReadRows(context.Background(), "SELECT * FROM table_that_does_not_exist", nil, func(pgx.Rows) error { return nil }); err == nil {
		t.Fatal("missing-table query unexpectedly succeeded")
	}
	assertPoolAvailable(t, pool)

	cancelled, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := repo.queryCostingReadRows(cancelled, "SELECT pg_sleep(5)", nil, func(rows pgx.Rows) error {
		for rows.Next() {
		}
		return rows.Err()
	}); err == nil {
		t.Fatal("cancelled query unexpectedly succeeded")
	}
	assertPoolAvailable(t, pool)
}

func TestQueryCostingReadRowsSerializesConcurrentWorkWithSingleConnection(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required")
	}

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	repo := NewRepository(pool, "public")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	errs := make(chan error, 6)
	for worker := 0; worker < cap(errs); worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- repo.queryCostingReadRows(ctx, "SELECT current_setting('jit') FROM (SELECT pg_sleep(0.01)) delay", nil, func(rows pgx.Rows) error {
				if !rows.Next() {
					return rows.Err()
				}
				var jit string
				if err := rows.Scan(&jit); err != nil {
					return err
				}
				if jit != "off" {
					return fmt.Errorf("transaction JIT = %s", jit)
				}
				return nil
			})
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func assertPoolAvailable(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var value int
	if err := pool.QueryRow(ctx, "SELECT 1").Scan(&value); err != nil {
		t.Fatalf("pool unavailable after read transaction: %v", err)
	}
	if value != 1 {
		t.Fatalf("pool check = %d, want 1", value)
	}
}
