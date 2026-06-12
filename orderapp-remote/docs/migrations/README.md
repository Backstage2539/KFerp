# KFerp Database Migrations

KFerp still uses `EnsureSchema` as the idempotent bootstrap path for existing tables, columns, and indexes. The `schema_migrations` ledger is the architecture boundary for future ordered migrations that need an auditable version, checksum, and applied timestamp.

Rules:

- Keep additive, idempotent bootstrap guards in `EnsureSchema` until the owning module is migrated to explicit versions.
- Put ordered migration metadata in the ledger path before introducing data-transforming or destructive changes.
- Do not add destructive migrations without a separate rollback/data-repair plan and operator approval.
