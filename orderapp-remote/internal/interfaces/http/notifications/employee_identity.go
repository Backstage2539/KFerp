package notifications

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// A WeChat-bound employee notification remains valid only while the same
// unique verified number and explicit notification binding remain active.
// Phone changes revoke that binding transactionally.
func employeeNotificationAvailable(ctx context.Context, pool *pgxpool.Pool, schema string, employeeID, miniUserID int64, boundAt time.Time, appID, openID string) (bool, error) {
	var available bool
	err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.company_employees e LEFT JOIN %[1]s.employee_login_passwords p ON p.employee_id=e.id JOIN %[1]s.mini_users u ON u.id=$2 WHERE e.id=$1 AND e.active AND u.active AND e.account_type='internal_employee' AND NOT COALESCE(p.login_disabled,false) AND ((p.employee_id IS NOT NULL AND p.updated_at<=$3 AND u.openid='erp-internal-employee:'||e.id::text) OR (u.openid='wechat-internal-employee:'||e.id::text||':'||encode(sha256(convert_to(trim(e.phone),'UTF8')),'hex') AND EXISTS (SELECT 1 FROM %[1]s.wechat_bindings b WHERE b.mini_user_id=u.id AND b.employee_id=e.id AND b.active AND b.subscribed AND b.notifications_enabled AND b.bound_at=$3 AND b.app_id=$4 AND b.openid=$5) AND (SELECT count(*) FROM %[1]s.company_employees x WHERE x.active AND x.phone=e.phone AND x.account_type IN ('internal_employee','channel_customer'))=1)))`, schema), employeeID, miniUserID, boundAt, appID, openID).Scan(&available)
	return available, err
}
