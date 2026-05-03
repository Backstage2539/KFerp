package customerportal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	customerportalapp "orderapp/internal/application/customerportal"

	"github.com/labstack/echo/v4"
)

type Service interface {
	Login(context.Context, customerportalapp.LoginCommand) (customerportalapp.LoginResult, error)
	Me(context.Context, string) (customerportalapp.CurrentContext, error)
	SwitchCurrentCustomer(context.Context, string, int64) (customerportalapp.CurrentContext, error)
	GetServicePage(context.Context, string, string) (customerportalapp.ServicePage, error)
	CreateDirectShipBatch(context.Context, string, customerportalapp.CreateDirectShipBatchCommand) (customerportalapp.DirectShipBatch, error)
	CreateProcessingRequest(context.Context, string, customerportalapp.CreateProcessingRequestCommand) (customerportalapp.ProcessingRequest, error)
}

type Dependencies struct {
	CustomerPortal Service
}

func RegisterRoutes(e *echo.Echo, deps Dependencies) {
	registerMiniAPI(e, deps.CustomerPortal)
}

type StaticIdentityProvider struct {
	OpenID  string
	UnionID string
}

func (p StaticIdentityProvider) Resolve(ctx context.Context, code string) (customerportalapp.MiniIdentity, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return customerportalapp.MiniIdentity{}, fmt.Errorf("code required")
	}
	openID := strings.TrimSpace(p.OpenID)
	if openID == "" {
		openID = "dev-openid-" + code
	}
	return customerportalapp.MiniIdentity{OpenID: openID, UnionID: strings.TrimSpace(p.UnionID)}, nil
}

type DisabledIdentityProvider struct{}

func (DisabledIdentityProvider) Resolve(ctx context.Context, code string) (customerportalapp.MiniIdentity, error) {
	return customerportalapp.MiniIdentity{}, customerportalapp.ErrMiniLoginDisabled
}

const defaultWechatCode2SessionEndpoint = "https://api.weixin.qq.com/sns/jscode2session"

type WechatIdentityProvider struct {
	AppID     string
	AppSecret string
	Endpoint  string
	Client    *http.Client
}

type wechatCode2SessionResponse struct {
	OpenID  string `json:"openid"`
	UnionID string `json:"unionid"`
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

func (p WechatIdentityProvider) Resolve(ctx context.Context, code string) (customerportalapp.MiniIdentity, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return customerportalapp.MiniIdentity{}, fmt.Errorf("code required")
	}
	appID := strings.TrimSpace(p.AppID)
	appSecret := strings.TrimSpace(p.AppSecret)
	if appID == "" || appSecret == "" {
		return customerportalapp.MiniIdentity{}, fmt.Errorf("wechat mini app credentials required")
	}
	endpoint := strings.TrimSpace(p.Endpoint)
	if endpoint == "" {
		endpoint = defaultWechatCode2SessionEndpoint
	}
	reqURL, err := url.Parse(endpoint)
	if err != nil {
		return customerportalapp.MiniIdentity{}, err
	}
	q := reqURL.Query()
	q.Set("appid", appID)
	q.Set("secret", appSecret)
	q.Set("js_code", code)
	q.Set("grant_type", "authorization_code")
	reqURL.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return customerportalapp.MiniIdentity{}, err
	}
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return customerportalapp.MiniIdentity{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return customerportalapp.MiniIdentity{}, fmt.Errorf("wechat code2session http status %d", resp.StatusCode)
	}
	var payload wechatCode2SessionResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return customerportalapp.MiniIdentity{}, err
	}
	if payload.ErrCode != 0 {
		return customerportalapp.MiniIdentity{}, fmt.Errorf("wechat code2session failed: %d %s", payload.ErrCode, payload.ErrMsg)
	}
	openID := strings.TrimSpace(payload.OpenID)
	if openID == "" {
		return customerportalapp.MiniIdentity{}, fmt.Errorf("openid required")
	}
	return customerportalapp.MiniIdentity{OpenID: openID, UnionID: strings.TrimSpace(payload.UnionID)}, nil
}
