package wechataccount

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	messages "orderapp/internal/application/messagecenter"
	"testing"
	"time"
)

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
