package messagecenter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/messagecenter"
)

// financeSnapshotTx freezes only verified customer-owned business facts.
func financeSnapshotTx(ctx context.Context, tx pgx.Tx, schema, event string, customerID int64, raw []byte) ([]byte, error) {
	var source struct {
		ReceiptID int64 `json:"receipt_id"`
		BillID    int64 `json:"bill_id"`
	}
	if err := json.Unmarshal(raw, &source); err != nil {
		return nil, err
	}
	n := app.OrderNotice{CustomerID: customerID}
	amount := func(v int64) string { return fmt.Sprintf("%d.%02d", v/100, v%100) }
	switch event {
	case "receipt_received":
		if source.ReceiptID <= 0 {
			return nil, errors.New("收款单标识无效")
		}
		d, err := receiptDetailTx(ctx, tx, schema, customerID, source.ReceiptID)
		if err != nil {
			return nil, err
		}
		n.ReceiptID = source.ReceiptID
		n.ReceiptNo = d.ReceiptNo
		n.ReceiptDate = d.ReceiptDate
		n.ReceivedAmount = amount(d.ActualCents)
		n.ReceivableAmount = amount(d.ReceivableCents)
	case "bill_issued":
		if source.BillID <= 0 {
			return nil, errors.New("账单标识无效")
		}
		d, err := billDetailTx(ctx, tx, schema, customerID, source.BillID)
		if err != nil {
			return nil, err
		}
		n.BillID = source.BillID
		n.BillNo = d.BillNo
		n.BillAmount = amount(d.AmountCents)
		n.BillPeriod = d.PeriodFrom + "~" + d.PeriodTo
	default:
		return nil, errors.New("未知财务事件")
	}
	if err := tx.QueryRow(ctx, "SELECT name FROM "+schema+".customers WHERE id=$1", customerID).Scan(&n.CustomerName); err != nil {
		return nil, err
	}
	return json.Marshal(n)
}
