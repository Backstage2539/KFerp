package sales

import (
	"context"
	"encoding/json"
	"fmt"
	salesapp "orderapp/internal/application/sales"
	"testing"
)

func TestNamedPriceTableOrderPostgresSelectionAndFrozenSource(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	defer pool.Close()
	ctx := context.Background()
	defer pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	_, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE %s.bean_list_publications(id BIGINT PRIMARY KEY,list_type TEXT, classification_template_id BIGINT DEFAULT 0,product_type_category_id BIGINT DEFAULT 0,owner_type TEXT,owner_key TEXT,status TEXT,publication_purpose TEXT,published_at TIMESTAMPTZ,config_json JSONB);
 INSERT INTO %s.bean_list_publications VALUES
 (1,'commercial',10,10,'customer','42','published','factory_supply','2026-09-01','{"publication_batch":{"release_id":"batch","table_key":"a","table_name":"227g","is_default_table":true}}'),
 (2,'commercial',10,10,'customer','42','published','factory_supply','2026-09-01','{"publication_batch":{"release_id":"batch","table_key":"b","table_name":"1kg","is_default_table":false}}'),
 (3,'commercial',10,10,'customer','43','published','factory_supply','2026-09-01','{"publication_batch":{"release_id":"foreign","table_name":"其他客户"}}'),
 (4,'green',20,20,'official','','published','factory_supply','2026-09-01','{}'),
 (5,'commercial',10,10,'customer','42','published','factory_supply','2026-08-01','{"publication_batch":{"release_id":"old","table_name":"旧版"}}')`, schema, schema))
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	for _, tc := range []struct {
		selected, actual []int64
		bad              bool
	}{
		{[]int64{2, 4}, []int64{2, 4}, false}, {nil, []int64{2, 4}, false},
		{[]int64{2}, []int64{1}, true}, {nil, []int64{1, 2}, true}, {[]int64{1, 2}, []int64{1}, true},
		{[]int64{3}, []int64{3}, true}, {[]int64{999}, []int64{999}, true}, {[]int64{2}, []int64{0}, true},
	} {
		meta, err := validateSelectedPriceTablesTx(ctx, tx, schema, salesapp.SaveOrderCommand{CustomerID: 42, SelectedPriceTableIDs: tc.selected}, tc.actual)
		if (err != nil) != tc.bad {
			t.Fatalf("selected=%v actual=%v err=%v", tc.selected, tc.actual, err)
		}
		if !tc.bad {
			var source map[string]any
			if err := json.Unmarshal([]byte(withNamedPriceTableSnapshot(`{"publication_id":2,"version_no":"V3.0.6"}`, meta[2])), &source); err != nil || source["price_table_name"] != "1kg" || source["version_no"] != "V3.0.6" {
				t.Fatalf("frozen source=%v err=%v", source, err)
			}
		}
	}
	for _, id := range []int64{1, 2} {
		ok, err := isCurrentDefaultOrderPublicationTx(ctx, tx, schema, 42, id, "commercial")
		if err != nil || !ok {
			t.Fatalf("current sibling %d=%v err=%v", id, ok, err)
		}
	}
	for _, id := range []int64{3, 5, 999} {
		ok, err := isCurrentDefaultOrderPublicationTx(ctx, tx, schema, 42, id, "commercial")
		if err != nil || ok {
			t.Fatalf("ineligible %d=%v err=%v", id, ok, err)
		}
	}
}
