package sales

import (
	"context"
	"errors"
	"fmt"
	salesapp "orderapp/internal/application/sales"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
)

func (r Repository) LoadSalesOrderSettings(ctx context.Context) (salesapp.SalesOrderSettings, error) {
	var settings salesapp.SalesOrderSettings
	q := fmt.Sprintf(`SELECT company_name, note, payment_text
		FROM %s.sales_order_settings
		WHERE id=1`, r.schema)
	err := r.pool.QueryRow(ctx, q).Scan(&settings.CompanyName, &settings.Note, &settings.PaymentText)
	if errors.Is(err, pgx.ErrNoRows) {
		return salesapp.SalesOrderSettings{}, nil
	}
	if err != nil {
		return salesapp.SalesOrderSettings{}, err
	}
	return settings, nil
}

func (r Repository) SaveSalesOrderSettings(ctx context.Context, cmd salesapp.SaveSalesOrderSettingsCommand) error {
	q := fmt.Sprintf(`INSERT INTO %s.sales_order_settings(id, company_name, note, payment_text, updated_at, updated_by)
		VALUES(1,$1,$2,$3,now(),$4)
		ON CONFLICT(id) DO UPDATE SET
			company_name=excluded.company_name,
			note=excluded.note,
			payment_text=excluded.payment_text,
			updated_at=now(),
			updated_by=excluded.updated_by`, r.schema)
	_, err := r.pool.Exec(ctx, q, cmd.CompanyName, cmd.Note, cmd.PaymentText, cmd.Actor)
	if err == nil {
		postgresinfra.AuditInsert(ctx, r.pool, r.schema, cmd.Actor, "sales_order_settings", nil, "update", postgresinfra.StrPtr("settings"), nil, postgresinfra.StrPtr(cmd.CompanyName), postgresinfra.AuditMeta{"company_name": cmd.CompanyName})
	}
	return err
}
