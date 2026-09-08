package production

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMaterialComponentSourceOwnerMustEqualMaterialMasterPostgres(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	schema := fmt.Sprintf("pr639_component_owner_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE %s.materials(id BIGINT PRIMARY KEY,owner_customer_id BIGINT NOT NULL DEFAULT 0,deprecated_at TIMESTAMPTZ)`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.materials(id,owner_customer_id) VALUES(10,0),(20,74),(30,75)`, schema)); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, tt := range []struct {
		name       string
		materialID int64
		ownerID    int64
		wantErr    bool
	}{
		{name: "factory material uses factory owner", materialID: 10},
		{name: "customer A material uses A owner", materialID: 20, ownerID: 74},
		{name: "customer A material rejects factory owner", materialID: 20, wantErr: true},
		{name: "customer A material rejects customer B", materialID: 20, ownerID: 75, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMaterialComponentSourceOwnerTx(ctx, tx, schema, "material", tt.materialID, tt.ownerID)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error=%v wantErr=%v", err, tt.wantErr)
			}
		})
	}
}
