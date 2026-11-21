package sales

import (
	"context"
	"fmt"
	"testing"

	"orderapp/internal/infrastructure/postgres/orderconfirmation"
)

func TestCurrentOrderReadPreservesAcceptedAndPendingRevision(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
		pool.Close()
	})
	fixture := fmt.Sprintf(`
		CREATE TABLE %[1]s.orders(id bigint PRIMARY KEY, note text, confirmation_pending jsonb);
		CREATE TABLE %[1]s.order_items(id bigint PRIMARY KEY, order_id bigint, qty integer);
		INSERT INTO %[1]s.orders VALUES (1,'original',null),(2,'other',null),
			(3,'old','{"order":{"id":3,"note":"proposed"},"items":[{"id":12,"order_id":3,"qty":17}]}'::jsonb);
		INSERT INTO %[1]s.order_items VALUES(5,1,4),(6,2,3),(7,3,2)`, schema)
	if _, err := pool.Exec(ctx, fixture); err != nil {
		t.Fatal(err)
	}
	query := fmt.Sprintf(`SELECT o.note, i.qty, md5(to_jsonb(o)::text || to_jsonb(i)::text)
		FROM %[1]s.orders o JOIN %[1]s.order_items i ON i.order_id=o.id WHERE o.id=$1`, schema)
	for _, tc := range []struct {
		id   int64
		note string
		qty  int
	}{{1, "original", 4}, {3, "proposed", 17}} {
		var baseline, scoped string
		for _, path := range []struct {
			sql      string
			revision *string
		}{
			{orderconfirmation.CurrentRead(query, schema), &baseline},
			{orderconfirmation.CurrentOrderRead(query, schema), &scoped},
		} {
			var note string
			var qty int
			if err := pool.QueryRow(ctx, path.sql, tc.id).Scan(&note, &qty, path.revision); err != nil {
				t.Fatal(err)
			}
			if note != tc.note || qty != tc.qty {
				t.Fatalf("id=%d note=%q qty=%d", tc.id, note, qty)
			}
		}
		if scoped != baseline {
			t.Fatalf("different order revision for id=%d", tc.id)
		}
	}

	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.orders VALUES(4,'legacy','{"items":null}'::jsonb)`, schema)); err != nil {
		t.Fatal(err)
	}

	var legacyCount int
	legacyItems := orderconfirmation.CurrentOrderRead(fmt.Sprintf("SELECT count(*) FROM %s.order_items i WHERE i.order_id=$1", schema), schema)
	if err := pool.QueryRow(ctx, legacyItems, 4).Scan(&legacyCount); err != nil || legacyCount != 0 {
		t.Fatalf("null pending items returned count=%d err=%v", legacyCount, err)
	}
}
