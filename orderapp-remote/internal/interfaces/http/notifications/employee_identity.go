package notifications

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// A WeChat-bound employee notification remains valid only while the same
// unique verified number is current and the employee record has not changed
// since binding. The latter prevents a changed-away-and-back phone from
// reviving a previously bound notification recipient.
func employeeNotificationAvailable(ctx context.Context, pool *pgxpool.Pool, schema string, employeeID, miniUserID int64, boundAt time.Time) (bool, error) {
	var available bool
	err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.company_employees e LEFT JOIN %[1]s.employee_login_passwords p ON p.employee_id=e.id JOIN %[1]s.mini_users u ON u.id=$2 WHERE e.id=$1 AND e.active AND u.active AND e.account_type='internal_employee' AND NOT COALESCE(p.login_disabled,false) AND ((p.employee_id IS NOT NULL AND p.updated_at<=$3 AND u.openid='erp-internal-employee:'||e.id::text) OR (u.openid='wechat-internal-employee:'||e.id::text||':'||encode(sha256(convert_to(trim(e.phone),'UTF8')),'hex') AND e.updated_at<=$3 AND (SELECT count(*) FROM %[1]s.company_employees x WHERE x.active AND x.phone=e.phone AND x.account_type IN ('internal_employee','channel_customer'))=1)))`, schema), employeeID, miniUserID, boundAt).Scan(&available)
	return available, err
}
