package customerportal

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	customerportalapp "orderapp/internal/application/customerportal"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool   *pgxpool.Pool
	schema string
}

type txQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

var passwordLoginPhoneRe = regexp.MustCompile(`^1\d{10}$`)

func NewRepository(pool *pgxpool.Pool, schema string) Repository {
	return Repository{pool: pool, schema: schema}
}

func (r Repository) CreateLoginSession(ctx context.Context, cmd customerportalapp.CreateLoginSessionCommand) (customerportalapp.LoginResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return customerportalapp.LoginResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	miniUserID, active, err := r.upsertMiniUserTx(ctx, tx, strings.TrimSpace(cmd.OpenID), strings.TrimSpace(cmd.UnionID), strings.TrimSpace(cmd.Phone), strings.TrimSpace(cmd.Nickname))
	if err != nil {
		return customerportalapp.LoginResult{}, err
	}
	if !active {
		return customerportalapp.LoginResult{}, customerportalapp.ErrMiniUserDisabled
	}
	result, err := r.createMiniSessionTx(ctx, tx, miniUserID, 0)
	if err != nil {
		return customerportalapp.LoginResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return customerportalapp.LoginResult{}, err
	}
	return result, nil
}

func (r Repository) CreatePhoneVerifiedLoginSession(ctx context.Context, cmd customerportalapp.CreatePhoneVerifiedLoginSessionCommand) (customerportalapp.LoginResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return customerportalapp.LoginResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	phone := strings.TrimSpace(cmd.Phone)
	if phone == "" {
		return customerportalapp.LoginResult{}, fmt.Errorf("phone required")
	}
	var employeeID int64
	var loginDisabled bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT e.id, COALESCE(p.login_disabled,false)
		FROM %s.company_employees e
		LEFT JOIN %s.employee_login_passwords p ON p.employee_id=e.id
		WHERE e.active=true
		  AND e.account_type='channel_customer'
		  AND e.phone=$1
		ORDER BY e.id
		LIMIT 1
	`, r.schema, r.schema), phone).Scan(&employeeID, &loginDisabled); err != nil {
		if err == pgx.ErrNoRows {
			return customerportalapp.LoginResult{}, customerportalapp.ErrMiniInvalidLogin
		}
		return customerportalapp.LoginResult{}, err
	}
	if loginDisabled {
		return customerportalapp.LoginResult{}, customerportalapp.ErrMiniAccountLoginDisabled
	}

	miniUserID, active, err := r.upsertMiniUserTx(ctx, tx, strings.TrimSpace(cmd.OpenID), strings.TrimSpace(cmd.UnionID), strings.TrimSpace(cmd.Phone), strings.TrimSpace(cmd.Nickname))
	if err != nil {
		return customerportalapp.LoginResult{}, err
	}
	if !active {
		return customerportalapp.LoginResult{}, customerportalapp.ErrMiniUserDisabled
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_portal_user_bindings(mini_user_id, customer_id, role, status, approved_by)
		SELECT $1, b.customer_id, COALESCE(NULLIF(b.role,''),'member'), 'approved', 'phone-verify-login'
		FROM %s.customer_erp_user_bindings b
		JOIN %s.customers c ON c.id=b.customer_id AND c.active=true
		JOIN %s.customer_portal_profiles p ON p.customer_id=b.customer_id AND p.enabled=true
		WHERE b.employee_id=$2 AND b.status='active'
		ON CONFLICT(mini_user_id, customer_id) DO UPDATE SET
			role=EXCLUDED.role,
			status='approved',
			approved_by='phone-verify-login'
	`, r.schema, r.schema, r.schema, r.schema), miniUserID, employeeID); err != nil {
		return customerportalapp.LoginResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.customer_portal_user_bindings
		SET status='revoked'
		WHERE mini_user_id=$1
		  AND customer_id NOT IN (
		    SELECT b.customer_id
		    FROM %s.customer_erp_user_bindings b
		    JOIN %s.customers c ON c.id=b.customer_id AND c.active=true
		    JOIN %s.customer_portal_profiles p ON p.customer_id=b.customer_id AND p.enabled=true
		    WHERE b.employee_id=$2 AND b.status='active'
		  )
	`, r.schema, r.schema, r.schema, r.schema), miniUserID, employeeID); err != nil {
		return customerportalapp.LoginResult{}, err
	}
	bindings, err := r.listBindingsTx(ctx, tx, miniUserID)
	if err != nil {
		return customerportalapp.LoginResult{}, err
	}
	if len(bindings) == 0 {
		return customerportalapp.LoginResult{}, customerportalapp.ErrCustomerBindingNotFound
	}
	result, err := r.createMiniSessionTx(ctx, tx, miniUserID, 0)
	if err != nil {
		return customerportalapp.LoginResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return customerportalapp.LoginResult{}, err
	}
	return result, nil
}

func (r Repository) upsertMiniUserTx(ctx context.Context, tx pgx.Tx, openID, unionID, phone, nickname string) (int64, bool, error) {
	if openID == "" {
		return 0, false, fmt.Errorf("openid required")
	}
	var miniUserID int64
	var active bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.mini_users(openid, unionid, phone, nickname, active, last_login_at)
		VALUES($1,$2,$3,$4,true,now())
		ON CONFLICT(openid) DO UPDATE SET
			unionid=CASE WHEN mini_users.active AND EXCLUDED.unionid<>'' THEN EXCLUDED.unionid ELSE mini_users.unionid END,
			phone=CASE WHEN mini_users.active AND EXCLUDED.phone<>'' THEN EXCLUDED.phone ELSE mini_users.phone END,
			nickname=CASE WHEN mini_users.active AND EXCLUDED.nickname<>'' THEN EXCLUDED.nickname ELSE mini_users.nickname END,
			last_login_at=CASE WHEN mini_users.active THEN now() ELSE mini_users.last_login_at END
		RETURNING id, active
	`, r.schema), openID, unionID, phone, nickname).Scan(&miniUserID, &active); err != nil {
		return 0, false, err
	}
	return miniUserID, active, nil
}

func (r Repository) createMiniSessionTx(ctx context.Context, tx pgx.Tx, miniUserID, preferredCustomerID int64) (customerportalapp.LoginResult, error) {
	bindings, err := r.listBindingsTx(ctx, tx, miniUserID)
	if err != nil {
		return customerportalapp.LoginResult{}, err
	}
	currentCustomerID := preferredCustomerID
	if currentCustomerID == 0 && len(bindings) > 0 {
		currentCustomerID = bindings[0].CustomerID
	}
	token, err := randomToken(24)
	if err != nil {
		return customerportalapp.LoginResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.mini_sessions(token, mini_user_id, current_customer_id, expire_at)
		VALUES($1,$2,NULLIF($3,0),'infinity'::timestamptz)
	`, r.schema), token, miniUserID, currentCustomerID); err != nil {
		return customerportalapp.LoginResult{}, err
	}
	capabilities, err := r.capabilitiesForCustomerTx(ctx, tx, currentCustomerID)
	if err != nil {
		return customerportalapp.LoginResult{}, err
	}
	themeKey, err := r.themeForCustomerTx(ctx, tx, currentCustomerID)
	if err != nil {
		return customerportalapp.LoginResult{}, err
	}
	entryMode, err := r.miniappEntryModeForCustomerTx(ctx, tx, currentCustomerID)
	if err != nil {
		return customerportalapp.LoginResult{}, err
	}
	return customerportalapp.LoginResult{
		Token:             token,
		MiniUserID:        miniUserID,
		CurrentCustomerID: currentCustomerID,
		Bindings:          bindings,
		Capabilities:      capabilities,
		ThemeKey:          themeKey,
		MiniappEntryMode:  entryMode,
	}, nil
}

func (r Repository) CreatePasswordLoginSession(ctx context.Context, cmd customerportalapp.CreatePasswordLoginSessionCommand) (customerportalapp.LoginResult, error) {
	login := strings.TrimSpace(cmd.Login)
	if login == "" {
		return customerportalapp.LoginResult{}, fmt.Errorf("login required")
	}
	password := strings.TrimSpace(cmd.Password)
	if password == "" {
		return customerportalapp.LoginResult{}, fmt.Errorf("password required")
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return customerportalapp.LoginResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	loginColumn := "e.name=$1"
	if passwordLoginPhoneRe.MatchString(login) {
		loginColumn = "e.phone=$1"
	}
	var employeeID int64
	var employeeName, employeePhone, accountType, passwordHash string
	var loginDisabled bool
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT e.id,
		       COALESCE(e.name,''),
		       COALESCE(e.phone,''),
		       COALESCE(NULLIF(e.account_type,''),'internal_employee'),
		       COALESCE(p.password_hash,''),
		       COALESCE(p.login_disabled,false)
		FROM %s.company_employees e
		LEFT JOIN %s.employee_login_passwords p ON p.employee_id=e.id
		WHERE e.active=true AND %s
		ORDER BY e.id
		LIMIT 1
	`, r.schema, r.schema, loginColumn), login).Scan(&employeeID, &employeeName, &employeePhone, &accountType, &passwordHash, &loginDisabled)
	if err == pgx.ErrNoRows {
		return customerportalapp.LoginResult{}, customerportalapp.ErrMiniInvalidLogin
	}
	if err != nil {
		return customerportalapp.LoginResult{}, err
	}
	if strings.TrimSpace(accountType) != "channel_customer" {
		if strings.TrimSpace(accountType) != "internal_employee" {
			return customerportalapp.LoginResult{}, customerportalapp.ErrCustomerBindingNotFound
		}
		if loginDisabled {
			return customerportalapp.LoginResult{}, customerportalapp.ErrMiniAccountLoginDisabled
		}
		if passwordHash == "" || passwordHash != erpPasswordHash(password) {
			return customerportalapp.LoginResult{}, customerportalapp.ErrMiniInvalidLogin
		}
		roles, permissions, err := r.employeeMiniAccessTx(ctx, tx, employeeID)
		if err != nil {
			return customerportalapp.LoginResult{}, err
		}
		if !containsString(roles, "admin") && !containsString(roles, "sales") {
			return customerportalapp.LoginResult{}, customerportalapp.ErrCustomerBindingNotFound
		}
		if !containsString(permissions, "orders.read") || !containsString(permissions, "orders.write") {
			return customerportalapp.LoginResult{}, customerportalapp.ErrCustomerBindingNotFound
		}
		miniUserID, err := r.upsertEmployeeMiniUserTx(ctx, tx, employeeID, employeeName, employeePhone)
		if err != nil {
			return customerportalapp.LoginResult{}, err
		}
		result, err := r.createMiniSessionTx(ctx, tx, miniUserID, 0)
		if err != nil {
			return customerportalapp.LoginResult{}, err
		}
		result.AccountType = "employee"
		result.EmployeeID = employeeID
		result.EmployeeName = strings.TrimSpace(employeeName)
		result.Roles = roles
		result.Permissions = permissions
		if err := tx.Commit(ctx); err != nil {
			return customerportalapp.LoginResult{}, err
		}
		return result, nil
	}
	if loginDisabled {
		return customerportalapp.LoginResult{}, customerportalapp.ErrMiniAccountLoginDisabled
	}
	if passwordHash == "" || passwordHash != erpPasswordHash(password) {
		return customerportalapp.LoginResult{}, customerportalapp.ErrMiniInvalidLogin
	}

	bindingRows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT b.customer_id
		FROM %s.customer_erp_user_bindings b
		JOIN %s.customers c ON c.id=b.customer_id AND c.active=true
		JOIN %s.customer_portal_profiles p ON p.customer_id=b.customer_id AND p.enabled=true
		WHERE b.employee_id=$1 AND b.status='active'
		ORDER BY b.id
	`, r.schema, r.schema, r.schema), employeeID)
	if err != nil {
		return customerportalapp.LoginResult{}, err
	}
	activeCustomerIDs := make([]int64, 0, 1)
	for bindingRows.Next() {
		var customerID int64
		if err := bindingRows.Scan(&customerID); err != nil {
			bindingRows.Close()
			return customerportalapp.LoginResult{}, err
		}
		activeCustomerIDs = append(activeCustomerIDs, customerID)
	}
	if err := bindingRows.Err(); err != nil {
		bindingRows.Close()
		return customerportalapp.LoginResult{}, err
	}
	bindingRows.Close()
	if len(activeCustomerIDs) == 0 {
		return customerportalapp.LoginResult{}, customerportalapp.ErrCustomerBindingNotFound
	}

	openID := fmt.Sprintf("erp-employee:%d", employeeID)
	nickname := strings.TrimSpace(employeeName)
	if nickname == "" {
		nickname = strings.TrimSpace(employeePhone)
	}
	var miniUserID int64
	var active bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.mini_users(openid, phone, nickname, active, last_login_at)
		VALUES($1,$2,$3,true,now())
		ON CONFLICT(openid) DO UPDATE SET
			phone=CASE WHEN mini_users.active AND EXCLUDED.phone<>'' THEN EXCLUDED.phone ELSE mini_users.phone END,
			nickname=CASE WHEN mini_users.active AND EXCLUDED.nickname<>'' THEN EXCLUDED.nickname ELSE mini_users.nickname END,
			last_login_at=CASE WHEN mini_users.active THEN now() ELSE mini_users.last_login_at END
		RETURNING id, active
	`, r.schema), openID, strings.TrimSpace(employeePhone), nickname).Scan(&miniUserID, &active); err != nil {
		return customerportalapp.LoginResult{}, err
	}
	if !active {
		return customerportalapp.LoginResult{}, customerportalapp.ErrMiniUserDisabled
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_portal_user_bindings(mini_user_id, customer_id, role, status, approved_by)
		SELECT $1, b.customer_id, COALESCE(NULLIF(b.role,''),'member'), 'approved', 'erp-password-login'
		FROM %s.customer_erp_user_bindings b
		JOIN %s.customers c ON c.id=b.customer_id AND c.active=true
		JOIN %s.customer_portal_profiles p ON p.customer_id=b.customer_id AND p.enabled=true
		WHERE b.employee_id=$2 AND b.status='active'
		ON CONFLICT(mini_user_id, customer_id) DO UPDATE SET
			role=EXCLUDED.role,
			status='approved',
			approved_by='erp-password-login'
	`, r.schema, r.schema, r.schema, r.schema), miniUserID, employeeID); err != nil {
		return customerportalapp.LoginResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.customer_portal_user_bindings
		SET status='revoked'
		WHERE mini_user_id=$1
		  AND customer_id NOT IN (
		    SELECT b.customer_id
		    FROM %s.customer_erp_user_bindings b
		    JOIN %s.customers c ON c.id=b.customer_id AND c.active=true
		    JOIN %s.customer_portal_profiles p ON p.customer_id=b.customer_id AND p.enabled=true
		    WHERE b.employee_id=$2 AND b.status='active'
		  )
	`, r.schema, r.schema, r.schema, r.schema), miniUserID, employeeID); err != nil {
		return customerportalapp.LoginResult{}, err
	}

	result, err := r.createMiniSessionTx(ctx, tx, miniUserID, 0)
	if err != nil {
		return customerportalapp.LoginResult{}, err
	}
	if len(result.Bindings) == 0 {
		return customerportalapp.LoginResult{}, customerportalapp.ErrCustomerBindingNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return customerportalapp.LoginResult{}, err
	}
	return result, nil
}

func (r Repository) CurrentContextByToken(ctx context.Context, token string) (customerportalapp.CurrentContext, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return customerportalapp.CurrentContext{}, customerportalapp.ErrMiniSessionNotFound
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return customerportalapp.CurrentContext{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var miniUserID, sessionCustomerID int64
	var openID string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT s.mini_user_id, COALESCE(s.current_customer_id,0), COALESCE(u.openid,'')
		FROM %s.mini_sessions s
		JOIN %s.mini_users u ON u.id=s.mini_user_id
		WHERE s.token=$1 AND s.expire_at>now() AND u.active=true
		FOR UPDATE OF s
	`, r.schema, r.schema), token).Scan(&miniUserID, &sessionCustomerID, &openID); err != nil {
		if err == pgx.ErrNoRows {
			return customerportalapp.CurrentContext{}, customerportalapp.ErrMiniSessionNotFound
		}
		return customerportalapp.CurrentContext{}, err
	}
	if strings.HasPrefix(openID, "erp-internal-employee:") {
		employeeID, parseErr := strconv.ParseInt(strings.TrimPrefix(openID, "erp-internal-employee:"), 10, 64)
		if parseErr != nil || employeeID <= 0 {
			return customerportalapp.CurrentContext{}, customerportalapp.ErrMiniSessionNotFound
		}
		var employeeName string
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(name,'') FROM %s.company_employees WHERE id=$1 AND active=true AND account_type='internal_employee'`, r.schema), employeeID).Scan(&employeeName); err != nil {
			return customerportalapp.CurrentContext{}, customerportalapp.ErrMiniSessionNotFound
		}
		roles, permissions, err := r.employeeMiniAccessTx(ctx, tx, employeeID)
		if err != nil {
			return customerportalapp.CurrentContext{}, err
		}
		if (!containsString(roles, "admin") && !containsString(roles, "sales")) || !containsString(permissions, "orders.read") || !containsString(permissions, "orders.write") {
			return customerportalapp.CurrentContext{}, customerportalapp.ErrCustomerBindingNotFound
		}
		if err := tx.Commit(ctx); err != nil {
			return customerportalapp.CurrentContext{}, err
		}
		return customerportalapp.CurrentContext{
			MiniUserID: miniUserID, AccountType: "employee", EmployeeID: employeeID,
			EmployeeName: employeeName, Roles: roles, Permissions: permissions,
			Bindings: []customerportalapp.CustomerBinding{}, Capabilities: []customerportalapp.Capability{},
			ThemeKey: customerportalapp.PortalThemeCleanOps, MiniappEntryMode: customerportalapp.MiniappEntryModeServices,
		}, nil
	}
	bindings, err := r.listBindingsTx(ctx, tx, miniUserID)
	if err != nil {
		return customerportalapp.CurrentContext{}, err
	}
	var currentCustomerID int64
	var currentCustomerName string
	for _, binding := range bindings {
		if binding.CustomerID == sessionCustomerID {
			currentCustomerID = binding.CustomerID
			currentCustomerName = binding.CustomerName
			break
		}
	}
	if currentCustomerID == 0 && len(bindings) > 0 {
		currentCustomerID = bindings[0].CustomerID
		currentCustomerName = bindings[0].CustomerName
	}
	if currentCustomerID != sessionCustomerID {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.mini_sessions
			SET current_customer_id=NULLIF($2,0)
			WHERE token=$1
		`, r.schema), token, currentCustomerID); err != nil {
			return customerportalapp.CurrentContext{}, err
		}
	}
	capabilities, err := r.capabilitiesForCustomerTx(ctx, tx, currentCustomerID)
	if err != nil {
		return customerportalapp.CurrentContext{}, err
	}
	themeKey, err := r.themeForCustomerTx(ctx, tx, currentCustomerID)
	if err != nil {
		return customerportalapp.CurrentContext{}, err
	}
	entryMode, err := r.miniappEntryModeForCustomerTx(ctx, tx, currentCustomerID)
	if err != nil {
		return customerportalapp.CurrentContext{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return customerportalapp.CurrentContext{}, err
	}
	return customerportalapp.CurrentContext{
		MiniUserID:          miniUserID,
		CurrentCustomerID:   currentCustomerID,
		CurrentCustomerName: currentCustomerName,
		Bindings:            bindings,
		Capabilities:        capabilities,
		ThemeKey:            themeKey,
		MiniappEntryMode:    entryMode,
	}, nil
}

func (r Repository) upsertEmployeeMiniUserTx(ctx context.Context, tx pgx.Tx, employeeID int64, name, phone string) (int64, error) {
	var miniUserID int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.mini_users(openid, phone, nickname, active, last_login_at)
		VALUES($1,$2,$3,true,now())
		ON CONFLICT(openid) DO UPDATE SET phone=EXCLUDED.phone,nickname=EXCLUDED.nickname,last_login_at=now()
		RETURNING id
	`, r.schema), fmt.Sprintf("erp-internal-employee:%d", employeeID), strings.TrimSpace(phone), strings.TrimSpace(name)).Scan(&miniUserID)
	return miniUserID, err
}

func (r Repository) employeeMiniAccessTx(ctx context.Context, q txQuerier, employeeID int64) ([]string, []string, error) {
	rows, err := q.Query(ctx, fmt.Sprintf(`
		SELECT DISTINCT er.role_code, rp.permission_code
		FROM %s.employee_roles er
		LEFT JOIN %s.auth_role_permissions rp ON rp.role_code=er.role_code
		WHERE er.employee_id=$1
		ORDER BY er.role_code, rp.permission_code
	`, r.schema, r.schema), employeeID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	roles, permissions := []string{}, []string{}
	for rows.Next() {
		var role string
		var permission *string
		if err := rows.Scan(&role, &permission); err != nil {
			return nil, nil, err
		}
		if !containsString(roles, role) {
			roles = append(roles, role)
		}
		if permission != nil && !containsString(permissions, *permission) {
			permissions = append(permissions, *permission)
		}
	}
	return roles, permissions, rows.Err()
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (r Repository) SwitchCurrentCustomer(ctx context.Context, token string, customerID int64) (customerportalapp.CurrentContext, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return customerportalapp.CurrentContext{}, customerportalapp.ErrMiniSessionNotFound
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return customerportalapp.CurrentContext{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var miniUserID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT s.mini_user_id
		FROM %s.mini_sessions s
		JOIN %s.mini_users u ON u.id=s.mini_user_id
		WHERE s.token=$1 AND s.expire_at>now() AND u.active=true
		FOR UPDATE OF s
	`, r.schema, r.schema), token).Scan(&miniUserID); err != nil {
		if err == pgx.ErrNoRows {
			return customerportalapp.CurrentContext{}, customerportalapp.ErrMiniSessionNotFound
		}
		return customerportalapp.CurrentContext{}, err
	}
	var bound int
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT 1
		FROM %s.customer_portal_user_bindings b
		JOIN %s.customers c ON c.id=b.customer_id AND c.active=true
		JOIN %s.customer_portal_profiles p ON p.customer_id=b.customer_id AND p.enabled=true
		WHERE b.mini_user_id=$1 AND b.customer_id=$2 AND b.status='approved'
	`, r.schema, r.schema, r.schema), miniUserID, customerID).Scan(&bound); err != nil {
		if err == pgx.ErrNoRows {
			return customerportalapp.CurrentContext{}, customerportalapp.ErrCustomerBindingNotFound
		}
		return customerportalapp.CurrentContext{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.mini_sessions
		SET current_customer_id=$2
		WHERE token=$1
	`, r.schema), token, customerID); err != nil {
		return customerportalapp.CurrentContext{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return customerportalapp.CurrentContext{}, err
	}
	return r.CurrentContextByToken(ctx, token)
}

func (r Repository) ListBindings(ctx context.Context, miniUserID int64) ([]customerportalapp.CustomerBinding, error) {
	return r.listBindingsTx(ctx, r.pool, miniUserID)
}

func (r Repository) CapabilitiesForCustomer(ctx context.Context, customerID int64) ([]customerportalapp.Capability, error) {
	return r.capabilitiesForCustomerTx(ctx, r.pool, customerID)
}

func (r Repository) listBindingsTx(ctx context.Context, q txQuerier, miniUserID int64) ([]customerportalapp.CustomerBinding, error) {
	rows, err := q.Query(ctx, fmt.Sprintf(`
		SELECT b.customer_id, COALESCE(NULLIF(p.display_name,''), c.name, ''), b.role, b.status
		FROM %s.customer_portal_user_bindings b
		JOIN %s.customers c ON c.id=b.customer_id
		JOIN %s.customer_portal_profiles p ON p.customer_id=b.customer_id AND p.enabled=true
		WHERE b.mini_user_id=$1 AND b.status='approved' AND c.active=true
		ORDER BY b.id
	`, r.schema, r.schema, r.schema), miniUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]customerportalapp.CustomerBinding, 0)
	for rows.Next() {
		var row customerportalapp.CustomerBinding
		if err := rows.Scan(&row.CustomerID, &row.CustomerName, &row.Role, &row.Status); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) capabilitiesForCustomerTx(ctx context.Context, q txQuerier, customerID int64) ([]customerportalapp.Capability, error) {
	if customerID <= 0 {
		return []customerportalapp.Capability{}, nil
	}
	if template, ok, err := r.capabilityTemplateForCustomerTx(ctx, q, customerID); err != nil {
		return nil, err
	} else if ok {
		return capabilityOptionsToCapabilities(template.Capabilities), nil
	}
	rows, err := q.Query(ctx, fmt.Sprintf(`
		SELECT capability_code, enabled, config_json
		FROM %s.customer_service_capabilities
		WHERE customer_id=$1
		ORDER BY capability_code
	`, r.schema), customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]customerportalapp.Capability, 0)
	for rows.Next() {
		var row customerportalapp.Capability
		var raw []byte
		if err := rows.Scan(&row.Code, &row.Enabled, &raw); err != nil {
			return nil, err
		}
		row.Config = map[string]any{}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &row.Config)
			if row.Config == nil {
				row.Config = map[string]any{}
			}
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) themeForCustomerTx(ctx context.Context, q txQuerier, customerID int64) (string, error) {
	if customerID <= 0 {
		return customerportalapp.PortalThemeCoffeeFactory, nil
	}
	if template, ok, err := r.capabilityTemplateForCustomerTx(ctx, q, customerID); err != nil {
		return "", err
	} else if ok {
		return customerportalapp.NormalizePortalThemeKey(template.ThemeKey), nil
	}
	var raw string
	err := q.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(NULLIF(theme_key,''),'coffee_factory')
		FROM %s.customer_portal_profiles
		WHERE customer_id=$1
	`, r.schema), customerID).Scan(&raw)
	if err == pgx.ErrNoRows {
		return customerportalapp.PortalThemeCoffeeFactory, nil
	}
	if err != nil {
		return "", err
	}
	return customerportalapp.NormalizePortalThemeKey(raw), nil
}

func (r Repository) miniappEntryModeForCustomerTx(ctx context.Context, q txQuerier, customerID int64) (string, error) {
	if customerID <= 0 {
		return customerportalapp.MiniappEntryModeServices, nil
	}
	if template, ok, err := r.capabilityTemplateForCustomerTx(ctx, q, customerID); err != nil {
		return "", err
	} else if ok {
		return customerportalapp.NormalizeMiniappEntryMode(template.MiniappEntryMode), nil
	}
	var raw string
	err := q.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(NULLIF(miniapp_entry_mode,''),'services')
		FROM %s.customer_portal_profiles
		WHERE customer_id=$1
	`, r.schema), customerID).Scan(&raw)
	if err == pgx.ErrNoRows {
		return customerportalapp.MiniappEntryModeServices, nil
	}
	if err != nil {
		return "", err
	}
	return customerportalapp.NormalizeMiniappEntryMode(raw), nil
}

func (r Repository) capabilityTemplateForCustomerTx(ctx context.Context, q txQuerier, customerID int64) (customerportalapp.CapabilityTemplate, bool, error) {
	var raw string
	err := q.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(capability_template_key,'')
		FROM %s.customer_portal_profiles
		WHERE customer_id=$1 AND enabled=true
	`, r.schema), customerID).Scan(&raw)
	if err == pgx.ErrNoRows {
		return customerportalapp.CapabilityTemplate{}, false, nil
	}
	if err != nil {
		return customerportalapp.CapabilityTemplate{}, false, err
	}
	key := customerportalapp.NormalizeCapabilityTemplateKey(raw)
	if strings.TrimSpace(raw) != "" && key == "" {
		return customerportalapp.CapabilityTemplate{}, false, customerportalapp.ErrCapabilityTemplateInvalid
	}
	if key == "" {
		return customerportalapp.CapabilityTemplate{}, false, nil
	}
	return r.capabilityTemplateForKeyTx(ctx, q, key)
}

func (r Repository) capabilityTemplateForKeyTx(ctx context.Context, q txQuerier, key string) (customerportalapp.CapabilityTemplate, bool, error) {
	key = customerportalapp.NormalizeCapabilityTemplateKey(key)
	if key == "" {
		return customerportalapp.CapabilityTemplate{}, false, customerportalapp.ErrCapabilityTemplateInvalid
	}
	row := q.QueryRow(ctx, fmt.Sprintf(`
		SELECT template_key,
		       parent_template_key,
		       label,
		       description,
		       theme_key,
		       miniapp_entry_mode,
		       erp_role_codes,
		       erp_permissions,
		       erp_view_keys,
		       capabilities_json,
		       active,
		       sort_order,
		       to_char(updated_at,'YYYY-MM-DD HH24:MI'),
		       updated_by
		FROM %s.customer_capability_templates
		WHERE template_key=$1
	`, r.schema), key)
	template, err := scanCapabilityTemplate(row)
	if err == nil {
		if !template.Active {
			return customerportalapp.CapabilityTemplate{}, false, customerportalapp.ErrCapabilityTemplateInvalid
		}
		return template, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return customerportalapp.CapabilityTemplate{}, false, err
	}
	if template, ok := customerportalapp.CustomerCapabilityTemplateByKey(key); ok && template.Active {
		return template, true, nil
	}
	return customerportalapp.CapabilityTemplate{}, false, customerportalapp.ErrCapabilityTemplateInvalid
}

func capabilityOptionsToCapabilities(options []customerportalapp.CapabilityOption) []customerportalapp.Capability {
	out := make([]customerportalapp.Capability, 0, len(options))
	for _, option := range options {
		config := map[string]any{}
		for key, value := range option.Config {
			config[key] = value
		}
		out = append(out, customerportalapp.Capability{
			Code:    strings.TrimSpace(option.Code),
			Enabled: option.Enabled,
			Config:  config,
		})
	}
	return out
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func erpPasswordHash(raw string) string {
	sum := sha256.Sum256([]byte("orderapp-mobile-auth:" + raw))
	return hex.EncodeToString(sum[:])
}
