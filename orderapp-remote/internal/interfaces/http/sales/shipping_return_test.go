package sales

import (
	"bytes"
	"context"
	"github.com/labstack/echo/v4"
	"github.com/xuri/excelize/v2"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	app "orderapp/internal/application/sales"
	d "orderapp/internal/domain/shippingreturn"
	"strings"
	"testing"
)

type returnAPIRepo struct {
	app.Repository
	imported int
	selected int
}

func (r *returnAPIRepo) RematchShippingReturn(context.Context, string, int64, []int64) (d.Batch, error) {
	return d.Batch{ID: 1, Rematched: true}, nil
}
func (r *returnAPIRepo) ShippingReturnTemplates(context.Context) ([]d.Template, error) {
	v := d.DefaultTemplate()
	v.CarrierID = 1
	return []d.Template{v}, nil
}
func (r *returnAPIRepo) SaveShippingReturnTemplates(context.Context, string, []d.Template) error {
	return nil
}
func (r *returnAPIRepo) ImportShippingReturn(_ context.Context, _, _, _ string, _ d.Template, rows []d.Row) (d.Batch, error) {
	r.imported++
	return d.Batch{ID: 1, Rows: rows, Pending: len(rows)}, nil
}
func (r *returnAPIRepo) ShippingReturnCandidates(context.Context, int64) ([]d.Row, error) {
	return []d.Row{{ID: 1, OrderID: 3, Version: "v1"}}, nil
}
func (r *returnAPIRepo) SelectShippingReturn(context.Context, string, int64, int64, string) (app.FillShipmentTrackingResult, error) {
	r.selected++
	return app.FillShipmentTrackingResult{Updated: 1}, nil
}
func TestShippingReturnAPI(t *testing.T) {
	r := &returnAPIRepo{}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Request().Header.Get("X-Test-Employee") == "yes" {
				c.Set("employee_id", int64(1))
			}
			return next(c)
		}
	})
	registerOrderShippingExcelRoutes(e, app.NewService(r), nil)
	call := func(method, path, contentType string, data *bytes.Buffer, employee bool) *httptest.ResponseRecorder {
		q := httptest.NewRequest(method, path, data)
		q.Header.Set("Content-Type", contentType)
		if employee {
			q.Header.Set("X-Test-Employee", "yes")
		}
		out := httptest.NewRecorder()
		e.ServeHTTP(out, q)
		return out
	}
	if res := call(http.MethodGet, "/api/orders/shipping-return-candidates", "", bytes.NewBuffer(nil), false); res.Code != 403 {
		t.Fatal(res.Code, res.Body)
	}
	if res := call(http.MethodGet, "/api/orders/shipping-return-candidates", "", bytes.NewBuffer(nil), true); res.Code != 200 || !strings.Contains(res.Body.String(), `"version":"v1"`) {
		t.Fatal(res.Code, res.Body)
	}
	for _, tc := range []struct {
		body     string
		employee bool
		want     int
	}{{`{"batch_id":1,"row_ids":[2,4]}`, true, 200}, {`{"batch_id":0}`, true, 400}, {`{"batch_id":1}`, false, 403}} {
		res := call(http.MethodPost, "/api/orders/shipping-return-rematch", "application/json", bytes.NewBufferString(tc.body), tc.employee)
		if res.Code != tc.want || r.selected != 0 || r.imported != 0 {
			t.Fatalf("rematch %d %s", res.Code, res.Body)
		}
	}
	f := excelize.NewFile()
	f.SetSheetName("Sheet1", "寄件列表")
	cfg := d.DefaultTemplate()
	for _, c := range cfg.Columns {
		f.SetCellValue(cfg.Sheet, c.Column+"1", c.Header)
	}
	for col, v := range map[string]string{"C": "SF0001", "J": "张*", "K": "138****1234", "S": "0"} {
		f.SetCellValue(cfg.Sheet, col+"2", v)
	}
	xlsx, _ := f.WriteToBuffer()
	body := new(bytes.Buffer)
	m := multipart.NewWriter(body)
	m.WriteField("template_id", "sf-recipient")
	part, _ := m.CreateFormFile("file", "test.xlsx")
	part.Write(xlsx.Bytes())
	m.Close()
	res := call(http.MethodPost, "/api/orders/shipping-tracking-excel", m.FormDataContentType(), body, true)
	if res.Code != 200 || r.imported != 1 || !strings.Contains(res.Body.String(), `"actual_cents":0`) {
		t.Fatal(res.Code, res.Body, r.imported)
	}
	res = call(http.MethodPost, "/api/orders/3/shipping-return-selection", "application/json", bytes.NewBufferString(`{"candidate_id":1,"version":"v1"}`), true)
	if res.Code != 200 || r.selected != 1 {
		t.Fatal(res.Code, res.Body)
	}
	res = call(http.MethodPost, "/api/orders/3/shipping-return-selection", "application/json", bytes.NewBufferString(`{"candidate_id":1}`), true)
	if res.Code != 400 || r.selected != 1 {
		t.Fatal(res.Code, res.Body)
	}
}
