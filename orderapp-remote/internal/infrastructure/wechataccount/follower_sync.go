package wechataccount

import (
	"context"
	"errors"
	"log"
	"time"
)

type FollowerStore interface {
	UpsertOfficialFollower(context.Context, string, string, string, bool, int64) error
}

// RunFollowerSync retries full snapshots periodically. Callback events remain
// authoritative for changes that occur after each snapshot starts.
func (w *WechatClient) RunFollowerSync(ctx context.Context, store FollowerStore, reconcile func(context.Context, string) error) {
	if !w.Config.Ready() || !w.Config.UnionIDEnabled {
		return
	}
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if err := w.SyncFollowers(ctx, store, reconcile); err != nil {
				log.Printf("wechat follower sync: %v", err)
			}
			timer.Reset(time.Hour)
		}
	}
}

// SyncFollowers stores only profiles returned as currently subscribed. The start
// time lets newer callback events win over an older paginated snapshot.
func (w *WechatClient) SyncFollowers(ctx context.Context, store FollowerStore, reconcile func(context.Context, string) error) error {
	started := time.Now().Unix()
	cursor := ""
	seen := map[string]bool{}
	for {
		ids, next, err := w.FollowerPage(ctx, cursor)
		if err != nil {
			return err
		}
		for start := 0; start < len(ids); start += 100 {
			end := start + 100
			if end > len(ids) {
				end = len(ids)
			}
			profiles, err := w.FollowerProfiles(ctx, ids[start:end])
			if err != nil {
				return err
			}
			for _, profile := range profiles {
				if !profile.Subscribed {
					continue
				}
				if err := store.UpsertOfficialFollower(ctx, w.Config.AppID, profile.OpenID, profile.UnionID, true, started); err != nil {
					return err
				}
				if reconcile != nil && profile.UnionID != "" {
					if err := reconcile(ctx, profile.UnionID); err != nil {
						return err
					}
				}
			}
		}
		if len(ids) == 0 || next == "" {
			return nil
		}
		if next == cursor || seen[next] {
			return errors.New("关注者分页游标重复")
		}
		seen[next] = true
		cursor = next
	}
}
