package catalog

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	app "orderapp/internal/application/catalog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func customerCatalogFixture(t *testing.T) (Repository, *pgxpool.Pool, string) {
	t.Helper()
	dsn := os.Getenv("ORDERAPP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL required")
	}
	ctx := context.Background()
	p, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	s := fmt.Sprintf("test_pr634_%d", time.Now().UnixNano())
	t.Cleanup(func() { p.Exec(context.Background(), "DROP SCHEMA "+s+" CASCADE"); p.Close() })
	_, err = p.Exec(ctx, fmt.Sprintf(`CREATE SCHEMA %[1]s;
 CREATE TABLE %[1]s.customers(id bigint PRIMARY KEY,name text,active boolean DEFAULT true);
 CREATE TABLE %[1]s.products(id bigint PRIMARY KEY,name text,customer_id bigint DEFAULT 0,active boolean DEFAULT true);
 CREATE TABLE %[1]s.product_customer_references(id bigserial PRIMARY KEY,product_id bigint,customer_id bigint,customer_display_name text,customer_item_code text DEFAULT '',active boolean DEFAULT true,remark text DEFAULT '',created_by text DEFAULT '',updated_by text DEFAULT '',created_at timestamptz DEFAULT now(),updated_at timestamptz DEFAULT now());
 CREATE UNIQUE INDEX ref_active ON %[1]s.product_customer_references(product_id,customer_id) WHERE active=true;
 CREATE TABLE %[1]s.business_groups(id bigint PRIMARY KEY,name text,code text DEFAULT '',sort_order int DEFAULT 100,active boolean DEFAULT true);
 CREATE TABLE %[1]s.business_group_items(id bigint PRIMARY KEY,group_id bigint,parent_id bigint DEFAULT 0,name text,code text DEFAULT '',sort_order int DEFAULT 100,active boolean DEFAULT true);
 CREATE TABLE %[1]s.business_group_assignments(id bigserial PRIMARY KEY,group_id bigint,group_item_id bigint,usage_key text DEFAULT 'product_catalog',object_key text DEFAULT 'product',object_id bigint,object_ref text DEFAULT '',sort_order int DEFAULT 100);
 CREATE TABLE %[1]s.audit_logs(id bigserial PRIMARY KEY,ts timestamptz DEFAULT now(),actor text,entity_type text,entity_id bigint,action text,field text,old_value text,new_value text,meta jsonb);
 INSERT INTO %[1]s.customers VALUES(42,'客户A',true),(43,'客户B',true),(44,'停用',false);
 INSERT INTO %[1]s.products VALUES(1,'商品1',0,true),(2,'商品2',0,true),(3,'停用',0,false),(4,'A专属',42,true),(5,'未分类',0,true);
 INSERT INTO %[1]s.business_groups(id,name) VALUES(10,'咖啡豆');
 INSERT INTO %[1]s.business_group_items(id,group_id,parent_id,name) VALUES(11,10,0,'精品'),(12,10,11,'产地'),(13,10,0,'拼配'),(14,10,0,'空分类');
 INSERT INTO %[1]s.business_group_assignments(group_id,group_item_id,object_id) VALUES(10,12,1),(10,13,2);
 `, s))
	if err != nil {
		t.Fatal(err)
	}
	if err := ensureCustomerCatalogSchema(ctx, p, s); err != nil {
		t.Fatal(err)
	}
	return NewRepository(p, s), p, s
}
func TestCustomerCatalogCopyAtomicAndIndependent(t *testing.T) {
	r, p, s := customerCatalogFixture(t)
	ctx := context.Background()
	copyTo := func(c int64, mode string, ids ...int64) app.CopyCustomerCatalogResult {
		t.Helper()
		v, e := r.CopyCustomerCatalog(ctx, app.CopyCustomerCatalogCommand{CustomerID: c, Mode: mode, ProductIDs: ids, Actor: "test"})
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	first := copyTo(42, "selected", 1)
	if first.Created != 1 {
		t.Fatalf("%+v", first)
	}
	c, e := r.CustomerCatalog(ctx, 42)
	if e != nil {
		t.Fatal(e)
	}
	if len(c.Nodes) != 3 || len(c.Assignments) != 1 {
		t.Fatalf("copy only full ancestor path: %+v", c)
	}
	var leaf int64
	for _, n := range c.Nodes {
		if n.SourceItemID == 12 {
			leaf = n.ID
		}
	}
	if e := r.RenameCustomerCatalogNode(ctx, app.RenameCustomerCatalogNodeCommand{ID: leaf, CustomerID: 42, Name: "客户产地", Actor: "test"}); e != nil {
		t.Fatal(e)
	}
	p.Exec(ctx, fmt.Sprintf(`UPDATE %s.product_customer_references SET customer_display_name='客户商品1' WHERE customer_id=42`, s))
	again := copyTo(42, "all")
	if again.Created != 2 || again.Unchanged != 1 {
		t.Fatalf("all public active only: %+v", again)
	}
	copyTo(43, "selected", 1)
	c, _ = r.CustomerCatalog(ctx, 42)
	b, _ := r.CustomerCatalog(ctx, 43)
	for _, n := range c.Nodes {
		if n.ID == leaf && n.Name != "客户产地" {
			t.Fatal("repeat reset rename")
		}
		if n.SourceItemID == 14 {
			t.Fatal("copied unrelated empty category")
		}
	}
	for _, n := range b.Nodes {
		if n.Name == "客户产地" {
			t.Fatal("cross customer rename")
		}
	}
	if e := r.RenameCustomerCatalogNode(ctx, app.RenameCustomerCatalogNodeCommand{ID: leaf, CustomerID: 43, Name: "越权", Actor: "test"}); e == nil {
		t.Fatal("cross customer update accepted")
	}
	var name string
	p.QueryRow(ctx, fmt.Sprintf(`SELECT customer_display_name FROM %s.product_customer_references WHERE customer_id=42 AND product_id=1`, s)).Scan(&name)
	if name != "客户商品1" {
		t.Fatal(name)
	}
	var before int
	p.QueryRow(ctx, "SELECT count(*) FROM "+s+".audit_logs").Scan(&before)
	if _, e = r.CopyCustomerCatalog(ctx, app.CopyCustomerCatalogCommand{CustomerID: 43, Mode: "selected", ProductIDs: []int64{2, 4}, Actor: "test"}); e == nil {
		t.Fatal("foreign owned accepted")
	}
	var n int
	p.QueryRow(ctx, "SELECT count(*) FROM "+s+".audit_logs").Scan(&n)
	if n != before {
		t.Fatal("failed batch wrote audit")
	}
	p.QueryRow(ctx, "SELECT count(*) FROM "+s+".products").Scan(&n)
	if n != 5 {
		t.Fatal("created products")
	}
	p.Exec(ctx, "UPDATE "+s+".product_customer_references SET active=false WHERE product_id=1 AND customer_id=42")
	restored := copyTo(42, "selected", 1, 1)
	if restored.Restored != 1 {
		t.Fatalf("%+v", restored)
	}
	p.QueryRow(ctx, "SELECT count(*) FROM "+s+".product_customer_references WHERE customer_id=42 AND product_id=1").Scan(&n)
	if n != 1 {
		t.Fatal("restoration duplicated reference")
	}
}
func TestCustomerCatalogBackfillIsPreviewableAndIdempotent(t *testing.T) {
	r, p, s := customerCatalogFixture(t)
	ctx := context.Background()
	p.Exec(ctx, "INSERT INTO "+s+".product_customer_references(product_id,customer_id,customer_display_name) VALUES(1,42,'历史客户名')")
	result, e := r.MigrateCustomerCatalog(ctx, true, "test")
	if e != nil || result.Missing != 1 || result.Applied != 0 {
		t.Fatalf("preview %+v %v", result, e)
	}
	result, e = r.MigrateCustomerCatalog(ctx, false, "test")
	if e != nil || result.Applied != 1 {
		t.Fatalf("apply %+v %v", result, e)
	}
	result, e = r.MigrateCustomerCatalog(ctx, false, "test")
	if e != nil || result.Applied != 0 {
		t.Fatalf("repeat %+v %v", result, e)
	}
	var name string
	p.QueryRow(ctx, "SELECT customer_display_name FROM "+s+".product_customer_references").Scan(&name)
	if name != "历史客户名" {
		t.Fatal(name)
	}
}

func TestCustomerCatalogConcurrentCopiesRemainUnique(t *testing.T) {
	r, p, s := customerCatalogFixture(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := r.CopyCustomerCatalog(ctx, app.CopyCustomerCatalogCommand{CustomerID: 42, Mode: "all", Actor: "concurrent"})
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	var refs, nodes int
	if e := p.QueryRow(ctx, "SELECT count(*) FROM "+s+".product_customer_references").Scan(&refs); e != nil {
		t.Fatal(e)
	}
	if e := p.QueryRow(ctx, "SELECT count(*) FROM "+s+".customer_product_catalog_nodes").Scan(&nodes); e != nil {
		t.Fatal(e)
	}
	if refs != 3 || nodes != 5 {
		t.Fatalf("refs=%d nodes=%d", refs, nodes)
	}
}
func TestCustomerCatalogMigrationRollbackEvidence(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(fmt.Sprint(changed), func(t *testing.T) {
			r, p, s := customerCatalogFixture(t)
			ctx := context.Background()
			if _, e := p.Exec(ctx, "INSERT INTO "+s+".product_customer_references(product_id,customer_id,customer_display_name,catalog_sort_order) VALUES(1,42,'原客户名',7)"); e != nil {
				t.Fatal(e)
			}
			if _, e := r.MigrateCustomerCatalog(ctx, false, "migration"); e != nil {
				t.Fatal(e)
			}
			var aid int64
			if e := p.QueryRow(ctx, "SELECT id FROM "+s+".audit_logs WHERE action='migrate_customer_catalog'").Scan(&aid); e != nil {
				t.Fatal(e)
			}
			if changed {
				if _, e := p.Exec(ctx, "UPDATE "+s+".customer_product_catalog_nodes SET name='客户新名称'"); e != nil {
					t.Fatal(e)
				}
			}
			data, e := os.ReadFile("../../../../../scripts/sql/pr634_customer_catalog_rollback.sql")
			if e != nil {
				t.Fatal(e)
			}
			sql := strings.ReplaceAll(strings.ReplaceAll(string(data), ":'schema'", "'"+s+"'"), ":'migration_audit_id'", fmt.Sprintf("'%d'", aid))
			conn, e := p.Acquire(ctx)
			if e != nil {
				t.Fatal(e)
			}
			defer conn.Release()
			_, e = conn.Exec(ctx, sql)
			if changed {
				if e == nil {
					t.Fatal("changed categories rolled back")
				}
				conn.Exec(ctx, "ROLLBACK")
			} else if e != nil {
				t.Fatal(e)
			}
			var n, position int
			if e = conn.QueryRow(ctx, "SELECT count(*) FROM "+s+".customer_product_catalog_nodes").Scan(&n); e != nil {
				t.Fatal(e)
			}
			if !changed && n != 0 {
				t.Fatal(n)
			}
			if changed && n != 3 {
				t.Fatal(n)
			}
			if e = conn.QueryRow(ctx, "SELECT catalog_sort_order FROM "+s+".product_customer_references").Scan(&position); e != nil {
				t.Fatal(e)
			}
			if !changed && position != 7 {
				t.Fatal(position)
			}
		})
	}
}
