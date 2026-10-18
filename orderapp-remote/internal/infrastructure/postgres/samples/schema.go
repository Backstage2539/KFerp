package samples

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`
 CREATE TABLE IF NOT EXISTS %[1]s.sample_requests(
 id BIGSERIAL PRIMARY KEY,mini_user_id BIGINT NOT NULL REFERENCES %[1]s.mini_users(id),request_key TEXT NOT NULL,request_hash TEXT NOT NULL,
 nickname TEXT NOT NULL,verified_phone TEXT NOT NULL,entry_key TEXT NOT NULL,publication_id BIGINT NOT NULL,publication_name TEXT NOT NULL,publication_version TEXT NOT NULL,
 receiver JSONB NOT NULL,note TEXT NOT NULL DEFAULT '',created_at TIMESTAMPTZ NOT NULL DEFAULT now(),UNIQUE(mini_user_id,request_key));
 CREATE TABLE IF NOT EXISTS %[1]s.sample_request_items(
 request_id BIGINT NOT NULL REFERENCES %[1]s.sample_requests(id),item_key TEXT NOT NULL,product_id BIGINT NOT NULL,code TEXT NOT NULL,name TEXT NOT NULL,position INT NOT NULL,
 PRIMARY KEY(request_id,item_key));
 CREATE TABLE IF NOT EXISTS %[1]s.sample_work_orders(
 id BIGSERIAL PRIMARY KEY,request_id BIGINT NOT NULL UNIQUE REFERENCES %[1]s.sample_requests(id),assignee_id BIGINT NOT NULL REFERENCES %[1]s.company_employees(id),
 status TEXT NOT NULL DEFAULT 'pending_contact' CHECK(status IN('pending_contact','ordered','closed')),
 followup_status TEXT NOT NULL DEFAULT 'pending' CHECK(followup_status IN('pending','following','won','abandoned')),
 revision BIGINT NOT NULL DEFAULT 1,customer_id BIGINT,order_id BIGINT UNIQUE REFERENCES %[1]s.orders(id),assigned_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 next_reminder_at TIMESTAMPTZ,last_reminded_at TIMESTAMPTZ,confirmed_json JSONB,updated_at TIMESTAMPTZ NOT NULL DEFAULT now());
 CREATE INDEX IF NOT EXISTS sample_work_orders_reminder_idx ON %[1]s.sample_work_orders(next_reminder_at) WHERE next_reminder_at IS NOT NULL;
 CREATE INDEX IF NOT EXISTS sample_work_orders_assignee_idx ON %[1]s.sample_work_orders(assignee_id,updated_at DESC);
 CREATE TABLE IF NOT EXISTS %[1]s.sample_activities(
 id BIGSERIAL PRIMARY KEY,request_id BIGINT NOT NULL REFERENCES %[1]s.sample_requests(id),kind TEXT NOT NULL,status TEXT NOT NULL DEFAULT '',note TEXT NOT NULL,actor TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now());
 CREATE INDEX IF NOT EXISTS sample_activities_request_idx ON %[1]s.sample_activities(request_id,id);
 `, schema))
	return err
}
