package sales

import (
	"context"
	"fmt"
	"testing"
)

func TestResolveOrderBeanListPublicationTxRejectsCustomerResale(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %s.bean_list_publications (
			id BIGSERIAL PRIMARY KEY,
			publication_purpose TEXT NOT NULL,
			list_type TEXT NOT NULL,
			version_no TEXT NOT NULL,
			status TEXT NOT NULL,
			owner_type TEXT NOT NULL,
			owner_key TEXT NOT NULL DEFAULT '',
			published_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`, schema)); err != nil {
		t.Fatalf("create bean_list_publications: %v", err)
	}
	var resaleID, factoryID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.bean_list_publications(publication_purpose,list_type,version_no,status,owner_type,owner_key)
		VALUES('customer_resale','commercial','RESALE-V1','published','official','')
		RETURNING id
	`, schema)).Scan(&resaleID); err != nil {
		t.Fatalf("insert customer_resale publication: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.bean_list_publications(publication_purpose,list_type,version_no,status,owner_type,owner_key)
		VALUES('factory_supply','commercial','SUPPLY-V1','published','official','')
		RETURNING id
	`, schema)).Scan(&factoryID); err != nil {
		t.Fatalf("insert factory_supply publication: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	repo := NewRepository(pool, schema)
	if _, _, err := repo.resolveOrderBeanListPublicationTx(ctx, tx, 0, resaleID, "commercial"); err == nil {
		t.Fatal("explicit customer_resale publication must be rejected for ERP order pricing")
	}
	gotID, gotVersion, err := repo.resolveOrderBeanListPublicationTx(ctx, tx, 0, factoryID, "commercial")
	if err != nil {
		t.Fatalf("resolve factory_supply publication: %v", err)
	}
	if gotID != factoryID || gotVersion != "SUPPLY-V1" {
		t.Fatalf("factory_supply publication = %d/%q, want %d/SUPPLY-V1", gotID, gotVersion, factoryID)
	}
}
