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

// CaptureBusinessCommitted fans out only verified and explicitly enabled order events.
func (w *Worker) CaptureBusinessCommitted(ctx context.Context) error {
	recipients, err := w.Repo.NotificationRecipients(ctx, w.AppID)
	if err != nil {
		return err
	}
	tx, err := w.Repo.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Serialize capture with configuration changes. A stale disabled config must
	// not discard a newly enabled event and a stale enabled config must not send.
	var configRaw []byte
	if err = tx.QueryRow(ctx, "SELECT config FROM "+w.Repo.schema+".message_external_settings WHERE id=1 FOR SHARE").Scan(&configRaw); err != nil {
		return err
	}
	cfg := DefaultExternalConfig()
	if err = json.Unmarshal(configRaw, &cfg); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, "SELECT id,event_type,source_key,payload,occurred_at,customer_id,snapshot_pending FROM "+w.Repo.schema+".message_business_events WHERE processed_at IS NULL ORDER BY id LIMIT 25 FOR UPDATE SKIP LOCKED")
	if err != nil {
		return err
	}
	type entry struct {
		id              int64
		event, key      string
		raw             []byte
		at              time.Time
		customerID      int64
		snapshotPending bool
	}
	var entries []entry
	for rows.Next() {
		var e entry
		if err = rows.Scan(&e.id, &e.event, &e.key, &e.raw, &e.at, &e.customerID, &e.snapshotPending); err != nil {
			rows.Close()
			return err
		}
		entries = append(entries, e)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	for _, e := range entries {
		if e.snapshotPending {
			since := cfg.OfficialEventSince[e.event]
			if !cfg.OfficialEnabled || len(filterEvents([]string{e.event}, cfg.ServiceEvents())) == 0 || since.IsZero() || e.at.Before(since) {
				if _, err = tx.Exec(ctx, "UPDATE "+w.Repo.schema+".message_business_events SET processed_at=now(),payload='{}'::jsonb,snapshot_pending=false WHERE id=$1", e.id); err != nil {
					return err
				}
				continue
			}
			var snapshot []byte
			snapshot, err = financeSnapshotTx(ctx, tx, w.Repo.schema, e.event, e.customerID, e.raw)
			if errors.Is(err, pgx.ErrNoRows) {
				if _, err = tx.Exec(ctx, "UPDATE "+w.Repo.schema+".message_business_events SET processed_at=now(),payload='{}'::jsonb,snapshot_pending=false,failure_reason='财务单据缺少有效日期或权限归属，事件未发送' WHERE id=$1", e.id); err != nil {
					return err
				}
				continue
			}
			if err != nil {
				return err
			}
			e.raw = snapshot
		}
		var notice app.OrderNotice
		if err = json.Unmarshal(e.raw, &notice); err != nil {
			return err
		}
		var data struct {
			ShippingStatus string `json:"shipping_status"`
		}
		if err = json.Unmarshal(e.raw, &data); err != nil {
			return err
		}
		notice.State.Shipping = data.ShippingStatus
		notice.ChangeID = e.id
		notice.BusinessEventKey = e.key
		notice.BusinessOccurredAt = e.at
		notice.Milestones = []string{e.event}
		since := cfg.OfficialEventSince[e.event]
		if cfg.OfficialEnabled && app.BusinessMilestoneLabels[e.event] != "" && len(filterEvents([]string{e.event}, cfg.ServiceEvents())) > 0 && !since.IsZero() && !e.at.Before(since) {
			var verified bool
			err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM "+w.Repo.schema+".message_business_test_verifications WHERE event=$1 AND status_value=$2 AND template_fingerprint=$3 AND detail_verified=true)", e.event, notice.StatusText(), businessTemplateFingerprint(cfg.Templates[e.event])).Scan(&verified)
			if err != nil {
				return err
			}
			if verified && w.Authorize != nil {
				var targets []Recipient
				for _, recipient := range recipients {
					if e.at.Before(recipient.ConsentAt) || len(filterEvents(notice.Milestones, recipient.Events)) == 0 {
						continue
					}
					ok, authErr := w.Authorize(ctx, recipient, notice)
					if authErr != nil {
						return authErr
					}
					if ok {
						targets = append(targets, recipient)
					}
				}
				if len(targets) > 0 {
					if _, err = w.Repo.enqueueNotice(ctx, tx, notice, targets, 0); err != nil {
						return err
					}
				}
			}
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf("UPDATE %s.message_business_events SET processed_at=now(),payload='{}'::jsonb,snapshot_pending=false WHERE id=$1", w.Repo.schema), e.id); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
