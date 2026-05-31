package support

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	authzapp "orderapp/internal/application/authz"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

const (
	viewContextFactory          = "factory"
	viewContextCustomer         = "customer"
	viewContextOrder            = "order"
	viewContextExternalCustomer = "external_customer"
)

type viewContextHandler struct {
	pool   *pgxpool.Pool
	schema string
	authz  AuthzService
}

type viewContextOption struct {
	Type         string `json:"type"`
	ID           int64  `json:"id"`
	Label        string `json:"label"`
	CustomerID   int64  `json:"customer_id,omitempty"`
	CustomerName string `json:"customer_name,omitempty"`
	CompanyName  string `json:"company_name,omitempty"`
	Contact      string `json:"contact,omitempty"`
	Phone        string `json:"phone,omitempty"`
	OrderID      int64  `json:"order_id,omitempty"`
	OrderNo      string `json:"order_no,omitempty"`
	OrderDate    string `json:"order_date,omitempty"`
	Status       string `json:"status,omitempty"`
}

type viewContextPreset struct {
	ID              int64           `json:"id"`
	Name            string          `json:"name"`
	ContextType     string          `json:"context_type"`
	ContextJSON     json.RawMessage `json:"context_json"`
	MenuKeysJSON    json.RawMessage `json:"menu_keys_json"`
	OwnerEmployeeID int64           `json:"owner_employee_id"`
	Active          bool            `json:"active"`
	SortOrder       int             `json:"sort_order"`
	CreatedAt       string          `json:"created_at"`
	UpdatedAt       string          `json:"updated_at"`
	CreatedBy       string          `json:"created_by"`
	UpdatedBy       string          `json:"updated_by"`
}

type viewContextPresetPayload struct {
	Name         string          `json:"name"`
	ContextType  string          `json:"context_type"`
	ContextJSON  json.RawMessage `json:"context_json"`
	MenuKeysJSON json.RawMessage `json:"menu_keys_json"`
	SortOrder    int             `json:"sort_order"`
}

func registerViewContextAPI(e *echo.Echo, pool *pgxpool.Pool, schema string, authz AuthzService) {
	h := viewContextHandler{pool: pool, schema: schema, authz: authz}
	e.GET("/api/view-context/options", h.options)
	e.GET("/api/view-context/presets", h.listPresets)
	e.POST("/api/view-context/presets", h.createPreset)
	e.PUT("/api/view-context/presets/:id", h.updatePreset)
	e.POST("/api/view-context/presets/:id/disable", h.disablePreset)
}

func ensureViewContextPresetTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.view_context_presets (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL DEFAULT '',
	context_type TEXT NOT NULL DEFAULT 'factory',
	context_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	menu_keys_json JSONB NOT NULL DEFAULT '[]'::jsonb,
	owner_employee_id BIGINT NOT NULL DEFAULT 0,
	active BOOLEAN NOT NULL DEFAULT true,
	sort_order INTEGER NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	created_by TEXT NOT NULL DEFAULT '',
	updated_by TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS view_context_presets_owner_active_idx ON %s.view_context_presets(owner_employee_id, active, sort_order, id);
`, schema, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	for _, stmt := range []string{
		`ALTER TABLE %[1]s.view_context_presets ADD COLUMN IF NOT EXISTS name TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE %[1]s.view_context_presets ADD COLUMN IF NOT EXISTS context_type TEXT NOT NULL DEFAULT 'factory'`,
		`ALTER TABLE %[1]s.view_context_presets ADD COLUMN IF NOT EXISTS context_json JSONB NOT NULL DEFAULT '{}'::jsonb`,
		`ALTER TABLE %[1]s.view_context_presets ADD COLUMN IF NOT EXISTS menu_keys_json JSONB NOT NULL DEFAULT '[]'::jsonb`,
		`ALTER TABLE %[1]s.view_context_presets ADD COLUMN IF NOT EXISTS owner_employee_id BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE %[1]s.view_context_presets ADD COLUMN IF NOT EXISTS active BOOLEAN NOT NULL DEFAULT true`,
		`ALTER TABLE %[1]s.view_context_presets ADD COLUMN IF NOT EXISTS sort_order INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE %[1]s.view_context_presets ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now()`,
		`ALTER TABLE %[1]s.view_context_presets ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`,
		`ALTER TABLE %[1]s.view_context_presets ADD COLUMN IF NOT EXISTS created_by TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE %[1]s.view_context_presets ADD COLUMN IF NOT EXISTS updated_by TEXT NOT NULL DEFAULT ''`,
	} {
		if _, err := pool.Exec(ctx, fmt.Sprintf(stmt, schema)); err != nil {
			return err
		}
	}
	return nil
}

func (h viewContextHandler) options(c echo.Context) error {
	actor, _, err := CurrentActor(c, h.authz)
	if err != nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": err.Error()})
	}
	contextType := normalizeViewContextOptionType(c.QueryParam("type"))
	if contextType == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid type"})
	}
	limit := IntParam(c, "limit", 20)
	if limit <= 0 {
		limit = 20
	}
	if limit > 80 {
		limit = 80
	}
	q := strings.TrimSpace(c.QueryParam("q"))
	customerID := int64(IntParam(c, "customer_id", 0))
	orderID := int64(IntParam(c, "order_id", 0))

	boundCustomerID := int64(0)
	if isExternalCustomerActor(actor) {
		boundCustomerID, err = h.boundCustomerID(c.Request().Context(), actor.EmployeeID)
		if err != nil {
			return c.JSON(http.StatusForbidden, map[string]string{"error": err.Error()})
		}
		if customerID > 0 && customerID != boundCustomerID {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "customer context forbidden"})
		}
		if orderID > 0 {
			orderCustomerID, err := h.orderCustomerID(c.Request().Context(), orderID)
			if err != nil {
				return c.JSON(http.StatusForbidden, map[string]string{"error": err.Error()})
			}
			if orderCustomerID != boundCustomerID {
				return c.JSON(http.StatusForbidden, map[string]string{"error": "order context forbidden"})
			}
		}
		customerID = boundCustomerID
	}

	var options []viewContextOption
	switch contextType {
	case viewContextCustomer:
		options, err = h.customerOptions(c.Request().Context(), q, customerID, limit)
	case viewContextOrder:
		options, err = h.orderOptions(c.Request().Context(), q, customerID, orderID, limit)
	default:
		err = fmt.Errorf("unsupported type")
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"type":    contextType,
		"options": options,
		"limit":   limit,
	})
}

func normalizeViewContextOptionType(value string) string {
	switch strings.TrimSpace(value) {
	case viewContextCustomer, viewContextExternalCustomer:
		return viewContextCustomer
	case viewContextOrder:
		return viewContextOrder
	default:
		return ""
	}
}

func isExternalCustomerActor(actor authzapp.Actor) bool {
	return strings.TrimSpace(actor.AccountType) == AccountTypeChannelCustomer
}

func (h viewContextHandler) boundCustomerID(ctx context.Context, employeeID int64) (int64, error) {
	if employeeID <= 0 {
		return 0, fmt.Errorf("employee required")
	}
	var id int64
	err := h.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT b.customer_id
		FROM %s.customer_erp_user_bindings b
		JOIN %s.customers c ON c.id=b.customer_id AND c.active=true
		JOIN %s.company_employees e ON e.id=b.employee_id AND e.active=true AND e.account_type='channel_customer'
		WHERE b.employee_id=$1 AND b.status='active'
		ORDER BY b.id DESC
		LIMIT 1
	`, h.schema, h.schema, h.schema), employeeID).Scan(&id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, fmt.Errorf("bound customer not found")
		}
		return 0, err
	}
	return id, nil
}

func (h viewContextHandler) orderCustomerID(ctx context.Context, orderID int64) (int64, error) {
	if orderID <= 0 {
		return 0, fmt.Errorf("order required")
	}
	var id int64
	err := h.pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(customer_id,0) FROM %s.orders WHERE id=$1 AND COALESCE(is_void,false)=false LIMIT 1`, h.schema), orderID).Scan(&id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, fmt.Errorf("order not found")
		}
		return 0, err
	}
	return id, nil
}

func (h viewContextHandler) customerOptions(ctx context.Context, q string, customerID int64, limit int) ([]viewContextOption, error) {
	where := []string{"active=true"}
	args := []any{}
	argn := 1
	if customerID > 0 {
		where = append(where, fmt.Sprintf("id=$%d", argn))
		args = append(args, customerID)
		argn++
	}
	if q != "" {
		where = append(where, fmt.Sprintf("(name ILIKE $%d OR company_name ILIKE $%d OR contact ILIKE $%d OR phone ILIKE $%d)", argn, argn, argn, argn))
		args = append(args, "%"+q+"%")
		argn++
	}
	args = append(args, limit)
	sql := fmt.Sprintf(`
		SELECT id, COALESCE(name,''), COALESCE(company_name,''), COALESCE(contact,''), COALESCE(phone,'')
		FROM %s.customers
		WHERE %s
		ORDER BY id
		LIMIT $%d
	`, h.schema, strings.Join(where, " AND "), argn)
	rows, err := h.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]viewContextOption, 0)
	for rows.Next() {
		var id int64
		var name, companyName, contact, phone string
		if err := rows.Scan(&id, &name, &companyName, &contact, &phone); err != nil {
			return nil, err
		}
		label := strings.TrimSpace(name)
		if label == "" {
			label = fmt.Sprintf("客户 #%d", id)
		}
		out = append(out, viewContextOption{
			Type:         viewContextCustomer,
			ID:           id,
			Label:        label,
			CustomerID:   id,
			CustomerName: label,
			CompanyName:  companyName,
			Contact:      contact,
			Phone:        phone,
		})
	}
	return out, rows.Err()
}

func (h viewContextHandler) orderOptions(ctx context.Context, q string, customerID, orderID int64, limit int) ([]viewContextOption, error) {
	where := []string{"COALESCE(o.is_void,false)=false"}
	args := []any{}
	argn := 1
	if customerID > 0 {
		where = append(where, fmt.Sprintf("COALESCE(o.customer_id,0)=$%d", argn))
		args = append(args, customerID)
		argn++
	}
	if orderID > 0 {
		where = append(where, fmt.Sprintf("o.id=$%d", argn))
		args = append(args, orderID)
		argn++
	}
	if q != "" {
		where = append(where, fmt.Sprintf("(o.order_no ILIKE $%d OR c.name ILIKE $%d)", argn, argn))
		args = append(args, "%"+q+"%")
		argn++
	}
	args = append(args, limit)
	sql := fmt.Sprintf(`
		SELECT o.id, COALESCE(o.order_no,''), COALESCE(to_char(o.order_date,'YYYY-MM-DD'),''), COALESCE(o.customer_id,0), COALESCE(c.name,'')
		FROM %s.orders o
		LEFT JOIN %s.customers c ON c.id=o.customer_id
		WHERE %s
		ORDER BY o.order_date DESC NULLS LAST, o.id DESC
		LIMIT $%d
	`, h.schema, h.schema, strings.Join(where, " AND "), argn)
	rows, err := h.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]viewContextOption, 0)
	for rows.Next() {
		var id, rowCustomerID int64
		var orderNo, orderDate, customerName string
		if err := rows.Scan(&id, &orderNo, &orderDate, &rowCustomerID, &customerName); err != nil {
			return nil, err
		}
		orderLabel := strings.TrimSpace(orderNo)
		if orderLabel == "" {
			orderLabel = fmt.Sprintf("订单 #%d", id)
		}
		label := orderLabel
		if strings.TrimSpace(customerName) != "" {
			label = fmt.Sprintf("%s / %s", orderLabel, strings.TrimSpace(customerName))
		}
		out = append(out, viewContextOption{
			Type:         viewContextOrder,
			ID:           id,
			Label:        label,
			CustomerID:   rowCustomerID,
			CustomerName: strings.TrimSpace(customerName),
			OrderID:      id,
			OrderNo:      orderNo,
			OrderDate:    orderDate,
			Status:       "正常",
		})
	}
	return out, rows.Err()
}

func (h viewContextHandler) listPresets(c echo.Context) error {
	employeeID := CurrentEmployeeID(c)
	rows, err := h.pool.Query(c.Request().Context(), fmt.Sprintf(`
		SELECT id, name, context_type, context_json, menu_keys_json, owner_employee_id, active, sort_order,
		       to_char(created_at,'YYYY-MM-DD HH24:MI:SS'), to_char(updated_at,'YYYY-MM-DD HH24:MI:SS'), created_by, updated_by
		FROM %s.view_context_presets
		WHERE active=true AND (owner_employee_id=0 OR owner_employee_id=$1)
		ORDER BY sort_order, id
	`, h.schema), employeeID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	defer rows.Close()
	out := make([]viewContextPreset, 0)
	for rows.Next() {
		row, err := scanViewContextPreset(rows)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"presets": out})
}

func (h viewContextHandler) createPreset(c echo.Context) error {
	payload, err := bindViewContextPresetPayload(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	actor := ActorOf(c)
	employeeID := CurrentEmployeeID(c)
	row, err := h.insertPreset(c.Request().Context(), payload, employeeID, actor)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	AuditInsert(c.Request().Context(), h.pool, h.schema, actor, "view_context_preset", &row.ID, "create_view_context_preset", viewContextStrPtr("name"), nil, viewContextStrPtr(row.Name), AuditMeta{
		"context_type": row.ContextType,
		"context_json": string(row.ContextJSON),
	})
	return c.JSON(http.StatusOK, map[string]any{"preset": row})
}

func (h viewContextHandler) updatePreset(c echo.Context) error {
	id := pathIDParam(c, "id")
	if id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	payload, err := bindViewContextPresetPayload(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	actor := ActorOf(c)
	row, err := h.updatePresetRow(c.Request().Context(), id, payload, actor)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	AuditInsert(c.Request().Context(), h.pool, h.schema, actor, "view_context_preset", &row.ID, "update_view_context_preset", viewContextStrPtr("name"), nil, viewContextStrPtr(row.Name), AuditMeta{
		"context_type": row.ContextType,
		"context_json": string(row.ContextJSON),
	})
	return c.JSON(http.StatusOK, map[string]any{"preset": row})
}

func (h viewContextHandler) disablePreset(c echo.Context) error {
	id := pathIDParam(c, "id")
	if id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	actor := ActorOf(c)
	row, err := h.disablePresetRow(c.Request().Context(), id, actor)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	AuditInsert(c.Request().Context(), h.pool, h.schema, actor, "view_context_preset", &row.ID, "disable_view_context_preset", viewContextStrPtr("active"), viewContextStrPtr("true"), viewContextStrPtr("false"), AuditMeta{
		"context_type": row.ContextType,
	})
	return c.JSON(http.StatusOK, map[string]any{"preset": row})
}

func bindViewContextPresetPayload(c echo.Context) (viewContextPresetPayload, error) {
	var payload viewContextPresetPayload
	if err := c.Bind(&payload); err != nil {
		return payload, fmt.Errorf("invalid request")
	}
	payload.Name = strings.TrimSpace(payload.Name)
	payload.ContextType = normalizePresetContextType(payload.ContextType)
	if payload.Name == "" {
		return payload, fmt.Errorf("name required")
	}
	if payload.ContextType == "" {
		return payload, fmt.Errorf("context_type required")
	}
	if len(payload.ContextJSON) == 0 {
		payload.ContextJSON = json.RawMessage(`{}`)
	}
	if !json.Valid(payload.ContextJSON) {
		return payload, fmt.Errorf("invalid context_json")
	}
	if len(payload.MenuKeysJSON) == 0 {
		payload.MenuKeysJSON = json.RawMessage(`[]`)
	}
	if !json.Valid(payload.MenuKeysJSON) {
		return payload, fmt.Errorf("invalid menu_keys_json")
	}
	return payload, nil
}

func normalizePresetContextType(value string) string {
	switch strings.TrimSpace(value) {
	case viewContextFactory:
		return viewContextFactory
	case viewContextCustomer:
		return viewContextCustomer
	case viewContextOrder:
		return viewContextOrder
	case viewContextExternalCustomer:
		return viewContextExternalCustomer
	default:
		return ""
	}
}

func (h viewContextHandler) insertPreset(ctx context.Context, payload viewContextPresetPayload, employeeID int64, actor string) (viewContextPreset, error) {
	row := h.pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.view_context_presets(name, context_type, context_json, menu_keys_json, owner_employee_id, active, sort_order, created_by, updated_by)
		VALUES($1,$2,$3::jsonb,$4::jsonb,$5,true,$6,$7,$7)
		RETURNING id, name, context_type, context_json, menu_keys_json, owner_employee_id, active, sort_order,
		          to_char(created_at,'YYYY-MM-DD HH24:MI:SS'), to_char(updated_at,'YYYY-MM-DD HH24:MI:SS'), created_by, updated_by
	`, h.schema), payload.Name, payload.ContextType, string(payload.ContextJSON), string(payload.MenuKeysJSON), employeeID, payload.SortOrder, actor)
	return scanViewContextPreset(row)
}

func (h viewContextHandler) updatePresetRow(ctx context.Context, id int64, payload viewContextPresetPayload, actor string) (viewContextPreset, error) {
	row := h.pool.QueryRow(ctx, fmt.Sprintf(`
		UPDATE %s.view_context_presets
		SET name=$2, context_type=$3, context_json=$4::jsonb, menu_keys_json=$5::jsonb, sort_order=$6, updated_by=$7, updated_at=now()
		WHERE id=$1
		RETURNING id, name, context_type, context_json, menu_keys_json, owner_employee_id, active, sort_order,
		          to_char(created_at,'YYYY-MM-DD HH24:MI:SS'), to_char(updated_at,'YYYY-MM-DD HH24:MI:SS'), created_by, updated_by
	`, h.schema), id, payload.Name, payload.ContextType, string(payload.ContextJSON), string(payload.MenuKeysJSON), payload.SortOrder, actor)
	return scanViewContextPreset(row)
}

func (h viewContextHandler) disablePresetRow(ctx context.Context, id int64, actor string) (viewContextPreset, error) {
	row := h.pool.QueryRow(ctx, fmt.Sprintf(`
		UPDATE %s.view_context_presets
		SET active=false, updated_by=$2, updated_at=now()
		WHERE id=$1
		RETURNING id, name, context_type, context_json, menu_keys_json, owner_employee_id, active, sort_order,
		          to_char(created_at,'YYYY-MM-DD HH24:MI:SS'), to_char(updated_at,'YYYY-MM-DD HH24:MI:SS'), created_by, updated_by
	`, h.schema), id, actor)
	return scanViewContextPreset(row)
}

type viewContextPresetScanner interface {
	Scan(dest ...any) error
}

func scanViewContextPreset(row viewContextPresetScanner) (viewContextPreset, error) {
	var out viewContextPreset
	if err := row.Scan(
		&out.ID,
		&out.Name,
		&out.ContextType,
		&out.ContextJSON,
		&out.MenuKeysJSON,
		&out.OwnerEmployeeID,
		&out.Active,
		&out.SortOrder,
		&out.CreatedAt,
		&out.UpdatedAt,
		&out.CreatedBy,
		&out.UpdatedBy,
	); err != nil {
		return viewContextPreset{}, err
	}
	if len(out.ContextJSON) == 0 {
		out.ContextJSON = json.RawMessage(`{}`)
	}
	if len(out.MenuKeysJSON) == 0 {
		out.MenuKeysJSON = json.RawMessage(`[]`)
	}
	return out, nil
}

func viewContextStrPtr(value string) *string {
	return &value
}

func pathIDParam(c echo.Context, name string) int64 {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param(name)), 10, 64)
	if err != nil || id < 0 {
		return 0
	}
	return id
}
