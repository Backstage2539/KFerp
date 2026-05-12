package support

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

var cnPhoneRe = regexp.MustCompile(`^1\d{10}$`)

const (
	AccountTypeInternalEmployee = "internal_employee"
	AccountTypeChannelCustomer  = "channel_customer"
)

type loginReq struct {
	Mode     string `json:"mode"`
	Login    string `json:"login"`
	Username string `json:"username"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
	Code     string `json:"code"`
}

type smsSendReq struct {
	Phone string `json:"phone"`
}

type pwdSetReq struct {
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

type accountStateReq struct {
	EmployeeID   int64 `json:"employee_id"`
	LoginEnabled bool  `json:"login_enabled"`
}

type accountTypeReq struct {
	EmployeeID  int64  `json:"employee_id"`
	AccountType string `json:"account_type"`
}

type passwordResetReq struct {
	EmployeeID int64  `json:"employee_id"`
	Password   string `json:"password"`
}

func normalizeAccountType(value string) string {
	switch strings.TrimSpace(value) {
	case AccountTypeChannelCustomer:
		return AccountTypeChannelCustomer
	default:
		return AccountTypeInternalEmployee
	}
}

func ensureEmployeeByPhone(ctx context.Context, pool *pgxpool.Pool, schema, phone string) (int64, string, error) {
	phone = strings.TrimSpace(phone)
	if !cnPhoneRe.MatchString(phone) {
		return 0, "", fmt.Errorf("invalid phone")
	}
	var eid int64
	var ename string
	err := pool.QueryRow(ctx, "SELECT id,COALESCE(name,'') FROM "+schema+".company_employees WHERE phone=$1 LIMIT 1", phone).Scan(&eid, &ename)
	if err == nil {
		return eid, strings.TrimSpace(ename), nil
	}
	// auto-bind on first login: create employee by phone in first active department
	var depID int64
	if err := pool.QueryRow(ctx, "SELECT id FROM "+schema+".company_departments WHERE active=true ORDER BY id LIMIT 1").Scan(&depID); err != nil {
		return 0, "", fmt.Errorf("department not found")
	}
	if err := pool.QueryRow(ctx,
		"INSERT INTO "+schema+".company_employees(name,phone,department_id,active) VALUES($1,$2,$3,true) ON CONFLICT (phone) DO UPDATE SET active=true RETURNING id,COALESCE(name,'')",
		phone, phone, depID,
	).Scan(&eid, &ename); err != nil {
		return 0, "", err
	}
	return eid, strings.TrimSpace(ename), nil
}

func passwordLoginIdentifier(req loginReq) string {
	for _, raw := range []string{req.Login, req.Username, req.Phone} {
		if value := strings.TrimSpace(raw); value != "" {
			return value
		}
	}
	return ""
}

func resolveEmployeeByPasswordLogin(ctx context.Context, pool *pgxpool.Pool, schema, login string) (int64, string, string, error) {
	login = strings.TrimSpace(login)
	if login == "" {
		return 0, "", "", fmt.Errorf("login required")
	}
	var eid int64
	var ename, phone string
	var err error
	if cnPhoneRe.MatchString(login) {
		err = pool.QueryRow(ctx, "SELECT id,COALESCE(name,''),COALESCE(phone,'') FROM "+schema+".company_employees WHERE phone=$1 AND active=true LIMIT 1", login).Scan(&eid, &ename, &phone)
	} else {
		err = pool.QueryRow(ctx, "SELECT id,COALESCE(name,''),COALESCE(phone,'') FROM "+schema+".company_employees WHERE name=$1 AND active=true ORDER BY id LIMIT 1", login).Scan(&eid, &ename, &phone)
	}
	if err != nil {
		return 0, "", "", fmt.Errorf("employee not found")
	}
	return eid, strings.TrimSpace(ename), strings.TrimSpace(phone), nil
}

func hashPassword(raw string) string {
	s := sha256.Sum256([]byte("orderapp-mobile-auth:" + raw))
	return hex.EncodeToString(s[:])
}

func randToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func bearerTokenFromHeader(authz string) string {
	authz = strings.TrimSpace(authz)
	if !strings.HasPrefix(strings.ToLower(authz), "bearer ") {
		return ""
	}
	return strings.TrimSpace(authz[7:])
}

func ensureMobileAuthTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.employee_login_passwords (
	employee_id BIGINT PRIMARY KEY REFERENCES %s.company_employees(id) ON DELETE CASCADE,
	password_hash TEXT NOT NULL,
	login_disabled BOOLEAN NOT NULL DEFAULT false,
	must_reset_password BOOLEAN NOT NULL DEFAULT false,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS %s.login_sms_codes (
	id BIGSERIAL PRIMARY KEY,
	phone TEXT NOT NULL,
	code TEXT NOT NULL,
	expire_at TIMESTAMPTZ NOT NULL,
	used BOOLEAN NOT NULL DEFAULT false,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS login_sms_codes_phone_idx ON %s.login_sms_codes(phone, created_at DESC);
CREATE TABLE IF NOT EXISTS %s.login_sessions (
	token TEXT PRIMARY KEY,
	employee_id BIGINT NOT NULL REFERENCES %s.company_employees(id) ON DELETE CASCADE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	expire_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS login_sessions_employee_idx ON %s.login_sessions(employee_id, created_at DESC);
`, schema, schema, schema, schema, schema, schema, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	for _, stmt := range []string{
		`ALTER TABLE %[1]s.employee_login_passwords ADD COLUMN IF NOT EXISTS login_disabled BOOLEAN NOT NULL DEFAULT false`,
		`ALTER TABLE %[1]s.employee_login_passwords ADD COLUMN IF NOT EXISTS must_reset_password BOOLEAN NOT NULL DEFAULT false`,
	} {
		if _, err := pool.Exec(ctx, fmt.Sprintf(stmt, schema)); err != nil {
			return err
		}
	}
	return nil
}

func registerMobileAuthAPI(e *echo.Echo, pool *pgxpool.Pool, schema string, authz AuthzService) {
	e.POST("/api/auth/password/set", func(c echo.Context) error {
		var req pwdSetReq
		if err := c.Bind(&req); err != nil {
			return c.JSON(400, map[string]string{"error": "invalid request"})
		}
		phone := strings.TrimSpace(req.Phone)
		pwd := strings.TrimSpace(req.Password)
		if !cnPhoneRe.MatchString(phone) {
			return c.JSON(400, map[string]string{"error": "invalid phone"})
		}
		if len(pwd) < 6 {
			return c.JSON(400, map[string]string{"error": "password too short"})
		}
		eid, _, err := ensureEmployeeByPhone(c.Request().Context(), pool, schema, phone)
		if err != nil {
			return c.JSON(400, map[string]string{"error": err.Error()})
		}
		_, err = pool.Exec(c.Request().Context(),
			"INSERT INTO "+schema+".employee_login_passwords(employee_id,password_hash,login_disabled,must_reset_password,updated_at) VALUES($1,$2,false,false,now()) ON CONFLICT (employee_id) DO UPDATE SET password_hash=excluded.password_hash,login_disabled=false,must_reset_password=false,updated_at=now()",
			eid, hashPassword(pwd),
		)
		if err != nil {
			return c.JSON(500, map[string]string{"error": err.Error()})
		}
		return c.JSON(200, map[string]any{"ok": true})
	})

	e.POST("/api/auth/sms/send", func(c echo.Context) error {
		var req smsSendReq
		if err := c.Bind(&req); err != nil {
			return c.JSON(400, map[string]string{"error": "invalid request"})
		}
		phone := strings.TrimSpace(req.Phone)
		if !cnPhoneRe.MatchString(phone) {
			return c.JSON(400, map[string]string{"error": "invalid phone"})
		}
		_, _, err := ensureEmployeeByPhone(c.Request().Context(), pool, schema, phone)
		if err != nil {
			return c.JSON(400, map[string]string{"error": err.Error()})
		}
		// 默认频控：60 秒内仅 1 次；每天最多 10 次
		var secLeft int64
		_ = pool.QueryRow(c.Request().Context(),
			"SELECT CASE WHEN MAX(created_at) IS NULL THEN 0 ELSE GREATEST(0, 60-EXTRACT(EPOCH FROM (now()-MAX(created_at)))::bigint) END FROM "+schema+".login_sms_codes WHERE phone=$1",
			phone,
		).Scan(&secLeft)
		if secLeft > 0 {
			return c.JSON(429, map[string]any{"error": "sms cooldown", "retry_after_sec": secLeft})
		}
		var cntToday int64
		_ = pool.QueryRow(c.Request().Context(),
			"SELECT COUNT(*)::bigint FROM "+schema+".login_sms_codes WHERE phone=$1 AND created_at>=date_trunc('day', now())",
			phone,
		).Scan(&cntToday)
		if cntToday >= 10 {
			return c.JSON(429, map[string]any{"error": "sms daily limit"})
		}
		codeToken, err := randToken(3)
		if err != nil {
			return c.JSON(500, map[string]string{"error": "gen code failed"})
		}
		code := fmt.Sprintf("%06s", codeToken[:6])
		_, err = pool.Exec(c.Request().Context(), "INSERT INTO "+schema+".login_sms_codes(phone,code,expire_at,used) VALUES($1,$2,now()+interval '5 minutes',false)", phone, code)
		if err != nil {
			return c.JSON(500, map[string]string{"error": err.Error()})
		}
		// 短信通道待接入前，回显用于联调
		return c.JSON(200, map[string]any{"ok": true, "code": code, "expire_minutes": 5})
	})

	e.POST("/api/auth/login", func(c echo.Context) error {
		var req loginReq
		if err := c.Bind(&req); err != nil {
			return c.JSON(400, map[string]string{"error": "invalid request"})
		}
		mode := strings.TrimSpace(req.Mode)
		if mode == "" {
			mode = "password"
		}
		var eid int64
		var ename, phone, auditLogin string

		switch mode {
		case "password":
			login := passwordLoginIdentifier(req)
			if login == "" {
				return c.JSON(400, map[string]string{"error": "login required"})
			}
			var err error
			eid, ename, phone, err = resolveEmployeeByPasswordLogin(c.Request().Context(), pool, schema, login)
			if err != nil {
				return c.JSON(401, map[string]string{"error": "invalid login"})
			}
			auditLogin = login
			disabled, err := isLoginDisabled(c.Request().Context(), pool, schema, eid)
			if err != nil {
				return c.JSON(500, map[string]string{"error": err.Error()})
			}
			if disabled {
				return c.JSON(403, map[string]string{"error": "login disabled"})
			}
			pwd := strings.TrimSpace(req.Password)
			if pwd == "" {
				return c.JSON(400, map[string]string{"error": "password required"})
			}
			var ph string
			err = pool.QueryRow(c.Request().Context(), "SELECT password_hash FROM "+schema+".employee_login_passwords WHERE employee_id=$1 AND login_disabled=false", eid).Scan(&ph)
			if err != nil || ph != hashPassword(pwd) {
				return c.JSON(401, map[string]string{"error": "invalid password"})
			}
		case "sms":
			phone = strings.TrimSpace(req.Phone)
			if !cnPhoneRe.MatchString(phone) {
				return c.JSON(400, map[string]string{"error": "invalid phone"})
			}
			var err error
			eid, ename, err = ensureEmployeeByPhone(c.Request().Context(), pool, schema, phone)
			if err != nil {
				return c.JSON(400, map[string]string{"error": err.Error()})
			}
			auditLogin = phone
			disabled, err := isLoginDisabled(c.Request().Context(), pool, schema, eid)
			if err != nil {
				return c.JSON(500, map[string]string{"error": err.Error()})
			}
			if disabled {
				return c.JSON(403, map[string]string{"error": "login disabled"})
			}
			code := strings.TrimSpace(req.Code)
			if len(code) != 6 {
				return c.JSON(400, map[string]string{"error": "invalid code"})
			}
			tag, err := pool.Exec(c.Request().Context(),
				"UPDATE "+schema+".login_sms_codes SET used=true WHERE id=(SELECT id FROM "+schema+".login_sms_codes WHERE phone=$1 AND code=$2 AND used=false AND expire_at>now() ORDER BY id DESC LIMIT 1) AND used=false",
				phone, code,
			)
			if err != nil || tag.RowsAffected() == 0 {
				return c.JSON(401, map[string]string{"error": "invalid or expired code"})
			}
		default:
			return c.JSON(400, map[string]string{"error": "mode must be password or sms"})
		}

		token, err := randToken(24)
		if err != nil {
			return c.JSON(500, map[string]string{"error": "gen token failed"})
		}
		_, err = pool.Exec(c.Request().Context(), "INSERT INTO "+schema+".login_sessions(token,employee_id,expire_at) VALUES($1,$2,now()+interval '7 days')", token, eid)
		if err != nil {
			return c.JSON(500, map[string]string{"error": err.Error()})
		}
		c.Set("employee_id", eid)
		c.Set("operator_employee", strings.TrimSpace(ename))
		c.Set("actor", strings.TrimSpace(ename))
		AuditInsert(c.Request().Context(), pool, schema, strings.TrimSpace(ename), "auth", &eid, "login", nil, nil, nil, AuditMeta{"mode": mode, "phone": phone, "login": auditLogin})
		return c.JSON(200, map[string]any{
			"ok":       true,
			"token":    token,
			"employee": map[string]any{"id": eid, "name": ename, "phone": phone},
		})
	})

	e.POST("/api/auth/logout", func(c echo.Context) error {
		token := bearerTokenFromHeader(c.Request().Header.Get("Authorization"))
		if token != "" {
			if _, err := pool.Exec(c.Request().Context(), "DELETE FROM "+schema+".login_sessions WHERE token=$1", token); err != nil {
				return c.JSON(500, map[string]string{"error": err.Error()})
			}
		}
		return c.JSON(200, map[string]any{"ok": true})
	})

	e.GET("/api/auth/accounts", func(c echo.Context) error {
		if err := requireCurrentPermission(c, authz, "auth.manage"); err != nil {
			return err
		}
		rows, err := pool.Query(c.Request().Context(), fmt.Sprintf(`
			SELECT e.id,COALESCE(e.name,''),COALESCE(e.phone,''),COALESCE(NULLIF(e.account_type,''),'internal_employee'),COALESCE(d.name,''),
			       COALESCE(p.password_hash,'') <> '' AS has_password,
			       COALESCE(p.login_disabled,false) AS login_disabled,
			       COALESCE(p.must_reset_password,false) AS must_reset_password
			FROM %s.company_employees e
			LEFT JOIN %s.company_departments d ON d.id=e.department_id
			LEFT JOIN %s.employee_login_passwords p ON p.employee_id=e.id
			WHERE e.active=true
			ORDER BY e.id
		`, schema, schema, schema))
		if err != nil {
			return c.JSON(500, map[string]string{"error": err.Error()})
		}
		defer rows.Close()
		out := make([]map[string]any, 0)
		for rows.Next() {
			var id int64
			var name, phone, accountType, department string
			var hasPassword, loginDisabled, mustReset bool
			if err := rows.Scan(&id, &name, &phone, &accountType, &department, &hasPassword, &loginDisabled, &mustReset); err != nil {
				return c.JSON(500, map[string]string{"error": err.Error()})
			}
			out = append(out, map[string]any{
				"employee_id":         id,
				"name":                name,
				"phone":               phone,
				"account_type":        normalizeAccountType(accountType),
				"department":          department,
				"has_password":        hasPassword,
				"login_disabled":      loginDisabled,
				"must_reset_password": mustReset,
			})
		}
		if err := rows.Err(); err != nil {
			return c.JSON(500, map[string]string{"error": err.Error()})
		}
		return c.JSON(200, map[string]any{"rows": out})
	})

	e.POST("/api/auth/account-type", func(c echo.Context) error {
		if err := requireCurrentPermission(c, authz, "auth.manage"); err != nil {
			return err
		}
		var req accountTypeReq
		if err := c.Bind(&req); err != nil || req.EmployeeID <= 0 {
			return c.JSON(400, map[string]string{"error": "invalid request"})
		}
		accountType := normalizeAccountType(req.AccountType)
		tag, err := pool.Exec(c.Request().Context(), fmt.Sprintf(`
			UPDATE %s.company_employees
			SET account_type=$2, updated_at=now()
			WHERE id=$1 AND active=true
		`, schema), req.EmployeeID, accountType)
		if err != nil {
			return c.JSON(500, map[string]string{"error": err.Error()})
		}
		if tag.RowsAffected() == 0 {
			return c.JSON(404, map[string]string{"error": "employee not found"})
		}
		if accountType == AccountTypeChannelCustomer {
			if _, err := pool.Exec(c.Request().Context(), fmt.Sprintf(`DELETE FROM %s.employee_roles WHERE employee_id=$1`, schema), req.EmployeeID); err != nil {
				return c.JSON(500, map[string]string{"error": err.Error()})
			}
		}
		AuditInsert(c.Request().Context(), pool, schema, ActorOf(c), "auth_account", &req.EmployeeID, "set_account_type", nil, nil, nil, AuditMeta{"account_type": accountType})
		return c.JSON(200, map[string]any{"ok": true, "account_type": accountType})
	})

	e.POST("/api/auth/account-state", func(c echo.Context) error {
		if err := requireCurrentPermission(c, authz, "auth.manage"); err != nil {
			return err
		}
		var req accountStateReq
		if err := c.Bind(&req); err != nil || req.EmployeeID <= 0 {
			return c.JSON(400, map[string]string{"error": "invalid request"})
		}
		_, err := pool.Exec(c.Request().Context(), fmt.Sprintf(`
			INSERT INTO %s.employee_login_passwords(employee_id,password_hash,login_disabled,updated_at)
			VALUES($1,'',$2,now())
			ON CONFLICT (employee_id) DO UPDATE SET login_disabled=excluded.login_disabled,updated_at=now()
		`, schema), req.EmployeeID, !req.LoginEnabled)
		if err != nil {
			return c.JSON(500, map[string]string{"error": err.Error()})
		}
		AuditInsert(c.Request().Context(), pool, schema, ActorOf(c), "auth_account", &req.EmployeeID, "set_login_enabled", nil, nil, nil, AuditMeta{"login_enabled": req.LoginEnabled})
		return c.JSON(200, map[string]any{"ok": true})
	})

	e.POST("/api/auth/password/reset", func(c echo.Context) error {
		if err := requireCurrentPermission(c, authz, "auth.manage"); err != nil {
			return err
		}
		var req passwordResetReq
		if err := c.Bind(&req); err != nil || req.EmployeeID <= 0 {
			return c.JSON(400, map[string]string{"error": "invalid request"})
		}
		pwd := strings.TrimSpace(req.Password)
		if len(pwd) < 6 {
			return c.JSON(400, map[string]string{"error": "password too short"})
		}
		_, err := pool.Exec(c.Request().Context(), fmt.Sprintf(`
			INSERT INTO %s.employee_login_passwords(employee_id,password_hash,login_disabled,must_reset_password,updated_at)
			VALUES($1,$2,false,true,now())
			ON CONFLICT (employee_id) DO UPDATE SET password_hash=excluded.password_hash,login_disabled=false,must_reset_password=true,updated_at=now()
		`, schema), req.EmployeeID, hashPassword(pwd))
		if err != nil {
			return c.JSON(500, map[string]string{"error": err.Error()})
		}
		AuditInsert(c.Request().Context(), pool, schema, ActorOf(c), "auth_account", &req.EmployeeID, "reset_password", nil, nil, nil, AuditMeta{"employee_id": req.EmployeeID})
		return c.JSON(200, map[string]any{"ok": true})
	})
}

func isLoginDisabled(ctx context.Context, pool *pgxpool.Pool, schema string, employeeID int64) (bool, error) {
	var disabled bool
	err := pool.QueryRow(ctx, "SELECT COALESCE(login_disabled,false) FROM "+schema+".employee_login_passwords WHERE employee_id=$1", employeeID).Scan(&disabled)
	if err != nil {
		return false, nil
	}
	return disabled, nil
}
