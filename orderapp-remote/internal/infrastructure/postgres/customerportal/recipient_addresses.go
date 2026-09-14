package customerportal

import (
	"context"
	"errors"
	"fmt"
	"strings"

	customerportalapp "orderapp/internal/application/customerportal"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
)

func (r Repository) ListCustomerRecipientAddresses(ctx context.Context, customerID int64, search string) ([]customerportalapp.CustomerRecipientAddress, error) {
	pattern := "%" + strings.TrimSpace(search) + "%"
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id,customer_id,recipient_name,phone,company,province,city,district,detail_address,
		       is_default,revision,to_char(updated_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM %s.customer_recipient_addresses
		WHERE customer_id=$1 AND active=true
		  AND ($2='%%' OR recipient_name ILIKE $2 OR phone ILIKE $2 OR company ILIKE $2
		       OR (province||city||district||detail_address) ILIKE $2)
		ORDER BY is_default DESC,updated_at DESC,id DESC
	`, r.schema), customerID, pattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []customerportalapp.CustomerRecipientAddress{}
	for rows.Next() {
		var row customerportalapp.CustomerRecipientAddress
		if err := rows.Scan(&row.ID, &row.CustomerID, &row.RecipientName, &row.Phone, &row.Company, &row.Province, &row.City, &row.District, &row.DetailAddress, &row.IsDefault, &row.Revision, &row.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) SaveCustomerRecipientAddress(ctx context.Context, cmd customerportalapp.SaveCustomerRecipientAddressCommand) (customerportalapp.CustomerRecipientAddress, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return customerportalapp.CustomerRecipientAddress{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, fmt.Sprintf("customer-recipient-address:%d", cmd.CustomerID)); err != nil {
		return customerportalapp.CustomerRecipientAddress{}, err
	}
	var activeCount int
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.customer_recipient_addresses WHERE customer_id=$1 AND active=true`, r.schema), cmd.CustomerID).Scan(&activeCount); err != nil {
		return customerportalapp.CustomerRecipientAddress{}, err
	}
	if activeCount == 0 {
		cmd.IsDefault = true
	}
	if cmd.IsDefault {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_recipient_addresses SET is_default=false,revision=revision+1,updated_at=now(),updated_by=$2 WHERE customer_id=$1 AND active=true AND is_default=true AND id<>$3`, r.schema), cmd.CustomerID, cmd.Actor, cmd.ID); err != nil {
			return customerportalapp.CustomerRecipientAddress{}, err
		}
	}
	var row customerportalapp.CustomerRecipientAddress
	if cmd.ID <= 0 {
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.customer_recipient_addresses(
				customer_id,recipient_name,phone,company,province,city,district,detail_address,is_default,created_by,updated_by
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$10)
			RETURNING id,customer_id,recipient_name,phone,company,province,city,district,detail_address,is_default,revision,
			          to_char(updated_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
		`, r.schema), cmd.CustomerID, cmd.RecipientName, cmd.Phone, cmd.Company, cmd.Province, cmd.City, cmd.District, cmd.DetailAddress, cmd.IsDefault, cmd.Actor).Scan(
			&row.ID, &row.CustomerID, &row.RecipientName, &row.Phone, &row.Company, &row.Province, &row.City, &row.District, &row.DetailAddress, &row.IsDefault, &row.Revision, &row.UpdatedAt,
		)
	} else {
		if cmd.ExpectedRevision <= 0 {
			return customerportalapp.CustomerRecipientAddress{}, fmt.Errorf("expected_revision required")
		}
		var revision int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT revision FROM %s.customer_recipient_addresses WHERE id=$1 AND customer_id=$2 AND active=true FOR UPDATE`, r.schema), cmd.ID, cmd.CustomerID).Scan(&revision); errors.Is(err, pgx.ErrNoRows) {
			return customerportalapp.CustomerRecipientAddress{}, fmt.Errorf("收件地址不存在")
		} else if err != nil {
			return customerportalapp.CustomerRecipientAddress{}, err
		}
		if cmd.ExpectedRevision > 0 && cmd.ExpectedRevision != revision {
			return customerportalapp.CustomerRecipientAddress{}, fmt.Errorf("收件地址已变化，请刷新后重试")
		}
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			UPDATE %s.customer_recipient_addresses
			SET recipient_name=$3,phone=$4,company=$5,province=$6,city=$7,district=$8,detail_address=$9,
			    is_default=$10,revision=revision+1,updated_by=$11,updated_at=now()
			WHERE id=$1 AND customer_id=$2 AND active=true
			RETURNING id,customer_id,recipient_name,phone,company,province,city,district,detail_address,is_default,revision,
			          to_char(updated_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
		`, r.schema), cmd.ID, cmd.CustomerID, cmd.RecipientName, cmd.Phone, cmd.Company, cmd.Province, cmd.City, cmd.District, cmd.DetailAddress, cmd.IsDefault, cmd.Actor).Scan(
			&row.ID, &row.CustomerID, &row.RecipientName, &row.Phone, &row.Company, &row.Province, &row.City, &row.District, &row.DetailAddress, &row.IsDefault, &row.Revision, &row.UpdatedAt,
		)
	}
	if err != nil {
		return customerportalapp.CustomerRecipientAddress{}, err
	}
	action := "create_recipient_address"
	if cmd.ID > 0 {
		action = "update_recipient_address"
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "customer_recipient_address", &row.ID, action, postgresinfra.StrPtr("address"), nil, postgresinfra.StrPtr(strings.Join([]string{row.Province, row.City, row.District, row.DetailAddress}, "")), postgresinfra.AuditMeta{"customer_id": cmd.CustomerID, "recipient_name": row.RecipientName, "phone": row.Phone, "company": row.Company, "is_default": row.IsDefault}); err != nil {
		return customerportalapp.CustomerRecipientAddress{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return customerportalapp.CustomerRecipientAddress{}, err
	}
	return row, nil
}

func (r Repository) DeleteCustomerRecipientAddress(ctx context.Context, cmd customerportalapp.DeleteCustomerRecipientAddressCommand) error {
	if cmd.ExpectedRevision <= 0 {
		return fmt.Errorf("expected_revision required")
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, fmt.Sprintf("customer-recipient-address:%d", cmd.CustomerID)); err != nil {
		return err
	}
	var revision int64
	var wasDefault bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT revision,is_default FROM %s.customer_recipient_addresses WHERE id=$1 AND customer_id=$2 AND active=true FOR UPDATE`, r.schema), cmd.ID, cmd.CustomerID).Scan(&revision, &wasDefault); errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("收件地址不存在")
	} else if err != nil {
		return err
	}
	if cmd.ExpectedRevision > 0 && cmd.ExpectedRevision != revision {
		return fmt.Errorf("收件地址已变化，请刷新后重试")
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_recipient_addresses SET active=false,is_default=false,revision=revision+1,updated_by=$3,updated_at=now() WHERE id=$1 AND customer_id=$2`, r.schema), cmd.ID, cmd.CustomerID, cmd.Actor); err != nil {
		return err
	}
	if wasDefault {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.customer_recipient_addresses SET is_default=true,revision=revision+1,updated_by=$2,updated_at=now()
			WHERE id=(SELECT id FROM %s.customer_recipient_addresses WHERE customer_id=$1 AND active=true ORDER BY updated_at DESC,id DESC LIMIT 1)
		`, r.schema, r.schema), cmd.CustomerID, cmd.Actor); err != nil {
			return err
		}
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "customer_recipient_address", &cmd.ID, "delete_recipient_address", postgresinfra.StrPtr("active"), postgresinfra.StrPtr("true"), postgresinfra.StrPtr("false"), postgresinfra.AuditMeta{"customer_id": cmd.CustomerID}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
