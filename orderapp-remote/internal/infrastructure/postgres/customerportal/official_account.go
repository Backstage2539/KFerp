package customerportal

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/customerportal"
	"strings"
	"time"
)

// OfficialAccountContext reuses the live customer binding and projected ERP
// account validation. An official-account binding is not a long-lived session.
func (r Repository) OfficialAccountContext(ctx context.Context, userID, customerID int64, boundAt time.Time) (app.CurrentContext, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.CurrentContext{}, err
	}
	defer tx.Rollback(ctx)
	var openID string
	if err = tx.QueryRow(ctx, fmt.Sprintf("SELECT openid FROM %s.mini_users WHERE id=$1 AND active=true", r.schema), userID).Scan(&openID); err != nil {
		return app.CurrentContext{}, app.ErrCustomerBindingNotFound
	}
	if strings.HasPrefix(openID, "erp-internal-employee:") {
		return app.CurrentContext{}, app.ErrCustomerBindingNotFound
	}
	v, err := r.validateProjectedMiniBindingsTx(ctx, tx, userID, openID, boundAt)
	if err != nil {
		return app.CurrentContext{}, err
	}
	if v.invalidatesSession(openID, customerID) {
		_ = tx.Commit(ctx)
		return app.CurrentContext{}, app.ErrCustomerBindingNotFound
	}
	bindings, err := r.listBindingsTx(ctx, tx, userID)
	if err != nil {
		return app.CurrentContext{}, err
	}
	c := app.CurrentContext{MiniUserID: userID, AccountType: "customer", Bindings: bindings}
	if customerID == 0 && len(bindings) == 1 {
		customerID = bindings[0].CustomerID
	}
	for _, b := range bindings {
		if b.CustomerID == customerID {
			c.CurrentCustomerID = b.CustomerID
			c.CurrentCustomerName = b.CustomerName
		}
	}
	if c.CurrentCustomerID > 0 {
		c.Capabilities, err = r.capabilitiesForCustomerTx(ctx, tx, c.CurrentCustomerID)
		if err != nil {
			return c, err
		}
	}
	return c, tx.Commit(ctx)
}

func (r Repository) LoadEntryPublication(ctx context.Context, id int64) (app.BeanListSummary, error) {
	var row app.BeanListSummary
	var config, content []byte
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT id,list_type,version_no,status,to_char(published_at,'YYYY-MM-DD HH24:MI'),changelog,config_json,content_json FROM %s.bean_list_publications WHERE id=$1 AND status='published' AND deleted_at IS NULL`, r.schema), id).Scan(&row.ID, &row.ListType, &row.VersionNo, &row.Status, &row.PublishedAt, &row.Changelog, &config, &content)
	if err == pgx.ErrNoRows {
		return row, app.ErrBeanListPublicationNotFound
	}
	if err != nil {
		return row, err
	}
	if !json.Valid(content) {
		return row, app.ErrBeanListPublicationNotFound
	}
	err = parseBeanListDisplaySummary(config, content, &row)
	if err != nil {
		return row, err
	}
	for _, group := range row.Groups {
		if len(group.Items) > 0 {
			return row, nil
		}
	}
	return row, app.ErrBeanListPublicationNotFound
}
func (r Repository) OfficialRecentOrders(ctx context.Context, c app.CurrentContext, limit int) ([]app.CustomerOrderSummary, error) {
	if c.CurrentCustomerID <= 0 || !c.HasAnyCapability([]string{app.CapabilityProductOrder, app.CapabilityDirectShip, app.CapabilityShippingQuery, app.CapabilityMall}) {
		return nil, app.ErrCustomerBindingNotFound
	}
	return r.listCustomerOrders(ctx, app.ServicePageQuery{CustomerID: c.CurrentCustomerID}, limit, true)
}
