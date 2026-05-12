package messagecenter

import (
	"context"
	"encoding/json"
	"fmt"

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
			INSERT INTO %s.message_deliveries(event_id, channel, target_type, target_key, target_employee_id, status)
			VALUES($1,$2,$3,$4,$5,'ready')
			ON CONFLICT(event_id, channel, target_type, target_key, target_employee_id) DO NOTHING
		`, r.schema),
			eventID,
			delivery.Channel,
			delivery.TargetType,
			delivery.TargetKey,
			delivery.TargetEmployeeID,
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
		FROM %s.message_events e
		JOIN %s.message_deliveries d ON d.event_id=e.id
		LEFT JOIN %s.message_reads r ON r.event_id=e.id AND r.employee_id=$%d
		WHERE %s
		  AND (
		  	d.target_type='broadcast'
		  	OR d.target_type='permission'
		  	OR (d.target_type='employee' AND d.target_employee_id=$%d)
		  )
		ORDER BY e.id DESC
		LIMIT $%d
	`, r.schema, r.schema, r.schema, employeeArg, joinWhere(where), employeeArg, limitArg)

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
