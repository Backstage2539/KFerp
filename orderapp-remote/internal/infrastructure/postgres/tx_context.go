package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type transactionContextKey struct{}
type businessProvenanceContextKey struct{}
type suppressMaterialDefaultContextKey struct{}

type BusinessProvenance struct {
	RunID  int64
	NodeID string
}

// WithTransaction lets a composed business operation reuse one PostgreSQL
// transaction across existing domain repositories without changing their
// application-level validation or audit behavior.
func WithTransaction(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, transactionContextKey{}, tx)
}

func TransactionFromContext(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(transactionContextKey{}).(pgx.Tx)
	return tx, ok && tx != nil
}

// WithBusinessProvenance attaches the product-creator execution identity to
// existing domain writes so their regular audit rows can be traced back to
// the template run and workflow node that produced them.
func WithBusinessProvenance(ctx context.Context, runID int64, nodeID string) context.Context {
	return context.WithValue(ctx, businessProvenanceContextKey{}, BusinessProvenance{RunID: runID, NodeID: nodeID})
}

func BusinessProvenanceFromContext(ctx context.Context) (BusinessProvenance, bool) {
	value, ok := ctx.Value(businessProvenanceContextKey{}).(BusinessProvenance)
	return value, ok && value.RunID > 0 && value.NodeID != ""
}

// WithoutAutomaticMaterialBomDefault lets a workflow explicitly publish a
// material BOM without changing a shared default. Normal BOM publication
// keeps its existing first-publication default behavior.
func WithoutAutomaticMaterialBomDefault(ctx context.Context) context.Context {
	return context.WithValue(ctx, suppressMaterialDefaultContextKey{}, true)
}

func AutomaticMaterialBomDefaultAllowed(ctx context.Context) bool {
	suppress, _ := ctx.Value(suppressMaterialDefaultContextKey{}).(bool)
	return !suppress
}
