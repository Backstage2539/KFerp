package wechataccount

import (
	"context"
	"errors"
	"net/url"
)

type FollowerProfile struct {
	OpenID     string
	UnionID    string
	Subscribed bool
}

func (w *WechatClient) FollowerPage(ctx context.Context, cursor string) ([]string, string, error) {
	token, err := w.accessToken(ctx)
	if err != nil {
		return nil, "", err
	}
	var out struct {
		Count int `json:"count"`
		Data  struct {
			OpenIDs []string `json:"openid"`
		} `json:"data"`
		Next string `json:"next_openid"`
	}
	err = w.call(ctx, "/cgi-bin/user/get", url.Values{"access_token": {token}, "next_openid": {cursor}}, nil, &out)
	if err != nil {
		return nil, "", err
	}
	if out.Count != len(out.Data.OpenIDs) {
		return nil, "", errors.New("关注者列表返回不完整")
	}
	return out.Data.OpenIDs, out.Next, nil
}

func (w *WechatClient) FollowerProfiles(ctx context.Context, ids []string) ([]FollowerProfile, error) {
	if len(ids) == 0 || len(ids) > 100 {
		return nil, errors.New("关注者批次数量无效")
	}
	token, err := w.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	users := make([]map[string]string, 0, len(ids))
	for _, id := range ids {
		users = append(users, map[string]string{"openid": id, "lang": "zh_CN"})
	}
	var out struct {
		Users []struct {
			OpenID    string `json:"openid"`
			UnionID   string `json:"unionid"`
			Subscribe int    `json:"subscribe"`
		} `json:"user_info_list"`
	}
	if err := w.call(ctx, "/cgi-bin/user/info/batchget", url.Values{"access_token": {token}}, map[string]any{"user_list": users}, &out); err != nil {
		return nil, err
	}
	if len(out.Users) != len(ids) {
		return nil, errors.New("关注者资料返回不完整")
	}
	profiles := make([]FollowerProfile, 0, len(ids))
	for _, user := range out.Users {
		profiles = append(profiles, FollowerProfile{OpenID: user.OpenID, UnionID: user.UnionID, Subscribed: user.Subscribe == 1})
	}
	return profiles, nil
}
