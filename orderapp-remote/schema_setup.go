package main

import (
	"context"

	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ensureAppSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	return postgresinfra.EnsureSchema(ctx, []postgresinfra.SchemaStep{
		{Name: "audit tables", Run: func(ctx context.Context) error { return ensureAuditTables(ctx, pool, schema) }},
		{Name: "requirements tables", Run: func(ctx context.Context) error { return ensureReqTables(ctx, pool, schema) }},
		{Name: "requirements seed", Run: func(ctx context.Context) error { return seedReqWorkflowA(ctx, pool, schema) }},
		{Name: "finished inventory", Run: func(ctx context.Context) error { return ensureFinishedInventoryTable(ctx, pool, schema) }},
		{Name: "finished allocation log", Run: func(ctx context.Context) error { return ensureFinishedAllocationLogTable(ctx, pool, schema) }},
		{Name: "materials", Run: func(ctx context.Context) error { return ensureMaterialTables(ctx, pool, schema) }},
		{Name: "bom", Run: func(ctx context.Context) error { return ensureBomTables(ctx, pool, schema) }},
		{Name: "bag spec mapping", Run: func(ctx context.Context) error { return ensureBagSpecMappingTable(ctx, pool, schema) }},
		{Name: "produce batch", Run: func(ctx context.Context) error { return ensureProduceBatchTables(ctx, pool, schema) }},
		{Name: "company staff", Run: func(ctx context.Context) error { return ensureCompanyStaffTables(ctx, pool, schema) }},
		{Name: "machine capacity", Run: func(ctx context.Context) error { return ensureMachineCapacityTable(ctx, pool, schema) }},
		{Name: "mobile auth", Run: func(ctx context.Context) error { return ensureMobileAuthTables(ctx, pool, schema) }},
		{Name: "production run", Run: func(ctx context.Context) error { return ensureProductionRunTable(ctx, pool, schema) }},
		{Name: "order process statuses", Run: func(ctx context.Context) error { return ensureOrderProcessStatuses(ctx, pool, schema) }},
		{Name: "sender settings", Run: func(ctx context.Context) error { return ensureSenderSettingsTable(ctx, pool, schema) }},
		{Name: "outsource fee columns", Run: func(ctx context.Context) error { return ensureOutsourceFeeColumns(ctx, pool, schema) }},
		{Name: "outsource template", Run: func(ctx context.Context) error { return ensureOutsourceTemplateTables(ctx, pool, schema) }},
	})
}
