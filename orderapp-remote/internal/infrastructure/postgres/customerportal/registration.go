package customerportal

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	app "orderapp/internal/application/customerportal"
	pg "orderapp/internal/infrastructure/postgres"
)

func ensureRegistrationSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %[1]s.mini_registrations(
 mini_user_id BIGINT PRIMARY KEY REFERENCES %[1]s.mini_users(id), nickname TEXT NOT NULL CHECK(length(nickname) BETWEEN 1 AND 32),
 verified_phone TEXT NOT NULL CHECK(length(verified_phone) BETWEEN 5 AND 32), verified_at TIMESTAMPTZ NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(), last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now());
 CREATE TABLE IF NOT EXISTS %[1]s.mini_phone_credentials(credential_hash TEXT PRIMARY KEY,used_at TIMESTAMPTZ NOT NULL DEFAULT now());`, schema))
	return err
}
func (r Repository) registrationTx(ctx context.Context, tx pgx.Tx, id int64) (*app.RegistrationProfile, error) {
	var p app.RegistrationProfile
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT p.mini_user_id,p.nickname,p.verified_phone,p.verified_at,p.created_at,p.last_seen_at,u.active FROM %s.mini_registrations p JOIN %s.mini_users u ON u.id=p.mini_user_id WHERE p.mini_user_id=$1`, r.schema, r.schema), id).Scan(&p.MiniUserID, &p.Nickname, &p.VerifiedPhone, &p.VerifiedAt, &p.CreatedAt, &p.LastSeenAt, &p.Active)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}
func (r Repository) UpdateRegistrationNickname(ctx context.Context, id int64, name string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.mini_registrations p SET nickname=$2,updated_at=now() FROM %s.mini_users u WHERE p.mini_user_id=$1 AND u.id=p.mini_user_id AND u.active`, r.schema, r.schema), id, name)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return app.ErrMiniSessionNotFound
	}
	if err = r.auditRegistrationTx(ctx, tx, id, "update_nickname"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) auditRegistrationTx(ctx context.Context, tx pgx.Tx, id int64, action string) error {
	return pg.AuditInsertTx(ctx, tx, r.schema, fmt.Sprintf("mini-user:%d", id), "mini_registration", &id, action, nil, nil, nil, pg.AuditMeta{"mini_user_id": id})
}
func (r Repository) ListRegistrations(ctx context.Context, q string, page, size int) ([]app.RegistrationProfile, int, error) {
	total := 0
	filter := ` FROM %[1]s.mini_registrations p JOIN %[1]s.mini_users u ON u.id=p.mini_user_id WHERE ($1='' OR p.nickname ILIKE '%%'||$1||'%%' OR p.verified_phone ILIKE '%%'||$1||'%%')`
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*)`+filter, r.schema), q).Scan(&total)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT p.mini_user_id,p.nickname,p.verified_phone,p.verified_at,p.created_at,p.last_seen_at,u.active`+filter+` ORDER BY p.created_at DESC,p.mini_user_id DESC LIMIT $2 OFFSET $3`, r.schema), q, size, (page-1)*size)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []app.RegistrationProfile{}
	for rows.Next() {
		var p app.RegistrationProfile
		if err = rows.Scan(&p.MiniUserID, &p.Nickname, &p.VerifiedPhone, &p.VerifiedAt, &p.CreatedAt, &p.LastSeenAt, &p.Active); err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}
func (r Repository) DisableRegistration(ctx context.Context, id int64, actor string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.mini_users SET active=false WHERE id=$1 AND EXISTS(SELECT 1 FROM %s.mini_registrations WHERE mini_user_id=$1)`, r.schema, r.schema), id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return app.ErrMiniInvalidLogin
	}
	if err = r.expireMiniUserSessionsTx(ctx, tx, id); err != nil {
		return err
	}
	if err = pg.AuditInsertTx(ctx, tx, r.schema, actor, "mini_registration", &id, "disable", nil, nil, nil, pg.AuditMeta{"mini_user_id": id}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
