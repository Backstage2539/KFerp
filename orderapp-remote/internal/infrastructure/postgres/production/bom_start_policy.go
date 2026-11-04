package production

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/production"
	"os"
	"strings"
)

// Freeze execution defaults on the work order, never mutate the selected route
// or creator template. Existing explicit route configuration wins. A plain
// route uses the BOM's output unit throughout and receipts at its last step.
func freezeBOMStartRoute(raw, unit, warehouse string) (string, error) {
	var route processTemplateSnapshot
	if err := json.Unmarshal([]byte(raw), &route); err != nil {
		return "", fmt.Errorf("工艺路线快照无效：%w", err)
	}
	if _, err := firstFrozenRouteSequence(route); err != nil {
		return "", err
	}
	if strings.TrimSpace(unit) == "" {
		return "", fmt.Errorf("BOM 产出缺少单位")
	}
	last := 0
	for _, op := range route.Operations {
		if op.Seq > last {
			last = op.Seq
		}
	}
	nodes := map[string]bool{}
	for i := range route.Operations {
		op := &route.Operations[i]
		var config app.OperationExecution
		if strings.TrimSpace(op.ExecutionJSON) != "" {
			if err := json.Unmarshal([]byte(op.ExecutionJSON), &config); err != nil {
				return "", fmt.Errorf("工序 %d 执行配置无效", op.Seq)
			}
		}
		if config.Version == 0 {
			config.Version = 1
			config.NodeKey = fmt.Sprintf("route-seq-%d", op.Seq)
			config.ReceiptEnabled = op.Seq == last
		}
		if config.Version != 1 || strings.TrimSpace(config.NodeKey) == "" || nodes[config.NodeKey] {
			return "", fmt.Errorf("工序 %d 执行版本或节点标识无效", op.Seq)
		}
		nodes[config.NodeKey] = true
		config.OutputUnit = firstNonEmpty(config.OutputUnit, unit)
		config.SourceWarehouse = firstNonEmpty(config.SourceWarehouse, "wip")
		config.ReceiptWarehouse = firstNonEmpty(config.ReceiptWarehouse, warehouse, "wip")
		if op.Seq == last && config.OutputUnit != unit {
			return "", fmt.Errorf("末道工序产出单位与 BOM 不一致")
		}
		value, err := json.Marshal(config)
		if err != nil {
			return "", err
		}
		op.ExecutionJSON = string(value)
	}
	value, err := json.Marshal(route)
	return string(value), err
}

// The compatible rollback build retains the additive schema and this guard.
// Enabling it pauses all new-policy business transactions; legacy work can run.
func checkBOMStartWriteTx(ctx context.Context, tx pgx.Tx, schema string, workID int64) error {
	if os.Getenv("KFERP_BOM_START_READ_ONLY") != "1" {
		return nil
	}
	var policy string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT material_consumption_policy FROM %s.work_orders WHERE id=$1`, schema), workID).Scan(&policy); err != nil {
		return err
	}
	if policy == "bom_start_v1" {
		return fmt.Errorf("新策略工单已暂停写入，请待开工扣料版本恢复后继续")
	}
	return nil
}

func requireLegacyExecutionTx(ctx context.Context, tx pgx.Tx, schema string, workID int64) error {
	var policy string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT material_consumption_policy FROM %s.work_orders WHERE id=$1`, schema), workID).Scan(&policy); err != nil {
		return err
	}
	if policy == "bom_start_v1" {
		return fmt.Errorf("请先确认用料并开工；本工单须在工位报工或生产记录中更正")
	}
	return nil
}
