package sales

import (
	"context"
	"encoding/json"
	"testing"
)

func TestOrderPublicationTraceFrozen(t *testing.T) {
	p, s := newSalesPostgresTestDB(t)
	defer p.Close()
	ctx := context.Background()
	_, e := p.Exec(ctx, "CREATE TABLE "+s+".customers(id bigint,name text);INSERT INTO "+s+".customers VALUES(42,'客户A'); CREATE TABLE "+s+".bean_list_publications(id bigint,owner_type text,owner_key text,product_type_name text,version_no text,published_at timestamptz); INSERT INTO "+s+".bean_list_publications VALUES(1,'customer','42','咖啡豆','V3.1','2026-09-07T10:00:00+08:00')")
	if e != nil {
		t.Fatal(e)
	}
	raw, e := orderPublicationTraceSnapshot(ctx, p, s, 1, `{"final_unit_price":33}`, false)
	if e != nil {
		t.Fatal(e)
	}
	var got map[string]any
	json.Unmarshal([]byte(raw), &got)
	if got["price_list_owner_name"] != "客户A" || got["price_list_name"] != "咖啡豆" || got["price_list_published_at"] != "2026-09-07 10:00:00" || got["version"] != "V3.1" || got["final_unit_price"] != float64(33) {
		t.Fatal(got)
	}
	p.Exec(ctx, "UPDATE "+s+".customers SET name='客户改名'")
	old, e := orderPublicationTraceSnapshot(ctx, p, s, 1, raw, true)
	if e != nil || old != raw {
		t.Fatal("existing order snapshot changed", e, old)
	}
}
