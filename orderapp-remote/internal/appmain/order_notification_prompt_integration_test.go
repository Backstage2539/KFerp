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

func TestOrderNotificationPromptHTTPAuthorizationAndVersion(t *testing.T) {
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
			t.Fatalf("%s %s = %d want %d: %s", method, path, res.Code, want, res.Body.String())
		}
		var out map[string]json.RawMessage
		if err := json.Unmarshal(res.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	mini := "/api/mini/order-notifications"
	admin := "/api/message-center/external/prompt/reselect"
	call("GET", mini, "", "", false, 401)
	out := call("GET", mini, f.salesToken, "", false, 200)
	if string(out["prompt_required"]) != "true" || string(out["prompt_version"]) != "1" {
		t.Fatalf("initial: %s %s", out["prompt_required"], out["prompt_version"])
	}
	_, err := f.pool.Exec(f.ctx, fmt.Sprintf(`INSERT INTO %s.wechat_bindings(app_id,openid,mini_user_id,customer_id,employee_id,active,subscribed,notifications_enabled) VALUES ('notify-app','self-inactive',$1,0,$2,false,true,false),('notify-app','other-scope',$1,123,0,true,true,true)`, f.schema), f.sales.MiniUserID, f.sales.EmployeeID)
	if err != nil {
		t.Fatal(err)
	}
	out = call("GET", mini, f.salesToken, "", false, 200)
	var rows []struct {
		Active     bool  `json:"active"`
		CustomerID int64 `json:"customer_id"`
	}
	if err = json.Unmarshal(out["rows"], &rows); err != nil || len(rows) != 1 || rows[0].Active || rows[0].CustomerID != 0 {
		t.Fatalf("scope/active: %s err=%v", out["rows"], err)
	}
	call("PUT", mini+"/prompt", f.salesToken, `{"version":1}`, false, 400)
	identity := fmt.Sprintf(`"mini_user_id":%d,"customer_id":0,"employee_id":%d`, f.sales.MiniUserID, f.sales.EmployeeID)
	call("PUT", mini+"/prompt", f.salesToken, `{"version":1,"enabled":true,"events":["order_accepted"]}`, false, 409)
	wrongIdentity := fmt.Sprintf(`"mini_user_id":%d,"customer_id":123,"employee_id":%d`, f.sales.MiniUserID, f.sales.EmployeeID)
	call("PUT", mini+"/prompt", f.salesToken, `{"version":1,"enabled":true,"events":["order_accepted"],`+wrongIdentity+`}`, false, 409)
	call("PUT", mini+"/prompt", f.salesToken, `{"version":1,"enabled":true,"events":["order_accepted"],`+identity+`}`, false, 200)
	call("POST", admin, f.salesToken, `{}`, false, 403)
	call("POST", admin, "", `{}`, true, 200)
	call("PUT", mini+"/prompt", f.salesToken, `{"version":1,"enabled":false,"events":[],`+identity+`}`, false, 409)
	call("PUT", mini, f.salesToken, `{"prompt_version":1,"enabled":false,"events":[]}`, false, 409)
	out = call("GET", mini, f.salesToken, "", false, 200)
	if string(out["prompt_required"]) != "true" || string(out["prompt_version"]) != "2" {
		t.Fatalf("reselect: %s %s", out["prompt_required"], out["prompt_version"])
	}
	call("PUT", mini+"/prompt", f.salesToken, `{"version":2,"enabled":false,"events":[],`+identity+`}`, false, 200)
	out = call("GET", mini, f.salesToken, "", false, 200)
	if string(out["prompt_required"]) != "false" {
		t.Fatalf("declined: %s", out["prompt_required"])
	}
}
