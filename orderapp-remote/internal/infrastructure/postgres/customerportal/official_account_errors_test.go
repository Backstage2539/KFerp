package customerportal

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	app "orderapp/internal/application/customerportal"
	"os"
	"testing"
	"time"
)

func TestOfficialAccountContextDistinguishesMissingUserAndDatabaseFailure(t *testing.T) {
	dsn := os.Getenv("ORDERAPP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated test database required")
	}
	ctx := context.Background()
	p, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	schema := fmt.Sprintf("test_official_errors_%d", time.Now().UnixNano())
	if _, err = p.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer p.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	repo := NewRepository(p, schema)
	_, err = repo.OfficialAccountContext(ctx, 1, 1, time.Now())
	if err == nil || errors.Is(err, app.ErrCustomerBindingNotFound) {
		t.Fatalf("database error was disguised: %v", err)
	}
	if _, err = p.Exec(ctx, "CREATE TABLE "+schema+".mini_users(id bigint,openid text,active boolean)"); err != nil {
		t.Fatal(err)
	}
	_, err = repo.OfficialAccountContext(ctx, 1, 1, time.Now())
	if !errors.Is(err, app.ErrCustomerBindingNotFound) {
		t.Fatalf("missing account not classified: %v", err)
	}
}
