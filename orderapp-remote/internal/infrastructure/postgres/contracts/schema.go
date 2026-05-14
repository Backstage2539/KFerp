package contracts

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	stmts := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.contract_documents (
			id BIGSERIAL PRIMARY KEY,
			title TEXT NOT NULL DEFAULT '',
			note TEXT NOT NULL DEFAULT '',
			source_filename TEXT NOT NULL DEFAULT '',
			source_content_type TEXT NOT NULL DEFAULT '',
			source_kind TEXT NOT NULL DEFAULT '',
			source_object_key TEXT NOT NULL DEFAULT '',
			source_bytes BIGINT NOT NULL DEFAULT 0,
			source_sha256 TEXT NOT NULL DEFAULT '',
			pdf_object_key TEXT NOT NULL DEFAULT '',
			pdf_bytes BIGINT NOT NULL DEFAULT 0,
			pdf_sha256 TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			created_by TEXT NOT NULL DEFAULT '',
			deleted_at TIMESTAMPTZ,
			deleted_by TEXT NOT NULL DEFAULT ''
		)`, schema),
		fmt.Sprintf(`ALTER TABLE %s.contract_documents ADD COLUMN IF NOT EXISTS note TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.contract_documents ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ`, schema),
		fmt.Sprintf(`ALTER TABLE %s.contract_documents ADD COLUMN IF NOT EXISTS deleted_by TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.contract_stamped_versions (
			id BIGSERIAL PRIMARY KEY,
			contract_id BIGINT NOT NULL REFERENCES %s.contract_documents(id) ON DELETE CASCADE,
			version_no INTEGER NOT NULL,
			seal_asset_id BIGINT NOT NULL DEFAULT 0,
			placements_json JSONB NOT NULL DEFAULT '[]'::jsonb,
			object_key TEXT NOT NULL DEFAULT '',
			bytes BIGINT NOT NULL DEFAULT 0,
			sha256 TEXT NOT NULL DEFAULT '',
			is_latest BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			created_by TEXT NOT NULL DEFAULT '',
			UNIQUE(contract_id, version_no)
		)`, schema, schema),
		fmt.Sprintf(`CREATE UNIQUE INDEX IF NOT EXISTS idx_%s_contract_stamped_latest ON %s.contract_stamped_versions(contract_id) WHERE is_latest`, schema, schema),
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}
