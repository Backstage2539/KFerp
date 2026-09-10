package sales

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"image/png"
	"net/http/httptest"
	app "orderapp/internal/application/customerfulfillment"
	salesapp "orderapp/internal/application/sales"
	postgressales "orderapp/internal/infrastructure/postgres/sales"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type documentAccountScope struct {
	accountScopeStub
	pool   *pgxpool.Pool
	schema string
}

func (s documentAccountScope) CustomerAccount(ctx context.Context, q app.AccountQuery) (app.AccountData, error) {
	d := app.AccountData{Rows: []app.AccountOrder{}, CustomerName: "测试客户"}
	var row app.AccountOrder
	err := s.pool.QueryRow(ctx, fmt.Sprintf(`SELECT id,order_no,is_void FROM %s.orders WHERE id=$1 AND customer_id=$2 AND NOT is_void`, s.schema), q.OrderID, q.CustomerID).Scan(&row.ID, &row.OrderNo, &row.IsVoid)
	if err == nil {
		d.Rows = append(d.Rows, row)
	}
	return d, nil
}
func TestCustomerAccountDocumentsAndRecipientLifecycle(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	seedConfirmationExecutionFixtures(t, ctx, pool, schema)
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	seedCombinedDocumentOrders(t, ctx, pool, schema, false)
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(1))
			c.Set("actor", "客户一")
			return next(c)
		}
	})
	svc := salesapp.NewService(postgressales.NewRepository(pool, schema, postgressales.WithSalesOrderAssetDir(t.TempDir())))
	registerOrderAPI(e, svc, nil, "", documentAccountScope{accountScopeStub{customer: 3, finance: true}, pool, schema})
	request := func(method, path, body string, code int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		e.ServeHTTP(w, r)
		if w.Code != code {
			t.Fatalf("%s %s: %d want %d: %s", method, path, w.Code, code, w.Body.String())
		}
		return w
	}
	old, err := svc.GenerateSalesOrderDocument(ctx, salesapp.GenerateSalesOrderDocumentCommand{OrderID: 1, Actor: "工厂"})
	if err != nil {
		t.Fatal(err)
	}
	oldFile, err := svc.LoadSalesOrderDocumentFile(ctx, 1, old.Document.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	oldBytes, err := os.ReadFile(oldFile.Path)
	if err != nil {
		t.Fatal(err)
	}
	request("PATCH", accountPrefix+"/orders/1/recipient", `{"receiver_name":"新收件人","receiver_phone":"13800138000","receiver_address":"上海市徐汇区新地址100号"}`, 200)
	if _, err := svc.ReviewOrder(ctx, salesapp.ReviewOrderCommand{OrderID: 1, Revision: 2, Decision: "accepted", Admin: true, Actor: "管理员"}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"1", "combined"} {
		query := ""
		body := "{}"
		if id == "combined" {
			query = "?order_ids=1,2"
			body = `{"order_ids":[1,2]}`
		}
		base := accountPrefix + "/orders/" + id
		request("GET", base+"/sales-order-preview"+query, "", 200)
		p := request("GET", base+"/sales-order-preview.pdf"+query, "", 200)
		if !bytes.HasPrefix(p.Body.Bytes(), []byte("%PDF")) {
			t.Fatal("invalid preview PDF")
		}
		request("POST", base+"/sales-orders", body, 200)
		request("POST", base+"/sales-order-images", body, 200)
		list := request("GET", base+"/sales-orders"+query, "", 200)
		if strings.Contains(list.Body.String(), "snapshot") || strings.Contains(list.Body.String(), "pdf_asset_id") {
			t.Fatal("internal document metadata leaked")
		}
		var result struct {
			Rows []struct {
				URL string `json:"download_url"`
			}
			Images []struct {
				URL string `json:"download_url"`
			} `json:"image_rows"`
		}
		if err := json.Unmarshal(list.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		for _, url := range []string{result.Rows[0].URL, result.Images[0].URL} {
			file := request("GET", url, "", 200)
			ext := "pdf"
			if strings.Contains(url, ".png") {
				ext = "png"
				if _, err := png.DecodeConfig(bytes.NewReader(file.Body.Bytes())); err != nil {
					t.Fatal(err)
				}
			} else if !bytes.HasPrefix(file.Body.Bytes(), []byte("%PDF")) {
				t.Fatal("invalid PDF")
			}
			if dir := os.Getenv("ACCOUNT_ARTIFACT_DIR"); dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "sales-"+id+"."+ext), file.Body.Bytes(), 0600); err != nil {
					t.Fatal(err)
				}
			}
		}
		request("GET", base+"/sales-files/999999.pdf"+query, "", 404)
	}
	docs, err := svc.ListSalesOrderDocuments(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(docs[0].Snapshot)
	if !strings.Contains(string(raw), `"receiver_address":"上海市徐汇区新地址100号"`) {
		t.Fatal("new sales snapshot omitted current recipient")
	}
	// A customer's document IDs must not unlock another customer's file.
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.orders SET customer_id=999 WHERE id=2`, schema))
	request("GET", accountPrefix+"/orders/combined/sales-order-preview?order_ids=1,2", "", 403)
	request("GET", accountPrefix+"/orders/combined/sales-files/1.pdf?order_ids=1,2", "", 403)
	for i, status := range []string{"已发货", "已出库", "已签收", "已收货", "已完成", "部分发货"} {
		id := 100 + i
		mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %s.ship_statuses(id,name)VALUES(%d,'%s');UPDATE %s.orders SET ship_status_id=%d WHERE id=1`, schema, id, status, schema, id))
		request("PATCH", accountPrefix+"/orders/1/recipient", `{"receiver_name":"禁止修改`+strconv.Itoa(i)+`","receiver_phone":"13800138000","receiver_address":"上海市徐汇区另一个地址"}`, 400)
	}
	afterBytes, err := os.ReadFile(oldFile.Path)
	if err != nil || !bytes.Equal(oldBytes, afterBytes) {
		t.Fatal("historical file changed")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, fmt.Sprintf("UPDATE %s.orders SET ship_status_id=100 WHERE id=1", schema)); err != nil {
		t.Fatal(err)
	}
	result := make(chan int, 1)
	go func() {
		req := httptest.NewRequest("PATCH", accountPrefix+"/orders/1/recipient", strings.NewReader(`{"receiver_name":"并发修改","receiver_phone":"13800138000","receiver_address":"新的地址"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		result <- rec.Code
	}()
	select {
	case code := <-result:
		t.Fatalf("recipient write ignored shipment lock: %d", code)
	case <-time.After(80 * time.Millisecond):
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case code := <-result:
		if code != 400 {
			t.Fatalf("concurrent shipment allowed recipient update %d", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("recipient request did not finish")
	}
	var n int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.audit_logs WHERE entity_type='order' AND field='recipient'`, schema)).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("recipient audit count %d", n)
	}
}
