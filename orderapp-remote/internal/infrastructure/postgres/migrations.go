package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

type Migration struct {
	Version  string
	Checksum string
	Up       func(context.Context) error
}

type MigrationExecer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

func MigrationLedgerDDL(schema string) string {
	schema = strings.TrimSpace(schema)
	if schema == "" {
		schema = "public"
	}
	return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.schema_migrations (
	version TEXT PRIMARY KEY,
	checksum TEXT NOT NULL,
	applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`, schema)
}

func EnsureMigrationLedger(ctx context.Context, exec MigrationExecer, schema string) error {
	if exec == nil {
		return fmt.Errorf("migration ledger exec is nil")
	}
	_, err := exec.Exec(ctx, MigrationLedgerDDL(schema))
	return err
}

func ValidateMigrations(migrations []Migration) error {
	seen := map[string]struct{}{}
	var previous string
	for idx, migration := range migrations {
		version := strings.TrimSpace(migration.Version)
		if version == "" {
			return fmt.Errorf("migration %d version is empty", idx)
		}
		if strings.TrimSpace(migration.Checksum) == "" {
			return fmt.Errorf("migration %s checksum is empty", version)
		}
		if previous != "" && version <= previous {
			return fmt.Errorf("migration %s must be sorted after %s", version, previous)
		}
		if _, ok := seen[version]; ok {
			return fmt.Errorf("migration %s is duplicated", version)
		}
		seen[version] = struct{}{}
		previous = version
	}
	return nil
}
