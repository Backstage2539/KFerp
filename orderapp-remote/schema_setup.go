package main

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ensureAppSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	if err := ensureReqTables(context.Background(), pool, schema); err != nil {
		return err
	}
	if err := seedReqWorkflowA(context.Background(), pool, schema); err != nil {
		return err
	}

	if err := ensureFinishedInventoryTable(context.Background(), pool, schema); err != nil {
		return err
	}
	if err := ensureFinishedAllocationLogTable(context.Background(), pool, schema); err != nil {
		return err
	}
	if err := ensureMaterialTables(context.Background(), pool, schema); err != nil {
		return err
	}
	if err := ensureBomTables(context.Background(), pool, schema); err != nil {
		return err
	}
	if err := ensureBagSpecMappingTable(context.Background(), pool, schema); err != nil {
		return err
	}
	if err := ensureProduceBatchTables(context.Background(), pool, schema); err != nil {
		return err
	}
	if err := ensureCompanyStaffTables(context.Background(), pool, schema); err != nil {
		return err
	}
	if err := ensureMachineCapacityTable(context.Background(), pool, schema); err != nil {
		return err
	}
	if err := ensureMobileAuthTables(context.Background(), pool, schema); err != nil {
		return err
	}
	if err := ensureProductionRunTable(context.Background(), pool, schema); err != nil {
		return err
	}
	if err := ensureOrderProcessStatuses(context.Background(), pool, schema); err != nil {
		return err
	}
	if err := ensureSenderSettingsTable(context.Background(), pool, schema); err != nil {
		return err
	}
	if err := ensureOutsourceFeeColumns(context.Background(), pool, schema); err != nil {
		return err
	}
	if err := ensureOutsourceTemplateTables(context.Background(), pool, schema); err != nil {
		return err
	}

	return nil
}
