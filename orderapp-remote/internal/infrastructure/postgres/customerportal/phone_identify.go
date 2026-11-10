package customerportal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/customerportal"
	postgresinfra "orderapp/internal/infrastructure/postgres"
)

const wechatEmployeePrefix = "wechat-internal-employee:"

func verifiedPhoneDigest(phone string) string {
	h := sha256.Sum256([]byte(strings.TrimSpace(phone)))
	return hex.EncodeToString(h[:])
}

// Only called after the server has exchanged the one-use WeChat phone code.
func (r Repository) identifyWechatEmployeeTx(ctx context.Context, tx pgx.Tx, cmd app.CreatePhoneVerifiedLoginSessionCommand) (app.LoginResult, bool, error) {
	var count int
	var id int64
	var kind, name string
	var disabled bool
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT count(*),COALESCE(min(e.id),0),COALESCE(min(e.account_type),''),COALESCE(min(e.name),''),COALESCE(bool_or(COALESCE(lp.login_disabled,false)),false) FROM %[1]s.company_employees e LEFT JOIN %[1]s.employee_login_passwords lp ON lp.employee_id=e.id WHERE e.active AND e.phone=$1 AND e.account_type IN ('internal_employee','channel_customer')`, r.schema), strings.TrimSpace(cmd.Phone)).Scan(&count, &id, &kind, &name, &disabled)
	if err != nil {
		return app.LoginResult{}, false, err
	}
	if count > 1 {
		return app.LoginResult{}, false, app.ErrMiniInvalidLogin
	}
	if count == 0 {
		return app.LoginResult{}, false, nil
	}
	if disabled {
		return app.LoginResult{}, false, app.ErrMiniAccountLoginDisabled
	}
	if kind != "internal_employee" {
		return app.LoginResult{}, false, nil
	}
	if len(cmd.CredentialHash) != 64 {
		return app.LoginResult{}, false, app.ErrMiniInvalidLogin
	}
	roles, perms, err := r.employeeMiniAccessTx(ctx, tx, id)
	if err != nil {
		return app.LoginResult{}, false, err
	}
	if (!containsString(roles, "admin") && !containsString(roles, "sales")) || !containsString(perms, "orders.read") || !containsString(perms, "orders.write") {
		return app.LoginResult{}, false, app.ErrCustomerBindingNotFound
	}
	tag, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.mini_phone_credentials(credential_hash) VALUES($1) ON CONFLICT DO NOTHING`, r.schema), cmd.CredentialHash)
	if err != nil {
		return app.LoginResult{}, false, err
	}
	if tag.RowsAffected() != 1 {
		return app.LoginResult{}, false, app.ErrPhoneCredentialUsed
	}
	openid := fmt.Sprintf("%s%d:%s", wechatEmployeePrefix, id, verifiedPhoneDigest(cmd.Phone))
	miniID, active, err := r.upsertMiniUserTx(ctx, tx, openid, "", cmd.Phone, name)
	if err != nil {
		return app.LoginResult{}, false, err
	}
	if !active {
		return app.LoginResult{}, false, app.ErrMiniUserDisabled
	}
	result, err := r.createMiniSessionTx(ctx, tx, miniID, 0)
	if err != nil {
		return app.LoginResult{}, false, err
	}
	result.AccountType = "employee"
	result.EmployeeID = id
	result.EmployeeName = name
	result.Roles = roles
	result.Permissions = perms
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, fmt.Sprintf("mini-user:%d", miniID), "mini_user", &miniID, "phone_employee_login", nil, nil, nil, postgresinfra.AuditMeta{"employee_id": id}); err != nil {
		return app.LoginResult{}, false, err
	}
	return result, true, nil
}
