package officialaccount

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type Config struct {
	Enabled                                    bool
	UnionIDEnabled                             bool
	MenuPublishEnabled                         bool
	AppID, AppSecret, Token, AESKey, MiniAppID string
}

func ConfigFromEnv(miniID string) Config {
	return Config{Enabled: os.Getenv("WECHAT_OFFICIAL_ENABLED") == "1", UnionIDEnabled: os.Getenv("WECHAT_OFFICIAL_UNIONID_ENABLED") == "1", MenuPublishEnabled: os.Getenv("WECHAT_OFFICIAL_MENU_PUBLISH_ENABLED") == "1", AppID: os.Getenv("WECHAT_OFFICIAL_APP_ID"), AppSecret: os.Getenv("WECHAT_OFFICIAL_APP_SECRET"), Token: os.Getenv("WECHAT_OFFICIAL_TOKEN"), AESKey: os.Getenv("WECHAT_OFFICIAL_AES_KEY"), MiniAppID: miniID}
}
func (c Config) Ready() bool {
	key, err := base64.StdEncoding.DecodeString(c.AESKey + "=")
	return err == nil && len(key) == 32 && c.Enabled && c.AppID != "" && c.AppSecret != "" && len(c.Token) >= 16 && len(c.AESKey) == 43 && c.MiniAppID != ""
}

type WechatClient struct {
	Config  Config
	BaseURL string
	Client  *http.Client
	mu      sync.Mutex
	token   string
	expires time.Time
}

func (w *WechatClient) call(ctx context.Context, path string, query url.Values, body any, result any) error {
	base := w.BaseURL
	if base == "" {
		base = "https://api.weixin.qq.com"
	}
	var reader io.Reader
	method := http.MethodGet
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
		method = http.MethodPost
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path+"?"+query.Encode(), reader)
	if err != nil {
		return errors.New("微信接口地址无效")
	}
	req.Header.Set("Content-Type", "application/json")
	client := w.Client
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return errors.New("微信接口暂时无法连接")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("微信接口 HTTP %d", res.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return err
	}
	var apiErr struct {
		Code int `json:"errcode"`
	}
	if err = json.Unmarshal(raw, &apiErr); err != nil {
		return errors.New("微信接口返回格式异常")
	}
	if apiErr.Code != 0 {
		return fmt.Errorf("微信接口错误 %d，请检查服务号权限和服务器 IP 白名单", apiErr.Code)
	}
	return json.Unmarshal(raw, result)
}
func (w *WechatClient) accessToken(ctx context.Context) (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if time.Now().Before(w.expires) {
		return w.token, nil
	}
	var out struct {
		Token   string `json:"access_token"`
		Expires int    `json:"expires_in"`
	}
	err := w.call(ctx, "/cgi-bin/token", url.Values{"grant_type": {"client_credential"}, "appid": {w.Config.AppID}, "secret": {w.Config.AppSecret}}, nil, &out)
	if err != nil {
		return "", err
	}
	if out.Token == "" {
		return "", errors.New("未取得公众号调用凭证")
	}
	w.token = out.Token
	w.expires = time.Now().Add(time.Duration(out.Expires-120) * time.Second)
	return w.token, nil
}
func (w *WechatClient) UnionID(ctx context.Context, openid string) (string, error) {
	t, err := w.accessToken(ctx)
	if err != nil {
		return "", err
	}
	var out struct {
		UnionID   string `json:"unionid"`
		Subscribe int    `json:"subscribe"`
	}
	err = w.call(ctx, "/cgi-bin/user/info", url.Values{"access_token": {t}, "openid": {openid}, "lang": {"zh_CN"}}, nil, &out)
	if out.Subscribe != 1 {
		return "", errors.New("请先关注公众号")
	}
	return strings.TrimSpace(out.UnionID), err
}
func (w *WechatClient) Menu(ctx context.Context) (json.RawMessage, error) {
	t, err := w.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	var out struct {
		Menu json.RawMessage `json:"selfmenu_info"`
	}
	err = w.call(ctx, "/cgi-bin/get_current_selfmenu_info", url.Values{"access_token": {t}}, nil, &out)
	if len(out.Menu) == 0 {
		out.Menu = json.RawMessage(`{"button":[]}`)
	}
	return out.Menu, err
}
func (w *WechatClient) Publish(ctx context.Context, menu json.RawMessage) error {
	t, err := w.accessToken(ctx)
	if err != nil {
		return err
	}
	var out map[string]any
	return w.call(ctx, "/cgi-bin/menu/create", url.Values{"access_token": {t}}, menu, &out)
}
