package sales

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	salesapp "orderapp/internal/application/sales"
	salesdomain "orderapp/internal/domain/sales"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func (r Repository) OrderForm(ctx context.Context, editID int64) (salesapp.OrderFormData, error) {
	data := salesapp.OrderFormData{Today: time.Now().Format("2006-01-02")}
	var err error
	if data.Customers, err = r.fetchOrderCustomers(ctx); err != nil {
		return salesapp.OrderFormData{}, err
	}
	if data.Sources, err = fetchOrderOptions(ctx, r.pool, fmt.Sprintf("SELECT id, name FROM %s.sources ORDER BY id", r.schema)); err != nil {
		return salesapp.OrderFormData{}, err
	}
	if data.ShipStatuses, err = fetchOrderOptions(ctx, r.pool, fmt.Sprintf("SELECT id, name FROM %s.ship_statuses ORDER BY id", r.schema)); err != nil {
		return salesapp.OrderFormData{}, err
	}
	if data.PayStatuses, err = fetchOrderOptions(ctx, r.pool, fmt.Sprintf("SELECT id, name FROM %s.pay_statuses ORDER BY id", r.schema)); err != nil {
		return salesapp.OrderFormData{}, err
	}
	if data.OrderTypes, err = fetchOrderOptions(ctx, r.pool, fmt.Sprintf("SELECT id, name FROM %s.order_types ORDER BY id", r.schema)); err != nil {
		return salesapp.OrderFormData{}, err
	}
	if data.Products, err = r.fetchOrderProducts(ctx); err != nil {
		return salesapp.OrderFormData{}, err
	}
	if data.Employees, err = r.fetchOrderEmployees(ctx); err != nil {
		return salesapp.OrderFormData{}, err
	}
	if editID > 0 {
		editData, err := r.fetchOrderEdit(ctx, editID)
		if err != nil {
			return salesapp.OrderFormData{}, err
		}
		data.EditData = editData
	}
	return data, nil
}

func (r Repository) fetchOrderCustomers(ctx context.Context) ([]salesapp.CustomerOption, error) {
	q := fmt.Sprintf(`
		SELECT id, name, COALESCE(contact,''), COALESCE(phone,''), COALESCE(default_source_id,0), COALESCE(default_order_type_id,0)
		FROM %s.customers
		WHERE active=true
		ORDER BY name
	`, r.schema)
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]salesapp.CustomerOption, 0)
	for rows.Next() {
		var row salesapp.CustomerOption
		if err := rows.Scan(&row.ID, &row.Name, &row.Contact, &row.Phone, &row.DefaultSourceID, &row.DefaultOrderTypeID); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) fetchOrderEmployees(ctx context.Context) ([]salesapp.EmployeeOption, error) {
	q := fmt.Sprintf(`
		SELECT e.id, e.name, COALESCE(e.phone,''), COALESCE(e.department_id,0), COALESCE(d.name,'')
		FROM %s.company_employees e
		LEFT JOIN %s.company_departments d ON d.id=e.department_id
		WHERE e.active=true
		ORDER BY e.id DESC
	`, r.schema, r.schema)
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]salesapp.EmployeeOption, 0)
	for rows.Next() {
		var row salesapp.EmployeeOption
		if err := rows.Scan(&row.ID, &row.Name, &row.Phone, &row.DepartmentID, &row.Department); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) fetchOrderProducts(ctx context.Context) ([]salesapp.ProductOption, error) {
	sqlstr := fmt.Sprintf(`SELECT id, name, COALESCE(roast_level,''), default_price,
		COALESCE(retail_price_100g, 0),
		COALESCE(retail_price_200g, 0),
		COALESCE(retail_price_227g, default_price, 0),
		COALESCE(retail_price_250g, 0),
		COALESCE(customer_id, 0),
		COALESCE(base_product_id, 0),
		COALESCE(NULLIF(visibility,''), 'public'),
		COALESCE(custom_type, '')
		FROM %s.products WHERE active=true ORDER BY name`, r.schema)
	rows, err := r.pool.Query(ctx, sqlstr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]salesapp.ProductOption, 0)
	for rows.Next() {
		var p salesapp.ProductOption
		if err := rows.Scan(&p.ID, &p.Name, &p.RoastLevel, &p.DefaultPrice, &p.RetailPrice100G, &p.RetailPrice200G, &p.RetailPrice227G, &p.RetailPrice250G, &p.CustomerID, &p.BaseProductID, &p.Visibility, &p.CustomType); err != nil {
			return nil, err
		}
		p.RetailSpecs = salesdomain.RetailAvailableSpecs(salesdomain.RetailSpecPrices{
			Price100G: p.RetailPrice100G,
			Price200G: p.RetailPrice200G,
			Price227G: p.RetailPrice227G,
			Price250G: p.RetailPrice250G,
		})
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	tierSQL := fmt.Sprintf(`
		SELECT id, product_id,
		       COALESCE(NULLIF(spec_g,0), 454),
		       COALESCE(min_qty_units, min_qty_lb),
		       COALESCE(max_qty_units, max_qty_lb),
		       COALESCE(price_per_unit, price_per_lb)
		FROM %s.product_price_tiers
		WHERE active=true
		ORDER BY product_id, COALESCE(NULLIF(spec_g,0), 454), COALESCE(min_qty_units, min_qty_lb)
	`, r.schema)
	trs, err := r.pool.Query(ctx, tierSQL)
	if err != nil {
		return out, nil
	}
	defer trs.Close()

	tierMap := map[int64][]salesapp.ProductTierOption{}
	for trs.Next() {
		var tid, pid int64
		var specG int64
		var min float64
		var max *float64
		var price float64
		if err := trs.Scan(&tid, &pid, &specG, &min, &max, &price); err != nil {
			return nil, err
		}
		tierMap[pid] = append(tierMap[pid], salesapp.ProductTierOption{ID: tid, SpecG: specG, MinQty: min, MaxQty: max, UnitPrice: price})
	}
	if err := trs.Err(); err != nil {
		return nil, err
	}

	for i := range out {
		out[i].Tiers = tierMap[out[i].ID]
	}
	return out, nil
}

func (r Repository) fetchOrderEdit(ctx context.Context, id int64) (*salesapp.OrderEditData, error) {
	q := fmt.Sprintf(`
		SELECT
			o.id,
			o.order_no,
			to_char(o.order_date,'YYYY-MM-DD') as order_date,
			COALESCE(o.customer_id,0) as customer_id,
			COALESCE(o.source_id,0) as source_id,
			COALESCE(o.order_type_id,0) as order_type_id,
			COALESCE(o.pay_status_id,0) as pay_status_id,
			COALESCE(o.payment_method,'') as payment_method,
			COALESCE(o.ship_status_id,0) as ship_status_id,
			COALESCE(o.ship_method,'') as ship_method,
			%s as ship_tracking_no,
			COALESCE(o.responsible_party_type,'') as responsible_party_type,
			COALESCE(o.responsible_party_id,0) as responsible_party_id,
			COALESCE(o.responsible_party_name,'') as responsible_party_name,
			COALESCE(o.notes,'') as notes,
			COALESCE(o.total_amount,0) as total_amount,
			COALESCE(o.shipping_amount,0) as shipping_amount,
			COALESCE(o.discount_amount,0) as discount_amount,
			COALESCE(o.round_to_int,false) as round_to_int,
			COALESCE(o.rounding_amount,0) as rounding_amount,
			COALESCE(o.grand_total,0) as grand_total,
			COALESCE(o.express_fee,'') as express_fee,
			COALESCE(o.outsource_material_fee,0) as outsource_material_fee,
			COALESCE(o.outsource_roast_fee,0) as outsource_roast_fee,
			COALESCE(o.outsource_packaging_fee,0) as outsource_packaging_fee,
			COALESCE(o.outsource_manual_fee,0) as outsource_manual_fee,
			COALESCE(o.outsource_tax_fee,0) as outsource_tax_fee,
			COALESCE(o.outsource_other_fee,0) as outsource_other_fee,
			COALESCE(o.outsource_total_fee,0) as outsource_total_fee,
			o.is_void,
			CASE WHEN o.voided_at IS NULL THEN NULL ELSE to_char(o.voided_at, 'YYYY-MM-DD HH24:MI:SS') END AS voided_at,
			o.void_reason
		FROM %s.orders o
		WHERE o.id=$1
	`, orderTrackingSummaryExpr(r.schema, "o"), r.schema)

	var d salesapp.OrderEditData
	var totalAmt, shipAmt, discAmt, roundAmt, grandAmt float64
	var outsourceMaterial, outsourceRoast, outsourcePackaging, outsourceManual, outsourceTax, outsourceOther, outsourceTotal float64
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&d.ID,
		&d.OrderNo,
		&d.OrderDate,
		&d.CustomerID,
		&d.SourceID,
		&d.OrderTypeID,
		&d.PayStatusID,
		&d.PaymentMethod,
		&d.ShipStatusID,
		&d.ShipMethod,
		&d.ShipTrackingNo,
		&d.ResponsibleType,
		&d.ResponsibleID,
		&d.ResponsibleName,
		&d.Notes,
		&totalAmt,
		&shipAmt,
		&discAmt,
		&d.RoundToInt,
		&roundAmt,
		&grandAmt,
		&d.ExpressFee,
		&outsourceMaterial,
		&outsourceRoast,
		&outsourcePackaging,
		&outsourceManual,
		&outsourceTax,
		&outsourceOther,
		&outsourceTotal,
		&d.IsVoid,
		&d.VoidedAt,
		&d.VoidReason,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	d.TotalAmount = fmt.Sprintf("%.2f", totalAmt)
	d.ShippingAmount = fmt.Sprintf("%.2f", shipAmt)
	d.DiscountAmount = fmt.Sprintf("%.2f", discAmt)
	d.RoundingAmount = fmt.Sprintf("%.2f", roundAmt)
	d.GrandTotal = fmt.Sprintf("%.2f", grandAmt)
	d.OutsourceMaterialFee = fmt.Sprintf("%.2f", outsourceMaterial)
	d.OutsourceRoastFee = fmt.Sprintf("%.2f", outsourceRoast)
	d.OutsourcePackagingFee = fmt.Sprintf("%.2f", outsourcePackaging)
	d.OutsourceManualFee = fmt.Sprintf("%.2f", outsourceManual)
	d.OutsourceTaxFee = fmt.Sprintf("%.2f", outsourceTax)
	d.OutsourceOtherFee = fmt.Sprintf("%.2f", outsourceOther)
	d.OutsourceTotalFee = fmt.Sprintf("%.2f", outsourceTotal)

	itemsQ := fmt.Sprintf(`
		SELECT oi.id, oi.line_no,
			COALESCE(oi.product_id,0),
			COALESCE(p.name,''),
			COALESCE(oi.item_note,''),
			COALESCE(oi.spec,''),
			COALESCE(oi.qty,0),
			COALESCE(oi.unit,''),
				COALESCE(oi.unit_price,0),
				COALESCE(oi.line_total,0),
				COALESCE(oi.price_tier_id,0),
				COALESCE(oi.discount_type,''),
				COALESCE(oi.discount_value,0),
				COALESCE(oi.discount_amount,0)
		FROM %s.order_items oi
		LEFT JOIN %s.products p ON p.id=oi.product_id
		WHERE oi.order_id=$1
		ORDER BY oi.line_no, oi.id
	`, r.schema, r.schema)
	rows, err := r.pool.Query(ctx, itemsQ, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	d.Items = make([]salesapp.OrderEditItem, 0)
	for rows.Next() {
		var it salesapp.OrderEditItem
		var qty, unitPrice, lineTotal, discountValue, discountAmount float64
		if err := rows.Scan(&it.ItemID, &it.LineNo, &it.ProductID, &it.Product, &it.Note, &it.Spec, &qty, &it.Unit, &unitPrice, &lineTotal, &it.PriceTierID, &it.DiscountType, &discountValue, &discountAmount); err != nil {
			return nil, err
		}
		it.Qty = trimFloatZero(qty)
		it.UnitPrice = fmt.Sprintf("%.2f", unitPrice)
		it.LineTotal = fmt.Sprintf("%.2f", lineTotal)
		it.DiscountValue = trimFloatZero(discountValue)
		it.DiscountAmount = fmt.Sprintf("%.2f", discountAmount)
		d.Items = append(d.Items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &d, nil
}

func (r Repository) LoadSenderProfile(ctx context.Context) (salesapp.SenderProfile, error) {
	profile := salesapp.SenderProfile{
		Label:     env("SENDER_LABEL", "默认寄件人"),
		Name:      env("SENDER_NAME", ""),
		Phone:     env("SENDER_PHONE", ""),
		Addr:      env("SENDER_ADDR", ""),
		Company:   env("SENDER_COMPANY", ""),
		Goods:     env("SENDER_GOODS", "茶叶"),
		BizType:   env("SF_BIZ_TYPE", ""),
		IsDefault: true,
		Active:    true,
	}
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, COALESCE(sender_label,''), COALESCE(sender_name,''), COALESCE(sender_phone,''), COALESCE(sender_addr,''), COALESCE(sender_company,''), COALESCE(sender_goods,''), COALESCE(sf_biz_type,''), is_default, active
		FROM %s.sender_settings
		WHERE active=true
		ORDER BY is_default DESC, id
		LIMIT 1
	`, r.schema)).Scan(
		&profile.ID, &profile.Label, &profile.Name, &profile.Phone, &profile.Addr, &profile.Company, &profile.Goods, &profile.BizType, &profile.IsDefault, &profile.Active,
	)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return salesapp.SenderProfile{}, err
	}
	return profile, nil
}

func (r Repository) LoadSenderProfileByID(ctx context.Context, id int64) (salesapp.SenderProfile, error) {
	var profile salesapp.SenderProfile
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, COALESCE(sender_label,''), COALESCE(sender_name,''), COALESCE(sender_phone,''), COALESCE(sender_addr,''), COALESCE(sender_company,''), COALESCE(sender_goods,''), COALESCE(sf_biz_type,''), is_default, active
		FROM %s.sender_settings
		WHERE id=$1 AND active=true
	`, r.schema), id).Scan(
		&profile.ID, &profile.Label, &profile.Name, &profile.Phone, &profile.Addr, &profile.Company, &profile.Goods, &profile.BizType, &profile.IsDefault, &profile.Active,
	)
	if err != nil {
		return salesapp.SenderProfile{}, err
	}
	return profile, nil
}

func (r Repository) ListSenderProfiles(ctx context.Context) ([]salesapp.SenderProfile, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, COALESCE(sender_label,''), COALESCE(sender_name,''), COALESCE(sender_phone,''), COALESCE(sender_addr,''), COALESCE(sender_company,''), COALESCE(sender_goods,''), COALESCE(sf_biz_type,''), is_default, active
		FROM %s.sender_settings
		WHERE active=true
		ORDER BY is_default DESC, id
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]salesapp.SenderProfile, 0)
	for rows.Next() {
		var profile salesapp.SenderProfile
		if err := rows.Scan(&profile.ID, &profile.Label, &profile.Name, &profile.Phone, &profile.Addr, &profile.Company, &profile.Goods, &profile.BizType, &profile.IsDefault, &profile.Active); err != nil {
			return nil, err
		}
		out = append(out, profile)
	}
	return out, rows.Err()
}

func (r Repository) SaveSenderProfile(ctx context.Context, profile salesapp.SenderProfile) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if profile.IsDefault {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.sender_settings SET is_default=false WHERE is_default=true`, r.schema)); err != nil {
			return err
		}
	}
	if profile.ID > 0 {
		_, err = tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.sender_settings
			SET sender_label=$2,sender_name=$3,sender_phone=$4,sender_addr=$5,sender_company=$6,sender_goods=$7,sf_biz_type=$8,is_default=$9,active=$10,updated_at=now()
			WHERE id=$1
		`, r.schema), profile.ID, profile.Label, profile.Name, profile.Phone, profile.Addr, profile.Company, profile.Goods, profile.BizType, profile.IsDefault, profile.Active)
	} else {
		_, err = tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.sender_settings(id, sender_label, sender_name, sender_phone, sender_addr, sender_company, sender_goods, sf_biz_type, is_default, active)
			VALUES ((SELECT COALESCE(MAX(id),0)+1 FROM %s.sender_settings), $1,$2,$3,$4,$5,$6,$7,$8,$9)
		`, r.schema, r.schema), profile.Label, profile.Name, profile.Phone, profile.Addr, profile.Company, profile.Goods, profile.BizType, profile.IsDefault, profile.Active)
	}
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.sender_settings
		SET is_default=true, active=true
		WHERE id=(SELECT id FROM %s.sender_settings WHERE active=true ORDER BY is_default DESC, id LIMIT 1)
		  AND NOT EXISTS (SELECT 1 FROM %s.sender_settings WHERE is_default=true AND active=true)
	`, r.schema, r.schema, r.schema)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func EnsureSenderSettingsTable(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.sender_settings (
		id SMALLINT PRIMARY KEY DEFAULT 1,
		sender_label TEXT NOT NULL DEFAULT '',
		sender_name TEXT NOT NULL DEFAULT '',
		sender_phone TEXT NOT NULL DEFAULT '',
		sender_addr TEXT NOT NULL DEFAULT '',
		sender_company TEXT NOT NULL DEFAULT '',
		sender_goods TEXT NOT NULL DEFAULT '茶叶',
		sf_biz_type TEXT NOT NULL DEFAULT '',
		is_default BOOLEAN NOT NULL DEFAULT false,
		active BOOLEAN NOT NULL DEFAULT true,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);
	ALTER TABLE %s.sender_settings ADD COLUMN IF NOT EXISTS sender_label TEXT NOT NULL DEFAULT '';
	ALTER TABLE %s.sender_settings ADD COLUMN IF NOT EXISTS is_default BOOLEAN NOT NULL DEFAULT false;
	ALTER TABLE %s.sender_settings ADD COLUMN IF NOT EXISTS active BOOLEAN NOT NULL DEFAULT true;
	INSERT INTO %s.sender_settings(id, sender_label, is_default, active) VALUES(1, '默认寄件人', true, true) ON CONFLICT (id) DO NOTHING;
	UPDATE %s.sender_settings
	SET sender_label=COALESCE(NULLIF(sender_label,''), NULLIF(sender_name,''), '默认寄件人');
	UPDATE %s.sender_settings
	SET is_default=true, active=true
	WHERE id=(SELECT id FROM %s.sender_settings ORDER BY is_default DESC, id LIMIT 1)
	  AND NOT EXISTS (SELECT 1 FROM %s.sender_settings WHERE is_default=true AND active=true);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_%s_sender_settings_one_default ON %s.sender_settings ((is_default)) WHERE is_default=true AND active=true;`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema)
	_, err := pool.Exec(ctx, q)
	return err
}

func trimFloatZero(v float64) string {
	s := strconv.FormatFloat(v, 'f', 4, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" {
		return "0"
	}
	return s
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
