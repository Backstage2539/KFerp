package messagecenter

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/messagecenter"
	"strconv"
	"strings"
	"time"
)

type DesktopDevice struct {
	Fingerprint string     `json:"fingerprint"`
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	GroupName   string     `json:"group_name"`
	Enabled     bool       `json:"enabled"`
	Verified    bool       `json:"verified"`
	LastSeen    *time.Time `json:"last_seen"`
	Health      string     `json:"health"`
}

func (r Repository) Devices(ctx context.Context) ([]DesktopDevice, error) {
	rows, e := r.pool.Query(ctx, "SELECT id,name,group_name,enabled,verified,last_seen,health,fingerprint FROM "+r.schema+".message_desktop_devices ORDER BY id")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []DesktopDevice{}
	for rows.Next() {
		var v DesktopDevice
		if e = rows.Scan(&v.ID, &v.Name, &v.GroupName, &v.Enabled, &v.Verified, &v.LastSeen, &v.Health, &v.Fingerprint); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (r Repository) AddDevice(ctx context.Context, name, group, actor string) (int64, string, error) {
	name, group = strings.TrimSpace(name), strings.TrimSpace(group)
	if name == "" || group == "" || len([]rune(name)) > 80 || len([]rune(group)) > 100 {
		return 0, "", errors.New("请填写设备及唯一目标群名称")
	}
	token := randomToken()
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return 0, "", e
	}
	defer tx.Rollback(ctx)
	var id int64
	e = tx.QueryRow(ctx, "INSERT INTO "+r.schema+".message_desktop_devices(name,group_name,token_hash) VALUES($1,$2,$3) RETURNING id", name, group, tokenHash(token)).Scan(&id)
	if e != nil {
		return 0, "", e
	}
	if e = r.externalAudit(ctx, tx, actor, id, "notification_device_pair", "", name+" / "+group); e != nil {
		return 0, "", e
	}
	return id, token, tx.Commit(ctx)
}
func (r Repository) RevokeDevice(ctx context.Context, id int64, actor string) error {
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	_, e = tx.Exec(ctx, "UPDATE "+r.schema+".message_desktop_devices SET enabled=false,verified=false WHERE id=$1", id)
	if e != nil {
		return e
	}
	if e = r.externalAudit(ctx, tx, actor, id, "notification_device_revoke", "enabled", "disabled"); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (r Repository) AuthenticateDevice(ctx context.Context, token string) (DesktopDevice, error) {
	var d DesktopDevice
	if len(token) != 64 {
		return d, errors.New("设备凭据无效")
	}
	e := r.pool.QueryRow(ctx, "SELECT id,name,group_name,enabled,verified,last_seen,health,fingerprint FROM "+r.schema+".message_desktop_devices WHERE token_hash=$1 AND enabled", tokenHash(token)).Scan(&d.ID, &d.Name, &d.GroupName, &d.Enabled, &d.Verified, &d.LastSeen, &d.Health, &d.Fingerprint)
	return d, e
}
func (r Repository) Heartbeat(ctx context.Context, d DesktopDevice, health, fingerprint string) error {
	if _, e := hex.DecodeString(fingerprint); e != nil || len(fingerprint) != 64 {
		return errors.New("发送程序版本标识无效")
	}
	switch health {
	case "ready", "logged_out", "locked", "permission_missing", "target_mismatch", "unsupported", "paused":
	default:
		return errors.New("设备状态无效")
	}
	_, e := r.pool.Exec(ctx, "UPDATE "+r.schema+".message_desktop_devices SET last_seen=now(),health=$1,verified=CASE WHEN fingerprint=$3 THEN verified ELSE false END,fingerprint=$3 WHERE id=$2 AND enabled", health, d.ID, fingerprint)
	return e
}
func (r Repository) ClaimDesktop(ctx context.Context, d DesktopDevice) (Delivery, error) {
	if d.Health != "ready" || d.LastSeen == nil || time.Since(*d.LastSeen) > time.Minute {
		return Delivery{}, errors.New("设备尚未准备就绪")
	}
	// One group sender at a time, including restarts and multiple polling processes.
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return Delivery{}, e
	}
	defer tx.Rollback(ctx)
	if e = tx.QueryRow(ctx, "SELECT enabled,verified,health,last_seen,fingerprint FROM "+r.schema+".message_desktop_devices WHERE id=$1 FOR UPDATE", d.ID).Scan(&d.Enabled, &d.Verified, &d.Health, &d.LastSeen, &d.Fingerprint); e != nil {
		return Delivery{}, e
	}
	if !d.Enabled || d.Health != "ready" || d.LastSeen == nil || time.Since(*d.LastSeen) > time.Minute {
		return Delivery{}, errors.New("设备已停用或离线")
	}
	var busy bool
	e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM "+r.schema+".message_deliveries WHERE lifecycle AND channel='wechat_desktop' AND target_key=$1 AND (status IN ('sending','unknown') OR updated_at>now()-interval '5 seconds' AND status IN ('delivered','confirmed')))", strconv.FormatInt(d.ID, 10)).Scan(&busy)
	if e != nil {
		return Delivery{}, e
	}
	if busy {
		return Delivery{}, pgx.ErrNoRows
	}
	// ClaimExternal uses its own connection while the device row serializes pollers.
	job, e := r.ClaimExternal(ctx, "wechat_desktop", strconv.FormatInt(d.ID, 10))
	if e != nil {
		return job, e
	}
	if !job.Notice.Test {
		cfg, e := r.ExternalConfig(ctx)
		if e != nil {
			return job, e
		}
		if !cfg.DesktopEnabled || cfg.DesktopID != d.ID || !d.Verified || job.CreatedAt.Before(cfg.DesktopSince) {
			r.FinishExternal(ctx, job, "skipped", "", "群渠道未启用或任务已过期")
			return Delivery{}, pgx.ErrNoRows
		}
	}
	if _, e = r.pool.Exec(ctx, "UPDATE "+r.schema+".message_deliveries SET recipient_json=jsonb_build_object('fingerprint',$1::text) WHERE id=$2 AND lease_token=$3", d.Fingerprint, job.ID, job.Lease); e != nil {
		return Delivery{}, e
	}
	return job, tx.Commit(ctx)
}
func (r Repository) DesktopResult(ctx context.Context, device DesktopDevice, id int64, lease, status, evidence string) error {
	if status != "delivered" && status != "unknown" && status != "failed" {
		return errors.New("发送结果无效")
	}
	var job Delivery
	var raw []byte
	job.ID = id
	job.Lease = lease
	e := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT d.attempts,e.payload_json FROM %[1]s.message_deliveries d JOIN %[1]s.message_events e ON e.id=d.event_id WHERE d.id=$1 AND d.lifecycle AND d.channel='wechat_desktop' AND d.target_key=$2 AND d.status='sending' AND d.lease_token=$3 AND d.recipient_json->>'fingerprint'=$4`, r.schema), id, strconv.FormatInt(device.ID, 10), lease, device.Fingerprint).Scan(&job.Attempts, &raw)
	if e != nil {
		return e
	}
	if e = json.Unmarshal(raw, &job.Notice); e != nil {
		return e
	}
	if status == "delivered" && evidence != job.Notice.GroupText() {
		return errors.New("发送内容读回不一致，不能确认成功")
	}
	if status == "delivered" {
		evidence = "已在绑定群读回完整消息"
	} else if len(evidence) > 300 {
		evidence = evidence[:300]
	}
	if e = r.FinishExternal(ctx, job, status, "", evidence); e != nil {
		return e
	}
	if status == "delivered" && job.Notice.Test {
		_, e = r.pool.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.message_desktop_devices SET verified=true WHERE id=$1 AND (SELECT count(DISTINCT milestone) FROM %[1]s.message_deliveries d JOIN %[1]s.message_events e ON e.id=d.event_id CROSS JOIN LATERAL jsonb_array_elements_text(e.payload_json->'milestones') milestone WHERE d.channel='wechat_desktop' AND d.target_key=$2 AND d.lifecycle AND d.status='delivered' AND e.payload_json->>'test'='true' AND d.recipient_json->>'fingerprint'=$3)=6`, r.schema), device.ID, strconv.FormatInt(device.ID, 10), device.Fingerprint)
	}
	return e
}
func (r Repository) QueueTest(ctx context.Context, event, channel string, miniID, deviceID int64, appID, actor string) (int64, error) {
	return r.QueueTestStatus(ctx, event, channel, miniID, deviceID, appID, actor, "")
}
func (r Repository) QueueTestStatus(ctx context.Context, event, channel string, miniID, deviceID int64, appID, actor, statusValue string) (int64, error) {
	return r.QueueTestDetail(ctx, event, channel, miniID, deviceID, appID, actor, statusValue, 0)
}
func (r Repository) QueueTestDetail(ctx context.Context, event, channel string, miniID, deviceID int64, appID, actor, statusValue string, detailID int64) (int64, error) {
	if event == "shipment_created" {
		if statusValue == "" {
			statusValue = "已发货"
		}
		if statusValue != "已发货" && statusValue != "部分发货" {
			return 0, errors.New("发货测试状态无效")
		}
	} else if statusValue != "" && !(event == "order_accepted" && statusValue == "已接单") {
		return 0, errors.New("该测试节点不支持指定状态")
	}
	if app.NotificationLabel(event) == "" || (channel == "wechat_desktop" && app.MilestoneLabels[event] == "") {
		return 0, errors.New("未知测试节点")
	}
	n := app.OrderNotice{Test: true, ChangeID: time.Now().UnixMilli(), OrderNo: "TEST-ORDER-001", CustomerName: "测试客户（虚拟）", Items: "测试商品 250g × 2袋", ResponsibleName: "测试负责人", OccurredAt: time.Now().In(time.FixedZone("CST", 8*3600)).Format("2006-01-02 15:04:05"), Milestones: []string{event}}
	if event == "shipment_created" {
		n.State.Shipping = statusValue
		n.TrackingNo = "TEST12345"
	}
	if event == "receipt_received" {
		n.ReceiptNo = "TESTRC001"
		n.ReceiptDate = time.Now().In(time.FixedZone("CST", 8*3600)).Format("2006-01-02")
		n.ReceivableAmount = "100.00"
		n.ReceivedAmount = "25.00"
	}
	if event == "bill_issued" {
		n.BillNo = "TESTBILL001"
		n.BillAmount = "100.00"
		n.BillPeriod = "2026-11-01~2026-11-11"
	}

	var recipients []Recipient
	if channel == "wechat_service_account" {
		all, e := r.NotificationRecipients(ctx, appID)
		if e != nil {
			return 0, e
		}
		for _, v := range all {
			if v.MiniUserID == miniID {
				recipients = append(recipients, v)
			}
		}
		if miniID <= 0 || len(recipients) == 0 {
			return 0, errors.New("测试账号尚未绑定、关注并开启通知")
		}
		if len(recipients) != 1 {
			return 0, errors.New("测试账号绑定多个微信，请先解除歧义后再测试")
		}
		deviceID = 0
	} else if channel == "wechat_desktop" {
		var ok bool
		e := r.pool.QueryRow(ctx, "SELECT enabled FROM "+r.schema+".message_desktop_devices WHERE id=$1", deviceID).Scan(&ok)
		if e != nil || !ok {
			return 0, errors.New("测试设备不可用")
		}
	} else {
		return 0, errors.New("未知测试渠道")
	}
	if detailID < 0 {
		return 0, errors.New("测试详情编号无效")
	}
	if detailID > 0 {
		if channel != "wechat_service_account" || app.BusinessMilestoneLabels[event] == "" {
			return 0, errors.New("该测试不支持真实详情链接")
		}
		var customerID int64
		switch event {
		case "order_accepted", "shipment_created":
			var responsibleID int64
			e := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(o.customer_id,0),CASE WHEN to_jsonb(o)->>'responsible_party_type'='employee' THEN COALESCE(NULLIF(to_jsonb(o)->>'responsible_party_id','')::bigint,0) ELSE COALESCE(c.responsible_employee_id,0) END FROM %[1]s.orders o LEFT JOIN %[1]s.customers c ON c.id=o.customer_id WHERE o.id=$1`, r.schema), detailID).Scan(&customerID, &responsibleID)
			if e != nil || customerID <= 0 {
				return 0, errors.New("测试订单不存在或不可用")
			}
			n.OrderID = detailID
			n.ResponsibleID = responsibleID
		case "receipt_received":
			e := r.pool.QueryRow(ctx, "SELECT customer_id FROM "+r.schema+".cash_receipts WHERE id=$1", detailID).Scan(&customerID)
			if e != nil || customerID <= 0 {
				return 0, errors.New("测试收款单不存在或不可用")
			}
			n.ReceiptID = detailID
		case "bill_issued":
			e := r.pool.QueryRow(ctx, "SELECT customer_id FROM "+r.schema+".customer_settlement_batches WHERE id=$1", detailID).Scan(&customerID)
			if e != nil || customerID <= 0 {
				return 0, errors.New("测试账单不存在或不可用")
			}
			n.BillID = detailID
		}
		n.CustomerID = customerID
		n.TestDetail = true
	}
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return 0, e
	}
	defer tx.Rollback(ctx)
	id, e := r.enqueueNotice(ctx, tx, n, recipients, deviceID)
	if e != nil {
		return 0, e
	}
	if e = r.externalAudit(ctx, tx, actor, id, "notification_test", "", channel+":"+event); e != nil {
		return 0, e
	}
	return id, tx.Commit(ctx)
}
