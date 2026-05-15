package messagecenter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	app "orderapp/internal/application/messagecenter"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool   *pgxpool.Pool
	schema string
}

func NewRepository(pool *pgxpool.Pool, schema string) Repository {
	return Repository{pool: pool, schema: schema}
}

func (r Repository) Publish(ctx context.Context, cmd app.PublishCommand) (int64, error) {
	payload, err := json.Marshal(cmd.Payload)
	if err != nil {
		return 0, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var eventID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.message_events(event_key, topic, event_type, source_type, source_id, actor, title, body, tone, payload_json)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb)
		ON CONFLICT(event_key) WHERE event_key <> '' DO UPDATE SET
			title=excluded.title,
			body=excluded.body,
			tone=excluded.tone,
			payload_json=excluded.payload_json
		RETURNING id
	`, r.schema),
		cmd.EventKey,
		cmd.Topic,
		cmd.EventType,
		cmd.SourceType,
		cmd.SourceID,
		cmd.Actor,
		cmd.Title,
		cmd.Body,
		cmd.Tone,
		string(payload),
	).Scan(&eventID); err != nil {
		return 0, err
	}

	for _, delivery := range cmd.Deliveries {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.message_deliveries(event_id, channel, target_type, target_key, target_employee_id, template_key, adapter_key, status)
			VALUES($1,$2,$3,$4,$5,$6,$7,'ready')
			ON CONFLICT(event_id, channel, target_type, target_key, target_employee_id) DO NOTHING
		`, r.schema),
			eventID,
			delivery.Channel,
			delivery.TargetType,
			delivery.TargetKey,
			delivery.TargetEmployeeID,
			delivery.TemplateKey,
			delivery.AdapterKey,
		); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return eventID, nil
}

func (r Repository) ListNotifications(ctx context.Context, query app.NotificationQuery) ([]app.Notification, error) {
	where := []string{"d.channel=$1", "d.status IN ('ready','delivered')"}
	args := []any{query.Channel}
	argn := 2
	if query.AfterID > 0 {
		where = append(where, fmt.Sprintf("e.id > $%d", argn))
		args = append(args, query.AfterID)
		argn++
	}
	if query.Status == "unread" && query.EmployeeID > 0 {
		where = append(where, "r.event_id IS NULL")
	}
	args = append(args, query.EmployeeID, query.Limit)
	employeeArg := argn
	limitArg := argn + 1

	sql := fmt.Sprintf(`
		SELECT e.id, e.topic, e.event_type, e.source_type, e.source_id,
		       e.title, e.body, e.tone, e.payload_json,
		       to_char(e.created_at,'YYYY-MM-DD HH24:MI:SS') AS created_at,
		       r.event_id IS NOT NULL AS read
		FROM %[1]s.message_events e
		JOIN %[1]s.message_deliveries d ON d.event_id=e.id
		LEFT JOIN %[1]s.message_reads r ON r.event_id=e.id AND r.employee_id=$%[2]d
		WHERE %[3]s
		  AND (
		     d.target_type='broadcast'
		     OR (
		        d.target_type='permission'
		        AND EXISTS (
		            SELECT 1
		            FROM %[1]s.employee_roles er
		            LEFT JOIN %[1]s.auth_role_permissions rp ON rp.role_code=er.role_code
		            WHERE er.employee_id=$%[2]d
		              AND (er.role_code='admin' OR rp.permission_code=d.target_key)
		        )
		     )
		     OR (
		        d.target_type='role'
		        AND EXISTS (
		            SELECT 1
		            FROM %[1]s.employee_roles er
		            WHERE er.employee_id=$%[2]d
		              AND (er.role_code='admin' OR er.role_code=d.target_key)
		        )
		     )
		     OR (d.target_type='employee' AND d.target_employee_id=$%[2]d)
		     OR (
		        d.target_type='order_responsible'
		        AND e.source_type='order'
		        AND EXISTS (
		            SELECT 1
		            FROM %[1]s.orders o
		            WHERE o.id=e.source_id
		              AND o.responsible_party_type='employee'
		              AND o.responsible_party_id=$%[2]d
		        )
		     )
		     OR (
		        d.target_type='order_customer'
		        AND e.source_type='order'
		        AND EXISTS (
		            SELECT 1
		            FROM %[1]s.orders o
		            JOIN %[1]s.customer_erp_user_bindings b ON b.customer_id=o.customer_id
		            WHERE o.id=e.source_id
		              AND b.employee_id=$%[2]d
		              AND b.status='active'
		        )
		     )
		  )
		ORDER BY e.id DESC
		LIMIT $%[4]d
	`, r.schema, employeeArg, joinWhere(where), limitArg)

	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]app.Notification, 0)
	for rows.Next() {
		var row app.Notification
		var payload []byte
		if err := rows.Scan(&row.ID, &row.Topic, &row.EventType, &row.SourceType, &row.SourceID, &row.Title, &row.Body, &row.Tone, &payload, &row.CreatedAt, &row.Read); err != nil {
			return nil, err
		}
		row.Payload = map[string]any{}
		if len(payload) > 0 {
			_ = json.Unmarshal(payload, &row.Payload)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) ListRules(ctx context.Context) ([]app.Rule, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, code, name, enabled, topic, event_type, source_type, channel,
		       target_type, target_key, target_employee_id, template_key, adapter_key, tone,
		       payload_match_json,
		       to_char(created_at,'YYYY-MM-DD HH24:MI:SS'),
		       to_char(updated_at,'YYYY-MM-DD HH24:MI:SS')
		FROM %s.message_notification_rules
		ORDER BY enabled DESC, event_type, code
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRules(rows)
}

func (r Repository) ListActiveRules(ctx context.Context, query app.RuleQuery) ([]app.Rule, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, code, name, enabled, topic, event_type, source_type, channel,
		       target_type, target_key, target_employee_id, template_key, adapter_key, tone,
		       payload_match_json,
		       to_char(created_at,'YYYY-MM-DD HH24:MI:SS'),
		       to_char(updated_at,'YYYY-MM-DD HH24:MI:SS')
		FROM %s.message_notification_rules
		WHERE enabled=true
		  AND event_type=$1
		  AND (topic='' OR topic=$2)
		  AND (source_type='' OR source_type=$3)
		ORDER BY id
	`, r.schema), strings.TrimSpace(query.EventType), strings.TrimSpace(query.Topic), strings.TrimSpace(query.SourceType))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRules(rows)
}

func (r Repository) SaveRule(ctx context.Context, cmd app.SaveRuleCommand) (app.Rule, error) {
	payloadMatch, err := json.Marshal(cmd.PayloadMatch)
	if err != nil {
		return app.Rule{}, err
	}
	enabled := true
	if cmd.Enabled != nil {
		enabled = *cmd.Enabled
	}
	row := r.pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.message_notification_rules(
			code, name, enabled, topic, event_type, source_type, channel,
			target_type, target_key, target_employee_id, template_key, adapter_key, tone, payload_match_json, updated_at
		)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14::jsonb,now())
		ON CONFLICT(code) DO UPDATE SET
			name=excluded.name,
			enabled=excluded.enabled,
			topic=excluded.topic,
			event_type=excluded.event_type,
			source_type=excluded.source_type,
			channel=excluded.channel,
			target_type=excluded.target_type,
			target_key=excluded.target_key,
			target_employee_id=excluded.target_employee_id,
			template_key=excluded.template_key,
			adapter_key=excluded.adapter_key,
			tone=excluded.tone,
			payload_match_json=excluded.payload_match_json,
			updated_at=now()
		RETURNING id, code, name, enabled, topic, event_type, source_type, channel,
		          target_type, target_key, target_employee_id, template_key, adapter_key, tone,
		          payload_match_json,
		          to_char(created_at,'YYYY-MM-DD HH24:MI:SS'),
		          to_char(updated_at,'YYYY-MM-DD HH24:MI:SS')
	`, r.schema),
		cmd.Code, cmd.Name, enabled, cmd.Topic, cmd.EventType, cmd.SourceType, cmd.Channel,
		cmd.TargetType, cmd.TargetKey, cmd.TargetEmployeeID, cmd.TemplateKey, cmd.AdapterKey, cmd.Tone, string(payloadMatch),
	)
	rules, err := scanRuleRows(row)
	if err != nil {
		return app.Rule{}, err
	}
	return rules[0], nil
}

type ruleScanner interface {
	Scan(dest ...any) error
}

type ruleRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func scanRules(rows ruleRows) ([]app.Rule, error) {
	out := make([]app.Rule, 0)
	for rows.Next() {
		rule, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func scanRuleRows(row ruleScanner) ([]app.Rule, error) {
	rule, err := scanRule(row)
	if err != nil {
		return nil, err
	}
	return []app.Rule{rule}, nil
}

func scanRule(scanner ruleScanner) (app.Rule, error) {
	var rule app.Rule
	var payload []byte
	if err := scanner.Scan(
		&rule.ID, &rule.Code, &rule.Name, &rule.Enabled, &rule.Topic, &rule.EventType, &rule.SourceType, &rule.Channel,
		&rule.TargetType, &rule.TargetKey, &rule.TargetEmployeeID, &rule.TemplateKey, &rule.AdapterKey, &rule.Tone,
		&payload, &rule.CreatedAt, &rule.UpdatedAt,
	); err != nil {
		return app.Rule{}, err
	}
	rule.PayloadMatch = map[string]any{}
	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &rule.PayloadMatch)
	}
	return rule, nil
}

func (r Repository) MarkRead(ctx context.Context, eventID, employeeID int64) error {
	if eventID <= 0 || employeeID <= 0 {
		return nil
	}
	_, err := r.pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.message_reads(event_id, employee_id, read_at)
		VALUES($1,$2,now())
		ON CONFLICT(event_id, employee_id) DO UPDATE SET read_at=excluded.read_at
	`, r.schema), eventID, employeeID)
	return err
}

func joinWhere(where []string) string {
	out := ""
	for i, item := range where {
		if i > 0 {
			out += " AND "
		}
		out += item
	}
	return out
}
