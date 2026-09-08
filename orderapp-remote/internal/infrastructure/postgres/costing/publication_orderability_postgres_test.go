package costing

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	app "orderapp/internal/application/costing"
	"orderapp/internal/infrastructure/postgres/productspecmigration"
	"os"
	"strings"
	"testing"
	"time"
)

func TestPublicationOrderabilityPostgresUsesRealOrderAuthority(t *testing.T) {
	r, ctx := orderabilityPostgres(t)
	_, err := r.pool.Exec(ctx, fmt.Sprintf(`
	CREATE TABLE %[1]s.products(id BIGINT PRIMARY KEY,name TEXT,active BOOLEAN DEFAULT true,parent_product_id BIGINT DEFAULT 0,customer_id BIGINT DEFAULT 0);
	CREATE TABLE %[1]s.production_bom_output_bindings(output_type TEXT,output_id BIGINT,is_default BOOLEAN,bom_id BIGINT,bom_version_id BIGINT);
	CREATE TABLE %[1]s.production_bom_versions(id BIGINT PRIMARY KEY,bom_id BIGINT,version_no TEXT,status TEXT);
	CREATE TABLE %[1]s.production_bom_specs(id BIGINT PRIMARY KEY,bom_id BIGINT,code TEXT,barcode TEXT,spec_key TEXT,name TEXT,inventory_unit TEXT);
	CREATE TABLE %[1]s.production_bom_version_variants(id BIGINT PRIMARY KEY,version_id BIGINT,bom_spec_id BIGINT,spec_name_snapshot TEXT,inventory_unit TEXT,is_default BOOLEAN,sort_order INT);
	INSERT INTO %[1]s.products(id,name,parent_product_id) VALUES(97,'测试生豆',0),(806,'旧公斤规格',97);
	`, r.schema))
	if err != nil {
		t.Fatal(err)
	}
	if err = productspecmigration.EnsureAuthorityView(ctx, r.pool, r.schema); err != nil {
		t.Fatal(err)
	}
	cmd := app.PublishBeanListCommand{ListType: "green", OwnerType: "official", Content: map[string]any{"price_rows": []any{map[string]any{"product_id": 806, "parent_product_id": 97, "final_unit_price": 30}}}}
	check := func(want string) {
		t.Helper()
		err := r.ValidateBeanListOrderability(ctx, cmd)
		if want == "" && err != nil {
			t.Fatal(err)
		}
		if want != "" && (err == nil || !strings.Contains(err.Error(), want)) {
			t.Fatalf("want %q got %v", want, err)
		}
	}
	check("测试生豆")
	check("默认已发布 BOM 规格")
	_, err = r.pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.production_bom_versions VALUES(81,8,'V1','published'); INSERT INTO %[1]s.production_bom_output_bindings VALUES('product',97,true,8,81);`, r.schema))
	if err != nil {
		t.Fatal(err)
	}
	check("默认已发布 BOM 规格") // A published BOM without a variant is still not orderable.
	_, err = r.pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.production_bom_specs VALUES(9001,8,'S-9001','','1kg','1kg','kg'); INSERT INTO %[1]s.production_bom_version_variants VALUES(9101,81,9001,'1kg','kg',true,1);`, r.schema))
	if err != nil {
		t.Fatal(err)
	}
	check("旧销售规格")
	row := map[string]any{"product_id": 97, "parent_product_id": 97, "bom_spec_id": 9001, "bom_variant_id": 9101, "final_unit_price": 30}
	cmd.Content["price_rows"] = []any{row}
	check("")
	row["bom_variant_id"] = 9199
	check("已不属于当前默认已发布 BOM")
	row["bom_variant_id"] = 9101
	_, err = r.pool.Exec(ctx, "UPDATE "+r.schema+".production_bom_versions SET status='draft'")
	if err != nil {
		t.Fatal(err)
	}
	check("默认已发布 BOM 规格")
	_, err = r.pool.Exec(ctx, "UPDATE "+r.schema+".products SET active=false WHERE id=97")
	if err != nil {
		t.Fatal(err)
	}
	check("已停用")
	_, err = r.pool.Exec(ctx, "UPDATE "+r.schema+".products SET active=true,customer_id=42 WHERE id=97; UPDATE "+r.schema+".production_bom_versions SET status='published'")
	if err != nil {
		t.Fatal(err)
	}
	check("不属于当前价格表归属")
	cmd.OwnerType = "customer"
	cmd.OwnerKey = "42"
	check("")
	cmd.OwnerKey = "43"
	check("不属于当前价格表归属")
	for _, table := range []string{"bean_list_publications", "audit_logs"} {
		var count int
		if err = r.pool.QueryRow(ctx, "SELECT count(*) FROM "+r.schema+"."+table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("read-only preflight wrote %s: %d %v", table, count, err)
		}
	}
}

func orderabilityPostgres(t *testing.T) (Repository, context.Context) {
	t.Helper()
	if os.Getenv("KF_RUN_POSTGRES_INTEGRATION") != "1" {
		t.Skip("disposable PostgreSQL integration")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("pr638_%d", time.Now().UnixNano())
	if _, err = pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); pool.Close() })
	if _, err = pool.Exec(ctx, "CREATE TABLE "+schema+".bean_list_publications(id BIGSERIAL); CREATE TABLE "+schema+".audit_logs(id BIGSERIAL)"); err != nil {
		t.Fatal(err)
	}
	return NewRepository(pool, schema), ctx
}
