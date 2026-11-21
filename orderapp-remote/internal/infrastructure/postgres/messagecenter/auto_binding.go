package messagecenter

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

var ErrNotificationBindingConflict = errors.New("微信身份已关联其他账号，请先处理绑定冲突")

func (r Repository) NotificationUnionID(ctx context.Context, mini, employee int64) (string, error) {
	var value string
	if employee > 0 {
		var count int
		err := r.pool.QueryRow(ctx, "SELECT count(DISTINCT unionid),COALESCE(min(unionid),'') FROM "+r.schema+".mini_wechat_identities WHERE mini_user_id=$1 AND unionid<>''", mini).Scan(&count, &value)
		if err != nil {
			return "", err
		}
		if count > 1 {
			return "", ErrNotificationBindingConflict
		}
		if count == 0 {
			return "", nil
		}
		return value, nil
	}
	err := r.pool.QueryRow(ctx, "SELECT unionid FROM "+r.schema+".mini_users WHERE id=$1 AND active", mini).Scan(&value)
	return value, err
}

// UpsertOfficialFollower records a server-side WeChat observation. Historical
// scans must pass their start time so a later unsubscribe always takes priority.
func (r Repository) UpsertOfficialFollower(ctx context.Context, appID, openID, unionID string, subscribed bool, eventTimestamp int64) error {
	if appID == "" || openID == "" || eventTimestamp <= 0 {
		return errors.New("invalid follower observation")
	}
	_, err := r.pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.message_wechat_followers(app_id,openid,unionid,subscribed,event_time)
		VALUES($1,$2,$3,$4,$5) ON CONFLICT(app_id,openid) DO UPDATE SET
		unionid=CASE WHEN excluded.unionid<>'' THEN excluded.unionid ELSE %[1]s.message_wechat_followers.unionid END,
		subscribed=excluded.subscribed,event_time=excluded.event_time
		WHERE excluded.event_time > %[1]s.message_wechat_followers.event_time OR
		(excluded.event_time = %[1]s.message_wechat_followers.event_time AND excluded.subscribed AND %[1]s.message_wechat_followers.subscribed)`, r.schema), appID, openID, unionID, subscribed, eventTimestamp)
	return err
}

// AutoBindNotification links only the identity of an already authenticated
// customer or employee. It never overwrites another active identity.
func (r Repository) AutoBindNotification(ctx context.Context, appID string, mini, customer, employee int64, unionID string, sessionTokens ...string) error {
	if unionID == "" || appID == "" || mini <= 0 || ((customer > 0) == (employee > 0)) {
		return nil
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if len(sessionTokens) > 0 {
		if err = lockNotificationSession(ctx, tx, r.schema, sessionTokens[0], mini, customer); err != nil {
			return err
		}
		if err = lockAutoBindingIdentity(ctx, tx, r.schema, mini, customer, employee); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", mini); err != nil {
		return err
	}
	var count int
	if err = tx.QueryRow(ctx, fmt.Sprintf(`WITH users AS (
		SELECT id FROM %[1]s.mini_users WHERE active AND unionid=$1
		UNION SELECT u.id FROM %[1]s.mini_wechat_identities i JOIN %[1]s.mini_users u ON u.id=i.mini_user_id WHERE u.active AND i.unionid=$1
	), identities AS (
		SELECT mini_user_id,customer_id,employee_id FROM %[1]s.message_notification_choices WHERE app_id=$2 AND enabled AND mini_user_id IN (SELECT id FROM users)
		UNION SELECT mini_user_id,customer_id,employee_id FROM %[1]s.wechat_bindings WHERE app_id=$2 AND active AND mini_user_id IN (SELECT id FROM users)
		UNION SELECT $3::bigint,$4::bigint,$5::bigint WHERE $3 IN (SELECT id FROM users)
	) SELECT count(*) FROM identities`, r.schema), unionID, appID, mini, customer, employee).Scan(&count); err != nil {
		return err
	}
	if count != 1 {
		return ErrNotificationBindingConflict
	}
	var openID string
	if err = tx.QueryRow(ctx, "SELECT count(*),COALESCE(min(openid),'') FROM "+r.schema+".message_wechat_followers WHERE app_id=$1 AND unionid=$2 AND subscribed", appID, unionID).Scan(&count, &openID); err != nil {
		return err
	}
	if count > 1 {
		return ErrNotificationBindingConflict
	}
	if count == 0 {
		return nil
	}
	var subscribed bool
	var timestamp int64
	if err = tx.QueryRow(ctx, "SELECT subscribed,event_time FROM "+r.schema+".message_wechat_followers WHERE app_id=$1 AND openid=$2 FOR UPDATE", appID, openID).Scan(&subscribed, &timestamp); err != nil {
		return err
	}
	if !subscribed {
		return nil
	}
	if _, err = tx.Exec(ctx, "INSERT INTO "+r.schema+".message_notification_subscription_events(app_id,openid) VALUES($1,$2) ON CONFLICT DO NOTHING", appID, openID); err != nil {
		return err
	}
	var lastUnsubscribe int64
	if err = tx.QueryRow(ctx, "SELECT last_unsubscribe FROM "+r.schema+".message_notification_subscription_events WHERE app_id=$1 AND openid=$2 FOR UPDATE", appID, openID).Scan(&lastUnsubscribe); err != nil {
		return err
	}
	if timestamp <= lastUnsubscribe {
		return nil
	}
	var previousMini, previousCustomer, previousEmployee int64
	var active, wasSubscribed bool
	err = tx.QueryRow(ctx, "SELECT mini_user_id,customer_id,employee_id,active,subscribed FROM "+r.schema+".wechat_bindings WHERE app_id=$1 AND openid=$2 FOR UPDATE", appID, openID).Scan(&previousMini, &previousCustomer, &previousEmployee, &active, &wasSubscribed)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if err == nil && active && (previousMini != mini || previousCustomer != customer || previousEmployee != employee) {
		return ErrNotificationBindingConflict
	}
	if err == nil && (previousMini != mini || previousCustomer != customer || previousEmployee != employee) {
		return ErrNotificationBindingConflict
	}
	if err == nil && active && wasSubscribed {
		return tx.Commit(ctx)
	}
	// An explicit choice is the only source of permission to send messages.
	_, enabled, events, err := selectedNotificationChoice(ctx, tx, r.schema, appID, mini, customer, employee)
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.wechat_bindings(app_id,openid,mini_user_id,customer_id,employee_id,active,subscribed,notifications_enabled,notification_events,bound_at,notification_consent_at,notification_subscription_time)
		VALUES($1,$2,$3,$4,$5,true,true,$6,$7,now(),CASE WHEN $6 THEN now() END,$8)
		ON CONFLICT(app_id,openid) DO UPDATE SET active=true,subscribed=true,notifications_enabled=excluded.notifications_enabled,notification_events=excluded.notification_events,notification_consent_at=excluded.notification_consent_at,notification_subscription_time=excluded.notification_subscription_time,bound_at=now()
		WHERE %[1]s.wechat_bindings.mini_user_id=excluded.mini_user_id AND %[1]s.wechat_bindings.customer_id=excluded.customer_id AND %[1]s.wechat_bindings.employee_id=excluded.employee_id`, r.schema), appID, openID, mini, customer, employee, enabled, events, timestamp)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrNotificationBindingConflict
	}
	if err = r.externalAudit(ctx, tx, fmt.Sprint("mini-user:", mini), mini, "notification_auto_bind", "", "自动识别服务号关注"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func lockAutoBindingIdentity(ctx context.Context, tx pgx.Tx, schema string, mini, customer, employee int64) error {
	var valid bool
	if customer > 0 {
		// Lock existing authorization rows before checking the complete projection.
		_, err := tx.Exec(ctx, fmt.Sprintf(`SELECT b.customer_id FROM %[1]s.customer_portal_user_bindings b
			JOIN %[1]s.customers c ON c.id=b.customer_id JOIN %[1]s.customer_portal_profiles p ON p.customer_id=c.id
			WHERE b.mini_user_id=$1 AND b.customer_id=$2 FOR SHARE OF b,c,p`, schema), mini, customer)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, fmt.Sprintf(`SELECT b.employee_id FROM %[1]s.customer_erp_user_bindings b JOIN %[1]s.company_employees e ON e.id=b.employee_id WHERE b.customer_id=$1 FOR SHARE OF b,e`, schema), customer)
		if err != nil {
			return err
		}
		err = tx.QueryRow(ctx, "SELECT "+customerIdentityEligibleSQL(schema, "$1", "$2"), customer, mini).Scan(&valid)
		if err != nil {
			return err
		}
	} else {
		_, err := tx.Exec(ctx, "SELECT id FROM "+schema+".company_employees WHERE id=$1 FOR SHARE", employee)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, "SELECT employee_id FROM "+schema+".employee_roles WHERE employee_id=$1 FOR SHARE", employee)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, "SELECT employee_id FROM "+schema+".employee_login_passwords WHERE employee_id=$1 FOR SHARE", employee)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, "SELECT rp.role_code FROM "+schema+".auth_role_permissions rp WHERE rp.role_code IN (SELECT role_code FROM "+schema+".employee_roles WHERE employee_id=$1) FOR SHARE", employee)
		if err != nil {
			return err
		}
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.mini_users u JOIN %[1]s.company_employees e ON e.id=$2
			LEFT JOIN %[1]s.employee_login_passwords lp ON lp.employee_id=e.id
			WHERE u.id=$1 AND u.active AND e.active AND e.account_type='internal_employee' AND NOT COALESCE(lp.login_disabled,false)
			AND ((lp.employee_id IS NOT NULL AND u.openid='erp-internal-employee:'||e.id::text)
			OR (u.openid='wechat-internal-employee:'||e.id::text||':'||encode(sha256(convert_to(trim(e.phone),'UTF8')),'hex')
			AND (SELECT count(*) FROM %[1]s.company_employees x WHERE x.active AND x.phone=e.phone AND x.account_type IN ('internal_employee','channel_customer'))=1)) AND %[2]s)`, schema, employeeAccessSQL(schema, "$2")), mini, employee).Scan(&valid)
		if err != nil {
			return err
		}
	}
	if !valid {
		return ErrNotificationIdentityChanged
	}
	return nil
}
