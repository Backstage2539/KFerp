package materials

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	postgrescompany "orderapp/internal/infrastructure/postgres/company"
	postgresmaterials "orderapp/internal/infrastructure/postgres/materials"
	supporthttp "orderapp/internal/interfaces/http/support"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMain(m *testing.M) {
	if root, err := findModuleRootForTests(); err == nil {
		_ = os.Chdir(root)
	}
	os.Exit(m.Run())
}

func findModuleRootForTests() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		next := filepath.Dir(dir)
		if next == dir {
			return "", os.ErrNotExist
		}
		dir = next
	}
}

func newProductionFlowTestDB(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()

	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required for materials API tests")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	schema := fmt.Sprintf("test_materials_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	if err := postgrescompany.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("company EnsureSchema: %v", err)
	}
	if err := supporthttp.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("support EnsureSchema: %v", err)
	}
	if err := postgresmaterials.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("materials EnsureSchema: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	})
	return pool, schema
}
