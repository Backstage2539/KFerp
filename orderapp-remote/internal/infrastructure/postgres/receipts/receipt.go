package receipts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Receipt is an immutable cash receipt snapshot. Amounts use integer cents;
// allocations preserve the original source's per-order breakdown.
type Receipt struct {
	CustomerID      int64           `json:"customer_id"`
	SourceType      string          `json:"source_type"`
	SourceID        int64           `json:"source_id"`
	ActualCents     int64           `json:"actual_cents"`
	ReceivableCents int64           `json:"receivable_cents"`
	ReceiptDate     string          `json:"receipt_date"`
	Method          string          `json:"method"`
	VoucherAssetID  int64           `json:"voucher_asset_id"`
	Actor           string          `json:"actor"`
	Allocations     json.RawMessage `json:"allocations"`
}

func (r Receipt) Validate() error {
	if r.CustomerID <= 0 || r.SourceID <= 0 || r.ActualCents <= 0 || r.ReceivableCents <= 0 || strings.TrimSpace(r.SourceType) == "" || strings.TrimSpace(r.Method) == "" || strings.TrimSpace(r.Actor) == "" || !json.Valid(r.Allocations) {
		return errors.New("收款事实无效")
	}
	return nil
}

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	s := pgx.Identifier{schema}.Sanitize()
	_, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.cash_receipts (
 id BIGSERIAL PRIMARY KEY,
 receipt_no TEXT GENERATED ALWAYS AS ('RC' || lpad(id::text, 12, '0')) STORED UNIQUE,
 customer_id BIGINT NOT NULL CHECK(customer_id > 0),
 source_type TEXT NOT NULL, source_id BIGINT NOT NULL CHECK(source_id > 0),
 actual_cents BIGINT NOT NULL CHECK(actual_cents > 0),
 receivable_cents BIGINT NOT NULL CHECK(receivable_cents > 0),
 receipt_date DATE,
 payment_method TEXT NOT NULL, voucher_asset_id BIGINT NOT NULL DEFAULT 0,
 actor TEXT NOT NULL, allocations JSONB NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(source_type,source_id)
 );
 ALTER TABLE %s.cash_receipts ADD COLUMN IF NOT EXISTS reversed_at TIMESTAMPTZ;
 ALTER TABLE %s.cash_receipts ADD COLUMN IF NOT EXISTS reversed_by TEXT NOT NULL DEFAULT '';`, s, s, s))
	return err
}

// RecordTx must be called in the same transaction as the business write.
func RecordTx(ctx context.Context, tx pgx.Tx, schema string, r Receipt) (int64, string, string, error) {
	if err := r.Validate(); err != nil {
		return 0, "", "", err
	}
	var id int64
	var number, date string
	err := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.cash_receipts(customer_id,source_type,source_id,actual_cents,receivable_cents,payment_method,voucher_asset_id,actor,allocations,receipt_date) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,NULLIF($10,'')::date) RETURNING id,receipt_no,COALESCE(receipt_date::text,'')`, pgx.Identifier{schema}.Sanitize()), r.CustomerID, r.SourceType, r.SourceID, r.ActualCents, r.ReceivableCents, strings.TrimSpace(r.Method), r.VoucherAssetID, strings.TrimSpace(r.Actor), r.Allocations, strings.TrimSpace(r.ReceiptDate)).Scan(&id, &number, &date)
	return id, number, date, err
}

// EnsureDateOpenTx is called after acquiring the shared finance-period lock.
// Legacy callers without an actual receipt date keep it unknown.
func EnsureDateOpenTx(ctx context.Context, tx pgx.Tx, schema, date string) error {
	if date == "" {
		return nil
	}
	if len(date) != 10 {
		return errors.New("实际到账日期无效")
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL AND to_regclass($2) IS NOT NULL`, schema+".finance_settings", schema+".finance_monthly_reports").Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return nil
	}
	var closed bool
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE((SELECT closing_mode FROM %[1]s.finance_settings WHERE id=1),'strong_lock')='strong_lock' AND EXISTS(SELECT 1 FROM %[1]s.finance_monthly_reports WHERE month=$1 AND status IN ('closed','adjusted'))`, pgx.Identifier{schema}.Sanitize()), date[:7]).Scan(&closed)
	if err != nil {
		return err
	}
	if closed {
		return errors.New("到账日期所属账期已结账，请办理财务更正")
	}
	return nil
}
