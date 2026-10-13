package officialaccount

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestPublishMenuPreservesUTF8AndURLSeparators(t *testing.T) {
	for name, menu := range map[string]string{
		"literal":        `{"button":[{"name":"我的订单","type":"miniprogram","appid":"wx-mini","pagepath":"pages/service/service?key=orders&source=official","url":"https://example.com/?a=1&b=2"}]}`,
		"stored_escapes": `{"button":[{"name":"\u6211\u7684\u8ba2\u5355","type":"miniprogram","appid":"wx-mini","pagepath":"pages/service/service?key=orders\u0026source=official","url":"https://example.com/?a=1\u0026b=2"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			var wire []byte
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/cgi-bin/menu/create" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				wire, _ = io.ReadAll(r.Body)
				w.Header().Set("Content-Type", "application/json")
				if bytes.Contains(wire, []byte(`\u`)) {
					io.WriteString(w, `{"errcode":40033,"errmsg":"invalid charset"}`)
					return
				}
				io.WriteString(w, `{"errcode":0,"errmsg":"ok"}`)
			}))
			defer server.Close()
			client := WechatClient{BaseURL: server.URL, token: "test-token", expires: time.Now().Add(time.Hour)}
			if err := client.Publish(context.Background(), json.RawMessage(menu)); err != nil {
				t.Fatalf("publish rejected: %v; wire=%s", err, wire)
			}
			var want, got any
			if err := json.Unmarshal([]byte(menu), &want); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(wire, &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) || !bytes.Contains(wire, []byte("我的订单")) || !bytes.Contains(wire, []byte("&source=official")) {
				t.Fatalf("menu changed in transit: %s", wire)
			}
		})
	}
}

func TestWechatCharsetErrorDoesNotSuggestCredentialChanges(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"errcode":40033,"errmsg":"invalid charset"}`)
	}))
	defer server.Close()
	client := WechatClient{BaseURL: server.URL, token: "test-token", expires: time.Now().Add(time.Hour)}
	err := client.Publish(context.Background(), json.RawMessage(`{"button":[]}`))
	if err == nil || !strings.Contains(err.Error(), "40033") || !strings.Contains(err.Error(), "编码") || strings.Contains(err.Error(), "白名单") {
		t.Fatalf("wrong charset guidance: %v", err)
	}
}
