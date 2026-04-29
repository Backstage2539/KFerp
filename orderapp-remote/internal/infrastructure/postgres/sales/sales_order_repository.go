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
	q := fmt.Sprintf(`SELECT s.company_name, s.note, s.payment_text,
			COALESCE(a.id,0), COALESCE(a.kind,''), COALESCE(a.filename,''), COALESCE(a.content_type,''), COALESCE(a.bytes,0), COALESCE(a.sha256,''), COALESCE(a.object_key,''), COALESCE(to_char(a.created_at,'YYYY-MM-DD HH24:MI:SS'),''), COALESCE(a.created_by,'')
		FROM %s.sales_order_settings s
		LEFT JOIN %s.sales_order_assets a ON a.id=s.seal_asset_id
		WHERE s.id=1`, r.schema, r.schema)
	var seal salesapp.SalesOrderAsset
	err := r.pool.QueryRow(ctx, q).Scan(&settings.CompanyName, &settings.Note, &settings.PaymentText, &seal.ID, &seal.Kind, &seal.Filename, &seal.ContentType, &seal.Bytes, &seal.SHA256, &seal.ObjectKey, &seal.CreatedAt, &seal.CreatedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return salesapp.SalesOrderSettings{}, nil
	}
	if err != nil {
		return salesapp.SalesOrderSettings{}, err
	}
	if seal.ID > 0 {
		seal.URL = salesOrderAssetURL(seal.ObjectKey)
		settings.Seal = &seal
	}
	codes, err := r.loadSalesOrderPaymentCodes(ctx)
	if err != nil {
		return salesapp.SalesOrderSettings{}, err
	}
	settings.PaymentCodes = codes
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

func (r Repository) SaveSalesOrderAsset(ctx context.Context, cmd salesapp.SaveSalesOrderAssetCommand) (salesapp.SalesOrderAsset, error) {
	var asset salesapp.SalesOrderAsset
	q := fmt.Sprintf(`INSERT INTO %s.sales_order_assets(kind, filename, content_type, bytes, sha256, object_key, created_by)
		VALUES($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, kind, filename, content_type, bytes, sha256, object_key, to_char(created_at,'YYYY-MM-DD HH24:MI:SS'), created_by`, r.schema)
	if err := r.pool.QueryRow(ctx, q, cmd.Kind, cmd.Filename, cmd.ContentType, cmd.Bytes, cmd.SHA256, cmd.ObjectKey, cmd.Actor).Scan(&asset.ID, &asset.Kind, &asset.Filename, &asset.ContentType, &asset.Bytes, &asset.SHA256, &asset.ObjectKey, &asset.CreatedAt, &asset.CreatedBy); err != nil {
		return salesapp.SalesOrderAsset{}, err
	}
	asset.URL = salesOrderAssetURL(asset.ObjectKey)
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, cmd.Actor, "sales_order_asset", &asset.ID, "create", postgresinfra.StrPtr("kind"), nil, postgresinfra.StrPtr(cmd.Kind), postgresinfra.AuditMeta{"object_key": asset.ObjectKey, "bytes": asset.Bytes})
	return asset, nil
}

func (r Repository) SaveSalesOrderPaymentCode(ctx context.Context, cmd salesapp.SaveSalesOrderPaymentCodeCommand) (salesapp.SalesOrderPaymentCode, error) {
	var code salesapp.SalesOrderPaymentCode
	if cmd.ID > 0 {
		q := fmt.Sprintf(`UPDATE %s.sales_order_payment_codes
			SET label=$2, description=$3, asset_id=$4, sort=$5, active=$6, updated_at=now()
			WHERE id=$1
			RETURNING id, label, description, asset_id, sort, active`, r.schema)
		if err := r.pool.QueryRow(ctx, q, cmd.ID, cmd.Label, cmd.Description, cmd.AssetID, cmd.Sort, cmd.Active).Scan(&code.ID, &code.Label, &code.Description, &code.AssetID, &code.Sort, &code.Active); err != nil {
			return salesapp.SalesOrderPaymentCode{}, err
		}
	} else {
		q := fmt.Sprintf(`INSERT INTO %s.sales_order_payment_codes(label, description, asset_id, sort, active)
			VALUES($1,$2,$3,$4,$5)
			RETURNING id, label, description, asset_id, sort, active`, r.schema)
		if err := r.pool.QueryRow(ctx, q, cmd.Label, cmd.Description, cmd.AssetID, cmd.Sort, cmd.Active).Scan(&code.ID, &code.Label, &code.Description, &code.AssetID, &code.Sort, &code.Active); err != nil {
			return salesapp.SalesOrderPaymentCode{}, err
		}
	}
	postgresinfra.AuditInsert(ctx, r.pool, r.schema, cmd.Actor, "sales_order_payment_code", &code.ID, "update", postgresinfra.StrPtr("label"), nil, postgresinfra.StrPtr(code.Label), postgresinfra.AuditMeta{"asset_id": code.AssetID})
	return code, nil
}

func (r Repository) DeleteSalesOrderPaymentCode(ctx context.Context, id int64, actor string) error {
	q := fmt.Sprintf(`UPDATE %s.sales_order_payment_codes SET active=false, updated_at=now() WHERE id=$1`, r.schema)
	_, err := r.pool.Exec(ctx, q, id)
	if err == nil {
		postgresinfra.AuditInsert(ctx, r.pool, r.schema, actor, "sales_order_payment_code", &id, "delete", postgresinfra.StrPtr("active"), postgresinfra.StrPtr("true"), postgresinfra.StrPtr("false"), nil)
	}
	return err
}

func (r Repository) SetSalesOrderSealAsset(ctx context.Context, assetID int64, actor string) error {
	q := fmt.Sprintf(`INSERT INTO %s.sales_order_settings(id, seal_asset_id, updated_at, updated_by)
		VALUES(1,$1,now(),$2)
		ON CONFLICT(id) DO UPDATE SET seal_asset_id=excluded.seal_asset_id, updated_at=now(), updated_by=excluded.updated_by`, r.schema)
	_, err := r.pool.Exec(ctx, q, assetID, actor)
	if err == nil {
		postgresinfra.AuditInsert(ctx, r.pool, r.schema, actor, "sales_order_settings", nil, "update", postgresinfra.StrPtr("seal_asset_id"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", assetID)), nil)
	}
	return err
}

func (r Repository) loadSalesOrderPaymentCodes(ctx context.Context) ([]salesapp.SalesOrderPaymentCode, error) {
	q := fmt.Sprintf(`SELECT pc.id, pc.label, pc.description, pc.asset_id, pc.sort, pc.active,
			a.id, a.kind, a.filename, a.content_type, a.bytes, a.sha256, a.object_key, to_char(a.created_at,'YYYY-MM-DD HH24:MI:SS'), a.created_by
		FROM %s.sales_order_payment_codes pc
		JOIN %s.sales_order_assets a ON a.id=pc.asset_id
		WHERE pc.active=true
		ORDER BY pc.sort, pc.id`, r.schema, r.schema)
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]salesapp.SalesOrderPaymentCode, 0)
	for rows.Next() {
		var code salesapp.SalesOrderPaymentCode
		if err := rows.Scan(&code.ID, &code.Label, &code.Description, &code.AssetID, &code.Sort, &code.Active, &code.Asset.ID, &code.Asset.Kind, &code.Asset.Filename, &code.Asset.ContentType, &code.Asset.Bytes, &code.Asset.SHA256, &code.Asset.ObjectKey, &code.Asset.CreatedAt, &code.Asset.CreatedBy); err != nil {
			return nil, err
		}
		code.Asset.URL = salesOrderAssetURL(code.Asset.ObjectKey)
		out = append(out, code)
	}
	return out, rows.Err()
}

func salesOrderAssetURL(objectKey string) string {
	if objectKey == "" {
		return ""
	}
	return "/assets/" + objectKey
}
