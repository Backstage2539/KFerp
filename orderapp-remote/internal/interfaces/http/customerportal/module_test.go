package customerportal

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	customerportalapp "orderapp/internal/application/customerportal"
)

func TestStaticIdentityProviderUsesConfiguredStableOpenID(t *testing.T) {
	got, err := (StaticIdentityProvider{OpenID: " dev-openid-van ", UnionID: " dev-union "}).Resolve(context.Background(), "wx-code")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.OpenID != "dev-openid-van" || got.UnionID != "dev-union" {
		t.Fatalf("identity = %+v", got)
	}
}

func TestWechatIdentityProviderExchangesCodeForOpenID(t *testing.T) {
	var gotQuery map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = map[string]string{
			"appid":      r.URL.Query().Get("appid"),
			"secret":     r.URL.Query().Get("secret"),
			"js_code":    r.URL.Query().Get("js_code"),
			"grant_type": r.URL.Query().Get("grant_type"),
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"openid":      "openid-from-wechat",
			"unionid":     "union-from-wechat",
			"session_key": "session-secret",
		})
	}))
	defer server.Close()

	provider := WechatIdentityProvider{
		AppID:     "wx-test-app",
		AppSecret: "wx-test-secret",
		Endpoint:  server.URL,
		Client:    server.Client(),
	}
	got, err := provider.Resolve(context.Background(), "login-code")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.OpenID != "openid-from-wechat" || got.UnionID != "union-from-wechat" {
		t.Fatalf("identity = %+v", got)
	}
	if gotQuery["appid"] != "wx-test-app" || gotQuery["secret"] != "wx-test-secret" || gotQuery["js_code"] != "login-code" || gotQuery["grant_type"] != "authorization_code" {
		t.Fatalf("query = %#v", gotQuery)
	}
}

func TestWechatIdentityProviderMapsWechatError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errcode": 40029,
			"errmsg":  "invalid code",
		})
	}))
	defer server.Close()

	provider := WechatIdentityProvider{AppID: "wx-test-app", AppSecret: "wx-test-secret", Endpoint: server.URL, Client: server.Client()}
	_, err := provider.Resolve(context.Background(), "bad-code")
	if err == nil || errors.Is(err, customerportalapp.ErrMiniLoginDisabled) {
		t.Fatalf("Resolve() err = %v, want provider error distinct from login disabled", err)
	}
}

func TestWechatIdentityProviderResolvesPhoneNumber(t *testing.T) {
	var tokenQuery map[string]string
	var phoneQuery map[string]string
	var phoneBody string
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenQuery = map[string]string{
			"appid":      r.URL.Query().Get("appid"),
			"secret":     r.URL.Query().Get("secret"),
			"grant_type": r.URL.Query().Get("grant_type"),
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "access-token"})
	}))
	defer tokenServer.Close()
	phoneServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		phoneQuery = map[string]string{"access_token": r.URL.Query().Get("access_token")}
		body, _ := io.ReadAll(r.Body)
		phoneBody = string(body)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"phone_info": map[string]any{
				"phoneNumber":     "13800138000",
				"purePhoneNumber": "13800138000",
				"countryCode":     "86",
			},
		})
	}))
	defer phoneServer.Close()

	provider := WechatIdentityProvider{
		AppID:               "wx-test-app",
		AppSecret:           "wx-test-secret",
		AccessTokenEndpoint: tokenServer.URL,
		PhoneNumberEndpoint: phoneServer.URL,
		Client:              tokenServer.Client(),
	}
	got, err := provider.ResolvePhoneNumber(context.Background(), "phone-code")
	if err != nil {
		t.Fatalf("ResolvePhoneNumber() error = %v", err)
	}
	if got.PhoneNumber != "13800138000" || got.PurePhoneNumber != "13800138000" || got.CountryCode != "86" {
		t.Fatalf("phone number = %+v", got)
	}
	if tokenQuery["appid"] != "wx-test-app" || tokenQuery["secret"] != "wx-test-secret" || tokenQuery["grant_type"] != "client_credential" {
		t.Fatalf("token query = %#v", tokenQuery)
	}
	if phoneQuery["access_token"] != "access-token" || phoneBody != `{"code":"phone-code"}` {
		t.Fatalf("phone query=%#v body=%s", phoneQuery, phoneBody)
	}
}
