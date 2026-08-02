package sales

import (
	"context"
	"fmt"
	"testing"
)

func TestIsCurrentDefaultOrderPublicationTxMatchesCustomerAndOfficialFallbackRules(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	defer pool.Close()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %s.bean_list_publications (
			id BIGINT PRIMARY KEY,
			owner_type TEXT NOT NULL,
			owner_key TEXT NOT NULL DEFAULT '',
			list_type TEXT NOT NULL,
			status TEXT NOT NULL,
			publication_purpose TEXT NOT NULL,
			classification_template_id BIGINT NOT NULL DEFAULT 0,
			product_type_category_id BIGINT NOT NULL DEFAULT 0,
			published_at TIMESTAMPTZ
		)
	`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.bean_list_publications
			(id,owner_type,owner_key,list_type,status,publication_purpose,classification_template_id,published_at)
		VALUES
			(101,'customer','8','commercial','published','factory_supply',0,'2026-08-01 08:00:00+00'),
			(102,'customer','8','commercial','published','factory_supply',0,'2026-08-02 08:00:00+00'),
			(103,'customer','8','commercial','published','factory_supply',0,NULL),
			(201,'official','','green','published','factory_supply',9,'2026-08-02 08:00:00+00'),
			(202,'official','','green','published','factory_supply',9,NULL),
			(301,'official','','retail','published','factory_supply',11,'2026-08-02 08:00:00+00'),
			(302,'customer','8','retail','published','factory_supply',11,'2026-08-02 09:00:00+00')
	`, schema)); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	checks := []struct {
		publicationID int64
		listType      string
		want          bool
	}{
		{publicationID: 101, listType: "commercial", want: false},
		{publicationID: 102, listType: "commercial", want: true},
		{publicationID: 103, listType: "commercial", want: false},
		{publicationID: 201, listType: "green", want: true},
		{publicationID: 202, listType: "green", want: false},
		{publicationID: 301, listType: "retail", want: false},
		{publicationID: 302, listType: "retail", want: true},
	}
	for _, check := range checks {
		got, err := isCurrentDefaultOrderPublicationTx(ctx, tx, schema, 8, check.publicationID, check.listType)
		if err != nil {
			t.Fatalf("publication %d: %v", check.publicationID, err)
		}
		if got != check.want {
			t.Fatalf("publication %d current=%v want=%v", check.publicationID, got, check.want)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}
