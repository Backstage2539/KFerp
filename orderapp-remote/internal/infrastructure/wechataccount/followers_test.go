package wechataccount

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

type followerTestStore struct {
	profiles   []FollowerProfile
	timestamps []int64
}

func (s *followerTestStore) UpsertOfficialFollower(_ context.Context, app, openid, unionid string, subscribed bool, ts int64) error {
	s.profiles = append(s.profiles, FollowerProfile{OpenID: openid, UnionID: unionid, Subscribed: subscribed})
	s.timestamps = append(s.timestamps, ts)
	return nil
}

func TestSyncFollowersAcrossPages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/stable_token":
			fmt.Fprint(w, `{"access_token":"token","expires_in":7200}`)
		case "/cgi-bin/user/get":
			if r.URL.Query().Get("next_openid") == "" {
				fmt.Fprint(w, `{"count":1,"data":{"openid":["one"]},"next_openid":"one"}`)
			} else {
				fmt.Fprint(w, `{"count":0,"data":{"openid":[]}}`)
			}
		case "/cgi-bin/user/info/batchget":
			fmt.Fprint(w, `{"user_info_list":[{"openid":"one","unionid":"union","subscribe":1}]}`)
		default:
			t.Errorf("unexpected %s", r.URL.Path)
		}
	}))
	defer server.Close()
	client := &WechatClient{BaseURL: server.URL, Client: server.Client(), Config: Config{AppID: "test"}}
	store := &followerTestStore{}
	if err := client.SyncFollowers(context.Background(), store, nil); err != nil {
		t.Fatal(err)
	}
	if len(store.profiles) != 1 || store.profiles[0].UnionID != "union" || store.timestamps[0] <= 0 {
		t.Fatalf("stored %+v", store)
	}
}

func TestFollowerListAndProfiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/stable_token":
			w.Write([]byte(`{"access_token":"token","expires_in":7200}`))
		case "/cgi-bin/user/get":
			if r.URL.Query().Get("next_openid") != "cursor" {
				t.Error("missing cursor")
			}
			w.Write([]byte(`{"count":2,"data":{"openid":["first","second"]},"next_openid":"next"}`))
		case "/cgi-bin/user/info/batchget":
			w.Write([]byte(`{"user_info_list":[{"openid":"first","unionid":"u1","subscribe":1},{"openid":"second","subscribe":0}]}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	client := &WechatClient{BaseURL: server.URL, Client: server.Client()}
	ids, next, err := client.FollowerPage(context.Background(), "cursor")
	if err != nil || len(ids) != 2 || next != "next" {
		t.Fatalf("page %v %q %v", ids, next, err)
	}
	users, err := client.FollowerProfiles(context.Background(), ids)
	if err != nil || len(users) != 2 || users[0].UnionID != "u1" || users[1].Subscribed {
		t.Fatalf("profiles %+v %v", users, err)
	}
}
