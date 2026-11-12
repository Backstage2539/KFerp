package finance

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/finance"
)

func (r Repository) LoadDailySnapshot(ctx context.Context, month string) (app.DailySnapshot, bool, error) {
	var raw []byte
	err := r.query().QueryRow(ctx, fmt.Sprintf(`SELECT snapshot_json->'daily_ledger' FROM %s.finance_monthly_reports WHERE month=$1`, r.schema), month).Scan(&raw)
	if err == pgx.ErrNoRows {
		return app.DailySnapshot{}, false, nil
	}
	if err != nil {
		return app.DailySnapshot{}, false, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return app.DailySnapshot{}, false, nil
	}
	var snapshot app.DailySnapshot
	if err = json.Unmarshal(raw, &snapshot); err != nil {
		return app.DailySnapshot{}, false, err
	}
	return snapshot, true, nil
}

func (r Repository) saveDailySnapshot(ctx context.Context, month string, snapshot app.DailySnapshot) error {
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	_, err = r.query().Exec(ctx, fmt.Sprintf(`UPDATE %s.finance_monthly_reports SET snapshot_json=jsonb_set(snapshot_json,'{daily_ledger}',$2::jsonb,true) WHERE month=$1`, r.schema), month, string(raw))
	return err
}
