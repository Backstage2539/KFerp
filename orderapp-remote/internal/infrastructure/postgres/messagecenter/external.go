package messagecenter

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/messagecenter"
	wechat "orderapp/internal/infrastructure/wechataccount"
	"strings"
	"time"
)

type ExternalConfig struct {
	OfficialEnabled bool                              `json:"official_enabled"`
	DesktopEnabled  bool                              `json:"desktop_enabled"`
	OfficialSince   time.Time                         `json:"official_since"`
	DesktopSince    time.Time                         `json:"desktop_since"`
	DesktopID       int64                             `json:"desktop_id"`
	Events          []string                          `json:"events"`
	Templates       map[string]wechat.TemplateMapping `json:"templates"`
}

func DefaultExternalConfig() ExternalConfig {
	return ExternalConfig{Events: []string{"created", "paid", "production_started", "production_finished", "shipped", "cancelled"}, Templates: map[string]wechat.TemplateMapping{}}
}
func (r Repository) ExternalConfig(ctx context.Context) (ExternalConfig, error) {
	c := DefaultExternalConfig()
	var b []byte
	err := r.pool.QueryRow(ctx, "SELECT config FROM "+r.schema+".message_external_settings WHERE id=1").Scan(&b)
	if err == nil {
		err = json.Unmarshal(b, &c)
	}
	return c, err
}
func (r Repository) SaveExternalConfig(ctx context.Context, c ExternalConfig, actor string) error {
	for _, e := range c.Events {
		if app.MilestoneLabels[e] == "" {
			return errors.New("未知通知节点")
		}
		if c.OfficialEnabled {
			if err := c.Templates[e].Validate(); err != nil {
				return fmt.Errorf("%s：%w", app.MilestoneLabels[e], err)
			}
		}
	}
	if len(c.Events) == 0 && (c.OfficialEnabled || c.DesktopEnabled) {
		return errors.New("至少选择一个通知节点")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var raw []byte
	if err = tx.QueryRow(ctx, "SELECT config FROM "+r.schema+".message_external_settings WHERE id=1 FOR UPDATE").Scan(&raw); err != nil {
		return err
	}
	old := DefaultExternalConfig()
	if err = json.Unmarshal(raw, &old); err != nil {
		return err
	}
	c.OfficialSince, c.DesktopSince = old.OfficialSince, old.DesktopSince
	if c.OfficialEnabled && !old.OfficialEnabled {
		c.OfficialSince = time.Now().UTC()
	}
	if c.DesktopEnabled && (!old.DesktopEnabled || old.DesktopID != c.DesktopID) {
		c.DesktopSince = time.Now().UTC()
	}
	if c.DesktopEnabled {
		var ready bool
		err = tx.QueryRow(ctx, "SELECT enabled AND verified AND health='ready' AND last_seen>now()-interval '1 minute' FROM "+r.schema+".message_desktop_devices WHERE id=$1", c.DesktopID).Scan(&ready)
		if err != nil || !ready {
			return errors.New("Mac 尚未在线并完成六类通知实测，不能启用群自动发送")
		}
	}
	b, _ := json.Marshal(c)
	_, err = tx.Exec(ctx, "UPDATE "+r.schema+".message_external_settings SET config=$1,updated_at=now() WHERE id=1", b)
	if err != nil {
		return err
	}
	if err = r.externalAudit(ctx, tx, actor, 1, "notification_config", string(raw), string(b)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (r Repository) externalAudit(ctx context.Context, tx pgx.Tx, actor string, id int64, action, old, new string) error {
	_, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.audit_logs(actor,entity_type,entity_id,action,field,old_value,new_value,meta) VALUES($1,'notification',$2,$3,'notification',$4,$5,'{}')`, r.schema), actor, id, action, old, new)
	return err
}
func randomToken() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func tokenHash(s string) string { v := sha256.Sum256([]byte(s)); return hex.EncodeToString(v[:]) }

type Recipient struct {
	AppID      string    `json:"app_id"`
	OpenID     string    `json:"openid"`
	MiniUserID int64     `json:"mini_user_id"`
	CustomerID int64     `json:"customer_id"`
	EmployeeID int64     `json:"employee_id"`
	BoundAt    time.Time `json:"bound_at"`
	ConsentAt  time.Time `json:"consent_at"`
	Events     []string  `json:"events"`
}

func (r Repository) NotificationRecipients(ctx context.Context, appID string) ([]Recipient, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT app_id,openid,mini_user_id,customer_id,employee_id,bound_at,notification_consent_at,notification_events FROM %s.wechat_bindings WHERE app_id=$1 AND active AND subscribed AND notifications_enabled AND notification_consent_at IS NOT NULL`, r.schema), appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Recipient{}
	for rows.Next() {
		var v Recipient
		var b []byte
		if err = rows.Scan(&v.AppID, &v.OpenID, &v.MiniUserID, &v.CustomerID, &v.EmployeeID, &v.BoundAt, &v.ConsentAt, &b); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(b, &v.Events); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

type Delivery struct {
	ID         int64                  `json:"id"`
	EventID    int64                  `json:"event_id"`
	Channel    string                 `json:"channel"`
	Target     string                 `json:"target"`
	Status     string                 `json:"status"`
	Attempts   int                    `json:"attempts"`
	Error      string                 `json:"error"`
	CreatedAt  time.Time              `json:"created_at"`
	ProviderID string                 `json:"provider_message_id"`
	Lease      string                 `json:"lease,omitempty"`
	Notice     app.OrderNotice        `json:"notice"`
	Recipient  Recipient              `json:"-"`
	Template   wechat.TemplateMapping `json:"-"`
}

func (r Repository) ExternalDeliveries(ctx context.Context) ([]Delivery, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT d.id,d.event_id,d.channel,CASE WHEN d.channel='wechat_desktop' THEN '生产群' ELSE '服务号接收人' END,d.status,d.attempts,d.error,d.created_at,d.provider_message_id,e.payload_json FROM %[1]s.message_deliveries d JOIN %[1]s.message_events e ON e.id=d.event_id WHERE d.lifecycle ORDER BY d.id DESC LIMIT 200`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Delivery{}
	for rows.Next() {
		var d Delivery
		var raw []byte
		if err = rows.Scan(&d.ID, &d.EventID, &d.Channel, &d.Target, &d.Status, &d.Attempts, &d.Error, &d.CreatedAt, &d.ProviderID, &raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &d.Notice); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
func (r Repository) RetryExternal(ctx context.Context, id int64, actor string, confirmUnknown bool) error {
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var status string
	if e = tx.QueryRow(ctx, "SELECT status FROM "+r.schema+".message_deliveries WHERE id=$1 AND lifecycle FOR UPDATE", id).Scan(&status); e != nil {
		return e
	}
	if status != "failed" && !(status == "unknown" && confirmUnknown) {
		return errors.New("仅失败可重试；结果不明须先核对未发送")
	}
	_, e = tx.Exec(ctx, "UPDATE "+r.schema+".message_deliveries SET status='ready',attempts=0,deliver_after=now(),lease_token='',lease_until=NULL,error='',updated_at=now() WHERE id=$1", id)
	if e != nil {
		return e
	}
	if e = r.externalAudit(ctx, tx, actor, id, "notification_retry", status, "ready"); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (r Repository) RecordProviderResult(ctx context.Context, appID, msgID, status string) error {
	if msgID == "" {
		return nil
	}
	if _, err := r.pool.Exec(ctx, "INSERT INTO "+r.schema+".message_provider_receipts(app_id,msg_id,status) VALUES($1,$2,$3) ON CONFLICT(app_id,msg_id) DO NOTHING", appID, msgID, status); err != nil {
		return err
	}
	if err := r.pool.QueryRow(ctx, "SELECT status FROM "+r.schema+".message_provider_receipts WHERE app_id=$1 AND msg_id=$2", appID, msgID).Scan(&status); err != nil {
		return err
	}
	next := "failed"
	if status == "success" {
		next = "delivered"
	}
	_, e := r.pool.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.message_deliveries SET status=$1,error=$2,updated_at=now(),delivered_at=CASE WHEN $1='delivered' THEN now() ELSE NULL END WHERE lifecycle AND channel='wechat_service_account' AND provider_message_id=$3 AND recipient_json->>'app_id'=$4 AND status IN ('accepted','unknown','sending')`, r.schema), next, strings.TrimSpace(status), msgID, appID)
	return e
}

// A human confirmation clears a blocked desktop queue but is not transport
// delivery proof and must never count towards the six-message enablement gate.
func (r Repository) ConfirmExternal(ctx context.Context, id int64, actor string) error {
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	tag, e := tx.Exec(ctx, "UPDATE "+r.schema+".message_deliveries SET status='confirmed',error='管理员核对已发送',updated_at=now() WHERE id=$1 AND lifecycle AND status='unknown'", id)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return errors.New("仅结果不明的通知可人工确认")
	}
	if e = r.externalAudit(ctx, tx, actor, id, "notification_confirm_sent", "unknown", "confirmed"); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
