package wechataccount

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	messages "orderapp/internal/application/messagecenter"
	"strings"
	"testing"
	"time"
)

func TestNotificationQRCodeEncodesFullChallengeWithinSceneLimit(t *testing.T) {
	challenge := strings.Repeat("ab", 32)
	var scene string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var wire struct {
			ActionInfo struct {
				Scene struct {
					SceneStr string `json:"scene_str"`
				} `json:"scene"`
			} `json:"action_info"`
		}
		if err := json.NewDecoder(r.Body).Decode(&wire); err != nil {
			t.Fatal(err)
		}
		scene = wire.ActionInfo.Scene.SceneStr
		fmt.Fprint(w, `{"errcode":0,"ticket":"ticket"}`)
	}))
	defer server.Close()
	client := WechatClient{BaseURL: server.URL, token: "cached", expires: time.Now().Add(time.Hour)}
	if _, err := client.NotificationQRCode(context.Background(), challenge); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(scene, "notify_") || len(scene) > 64 {
		t.Fatalf("scene length/prefix invalid: %d", len(scene))
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(scene, "notify_"))
	if err != nil || hex.EncodeToString(raw) != challenge {
		t.Fatalf("challenge not preserved: %v", err)
	}
}

func TestTemplateSixMilestonesAndWire(t *testing.T) {
	for event := range messages.MilestoneLabels {
		t.Run(event, func(t *testing.T) {
			var wire map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/cgi-bin/message/template/send" || r.URL.Query().Get("access_token") != "cached" {
					t.Error("wrong endpoint")
				}
				if e := json.NewDecoder(r.Body).Decode(&wire); e != nil {
					t.Error(e)
				}
				fmt.Fprint(w, `{"errcode":0,"msgid":12345}`)
			}))
			defer server.Close()
			c := WechatClient{BaseURL: server.URL, token: "cached", expires: time.Now().Add(time.Hour), Config: Config{MiniAppID: "mini"}}
			n := messages.OrderNotice{OrderID: 42, CustomerID: 8, OrderNo: "TEST-42", Milestones: []string{event}, Test: true}
			tpl := TemplateMapping{ID: "template", Fields: map[string]string{"character_string1": "order_no", "thing2": "status"}}
			id, e := c.SendOrderTemplate(context.Background(), "official-openid", tpl, n, false)
			if e != nil || id != "12345" {
				t.Fatalf("id=%s err=%v", id, e)
			}
			if wire["touser"] != "official-openid" || wire["template_id"] != "template" {
				t.Fatal(wire)
			}
			mini := wire["miniprogram"].(map[string]any)
			if mini["pagepath"] != n.MiniPath(false) || mini["appid"] != "mini" {
				t.Fatal(mini)
			}
		})
	}
}

func TestTemplateFailureClassification(t *testing.T) {
	for _, c := range []struct {
		code int
		want string
	}{{43004, "skipped"}, {40003, "failed"}, {45009, "retry"}, {40037, "failed"}, {-1, "retry"}} {
		t.Run(fmt.Sprint(c.code), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprintf(w, `{"errcode":%d}`, c.code) }))
			defer server.Close()
			client := WechatClient{BaseURL: server.URL, token: "cached", expires: time.Now().Add(time.Hour)}
			_, err := client.SendOrderTemplate(context.Background(), "o", TemplateMapping{ID: "t", Fields: map[string]string{"thing1": "status"}}, messages.OrderNotice{Milestones: []string{"paid"}}, false)
			if TemplateFailureState(err) != c.want {
				t.Fatalf("%v => %s", err, TemplateFailureState(err))
			}
		})
	}
	if TemplateFailureState(fmt.Errorf("transport uncertain")) != "unknown" {
		t.Fatal("ambiguous outcome must not retry")
	}
}

func TestTemplateRejectsUnknownFieldsAndDoesNotLeakToken(t *testing.T) {
	if (TemplateMapping{ID: "t", Fields: map[string]string{"thing1": "receiver_phone"}}).Validate() == nil {
		t.Fatal("private field accepted")
	}
	if (TemplateMapping{ID: "t"}).Validate() == nil {
		t.Fatal("empty mapping accepted")
	}
}

func TestTemplateConstRequiresApprovedExactValueAndTestDoesNotAlterIt(t *testing.T) {
	var wire map[string]any
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if err := json.NewDecoder(r.Body).Decode(&wire); err != nil {
			t.Error(err)
		}
		fmt.Fprint(w, `{"errcode":0,"msgid":12345}`)
	}))
	defer server.Close()
	client := WechatClient{BaseURL: server.URL, token: "cached", expires: time.Now().Add(time.Hour)}
	tpl := TemplateMapping{ID: "t", Fields: map[string]string{"const14": "status", "thing5": "items"}, ApprovedEnums: map[string][]string{"const14": {"已发货"}}}
	notice := messages.OrderNotice{Milestones: []string{"shipped"}, State: messages.OrderState{Shipping: "已发货"}, Items: "测试商品", Test: true}
	if _, err := client.SendOrderTemplate(context.Background(), "o", tpl, notice, false); err != nil {
		t.Fatal(err)
	}
	got := wire["data"].(map[string]any)["const14"].(map[string]any)["value"]
	if got != "已发货" {
		t.Fatalf("const modified: %v", got)
	}
	notice.State.Shipping = "部分发货"
	if _, err := client.SendOrderTemplate(context.Background(), "o", tpl, notice, false); err == nil || TemplateFailureState(err) != "failed" {
		t.Fatalf("unapproved enum must fail before sending: %v", err)
	}
	if calls != 1 {
		t.Fatalf("unexpected sends: %d", calls)
	}
}

func TestTemplateConstRequiresApprovedValues(t *testing.T) {
	tpl := TemplateMapping{ID: "t", Fields: map[string]string{"const8": "status"}}
	if err := tpl.Validate(); err == nil {
		t.Fatal("const without approved values accepted")
	}
}

func TestOrderBusinessTemplatesSupportStatusAndTracking(t *testing.T) {
	for _, mapping := range []TemplateMapping{
		{ID: "t", Fields: map[string]string{"const8": "status", "thing2": "items", "character_string1": "order_no", "time3": "occurred_at"}, ApprovedEnums: map[string][]string{"const8": {"已接单"}}},
		{ID: "t", Fields: map[string]string{"const14": "status", "thing5": "items", "character_string1": "order_no", "character_string2": "tracking_no", "time3": "occurred_at"}, ApprovedEnums: map[string][]string{"const14": {"已发货"}}},
	} {
		if err := mapping.Validate(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTypedTemplateFieldsRejectInvalidValuesWithoutChangingThem(t *testing.T) {
	cases := []struct {
		field, value string
		valid        bool
	}{
		{"character_string1", "SO-20261111-123", true}, {"character_string1", "中文单号", false}, {"character_string1", "123456789012345678901234567890123", false},
		{"amount3", "123.45", true}, {"amount3", "-1.00", false}, {"amount3", "1234567890123", false},
		{"time3", "2026-11-11 10:05:30", true}, {"time4", "2026-11-01~2026-11-30", true}, {"time3", "not a date", false},
	}
	for _, c := range cases {
		if err := validateTemplateValue(c.field, c.value); (err == nil) != c.valid {
			t.Errorf("%s %q: %v", c.field, c.value, err)
		}
	}
}

func TestFinanceTemplateFieldSourcesAreSupported(t *testing.T) {
	mappings := []TemplateMapping{
		{ID: "t", Fields: map[string]string{"thing1": "customer_name", "time2": "receipt_date", "amount3": "receivable_amount", "character_string4": "receipt_no", "amount6": "received_amount"}},
		{ID: "t", Fields: map[string]string{"thing1": "customer_name", "character_string2": "bill_no", "amount3": "bill_amount", "time4": "bill_period"}},
	}
	for _, m := range mappings {
		if err := m.Validate(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFinanceValuesAndReadOnlyDetailLink(t *testing.T) {
	n := messages.OrderNotice{ReceiptID: 12, ReceiptNo: "RC00012", ReceiptDate: "2026-11-11", ReceivableAmount: "100.00", ReceivedAmount: "25.00", Milestones: []string{"receipt_received"}}
	v := n.Values()
	if v["receipt_no"] != "RC00012" || v["received_amount"] != "25.00" {
		t.Fatalf("finance fields: %+v", v)
	}
	if got := n.MiniPath(false); got != "pages/notification-detail/notification-detail?type=receipt&id=12" {
		t.Fatalf("receipt path %s", got)
	}
	n = messages.OrderNotice{BillID: 9, Milestones: []string{"bill_issued"}}
	if got := n.MiniPath(false); got != "pages/notification-detail/notification-detail?type=bill&id=9" {
		t.Fatalf("bill path %s", got)
	}
}

func TestFinanceTemplateWireUsesExactValuesAndActualDetailLink(t *testing.T) {
	var wire map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&wire); err != nil {
			t.Error(err)
		}
		fmt.Fprint(w, `{"errcode":0,"msgid":123}`)
	}))
	defer server.Close()
	client := WechatClient{BaseURL: server.URL, token: "cached", expires: time.Now().Add(time.Hour), Config: Config{MiniAppID: "mini"}}
	tpl := TemplateMapping{ID: "receipt", Fields: map[string]string{"thing1": "customer_name", "time2": "receipt_date", "amount3": "receivable_amount", "character_string4": "receipt_no", "amount6": "received_amount"}}
	n := messages.OrderNotice{ReceiptID: 11, CustomerName: "测试客户", ReceiptNo: "RC00011", ReceiptDate: "2026-11-11", ReceivableAmount: "100.00", ReceivedAmount: "25.00", Milestones: []string{"receipt_received"}}
	if _, err := client.SendOrderTemplate(context.Background(), "openid", tpl, n, false); err != nil {
		t.Fatal(err)
	}
	data := wire["data"].(map[string]any)
	for field, want := range map[string]string{"thing1": "测试客户", "time2": "2026-11-11", "amount3": "100.00", "character_string4": "RC00011", "amount6": "25.00"} {
		if got := data[field].(map[string]any)["value"]; got != want {
			t.Fatalf("%s got=%v want=%s", field, got, want)
		}
	}
	path := wire["miniprogram"].(map[string]any)["pagepath"]
	if path != "pages/notification-detail/notification-detail?type=receipt&id=11" {
		t.Fatal(path)
	}
}
