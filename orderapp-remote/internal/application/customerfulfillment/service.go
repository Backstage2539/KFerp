package customerfulfillment

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"
)

type ImportType string

const (
	ImportTypeProcessingWorkbook ImportType = "processing_workbook"
	ImportTypeDirectShipWorkbook ImportType = "direct_ship_workbook"
	ImportTypeSettlementWorkbook ImportType = "settlement_workbook"
)

type ParsedWorkbook struct {
	ImportType ImportType    `json:"import_type"`
	Rows       []ParsedRow   `json:"rows"`
	Summary    ImportSummary `json:"summary"`
}

type ParsedRow struct {
	SheetName   string         `json:"sheet_name"`
	RowNo       int            `json:"row_no"`
	RowType     string         `json:"row_type"`
	ExternalKey string         `json:"external_key"`
	Payload     map[string]any `json:"payload"`
	Error       string         `json:"error,omitempty"`
}

type ImportSummary struct {
	TotalRows         int `json:"total_rows"`
	ValidRows         int `json:"valid_rows"`
	InvalidRows       int `json:"invalid_rows"`
	RawBeanReceipts   int `json:"raw_bean_receipts"`
	RawBeanIssues     int `json:"raw_bean_issues"`
	RawBeanBalances   int `json:"raw_bean_balances"`
	CustomerSKUs      int `json:"customer_skus"`
	PackagingBalances int `json:"packaging_balances"`
	ProcessingOrders  int `json:"processing_orders"`
	PackagingJobs     int `json:"packaging_jobs"`
	ConversionJobs    int `json:"conversion_jobs"`
	DirectShipOrders  int `json:"direct_ship_orders"`
	DirectShipItems   int `json:"direct_ship_items"`
	FeeItems          int `json:"fee_items"`
	SettlementBatches int `json:"settlement_batches"`
}

type ParseImportCommand struct {
	CustomerID     int64
	ImportType     ImportType
	SourceFilename string
	Reader         io.Reader
	CreatedBy      string
}

type StoreParsedImportCommand struct {
	CustomerID     int64
	ImportType     ImportType
	SourceFilename string
	SourceSHA256   string
	Parsed         ParsedWorkbook
	CreatedBy      string
}

type ApplyImportCommand struct {
	BatchID int64
	Actor   string
}

type CreateSettlementCommand struct {
	CustomerID int64
	PeriodFrom string
	PeriodTo   string
	CreatedBy  string
}

type OverviewQuery struct {
	CustomerID int64
}

type ListImportsQuery struct {
	CustomerID int64
	Limit      int
	Offset     int
}

type ImportBatch struct {
	ID             int64         `json:"id"`
	CustomerID     int64         `json:"customer_id"`
	ImportType     ImportType    `json:"import_type"`
	SourceFilename string        `json:"source_filename"`
	SourceSHA256   string        `json:"source_sha256"`
	Status         string        `json:"status"`
	Summary        ImportSummary `json:"summary"`
	CreatedBy      string        `json:"created_by"`
	CreatedAt      string        `json:"created_at"`
	AppliedAt      string        `json:"applied_at,omitempty"`
}

type ApplyResult struct {
	BatchID          int64 `json:"batch_id"`
	AppliedRows      int   `json:"applied_rows"`
	SkippedRows      int   `json:"skipped_rows"`
	ProcessingOrders int   `json:"processing_orders"`
	DirectShipOrders int   `json:"direct_ship_orders"`
	FeeItems         int   `json:"fee_items"`
}

type SettlementResult struct {
	BatchID          int64  `json:"batch_id"`
	CustomerID       int64  `json:"customer_id"`
	PeriodFrom       string `json:"period_from"`
	PeriodTo         string `json:"period_to"`
	FeeItems         int    `json:"fee_items"`
	TotalAmountCents int64  `json:"total_amount_cents"`
}

type Overview struct {
	CustomerID       int64                    `json:"customer_id"`
	CustomerName     string                   `json:"customer_name"`
	Imports          []ImportBatch            `json:"imports,omitempty"`
	CustodyBalances  []CustodyBalance         `json:"custody_balances,omitempty"`
	ProcessingOrders []ProcessingOrderSummary `json:"processing_orders,omitempty"`
	DirectShipOrders []DirectShipOrderSummary `json:"direct_ship_orders,omitempty"`
	Fees             []FeeItemSummary         `json:"fees,omitempty"`
	Settlements      []SettlementSummary      `json:"settlements,omitempty"`
}

type CustodyBalance struct {
	ItemType      string `json:"item_type"`
	ItemName      string `json:"item_name"`
	Spec          string `json:"spec,omitempty"`
	QuantityG     int64  `json:"quantity_g"`
	QuantityUnits int64  `json:"quantity_units"`
}

type ProcessingOrderSummary struct {
	WorkOrderNo string `json:"work_order_no"`
	ProductName string `json:"product_name"`
	Status      string `json:"status"`
	QuantityG   int64  `json:"quantity_g"`
	Units       int64  `json:"units"`
}

type DirectShipOrderSummary struct {
	OrderNo         string `json:"order_no"`
	OrderDate       string `json:"order_date"`
	ReceiverAddress string `json:"receiver_address"`
	Status          string `json:"status"`
	ItemCount       int    `json:"item_count"`
}

type FeeItemSummary struct {
	FeeType     string `json:"fee_type"`
	FeeName     string `json:"fee_name"`
	AmountCents int64  `json:"amount_cents"`
	Source      string `json:"source,omitempty"`
}

type SettlementSummary struct {
	BatchID          int64  `json:"batch_id"`
	PeriodFrom       string `json:"period_from"`
	PeriodTo         string `json:"period_to"`
	Status           string `json:"status"`
	TotalAmountCents int64  `json:"total_amount_cents"`
}

type Repository interface {
	StoreParsedImport(context.Context, StoreParsedImportCommand) (ImportBatch, error)
	ApplyImport(context.Context, ApplyImportCommand) (ApplyResult, error)
	CreateSettlement(context.Context, CreateSettlementCommand) (SettlementResult, error)
	Overview(context.Context, OverviewQuery) (Overview, error)
	ListImports(context.Context, ListImportsQuery) ([]ImportBatch, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ParseImport(ctx context.Context, cmd ParseImportCommand) (ImportBatch, error) {
	if cmd.CustomerID <= 0 {
		return ImportBatch{}, fmt.Errorf("customer required")
	}
	if !validImportType(cmd.ImportType) {
		return ImportBatch{}, fmt.Errorf("import type invalid")
	}
	if cmd.Reader == nil {
		return ImportBatch{}, fmt.Errorf("file required")
	}
	const maxImportBytes = 20 << 20
	data, err := io.ReadAll(io.LimitReader(cmd.Reader, maxImportBytes+1))
	if err != nil {
		return ImportBatch{}, err
	}
	if len(data) == 0 {
		return ImportBatch{}, fmt.Errorf("file required")
	}
	if len(data) > maxImportBytes {
		return ImportBatch{}, fmt.Errorf("file too large")
	}
	sum := sha256.Sum256(data)
	parsed, err := ParseWorkbook(cmd.ImportType, bytes.NewReader(data))
	if err != nil {
		return ImportBatch{}, err
	}
	sourceFilename := strings.TrimSpace(cmd.SourceFilename)
	if sourceFilename == "" {
		sourceFilename = "upload.xlsx"
	}
	return s.repo.StoreParsedImport(ctx, StoreParsedImportCommand{
		CustomerID:     cmd.CustomerID,
		ImportType:     cmd.ImportType,
		SourceFilename: sourceFilename,
		SourceSHA256:   hex.EncodeToString(sum[:]),
		Parsed:         parsed,
		CreatedBy:      strings.TrimSpace(cmd.CreatedBy),
	})
}

func (s *Service) ApplyImport(ctx context.Context, cmd ApplyImportCommand) (ApplyResult, error) {
	if cmd.BatchID <= 0 {
		return ApplyResult{}, fmt.Errorf("batch required")
	}
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	return s.repo.ApplyImport(ctx, cmd)
}

func (s *Service) CreateSettlement(ctx context.Context, cmd CreateSettlementCommand) (SettlementResult, error) {
	if cmd.CustomerID <= 0 {
		return SettlementResult{}, fmt.Errorf("customer required")
	}
	from, err := time.Parse("2006-01-02", strings.TrimSpace(cmd.PeriodFrom))
	if err != nil {
		return SettlementResult{}, fmt.Errorf("period from invalid")
	}
	to, err := time.Parse("2006-01-02", strings.TrimSpace(cmd.PeriodTo))
	if err != nil {
		return SettlementResult{}, fmt.Errorf("period to invalid")
	}
	if to.Before(from) {
		return SettlementResult{}, fmt.Errorf("period invalid")
	}
	cmd.PeriodFrom = from.Format("2006-01-02")
	cmd.PeriodTo = to.Format("2006-01-02")
	cmd.CreatedBy = strings.TrimSpace(cmd.CreatedBy)
	return s.repo.CreateSettlement(ctx, cmd)
}

func (s *Service) Overview(ctx context.Context, query OverviewQuery) (Overview, error) {
	if query.CustomerID <= 0 {
		return Overview{}, fmt.Errorf("customer required")
	}
	return s.repo.Overview(ctx, query)
}

func (s *Service) ListImports(ctx context.Context, query ListImportsQuery) ([]ImportBatch, error) {
	if query.CustomerID <= 0 {
		return nil, fmt.Errorf("customer required")
	}
	return s.repo.ListImports(ctx, query)
}

func validImportType(importType ImportType) bool {
	switch importType {
	case ImportTypeProcessingWorkbook, ImportTypeDirectShipWorkbook, ImportTypeSettlementWorkbook:
		return true
	default:
		return false
	}
}
