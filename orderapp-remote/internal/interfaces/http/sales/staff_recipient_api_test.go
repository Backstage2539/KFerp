package sales

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	salesapp "orderapp/internal/application/sales"
	postgressales "orderapp/internal/infrastructure/postgres/sales"

	"github.com/labstack/echo/v4"
)

func TestPortalRecipientPatchPreservesPendingCompanyWhenOmitted(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	seedConfirmationExecutionFixtures(t, ctx, pool, schema)
	if err := postgressales.EnsureCustomerOrderSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
INSERT INTO %[1]s.orders(id,order_no,customer_id,ship_status_id,process_status_id,portal_service_code,
 receiver_name,receiver_phone,receiver_address,receiver_company)
 VALUES(42,'SO-PENDING-COMPANY',3,1,1,'product_order','原收件人','13800000001','原地址','已确认公司');
INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,qty,unit,unit_price,line_total)
 VALUES(43,42,1,7,'原商品',2,'袋',33,66);
`, schema))
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(1))
			c.Set("actor", "客户一")
			return next(c)
		}
	})
	svc := salesapp.NewService(postgressales.NewRepository(pool, schema))
	registerOrderAPI(e, svc, nil, workspaceTestScope{})
	for _, body := range []string{
		`{"receiver_name":"收件人","receiver_phone":"13800000002","receiver_address":"待确认地址","receiver_company":"待确认公司"}`,
		`{"receiver_name":"收件人","receiver_phone":"13800000002","receiver_address":"再次修改地址"}`,
	} {
		r := httptest.NewRequest(http.MethodPatch, "/api/customer-processing/portal/orders/42/recipient", strings.NewReader(body))
		r.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		w := httptest.NewRecorder()
		e.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("portal recipient PATCH=%d: %s", w.Code, w.Body.String())
		}
	}
	var accepted, pending, pendingAddress string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT receiver_company,confirmation_pending->'order'->>'receiver_company',confirmation_pending->'order'->>'receiver_address' FROM %s.orders WHERE id=42`, schema)).Scan(&accepted, &pending, &pendingAddress); err != nil {
		t.Fatal(err)
	}
	if accepted != "已确认公司" || pending != "待确认公司" || pendingAddress != "再次修改地址" {
		t.Fatalf("portal company snapshots changed: accepted=%q pending=%q address=%q", accepted, pending, pendingAddress)
	}
}
