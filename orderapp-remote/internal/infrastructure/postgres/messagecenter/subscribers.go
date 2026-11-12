package messagecenter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type NotificationSubscriber struct {
	MiniUserID             int64    `json:"mini_user_id"`
	CustomerID             int64    `json:"customer_id"`
	EmployeeID             int64    `json:"employee_id"`
	IdentityType           string   `json:"identity_type"`
	Name                   string   `json:"name"`
	Phone                  string   `json:"phone"`
	CustomerPhone          string   `json:"customer_phone"`
	AccountPhone           string   `json:"account_phone"`
	AccountName            string   `json:"account_name"`
	IdentityActive         bool     `json:"identity_active"`
	HasBinding             bool     `json:"has_binding"`
	BindingCount           int64    `json:"binding_count"`
	ActiveBindingCount     int64    `json:"active_binding_count"`
	SubscribedBindingCount int64    `json:"subscribed_binding_count"`
	EnabledBindingCount    int64    `json:"enabled_binding_count"`
	Active                 bool     `json:"active"`
	Subscribed             bool     `json:"subscribed"`
	NotificationsEnabled   bool     `json:"notifications_enabled"`
	ChoiceRecorded         bool     `json:"choice_recorded"`
	ChoiceEnabled          bool     `json:"choice_enabled"`
	Events                 []string `json:"events"`
	PromptRequired         bool     `json:"prompt_required"`
}

// Match the portal's live customer binding rules without changing a stale
// projected binding as a side effect of opening this read-only directory.
func customerIdentityEligibleSQL(schema, customer, mini string) string {
	return fmt.Sprintf(`EXISTS(SELECT 1 FROM %[1]s.customer_portal_user_bindings pb JOIN %[1]s.customers pc ON pc.id=pb.customer_id AND pc.active JOIN %[1]s.customer_portal_profiles pp ON pp.customer_id=pc.id AND pp.enabled JOIN %[1]s.mini_users pu ON pu.id=pb.mini_user_id AND pu.active WHERE pb.customer_id=%[2]s AND pb.mini_user_id=%[3]s AND pb.status='approved' AND (
	(COALESCE(pb.approved_by,'') NOT IN ('erp-password-login','phone-verify-login') AND COALESCE(pb.approved_by,'') NOT LIKE 'erp-password-login:%%' AND COALESCE(pb.approved_by,'') NOT LIKE 'phone-verify-login:%%') OR
	EXISTS(SELECT 1 FROM %[1]s.customer_erp_user_bindings eb JOIN %[1]s.company_employees ee ON ee.id=eb.employee_id LEFT JOIN %[1]s.employee_login_passwords ep ON ep.employee_id=ee.id WHERE eb.customer_id=pc.id AND eb.status='active' AND ee.active AND ee.account_type='channel_customer' AND NOT COALESCE(ep.login_disabled,false) AND (
	((pb.approved_by='erp-password-login' OR pb.approved_by='erp-password-login:'||ee.id::text) AND pu.openid='erp-employee:'||ee.id::text) OR
	(pb.approved_by='phone-verify-login:'||ee.id::text AND ee.phone=pu.phone AND (SELECT count(*) FROM %[1]s.company_employees pe WHERE pe.active AND pe.phone=pu.phone AND pe.account_type IN ('channel_customer','internal_employee'))=1)))))`, schema, customer, mini)
}

func employeeAccessSQL(schema, employee string) string {
	return fmt.Sprintf(`(EXISTS(SELECT 1 FROM %[1]s.employee_roles er WHERE er.employee_id=%[2]s AND er.role_code IN ('sales','admin')) AND EXISTS(SELECT 1 FROM %[1]s.employee_roles er JOIN %[1]s.auth_role_permissions rp ON rp.role_code=er.role_code WHERE er.employee_id=%[2]s AND rp.permission_code='orders.read') AND EXISTS(SELECT 1 FROM %[1]s.employee_roles er JOIN %[1]s.auth_role_permissions rp ON rp.role_code=er.role_code WHERE er.employee_id=%[2]s AND rp.permission_code='orders.write'))`, schema, employee)
}

// NotificationSubscribers starts with customer and employee records so a
// customer without a mini-program account remains visible to administrators.
// Associations come only from explicit identity relationships, never phone.
func (r Repository) NotificationSubscribers(ctx context.Context, appID, search string) ([]NotificationSubscriber, error) {
	q := fmt.Sprintf(`WITH identities AS (
	 SELECT b.customer_id,0::bigint employee_id,b.mini_user_id FROM %[1]s.customer_portal_user_bindings b WHERE b.status='approved'
	 UNION SELECT customer_id,employee_id,mini_user_id FROM %[1]s.message_notification_choices WHERE app_id=$1
	 UNION SELECT customer_id,employee_id,mini_user_id FROM %[1]s.wechat_bindings WHERE app_id=$1
	 UNION SELECT 0,e.id,u.id FROM %[1]s.company_employees e JOIN %[1]s.mini_users u ON u.openid='erp-internal-employee:'||e.id::text OR u.openid LIKE 'wechat-internal-employee:'||e.id::text||':%%'
	), base AS (
	 SELECT c.id customer_id,0::bigint employee_id,'customer'::text identity_type,c.name,COALESCE(c.phone,'') phone,c.active identity_active,COALESCE(i.mini_user_id,0) mini_user_id
	 FROM %[1]s.customers c LEFT JOIN identities i ON i.customer_id=c.id AND i.employee_id=0
	 UNION ALL
	 SELECT 0,e.id,'employee',e.name,COALESCE(e.phone,''),e.active,COALESCE(i.mini_user_id,0)
	 FROM %[1]s.company_employees e LEFT JOIN identities i ON i.employee_id=e.id AND i.customer_id=0 WHERE e.account_type='internal_employee'
	)
	SELECT b.mini_user_id,b.customer_id,b.employee_id,b.identity_type,b.name,b.phone,CASE WHEN b.identity_type='customer' THEN b.phone ELSE '' END,
	 COALESCE(u.phone,''),COALESCE(u.nickname,''),b.identity_active AND (b.mini_user_id=0 OR (COALESCE(u.active,false) AND CASE WHEN b.customer_id>0 THEN %[2]s ELSE %[3]s AND EXISTS(SELECT 1 FROM %[1]s.company_employees ve LEFT JOIN %[1]s.employee_login_passwords lp ON lp.employee_id=ve.id WHERE ve.id=b.employee_id AND ve.active AND ve.account_type='internal_employee' AND NOT COALESCE(lp.login_disabled,false) AND ((lp.employee_id IS NOT NULL AND u.openid='erp-internal-employee:'||ve.id::text) OR (u.openid='wechat-internal-employee:'||ve.id::text||':'||encode(sha256(convert_to(trim(ve.phone),'UTF8')),'hex') AND (SELECT count(*) FROM %[1]s.company_employees du WHERE du.active AND du.phone=ve.phone AND du.account_type IN ('internal_employee','channel_customer'))=1))) END)),COALESCE(w.present,false),COALESCE(w.active,false),COALESCE(w.subscribed,false),COALESCE(w.notifications_enabled,false),
	 COALESCE(w.binding_count,0),COALESCE(w.active_count,0),COALESCE(w.subscribed_count,0),COALESCE(w.enabled_count,0),
	 (c.version IS NOT NULL),COALESCE(c.enabled,false),COALESCE(c.events,'[]'::jsonb),
	 (b.mini_user_id>0 AND (c.version IS NULL OR c.version<p.version+COALESCE(i.revision,0)))
	FROM base b CROSS JOIN %[1]s.message_notification_prompt_settings p
	LEFT JOIN %[1]s.mini_users u ON u.id=b.mini_user_id
	LEFT JOIN %[1]s.message_notification_choices c ON c.app_id=$1 AND c.mini_user_id=b.mini_user_id AND c.customer_id=b.customer_id AND c.employee_id=b.employee_id
	LEFT JOIN %[1]s.message_notification_individual_prompts i ON i.app_id=$1 AND i.mini_user_id=b.mini_user_id AND i.customer_id=b.customer_id AND i.employee_id=b.employee_id
	LEFT JOIN LATERAL (SELECT count(*)>0 present,COALESCE(bool_or(w.active),false) active,COALESCE(bool_or(w.active AND w.subscribed),false) subscribed,COALESCE(bool_or(w.active AND w.subscribed AND w.notifications_enabled),false) notifications_enabled,count(*) binding_count,count(*) FILTER(WHERE w.active) active_count,count(*) FILTER(WHERE w.active AND w.subscribed) subscribed_count,count(*) FILTER(WHERE w.active AND w.subscribed AND w.notifications_enabled) enabled_count FROM %[1]s.wechat_bindings w WHERE w.app_id=$1 AND w.mini_user_id=b.mini_user_id AND w.customer_id=b.customer_id AND w.employee_id=b.employee_id) w ON true
	WHERE p.id=1 AND ($2='' OR b.name ILIKE '%%'||$2||'%%' OR b.phone ILIKE '%%'||$2||'%%' OR COALESCE(u.phone,'') ILIKE '%%'||$2||'%%' OR COALESCE(u.nickname,'') ILIKE '%%'||$2||'%%' OR b.mini_user_id::text=$2)
	ORDER BY b.identity_type,b.name,b.customer_id,b.employee_id,b.mini_user_id`, r.schema, customerIdentityEligibleSQL(r.schema, "b.customer_id", "b.mini_user_id"), employeeAccessSQL(r.schema, "b.employee_id"))
	rows, err := r.pool.Query(ctx, q, appID, strings.TrimSpace(search))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []NotificationSubscriber{}
	for rows.Next() {
		var v NotificationSubscriber
		var raw []byte
		if err = rows.Scan(&v.MiniUserID, &v.CustomerID, &v.EmployeeID, &v.IdentityType, &v.Name, &v.Phone, &v.CustomerPhone, &v.AccountPhone, &v.AccountName, &v.IdentityActive, &v.HasBinding, &v.Active, &v.Subscribed, &v.NotificationsEnabled, &v.BindingCount, &v.ActiveBindingCount, &v.SubscribedBindingCount, &v.EnabledBindingCount, &v.ChoiceRecorded, &v.ChoiceEnabled, &raw, &v.PromptRequired); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &v.Events); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r Repository) RequireIdentityNotificationReselect(ctx context.Context, actor, appID string, mini, customer, employee int64) error {
	if appID == "" || mini <= 0 || ((customer > 0) == (employee > 0)) {
		return errors.New("通知账号无效")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", mini); err != nil {
		return err
	}
	// Serialize with both versioned saves and global reselect.
	var global int64
	if err = tx.QueryRow(ctx, "SELECT version FROM "+r.schema+".message_notification_prompt_settings WHERE id=1 FOR UPDATE").Scan(&global); err != nil {
		return err
	}
	var valid bool
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.mini_users u WHERE $1::text<>'' AND u.id=$2 AND u.active AND (
	($3>0 AND %[2]s)
	OR ($4>0 AND %[3]s AND EXISTS(SELECT 1 FROM %[1]s.company_employees e LEFT JOIN %[1]s.employee_login_passwords lp ON lp.employee_id=e.id WHERE e.id=$4 AND e.active AND e.account_type='internal_employee' AND NOT COALESCE(lp.login_disabled,false) AND ((lp.employee_id IS NOT NULL AND u.openid='erp-internal-employee:'||e.id::text) OR (u.openid='wechat-internal-employee:'||e.id::text||':'||encode(sha256(convert_to(trim(e.phone),'UTF8')),'hex') AND (SELECT count(*) FROM %[1]s.company_employees x WHERE x.active AND x.phone=e.phone AND x.account_type IN ('internal_employee','channel_customer'))=1))))))`, r.schema, customerIdentityEligibleSQL(r.schema, "$3", "$2"), employeeAccessSQL(r.schema, "$4")), appID, mini, customer, employee).Scan(&valid)
	if err != nil {
		return err
	}
	if !valid {
		return errors.New("账号不存在或未关联该客户/员工")
	}
	var revision int64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %[1]s.message_notification_individual_prompts(app_id,mini_user_id,customer_id,employee_id,revision) VALUES($1,$2,$3,$4,1) ON CONFLICT(app_id,mini_user_id,customer_id,employee_id) DO UPDATE SET revision=%[1]s.message_notification_individual_prompts.revision+1 RETURNING revision`, r.schema), appID, mini, customer, employee).Scan(&revision); err != nil {
		return err
	}
	if err = r.externalAudit(ctx, tx, actor, mini, "notification_identity_reselect", "", fmt.Sprintf("customer=%d employee=%d version=%d", customer, employee, global+revision)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
