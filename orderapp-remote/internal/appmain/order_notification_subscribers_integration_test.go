package appmain

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	sales "orderapp/internal/application/sales"
	salespg "orderapp/internal/infrastructure/postgres/sales"
	wechat "orderapp/internal/infrastructure/wechataccount"
	notify "orderapp/internal/interfaces/http/notifications"
)

func TestNotificationSubscriberAdminHTTPAndStalePopup(t *testing.T) {
	f := newSampleFixture(t)
	notify.RegisterRoutes(f.e, f.pool, f.schema, f.portal, wechatTestAuthz{}, sales.NewService(salespg.NewRepository(f.pool, f.schema)), wechat.Config{AppID: "notify-app"})
	call := func(method, path, token, body string, admin bool, want int) map[string]json.RawMessage {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if admin {
			req.Header.Set("X-Test-Admin", "yes")
		}
		res := httptest.NewRecorder()
		f.e.ServeHTTP(res, req)
		if res.Code != want {
			t.Fatalf("%s %s: %d want %d %s", method, path, res.Code, want, res.Body.String())
		}
		var out map[string]json.RawMessage
		_ = json.Unmarshal(res.Body.Bytes(), &out)
		return out
	}
	const admin = "/api/message-center/external/subscribers"
	const mini = "/api/mini/order-notifications"
	ident := fmt.Sprintf(`"mini_user_id":%d,"customer_id":0,"employee_id":%d`, f.sales.MiniUserID, f.sales.EmployeeID)
	call("GET", admin, f.salesToken, "", false, 403)
	call("POST", admin+"/reselect", f.salesToken, "{"+ident+"}", false, 403)
	out := call("GET", admin+"?q=13890000002", "", "", true, 200)
	var rows []map[string]any
	if err := json.Unmarshal(out["items"], &rows); err != nil || len(rows) != 1 {
		t.Fatalf("list %s %v", out["items"], err)
	}
	call("PUT", mini+"/prompt", f.salesToken, `{"version":1,"enabled":true,"events":["shipment_created"],`+ident+`}`, false, 200)
	call("POST", admin+"/reselect", "", "{"+ident+"}", true, 200)
	call("PUT", mini+"/prompt", f.salesToken, `{"version":1,"enabled":false,"events":[],`+ident+`}`, false, 409)
	out = call("GET", mini, f.salesToken, "", false, 200)
	if string(out["prompt_required"]) != "true" || string(out["prompt_version"]) != "2" {
		t.Fatalf("prompt %s", out)
	}
	call("POST", admin+"/reselect", "", fmt.Sprintf(`{"mini_user_id":%d,"customer_id":999999,"employee_id":0}`, f.sales.MiniUserID), true, 400)
	f.exec("UPDATE %s.company_employees SET active=false WHERE id=$1", f.sales.EmployeeID)
	call("POST", admin+"/reselect", "", "{"+ident+"}", true, 400)
}
