package costing

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	appcosting "orderapp/internal/application/costing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func customerOrderBindingPostgres(t *testing.T) (Repository, context.Context) {
	t.Helper()
	if os.Getenv("KF_RUN_POSTGRES_INTEGRATION") != "1" {
		t.Skip("disposable PostgreSQL integration")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("pr664_price_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		pool.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); pool.Close() })
	ddl := fmt.Sprintf(`
		CREATE TABLE %[1]s.customers(id BIGINT PRIMARY KEY);
		CREATE TABLE %[1]s.bean_list_publications(
			id BIGSERIAL PRIMARY KEY,list_type TEXT NOT NULL DEFAULT 'commercial',publication_purpose TEXT NOT NULL DEFAULT 'factory_supply',
			product_type_category_id BIGINT NOT NULL DEFAULT 0,product_type_name TEXT NOT NULL DEFAULT '',classification_template_id BIGINT NOT NULL DEFAULT 0,
			publication_table_name TEXT NOT NULL DEFAULT '',publication_table_key TEXT NOT NULL DEFAULT '',config_json JSONB NOT NULL DEFAULT '{}',
			version_no TEXT NOT NULL,owner_type TEXT NOT NULL,owner_key TEXT NOT NULL DEFAULT '',status TEXT NOT NULL DEFAULT 'published',
			deleted_at TIMESTAMPTZ NULL,published_at TIMESTAMPTZ NULL DEFAULT now(),updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TABLE %[1]s.customer_order_price_table_bindings(
			customer_id BIGINT NOT NULL REFERENCES %[1]s.customers(id),usage_code TEXT NOT NULL,product_type_key TEXT NOT NULL,
			publication_id BIGINT NOT NULL REFERENCES %[1]s.bean_list_publications(id),revision BIGINT NOT NULL DEFAULT 1,updated_by TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),PRIMARY KEY(customer_id,usage_code,product_type_key)
		);
		CREATE TABLE %[1]s.audit_logs(id BIGSERIAL PRIMARY KEY,actor TEXT,entity_type TEXT,entity_id BIGINT,action TEXT,field TEXT,old_value TEXT,new_value TEXT,meta JSONB);
	`, schema)
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	return NewRepository(pool, schema), ctx
}

func insertCustomerOrderPublication(t *testing.T, r Repository, ctx context.Context, ownerType, ownerKey, tableName, version string) int64 {
	t.Helper()
	var id int64
	if err := r.pool.QueryRow(ctx, `INSERT INTO `+r.schema+`.bean_list_publications(
		list_type,product_type_category_id,product_type_name,publication_table_name,publication_table_key,version_no,owner_type,owner_key
	) VALUES('commercial',56,'烘焙咖啡豆',$1,'customer-roasted',$2,$3,$4) RETURNING id`, tableName, version, ownerType, ownerKey).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestCustomerOrderPriceTableBindingPinsExactCustomerPublication(t *testing.T) {
	r, ctx := customerOrderBindingPostgres(t)
	if _, err := r.pool.Exec(ctx, `INSERT INTO `+r.schema+`.customers(id) VALUES(450),(451)`); err != nil {
		t.Fatal(err)
	}
	v41 := insertCustomerOrderPublication(t, r, ctx, "customer", "450", "陈丹燕价一件代发格表", "V3.0.41")
	public := insertCustomerOrderPublication(t, r, ctx, "official", "", "公共价格表", "V3.0.41")
	other := insertCustomerOrderPublication(t, r, ctx, "customer", "451", "其他客户价格表", "V3.0.41")
	for _, publicationID := range []int64{public, other} {
		if _, err := r.SaveCustomerOrderPriceTableBinding(ctx, appcosting.SaveCustomerOrderPriceTableBindingCommand{
			CustomerID: 450, UsageCode: appcosting.CustomerOrderPriceTableUsageDirectShip, PublicationID: publicationID, Actor: "tester",
		}); err == nil {
			t.Fatalf("publication %d outside customer scope was accepted", publicationID)
		}
	}
	config, err := r.SaveCustomerOrderPriceTableBinding(ctx, appcosting.SaveCustomerOrderPriceTableBindingCommand{
		CustomerID: 450, UsageCode: appcosting.CustomerOrderPriceTableUsageDirectShip, PublicationID: v41, Actor: "tester",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Candidates) != 1 || len(config.Bindings) != 1 || config.Bindings[0].PublicationID != v41 || config.Bindings[0].Version != "V3.0.41" {
		t.Fatalf("initial config=%+v", config)
	}
	if _, err := r.SaveCustomerOrderPriceTableBinding(ctx, appcosting.SaveCustomerOrderPriceTableBindingCommand{
		CustomerID: 450, UsageCode: appcosting.CustomerOrderPriceTableUsageDirectShip, PublicationID: v41, Actor: "tester",
	}); err == nil || !strings.Contains(err.Error(), "已变化") {
		t.Fatalf("missing revision update error=%v", err)
	}
	v42 := insertCustomerOrderPublication(t, r, ctx, "customer", "450", "陈丹燕价一件代发格表", "V3.0.42")
	config, err = r.CustomerOrderPriceTableConfig(ctx, appcosting.CustomerOrderPriceTableQuery{CustomerID: 450})
	if err != nil || len(config.Bindings) != 1 || config.Bindings[0].PublicationID != v41 || len(config.Candidates) != 2 {
		t.Fatalf("new publication changed pinned binding: config=%+v err=%v", config, err)
	}
	config, err = r.SaveCustomerOrderPriceTableBinding(ctx, appcosting.SaveCustomerOrderPriceTableBindingCommand{
		CustomerID: 450, UsageCode: appcosting.CustomerOrderPriceTableUsageProductOrder, PublicationID: v41, Actor: "tester",
	})
	if err != nil || len(config.Bindings) != 2 {
		t.Fatalf("independent product-order binding config=%+v err=%v", config, err)
	}
	config, err = r.SaveCustomerOrderPriceTableBinding(ctx, appcosting.SaveCustomerOrderPriceTableBindingCommand{
		CustomerID: 450, UsageCode: appcosting.CustomerOrderPriceTableUsageDirectShip, ProductTypeKey: "classification:56", PublicationID: v42, ExpectedRevision: 1, Actor: "tester",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.rejectBoundCustomerOrderPriceTablePublications(ctx, r.pool, []int64{v41}); err == nil || !strings.Contains(err.Error(), "商品下单") {
		t.Fatalf("bound publication archive/delete guard error=%v", err)
	}
	direct := config.Bindings[0]
	for _, row := range config.Bindings {
		if row.UsageCode == appcosting.CustomerOrderPriceTableUsageDirectShip {
			direct = row
		}
	}
	config, err = r.SaveCustomerOrderPriceTableBinding(ctx, appcosting.SaveCustomerOrderPriceTableBindingCommand{
		CustomerID: 450, UsageCode: appcosting.CustomerOrderPriceTableUsageDirectShip, ProductTypeKey: "classification:56", ExpectedRevision: direct.Revision, Actor: "tester",
	})
	if err != nil || len(config.Bindings) != 1 || config.Bindings[0].UsageCode != appcosting.CustomerOrderPriceTableUsageProductOrder {
		t.Fatalf("clear direct-ship binding config=%+v err=%v", config, err)
	}
	var audits int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM `+r.schema+`.audit_logs WHERE action='save_customer_order_price_table_binding'`).Scan(&audits); err != nil || audits != 4 {
		t.Fatalf("binding audits=%d err=%v", audits, err)
	}
}
