package pageentry

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`
 CREATE TABLE IF NOT EXISTS %[1]s.page_entries(
 entry_key text PRIMARY KEY,draft jsonb NOT NULL,published jsonb,revision bigint NOT NULL DEFAULT 1,
 published_revision bigint NOT NULL DEFAULT 0,enabled boolean NOT NULL DEFAULT false,has_draft boolean NOT NULL DEFAULT true,
 deleted_at timestamptz,updated_at timestamptz NOT NULL DEFAULT now());
 CREATE TABLE IF NOT EXISTS %[1]s.page_entry_revisions(
 entry_key text NOT NULL REFERENCES %[1]s.page_entries(entry_key),revision bigint NOT NULL,action text NOT NULL,
 document jsonb NOT NULL,actor text NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),PRIMARY KEY(entry_key,revision));
 CREATE TABLE IF NOT EXISTS %[1]s.page_entry_assets(
 asset_key text PRIMARY KEY,entry_key text NOT NULL REFERENCES %[1]s.page_entries(entry_key),mime text NOT NULL,
 data bytea NOT NULL,created_at timestamptz NOT NULL DEFAULT now());
 CREATE TABLE IF NOT EXISTS %[1]s.page_web_sessions(
 token_hash text PRIMARY KEY,mini_token text NOT NULL DEFAULT '',mini_user_id bigint NOT NULL DEFAULT 0,
 customer_id bigint NOT NULL DEFAULT 0,app_id text NOT NULL DEFAULT '',openid text NOT NULL DEFAULT '',
 bound_at timestamptz,created_at timestamptz NOT NULL DEFAULT now(),expires_at timestamptz NOT NULL);
 CREATE TABLE IF NOT EXISTS %[1]s.page_oauth_states(
 state_hash text PRIMARY KEY,browser_hash text NOT NULL,entry_key text NOT NULL,expires_at timestamptz NOT NULL);
 CREATE TABLE IF NOT EXISTS %[1]s.page_login_attempts(
 identity_hash text PRIMARY KEY,bucket timestamptz NOT NULL DEFAULT now(),attempts integer NOT NULL DEFAULT 0);
 `, schema))
	return err
}
