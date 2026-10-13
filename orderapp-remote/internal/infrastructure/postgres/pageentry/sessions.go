package pageentry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	app "orderapp/internal/application/pageentry"
	"time"
)

func Hash(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }

type Session struct {
	MiniToken          string
	UserID, CustomerID int64
	AppID, OpenID      string
	BoundAt            *time.Time
	CreatedAt          time.Time
}

func (r Repository) NewSession(ctx context.Context, s Session) (string, error) {
	token, err := app.NewKey()
	if err != nil {
		return "", err
	}
	extra, err := app.NewKey()
	if err != nil {
		return "", err
	}
	token += extra
	_, err = r.Pool.Exec(ctx, r.q(`INSERT INTO %[1]s.page_web_sessions(token_hash,mini_token,mini_user_id,customer_id,app_id,openid,bound_at,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,now()+interval '7 days')`), Hash(token), s.MiniToken, s.UserID, s.CustomerID, s.AppID, s.OpenID, s.BoundAt)
	return token, err
}
func (r Repository) Session(ctx context.Context, token string) (s Session, err error) {
	if len(token) != 64 {
		return s, app.ErrDenied
	}
	err = r.Pool.QueryRow(ctx, r.q(`SELECT mini_token,mini_user_id,customer_id,app_id,openid,bound_at,created_at FROM %[1]s.page_web_sessions WHERE token_hash=$1 AND expires_at>now()`), Hash(token)).Scan(&s.MiniToken, &s.UserID, &s.CustomerID, &s.AppID, &s.OpenID, &s.BoundAt, &s.CreatedAt)
	return
}
func (r Repository) EndSession(ctx context.Context, token string) error {
	_, err := r.Pool.Exec(ctx, r.q(`DELETE FROM %[1]s.page_web_sessions WHERE token_hash=$1`), Hash(token))
	return err
}
func (r Repository) SelectCustomer(ctx context.Context, token string, id int64) error {
	_, err := r.Pool.Exec(ctx, r.q(`UPDATE %[1]s.page_web_sessions SET customer_id=$2 WHERE token_hash=$1 AND expires_at>now()`), Hash(token), id)
	return err
}
func (r Repository) OAuthState(ctx context.Context, state, browser, key string) error {
	_, err := r.Pool.Exec(ctx, r.q(`WITH cleaned AS (DELETE FROM %[1]s.page_oauth_states WHERE expires_at<now()) INSERT INTO %[1]s.page_oauth_states(state_hash,browser_hash,entry_key,expires_at) VALUES($1,$2,$3,now()+interval '5 minutes')`), Hash(state), Hash(browser), key)
	return err
}
func (r Repository) ConsumeState(ctx context.Context, state, browser string) (key string, err error) {
	if !app.ValidKey(state) || !app.ValidKey(browser) {
		return "", app.ErrDenied
	}
	err = r.Pool.QueryRow(ctx, r.q(`DELETE FROM %[1]s.page_oauth_states WHERE state_hash=$1 AND browser_hash=$2 AND expires_at>now() RETURNING entry_key`), Hash(state), Hash(browser)).Scan(&key)
	return
}
func (r Repository) AllowLogin(ctx context.Context, identity string) (bool, error) {
	var n int
	err := r.Pool.QueryRow(ctx, r.q(`INSERT INTO %[1]s.page_login_attempts(identity_hash,attempts) VALUES($1,1) ON CONFLICT(identity_hash) DO UPDATE SET attempts=CASE WHEN %[1]s.page_login_attempts.bucket<now()-interval '5 minutes' THEN 1 ELSE %[1]s.page_login_attempts.attempts+1 END,bucket=CASE WHEN %[1]s.page_login_attempts.bucket<now()-interval '5 minutes' THEN now() ELSE %[1]s.page_login_attempts.bucket END RETURNING attempts`), Hash(identity)).Scan(&n)
	return n <= 10, err
}
