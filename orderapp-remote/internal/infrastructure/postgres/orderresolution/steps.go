package orderresolution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	productionapp "orderapp/internal/application/production"
	domain "orderapp/internal/domain/orderresolution"
	infra "orderapp/internal/infrastructure/postgres"
	productionpg "orderapp/internal/infrastructure/postgres/production"
	"strings"
	"time"
)

type StepCommand struct {
	Choice         string `json:"choice"`
	AmountCents    int64  `json:"amount_cents"`
	Reference      string `json:"reference"`
	Account        string `json:"account"`
	Note           string `json:"note"`
	TargetID       int64  `json:"target_id"`
	PostingDate    string `json:"posting_date"`
	IdempotencyKey string `json:"idempotency_key"`
}

func (r Repository) CompleteStep(ctx context.Context, orderID, requestID, stepID int64, cmd StepCommand, actor string) (Result, error) {
	out, err := r.completeStep(ctx, orderID, requestID, stepID, cmd, actor)
	if err != nil && strings.TrimSpace(actor) != "" && strings.TrimSpace(cmd.IdempotencyKey) != "" && len(cmd.IdempotencyKey) <= 128 {
		// The business transaction has already rolled back. Failure evidence is a
		// separate audited transaction and must never preserve partial effects.
		if recordErr := r.recordStepFailure(ctx, orderID, requestID, stepID, cmd, actor, err); recordErr != nil {
			return out, errors.Join(err, fmt.Errorf("本次失败记录未能保存，请刷新处理单后重试"))
		}
	}
	return out, err
}

func (r Repository) completeStep(ctx context.Context, orderID, requestID, stepID int64, cmd StepCommand, actor string) (Result, error) {
	var out Result
	if strings.TrimSpace(actor) == "" || strings.TrimSpace(cmd.IdempotencyKey) == "" || len(cmd.IdempotencyKey) > 128 {
		return out, fmt.Errorf("缺少操作人或提交标识")
	}
	raw, err := json.Marshal(cmd)
	if err != nil {
		return out, err
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	if err = r.lock(ctx, tx, orderID); err != nil {
		return out, err
	}
	var applied bool
	var originalRaw []byte
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT applied,context_json FROM %s.order_resolution_requests WHERE id=$1 AND order_id=$2 FOR UPDATE`, r.schema), requestID, orderID).Scan(&applied, &originalRaw); err != nil {
		return out, err
	}
	out, err = r.result(ctx, tx, requestID)
	if err != nil {
		return out, err
	}
	var original domain.Context
	if err = json.Unmarshal(originalRaw, &original); err != nil {
		return out, err
	}
	var step Step
	found := false
	for _, s := range out.Steps {
		if s.ID == stepID {
			step = s
			found = true
			break
		}
	}
	if !found {
		return out, fmt.Errorf("待办不属于本处理单")
	}
	var previous string
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT request_hash FROM %s.order_resolution_step_actions WHERE step_id=$1 AND idempotency_key=$2`, r.schema), stepID, cmd.IdempotencyKey).Scan(&previous)
	if err == nil {
		if previous != hash(raw) {
			return out, fmt.Errorf("提交标识已用于不同内容")
		}
		return out, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	if out.Status != "pending" || !one(step.Status, "pending", "failed") {
		return out, fmt.Errorf("本待办已完成或处理单已结束")
	}
	if reason := unavailableCapabilities()[step.Kind+":"+cmd.Choice]; reason != "" {
		return out, fmt.Errorf("%s", reason)
	}
	currentPolicy, err := r.policy(ctx, tx)
	if err != nil {
		return out, err
	}
	if !currentPolicy.Policy.Allows("action", out.Command.Action) || !currentPolicy.Policy.Allows(step.Kind, cmd.Choice) {
		return out, fmt.Errorf("管理员已停用此操作或处置，请联系管理员核对处理单")
	}
	if step.Kind == "approval" && currentPolicy.Policy.Approvals[out.Command.Action] != step.Permission {
		return out, fmt.Errorf("审批要求已变化，请重新预览处理方案")
	}
	completed := true
	switch step.Kind {
	case "stock_reservation":
		if cmd.Choice != step.Choice || !one(cmd.Choice, "release", "keep") || strings.TrimSpace(cmd.Note) == "" {
			return out, fmt.Errorf("请确认预览中的预留处置并填写仓库说明")
		}
		// Release is posted with the final order adjustment, so an approved
		// quantity never leaves a half-applied order if another check fails.
	case "direct_ship":
		if cmd.Choice != step.Choice || !one(cmd.Choice, "cancel", "replace_supply") || strings.TrimSpace(cmd.Note) == "" {
			return out, fmt.Errorf("请确认预览中的代发处置并填写说明")
		}
		if cmd.Choice == "replace_supply" {
			err = fmt.Errorf("缺少原预览的替代货源选择，请刷新处理方案")
			for _, disposition := range out.Command.Dispositions {
				if disposition.Kind == "direct_ship" && disposition.ID == step.SourceID {
					err = r.replaceSupply(ctx, tx, orderID, requestID, step.SourceID, disposition, cmd.Note, actor)
					break
				}
			}
		} else {
			err = r.cancelDirectShip(ctx, tx, orderID, requestID, step.SourceID, cmd.Note, actor)
		}
	case "funds":
		for _, other := range out.Steps {
			if other.ID != stepID && other.Kind != "funds" && one(other.Status, "pending", "failed") {
				return out, fmt.Errorf("请先完成生产、库存、费用及账务处置，再处理资金")
			}
		}
		completed, err = r.applyFunds(ctx, tx, orderID, requestID, stepID, step.SourceID, applied, out.Plan, cmd, actor)
	case "production":
		if cmd.Choice != step.Choice || strings.TrimSpace(cmd.Note) == "" {
			return out, fmt.Errorf("请确认原预览的生产处置并填写说明")
		}
		if cmd.Choice == "continue_company" {
			err = r.confirmCompanyProduction(ctx, tx, orderID, requestID, step.SourceID, cmd.Note, actor)
			break
		}
		if cmd.Choice != "cancel_unstarted" {
			return out, fmt.Errorf("该生产处置需核对余料、在制品及目标需求")
		}
		var shared bool
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.production_plan_items WHERE production_plan_id=$1 AND cardinality(string_to_array(replace(order_nos,' ',''),','))>1)`, r.schema), step.SourceID).Scan(&shared)
		if err == nil && shared {
			return out, fmt.Errorf("共享生产不能整体取消，请先拆分剩余需求")
		}
		if err == nil {
			_, err = productionpg.NewRepository(r.pool, r.schema).CancelProductionPlanTx(ctx, tx, productionapp.CancelProductionPlanCommand{ID: step.SourceID, Operator: actor, Note: cmd.Note})
		}
	case "fee":
		if cmd.Choice != step.Choice || strings.TrimSpace(cmd.Note) == "" || cmd.Choice == "defer" {
			return out, fmt.Errorf("请确认预览中的费用处置并填写财务说明；暂不处理项需先更新方案")
		}
	case "billing":
		if cmd.Choice != "adjust" || strings.TrimSpace(cmd.Note) == "" || strings.TrimSpace(cmd.Reference) == "" {
			return out, fmt.Errorf("请确认账单更正并填写核对凭据及说明")
		}
	case "approval":
		if cmd.Choice != "approve" || strings.TrimSpace(cmd.Note) == "" {
			return out, fmt.Errorf("请明确批准并填写审核说明")
		}
	case "external":
		current, loadErr := r.load(ctx, tx, orderID)
		if loadErr != nil {
			return out, loadErr
		}
		if current.ExternalVersion != original.ExternalVersion {
			return out, fmt.Errorf("%w：开票记录已变化，请刷新处理方案后核实凭据", ErrStale)
		}
		if !one(cmd.Choice, "none", "recorded") || strings.TrimSpace(cmd.Note) == "" {
			return out, fmt.Errorf("请选择核实结果并填写说明")
		}
		if original.Invoiced && cmd.Choice == "none" {
			return out, fmt.Errorf("原单已有关联开票记录，请登记冲减或更正凭据")
		}
		if cmd.Choice == "recorded" && strings.TrimSpace(cmd.Reference) == "" {
			return out, fmt.Errorf("请登记冲减或更正凭据")
		}
	case "period":
		if cmd.Choice != "approve" || strings.TrimSpace(cmd.Note) == "" || strings.TrimSpace(cmd.Reference) == "" {
			return out, fmt.Errorf("请确认历史更正并填写批准依据与说明")
		}
		err = r.approveHistoricalPeriod(ctx, tx, requestID, out.Command.PostingDate, cmd, actor)
	default:
		err = fmt.Errorf("请先完成关联业务处置：%s", step.Label)
	}
	if err != nil {
		return out, err
	}
	state := "pending"
	if completed {
		state = "completed"
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_resolution_steps SET status=$2,choice=$3,result_json=$4,actor=$5,last_error='',updated_at=now() WHERE id=$1`, r.schema), stepID, state, cmd.Choice, raw, actor); err != nil {
		return out, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_resolution_step_actions(step_id,idempotency_key,request_hash,result_json,actor)VALUES($1,$2,$3,$4,$5)`, r.schema), stepID, cmd.IdempotencyKey, hash(raw), raw, actor); err != nil {
		return out, err
	}
	out, err = r.result(ctx, tx, requestID)
	if err != nil {
		return out, err
	}
	pending := []domain.Task{}
	for _, s := range out.Steps {
		if one(s.Status, "pending", "failed") {
			pending = append(pending, s.Task)
		}
	}
	if !applied && (len(pending) == 0 || (out.Command.Timing == "stop_first" && onlyFunds(pending))) {
		if out.Command.PostingMode == "open_period" {
			if err = r.checkPeriod(ctx, tx, out.Command.PostingDate); err != nil {
				return out, err
			}
		}
		if err = r.apply(ctx, tx, orderID, requestID, original, out.Command, out.Plan, actor); err != nil {
			return out, err
		}
	}
	if len(pending) == 0 {
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_resolution_requests SET status='completed',updated_at=now() WHERE id=$1`, r.schema), requestID); err != nil {
			return out, err
		}
	}
	if err = infra.AuditInsertTx(ctx, tx, r.schema, actor, "order", &orderID, "resolution_step", nil, nil, nil, infra.AuditMeta{"resolution_id": requestID, "step_id": stepID, "kind": step.Kind, "command": cmd}); err != nil {
		return out, err
	}
	out, err = r.result(ctx, tx, requestID)
	if err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}
func (r Repository) lock(ctx context.Context, tx pgx.Tx, id int64) error {
	var customer int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(customer_id,0)FROM %s.orders WHERE id=$1`, r.schema), id).Scan(&customer); err != nil {
		return err
	}
	for _, key := range []string{fmt.Sprintf("mini-direct-ship:%d", customer), r.schema + ":finance-period", fmt.Sprintf("order-resolution:%d", id)} {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, key); err != nil {
			return err
		}
	}
	_, err := tx.Exec(ctx, fmt.Sprintf(`SELECT id FROM %s.orders WHERE id=$1 FOR UPDATE`, r.schema), id)
	return err
}
func (r Repository) applyFunds(ctx context.Context, tx pgx.Tx, orderID, requestID, stepID, feeID int64, applied bool, p domain.Plan, cmd StepCommand, actor string) (bool, error) {
	if !domain.AllowedChoice("funds", cmd.Choice) || cmd.AmountCents <= 0 {
		return false, fmt.Errorf("请选择资金去向并填写正数金额")
	}
	if _, err := time.Parse("2006-01-02", cmd.PostingDate); err != nil {
		return false, fmt.Errorf("请选择实际处理日期")
	}
	if err := r.checkPeriod(ctx, tx, cmd.PostingDate); err != nil {
		return false, err
	}
	if strings.TrimSpace(cmd.Note) == "" {
		return false, fmt.Errorf("请填写资金处理说明")
	}
	if cmd.Choice == "refund" && (strings.TrimSpace(cmd.Account) == "" || strings.TrimSpace(cmd.Reference) == "") {
		return false, fmt.Errorf("请填写实际付款账户和退款凭据")
	}
	var customer, total, credit, received, used int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(customer_id,0),round((CASE WHEN grand_total<>0 OR discount_amount<>0 OR shipping_amount<>0 OR rounding_amount<>0 THEN grand_total ELSE total_amount END)*100)::bigint,resolution_credit_cents,round(GREATEST(COALESCE(prepayment_amount,0),COALESCE(payment_goods_amount,0)+COALESCE(payment_shipping_amount,0))*100)::bigint FROM %s.orders WHERE id=$1 FOR UPDATE`, r.schema), orderID).Scan(&customer, &total, &credit, &received); err != nil {
		return false, err
	}
	if customer <= 0 && cmd.Choice != "refund" {
		return false, fmt.Errorf("客户身份不完整，不能留存余额或抵扣其他应收")
	}
	current, err := r.load(ctx, tx, orderID)
	if err != nil {
		return false, err
	}
	if current.PaidCents > received {
		received = current.PaidCents
	}
	if !applied {
		credit += p.CreditCents
	}
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(sum(amount_cents),0)FROM %s.order_resolution_financial_entries WHERE order_id=$1 AND source_fee_id=$2 AND kind IN('refund','customer_credit','offset')`, r.schema), orderID, feeID).Scan(&used); err != nil {
		return false, err
	}
	available := max(int64(0), received+current.OffsetCents-max(int64(0), total-credit)) - used
	if feeID > 0 {
		found := false
		for _, fee := range current.Dependencies {
			if fee.Kind != "fee" || fee.ID != feeID {
				continue
			}
			found = true
			net := fee.AmountCents
			if !applied {
				for _, effect := range p.Fees {
					if effect.SourceID == feeID {
						net -= effect.CreditCents
					}
				}
			}
			available = max(int64(0), fee.PaidCents-max(int64(0), net)) - used
		}
		if !found {
			return false, fmt.Errorf("费用资金来源已变化，请重新核对")
		}
	}
	if cmd.AmountCents > available {
		return false, fmt.Errorf("处理金额超过当前可退余额 ¥%.2f", float64(max(int64(0), available))/100)
	}
	reference := cmd.Reference
	if reference == "" {
		reference = fmt.Sprintf("step:%d:%s", stepID, cmd.IdempotencyKey)
	}
	if cmd.Choice == "offset" {
		if cmd.TargetID <= 0 || cmd.TargetID == orderID {
			return false, fmt.Errorf("请选择同客户的另一笔应收")
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`SELECT id FROM %s.orders WHERE id=$1 FOR UPDATE`, r.schema), cmd.TargetID); err != nil {
			return false, err
		}
		var processing bool
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.order_resolution_requests WHERE order_id=$1 AND status='pending')`, r.schema), cmd.TargetID).Scan(&processing); err != nil {
			return false, err
		}
		if processing {
			return false, fmt.Errorf("目标订单正在处理中，请完成后再抵扣")
		}
		target, e := r.load(ctx, tx, cmd.TargetID)
		if e != nil {
			return false, e
		}
		due := max(int64(0), target.TotalCents-target.CreditCents-max(int64(0), target.PaidCents+target.OffsetCents-target.CreditUsedCents))
		if target.CustomerID != customer || target.Void || cmd.AmountCents > due {
			return false, fmt.Errorf("目标订单客户、状态或可抵扣金额不符")
		}

		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET resolution_offset_cents=resolution_offset_cents+$2 WHERE id=$1`, r.schema), cmd.TargetID, cmd.AmountCents); err != nil {
			return false, err
		}
		if err = infra.AuditInsertTx(ctx, tx, r.schema, actor, "order", &cmd.TargetID, "resolution_offset", nil, nil, nil, infra.AuditMeta{"source_order_id": orderID, "resolution_id": requestID, "amount_cents": cmd.AmountCents}); err != nil {
			return false, err
		}
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_resolution_financial_entries(request_id,order_id,customer_id,posting_date,kind,amount_cents,reference,actor,account,target_order_id,source_fee_id)VALUES($1,$2,$3,$4::date,$5,$6,$7,$8,$9,$10,$11)`, r.schema), requestID, orderID, customer, cmd.PostingDate, cmd.Choice, cmd.AmountCents, reference, actor, cmd.Account, cmd.TargetID, feeID); err != nil {
		return false, err
	}
	table, source := "orders", orderID
	if feeID > 0 {
		table, source = "customer_fee_items", feeID
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.%s SET resolution_settled_out_cents=resolution_settled_out_cents+$2 WHERE id=$1`, r.schema, table), source, cmd.AmountCents); err != nil {
		return false, err
	}
	return cmd.AmountCents == available, nil
}
func (r Repository) approveHistoricalPeriod(ctx context.Context, tx pgx.Tx, requestID int64, date string, cmd StepCommand, actor string) error {
	var raw []byte
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT snapshot_json FROM %s.finance_monthly_reports WHERE month=left($1,7) AND status IN('closed','adjusted') FOR UPDATE`, r.schema), date).Scan(&raw)
	if err != nil {
		return fmt.Errorf("所选历史账期没有已结账快照，请使用开放账期调整")
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_resolution_period_confirmations(request_id,month,original_snapshot,reference,note,actor)VALUES($1,left($2,7),$3,$4,$5,$6)`, r.schema), requestID, date, raw, cmd.Reference, cmd.Note, actor); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_resolution_period_approvals(request_id,month,original_snapshot,reference,note,actor)VALUES($1,left($2,7),$3,$4,$5,$6)ON CONFLICT(request_id)DO UPDATE SET month=EXCLUDED.month,original_snapshot=EXCLUDED.original_snapshot,reference=EXCLUDED.reference,note=EXCLUDED.note,actor=EXCLUDED.actor,created_at=now()`, r.schema), requestID, date, raw, cmd.Reference, cmd.Note, actor)
	return err
}
