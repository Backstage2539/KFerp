package messagecenter

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	portal "orderapp/internal/application/customerportal"
)

// ReconcileWechatUnion uses live server-side sessions to restore an explicit
// notification selection when the matching follow arrives after login.
func ReconcileWechatUnion(ctx context.Context, pool *pgxpool.Pool, schema string, service *portal.Service, appID, unionID string) error {
	if pool == nil || service == nil || appID == "" || unionID == "" {
		return nil
	}
	rows, err := pool.Query(ctx, fmt.Sprintf(`SELECT DISTINCT s.token FROM %[1]s.mini_sessions s JOIN %[1]s.mini_users u ON u.id=s.mini_user_id
		WHERE u.active AND s.expire_at>now() AND (u.unionid=$1 OR EXISTS
		(SELECT 1 FROM %[1]s.mini_wechat_identities i WHERE i.mini_user_id=u.id AND i.unionid=$1))`, schema), unionID)
	if err != nil {
		return err
	}
	var tokens []string
	for rows.Next() {
		var token string
		if err = rows.Scan(&token); err != nil {
			rows.Close()
			return err
		}
		tokens = append(tokens, token)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	type identity struct{ mini, customer, employee int64 }
	identities := map[identity]string{}
	for _, token := range tokens {
		cur, e := service.Me(ctx, token)
		if e != nil || cur.MiniUserID <= 0 {
			continue
		}
		id := identity{mini: cur.MiniUserID, customer: cur.CurrentCustomerID}
		if cur.AccountType == "employee" {
			id.customer = 0
			id.employee = cur.EmployeeID
		}
		if (id.customer > 0) == (id.employee > 0) {
			continue
		}
		var selected bool
		err = pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM "+schema+".message_notification_choices WHERE app_id=$1 AND mini_user_id=$2 AND customer_id=$3 AND employee_id=$4 AND enabled)", appID, id.mini, id.customer, id.employee).Scan(&selected)
		if err != nil {
			return err
		}
		if selected {
			identities[id] = token
		}
	}
	if len(identities) != 1 {
		return nil
	}
	for id, token := range identities {
		err := NewRepository(pool, schema).AutoBindNotification(ctx, appID, id.mini, id.customer, id.employee, unionID, token)
		if errors.Is(err, ErrNotificationBindingConflict) || errors.Is(err, ErrNotificationIdentityChanged) {
			return nil
		}
		return err
	}
	return nil
}
