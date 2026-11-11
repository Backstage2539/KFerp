package messagecenter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/messagecenter"
)

type NotificationChoice struct {
	Enabled bool     `json:"enabled"`
	Events  []string `json:"events"`
}
type NotificationPromptState struct {
	Required bool                `json:"prompt_required"`
	Version  int64               `json:"prompt_version"`
	Choice   *NotificationChoice `json:"choice"`
}

var ErrNotificationChoiceStale = errors.New("通知选项已更新，请重新选择")
var ErrNotificationIdentityChanged = errors.New("当前账号或客户已变化，请重新选择")

func (r Repository) NotificationPrompt(ctx context.Context, appID string, mini, customer, employee int64) (NotificationPromptState, error) {
	var s NotificationPromptState
	var version *int64
	var enabled *bool
	var events []byte
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT p.version,c.version,c.enabled,c.events FROM %[1]s.message_notification_prompt_settings p LEFT JOIN %[1]s.message_notification_choices c ON c.app_id=$1 AND c.mini_user_id=$2 AND c.customer_id=$3 AND c.employee_id=$4 WHERE p.id=1`, r.schema), appID, mini, customer, employee).Scan(&s.Version, &version, &enabled, &events)
	if err != nil {
		return s, err
	}
	s.Required = version == nil || *version < s.Version
	if version != nil {
		s.Choice = &NotificationChoice{Enabled: *enabled}
		if err = json.Unmarshal(events, &s.Choice.Events); err != nil {
			return s, err
		}
	}
	return s, nil
}

// SaveNotificationChoice records an explicit decision even before WeChat binding;
// the choice never creates or changes a binding on its own.
func (r Repository) SaveNotificationChoice(ctx context.Context, appID string, mini, customer, employee, version int64, enabled bool, events []string, sessionTokens ...string) error {
	if appID == "" || mini <= 0 || ((customer > 0) == (employee > 0)) || version <= 0 {
		return errors.New("通知选择无效")
	}
	seen := map[string]bool{}
	for _, v := range events {
		if app.NotificationLabel(v) == "" || seen[v] {
			return errors.New("通知类型无效")
		}
		seen[v] = true
	}
	if enabled && len(events) == 0 {
		return errors.New("请选择通知类型")
	}
	if !enabled {
		events = []string{}
	}
	if events == nil {
		events = []string{}
	}
	raw, err := json.Marshal(events)
	if err != nil {
		return err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if len(sessionTokens) > 0 {
		if err = lockNotificationSession(ctx, tx, r.schema, sessionTokens[0], mini, customer); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", mini); err != nil {
		return err
	}
	var current int64
	if err = tx.QueryRow(ctx, "SELECT version FROM "+r.schema+".message_notification_prompt_settings WHERE id=1 FOR UPDATE").Scan(&current); err != nil {
		return err
	}
	if current != version {
		return ErrNotificationChoiceStale
	}
	// Keep binding-before-choice lock order aligned with unsubscribe callbacks.
	_, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.wechat_bindings SET notifications_enabled=CASE WHEN $1 THEN active AND subscribed ELSE false END,notification_events=$2,notification_consent_at=CASE WHEN $1 AND active AND subscribed THEN now() ELSE notification_consent_at END WHERE app_id=$3 AND mini_user_id=$4 AND customer_id=$5 AND employee_id=$6`, r.schema), enabled, raw, appID, mini, customer, employee)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.message_notification_choices(app_id,mini_user_id,customer_id,employee_id,version,enabled,events) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(app_id,mini_user_id,customer_id,employee_id) DO UPDATE SET version=excluded.version,enabled=excluded.enabled,events=excluded.events,updated_at=now()`, r.schema), appID, mini, customer, employee, version, enabled, raw)
	if err != nil {
		return err
	}
	if err = r.externalAudit(ctx, tx, fmt.Sprint("mini-user:", mini), mini, "notification_choice", "", fmt.Sprintf("enabled=%t version=%d", enabled, version)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func lockNotificationSession(ctx context.Context, tx pgx.Tx, schema, token string, mini, customer int64) error {
	if token == "" {
		return ErrNotificationIdentityChanged
	}
	var currentMini, currentCustomer int64
	err := tx.QueryRow(ctx, "SELECT mini_user_id,COALESCE(current_customer_id,0) FROM "+schema+".mini_sessions WHERE token=$1 AND expire_at>now() FOR UPDATE", token).Scan(&currentMini, &currentCustomer)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotificationIdentityChanged
	}
	if err != nil {
		return err
	}
	if currentMini != mini || (customer > 0 && currentCustomer != customer) {
		return ErrNotificationIdentityChanged
	}
	return nil
}

func (r Repository) RequireNotificationReselect(ctx context.Context, actor string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var version int64
	if err = tx.QueryRow(ctx, "UPDATE "+r.schema+".message_notification_prompt_settings SET version=version+1 WHERE id=1 RETURNING version").Scan(&version); err != nil {
		return err
	}
	if err = r.externalAudit(ctx, tx, actor, 1, "notification_prompt_reselect", "", fmt.Sprint(version)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func selectedNotificationChoice(ctx context.Context, tx pgx.Tx, schema, appID string, mini, customer, employee int64) (bool, bool, []byte, error) {
	var enabled bool
	var raw []byte
	err := tx.QueryRow(ctx, "SELECT enabled,events FROM "+schema+".message_notification_choices WHERE app_id=$1 AND mini_user_id=$2 AND customer_id=$3 AND employee_id=$4", appID, mini, customer, employee).Scan(&enabled, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, false, []byte("[]"), nil
	}
	return err == nil, enabled, raw, err
}
