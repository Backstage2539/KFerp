package messagecenter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	app "orderapp/internal/application/messagecenter"
	wechat "orderapp/internal/infrastructure/wechataccount"
)

func businessTemplateFingerprint(t wechat.TemplateMapping) string {
	b, _ := json.Marshal(t)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// VerifyBusinessTest records a human's explicit confirmation after a successful
// provider callback; an accepted request or an unreceived test is insufficient.
func (r Repository) VerifyBusinessTest(ctx context.Context, deliveryID int64, actor string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var raw []byte
	var status, fingerprint string
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT e.payload_json,d.status,d.template_fingerprint FROM %[1]s.message_deliveries d JOIN %[1]s.message_events e ON e.id=d.event_id WHERE d.id=$1 AND d.lifecycle AND d.channel='wechat_service_account' FOR UPDATE OF d`, r.schema), deliveryID).Scan(&raw, &status, &fingerprint)
	if err != nil {
		return err
	}
	var n app.OrderNotice
	if err = json.Unmarshal(raw, &n); err != nil {
		return err
	}
	if status != "delivered" || !n.Test || !n.TestDetail || len(n.Milestones) != 1 || app.BusinessMilestoneLabels[n.Milestones[0]] == "" || fingerprint == "" {
		return errors.New("须先收到带已授权真实详情链接的业务测试并确认跳转")
	}
	var configRaw []byte
	if err = tx.QueryRow(ctx, "SELECT config FROM "+r.schema+".message_external_settings WHERE id=1 FOR UPDATE").Scan(&configRaw); err != nil {
		return err
	}
	cfg := DefaultExternalConfig()
	if err = json.Unmarshal(configRaw, &cfg); err != nil {
		return err
	}
	if businessTemplateFingerprint(cfg.Templates[n.Milestones[0]]) != fingerprint {
		return errors.New("模板已变更，请重新测试")
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.message_business_test_verifications(event,status_value,template_fingerprint,delivery_id,verified_by,detail_verified) VALUES($1,$2,$3,$4,$5,true) ON CONFLICT(event,status_value,template_fingerprint) DO UPDATE SET delivery_id=EXCLUDED.delivery_id,verified_by=EXCLUDED.verified_by,verified_at=now(),detail_verified=true`, r.schema), n.Milestones[0], n.StatusText(), fingerprint, deliveryID, actor)
	if err != nil {
		return err
	}
	if err = r.externalAudit(ctx, tx, actor, deliveryID, "verify_business_test", "", n.Milestones[0]+":"+n.StatusText()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
