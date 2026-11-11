package messagecenter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"log"
	app "orderapp/internal/application/messagecenter"
	wechat "orderapp/internal/infrastructure/wechataccount"
	"strconv"
	"time"
)

type OrderSender interface {
	SendOrderTemplate(context.Context, string, wechat.TemplateMapping, app.OrderNotice, bool) (string, error)
}
type Worker struct {
	Repo      Repository
	AppID     string
	Sender    OrderSender
	Authorize func(context.Context, Recipient, app.OrderNotice) (bool, error)
}

func filterEvents(events, allowed []string) []string {
	var out []string
	for _, v := range events {
		for _, a := range allowed {
			if v == a {
				out = append(out, v)
				break
			}
		}
	}
	return out
}
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.Tick(ctx); err != nil {
				log.Printf("order notification worker: %v", err)
			}
		}
	}
}
func (w *Worker) Tick(ctx context.Context) error {
	if err := w.CaptureCommitted(ctx); err != nil {
		return err
	}
	if err := w.CaptureBusinessCommitted(ctx); err != nil {
		return err
	}
	// A process may have died after sending. Never blindly retry an expired lease.
	if _, err := w.Repo.pool.Exec(ctx, "UPDATE "+w.Repo.schema+".message_deliveries SET status='unknown',error='发送期间进程中断，请核对后处理',updated_at=now() WHERE lifecycle AND status='sending' AND lease_until<now()"); err != nil {
		return err
	}
	for i := 0; i < 20; i++ {
		d, err := w.Repo.ClaimExternal(ctx, "wechat_service_account", "")
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		w.send(ctx, d)
	}
	return nil
}
func (w *Worker) CaptureCommitted(ctx context.Context) error {
	cfg, err := w.Repo.ExternalConfig(ctx)
	if err != nil {
		return err
	}
	recipients, err := w.Repo.NotificationRecipients(ctx, w.AppID)
	if err != nil {
		return err
	}
	tx, err := w.Repo.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, "SELECT id,created,before_state,after_state,snapshot,occurred_at FROM "+w.Repo.schema+".message_order_changes WHERE processed_at IS NULL ORDER BY id LIMIT 25 FOR UPDATE SKIP LOCKED")
	if err != nil {
		return err
	}
	type change struct {
		id                      int64
		created                 bool
		before, after, snapshot []byte
		at                      time.Time
	}
	var changes []change
	for rows.Next() {
		var c change
		if err = rows.Scan(&c.id, &c.created, &c.before, &c.after, &c.snapshot, &c.at); err != nil {
			rows.Close()
			return err
		}
		changes = append(changes, c)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	for _, c := range changes {
		var before, after app.OrderState
		var n app.OrderNotice
		if err = json.Unmarshal(c.before, &before); err != nil {
			return err
		}
		if err = json.Unmarshal(c.after, &after); err != nil {
			return err
		}
		if err = json.Unmarshal(c.snapshot, &n); err != nil {
			return err
		}
		n.ChangeID, n.State = c.id, after
		milestones := app.OrderMilestones(before, after, c.created)
		officialNotice := n
		officialNotice.Milestones = filterEvents(milestones, cfg.ServiceEvents())
		if len(officialNotice.Milestones) > 0 && cfg.OfficialEnabled && !c.at.Before(cfg.OfficialSince) {
			var targets []Recipient
			for _, recipient := range recipients {
				if c.at.Before(recipient.ConsentAt) || len(filterEvents(officialNotice.Milestones, recipient.Events)) == 0 || w.Authorize == nil {
					continue
				}
				ok, e := w.Authorize(ctx, recipient, officialNotice)
				if e != nil {
					return e
				}
				if ok {
					targets = append(targets, recipient)
				}
			}
			if len(targets) > 0 {
				if _, err = w.Repo.enqueueNotice(ctx, tx, officialNotice, targets, 0); err != nil {
					return err
				}
			}
		}
		groupNotice := n
		groupNotice.Milestones = filterEvents(milestones, cfg.Events)
		if len(groupNotice.Milestones) > 0 && cfg.DesktopEnabled && !c.at.Before(cfg.DesktopSince) {
			if _, err = w.Repo.enqueueNotice(ctx, tx, groupNotice, nil, cfg.DesktopID); err != nil {
				return err
			}
		}
		if _, err = tx.Exec(ctx, "UPDATE "+w.Repo.schema+".message_order_changes SET processed_at=now() WHERE id=$1", c.id); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r Repository) enqueueNotice(ctx context.Context, tx pgx.Tx, n app.OrderNotice, recipients []Recipient, device int64) (int64, error) {
	b, err := json.Marshal(n)
	if err != nil {
		return 0, err
	}
	var payload map[string]any
	if err = json.Unmarshal(b, &payload); err != nil {
		return 0, err
	}
	key := fmt.Sprintf("order.change.%d", n.ChangeID)
	if !n.Test {
		if device > 0 {
			key += ".group"
		} else {
			key += ".official"
		}
	}
	if n.BusinessEventKey != "" && !n.Test {
		key = "business." + n.BusinessEventKey
	}
	if n.Test {
		key = "order.test." + randomToken()
	}
	var deliveries []app.DeliveryCommand
	for _, v := range recipients {
		deliveries = append(deliveries, app.DeliveryCommand{Channel: "wechat_service_account", TargetType: "channel_identity", TargetKey: v.AppID + ":" + v.OpenID, AdapterKey: "wechat_service_account"})
	}
	if device > 0 {
		deliveries = append(deliveries, app.DeliveryCommand{Channel: "wechat_desktop", TargetType: "channel_identity", TargetKey: strconv.FormatInt(device, 10), AdapterKey: "wechat_desktop"})
	}
	id, err := r.WithTransaction(tx).Publish(ctx, app.PublishCommand{EventKey: key, Topic: "orders", EventType: "order.lifecycle_changed", SourceType: "order", SourceID: n.OrderID, Title: n.StatusText(), Body: n.GroupText(), Payload: payload, Deliveries: deliveries})
	if err != nil {
		return 0, err
	}
	for _, v := range recipients {
		raw, _ := json.Marshal(v)
		if _, err = tx.Exec(ctx, "UPDATE "+r.schema+".message_deliveries SET lifecycle=true,recipient_json=$1 WHERE event_id=$2 AND channel='wechat_service_account' AND target_key=$3", raw, id, v.AppID+":"+v.OpenID); err != nil {
			return 0, err
		}
	}
	_, err = tx.Exec(ctx, "UPDATE "+r.schema+".message_deliveries SET lifecycle=true WHERE event_id=$1 AND channel='wechat_desktop'", id)
	return id, err
}

func (r Repository) ClaimExternal(ctx context.Context, channel, target string) (Delivery, error) {
	var d Delivery
	var payload, recipient []byte
	lease := randomToken()
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`WITH candidate AS (
 SELECT id FROM %[1]s.message_deliveries WHERE lifecycle AND channel=$1 AND ($2='' OR target_key=$2) AND status IN ('ready','retry') AND (deliver_after IS NULL OR deliver_after<=now()) ORDER BY id LIMIT 1 FOR UPDATE SKIP LOCKED
 ),claimed AS (UPDATE %[1]s.message_deliveries d SET status='sending',attempts=attempts+1,lease_token=$3,lease_until=now()+interval '2 minutes',updated_at=now() FROM candidate c WHERE d.id=c.id RETURNING d.*)
 SELECT d.id,d.event_id,d.channel,d.target_key,d.attempts,d.created_at,d.recipient_json,e.payload_json FROM claimed d JOIN %[1]s.message_events e ON e.id=d.event_id`, r.schema), channel, target, lease).Scan(&d.ID, &d.EventID, &d.Channel, &d.Target, &d.Attempts, &d.CreatedAt, &recipient, &payload)
	if err != nil {
		return d, err
	}
	d.Lease = lease
	d.Status = "sending"
	if err = json.Unmarshal(payload, &d.Notice); err != nil {
		return d, err
	}
	err = json.Unmarshal(recipient, &d.Recipient)
	return d, err
}
func (r Repository) FinishExternal(ctx context.Context, d Delivery, status, provider, reason string) error {
	switch status {
	case "accepted", "delivered", "retry", "failed", "unknown", "skipped":
	default:
		return errors.New("invalid delivery state")
	}
	if status == "retry" && d.Attempts >= 5 {
		status = "failed"
	}
	delay := time.Duration(1<<min(d.Attempts, 6)) * time.Minute
	tag, err := r.pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.message_deliveries SET status=$1,provider_message_id=$2,error=$3,deliver_after=$4,lease_until=NULL,updated_at=now(),delivered_at=CASE WHEN $1='delivered' THEN now() ELSE NULL END WHERE id=$5 AND lifecycle AND status='sending' AND lease_token=$6`, r.schema), status, provider, reason, time.Now().Add(delay), d.ID, d.Lease)
	if err == nil && tag.RowsAffected() != 1 {
		return errors.New("投递租约已失效，请核对发送结果")
	}
	if err == nil && status == "accepted" && provider != "" {
		_, err = r.pool.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.message_deliveries d SET status=CASE WHEN p.status='success' THEN 'delivered' ELSE 'failed' END,error=p.status,delivered_at=CASE WHEN p.status='success' THEN p.received_at ELSE NULL END FROM %[1]s.message_provider_receipts p WHERE d.id=$1 AND d.provider_message_id=p.msg_id AND d.recipient_json->>'app_id'=p.app_id AND d.status='accepted'`, r.schema), d.ID)
	}
	return err
}
func (w *Worker) send(ctx context.Context, d Delivery) {
	finish := func(state, id, reason string) {
		if err := w.Repo.FinishExternal(ctx, d, state, id, reason); err != nil {
			log.Printf("notification %d result could not be recorded: %v", d.ID, err)
		}
	}
	cfg, err := w.Repo.ExternalConfig(ctx)
	if err != nil {
		finish("retry", "", "无法读取渠道配置，尚未发送")
		return
	}
	if !d.Notice.Test && (!cfg.OfficialEnabled || d.CreatedAt.Before(cfg.OfficialSince)) {
		finish("skipped", "", "渠道停用或启用前的旧通知")
		return
	}
	current, err := w.Repo.NotificationRecipients(ctx, w.AppID)
	if err != nil {
		finish("retry", "", "无法核对接收身份，尚未发送")
		return
	}
	var recipient *Recipient
	for _, v := range current {
		if v.OpenID == d.Recipient.OpenID && v.MiniUserID == d.Recipient.MiniUserID && v.EmployeeID == d.Recipient.EmployeeID && v.CustomerID == d.Recipient.CustomerID && v.BoundAt.Equal(d.Recipient.BoundAt) {
			v := v
			recipient = &v
			break
		}
	}
	if recipient == nil {
		finish("skipped", "", "已取消关注、通知或绑定")
		return
	}
	if !d.Notice.Test && d.CreatedAt.Before(recipient.ConsentAt) {
		finish("skipped", "", "订阅偏好变更前的旧通知")
		return
	}
	d.Notice.Milestones = filterEvents(d.Notice.Milestones, recipient.Events)
	if !d.Notice.Test {
		d.Notice.Milestones = filterEvents(d.Notice.Milestones, cfg.ServiceEvents())
	}
	if len(d.Notice.Milestones) == 0 {
		finish("skipped", "", "节点通知已关闭")
		return
	}
	if !d.Notice.Test || d.Notice.TestDetail {
		if w.Authorize == nil {
			finish("failed", "", "缺少权限校验")
			return
		}
		ok, e := w.Authorize(ctx, *recipient, d.Notice)
		if e != nil {
			finish("retry", "", "暂时无法核对订单权限，尚未发送")
			return
		}
		if !ok {
			finish("skipped", "", "订单权限或账号已失效")
			return
		}
	}
	if !d.Notice.Test && len(d.Notice.Milestones) == 1 && app.BusinessMilestoneLabels[d.Notice.Milestones[0]] != "" {
		event := d.Notice.Milestones[0]
		since := cfg.OfficialEventSince[event]
		if since.IsZero() || d.Notice.BusinessOccurredAt.IsZero() || d.Notice.BusinessOccurredAt.Before(since) {
			finish("skipped", "", "节点停用或启用前的旧通知")
			return
		}
		var verified bool
		err = w.Repo.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM "+w.Repo.schema+".message_business_test_verifications WHERE event=$1 AND status_value=$2 AND template_fingerprint=$3 AND detail_verified=true)", event, d.Notice.StatusText(), businessTemplateFingerprint(cfg.Templates[event])).Scan(&verified)
		if err != nil {
			finish("retry", "", "无法核对测试记录，尚未发送")
			return
		}
		if !verified {
			finish("failed", "", "该状态尚未通过真实测试")
			return
		}
	}
	tpl := cfg.Templates[d.Notice.Milestones[len(d.Notice.Milestones)-1]]
	if err = tpl.Validate(); err != nil {
		finish("failed", "", err.Error())
		return
	}
	if w.Sender == nil {
		finish("failed", "", "公众号发送服务不可用")
		return
	}
	if d.Notice.Test && len(d.Notice.Milestones) == 1 && app.BusinessMilestoneLabels[d.Notice.Milestones[0]] != "" {
		if _, err = w.Repo.pool.Exec(ctx, "UPDATE "+w.Repo.schema+".message_deliveries SET template_fingerprint=$1 WHERE id=$2 AND status='sending' AND lease_token=$3", businessTemplateFingerprint(tpl), d.ID, d.Lease); err != nil {
			finish("retry", "", "无法记录测试模板，尚未发送")
			return
		}
	}
	id, err := w.Sender.SendOrderTemplate(ctx, recipient.OpenID, tpl, d.Notice, recipient.EmployeeID > 0)
	reason := ""
	if err != nil {
		reason = err.Error()
	}
	finish(wechat.TemplateFailureState(err), id, reason)
}
