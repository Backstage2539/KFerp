package wechatweb

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	app "orderapp/internal/application/pageentry"
	"os"
	"strings"
	"time"
)

type OAuthIdentity struct{ OpenID, UnionID string }
type Config struct {
	OAuthEnabled, UnionIDEnabled   bool
	PublicOrigin, AppID, AppSecret string
	Exchange                       func(context.Context, string) (OAuthIdentity, error)
}

func ConfigFromEnv() Config {
	return Config{OAuthEnabled: os.Getenv("WECHAT_WEB_OAUTH_ENABLED") == "1", UnionIDEnabled: os.Getenv("WECHAT_OFFICIAL_UNIONID_ENABLED") == "1", PublicOrigin: strings.TrimRight(os.Getenv("WECHAT_WEB_PUBLIC_ORIGIN"), "/"), AppID: os.Getenv("WECHAT_OFFICIAL_APP_ID"), AppSecret: os.Getenv("WECHAT_OFFICIAL_APP_SECRET")}
}
func (c Config) Ready() bool {
	u, err := url.Parse(c.PublicOrigin)
	return c.OAuthEnabled && c.AppID != "" && c.AppSecret != "" && err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && u.Path == ""
}
func (c Config) ExchangeCode(ctx context.Context, code string) (OAuthIdentity, error) {
	if c.Exchange != nil {
		return c.Exchange(ctx, code)
	}
	q := url.Values{"appid": {c.AppID}, "secret": {c.AppSecret}, "code": {code}, "grant_type": {"authorization_code"}}
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.weixin.qq.com/sns/oauth2/access_token?"+q.Encode(), nil)
	if err != nil {
		return OAuthIdentity{}, app.ErrDenied
	}
	res, err := (&http.Client{Timeout: 4 * time.Second}).Do(req)
	if err != nil {
		return OAuthIdentity{}, app.ErrDenied
	}
	defer res.Body.Close()
	var v struct {
		OpenID  string `json:"openid"`
		UnionID string `json:"unionid"`
		Error   int    `json:"errcode"`
	}
	if res.StatusCode != 200 || json.NewDecoder(io.LimitReader(res.Body, 65536)).Decode(&v) != nil || v.Error != 0 || v.OpenID == "" {
		return OAuthIdentity{}, app.ErrDenied
	}
	return OAuthIdentity{v.OpenID, v.UnionID}, nil
}
