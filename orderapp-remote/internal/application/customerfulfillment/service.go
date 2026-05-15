package customerfulfillment

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ImportType string

const (
	ImportTypeProcessingWorkbook ImportType = "processing_workbook"
	ImportTypeDirectShipWorkbook ImportType = "direct_ship_workbook"
	ImportTypeSettlementWorkbook ImportType = "settlement_workbook"
)

var externalUserPhoneRe = regexp.MustCompile(`^1\d{10}$`)

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

var ErrCustomerERPBindingNotFound = errors.New("customer erp binding not found")

type CustomerERPContext struct {
	EmployeeID    int64  `json:"employee_id"`
	CustomerID    int64  `json:"customer_id"`
	CustomerName  string `json:"customer_name"`
	BindingRole   string `json:"binding_role"`
	BindingStatus string `json:"binding_status"`
}

type CustomerPortalOverview struct {
	CustomerID       int64                    `json:"customer_id"`
	CustomerName     string                   `json:"customer_name"`
	Capabilities     []string                 `json:"capabilities,omitempty"`
	CustodyBalances  []CustodyBalance         `json:"custody_balances,omitempty"`
	FinishedGoods    []FinishedGoodsBalance   `json:"finished_goods,omitempty"`
	ProcessingOrders []ProcessingOrderSummary `json:"processing_orders,omitempty"`
	DirectShipOrders []DirectShipOrderSummary `json:"direct_ship_orders,omitempty"`
	Fees             []FeeItemSummary         `json:"fees,omitempty"`
	Settlements      []SettlementSummary      `json:"settlements,omitempty"`
}

type SubmitCustomerProcessingWorkOrderCommand struct {
	EmployeeID         int64
	CustomerID         int64
	ProductID          int64
	ProductName        string
	RawBeanItemID      int64
	RawBeanName        string
	InputQuantityG     int64
	PlannedOutputUnits int64
	ExpectedDate       string
	Note               string
}

type SubmitCustomerDirectShipOrderCommand struct {
	EmployeeID      int64
	CustomerID      int64
	ReceiverName    string
	ReceiverPhone   string
	ReceiverAddress string
	ReceiverCompany string
	ShippingAmount  float64
	ProductID       int64
	ProductName     string
	Spec            string
	QuantityUnits   int64
	Items           []SubmitCustomerDirectShipOrderItem
	Note            string
}

type SubmitCustomerDirectShipOrderItem struct {
	ProductID     int64  `json:"product_id"`
	ProductName   string `json:"product_name,omitempty"`
	Spec          string `json:"spec,omitempty"`
	SpecG         int64  `json:"spec_g,omitempty"`
	QuantityUnits int64  `json:"quantity_units"`
	Note          string `json:"note,omitempty"`
}

type AdjustCustodyInventoryCommand struct {
	CustomerID         int64
	ItemType           string
	ItemName           string
	Spec               string
	QuantityGDelta     int64
	QuantityUnitsDelta int64
	Note               string
	Actor              string
}

type UpsertCustomerERPBindingCommand struct {
	CustomerID int64
	EmployeeID int64
	Role       string
	Status     string
	Actor      string
}

type CustomerERPBinding struct {
	CustomerID   int64  `json:"customer_id"`
	EmployeeID   int64  `json:"employee_id"`
	EmployeeName string `json:"employee_name"`
	Role         string `json:"role"`
	Status       string `json:"status"`
	UpdatedBy    string `json:"updated_by,omitempty"`
	UpdatedAt    string `json:"updated_at,omitempty"`
}

type CreateExternalUserCommand struct {
	CustomerID int64
	Name       string
	Phone      string
	Password   string
	Actor      string
}

type ResetExternalUserPasswordCommand struct {
	CustomerID int64
	EmployeeID int64
	Password   string
	Actor      string
}

type SetExternalUserLoginEnabledCommand struct {
	CustomerID   int64
	EmployeeID   int64
	LoginEnabled bool
	Actor        string
}

type CustomerExternalUser struct {
	CustomerID    int64  `json:"customer_id"`
	EmployeeID    int64  `json:"employee_id"`
	Name          string `json:"name"`
	Phone         string `json:"phone"`
	LoginEnabled  bool   `json:"login_enabled"`
	HasPassword   bool   `json:"has_password"`
	BindingStatus string `json:"binding_status"`
	UpdatedAt     string `json:"updated_at,omitempty"`
}

type CustomerFulfillmentOptions struct {
	CustomerSKUs []CustomerSKUOption `json:"customer_skus,omitempty"`
	CustodyItems []CustodyItemOption `json:"custody_items,omitempty"`
	Employees    []EmployeeOption    `json:"employees,omitempty"`
	Recipients   []RecipientOption   `json:"recipients,omitempty"`
}

type CustomerSKUOption struct {
	ProductID     int64  `json:"product_id"`
	BaseProductID int64  `json:"base_product_id,omitempty"`
	SKUCode       string `json:"sku_code,omitempty"`
	ProductName   string `json:"product_name"`
	Spec          string `json:"spec,omitempty"`
	RoastDegree   string `json:"roast_degree,omitempty"`
	DefaultPrice  float64 `json:"default_price,omitempty"`
	Tiers         []CustomerSKUPriceTier `json:"tiers,omitempty"`
	Source        string `json:"source,omitempty"`
}

type CustomerSKUPriceTier struct {
	ID        int64    `json:"id"`
	SpecG     int64    `json:"spec_g"`
	Min       float64  `json:"min"`
	Max       *float64 `json:"max,omitempty"`
	UnitPrice float64  `json:"unit_price"`
}

type CustodyItemOption struct {
	ItemID        int64  `json:"item_id"`
	ItemType      string `json:"item_type"`
	ItemName      string `json:"item_name"`
	Spec          string `json:"spec,omitempty"`
	QuantityG     int64  `json:"quantity_g"`
	QuantityUnits int64  `json:"quantity_units"`
}

type EmployeeOption struct {
	ID         int64  `json:"employee_id"`
	Name       string `json:"name"`
	Phone      string `json:"phone,omitempty"`
	Department string `json:"department,omitempty"`
	Active     bool   `json:"active"`
}

type RecipientOption struct {
	ReceiverName    string `json:"receiver_name,omitempty"`
	ReceiverPhone   string `json:"receiver_phone,omitempty"`
	ReceiverCompany string `json:"receiver_company,omitempty"`
	ReceiverAddress string `json:"receiver_address"`
	LastOrderNo     string `json:"last_order_no,omitempty"`
	LastUsedAt      string `json:"last_used_at,omitempty"`
}

type OverviewQuery struct {
	CustomerID int64
}

type ListImportsQuery struct {
	CustomerID int64
	Limit      int
	Offset     int
}

type ListImportRowsQuery struct {
	BatchID int64
	Status  string
	Limit   int
	Offset  int
}

type ImportPreviewQuery struct {
	BatchID int64
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

type ImportRow struct {
	ID          int64  `json:"id"`
	BatchID     int64  `json:"batch_id"`
	SheetName   string `json:"sheet_name"`
	RowNo       int    `json:"row_no"`
	RowType     string `json:"row_type"`
	ExternalKey string `json:"external_key,omitempty"`
	Status      string `json:"status"`
	Error       string `json:"error,omitempty"`
}

type ImportPreviewEffect struct {
	Label string `json:"label"`
	Value int    `json:"value"`
}

type ImportPreview struct {
	Batch   ImportBatch           `json:"batch"`
	Effects []ImportPreviewEffect `json:"effects"`
	Warning string                `json:"warning"`
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
	Capabilities     []string                 `json:"capabilities,omitempty"`
	Imports          []ImportBatch            `json:"imports,omitempty"`
	CustodyBalances  []CustodyBalance         `json:"custody_balances,omitempty"`
	FinishedGoods    []FinishedGoodsBalance   `json:"finished_goods,omitempty"`
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

type FinishedGoodsBalance struct {
	ProductID     int64  `json:"product_id"`
	ProductName   string `json:"product_name"`
	SpecG         int64  `json:"spec_g"`
	Warehouse     string `json:"warehouse"`
	QuantityG     int64  `json:"quantity_g"`
	QuantityUnits int64  `json:"quantity_units"`
	Status        string `json:"status"`
}

type ProcessingOrderSummary struct {
	WorkOrderNo string `json:"work_order_no"`
	ProductName string `json:"product_name"`
	Status      string `json:"status"`
	QuantityG   int64  `json:"quantity_g"`
	Units       int64  `json:"units"`
}

type DirectShipOrderSummary struct {
	OrderID         int64  `json:"order_id,omitempty"`
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
	ImportBatch(context.Context, int64) (ImportBatch, error)
	ListImportRows(context.Context, ListImportRowsQuery) ([]ImportRow, error)
	ApplyImport(context.Context, ApplyImportCommand) (ApplyResult, error)
	CustomerPortalContext(context.Context, int64) (CustomerERPContext, error)
	CustomerPortalOverview(context.Context, int64) (CustomerPortalOverview, error)
	SubmitCustomerProcessingWorkOrder(context.Context, SubmitCustomerProcessingWorkOrderCommand) (ProcessingOrderSummary, error)
	SubmitCustomerDirectShipOrder(context.Context, SubmitCustomerDirectShipOrderCommand) (DirectShipOrderSummary, error)
	AdjustCustodyInventory(context.Context, AdjustCustodyInventoryCommand) (CustodyBalance, error)
	UpsertCustomerERPBinding(context.Context, UpsertCustomerERPBindingCommand) (CustomerERPBinding, error)
	ListCustomerERPBindings(context.Context, int64) ([]CustomerERPBinding, error)
	CustomerERPWorkbenchAvailable(context.Context, int64) (bool, error)
	CreateExternalUser(context.Context, CreateExternalUserCommand) (CustomerExternalUser, error)
	ListExternalUsers(context.Context, int64) ([]CustomerExternalUser, error)
	ResetExternalUserPassword(context.Context, ResetExternalUserPasswordCommand) (CustomerExternalUser, error)
	SetExternalUserLoginEnabled(context.Context, SetExternalUserLoginEnabledCommand) (CustomerExternalUser, error)
	CustomerFulfillmentOptions(context.Context, int64) (CustomerFulfillmentOptions, error)
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

func (s *Service) CustomerPortalOverview(ctx context.Context, employeeID int64) (CustomerPortalOverview, error) {
	if employeeID <= 0 {
		return CustomerPortalOverview{}, fmt.Errorf("employee required")
	}
	return s.repo.CustomerPortalOverview(ctx, employeeID)
}

func (s *Service) CustomerPortalOptions(ctx context.Context, employeeID int64) (CustomerFulfillmentOptions, error) {
	if employeeID <= 0 {
		return CustomerFulfillmentOptions{}, fmt.Errorf("employee required")
	}
	current, err := s.repo.CustomerPortalContext(ctx, employeeID)
	if err != nil {
		return CustomerFulfillmentOptions{}, err
	}
	return s.repo.CustomerFulfillmentOptions(ctx, current.CustomerID)
}

func (s *Service) SubmitCustomerProcessingWorkOrder(ctx context.Context, cmd SubmitCustomerProcessingWorkOrderCommand) (ProcessingOrderSummary, error) {
	if cmd.EmployeeID <= 0 && cmd.CustomerID <= 0 {
		return ProcessingOrderSummary{}, fmt.Errorf("employee or customer required")
	}
	if cmd.CustomerID < 0 {
		return ProcessingOrderSummary{}, fmt.Errorf("customer invalid")
	}
	cmd.ProductName = strings.Join(strings.Fields(strings.TrimSpace(cmd.ProductName)), " ")
	cmd.RawBeanName = strings.Join(strings.Fields(strings.TrimSpace(cmd.RawBeanName)), " ")
	cmd.ExpectedDate = normalizeOptionalDate(cmd.ExpectedDate)
	cmd.Note = strings.TrimSpace(cmd.Note)
	if cmd.ProductName == "" && cmd.ProductID <= 0 {
		return ProcessingOrderSummary{}, fmt.Errorf("product required")
	}
	if cmd.InputQuantityG <= 0 {
		return ProcessingOrderSummary{}, fmt.Errorf("input quantity required")
	}
	if cmd.PlannedOutputUnits <= 0 {
		return ProcessingOrderSummary{}, fmt.Errorf("planned output required")
	}
	return s.repo.SubmitCustomerProcessingWorkOrder(ctx, cmd)
}

func (s *Service) SubmitCustomerDirectShipOrder(ctx context.Context, cmd SubmitCustomerDirectShipOrderCommand) (DirectShipOrderSummary, error) {
	if cmd.EmployeeID <= 0 && cmd.CustomerID <= 0 {
		return DirectShipOrderSummary{}, fmt.Errorf("employee or customer required")
	}
	if cmd.CustomerID < 0 {
		return DirectShipOrderSummary{}, fmt.Errorf("customer invalid")
	}
	cmd.ReceiverName = strings.Join(strings.Fields(strings.TrimSpace(cmd.ReceiverName)), " ")
	cmd.ReceiverPhone = strings.TrimSpace(cmd.ReceiverPhone)
	cmd.ReceiverAddress = strings.Join(strings.Fields(strings.TrimSpace(cmd.ReceiverAddress)), " ")
	cmd.ReceiverCompany = strings.Join(strings.Fields(strings.TrimSpace(cmd.ReceiverCompany)), " ")
	cmd.ProductName = strings.Join(strings.Fields(strings.TrimSpace(cmd.ProductName)), " ")
	cmd.Spec = strings.Join(strings.Fields(strings.TrimSpace(cmd.Spec)), " ")
	if cmd.ShippingAmount < 0 {
		cmd.ShippingAmount = 0
	}
	for i := range cmd.Items {
		cmd.Items[i].ProductName = strings.Join(strings.Fields(strings.TrimSpace(cmd.Items[i].ProductName)), " ")
		cmd.Items[i].Spec = strings.Join(strings.Fields(strings.TrimSpace(cmd.Items[i].Spec)), " ")
		cmd.Items[i].Note = strings.TrimSpace(cmd.Items[i].Note)
	}
	cmd.Note = strings.TrimSpace(cmd.Note)
	if cmd.ReceiverName == "" {
		return DirectShipOrderSummary{}, fmt.Errorf("receiver_name required")
	}
	if cmd.ReceiverPhone == "" {
		return DirectShipOrderSummary{}, fmt.Errorf("receiver_phone required")
	}
	if cmd.ReceiverAddress == "" {
		return DirectShipOrderSummary{}, fmt.Errorf("receiver_address required")
	}
	if len(cmd.Items) == 0 {
		cmd.Items = []SubmitCustomerDirectShipOrderItem{{
			ProductID:     cmd.ProductID,
			ProductName:   cmd.ProductName,
			Spec:          cmd.Spec,
			QuantityUnits: cmd.QuantityUnits,
			Note:          cmd.Note,
		}}
	}
	for _, item := range cmd.Items {
		if item.ProductID <= 0 && item.ProductName == "" {
			return DirectShipOrderSummary{}, fmt.Errorf("product required")
		}
		specG := item.SpecG
		if specG <= 0 {
			specG = parseCustomerFulfillmentSpecG(item.Spec)
		}
		if specG <= 0 {
			return DirectShipOrderSummary{}, fmt.Errorf("spec required")
		}
		if item.QuantityUnits <= 0 {
			return DirectShipOrderSummary{}, fmt.Errorf("quantity required")
		}
	}
	return s.repo.SubmitCustomerDirectShipOrder(ctx, cmd)
}

func parseCustomerFulfillmentSpecG(spec string) int64 {
	spec = strings.TrimSpace(strings.TrimSuffix(strings.ToLower(spec), "g"))
	if spec == "" {
		return 0
	}
	n, err := strconv.ParseInt(spec, 10, 64)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func (s *Service) AdjustCustodyInventory(ctx context.Context, cmd AdjustCustodyInventoryCommand) (CustodyBalance, error) {
	if cmd.CustomerID <= 0 {
		return CustodyBalance{}, fmt.Errorf("customer required")
	}
	cmd.ItemType = normalizeCustodyItemType(cmd.ItemType)
	cmd.ItemName = strings.Join(strings.Fields(strings.TrimSpace(cmd.ItemName)), " ")
	cmd.Spec = strings.Join(strings.Fields(strings.TrimSpace(cmd.Spec)), " ")
	cmd.Note = strings.TrimSpace(cmd.Note)
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.ItemType == "" {
		return CustodyBalance{}, fmt.Errorf("item_type invalid")
	}
	if cmd.ItemName == "" {
		return CustodyBalance{}, fmt.Errorf("item_name required")
	}
	if cmd.QuantityGDelta == 0 && cmd.QuantityUnitsDelta == 0 {
		return CustodyBalance{}, fmt.Errorf("quantity delta required")
	}
	return s.repo.AdjustCustodyInventory(ctx, cmd)
}

func (s *Service) UpsertCustomerERPBinding(ctx context.Context, cmd UpsertCustomerERPBindingCommand) (CustomerERPBinding, error) {
	if cmd.CustomerID <= 0 {
		return CustomerERPBinding{}, fmt.Errorf("customer required")
	}
	if cmd.EmployeeID <= 0 {
		return CustomerERPBinding{}, fmt.Errorf("employee required")
	}
	cmd.Role = strings.TrimSpace(cmd.Role)
	if cmd.Role == "" {
		cmd.Role = "customer"
	}
	cmd.Status = normalizeCustomerERPBindingStatus(cmd.Status)
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	return s.repo.UpsertCustomerERPBinding(ctx, cmd)
}

func (s *Service) ListCustomerERPBindings(ctx context.Context, customerID int64) ([]CustomerERPBinding, error) {
	if customerID <= 0 {
		return nil, fmt.Errorf("customer required")
	}
	return s.repo.ListCustomerERPBindings(ctx, customerID)
}

func (s *Service) CustomerERPWorkbenchAvailable(ctx context.Context, customerID int64) (bool, error) {
	if customerID <= 0 {
		return false, fmt.Errorf("customer required")
	}
	return s.repo.CustomerERPWorkbenchAvailable(ctx, customerID)
}

func (s *Service) CreateExternalUser(ctx context.Context, cmd CreateExternalUserCommand) (CustomerExternalUser, error) {
	if cmd.CustomerID <= 0 {
		return CustomerExternalUser{}, fmt.Errorf("customer required")
	}
	cmd.Name = strings.Join(strings.Fields(strings.TrimSpace(cmd.Name)), " ")
	cmd.Phone = strings.TrimSpace(cmd.Phone)
	cmd.Password = strings.TrimSpace(cmd.Password)
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if cmd.Name == "" {
		return CustomerExternalUser{}, fmt.Errorf("name required")
	}
	if !externalUserPhoneRe.MatchString(cmd.Phone) {
		return CustomerExternalUser{}, fmt.Errorf("phone invalid")
	}
	if len(cmd.Password) < 6 {
		return CustomerExternalUser{}, fmt.Errorf("password too short")
	}
	return s.repo.CreateExternalUser(ctx, cmd)
}

func (s *Service) ListExternalUsers(ctx context.Context, customerID int64) ([]CustomerExternalUser, error) {
	if customerID <= 0 {
		return nil, fmt.Errorf("customer required")
	}
	return s.repo.ListExternalUsers(ctx, customerID)
}

func (s *Service) ResetExternalUserPassword(ctx context.Context, cmd ResetExternalUserPasswordCommand) (CustomerExternalUser, error) {
	if cmd.CustomerID <= 0 {
		return CustomerExternalUser{}, fmt.Errorf("customer required")
	}
	if cmd.EmployeeID <= 0 {
		return CustomerExternalUser{}, fmt.Errorf("employee required")
	}
	cmd.Password = strings.TrimSpace(cmd.Password)
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	if len(cmd.Password) < 6 {
		return CustomerExternalUser{}, fmt.Errorf("password too short")
	}
	return s.repo.ResetExternalUserPassword(ctx, cmd)
}

func (s *Service) SetExternalUserLoginEnabled(ctx context.Context, cmd SetExternalUserLoginEnabledCommand) (CustomerExternalUser, error) {
	if cmd.CustomerID <= 0 {
		return CustomerExternalUser{}, fmt.Errorf("customer required")
	}
	if cmd.EmployeeID <= 0 {
		return CustomerExternalUser{}, fmt.Errorf("employee required")
	}
	cmd.Actor = strings.TrimSpace(cmd.Actor)
	return s.repo.SetExternalUserLoginEnabled(ctx, cmd)
}

func (s *Service) CustomerFulfillmentOptions(ctx context.Context, customerID int64) (CustomerFulfillmentOptions, error) {
	if customerID <= 0 {
		return CustomerFulfillmentOptions{}, fmt.Errorf("customer required")
	}
	return s.repo.CustomerFulfillmentOptions(ctx, customerID)
}

func (s *Service) ListImportRows(ctx context.Context, query ListImportRowsQuery) ([]ImportRow, error) {
	if query.BatchID <= 0 {
		return nil, fmt.Errorf("batch required")
	}
	query.Status = normalizeImportRowStatus(query.Status)
	if query.Limit <= 0 {
		query.Limit = 200
	}
	if query.Limit > 500 {
		query.Limit = 500
	}
	if query.Offset < 0 {
		query.Offset = 0
	}
	return s.repo.ListImportRows(ctx, query)
}

func (s *Service) ImportPreview(ctx context.Context, query ImportPreviewQuery) (ImportPreview, error) {
	if query.BatchID <= 0 {
		return ImportPreview{}, fmt.Errorf("batch required")
	}
	batch, err := s.repo.ImportBatch(ctx, query.BatchID)
	if err != nil {
		return ImportPreview{}, err
	}
	return ImportPreview{
		Batch:   batch,
		Effects: importPreviewEffects(batch.Summary),
		Warning: "预览不写入业务数据；确认应用后才会写入库存、工单、订单和费用。",
	}, nil
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

func normalizeImportRowStatus(status string) string {
	switch strings.TrimSpace(status) {
	case "valid", "invalid", "applied":
		return strings.TrimSpace(status)
	default:
		return ""
	}
}

func normalizeOptionalDate(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return ""
	}
	return parsed.Format("2006-01-02")
}

func normalizeCustodyItemType(value string) string {
	switch strings.TrimSpace(value) {
	case "raw_bean", "packaging", "product":
		return strings.TrimSpace(value)
	default:
		return ""
	}
}

func normalizeCustomerERPBindingStatus(value string) string {
	switch strings.TrimSpace(value) {
	case "inactive", "disabled":
		return "inactive"
	default:
		return "active"
	}
}

func importPreviewEffects(summary ImportSummary) []ImportPreviewEffect {
	effects := make([]ImportPreviewEffect, 0, 12)
	addPreviewEffect(&effects, "将应用有效行", summary.ValidRows)
	addPreviewEffect(&effects, "需先处理错误行", summary.InvalidRows)
	addPreviewEffect(&effects, "托管生豆入库", summary.RawBeanReceipts)
	addPreviewEffect(&effects, "托管生豆出库", summary.RawBeanIssues)
	addPreviewEffect(&effects, "托管生豆盘点", summary.RawBeanBalances)
	addPreviewEffect(&effects, "客户 SKU", summary.CustomerSKUs)
	addPreviewEffect(&effects, "包材库存", summary.PackagingBalances)
	addPreviewEffect(&effects, "加工工单", summary.ProcessingOrders)
	addPreviewEffect(&effects, "包装任务", summary.PackagingJobs)
	addPreviewEffect(&effects, "库存转换", summary.ConversionJobs)
	addPreviewEffect(&effects, "代发订单", summary.DirectShipOrders)
	addPreviewEffect(&effects, "代发明细", summary.DirectShipItems)
	addPreviewEffect(&effects, "费用明细", summary.FeeItems)
	addPreviewEffect(&effects, "结算批次", summary.SettlementBatches)
	return effects
}

func addPreviewEffect(effects *[]ImportPreviewEffect, label string, value int) {
	if value <= 0 {
		return
	}
	*effects = append(*effects, ImportPreviewEffect{Label: label, Value: value})
}
