package freight

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	domain "orderapp/internal/domain/freight"
	"strings"
)

// ResolvePolicy is only for quoting; editors always load the stored override.
// A configured template that is inactive or lacks a rate must fail at quote time.
func (r *Repository) ResolvePolicy(ctx context.Context, q Queryer, id int64, scene string, lock bool) (domain.Policy, error) {
	p, err := r.loadFulfillmentPolicy(ctx, q, id, scene, lock)
	if err != nil || p.TemplateID > 0 || p.FreeShipping {
		return p, err
	}
	defaultsRevision := p.FulfillmentDefaultsRevision
	if scene != "sale" {
		p, err = r.LoadPolicy(ctx, q, id, "sale", lock)
		if err != nil {
			return p, err
		}
	}
	if p.TemplateID == 0 && !p.FreeShipping {
		p, err = r.LoadDefaultPolicy(ctx, q, lock)
	}
	p.FulfillmentDefaultsRevision = defaultsRevision
	p.CustomerID = id
	p.Scene = scene
	return p, err
}

func (r *Repository) LoadDefaultPolicy(ctx context.Context, q Queryer, lock bool) (domain.Policy, error) {
	p := domain.Policy{Scene: "sale"}
	suffix := ""
	if lock {
		suffix = " FOR SHARE"
	}
	err := q.QueryRow(ctx, fmt.Sprintf(`SELECT revision,template_id,surcharge_cents,free_shipping FROM %s.system_shipping_policy WHERE singleton=true%s`, r.schema, suffix)).Scan(&p.Revision, &p.TemplateID, &p.SurchargeCents, &p.FreeShipping)
	if errors.Is(err, pgx.ErrNoRows) {
		err = nil
	}
	return p, err
}
func (r *Repository) DefaultPolicy(ctx context.Context) (domain.Policy, error) {
	return r.LoadDefaultPolicy(ctx, r.pool, false)
}
func (r *Repository) SaveDefaultPolicy(ctx context.Context, p domain.Policy, expected int64, actor string) (domain.Policy, error) {
	p.CustomerID = 0
	p.Scene = "sale"
	if err := domain.ValidatePolicy(p); err != nil {
		return p, err
	}
	if strings.TrimSpace(actor) == "" {
		return p, fmt.Errorf("操作人必填")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return p, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, r.schema+":system-shipping-policy"); err != nil {
		return p, err
	}
	old, err := r.LoadDefaultPolicy(ctx, tx, true)
	if err != nil {
		return p, err
	}
	if old.Revision != expected {
		return p, fmt.Errorf("系统默认运费配置已变化，请刷新")
	}
	if p.TemplateID > 0 {
		tm, e := r.LoadTemplate(ctx, tx, p.TemplateID, true)
		if e != nil || !tm.Active {
			return p, fmt.Errorf("运费模板不存在或停用")
		}
	}
	p.Revision = old.Revision + 1
	_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.system_shipping_policy(singleton,revision,template_id,surcharge_cents,free_shipping,updated_by) VALUES(true,$1,$2,$3,$4,$5) ON CONFLICT(singleton) DO UPDATE SET revision=EXCLUDED.revision,template_id=EXCLUDED.template_id,surcharge_cents=EXCLUDED.surcharge_cents,free_shipping=EXCLUDED.free_shipping,updated_by=EXCLUDED.updated_by,updated_at=now()`, r.schema), p.Revision, p.TemplateID, p.SurchargeCents, p.FreeShipping, actor)
	if err != nil {
		return p, err
	}
	if err = r.Audit(ctx, tx, actor, "system_shipping_policy", 0, "save", old, p); err != nil {
		return p, err
	}
	return p, tx.Commit(ctx)
}
