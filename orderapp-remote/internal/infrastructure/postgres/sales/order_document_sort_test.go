package sales

import (
	"context"
	"os"
	"reflect"
	"regexp"
	"testing"
)

func TestOrderListDocumentDateSort(t *testing.T) {
	pool, s := newSalesPostgresTestDB(t)
	defer pool.Close()
	ctx := context.Background()
	source, e := os.ReadFile("order_queries.go")
	if e != nil {
		t.Fatal(e)
	}
	clause := regexp.MustCompile(`ORDER BY o\.[^\n]+`).FindString(string(source))
	_, e = pool.Exec(ctx, "CREATE TABLE "+s+".orders(id int,document_date date,order_date date); INSERT INTO "+s+".orders VALUES(1,'2026-09-07','2026-01-01'),(2,'2026-09-06','2026-12-01'),(3,'2026-09-07','2026-09-01')")
	if e != nil {
		t.Fatal(e)
	}
	check := func() {
		rows, e := pool.Query(ctx, "SELECT id FROM "+s+".orders o "+clause)
		if e != nil {
			t.Fatal(e)
		}
		defer rows.Close()
		var got []int
		for rows.Next() {
			var id int
			rows.Scan(&id)
			got = append(got, id)
		}
		if !reflect.DeepEqual(got, []int{3, 1, 2}) {
			t.Fatalf("document date order = %v", got)
		}
	}
	check()
	pool.Exec(ctx, "UPDATE "+s+".orders SET order_date='2027-01-01' WHERE id=2")
	check()
}
