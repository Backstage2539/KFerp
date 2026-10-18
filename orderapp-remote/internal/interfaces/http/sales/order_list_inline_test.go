package sales

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http/httptest"
	salesapp "orderapp/internal/application/sales"
	"reflect"
	"testing"
)

type inlineOrderListRepo struct{ salesapp.Repository }

func (*inlineOrderListRepo) ListOrders(context.Context, salesapp.OrderListQuery) (salesapp.OrderListResult, error) {
	return salesapp.OrderListResult{Rows: []salesapp.OrderRow{{ID: 1, OrderType: "批发", ProductKindSummary: "drip_bag", ProductTypeLabels: []string{"挂耳", "自定义分类"}, IsSampleOrder: true}}}, nil
}
func TestOrderListInlineAPIContract(t *testing.T) {
	e := echo.New()
	registerOrderAPI(e, salesapp.NewService(&inlineOrderListRepo{}), nil)
	r := httptest.NewRecorder()
	e.ServeHTTP(r, httptest.NewRequest("GET", "/api/orders", nil))
	if r.Code != 200 {
		t.Fatalf("status %d %s", r.Code, r.Body.String())
	}
	var b struct {
		Rows []map[string]any `json:"rows"`
	}
	if err := json.Unmarshal(r.Body.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	row := b.Rows[0]
	if !reflect.DeepEqual(row["product_type_labels"], []any{"挂耳", "自定义分类"}) || row["is_sample_order"] != true || row["product_kind_summary"] != "drip_bag" {
		t.Fatalf("contract: %v", row)
	}
}
func TestOrderListInlinePostgresAllTypesAndSample(t *testing.T) {
	pool, s := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, s)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
 CREATE TABLE %[1]s.customer_order_production_demands(order_id bigint);
 CREATE TABLE %[1]s.sample_work_orders(id bigint,order_id bigint);
 INSERT INTO %[1]s.product_categories(id,parent_id,name) VALUES(501,0,'自定义饮品'),(502,501,'冷萃'),(503,0,'咖啡器具');
 INSERT INTO %[1]s.products(id,name,product_category_id,parent_product_id,product_kind,active) VALUES
 (801,'主商品',502,0,'roasted',false),(802,'规格商品',0,801,'roasted',true),(803,'器具',503,0,'mystery',true),(804,'无分类',0,0,'mystery',true);
 INSERT INTO %[1]s.orders(id,order_no,customer_id,order_type_id,order_date,document_date,notes) VALUES(9001,'SO-INLINE-MIX',3,1,'2026-10-18','2026-10-18','样品快照'),(9002,'SO-INLINE-ORDINARY',3,1,'2026-10-18','2026-10-18','样品备注不算关联'),(9003,'SO-INLINE-EMPTY',3,1,'2026-10-18','2026-10-18','');
 INSERT INTO %[1]s.order_items(order_id,line_no,product_id,product_kind,product_name_snapshot,qty,unit_price,line_total) VALUES
 (9001,1,802,'roasted','规格商品',1,0,0),(9001,2,801,'roasted','主商品',1,0,0),(9001,3,803,'mystery','器具',1,0,0),(9001,4,804,'drip_bag','历史挂耳',1,0,0),(9001,5,0,'unknown','历史未知',1,0,0),
 (9002,1,0,'green_bean','历史生豆',1,10,10);
 INSERT INTO %[1]s.sample_work_orders VALUES(1,9001),(2,9001);
 `, s))
	e := newOrderAPITestEcho(pool, s)
	for _, tc := range []struct {
		id     int
		types  []string
		sample bool
	}{{9001, []string{"自定义饮品", "咖啡器具", "挂耳", "未分类"}, true}, {9002, []string{"生豆"}, false}, {9003, []string{}, false}} {
		r := httptest.NewRecorder()
		e.ServeHTTP(r, httptest.NewRequest("GET", fmt.Sprintf("/api/orders?order_id=%d", tc.id), nil))
		if r.Code != 200 {
			t.Fatalf("%d %s", r.Code, r.Body.String())
		}
		var b struct {
			Rows []salesapp.OrderRow `json:"rows"`
		}
		if err := json.Unmarshal(r.Body.Bytes(), &b); err != nil {
			t.Fatal(err)
		}
		if len(b.Rows) != 1 || !reflect.DeepEqual(b.Rows[0].ProductTypeLabels, tc.types) || b.Rows[0].IsSampleOrder != tc.sample {
			t.Fatalf("order %d: %+v", tc.id, b.Rows)
		}
	}
}
