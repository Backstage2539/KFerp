package customer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

var (
	ErrCustomerMaintenanceForbidden = errors.New("customer maintenance forbidden")
	ErrCustomerNotFound             = errors.New("customer not found")
)

type MaintenanceValidationError struct {
	message string
}

func (e *MaintenanceValidationError) Error() string {
	return e.message
}

func NewMaintenanceValidationError(message string) error {
	return &MaintenanceValidationError{message: strings.TrimSpace(message)}
}

func MaintenanceValidationMessage(err error) (string, bool) {
	var validationErr *MaintenanceValidationError
	if !errors.As(err, &validationErr) || validationErr == nil || validationErr.message == "" {
		return "", false
	}
	return validationErr.message, true
}

const (
	CustomerTypeRetail    = "retail"
	CustomerTypeEcommerce = "ecommerce"
	CustomerTypeWholesale = "wholesale"
	CustomerTypeChannel   = "channel"
)

type UpsertCommand struct {
	Name                  string
	RawName               string
	CustomerType          string
	CompanyName           string
	CompanyAddress        string
	CompanyPhone          string
	Contact               string
	Phone                 string
	Address               string
	DefaultSourceID       string
	DefaultOrderTypeID    string
	ResponsibleEmployeeID string
	Active                string
	PortalEnabled         *bool
	CapabilityTemplateKey string
}

type InlineUpdateCommand struct {
	Name                  string
	CustomerType          string
	CompanyName           string
	CompanyAddress        string
	CompanyPhone          string
	Contact               string
	Phone                 string
	Address               string
	DefaultSourceID       string
	DefaultOrderTypeID    string
	ResponsibleEmployeeID string
	Active                string
}

type Prefs struct {
	ID              int64   `json:"id"`
	DefaultSourceID *int    `json:"default_source_id"`
	SourceName      *string `json:"source_name"`
	DefaultTypeID   *int    `json:"default_order_type_id"`
	TypeName        *string `json:"order_type_name"`
	Address         *string `json:"address"`
}

type SaveAssetCommand struct {
	CustomerID  int64
	Kind        string
	Reader      io.Reader
	ContentType string
	Filename    string
	MaxBytes    int64
	Actor       string
}

type SaveAssetResult struct {
	CustomerID int64
	ObjectKey  string
	Bytes      int64
	SHA256     string
}

type DeleteAssetResult struct {
	CustomerID int64
	ObjectKey  string
}

type ListQuery struct {
	Query                 string
	Limit                 int
	Offset                int
	CustomerType          string
	Active                *bool
	ResponsibleEmployeeID int64
	SortBy                string
	SortDirection         string
}

type MaintenancePrincipal struct {
	EmployeeID   int64
	EmployeeName string
	IsAdmin      bool
}

type ListResult struct {
	Rows                []CustomerRow
	Sources             []Option
	OrderTypes          []Option
	Employees           []Option
	CustomerTypeOptions []CustomerTypeOption
	Total               int
	HasNext             bool
}

type CustomerRow struct {
	ID                      int64   `json:"id"`
	Name                    string  `json:"name"`
	CustomerType            string  `json:"customer_type"`
	CompanyName             string  `json:"company_name"`
	CompanyAddress          string  `json:"company_address"`
	CompanyPhone            string  `json:"company_phone"`
	Contact                 *string `json:"contact"`
	Phone                   *string `json:"phone"`
	Address                 *string `json:"address"`
	Active                  bool    `json:"active"`
	DefaultSourceID         *int    `json:"default_source_id"`
	DefaultOrderTypeID      *int    `json:"default_order_type_id"`
	ResponsibleEmployeeID   *int    `json:"responsible_employee_id"`
	ResponsibleEmployeeName string  `json:"responsible_employee_name"`
	PortalEnabled           bool    `json:"portal_enabled"`
	CapabilityTemplateKey   string  `json:"capability_template_key"`
	Updated                 string  `json:"updated"`
}

type CustomerEditData struct {
	ID                      int64
	Name                    string
	RawName                 string
	CustomerType            string
	CompanyName             string
	CompanyAddress          string
	CompanyPhone            string
	Contact                 string
	Phone                   string
	Address                 string
	DefaultSourceID         string
	DefaultOrderTypeID      string
	ResponsibleEmployeeID   string
	ResponsibleEmployeeName string
	PortalEnabled           bool
	CapabilityTemplateKey   string
	Active                  bool
}

type CustomerAsset struct {
	ID          int64
	CustomerID  int64
	Kind        string
	ObjectKey   string
	ContentType string
	Bytes       int64
	Sha256      string
	CreatedAt   string
}

type CustomerDashboard struct {
	TotalOrders     int
	UnpaidOrders    int
	UnshippedOrders int
	InProduction    int
	InShipping      int
	Completed       int
}

type Option struct {
	ID   int64
	Name string
}

type CustomerTypeOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type CreateCustomerTypeCommand struct {
	Label string
	Value string
}

type CreateOrderTypeCommand struct {
	Name string
}

type EditorData struct {
	Customer            CustomerEditData
	Sources             []Option
	OrderTypes          []Option
	Employees           []Option
	CustomerTypeOptions []CustomerTypeOption
	Assets              []CustomerAsset
	Dashboard           CustomerDashboard
}

type AssetObject struct {
	ObjectKey   string
	ContentType string
}

type Repository interface {
	Upsert(ctx context.Context, actor string, id *int64, cmd UpsertCommand) (int64, error)
	List(ctx context.Context, query ListQuery) (ListResult, error)
	Editor(ctx context.Context, id int64) (*EditorData, error)
	Prefs(ctx context.Context, id int64) (*Prefs, error)
	AssetObject(ctx context.Context, assetID int64) (AssetObject, error)
	SaveAsset(ctx context.Context, cmd SaveAssetCommand) (SaveAssetResult, error)
	DeleteAsset(ctx context.Context, actor string, assetID int64) (DeleteAssetResult, error)
	InlineUpdate(ctx context.Context, actor string, id int64, cmd InlineUpdateCommand) error
	Delete(ctx context.Context, actor string, id int64) error
	ListCustomerTypeOptions(ctx context.Context) ([]CustomerTypeOption, error)
	CreateCustomerTypeOption(ctx context.Context, actor string, cmd CreateCustomerTypeCommand) (CustomerTypeOption, error)
	CreateOrderTypeOption(ctx context.Context, actor string, cmd CreateOrderTypeCommand) (Option, error)
}

type ManagedRepository interface {
	ListManaged(ctx context.Context, actor MaintenancePrincipal, query ListQuery) (ListResult, error)
	EditorManaged(ctx context.Context, actor MaintenancePrincipal, id int64) (*EditorData, error)
	UpsertManaged(ctx context.Context, actor MaintenancePrincipal, id *int64, cmd UpsertCommand) (int64, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Upsert(ctx context.Context, actor string, id *int64, cmd UpsertCommand) (int64, error) {
	if err := validateRequiredCustomerProfileDefaults(cmd.CustomerType, cmd.DefaultSourceID, cmd.DefaultOrderTypeID); err != nil {
		return 0, err
	}
	cmd.CustomerType = NormalizeCustomerType(cmd.CustomerType)
	cmd.CapabilityTemplateKey = ""
	return s.repo.Upsert(ctx, actor, id, cmd)
}

func (s *Service) List(ctx context.Context, query ListQuery) (ListResult, error) {
	query = normalizeListQuery(query)
	return s.repo.List(ctx, query)
}

func (s *Service) ListManaged(ctx context.Context, actor MaintenancePrincipal, query ListQuery) (ListResult, error) {
	actor, err := normalizeMaintenancePrincipal(actor)
	if err != nil {
		return ListResult{}, err
	}
	repo, ok := s.repo.(ManagedRepository)
	if !ok {
		return ListResult{}, fmt.Errorf("managed customer repository required")
	}
	result, err := repo.ListManaged(ctx, actor, normalizeListQuery(query))
	if err != nil {
		return ListResult{}, err
	}
	if !actor.IsAdmin {
		employees := make([]Option, 0, 1)
		for _, employee := range result.Employees {
			if employee.ID == actor.EmployeeID {
				employees = append(employees, employee)
				break
			}
		}
		result.Employees = employees
	}
	return result, nil
}

func normalizeListQuery(query ListQuery) ListQuery {
	query.CustomerType = NormalizeCustomerTypeFilter(query.CustomerType)
	query.SortBy = NormalizeCustomerSortBy(query.SortBy)
	query.SortDirection = NormalizeCustomerSortDirection(query.SortDirection)
	if query.Limit <= 0 {
		query.Limit = 10
	}
	if query.Limit > 200 {
		query.Limit = 200
	}
	if query.Offset < 0 {
		query.Offset = 0
	}
	return query
}

func (s *Service) Editor(ctx context.Context, id int64) (*EditorData, error) {
	return s.repo.Editor(ctx, id)
}

func (s *Service) EditorManaged(ctx context.Context, actor MaintenancePrincipal, id int64) (*EditorData, error) {
	actor, err := normalizeMaintenancePrincipal(actor)
	if err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, ErrCustomerNotFound
	}
	repo, ok := s.repo.(ManagedRepository)
	if !ok {
		return nil, fmt.Errorf("managed customer repository required")
	}
	return repo.EditorManaged(ctx, actor, id)
}

func (s *Service) UpsertManaged(ctx context.Context, actor MaintenancePrincipal, id *int64, cmd UpsertCommand) (int64, error) {
	actor, err := normalizeMaintenancePrincipal(actor)
	if err != nil {
		return 0, err
	}
	if err := validateRequiredCustomerProfileDefaults(cmd.CustomerType, cmd.DefaultSourceID, cmd.DefaultOrderTypeID); err != nil {
		return 0, err
	}
	cmd.CustomerType = NormalizeCustomerType(cmd.CustomerType)
	cmd.CapabilityTemplateKey = ""
	if !actor.IsAdmin {
		cmd.ResponsibleEmployeeID = strconv.FormatInt(actor.EmployeeID, 10)
		cmd.PortalEnabled = nil
		if id == nil {
			cmd.Active = "on"
		}
	}
	repo, ok := s.repo.(ManagedRepository)
	if !ok {
		return 0, fmt.Errorf("managed customer repository required")
	}
	return repo.UpsertManaged(ctx, actor, id, cmd)
}

func normalizeMaintenancePrincipal(actor MaintenancePrincipal) (MaintenancePrincipal, error) {
	if actor.EmployeeID <= 0 {
		return MaintenancePrincipal{}, ErrCustomerMaintenanceForbidden
	}
	actor.EmployeeName = strings.TrimSpace(actor.EmployeeName)
	return actor, nil
}

func (s *Service) Prefs(ctx context.Context, id int64) (*Prefs, error) {
	return s.repo.Prefs(ctx, id)
}

func (s *Service) AssetObject(ctx context.Context, assetID int64) (AssetObject, error) {
	return s.repo.AssetObject(ctx, assetID)
}

func (s *Service) SaveAsset(ctx context.Context, cmd SaveAssetCommand) (SaveAssetResult, error) {
	return s.repo.SaveAsset(ctx, cmd)
}

func (s *Service) DeleteAsset(ctx context.Context, actor string, assetID int64) (DeleteAssetResult, error) {
	return s.repo.DeleteAsset(ctx, actor, assetID)
}

func (s *Service) InlineUpdate(ctx context.Context, actor string, id int64, cmd InlineUpdateCommand) error {
	if err := validateRequiredCustomerProfileDefaults(cmd.CustomerType, cmd.DefaultSourceID, cmd.DefaultOrderTypeID); err != nil {
		return err
	}
	cmd.CustomerType = NormalizeCustomerType(cmd.CustomerType)
	return s.repo.InlineUpdate(ctx, actor, id, cmd)
}

func (s *Service) Delete(ctx context.Context, actor string, id int64) error {
	return s.repo.Delete(ctx, actor, id)
}

func (s *Service) CreateCustomerTypeOption(ctx context.Context, actor string, cmd CreateCustomerTypeCommand) (CustomerTypeOption, error) {
	cmd.Label = strings.TrimSpace(cmd.Label)
	cmd.Value = strings.TrimSpace(cmd.Value)
	if cmd.Label == "" {
		return CustomerTypeOption{}, fmt.Errorf("label required")
	}
	return s.repo.CreateCustomerTypeOption(ctx, actor, cmd)
}

func (s *Service) CreateOrderTypeOption(ctx context.Context, actor string, cmd CreateOrderTypeCommand) (Option, error) {
	cmd.Name = strings.TrimSpace(cmd.Name)
	if cmd.Name == "" {
		return Option{}, fmt.Errorf("name required")
	}
	return s.repo.CreateOrderTypeOption(ctx, actor, cmd)
}

func DefaultCustomerTypeOptions() []CustomerTypeOption {
	return []CustomerTypeOption{
		{Value: CustomerTypeRetail, Label: "零售客户"},
		{Value: CustomerTypeEcommerce, Label: "电商客户"},
		{Value: CustomerTypeWholesale, Label: "批发客户"},
		{Value: CustomerTypeChannel, Label: "渠道客户"},
	}
}

func NormalizeCustomerType(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return CustomerTypeRetail
	}
	return value
}

func validRequiredCustomerType(value string) bool {
	return strings.TrimSpace(value) != ""
}

func positiveIDString(value string) bool {
	n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return err == nil && n > 0
}

func validateRequiredCustomerProfileDefaults(customerType, sourceID, orderTypeID string) error {
	missing := make([]string, 0, 3)
	if !validRequiredCustomerType(customerType) {
		missing = append(missing, "客户类型")
	}
	if !positiveIDString(sourceID) {
		missing = append(missing, "来源")
	}
	if !positiveIDString(orderTypeID) {
		missing = append(missing, "订单类型")
	}
	if len(missing) > 0 {
		return NewMaintenanceValidationError(fmt.Sprintf("请维护客户资料：%s", strings.Join(missing, "、")))
	}
	return nil
}

func NormalizeCustomerTypeFilter(value string) string {
	return strings.TrimSpace(value)
}

func NormalizeCustomerSortBy(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "name", "updated":
		return strings.TrimSpace(strings.ToLower(value))
	default:
		return "name"
	}
}

func NormalizeCustomerSortDirection(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "desc", "descending":
		return "desc"
	default:
		return "asc"
	}
}
