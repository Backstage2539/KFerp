package messagecenter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	app "orderapp/internal/application/messagecenter"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EnsureBusinessEventSchema is safe to invoke during business schema setup as
// well as message-center setup; business transactions must never skip the outbox.
func EnsureBusinessEventSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	schema = pgx.Identifier{schema}.Sanitize()
	_, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.message_business_events (
 id BIGSERIAL PRIMARY KEY, event_type TEXT NOT NULL, source_key TEXT NOT NULL UNIQUE,
 order_id BIGINT NOT NULL DEFAULT 0, customer_id BIGINT NOT NULL,
 payload JSONB NOT NULL, occurred_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(), processed_at TIMESTAMPTZ);
 ALTER TABLE %[1]s.message_business_events ADD COLUMN IF NOT EXISTS failure_reason TEXT NOT NULL DEFAULT '';
 ALTER TABLE %[1]s.message_business_events ADD COLUMN IF NOT EXISTS status_value TEXT NOT NULL DEFAULT '';
 ALTER TABLE %[1]s.message_business_events ADD COLUMN IF NOT EXISTS snapshot_pending BOOLEAN NOT NULL DEFAULT false;
 CREATE INDEX IF NOT EXISTS message_business_events_pending ON %[1]s.message_business_events(id) WHERE processed_at IS NULL;
 CREATE TABLE IF NOT EXISTS %[1]s.message_external_settings(id INT PRIMARY KEY CHECK(id=1),config JSONB NOT NULL DEFAULT '{}',updated_at TIMESTAMPTZ NOT NULL DEFAULT now());
 INSERT INTO %[1]s.message_external_settings(id) VALUES(1) ON CONFLICT DO NOTHING;
 CREATE TABLE IF NOT EXISTS %[1]s.message_business_test_verifications(event TEXT NOT NULL,status_value TEXT NOT NULL,template_fingerprint TEXT NOT NULL,delivery_id BIGINT NOT NULL UNIQUE,verified_by TEXT NOT NULL,verified_at TIMESTAMPTZ NOT NULL DEFAULT now(),PRIMARY KEY(event,status_value,template_fingerprint));
 ALTER TABLE %[1]s.message_business_test_verifications ADD COLUMN IF NOT EXISTS detail_verified BOOLEAN NOT NULL DEFAULT false`, schema))
	return err
}

// RecordBusinessEvent stores an eligible order event in the business transaction.
// Inactive events become non-replayable markers without sensitive payload.
func RecordBusinessEvent(ctx context.Context, tx pgx.Tx, schema, eventType, sourceKey string, orderID, customerID int64, payload any) error {
	if strings.TrimSpace(eventType) == "" || strings.TrimSpace(sourceKey) == "" || customerID <= 0 {
		return errors.New("invalid business notification event")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	schema = pgx.Identifier{schema}.Sanitize()
	// Finance events are never consumed until their separate authorization exists.
	eligible := false
	snapshotPending := false
	statusValue := ""
	if eventType == "order_accepted" {
		statusValue = "已接单"
	} else if eventType == "shipment_created" {
		var values struct {
			Status string `json:"shipping_status"`
		}
		if json.Unmarshal(raw, &values) == nil {
			switch values.Status {
			case "已发货", "部分发货", "已出库":
				statusValue = values.Status
			default:
				statusValue = "其他"
			}
		}
	}
	failureReason := ""
	if eventType == "order_accepted" || eventType == "shipment_created" || eventType == "receipt_received" || eventType == "bill_issued" {
		var configRaw []byte
		if _, err = tx.Exec(ctx, "SAVEPOINT business_notice_config"); err != nil {
			return err
		}
		if err = tx.QueryRow(ctx, "SELECT config FROM "+schema+".message_external_settings WHERE id=1 FOR SHARE").Scan(&configRaw); err != nil {
			if _, rollbackErr := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT business_notice_config"); rollbackErr != nil {
				return rollbackErr
			}
			failureReason = "无法读取通知配置，事件未发送"
			configRaw = []byte(`{}`)
		}
		cfg := DefaultExternalConfig()
		if err = json.Unmarshal(configRaw, &cfg); err != nil {
			failureReason = "通知配置无效，事件未发送"
			cfg = DefaultExternalConfig()
		}
		if cfg.OfficialEnabled && len(filterEvents([]string{eventType}, cfg.ServiceEvents())) > 0 && !cfg.OfficialEventSince[eventType].IsZero() {
			status := app.BusinessMilestoneLabels[eventType]
			if eventType == "shipment_created" {
				var values struct {
					Status   string `json:"shipping_status"`
					Tracking string `json:"tracking_no"`
				}
				if err = json.Unmarshal(raw, &values); err != nil {
					return err
				}
				status = values.Status
				if values.Tracking == "" {
					status = ""
				}
			}
			if status == "" {
				failureReason = "缺少有效发货状态或快递单号，事件未发送"
			}
			if status != "" {
				if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM "+schema+".message_business_test_verifications WHERE event=$1 AND status_value=$2 AND template_fingerprint=$3 AND detail_verified=true)", eventType, status, businessTemplateFingerprint(cfg.Templates[eventType])).Scan(&eligible); err != nil {
					if _, rollbackErr := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT business_notice_config"); rollbackErr != nil {
						return rollbackErr
					}
					// Configuration was enabled, but verification is temporarily unavailable.
					// Retain a candidate; the consumer will recheck proof before sending.
					eligible = true
					if eventType == "receipt_received" || eventType == "bill_issued" {
						var source struct {
							ReceiptID int64 `json:"receipt_id"`
							BillID    int64 `json:"bill_id"`
						}
						if e := json.Unmarshal(raw, &source); e != nil {
							return e
						}
						raw, _ = json.Marshal(source)
						snapshotPending = true
					}
				} else if !eligible {
					failureReason = "该业务状态尚未完成真实测试，事件未发送"
				}
			}
		}
		if eligible && !snapshotPending && (eventType == "receipt_received" || eventType == "bill_issued") {
			var snap []byte
			snap, err = financeSnapshotTx(ctx, tx, schema, eventType, customerID, raw)
			if err != nil {
				noRows := errors.Is(err, pgx.ErrNoRows)
				if _, rollbackErr := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT business_notice_config"); rollbackErr != nil {
					return rollbackErr
				}
				if noRows {
					eligible = false
					failureReason = "财务单据缺少有效日期或权限归属，事件未发送"
				} else {
					var source struct {
						ReceiptID int64 `json:"receipt_id"`
						BillID    int64 `json:"bill_id"`
					}
					if e := json.Unmarshal(raw, &source); e != nil {
						return e
					}
					raw, _ = json.Marshal(source)
					snapshotPending = true
				}
			} else {
				raw = snap
			}
		}
		if _, err = tx.Exec(ctx, "RELEASE SAVEPOINT business_notice_config"); err != nil {
			return err
		}
	}
	if failureReason != "" {
		log.Printf("business notification %s %s: %s", eventType, sourceKey, failureReason)
	}
	if eligible {
		_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.message_business_events(event_type,source_key,order_id,customer_id,payload,snapshot_pending) VALUES($1,$2,$3,$4,$5::jsonb,$6) ON CONFLICT(source_key) DO NOTHING`, schema), eventType, sourceKey, orderID, customerID, raw, snapshotPending)
	} else {
		_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.message_business_events(event_type,source_key,order_id,customer_id,payload,processed_at,failure_reason,status_value) VALUES($1,$2,$3,$4,'{}'::jsonb,clock_timestamp(),$5,$6) ON CONFLICT(source_key) DO NOTHING`, schema), eventType, sourceKey, orderID, customerID, failureReason, statusValue)
	}
	return err
}

// BusinessEventFailures exposes blocked events to notification administrators;
// it contains no customer, amount or event payload.
func (r Repository) BusinessEventFailures(ctx context.Context) ([]map[string]any, error) {
	rows, err := r.pool.Query(ctx, "SELECT id,event_type,failure_reason,occurred_at,status_value,CASE WHEN event_type IN ('receipt_received','bill_issued') THEN 0 ELSE order_id END FROM "+r.schema+".message_business_events WHERE failure_reason<>'' ORDER BY id DESC LIMIT 100")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []map[string]any{}
	for rows.Next() {
		var id, orderID int64
		var event, reason, status string
		var at time.Time
		if err = rows.Scan(&id, &event, &reason, &at, &status, &orderID); err != nil {
			return nil, err
		}
		result = append(result, map[string]any{"id": id, "event": event, "reason": reason, "occurred_at": at, "status_value": status, "order_id": orderID})
	}
	return result, rows.Err()
}
