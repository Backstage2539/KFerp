package messagecenter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
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
func (r Repository) RecordNotificationScan(ctx context.Context, token, appID, openid string, eventTime ...int64) error {
	if len(token) != 64 || openid == "" {
		return errors.New("invalid binding challenge")
	}
	timestamp := time.Now().Unix()
	if len(eventTime) > 0 && eventTime[0] > 0 {
		timestamp = eventTime[0]
	}
	_, e := r.pool.Exec(ctx, "UPDATE "+r.schema+".message_notification_oauth SET app_id=$1,openid=$2,scan_event_time=GREATEST(scan_event_time,$4) WHERE state_hash=$3 AND expires_at>now() AND consumed_at IS NULL AND (openid='' OR openid=$2)", appID, openid, tokenHash(token), timestamp)
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
	if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", mini); e != nil {
		return e
	}
	var openid string
	var boundCustomer, boundEmployee, scanTime int64
	e = tx.QueryRow(ctx, "SELECT openid,customer_id,employee_id,scan_event_time FROM "+r.schema+".message_notification_oauth WHERE state_hash=$1 AND mini_user_id=$2 AND app_id=$3 AND expires_at>now() AND consumed_at IS NULL FOR UPDATE", tokenHash(token), mini, appID).Scan(&openid, &boundCustomer, &boundEmployee, &scanTime)
	if e != nil || openid == "" || boundCustomer != customer || boundEmployee != employee {
		return errors.New("绑定请求已过期或账号已变化，请重新扫码")
	}
	if _, e = tx.Exec(ctx, "INSERT INTO "+r.schema+".message_notification_subscription_events(app_id,openid) VALUES($1,$2) ON CONFLICT DO NOTHING", appID, openid); e != nil {
		return e
	}
	var lastUnsubscribe int64
	if e = tx.QueryRow(ctx, "SELECT last_unsubscribe FROM "+r.schema+".message_notification_subscription_events WHERE app_id=$1 AND openid=$2 FOR UPDATE", appID, openid).Scan(&lastUnsubscribe); e != nil {
		return e
	}
	if scanTime <= 0 || lastUnsubscribe >= scanTime {
		return errors.New("关注状态已变化，请重新扫码")
	}
	// Serialize with unsubscribe before reading a prior choice. A stale choice
	// must never re-enable notifications after the unsubscribe transaction.
	var existingOpenID string
	if e = tx.QueryRow(ctx, "SELECT openid FROM "+r.schema+".wechat_bindings WHERE app_id=$1 AND openid=$2 FOR UPDATE", appID, openid).Scan(&existingOpenID); e != nil && !errors.Is(e, pgx.ErrNoRows) {
		return e
	}
	hasChoice, enabled, events, e := selectedNotificationChoice(ctx, tx, r.schema, appID, mini, customer, employee)
	if e != nil {
		return e
	}
	tag, e := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.wechat_bindings(app_id,openid,mini_user_id,customer_id,employee_id,active,subscribed,notifications_enabled,notification_events,bound_at,notification_consent_at,notification_subscription_time) VALUES($1,$2,$3,$4,$5,true,true,$6,$7,now(),CASE WHEN $6 THEN now() END,$9) ON CONFLICT(app_id,openid) DO UPDATE SET mini_user_id=excluded.mini_user_id,customer_id=excluded.customer_id,employee_id=excluded.employee_id,active=true,subscribed=true,notification_subscription_time=GREATEST(%[1]s.wechat_bindings.notification_subscription_time,excluded.notification_subscription_time),
 notifications_enabled=CASE WHEN NOT $8 AND %[1]s.wechat_bindings.mini_user_id=excluded.mini_user_id AND %[1]s.wechat_bindings.customer_id=excluded.customer_id AND %[1]s.wechat_bindings.employee_id=excluded.employee_id THEN %[1]s.wechat_bindings.notifications_enabled ELSE excluded.notifications_enabled END,
 notification_events=CASE WHEN NOT $8 AND %[1]s.wechat_bindings.mini_user_id=excluded.mini_user_id AND %[1]s.wechat_bindings.customer_id=excluded.customer_id AND %[1]s.wechat_bindings.employee_id=excluded.employee_id THEN %[1]s.wechat_bindings.notification_events ELSE excluded.notification_events END,
 notification_consent_at=CASE WHEN NOT $8 AND %[1]s.wechat_bindings.mini_user_id=excluded.mini_user_id AND %[1]s.wechat_bindings.customer_id=excluded.customer_id AND %[1]s.wechat_bindings.employee_id=excluded.employee_id THEN %[1]s.wechat_bindings.notification_consent_at ELSE excluded.notification_consent_at END,bound_at=now() WHERE NOT %[1]s.wechat_bindings.active OR %[1]s.wechat_bindings.mini_user_id=excluded.mini_user_id`, r.schema), appID, openid, mini, customer, employee, enabled, events, hasChoice, scanTime)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return errors.New("该微信已绑定其他账号，请先解绑")
	}
	if _, e = tx.Exec(ctx, "UPDATE "+r.schema+".message_notification_oauth SET consumed_at=now() WHERE state_hash=$1", tokenHash(token)); e != nil {
		return e
	}
	if e = r.externalAudit(ctx, tx, fmt.Sprint("mini-user:", mini), mini, "notification_binding_consent", "", "服务号绑定已确认"); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

type NotificationPreference struct {
	Active     bool      `json:"active"`
	Enabled    bool      `json:"enabled"`
	Subscribed bool      `json:"subscribed"`
	Events     []string  `json:"events"`
	CustomerID int64     `json:"customer_id"`
	EmployeeID int64     `json:"employee_id"`
	BoundAt    time.Time `json:"bound_at"`
}

func (r Repository) NotificationPreferences(ctx context.Context, appID string, mini int64) ([]NotificationPreference, error) {
	rows, e := r.pool.Query(ctx, "SELECT active,notifications_enabled AND active,subscribed,notification_events,customer_id,employee_id,bound_at FROM "+r.schema+".wechat_bindings WHERE app_id=$1 AND mini_user_id=$2", appID, mini)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []NotificationPreference{}
	for rows.Next() {
		var v NotificationPreference
		var raw []byte
		if e = rows.Scan(&v.Active, &v.Enabled, &v.Subscribed, &raw, &v.CustomerID, &v.EmployeeID, &v.BoundAt); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(raw, &v.Events); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (r Repository) SaveNotificationPreference(ctx context.Context, appID string, mini, customer, employee int64, enabled bool, events []string, expectedVersions ...int64) error {
	return r.saveNotificationPreference(ctx, appID, mini, customer, employee, enabled, events, "", expectedVersions...)
}
func (r Repository) SaveNotificationPreferenceForSession(ctx context.Context, appID string, mini, customer, employee int64, enabled bool, events []string, token string, expectedVersions ...int64) error {
	return r.saveNotificationPreference(ctx, appID, mini, customer, employee, enabled, events, token, expectedVersions...)
}
func (r Repository) saveNotificationPreference(ctx context.Context, appID string, mini, customer, employee int64, enabled bool, events []string, token string, expectedVersions ...int64) error {
	for _, v := range events {
		if app.NotificationLabel(v) == "" {
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
	if token != "" {
		if e = lockNotificationSession(ctx, tx, r.schema, token, mini, customer); e != nil {
			return e
		}
	}
	if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", mini); e != nil {
		return e
	}
	var currentVersion int64
	if e = tx.QueryRow(ctx, "SELECT version FROM "+r.schema+".message_notification_prompt_settings WHERE id=1 FOR UPDATE").Scan(&currentVersion); e != nil {
		return e
	}
	hasVersion := len(expectedVersions) > 0
	choiceVersion := int64(0)
	if hasVersion {
		if expectedVersions[0] != currentVersion {
			return ErrNotificationChoiceStale
		}
		choiceVersion = currentVersion
	}
	tag, e := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.wechat_bindings SET notifications_enabled=$1,notification_events=$2,notification_consent_at=CASE WHEN $1 AND (NOT notifications_enabled OR notification_events IS DISTINCT FROM $2::jsonb) THEN now() ELSE notification_consent_at END WHERE app_id=$3 AND mini_user_id=$4 AND customer_id=$5 AND employee_id=$6 AND (NOT $1 OR active AND subscribed)`, r.schema), enabled, raw, appID, mini, customer, employee)
	if e != nil {
		return e
	}
	if tag.RowsAffected() == 0 {
		return errors.New("请先关注服务号并完成通知绑定")
	}
	if _, e = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.message_notification_choices(app_id,mini_user_id,customer_id,employee_id,version,enabled,events) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(app_id,mini_user_id,customer_id,employee_id) DO UPDATE SET version=CASE WHEN $8 THEN excluded.version ELSE %[1]s.message_notification_choices.version END,enabled=excluded.enabled,events=excluded.events,updated_at=now()`, r.schema), appID, mini, customer, employee, choiceVersion, enabled, raw, hasVersion); e != nil {
		return e
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
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if !subscribed {
		if _, e = tx.Exec(ctx, "INSERT INTO "+r.schema+".message_notification_subscription_events(app_id,openid,last_unsubscribe) VALUES($1,$2,$3) ON CONFLICT(app_id,openid) DO UPDATE SET last_unsubscribe=GREATEST("+r.schema+".message_notification_subscription_events.last_unsubscribe,excluded.last_unsubscribe)", appID, openid, timestamp); e != nil {
			return e
		}
	}
	var mini, customer, employee int64
	e = tx.QueryRow(ctx, "UPDATE "+r.schema+".wechat_bindings SET subscribed=$1,notifications_enabled=CASE WHEN $1 AND subscribed THEN notifications_enabled ELSE false END,active=CASE WHEN $1 THEN active ELSE false END,notification_subscription_time=$4 WHERE app_id=$2 AND openid=$3 AND notification_subscription_time<=$4 RETURNING mini_user_id,customer_id,employee_id", subscribed, appID, openid, timestamp).Scan(&mini, &customer, &employee)
	if errors.Is(e, pgx.ErrNoRows) {
		return tx.Commit(ctx)
	}
	if e != nil {
		return e
	}
	if !subscribed {
		if _, e = tx.Exec(ctx, "UPDATE "+r.schema+".message_notification_choices SET version=0,enabled=false,events='[]'::jsonb,updated_at=now() WHERE app_id=$1 AND mini_user_id=$2 AND customer_id=$3 AND employee_id=$4", appID, mini, customer, employee); e != nil {
			return e
		}
	}
	return tx.Commit(ctx)
}
