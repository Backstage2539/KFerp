package samples

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

var ErrForbidden = errors.New("无权访问此样品工单")
var ErrNotFound = errors.New("样品申请不存在")
var ErrConflict = errors.New("工单已更新，请刷新后重试")

type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }
func Invalid(s string) error             { return &ValidationError{s} }

type Principal struct {
	EmployeeID, MiniUserID   int64
	PriceCustomerID          int64 // Authenticated page scope, derived from current customer capabilities.
	Admin, Sales, Registered bool
	Actor                    string
}

func (p Principal) Staff() bool { return p.Admin || (p.EmployeeID > 0 && p.Sales) }

type Receiver struct {
	Name    string `json:"name"`
	Phone   string `json:"phone"`
	Address string `json:"address"`
}
type Item struct {
	Key       string `json:"key"`
	ProductID int64  `json:"product_id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
}
type Activity struct {
	ID        int64     `json:"id"`
	Kind      string    `json:"kind"`
	Status    string    `json:"status"`
	Note      string    `json:"note"`
	Actor     string    `json:"actor"`
	CreatedAt time.Time `json:"created_at"`
}
type Request struct {
	ID                 int64           `json:"id"`
	Number             string          `json:"number"`
	MiniUserID         int64           `json:"mini_user_id,omitempty"`
	Nickname           string          `json:"nickname"`
	VerifiedPhone      string          `json:"verified_phone,omitempty"`
	EntryKey           string          `json:"entry_key"`
	PublicationID      int64           `json:"publication_id"`
	PublicationName    string          `json:"publication_name"`
	PublicationVersion string          `json:"publication_version"`
	Receiver           Receiver        `json:"receiver"`
	Note               string          `json:"note"`
	Items              []Item          `json:"items"`
	WorkOrderID        int64           `json:"work_order_id"`
	AssigneeID         int64           `json:"assignee_id"`
	AssigneeName       string          `json:"assignee_name"`
	Status             string          `json:"status"`
	FollowupStatus     string          `json:"followup_status"`
	Revision           int64           `json:"revision"`
	CustomerID         int64           `json:"customer_id"`
	OrderID            int64           `json:"order_id"`
	OrderNo            string          `json:"order_no"`
	ShipmentStatus     string          `json:"shipment_status"`
	CreatedAt          time.Time       `json:"created_at"`
	NextReminderAt     *time.Time      `json:"next_reminder_at,omitempty"`
	Confirmation       *ConfirmCommand `json:"confirmation,omitempty"`
	Activities         []Activity      `json:"activities,omitempty"`
}
type SubmitCommand struct {
	RequestID     string   `json:"request_id"`
	EntryKey      string   `json:"entry_key"`
	PublicationID int64    `json:"publication_id"`
	ProductKeys   []string `json:"product_keys"`
	Receiver      Receiver `json:"receiver"`
	Note          string   `json:"note"`
}
type AssignCommand struct {
	Revision   int64 `json:"revision"`
	EmployeeID int64 `json:"employee_id"`
}
type FollowupCommand struct {
	Revision int64  `json:"revision"`
	Status   string `json:"status"`
	Note     string `json:"note"`
}
type NewCustomer struct {
	Name         string `json:"name"`
	CustomerType string `json:"customer_type"`
	SourceID     int64  `json:"source_id"`
	OrderTypeID  int64  `json:"order_type_id"`
}
type ConfirmItem struct {
	SpecName  string  `json:"spec_name,omitempty"`
	Unit      string  `json:"unit,omitempty"`
	ProductID int64   `json:"product_id"`
	BomSpecID int64   `json:"bom_spec_id"`
	Qty       int64   `json:"qty"`
	UnitPrice float64 `json:"unit_price"`
}
type ConfirmCommand struct {
	Revision       int64         `json:"revision"`
	CustomerID     int64         `json:"customer_id"`
	NewCustomer    *NewCustomer  `json:"new_customer,omitempty"`
	Receiver       Receiver      `json:"receiver"`
	Items          []ConfirmItem `json:"items"`
	ShippingAmount float64       `json:"shipping_amount"`
	Note           string        `json:"note"`
}
type Option struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
type SpecOption struct {
	ID        int64  `json:"id"`
	ProductID int64  `json:"product_id"`
	Name      string `json:"name"`
	Unit      string `json:"unit"`
}
type Options struct {
	SuggestedCustomerID int64        `json:"suggested_customer_id"`
	Customers           []Option     `json:"customers"`
	Employees           []Option     `json:"employees"`
	Specs               []SpecOption `json:"specs"`
	Sources             []Option     `json:"sources"`
	OrderTypes          []Option     `json:"order_types"`
	CustomerTypes       []string     `json:"customer_types"`
}
type Query struct {
	Search, Status string
	Page, PageSize int
	MiniUserID     int64
}
type ListResult struct {
	Rows    []Request `json:"rows"`
	Total   int       `json:"total"`
	IsAdmin bool      `json:"is_admin"`
}
type Repository interface {
	Submit(context.Context, Principal, SubmitCommand) (Request, error)
	List(context.Context, Principal, Query) (ListResult, error)
	Get(context.Context, Principal, int64) (Request, error)
	Assign(context.Context, Principal, int64, AssignCommand) (Request, error)
	Confirm(context.Context, Principal, int64, ConfirmCommand) (Request, error)
	Followup(context.Context, Principal, int64, FollowupCommand) (Request, error)
	Options(context.Context, Principal, int64, string) (Options, error)
	Remind(context.Context, time.Time) (int, error)
}
type Service struct{ Repository }

func NewService(r Repository) *Service { return &Service{r} }
func CanRead(p Principal, r Request) bool {
	return p.Staff() && (p.Admin || r.AssigneeID == p.EmployeeID) || !p.Staff() && p.EmployeeID == 0 && p.Registered && p.MiniUserID > 0 && p.MiniUserID == r.MiniUserID
}
func CanWork(p Principal, r Request) bool {
	return p.Staff() && (p.Admin || r.AssigneeID == p.EmployeeID)
}
func Terminal(s string) bool { return s == "won" || s == "abandoned" }
func NextFollowup(status, note string, now time.Time) (*time.Time, error) {
	if status != "pending" && status != "following" && !Terminal(status) {
		return nil, Invalid("请选择有效的回访状态")
	}
	if strings.TrimSpace(note) == "" || len([]rune(note)) > 2000 {
		return nil, Invalid("请填写不超过2000字的回访说明")
	}
	if Terminal(status) {
		return nil, nil
	}
	next := now.Add(7 * 24 * time.Hour)
	return &next, nil
}
func ValidateReceiver(r Receiver) error {
	if strings.TrimSpace(r.Name) == "" || strings.TrimSpace(r.Phone) == "" || strings.TrimSpace(r.Address) == "" {
		return Invalid("请填写收件人、电话和地址")
	}
	if len([]rune(r.Name)) > 100 || len(r.Phone) > 32 || len([]rune(r.Address)) > 500 {
		return Invalid("收件信息过长")
	}
	return nil
}
func ValidateSubmit(c SubmitCommand) error {
	if len(c.RequestID) < 8 || len(c.RequestID) > 100 || len(c.EntryKey) != 32 || c.PublicationID <= 0 || len(c.ProductKeys) == 0 || len(c.ProductKeys) > 100 {
		return Invalid("请选择价格表商品后提交")
	}
	if len([]rune(c.Note)) > 2000 {
		return Invalid("留言不能超过2000字")
	}
	return ValidateReceiver(c.Receiver)
}
func ValidateConfirm(c ConfirmCommand) error {
	if c.CustomerID <= 0 && c.NewCustomer == nil {
		return Invalid("请选择或新建客户")
	}
	if err := ValidateReceiver(c.Receiver); err != nil {
		return err
	}
	if len(c.Items) == 0 || len(c.Items) > 100 {
		return Invalid("请选择需要寄出的商品")
	}
	if !wholeCentAmount(c.ShippingAmount) {
		return Invalid("运费必须为非负金额，最多保留两位小数")
	}
	seen := map[string]bool{}
	for _, i := range c.Items {
		if i.ProductID <= 0 || i.BomSpecID <= 0 || i.Qty <= 0 || i.Qty > 100000 || !wholeCentAmount(i.UnitPrice) {
			return Invalid("请选择有效BOM规格、数量和非负单价（金额最多两位小数）")
		}
		key := fmt.Sprintf("%d:%d", i.ProductID, i.BomSpecID)
		if seen[key] {
			return Invalid("同一商品规格请合并数量")
		}
		seen[key] = true
	}
	return nil
}

func wholeCentAmount(value float64) bool {
	cents := value * 100
	return value >= 0 && !math.IsNaN(cents) && !math.IsInf(cents, 0) && math.Abs(cents-math.Round(cents)) < 0.000001
}
