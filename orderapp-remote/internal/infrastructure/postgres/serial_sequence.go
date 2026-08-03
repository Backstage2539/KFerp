package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ownedIDSequence struct {
	tableSchema    string
	tableName      string
	sequenceSchema string
	sequenceName   string
}

// SyncSerialIDSequences advances serial/identity sequences owned by id columns
// when imported rows have moved past the sequence. It never lowers a sequence.
func SyncSerialIDSequences(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	rows, err := pool.Query(ctx, `
SELECT table_ns.nspname, table_class.relname, sequence_ns.nspname, sequence_class.relname
FROM pg_class AS sequence_class
JOIN pg_namespace AS sequence_ns ON sequence_ns.oid = sequence_class.relnamespace
JOIN pg_depend AS dependency
  ON dependency.classid = 'pg_class'::regclass
 AND dependency.objid = sequence_class.oid
 AND dependency.deptype IN ('a', 'i')
JOIN pg_class AS table_class ON table_class.oid = dependency.refobjid
JOIN pg_namespace AS table_ns ON table_ns.oid = table_class.relnamespace
JOIN pg_attribute AS column_attribute
  ON column_attribute.attrelid = table_class.oid
 AND column_attribute.attnum = dependency.refobjsubid
WHERE sequence_class.relkind = 'S'
  AND table_ns.nspname = $1
  AND column_attribute.attname = 'id'
ORDER BY table_class.relname`, schema)
	if err != nil {
		return fmt.Errorf("list owned id sequences: %w", err)
	}

	sequences := make([]ownedIDSequence, 0)
	for rows.Next() {
		var sequence ownedIDSequence
		if err := rows.Scan(&sequence.tableSchema, &sequence.tableName, &sequence.sequenceSchema, &sequence.sequenceName); err != nil {
			rows.Close()
			return fmt.Errorf("scan owned id sequence: %w", err)
		}
		sequences = append(sequences, sequence)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate owned id sequences: %w", err)
	}
	rows.Close()

	for _, sequence := range sequences {
		if err := syncOwnedIDSequence(ctx, pool, sequence); err != nil {
			return err
		}
	}

	return nil
}

func syncOwnedIDSequence(ctx context.Context, pool *pgxpool.Pool, sequence ownedIDSequence) error {
	tableIdentifier := pgx.Identifier{sequence.tableSchema, sequence.tableName}.Sanitize()
	sequenceIdentifier := pgx.Identifier{sequence.sequenceSchema, sequence.sequenceName}.Sanitize()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin sequence synchronization for %s: %w", tableIdentifier, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Block inserts briefly so MAX(id), the sequence check, and setval describe
	// one consistent state even when another application instance is running.
	if _, err := tx.Exec(ctx, "LOCK TABLE "+tableIdentifier+" IN ACCESS EXCLUSIVE MODE"); err != nil {
		return fmt.Errorf("lock %s for sequence synchronization: %w", tableIdentifier, err)
	}

	var maxID int64
	if err := tx.QueryRow(ctx, "SELECT COALESCE(MAX(id), 0) FROM "+tableIdentifier).Scan(&maxID); err != nil {
		return fmt.Errorf("read maximum id for %s: %w", tableIdentifier, err)
	}

	var lastValue int64
	if err := tx.QueryRow(ctx, "SELECT last_value FROM "+sequenceIdentifier).Scan(&lastValue); err != nil {
		return fmt.Errorf("read last value for %s: %w", sequenceIdentifier, err)
	}

	nextValue, repair := serialSequenceRepairValue(maxID, lastValue)
	if repair {
		if _, err := tx.Exec(ctx, "SELECT setval($1::regclass, $2, false)", sequenceIdentifier, nextValue); err != nil {
			return fmt.Errorf("advance %s for %s: %w", sequenceIdentifier, tableIdentifier, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit sequence synchronization for %s: %w", tableIdentifier, err)
	}
	return nil
}

func serialSequenceRepairValue(maxID, lastValue int64) (int64, bool) {
	if maxID <= 0 || lastValue > maxID {
		return 0, false
	}
	return maxID + 1, true
}
