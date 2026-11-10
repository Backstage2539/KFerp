package messagecenter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	app "orderapp/internal/application/messagecenter"
	"time"
)

func (r Repository) StartNotificationBinding(ctx context.Context, mini, customer, employee int64) (string, error) {
	if mini <= 0 || (customer <= 0 && employee <= 0) {
		return "", errors.New("请先登录客户或员工账号")
	}
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return "", e
	}
	defer tx.Rollback(ctx)
	// Serialize issuance for the same authenticated mini account without storing a secret.
	if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", mini); e != nil {
		return "", e
	}
	var recent int
	if e = tx.QueryRow(ctx, "SELECT count(*) FROM "+r.schema+".message_notification_oauth WHERE mini_user_id=$1 AND expires_at>now()-interval '5 minutes'", mini).Scan(&recent); e != nil {
		return "", e
	}
	if recent >= 5 {
		return "", errors.New("请求过于频繁，请稍后重新生成二维码")
	}
	token := randomToken()
	if _, e = tx.Exec(ctx, "INSERT INTO "+r.schema+".message_notification_oauth(state_hash,mini_user_id,employee_id,customer_id,expires_at) VALUES($1,$2,$3,$4,now()+interval '5 minutes')", tokenHash(token), mini, employee, customer); e != nil {
		return "", e
	}
	if e = r.externalAudit(ctx, tx, fmt.Sprint("mini-user:", mini), mini, "notification_binding_start", "", "生成关注确认请求"); e != nil {
		return "", e
	}
	return token, tx.Commit(ctx)
}
func (r Repository) RecordNotificationScan(ctx context.Context, token, appID, openid string) error {
	if len(token) != 64 || openid == "" {
		return errors.New("invalid binding challenge")
	}
	_, e := r.pool.Exec(ctx, "UPDATE "+r.schema+".message_notification_oauth SET app_id=$1,openid=$2 WHERE state_hash=$3 AND expires_at>now() AND consumed_at IS NULL AND (openid='' OR openid=$2)", appID, openid, tokenHash(token))
	return e
}
func (r Repository) BindingChallenge(ctx context.Context, token string, mini int64) (string, error) {
	var openid string
	e := r.pool.QueryRow(ctx, "SELECT openid FROM "+r.schema+".message_notification_oauth WHERE state_hash=$1 AND mini_user_id=$2 AND expires_at>now() AND consumed_at IS NULL", tokenHash(token), mini).Scan(&openid)
	return openid, e
}
func (r Repository) ConfirmNotificationBinding(ctx context.Context, token, appID string, mini, customer, employee int64) error {
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var openid string
	var boundCustomer, boundEmployee int64
	e = tx.QueryRow(ctx, "SELECT openid,customer_id,employee_id FROM "+r.schema+".message_notification_oauth WHERE state_hash=$1 AND mini_user_id=$2 AND app_id=$3 AND expires_at>now() AND consumed_at IS NULL FOR UPDATE", tokenHash(token), mini, appID).Scan(&openid, &boundCustomer, &boundEmployee)
	if e != nil || openid == "" || boundCustomer != customer || boundEmployee != employee {
		return errors.New("绑定请求已过期或账号已变化，请重新扫码")
	}
	tag, e := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.wechat_bindings(app_id,openid,mini_user_id,customer_id,employee_id,active,subscribed,notifications_enabled,bound_at,notification_consent_at) VALUES($1,$2,$3,$4,$5,true,true,true,now(),now()) ON CONFLICT(app_id,openid) DO UPDATE SET mini_user_id=excluded.mini_user_id,customer_id=excluded.customer_id,employee_id=excluded.employee_id,active=true,subscribed=true,notifications_enabled=true,bound_at=now(),notification_consent_at=now() WHERE NOT %[1]s.wechat_bindings.active OR %[1]s.wechat_bindings.mini_user_id=excluded.mini_user_id`, r.schema), appID, openid, mini, customer, employee)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return errors.New("该微信已绑定其他账号，请先解绑")
	}
	if _, e = tx.Exec(ctx, "UPDATE "+r.schema+".message_notification_oauth SET consumed_at=now() WHERE state_hash=$1", tokenHash(token)); e != nil {
		return e
	}
	if e = r.externalAudit(ctx, tx, fmt.Sprint("mini-user:", mini), mini, "notification_binding_consent", "", "订单通知已开启"); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

type NotificationPreference struct {
	Enabled    bool      `json:"enabled"`
	Subscribed bool      `json:"subscribed"`
	Events     []string  `json:"events"`
	CustomerID int64     `json:"customer_id"`
	EmployeeID int64     `json:"employee_id"`
	BoundAt    time.Time `json:"bound_at"`
}

func (r Repository) NotificationPreferences(ctx context.Context, appID string, mini int64) ([]NotificationPreference, error) {
	rows, e := r.pool.Query(ctx, "SELECT notifications_enabled AND active,subscribed,notification_events,customer_id,employee_id,bound_at FROM "+r.schema+".wechat_bindings WHERE app_id=$1 AND mini_user_id=$2", appID, mini)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []NotificationPreference{}
	for rows.Next() {
		var v NotificationPreference
		var raw []byte
		if e = rows.Scan(&v.Enabled, &v.Subscribed, &raw, &v.CustomerID, &v.EmployeeID, &v.BoundAt); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(raw, &v.Events); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (r Repository) SaveNotificationPreference(ctx context.Context, appID string, mini, customer, employee int64, enabled bool, events []string) error {
	for _, v := range events {
		if app.MilestoneLabels[v] == "" {
			return errors.New("未知通知节点")
		}
	}
	if enabled && len(events) == 0 {
		return errors.New("请选择通知节点")
	}
	raw, _ := json.Marshal(events)
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	tag, e := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.wechat_bindings SET notifications_enabled=$1,notification_events=$2,notification_consent_at=CASE WHEN $1 AND NOT notifications_enabled THEN now() ELSE notification_consent_at END WHERE app_id=$3 AND mini_user_id=$4 AND customer_id=$5 AND employee_id=$6 AND (NOT $1 OR active AND subscribed)`, r.schema), enabled, raw, appID, mini, customer, employee)
	if e != nil {
		return e
	}
	if tag.RowsAffected() == 0 {
		return errors.New("请先关注服务号并完成通知绑定")
	}
	if e = r.externalAudit(ctx, tx, fmt.Sprint("mini-user:", mini), mini, "notification_preference", "", fmt.Sprint(enabled)); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (r Repository) SubscriptionEvent(ctx context.Context, appID, openid string, subscribed bool, eventTime ...int64) error {
	timestamp := time.Now().Unix()
	if len(eventTime) > 0 {
		timestamp = eventTime[0]
	}
	_, e := r.pool.Exec(ctx, "UPDATE "+r.schema+".wechat_bindings SET subscribed=$1,notifications_enabled=CASE WHEN $1 AND subscribed THEN notifications_enabled ELSE false END,active=CASE WHEN $1 THEN active ELSE false END,notification_subscription_time=$4 WHERE app_id=$2 AND openid=$3 AND notification_subscription_time<=$4", subscribed, appID, openid, timestamp)
	return e
}
