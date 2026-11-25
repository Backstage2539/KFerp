package customerfulfillment

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http/httptest"
	salesapp "orderapp/internal/application/sales"
	d "orderapp/internal/domain/shippingreturn"
	sp "orderapp/internal/infrastructure/postgres/sales"
	saleshttp "orderapp/internal/interfaces/http/sales"
	"testing"
)

func TestShippingReturnBatchFilteredOrdersHTTP(t *testing.T) {
	f := newFulfillmentEditFixture(t)
	r := sp.NewRepository(f.pool, f.schema)
	cfg := d.DefaultTemplate()
	cfg.CarrierID = 1
	ctx := context.Background()
	if _, err := f.pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.orders(order_no,order_date,document_date,customer_id) SELECT 'PAGE-'||n, current_date+10,current_date+10,501 FROM generate_series(1,12) n`, f.schema)); err != nil {
		t.Fatal(err)
	}
	importRows := func(key string, rows []d.Row) d.Batch {
		t.Helper()
		b, err := r.ImportShippingReturn(ctx, "tester", "returns.xlsx", key, cfg, rows)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	item := func(n int, tracking string) d.Row {
		return d.Row{RowNo: n, TrackingNo: tracking, Receiver: "测*", Phone: "138****0000", Status: "pending", ActualCents: 2010}
	}
	other := item(2, "SF-OTHER")
	other.Phone = "139****9999"
	importRows("another-file", []d.Row{other})
	skipped := item(12, "SF-SKIP")
	skipped.Error = "单号或运费为空"
	skipped.Status = "skipped"
	wrongName := item(8, "SF-DIFFERENT-NAME")
	wrongName.Receiver = "陈*"
	noPhone := item(9, "SF-DIFFERENT-PHONE")
	noPhone.Phone = "137****1234"
	b := importRows("current-file", []d.Row{item(6, "SF-ONE"), item(7, "SF-TWO"), wrongName, noPhone, skipped})
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error { c.Set("employee_id", int64(703)); return next(c) }
	})
	saleshttp.RegisterRoutes(e, saleshttp.Dependencies{Sales: salesapp.NewService(r), Authz: fulfillmentEditTestAuthz{}, FulfillmentEditor: f.svc})
	get := func(path string, out any) {
		t.Helper()
		w := httptest.NewRecorder()
		e.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
		if err := json.Unmarshal(w.Body.Bytes(), out); err != nil {
			t.Fatal(err)
		}
	}
	var report struct {
		Pending int
		Rows    []struct {
			Status, Error string
			Candidates    []struct {
				OrderID   int64  `json:"order_id"`
				BatchID   int64  `json:"batch_id"`
				Customer  string `json:"customer_name"`
				CanSelect bool   `json:"can_select"`
				Block     string `json:"block_reason"`
			}
		}
	}
	path := fmt.Sprintf("/api/orders/shipping-return-batches/%d", b.ID)
	get(path, &report)
	if report.Pending != 2 || len(report.Rows) != 5 || report.Rows[2].Error != "待发订单手机号可见位一致，但收件姓名不符" || report.Rows[3].Error != "待发订单中没有收件手机号前三位和后四位一致的记录" || report.Rows[4].Error != "S 列应收金额为空" {
		t.Fatalf("report %+v", report)
	}
	for _, row := range report.Rows[:2] {
		if len(row.Candidates) != 1 {
			t.Fatalf("row %+v", row)
		}
		c := row.Candidates[0]
		if c.OrderID != f.request.OrderID || c.BatchID != b.ID || c.Customer == "" || c.CanSelect || c.Block == "" {
			t.Fatalf("wrong candidate %+v", c)
		}
	}
	list := func(extra string, want int) {
		t.Helper()
		var data struct {
			Total int
			Rows  []struct{ ID int64 }
		}
		get(fmt.Sprintf("/api/orders?shipping_return_batch_id=%d&limit=1%s", b.ID, extra), &data)
		if data.Total != want || len(data.Rows) != want || want > 0 && data.Rows[0].ID != f.request.OrderID {
			t.Fatalf("filtered list %+v", data)
		}
	}
	var ordinary struct {
		Total int
		Rows  []struct{ ID int64 }
	}
	get("/api/orders?limit=10", &ordinary)
	if ordinary.Total < 13 || len(ordinary.Rows) != 10 {
		t.Fatalf("expected paginated fixture: %+v", ordinary)
	}
	for _, o := range ordinary.Rows {
		if o.ID == f.request.OrderID {
			t.Fatal("matched order should start off-page")
		}
	}
	list("", 1)
	list("&q=no-such-order", 0)
	if _, err := f.pool.Exec(ctx, fmt.Sprintf("UPDATE %s.orders SET receiver_phone='13900009999' WHERE id=$1", f.schema), f.request.OrderID); err != nil {
		t.Fatal(err)
	}
	list("", 0)
	get(path, &report)
	if report.Rows[0].Status != "unmatched" || report.Pending != 0 {
		t.Fatalf("stale candidates %+v", report)
	}
	// Simulate an already confirmed row in this batch. The order must remain in the filter.
	if _, err := f.pool.Exec(ctx, fmt.Sprintf("UPDATE %s.shipping_return_rows SET status='applied',selected_order_id=$1 WHERE id=$2", f.schema), f.request.OrderID, b.Rows[0].ID); err != nil {
		t.Fatal(err)
	}
	list("", 1)
}
