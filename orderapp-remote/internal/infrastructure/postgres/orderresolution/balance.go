package orderresolution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	infra "orderapp/internal/infrastructure/postgres"
	"strings"
	"time"
)

type Balance struct {
	ID             int64  `json:"id"`
	OrderID        int64  `json:"order_id"`
	OrderNo        string `json:"order_no"`
	AmountCents    int64  `json:"amount_cents"`
	AvailableCents int64  `json:"available_cents"`
	PostingDate    string `json:"posting_date"`
}
type BalanceUse struct {
	ID            int64  `json:"id"`
	OrderNo       string `json:"order_no"`
	TargetOrderNo string `json:"target_order_no"`
	Choice        string `json:"choice"`
	AmountCents   int64  `json:"amount_cents"`
	PostingDate   string `json:"posting_date"`
	Account       string `json:"account"`
	Reference     string `json:"reference"`
	Note          string `json:"note"`
	Actor         string `json:"actor"`
}

func (r Repository) BalanceHistory(ctx context.Context, orderID int64) ([]BalanceUse, error) {
	out := []BalanceUse{}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT u.id,o.order_no,COALESCE(target.order_no,''),u.kind,u.amount_cents,to_char(u.posting_date,'YYYY-MM-DD'),u.account,u.reference,u.note,u.actor FROM %[1]s.order_resolution_balance_uses u JOIN %[1]s.order_resolution_financial_entries f ON f.id=u.source_entry_id JOIN %[1]s.orders o ON o.id=f.order_id JOIN %[1]s.orders current ON current.customer_id=u.customer_id LEFT JOIN %[1]s.orders target ON target.id=u.target_order_id AND u.kind='offset' WHERE current.id=$1 ORDER BY u.id DESC LIMIT 100`, r.schema), orderID)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var v BalanceUse
		if err = rows.Scan(&v.ID, &v.OrderNo, &v.TargetOrderNo, &v.Choice, &v.AmountCents, &v.PostingDate, &v.Account, &v.Reference, &v.Note, &v.Actor); err != nil {
			return out, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

type BalanceCommand struct {
	StepCommand
	SourceEntryID          int64 `json:"source_entry_id"`
	ExpectedAvailableCents int64 `json:"expected_available_cents"`
	ExpectedTargetDueCents int64 `json:"expected_target_due_cents"`
}

func (r Repository) Balances(ctx context.Context, orderID int64) ([]Balance, error) {
	out := []Balance{}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT f.id,f.order_id,o.order_no,f.amount_cents,f.amount_cents-COALESCE(u.used,0),to_char(f.posting_date,'YYYY-MM-DD') FROM %[1]s.order_resolution_financial_entries f JOIN %[1]s.orders o ON o.id=f.order_id JOIN %[1]s.orders current ON current.customer_id=f.customer_id LEFT JOIN LATERAL(SELECT sum(amount_cents)used FROM %[1]s.order_resolution_balance_uses WHERE source_entry_id=f.id)u ON true WHERE current.id=$1 AND f.kind='customer_credit' AND f.customer_id>0 AND f.amount_cents>COALESCE(u.used,0)ORDER BY f.id`, r.schema), orderID)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var b Balance
		if err = rows.Scan(&b.ID, &b.OrderID, &b.OrderNo, &b.AmountCents, &b.AvailableCents, &b.PostingDate); err != nil {
			return out, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
func (r Repository) UseBalance(ctx context.Context, orderID int64, cmd BalanceCommand, actor string) error {
	if strings.TrimSpace(actor) == "" || strings.TrimSpace(cmd.IdempotencyKey) == "" || len(cmd.IdempotencyKey) > 128 {
		return fmt.Errorf("缺少操作人或提交标识")
	}
	if !one(cmd.Choice, "refund", "offset") || cmd.SourceEntryID <= 0 || cmd.AmountCents <= 0 || cmd.ExpectedAvailableCents <= 0 {
		return fmt.Errorf("请选择余额来源、去向和正数金额")
	}
	if strings.TrimSpace(cmd.Note) == "" {
		return fmt.Errorf("请填写余额处理说明")
	}
	if _, err := time.Parse("2006-01-02", cmd.PostingDate); err != nil {
		return fmt.Errorf("请选择实际处理日期")
	}
	if cmd.Choice == "refund" && (strings.TrimSpace(cmd.Account) == "" || strings.TrimSpace(cmd.Reference) == "") {
		return fmt.Errorf("请登记实际付款账户和退款凭据")
	}
	raw, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = r.lock(ctx, tx, orderID); err != nil {
		return err
	}
	var customer int64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT customer_id FROM %s.orders WHERE id=$1`, r.schema), orderID).Scan(&customer); err != nil {
		return err
	}
	if customer <= 0 {
		return fmt.Errorf("客户身份不完整，不能使用客户余额")
	}
	var previous string
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT request_hash FROM %s.order_resolution_balance_uses WHERE customer_id=$1 AND idempotency_key=$2`, r.schema), customer, cmd.IdempotencyKey).Scan(&previous)
	if err == nil {
		if previous != hash(raw) {
			return fmt.Errorf("提交标识已用于不同内容")
		}
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if err = r.checkPeriod(ctx, tx, cmd.PostingDate); err != nil {
		return err
	}
	policy, err := r.policy(ctx, tx)
	if err != nil {
		return err
	}
	if !policy.Policy.Allows("funds", cmd.Choice) {
		return fmt.Errorf("管理员已停用所选资金去向")
	}
	var sourceOrder, available int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT f.order_id,f.amount_cents-COALESCE((SELECT sum(amount_cents) FROM %[1]s.order_resolution_balance_uses u WHERE u.source_entry_id=f.id),0) FROM %[1]s.order_resolution_financial_entries f WHERE f.id=$1 AND f.customer_id=$2 AND f.kind='customer_credit' FOR UPDATE`, r.schema), cmd.SourceEntryID, customer).Scan(&sourceOrder, &available)
	if err != nil {
		return fmt.Errorf("余额来源不存在或不属于本客户")
	}
	if available != cmd.ExpectedAvailableCents {
		return ErrStale
	}
	if cmd.Choice == "refund" {
		var duplicate bool
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.order_resolution_balance_uses WHERE source_entry_id=$1 AND kind='refund' AND reference=$2)`, r.schema), cmd.SourceEntryID, cmd.Reference).Scan(&duplicate); err != nil {
			return err
		}
		if duplicate {
			return fmt.Errorf("该余额已登记此付款凭据，请核对已有退款记录")
		}
	}
	if cmd.AmountCents > available {
		return fmt.Errorf("处理金额超过当前客户余额")
	}
	if cmd.Choice == "offset" {
		if cmd.TargetID <= 0 || cmd.ExpectedTargetDueCents <= 0 {
			return fmt.Errorf("请选择具体应收并刷新待收金额")
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`SELECT id FROM %s.orders WHERE id=$1 FOR UPDATE`, r.schema), cmd.TargetID); err != nil {
			return err
		}
		var pending bool
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.order_resolution_requests WHERE order_id=$1 AND status='pending')`, r.schema), cmd.TargetID).Scan(&pending); err != nil {
			return err
		}
		target, err := r.load(ctx, tx, cmd.TargetID)
		if err != nil {
			return err
		}
		due := max(int64(0), target.TotalCents-target.CreditCents-max(int64(0), target.PaidCents+target.OffsetCents-target.CreditUsedCents))
		if target.CustomerID != customer || target.Void || pending {
			return fmt.Errorf("目标客户、状态或处理进度不符")
		}
		if due != cmd.ExpectedTargetDueCents {
			return ErrStale
		}
		if cmd.AmountCents > due {
			return fmt.Errorf("抵扣金额超过目标待收金额")
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET resolution_offset_cents=resolution_offset_cents+$2 WHERE id=$1`, r.schema), cmd.TargetID, cmd.AmountCents); err != nil {
			return err
		}
		if err = infra.AuditInsertTx(ctx, tx, r.schema, actor, "order", &cmd.TargetID, "customer_balance_offset", nil, nil, nil, infra.AuditMeta{"source_order_id": sourceOrder, "source_entry_id": cmd.SourceEntryID, "amount_cents": cmd.AmountCents}); err != nil {
			return err
		}
	}
	targetID := int64(0)
	if cmd.Choice == "offset" {
		targetID = cmd.TargetID
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_resolution_balance_uses(customer_id,source_entry_id,kind,amount_cents,target_order_id,posting_date,account,reference,note,idempotency_key,request_hash,actor)VALUES($1,$2,$3,$4,$5,$6::date,$7,$8,$9,$10,$11,$12)`, r.schema), customer, cmd.SourceEntryID, cmd.Choice, cmd.AmountCents, targetID, cmd.PostingDate, cmd.Account, cmd.Reference, cmd.Note, cmd.IdempotencyKey, hash(raw), actor); err != nil {
		return err
	}
	if err = infra.AuditInsertTx(ctx, tx, r.schema, actor, "order", &sourceOrder, "customer_balance_use", nil, nil, nil, infra.AuditMeta{"command": cmd, "customer_id": customer}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
