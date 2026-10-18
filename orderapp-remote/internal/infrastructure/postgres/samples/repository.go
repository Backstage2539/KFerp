package samples

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	messages "orderapp/internal/application/messagecenter"
	app "orderapp/internal/application/samples"
	pg "orderapp/internal/infrastructure/postgres"
	portalpg "orderapp/internal/infrastructure/postgres/customerportal"
	messagepg "orderapp/internal/infrastructure/postgres/messagecenter"
	"strings"
	"time"
)

type Repository struct {
	Pool   *pgxpool.Pool
	Schema string
}

func NewRepository(p *pgxpool.Pool, s string) Repository { return Repository{p, s} }

type querier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (r Repository) query() string {
	return fmt.Sprintf(`SELECT row_to_json(x) FROM (
 SELECT r.id,'SP-'||to_char(r.created_at,'YYYYMMDD')||'-'||r.id::text AS number,r.mini_user_id,r.nickname,r.verified_phone,r.entry_key,r.publication_id,r.publication_name,r.publication_version,r.receiver,r.note,r.created_at,
 COALESCE(w.id,0) work_order_id,COALESCE(w.assignee_id,0) assignee_id,COALESCE(e.name,'') assignee_name,COALESCE(w.status,'pending_assignment') status,COALESCE(w.followup_status,'pending') followup_status,COALESCE(w.revision,0) revision,
 COALESCE(w.customer_id,0) customer_id,COALESCE(w.order_id,0) order_id,COALESCE(o.order_no,'') order_no,COALESCE(ss.name,'') shipment_status,w.next_reminder_at,w.confirmed_json AS confirmation,
 COALESCE((SELECT json_agg(json_build_object('key',i.item_key,'product_id',i.product_id,'code',i.code,'name',i.name) ORDER BY i.position) FROM %[1]s.sample_request_items i WHERE i.request_id=r.id),'[]') items
 FROM %[1]s.sample_requests r LEFT JOIN %[1]s.sample_work_orders w ON w.request_id=r.id LEFT JOIN %[1]s.company_employees e ON e.id=w.assignee_id LEFT JOIN %[1]s.orders o ON o.id=w.order_id LEFT JOIN %[1]s.ship_statuses ss ON ss.id=o.ship_status_id
 WHERE r.id=$1) x`, r.Schema)
}
func (r Repository) raw(ctx context.Context, q querier, id int64) (app.Request, error) {
	var raw []byte
	var v app.Request
	err := q.QueryRow(ctx, r.query(), id).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, app.ErrNotFound
	}
	if err != nil {
		return v, err
	}
	err = json.Unmarshal(raw, &v)
	return v, err
}
func (r Repository) Get(ctx context.Context, p app.Principal, id int64) (app.Request, error) {
	v, err := r.raw(ctx, r.Pool, id)
	if err != nil {
		return v, err
	}
	if !app.CanRead(p, v) {
		return app.Request{}, app.ErrForbidden
	}
	if !p.Staff() {
		v.VerifiedPhone = ""
		v.MiniUserID = 0
		v.CustomerID = 0
		v.OrderID = 0
		v.FollowupStatus = ""
		v.NextReminderAt = nil
		v.Confirmation = nil
		return v, nil
	}
	rows, err := r.Pool.Query(ctx, "SELECT id,kind,status,note,actor,created_at FROM "+r.Schema+".sample_activities WHERE request_id=$1 ORDER BY id", id)
	if err != nil {
		return v, err
	}
	defer rows.Close()
	v.Activities = []app.Activity{}
	for rows.Next() {
		var a app.Activity
		if err = rows.Scan(&a.ID, &a.Kind, &a.Status, &a.Note, &a.Actor, &a.CreatedAt); err != nil {
			return v, err
		}
		v.Activities = append(v.Activities, a)
	}
	return v, rows.Err()
}
func (r Repository) List(ctx context.Context, p app.Principal, q app.Query) (app.ListResult, error) {
	out := app.ListResult{Rows: []app.Request{}, IsAdmin: p.Admin}
	if !p.Staff() && (!p.Registered || p.MiniUserID <= 0) {
		return out, app.ErrForbidden
	}
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 || q.PageSize > 100 {
		q.PageSize = 30
	}
	where := ` FROM %[1]s.sample_requests r LEFT JOIN %[1]s.sample_work_orders w ON w.request_id=r.id WHERE ($1 OR ($2::bigint>0 AND w.assignee_id=$2) OR ($2=0 AND r.mini_user_id=$3)) AND ($4='' OR r.nickname ILIKE '%%'||$4||'%%' OR r.verified_phone ILIKE '%%'||$4||'%%' OR r.id::text=$4 OR ('SP-'||to_char(r.created_at,'YYYYMMDD')||'-'||r.id::text) ILIKE '%%'||$4||'%%' OR r.receiver->>'name' ILIKE '%%'||$4||'%%' OR r.receiver->>'phone' ILIKE '%%'||$4||'%%') AND ($5='' OR COALESCE(w.status,'pending_assignment')=$5 OR w.followup_status=$5) AND ($6::bigint=0 OR r.mini_user_id=$6)`
	args := []any{p.Admin, p.EmployeeID, p.MiniUserID, strings.TrimSpace(q.Search), q.Status, q.MiniUserID}
	if err := r.Pool.QueryRow(ctx, fmt.Sprintf("SELECT count(*)"+where, r.Schema), args...).Scan(&out.Total); err != nil {
		return out, err
	}
	rows, err := r.Pool.Query(ctx, fmt.Sprintf("SELECT r.id"+where+" ORDER BY r.id DESC LIMIT $7 OFFSET $8", r.Schema), append(args, q.PageSize, (q.Page-1)*q.PageSize)...)
	if err != nil {
		return out, err
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return out, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	for _, id := range ids {
		v, err := r.Get(ctx, p, id)
		if errors.Is(err, app.ErrForbidden) {
			continue
		}
		if err != nil {
			return out, err
		}
		v.Activities = nil
		out.Rows = append(out.Rows, v)
	}
	return out, nil
}
func (r Repository) lock(ctx context.Context, tx pgx.Tx, p app.Principal, id, rev int64) (app.Request, error) {
	var key int64
	if err := tx.QueryRow(ctx, "SELECT id FROM "+r.Schema+".sample_requests WHERE id=$1 FOR UPDATE", id).Scan(&key); errors.Is(err, pgx.ErrNoRows) {
		return app.Request{}, app.ErrNotFound
	} else if err != nil {
		return app.Request{}, err
	}
	v, err := r.raw(ctx, tx, id)
	if err != nil {
		return v, err
	}
	if !app.CanWork(p, v) {
		return v, app.ErrForbidden
	}
	if rev >= 0 && v.Revision != rev {
		return v, app.ErrConflict
	}
	return v, nil
}
func (r Repository) activity(ctx context.Context, tx pgx.Tx, id int64, kind, status, note, actor string) error {
	if _, err := tx.Exec(ctx, "INSERT INTO "+r.Schema+".sample_activities(request_id,kind,status,note,actor) VALUES($1,$2,$3,$4,$5)", id, kind, status, note, actor); err != nil {
		return err
	}
	return pg.AuditInsertTx(ctx, tx, r.Schema, actor, "sample_request", &id, kind, nil, nil, nil, pg.AuditMeta{"status": status, "note": note})
}
func (r Repository) notify(ctx context.Context, tx pgx.Tx, id int64, key, event, title string, employeeID int64) error {
	delivery := messages.DeliveryCommand{Channel: messages.ChannelERPPlatform, TargetType: "employee", TargetEmployeeID: employeeID}
	if employeeID == 0 {
		delivery.TargetType = "role"
		delivery.TargetKey = "admin"
	}
	svc := messagepg.NewRepository(r.Pool, r.Schema).WithTransaction(tx)
	_, err := svc.Publish(ctx, messages.PublishCommand{EventKey: key, Topic: "samples", EventType: event, SourceType: "sample_request", SourceID: id, Actor: "system", Title: title, Tone: "info", Body: "打开样品工单查看并处理", Payload: map[string]any{"request_id": id, "view": "sampleRequests"}, Deliveries: []messages.DeliveryCommand{delivery}})
	return err
}
func (r Repository) Submit(ctx context.Context, p app.Principal, c app.SubmitCommand) (app.Request, error) {
	if p.Staff() || p.EmployeeID > 0 || !p.Registered || p.MiniUserID <= 0 {
		return app.Request{}, app.ErrForbidden
	}
	if err := app.ValidateSubmit(c); err != nil {
		return app.Request{}, err
	}
	raw, _ := json.Marshal(c)
	hash := sha256.Sum256(raw)
	digest := hex.EncodeToString(hash[:])
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return app.Request{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", fmt.Sprintf("sample-submit:%d:%s", p.MiniUserID, c.RequestID)); err != nil {
		return app.Request{}, err
	}
	var oldID int64
	var oldHash string
	err = tx.QueryRow(ctx, "SELECT id,request_hash FROM "+r.Schema+".sample_requests WHERE mini_user_id=$1 AND request_key=$2", p.MiniUserID, c.RequestID).Scan(&oldID, &oldHash)
	if err == nil {
		if digest != oldHash {
			return app.Request{}, app.ErrConflict
		}
		if err = tx.Commit(ctx); err != nil {
			return app.Request{}, err
		}
		return r.Get(ctx, p, oldID)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return app.Request{}, err
	}
	var name, version, nickname, phone string
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT e.published->>'name',b.version_no,p.nickname,p.verified_phone FROM %[1]s.page_entries e JOIN %[1]s.bean_list_publications b ON b.id=(e.published->>'publication_id')::bigint JOIN %[1]s.mini_registrations p ON p.mini_user_id=$3 JOIN %[1]s.mini_users u ON u.id=p.mini_user_id AND u.active WHERE e.entry_key=$1 AND e.enabled AND e.deleted_at IS NULL AND e.published->>'kind'='price' AND e.published->>'bean_center'='true' AND b.id=$2 AND b.owner_type='official' AND b.status='published' AND b.deleted_at IS NULL AND (e.published->>'visibility' IN('public','registered') OR EXISTS(SELECT 1 FROM %[1]s.customer_portal_user_bindings cb JOIN %[1]s.customers customer ON customer.id=cb.customer_id AND customer.active JOIN %[1]s.customer_portal_profiles cp ON cp.customer_id=customer.id AND cp.enabled WHERE cb.mini_user_id=$3 AND cb.customer_id=$4 AND cb.status='approved')) FOR SHARE OF e,b,u,p`, r.Schema), c.EntryKey, c.PublicationID, p.MiniUserID, p.PriceCustomerID).Scan(&name, &version, &nickname, &phone)
	if errors.Is(err, pgx.ErrNoRows) {
		return app.Request{}, app.Invalid("价格表已更新或不可申请，请重新打开价格表")
	}
	if err != nil {
		return app.Request{}, err
	}
	publication, err := portalpg.NewRepository(r.Pool, r.Schema).LoadEntryPublicationTx(ctx, tx, c.PublicationID)
	if err != nil {
		return app.Request{}, err
	}
	candidates := map[string]app.Item{}
	for _, g := range publication.Groups {
		for _, i := range g.Items {
			if i.ProductID > 0 && i.SampleKey != "" {
				candidates[i.SampleKey] = app.Item{Key: i.SampleKey, ProductID: i.ProductID, Code: i.Code, Name: i.Name}
			}
		}
	}
	selected := []app.Item{}
	seen := map[int64]bool{}
	for _, key := range c.ProductKeys {
		i, ok := candidates[key]
		if !ok {
			return app.Request{}, app.Invalid("申请商品不属于当前价格表")
		}
		if seen[i.ProductID] {
			continue
		}
		var active bool
		err = tx.QueryRow(ctx, "SELECT active AND COALESCE(parent_product_id,0)=0 FROM "+r.Schema+".products WHERE id=$1", i.ProductID).Scan(&active)
		if errors.Is(err, pgx.ErrNoRows) || err == nil && !active {
			return app.Request{}, app.Invalid("商品已停用，请重新选择")
		}
		if err != nil {
			return app.Request{}, err
		}
		seen[i.ProductID] = true
		selected = append(selected, i)
	}
	receiver, _ := json.Marshal(c.Receiver)
	var id int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.sample_requests(mini_user_id,request_key,request_hash,nickname,verified_phone,entry_key,publication_id,publication_name,publication_version,receiver,note) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id`, r.Schema), p.MiniUserID, c.RequestID, digest, nickname, phone, c.EntryKey, c.PublicationID, name, version, receiver, strings.TrimSpace(c.Note)).Scan(&id)
	if err != nil {
		return app.Request{}, err
	}
	for pos, i := range selected {
		if _, err = tx.Exec(ctx, "INSERT INTO "+r.Schema+".sample_request_items(request_id,item_key,product_id,code,name,position) VALUES($1,$2,$3,$4,$5,$6)", id, i.Key, i.ProductID, i.Code, i.Name, pos); err != nil {
			return app.Request{}, err
		}
	}
	if err = r.activity(ctx, tx, id, "submit", "pending_assignment", "提交样品申请", p.Actor); err != nil {
		return app.Request{}, err
	}
	if err = r.notify(ctx, tx, id, fmt.Sprintf("sample:%d:submitted", id), "sample.submitted", fmt.Sprintf("新样品申请 #%d 待分配", id), 0); err != nil {
		return app.Request{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return app.Request{}, err
	}
	return r.Get(ctx, p, id)
}
func (r Repository) Assign(ctx context.Context, p app.Principal, id int64, c app.AssignCommand) (app.Request, error) {
	if !p.Admin {
		return app.Request{}, app.ErrForbidden
	}
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return app.Request{}, err
	}
	defer tx.Rollback(ctx)
	v, err := r.lock(ctx, tx, p, id, c.Revision)
	if err != nil {
		return v, err
	}
	if app.Terminal(v.FollowupStatus) {
		return v, app.Invalid("已结束的工单不能改派")
	}
	var name string
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT e.name FROM %[1]s.company_employees e WHERE e.id=$1 AND e.active AND COALESCE(NULLIF(e.account_type,''),'internal_employee')='internal_employee' AND EXISTS(SELECT 1 FROM %[1]s.employee_roles er WHERE er.employee_id=e.id AND er.role_code IN('sales','admin')) AND NOT EXISTS(SELECT 1 FROM %[1]s.employee_login_passwords ep WHERE ep.employee_id=e.id AND ep.login_disabled) FOR SHARE OF e`, r.Schema), c.EmployeeID).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, app.Invalid("请选择启用的销售或管理员")
	}
	if err != nil {
		return v, err
	}
	if v.AssigneeID == c.EmployeeID {
		return v, nil
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.sample_work_orders(request_id,assignee_id,next_reminder_at) VALUES($1,$2,now()+interval '7 days') ON CONFLICT(request_id) DO UPDATE SET assignee_id=EXCLUDED.assignee_id,revision=sample_work_orders.revision+1,updated_at=now()`, r.Schema), id, c.EmployeeID)
	if err != nil {
		return v, err
	}
	if err = r.activity(ctx, tx, id, "assign", v.FollowupStatus, "分配给 "+name, p.Actor); err != nil {
		return v, err
	}
	if err = r.notify(ctx, tx, id, fmt.Sprintf("sample:%d:assignment:%d", id, v.Revision+1), "sample.assigned", fmt.Sprintf("收到样品工单 #%d", id), c.EmployeeID); err != nil {
		return v, err
	}
	if err = tx.Commit(ctx); err != nil {
		return v, err
	}
	return r.Get(ctx, p, id)
}
func (r Repository) Followup(ctx context.Context, p app.Principal, id int64, c app.FollowupCommand) (app.Request, error) {
	next, err := app.NextFollowup(c.Status, c.Note, time.Now().UTC())
	if err != nil {
		return app.Request{}, err
	}
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return app.Request{}, err
	}
	defer tx.Rollback(ctx)
	v, err := r.lock(ctx, tx, p, id, c.Revision)
	if err != nil {
		return v, err
	}
	if v.WorkOrderID == 0 {
		return v, app.Invalid("请先分配工单")
	}
	if app.Terminal(v.FollowupStatus) {
		return v, app.Invalid("工单已结束")
	}
	_, err = tx.Exec(ctx, "UPDATE "+r.Schema+".sample_work_orders SET followup_status=$2,next_reminder_at=$3,revision=revision+1,status=CASE WHEN $2 IN('won','abandoned') THEN 'closed' ELSE status END,updated_at=now() WHERE request_id=$1", id, c.Status, next)
	if err != nil {
		return v, err
	}
	if err = r.activity(ctx, tx, id, "followup", c.Status, strings.TrimSpace(c.Note), p.Actor); err != nil {
		return v, err
	}
	if err = tx.Commit(ctx); err != nil {
		return v, err
	}
	return r.Get(ctx, p, id)
}
func (r Repository) Remind(ctx context.Context, now time.Time) (int, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	// Request rows serialize reminder emission with follow-up, reassignment and closure.
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT r.id,w.assignee_id,w.next_reminder_at FROM %[1]s.sample_requests r JOIN %[1]s.sample_work_orders w ON w.request_id=r.id WHERE w.next_reminder_at<=$1 AND w.followup_status NOT IN('won','abandoned') ORDER BY w.next_reminder_at LIMIT 200 FOR UPDATE OF r,w SKIP LOCKED`, r.Schema), now)
	if err != nil {
		return 0, err
	}
	type due struct {
		id, employee int64
		at           time.Time
	}
	batch := []due{}
	for rows.Next() {
		var d due
		if err = rows.Scan(&d.id, &d.employee, &d.at); err != nil {
			rows.Close()
			return 0, err
		}
		batch = append(batch, d)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	for _, d := range batch {
		if err = r.notify(ctx, tx, d.id, fmt.Sprintf("sample:%d:reminder:%d", d.id, d.at.UnixNano()), "sample.followup_due", fmt.Sprintf("样品工单 #%d 已7天未更新回访", d.id), d.employee); err != nil {
			return 0, err
		}
		if _, err = tx.Exec(ctx, "UPDATE "+r.Schema+".sample_work_orders SET next_reminder_at=$2,last_reminded_at=$3 WHERE request_id=$1", d.id, now.Add(7*24*time.Hour), now); err != nil {
			return 0, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return len(batch), nil
}
