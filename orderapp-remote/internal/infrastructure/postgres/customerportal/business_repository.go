package customerportal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	customerportalapp "orderapp/internal/application/customerportal"
	catalogdomain "orderapp/internal/domain/catalog"
	salesdomain "orderapp/internal/domain/sales"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	"orderapp/internal/infrastructure/postgres/orderbeans"

	"github.com/jackc/pgx/v5"
)

func (r Repository) LoadServicePage(ctx context.Context, query customerportalapp.ServicePageQuery) (customerportalapp.ServicePage, error) {
	limit := query.Limit
	defaultLimit := 20
	maxLimit := 50
	if query.Key == customerportalapp.ServiceKeySettlement {
		defaultLimit = 200
		maxLimit = 200
	}
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	page := customerportalapp.ServicePage{Key: query.Key}
	var err error
	switch query.Key {
	case customerportalapp.ServiceKeyBeanList:
		page.BeanLists, page.BeanListVersions, page.HasBeanListVersions, err = r.loadBeanListServiceData(ctx, query.CustomerID, limit)
	case customerportalapp.ServiceKeyOrders:
		page.Orders, err = r.listCustomerOrders(ctx, query, limit, true)
	case customerportalapp.ServiceKeyProductOrder:
		if page.Products, err = r.listProducts(ctx, query.CustomerID, limit); err != nil {
			return customerportalapp.ServicePage{}, err
		}
		page.Orders, err = r.listCustomerOrders(ctx, query, limit, true)
	case customerportalapp.ServiceKeyDirectShip:
		if page.Products, err = r.listProducts(ctx, query.CustomerID, limit); err != nil {
			return customerportalapp.ServicePage{}, err
		}
		if page.DirectShipBatches, err = r.listDirectShipBatches(ctx, query.CustomerID, limit); err != nil {
			return customerportalapp.ServicePage{}, err
		}
		page.Orders, err = r.listCustomerOrders(ctx, query, limit, true)
	case customerportalapp.ServiceKeyProcessing:
		if page.Products, err = r.listProducts(ctx, query.CustomerID, limit); err != nil {
			return customerportalapp.ServicePage{}, err
		}
		if page.Inventory, err = r.listInventory(ctx, query.CustomerID, limit); err != nil {
			return customerportalapp.ServicePage{}, err
		}
		page.ProcessingRequests, err = r.listProcessingRequests(ctx, query.CustomerID, limit)
	case customerportalapp.ServiceKeyInventory:
		page.Inventory, err = r.listInventory(ctx, query.CustomerID, limit)
	case customerportalapp.ServiceKeyShipping:
		page.Orders, err = r.listCustomerOrders(ctx, query, limit, true)
	case customerportalapp.ServiceKeySettlement:
		if page.FeeItems, err = r.listFeeItems(ctx, query.CustomerID, limit); err != nil {
			return customerportalapp.ServicePage{}, err
		}
		if page.SettlementBatches, err = r.listSettlementBatches(ctx, query.CustomerID, limit); err != nil {
			return customerportalapp.ServicePage{}, err
		}
		page.Orders, err = r.listCustomerOrders(ctx, query, limit, false)
	default:
		err = fmt.Errorf("service key invalid")
	}
	if err != nil {
		return customerportalapp.ServicePage{}, err
	}
	return page, nil
}

func (r Repository) LoadMallPage(ctx context.Context, customerID int64) (customerportalapp.MallPage, error) {
	page := customerportalapp.MallPage{
		ThemeKey:         customerportalapp.PortalThemeCoffeeFactory,
		MiniappEntryMode: customerportalapp.MiniappEntryModeServices,
		Products:         []customerportalapp.MallProduct{},
	}
	_ = r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(NULLIF(p.theme_key,''),'coffee_factory'),
		       COALESCE(NULLIF(p.miniapp_entry_mode,''),'services'),
		       COALESCE(NULLIF(p.display_name,''), c.name, '')
		FROM %s.customers c
		LEFT JOIN %s.customer_portal_profiles p ON p.customer_id=c.id
		WHERE c.id=$1
	`, r.schema, r.schema), customerID).Scan(&page.ThemeKey, &page.MiniappEntryMode, &page.CurrentCustomerName)
	page.ThemeKey = customerportalapp.NormalizePortalThemeKey(page.ThemeKey)
	page.MiniappEntryMode = customerportalapp.NormalizeMiniappEntryMode(page.MiniappEntryMode)
	page.CurrentCustomerID = customerID

	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT m.id, m.product_id, COALESCE(p.name,''), COALESCE(NULLIF(m.title,''), p.name, ''), m.subtitle, m.description,
		       COALESCE(NULLIF(p.product_kind,''), 'roasted_bean'),
		       COALESCE(p.drip_bag_grams,10)::float8,
		       COALESCE(p.drip_box_bag_count,10),
		       m.image_url, m.spec_g, m.unit_price, m.template_key, m.status, m.sort_order,
		       to_char(m.updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.mall_products m
		JOIN %s.products p ON p.id=m.product_id
		WHERE p.active=true AND m.status='published'
		  AND %s
		ORDER BY m.sort_order, m.id
	`, r.schema, r.schema, mallProductPublicCatalogSQL("p")))
	if err != nil {
		return customerportalapp.MallPage{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var row customerportalapp.MallProduct
		if err := rows.Scan(&row.ID, &row.ProductID, &row.ProductName, &row.Title, &row.Subtitle, &row.Description, &row.ProductKind, &row.DripBagGrams, &row.DripBoxBagCount, &row.ImageURL, &row.SpecG, &row.UnitPrice, &row.TemplateKey, &row.Status, &row.SortOrder, &row.UpdatedAt); err != nil {
			return customerportalapp.MallPage{}, err
		}
		row.ProductKind = catalogdomain.NormalizeProductKind(row.ProductKind)
		if row.ProductKind == catalogdomain.ProductKindDripBag {
			row.SalesUnits = []string{"bag", "box"}
			row.MallPrice = row.UnitPrice
		}
		row.TemplateKey = customerportalapp.NormalizeMallTemplateKey(row.TemplateKey)
		row.Status = customerportalapp.NormalizeMallProductStatus(row.Status)
		page.Products = append(page.Products, row)
	}
	return page, rows.Err()
}

func (r Repository) LoadBeanListPublication(ctx context.Context, customerID, publicationID int64) (customerportalapp.BeanListSummary, error) {
	var row customerportalapp.BeanListSummary
	var configJSON []byte
	var contentJSON []byte
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, list_type, version_no, status, to_char(published_at,'YYYY-MM-DD HH24:MI'), changelog, config_json, content_json
		FROM %s.bean_list_publications
		WHERE id=$1
		  AND COALESCE(NULLIF(publication_purpose,''),'factory_supply')='factory_supply'
		  AND status='published'
		  AND ((owner_type='customer' AND owner_key=$2) OR owner_type='official')
	`, r.schema), publicationID, fmt.Sprintf("%d", customerID)).
		Scan(&row.ID, &row.ListType, &row.VersionNo, &row.Status, &row.PublishedAt, &row.Changelog, &configJSON, &contentJSON)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return customerportalapp.BeanListSummary{}, customerportalapp.ErrBeanListPublicationNotFound
		}
		return customerportalapp.BeanListSummary{}, err
	}
	if err := parseBeanListDisplaySummary(configJSON, contentJSON, &row); err != nil {
		return customerportalapp.BeanListSummary{}, err
	}
	return row, nil
}

func (r Repository) LoadBeanListPublicationAsset(ctx context.Context, publicationID int64, assetType string) (customerportalapp.BeanListPublicationAsset, error) {
	assetType = strings.TrimSpace(assetType)
	if assetType == "" {
		assetType = "pdf"
	}
	var asset customerportalapp.BeanListPublicationAsset
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT publication_id, asset_type, content_type, cache_key, payload
		FROM %s.bean_list_publication_assets
		WHERE publication_id=$1 AND asset_type=$2
	`, r.schema), publicationID, assetType).Scan(&asset.PublicationID, &asset.AssetType, &asset.ContentType, &asset.CacheKey, &asset.Payload)
	if err != nil {
		return customerportalapp.BeanListPublicationAsset{}, err
	}
	return asset, nil
}

func (r Repository) SaveBeanListPublicationAsset(ctx context.Context, asset customerportalapp.BeanListPublicationAsset, actor string) (customerportalapp.BeanListPublicationAsset, error) {
	asset.AssetType = strings.TrimSpace(asset.AssetType)
	if asset.AssetType == "" {
		asset.AssetType = "pdf"
	}
	asset.ContentType = strings.TrimSpace(asset.ContentType)
	if asset.ContentType == "" {
		asset.ContentType = "application/octet-stream"
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return customerportalapp.BeanListPublicationAsset{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var created bool
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		WITH inserted AS (
			INSERT INTO %[1]s.bean_list_publication_assets(publication_id, asset_type, content_type, cache_key, payload, created_by)
			VALUES($1,$2,$3,$4,$5,$6)
			ON CONFLICT(publication_id, asset_type) DO NOTHING
			RETURNING publication_id, asset_type, content_type, cache_key, payload, true
		)
		SELECT publication_id, asset_type, content_type, cache_key, payload, true FROM inserted
		UNION ALL
		SELECT publication_id, asset_type, content_type, cache_key, payload, false
		FROM %[1]s.bean_list_publication_assets
		WHERE publication_id=$1 AND asset_type=$2 AND NOT EXISTS (SELECT 1 FROM inserted)
		LIMIT 1
	`, r.schema), asset.PublicationID, asset.AssetType, asset.ContentType, asset.CacheKey, asset.Payload, strings.TrimSpace(actor)).
		Scan(&asset.PublicationID, &asset.AssetType, &asset.ContentType, &asset.CacheKey, &asset.Payload, &created)
	if err != nil {
		return customerportalapp.BeanListPublicationAsset{}, err
	}
	if created {
		if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, strings.TrimSpace(actor), "bean_list_publication_asset", &asset.PublicationID, "create", postgresinfra.StrPtr("asset_type"), nil, postgresinfra.StrPtr(asset.AssetType), postgresinfra.AuditMeta{
			"publication_id": asset.PublicationID,
			"asset_type":     asset.AssetType,
			"cache_key":      asset.CacheKey,
		}); err != nil {
			return customerportalapp.BeanListPublicationAsset{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return customerportalapp.BeanListPublicationAsset{}, err
	}
	return asset, nil
}

func (r Repository) AcknowledgeBeanListPublication(ctx context.Context, customerID, publicationID int64, actor string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_bean_list_acknowledgements(customer_id, publication_id, acknowledged_by)
		VALUES($1,$2,$3)
		ON CONFLICT(customer_id, publication_id) DO NOTHING
	`, r.schema), customerID, publicationID, strings.TrimSpace(actor))
	if err != nil {
		return err
	}
	if tag.RowsAffected() > 0 {
		if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, strings.TrimSpace(actor), "customer_bean_list_acknowledgement", &publicationID, "acknowledge", postgresinfra.StrPtr("publication_id"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", publicationID)), postgresinfra.AuditMeta{
			"customer_id":    customerID,
			"publication_id": publicationID,
		}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r Repository) LoadResaleBeanListPage(ctx context.Context, customerID int64) (customerportalapp.ResaleBeanListPage, error) {
	factoryRows, err := r.listFactorySupplyBeanLists(ctx, customerID, 50)
	if err != nil {
		return customerportalapp.ResaleBeanListPage{}, err
	}
	resaleRows, err := r.ListCustomerResaleBeanListVersions(ctx, customerID, 50)
	if err != nil {
		return customerportalapp.ResaleBeanListPage{}, err
	}
	templates, err := r.listAuthorizedResaleGradientTemplates(ctx, customerID)
	if err != nil {
		return customerportalapp.ResaleBeanListPage{}, err
	}
	return customerportalapp.ResaleBeanListPage{
		FactorySupplyBeanLists:   factoryRows,
		CustomerResaleBeanLists:  resaleRows,
		GradientTemplates:        templates,
		FactoryPriceTableGroups:  customerPriceTableGroups(factoryRows, nil, false),
		CustomerPriceTableGroups: customerPriceTableGroups(resaleRows, nil, true),
	}, nil
}

func (r Repository) LoadCustomerProductsPage(ctx context.Context, customerID int64) (customerportalapp.CustomerProductsPage, error) {
	template, err := r.currentCustomerProductClassificationTemplate(ctx, customerID)
	if err != nil {
		return customerportalapp.CustomerProductsPage{}, err
	}
	categories, err := r.listCustomerProductCategories(ctx, template.ID)
	if err != nil {
		return customerportalapp.CustomerProductsPage{}, err
	}
	products, err := r.listCustomerProductSummaries(ctx, customerID, template.ID)
	if err != nil {
		return customerportalapp.CustomerProductsPage{}, err
	}
	categories = attachCustomerProductCategoryCounts(categories, products)
	productCounts := customerProductCountsByListType(products)
	factoryRows, err := r.listFactorySupplyBeanLists(ctx, customerID, 50)
	if err != nil {
		return customerportalapp.CustomerProductsPage{}, err
	}
	resaleRows, err := r.ListCustomerResaleBeanListVersions(ctx, customerID, 100)
	if err != nil {
		return customerportalapp.CustomerProductsPage{}, err
	}
	return customerportalapp.CustomerProductsPage{
		ClassificationTemplate:   template,
		Categories:               categories,
		Products:                 products,
		FactoryPriceTableGroups:  customerPriceTableGroups(factoryRows, productCounts, false),
		CustomerPriceTableGroups: customerPriceTableGroups(resaleRows, productCounts, true),
	}, nil
}

func (r Repository) CreateCustomerProductCategory(ctx context.Context, cmd customerportalapp.CustomerProductCategoryCommand) (customerportalapp.CustomerProductCategory, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return customerportalapp.CustomerProductCategory{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	template, idMap, err := r.ensureCustomerProductClassificationTemplate(ctx, tx, cmd.CustomerID, cmd.Actor)
	if err != nil {
		return customerportalapp.CustomerProductCategory{}, err
	}
	parentID := cmd.ParentID
	if parentID > 0 {
		if mapped := idMap[parentID]; mapped > 0 {
			parentID = mapped
		}
	}
	level := 1
	if parentID > 0 {
		level = 2
		if err := r.ensureCustomerCategoryBelongsToTemplate(ctx, tx, template.ID, parentID); err != nil {
			return customerportalapp.CustomerProductCategory{}, err
		}
	}
	sortOrder := cmd.SortOrder
	if sortOrder <= 0 {
		sortOrder = r.nextCustomerCategorySortOrder(ctx, tx, template.ID, parentID)
	}
	var id int64
	name := strings.TrimSpace(cmd.Name)
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_classification_template_categories(template_id, parent_id, name, level, sort_order, active)
		VALUES($1,$2,$3,$4,$5,true)
		RETURNING id
	`, r.schema), template.ID, parentID, name, level, sortOrder).Scan(&id); err != nil {
		return customerportalapp.CustomerProductCategory{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product_classification_category", &id, "mini_create_customer_product_category", postgresinfra.StrPtr("name"), nil, postgresinfra.StrPtr(name), postgresinfra.AuditMeta{"customer_id": cmd.CustomerID, "template_id": template.ID, "parent_id": parentID, "sort_order": sortOrder}); err != nil {
		return customerportalapp.CustomerProductCategory{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return customerportalapp.CustomerProductCategory{}, err
	}
	return customerportalapp.CustomerProductCategory{ID: id, TemplateID: template.ID, ParentID: parentID, Name: name, Level: level, SortOrder: sortOrder}, nil
}

func (r Repository) UpdateCustomerProductCategory(ctx context.Context, cmd customerportalapp.CustomerProductCategoryCommand) (customerportalapp.CustomerProductCategory, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return customerportalapp.CustomerProductCategory{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	template, idMap, err := r.ensureCustomerProductClassificationTemplate(ctx, tx, cmd.CustomerID, cmd.Actor)
	if err != nil {
		return customerportalapp.CustomerProductCategory{}, err
	}
	categoryID := customerCategoryIDAfterDerive(cmd.ID, idMap)
	name := strings.TrimSpace(cmd.Name)
	var row customerportalapp.CustomerProductCategory
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		UPDATE %s.product_classification_template_categories
		SET name=$3, updated_at=now()
		WHERE id=$1 AND template_id=$2 AND active=true
		RETURNING id, template_id, COALESCE(parent_id,0), name, COALESCE(level,1), COALESCE(sort_order,100)
	`, r.schema), categoryID, template.ID, name).Scan(&row.ID, &row.TemplateID, &row.ParentID, &row.Name, &row.Level, &row.SortOrder); err != nil {
		return customerportalapp.CustomerProductCategory{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product_classification_category", &row.ID, "mini_update_customer_product_category", postgresinfra.StrPtr("name"), nil, postgresinfra.StrPtr(name), postgresinfra.AuditMeta{"customer_id": cmd.CustomerID, "template_id": template.ID}); err != nil {
		return customerportalapp.CustomerProductCategory{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return customerportalapp.CustomerProductCategory{}, err
	}
	return row, nil
}

func (r Repository) DeleteCustomerProductCategory(ctx context.Context, customerID, categoryID int64, actor string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	template, idMap, err := r.ensureCustomerProductClassificationTemplate(ctx, tx, customerID, actor)
	if err != nil {
		return err
	}
	categoryID = customerCategoryIDAfterDerive(categoryID, idMap)
	tag, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.product_classification_template_categories
		SET active=false, updated_at=now()
		WHERE id=$1 AND template_id=$2 AND active=true
	`, r.schema), categoryID, template.ID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("classification category not found")
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.customer_product_alias_classification_assignments
		SET category_id=0, updated_at=now(), updated_by=$3
		WHERE template_id=$1 AND category_id=$2
	`, r.schema), template.ID, categoryID, actor); err != nil {
		return err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "product_classification_category", &categoryID, "mini_delete_customer_product_category", postgresinfra.StrPtr("active"), postgresinfra.StrPtr("true"), postgresinfra.StrPtr("false"), postgresinfra.AuditMeta{"customer_id": customerID, "template_id": template.ID}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) MoveCustomerProductCategory(ctx context.Context, cmd customerportalapp.CustomerProductCategoryMoveCommand) (customerportalapp.CustomerProductCategory, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return customerportalapp.CustomerProductCategory{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	template, idMap, err := r.ensureCustomerProductClassificationTemplate(ctx, tx, cmd.CustomerID, cmd.Actor)
	if err != nil {
		return customerportalapp.CustomerProductCategory{}, err
	}
	categoryID := customerCategoryIDAfterDerive(cmd.ID, idMap)
	parentID := cmd.ParentID
	if parentID > 0 {
		if mapped := idMap[parentID]; mapped > 0 {
			parentID = mapped
		}
		if err := r.ensureCustomerCategoryBelongsToTemplate(ctx, tx, template.ID, parentID); err != nil {
			return customerportalapp.CustomerProductCategory{}, err
		}
	}
	var currentParent int64
	var currentSort int
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(parent_id,0), COALESCE(sort_order,100)
		FROM %s.product_classification_template_categories
		WHERE id=$1 AND template_id=$2 AND active=true
	`, r.schema), categoryID, template.ID).Scan(&currentParent, &currentSort); err != nil {
		return customerportalapp.CustomerProductCategory{}, err
	}
	if strings.TrimSpace(cmd.Direction) == "" && cmd.SortOrder <= 0 && cmd.ParentID <= 0 {
		return customerportalapp.CustomerProductCategory{}, fmt.Errorf("move direction required")
	}
	if cmd.ParentID <= 0 {
		parentID = currentParent
	}
	sortOrder := cmd.SortOrder
	if sortOrder <= 0 {
		sortOrder = currentSort
		switch strings.TrimSpace(cmd.Direction) {
		case "up":
			sortOrder -= 10
		case "down":
			sortOrder += 10
		}
		if sortOrder <= 0 {
			sortOrder = 10
		}
	}
	level := 1
	if parentID > 0 {
		level = 2
	}
	var row customerportalapp.CustomerProductCategory
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		UPDATE %s.product_classification_template_categories
		SET parent_id=$3, level=$4, sort_order=$5, updated_at=now()
		WHERE id=$1 AND template_id=$2 AND active=true
		RETURNING id, template_id, COALESCE(parent_id,0), name, COALESCE(level,1), COALESCE(sort_order,100)
	`, r.schema), categoryID, template.ID, parentID, level, sortOrder).Scan(&row.ID, &row.TemplateID, &row.ParentID, &row.Name, &row.Level, &row.SortOrder); err != nil {
		return customerportalapp.CustomerProductCategory{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "product_classification_category", &row.ID, "mini_move_customer_product_category", postgresinfra.StrPtr("sort_order"), postgresinfra.StrPtr(fmt.Sprintf("%d", currentSort)), postgresinfra.StrPtr(fmt.Sprintf("%d", sortOrder)), postgresinfra.AuditMeta{"customer_id": cmd.CustomerID, "template_id": template.ID, "parent_id": parentID, "direction": cmd.Direction}); err != nil {
		return customerportalapp.CustomerProductCategory{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return customerportalapp.CustomerProductCategory{}, err
	}
	return row, nil
}

func (r Repository) AssignCustomerProductCategory(ctx context.Context, cmd customerportalapp.CustomerProductCategoryAssignmentCommand) (customerportalapp.CustomerProductSummary, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return customerportalapp.CustomerProductSummary{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	template, idMap, err := r.ensureCustomerProductClassificationTemplate(ctx, tx, cmd.CustomerID, cmd.Actor)
	if err != nil {
		return customerportalapp.CustomerProductSummary{}, err
	}
	categoryID := cmd.CategoryID
	if categoryID > 0 {
		if mapped := idMap[categoryID]; mapped > 0 {
			categoryID = mapped
		}
		if err := r.ensureCustomerCategoryBelongsToTemplate(ctx, tx, template.ID, categoryID); err != nil {
			return customerportalapp.CustomerProductSummary{}, err
		}
	}
	var aliasCustomerID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT customer_id FROM %s.customer_product_aliases
		WHERE id=$1 AND active=true
	`, r.schema), cmd.ProductID).Scan(&aliasCustomerID); err != nil {
		return customerportalapp.CustomerProductSummary{}, err
	}
	if aliasCustomerID != cmd.CustomerID {
		return customerportalapp.CustomerProductSummary{}, fmt.Errorf("customer product not found")
	}
	sortOrder := cmd.SortOrder
	if sortOrder <= 0 {
		sortOrder = 100
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		DELETE FROM %s.customer_product_alias_classification_assignments
		WHERE alias_id=$1 AND template_id<>$2
	`, r.schema), cmd.ProductID, template.ID); err != nil {
		return customerportalapp.CustomerProductSummary{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_product_alias_classification_assignments(alias_id, template_id, category_id, sort_order, updated_by, created_at, updated_at)
		VALUES($1,$2,$3,$4,$5,now(),now())
		ON CONFLICT(alias_id, template_id) DO UPDATE SET
			category_id=excluded.category_id,
			sort_order=excluded.sort_order,
			updated_by=excluded.updated_by,
			updated_at=now()
	`, r.schema), cmd.ProductID, template.ID, categoryID, sortOrder, cmd.Actor); err != nil {
		return customerportalapp.CustomerProductSummary{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "customer_product_alias_classification_assignment", &cmd.ProductID, "mini_assign_customer_product_category", postgresinfra.StrPtr("category_id"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", categoryID)), postgresinfra.AuditMeta{"customer_id": cmd.CustomerID, "template_id": template.ID, "category_id": categoryID}); err != nil {
		return customerportalapp.CustomerProductSummary{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return customerportalapp.CustomerProductSummary{}, err
	}
	products, err := r.listCustomerProductSummaries(ctx, cmd.CustomerID, template.ID)
	if err != nil {
		return customerportalapp.CustomerProductSummary{}, err
	}
	for _, product := range products {
		if product.ID == cmd.ProductID {
			return product, nil
		}
	}
	return customerportalapp.CustomerProductSummary{ID: cmd.ProductID, CategoryID: categoryID}, nil
}

func (r Repository) LoadResaleBeanListEditor(ctx context.Context, customerID, sourcePublicationID int64) (customerportalapp.ResaleBeanListEditor, error) {
	source, err := r.loadFactorySupplyBeanListPublication(ctx, customerID, sourcePublicationID)
	if err != nil {
		return customerportalapp.ResaleBeanListEditor{}, err
	}
	versions, err := r.ListCustomerResaleBeanListVersions(ctx, customerID, 100)
	if err != nil {
		return customerportalapp.ResaleBeanListEditor{}, err
	}
	templates, err := r.listAuthorizedResaleGradientTemplates(ctx, customerID)
	if err != nil {
		return customerportalapp.ResaleBeanListEditor{}, err
	}
	return customerportalapp.ResaleBeanListEditor{
		Source:            source,
		NextVersionNo:     nextResaleBeanListVersionForRepository(versions),
		GradientTemplates: templates,
	}, nil
}

func (r Repository) LoadResaleBeanListPublication(ctx context.Context, customerID, publicationID int64) (customerportalapp.BeanListSummary, error) {
	var row customerportalapp.BeanListSummary
	var configJSON []byte
	var contentJSON []byte
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, list_type, version_no, status, to_char(published_at,'YYYY-MM-DD HH24:MI'), changelog, config_json, content_json
		FROM %s.bean_list_publications
		WHERE id=$1
		  AND COALESCE(NULLIF(publication_purpose,''),'factory_supply')='customer_resale'
		  AND owner_type='customer'
		  AND owner_key=$2
		  AND status='published'
	`, r.schema), publicationID, fmt.Sprintf("%d", customerID)).
		Scan(&row.ID, &row.ListType, &row.VersionNo, &row.Status, &row.PublishedAt, &row.Changelog, &configJSON, &contentJSON)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return customerportalapp.BeanListSummary{}, customerportalapp.ErrBeanListPublicationNotFound
		}
		return customerportalapp.BeanListSummary{}, err
	}
	if err := parseBeanListDisplaySummary(configJSON, contentJSON, &row); err != nil {
		return customerportalapp.BeanListSummary{}, err
	}
	return row, nil
}

func (r Repository) LoadAuthorizedResaleGradientTemplate(ctx context.Context, customerID, templateID int64) (customerportalapp.ResaleGradientTemplate, error) {
	templates, err := r.listAuthorizedResaleGradientTemplates(ctx, customerID)
	if err != nil {
		return customerportalapp.ResaleGradientTemplate{}, err
	}
	for _, row := range templates {
		if row.ID == templateID {
			return row, nil
		}
	}
	return customerportalapp.ResaleGradientTemplate{}, customerportalapp.ErrResaleGradientTemplateNotFound
}

func (r Repository) ListCustomerResaleBeanListVersions(ctx context.Context, customerID int64, limit int) ([]customerportalapp.BeanListSummary, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, list_type, version_no, status, to_char(published_at,'YYYY-MM-DD HH24:MI'), changelog, config_json, content_json
		FROM %s.bean_list_publications
		WHERE COALESCE(NULLIF(publication_purpose,''),'factory_supply')='customer_resale'
		  AND owner_type='customer'
		  AND owner_key=$1
		  AND status IN ('published','draft')
		ORDER BY CASE WHEN status='published' THEN 0 ELSE 1 END, published_at DESC, id DESC
		LIMIT $2
	`, r.schema), fmt.Sprintf("%d", customerID), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBeanListSummaries(rows)
}

func (r Repository) SaveCustomerResaleBeanListPublication(ctx context.Context, cmd customerportalapp.SaveCustomerResaleBeanListPublicationCommand) (customerportalapp.BeanListSummary, error) {
	status := strings.TrimSpace(cmd.Status)
	if status != "draft" {
		status = "published"
	}
	config, err := json.Marshal(cmd.Config)
	if err != nil {
		return customerportalapp.BeanListSummary{}, err
	}
	content, err := json.Marshal(cmd.Content)
	if err != nil {
		return customerportalapp.BeanListSummary{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return customerportalapp.BeanListSummary{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var row customerportalapp.BeanListSummary
	var configJSON []byte
	var contentJSON []byte
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.bean_list_publications(
			publication_purpose, list_type, version_no, status, owner_type, owner_key,
			price_source_publication_id, style_source_publication_id, source_version_no,
			config_json, content_json, changelog, actor
		)
		VALUES('customer_resale',$1,$2,$3,'customer',$4,NULLIF($5,0),NULLIF($6,0),$7,$8::jsonb,$9::jsonb,$10,$11)
		RETURNING id, list_type, version_no, status, to_char(published_at,'YYYY-MM-DD HH24:MI'), changelog, config_json, content_json
	`, r.schema), cmd.ListType, cmd.VersionNo, status, fmt.Sprintf("%d", cmd.CustomerID), cmd.PriceSourcePublicationID, cmd.StyleSourcePublicationID, cmd.SourceVersionNo, config, content, cmd.Changelog, strings.TrimSpace(cmd.Actor)).
		Scan(&row.ID, &row.ListType, &row.VersionNo, &row.Status, &row.PublishedAt, &row.Changelog, &configJSON, &contentJSON); err != nil {
		return customerportalapp.BeanListSummary{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, strings.TrimSpace(cmd.Actor), "bean_list_publication", &row.ID, status, postgresinfra.StrPtr("publication_purpose"), nil, postgresinfra.StrPtr("customer_resale"), postgresinfra.AuditMeta{
		"customer_id":                 cmd.CustomerID,
		"publication_purpose":         "customer_resale",
		"price_source_publication_id": cmd.PriceSourcePublicationID,
		"source_version_no":           cmd.SourceVersionNo,
		"version_no":                  cmd.VersionNo,
	}); err != nil {
		return customerportalapp.BeanListSummary{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return customerportalapp.BeanListSummary{}, err
	}
	if err := parseBeanListDisplaySummary(configJSON, contentJSON, &row); err != nil {
		return customerportalapp.BeanListSummary{}, err
	}
	return row, nil
}

func (r Repository) loadBeanListServiceData(ctx context.Context, customerID int64, limit int) ([]customerportalapp.BeanListSummary, []customerportalapp.BeanListVersionOption, bool, error) {
	customerRows, err := r.listCustomerBeanListVersions(ctx, customerID, limit)
	if err != nil {
		return nil, nil, false, err
	}
	if len(customerRows) > 0 {
		options := beanListVersionOptions(customerRows)
		fixedID, fixed := r.fixedBeanListPublicationID(ctx, customerID)
		if fixed && fixedID > 0 {
			for _, row := range customerRows {
				if row.ID == fixedID {
					row.RequiresAcknowledgement = !r.beanListAcknowledged(ctx, customerID, row.ID)
					return []customerportalapp.BeanListSummary{row}, options, true, nil
				}
			}
		}
		customerRows[0].RequiresAcknowledgement = !r.beanListAcknowledged(ctx, customerID, customerRows[0].ID)
		if len(customerRows) > 1 {
			customerRows[0].Diff = customerportalapp.BeanListDiffBetween(customerRows[1], customerRows[0])
		}
		return []customerportalapp.BeanListSummary{customerRows[0]}, options, true, nil
	}
	rows, err := r.listLatestOfficialBeanLists(ctx, limit)
	if err != nil {
		return nil, nil, false, err
	}
	return rows, nil, false, nil
}

func (r Repository) beanListAcknowledged(ctx context.Context, customerID, publicationID int64) bool {
	var ok bool
	_ = r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT EXISTS (
			SELECT 1 FROM %s.customer_bean_list_acknowledgements
			WHERE customer_id=$1 AND publication_id=$2
		)
	`, r.schema), customerID, publicationID).Scan(&ok)
	return ok
}

func (r Repository) listCustomerBeanListVersions(ctx context.Context, customerID int64, limit int) ([]customerportalapp.BeanListSummary, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, list_type, version_no, status, to_char(published_at,'YYYY-MM-DD HH24:MI'), changelog, config_json, content_json
		FROM %s.bean_list_publications
		WHERE COALESCE(NULLIF(publication_purpose,''),'factory_supply')='factory_supply'
		  AND owner_type='customer' AND owner_key=$1 AND status='published'
		ORDER BY published_at DESC, id DESC
		LIMIT $2
	`, r.schema), fmt.Sprintf("%d", customerID), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out, err := scanBeanListSummaries(rows)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r Repository) fixedBeanListPublicationID(ctx context.Context, customerID int64) (int64, bool) {
	var mode string
	var publicationID int64
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(bean_list_mode,'latest'), COALESCE(bean_list_publication_id,0)
		FROM %s.customer_portal_profiles
		WHERE customer_id=$1
	`, r.schema), customerID).Scan(&mode, &publicationID)
	if err != nil {
		return 0, false
	}
	return publicationID, strings.TrimSpace(mode) == "fixed"
}

func beanListVersionOptions(rows []customerportalapp.BeanListSummary) []customerportalapp.BeanListVersionOption {
	out := make([]customerportalapp.BeanListVersionOption, 0, len(rows))
	for _, row := range rows {
		out = append(out, customerportalapp.BeanListVersionOption{
			ID:          row.ID,
			ListType:    row.ListType,
			VersionNo:   row.VersionNo,
			Title:       row.Title,
			PublishedAt: row.PublishedAt,
			CacheKey:    row.CacheKey,
		})
	}
	return out
}

func (r Repository) listLatestOfficialBeanLists(ctx context.Context, limit int) ([]customerportalapp.BeanListSummary, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, list_type, version_no, status, to_char(published_at,'YYYY-MM-DD HH24:MI'), changelog, config_json, content_json
		FROM (
			SELECT DISTINCT ON (list_type) id, list_type, version_no, status, published_at, changelog, config_json, content_json
			FROM %s.bean_list_publications
			WHERE COALESCE(NULLIF(publication_purpose,''),'factory_supply')='factory_supply'
			  AND owner_type='official' AND status='published'
			ORDER BY list_type, published_at DESC, id DESC
		) latest
		ORDER BY published_at DESC, id DESC
		LIMIT $1
	`, r.schema), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBeanListSummaries(rows)
}

func (r Repository) loadFactorySupplyBeanListPublication(ctx context.Context, customerID, publicationID int64) (customerportalapp.BeanListSummary, error) {
	var row customerportalapp.BeanListSummary
	var configJSON []byte
	var contentJSON []byte
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, list_type, version_no, status, to_char(published_at,'YYYY-MM-DD HH24:MI'), changelog, config_json, content_json
		FROM %s.bean_list_publications
		WHERE id=$1
		  AND COALESCE(NULLIF(publication_purpose,''),'factory_supply')='factory_supply'
		  AND status='published'
		  AND ((owner_type='customer' AND owner_key=$2) OR owner_type='official')
	`, r.schema), publicationID, fmt.Sprintf("%d", customerID)).
		Scan(&row.ID, &row.ListType, &row.VersionNo, &row.Status, &row.PublishedAt, &row.Changelog, &configJSON, &contentJSON)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return customerportalapp.BeanListSummary{}, customerportalapp.ErrBeanListPublicationNotFound
		}
		return customerportalapp.BeanListSummary{}, err
	}
	if err := parseBeanListDisplaySummary(configJSON, contentJSON, &row); err != nil {
		return customerportalapp.BeanListSummary{}, err
	}
	return row, nil
}

func (r Repository) listFactorySupplyBeanLists(ctx context.Context, customerID int64, limit int) ([]customerportalapp.BeanListSummary, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, list_type, version_no, status, to_char(published_at,'YYYY-MM-DD HH24:MI'), changelog, config_json, content_json
		FROM %s.bean_list_publications
		WHERE COALESCE(NULLIF(publication_purpose,''),'factory_supply')='factory_supply'
		  AND status='published'
		  AND ((owner_type='customer' AND owner_key=$1) OR owner_type='official')
		ORDER BY CASE WHEN owner_type='customer' THEN 0 ELSE 1 END, published_at DESC, id DESC
		LIMIT $2
	`, r.schema), fmt.Sprintf("%d", customerID), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBeanListSummaries(rows)
}

func (r Repository) currentCustomerProductClassificationTemplate(ctx context.Context, customerID int64) (customerportalapp.CustomerProductClassificationTemplate, error) {
	var row customerportalapp.CustomerProductClassificationTemplate
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT t.id, COALESCE(t.customer_id,0), COALESCE(t.source_template_id,0), t.name, false
		FROM %s.customer_product_alias_classification_template_usages u
		JOIN %s.product_classification_templates t ON t.id=u.classification_template_id AND t.active=true
		WHERE u.customer_id=$1 AND u.active=true AND COALESCE(t.customer_id,0)=$1
		ORDER BY COALESCE(u.sort_order,100), t.id
		LIMIT 1
	`, r.schema, r.schema), customerID).Scan(&row.ID, &row.CustomerID, &row.DerivedFromTemplateID, &row.Name, &row.ReadOnly)
	if err == nil {
		return row, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return customerportalapp.CustomerProductClassificationTemplate{}, err
	}
	err = r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT t.id, COALESCE(t.customer_id,0), COALESCE(t.source_template_id,0), t.name, true
		FROM %s.product_classification_template_usages u
		JOIN %s.product_classification_templates t ON t.id=u.classification_template_id AND t.active=true
		WHERE u.active=true
		ORDER BY COALESCE(u.sort_order,100), t.id
		LIMIT 1
	`, r.schema, r.schema)).Scan(&row.ID, &row.CustomerID, &row.DerivedFromTemplateID, &row.Name, &row.ReadOnly)
	if err == nil {
		return row, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return customerportalapp.CustomerProductClassificationTemplate{}, nil
	}
	return customerportalapp.CustomerProductClassificationTemplate{}, err
}

func (r Repository) listCustomerProductCategories(ctx context.Context, templateID int64) ([]customerportalapp.CustomerProductCategory, error) {
	if templateID <= 0 {
		return []customerportalapp.CustomerProductCategory{}, nil
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, template_id, COALESCE(parent_id,0), name, COALESCE(level,1), COALESCE(sort_order,100)
		FROM %s.product_classification_template_categories
		WHERE template_id=$1 AND active=true
		ORDER BY COALESCE(parent_id,0), COALESCE(sort_order,100), id
	`, r.schema), templateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []customerportalapp.CustomerProductCategory{}
	for rows.Next() {
		var row customerportalapp.CustomerProductCategory
		if err := rows.Scan(&row.ID, &row.TemplateID, &row.ParentID, &row.Name, &row.Level, &row.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) listCustomerProductSummaries(ctx context.Context, customerID, templateID int64) ([]customerportalapp.CustomerProductSummary, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT a.id,
		       a.product_id,
		       COALESCE(NULLIF(a.customer_item_code,''), 'SKU-' || LPAD(a.product_id::text, 6, '0')),
		       COALESCE(NULLIF(a.display_name,''), p.name, ''),
		       COALESCE(NULLIF(p.product_kind,''),'roasted_bean'),
		       COALESCE(ass.category_id,0),
		       COALESCE(cat.name,''),
		       COALESCE(a.sort_order,0)
		FROM %s.customer_product_aliases a
		JOIN %s.products p ON p.id=a.product_id
		LEFT JOIN %s.customer_product_alias_classification_assignments ass
		  ON ass.alias_id=a.id AND ass.template_id=$2
		LEFT JOIN %s.product_classification_template_categories cat
		  ON cat.id=ass.category_id AND cat.template_id=$2 AND cat.active=true
		WHERE a.customer_id=$1
		  AND a.active=true
		  AND COALESCE(a.include_in_price_list,true)=true
		  AND COALESCE(p.active,true)=true
		ORDER BY COALESCE(cat.sort_order,9999), COALESCE(a.sort_order,0), a.id
	`, r.schema, r.schema, r.schema, r.schema), customerID, templateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []customerportalapp.CustomerProductSummary{}
	for rows.Next() {
		var row customerportalapp.CustomerProductSummary
		if err := rows.Scan(&row.ID, &row.ProductID, &row.Code, &row.Name, &row.ProductKind, &row.CategoryID, &row.CategoryName, &row.SortOrder); err != nil {
			return nil, err
		}
		row.ListType = customerProductListType(row.ProductKind)
		row.ListTypeLabel = beanListTypeLabel(row.ListType)
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) ensureCustomerProductClassificationTemplate(ctx context.Context, tx pgx.Tx, customerID int64, actor string) (customerportalapp.CustomerProductClassificationTemplate, map[int64]int64, error) {
	var row customerportalapp.CustomerProductClassificationTemplate
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT t.id, COALESCE(t.customer_id,0), COALESCE(t.source_template_id,0), t.name, false
		FROM %s.customer_product_alias_classification_template_usages u
		JOIN %s.product_classification_templates t ON t.id=u.classification_template_id AND t.active=true
		WHERE u.customer_id=$1 AND u.active=true AND COALESCE(t.customer_id,0)=$1
		ORDER BY COALESCE(u.sort_order,100), t.id
		LIMIT 1
	`, r.schema, r.schema), customerID).Scan(&row.ID, &row.CustomerID, &row.DerivedFromTemplateID, &row.Name, &row.ReadOnly)
	if err == nil {
		return row, map[int64]int64{}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return customerportalapp.CustomerProductClassificationTemplate{}, nil, err
	}

	source := customerportalapp.CustomerProductClassificationTemplate{}
	_ = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT t.id, COALESCE(t.customer_id,0), COALESCE(t.source_template_id,0), t.name, false
		FROM %s.product_classification_template_usages u
		JOIN %s.product_classification_templates t ON t.id=u.classification_template_id AND t.active=true
		WHERE u.active=true
		ORDER BY COALESCE(u.sort_order,100), t.id
		LIMIT 1
	`, r.schema, r.schema)).Scan(&source.ID, &source.CustomerID, &source.DerivedFromTemplateID, &source.Name, &source.ReadOnly)

	var customerName string
	_ = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(NULLIF(name,''),'客户') FROM %s.customers WHERE id=$1`, r.schema), customerID).Scan(&customerName)
	name := strings.TrimSpace(customerName)
	if name == "" {
		name = fmt.Sprintf("客户%d", customerID)
	}
	name = fmt.Sprintf("%s 商品分类", name)
	var templateID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_classification_templates(customer_id, source_template_id, template_state, name, remark, active, sort_order, created_by, updated_by)
		VALUES($1,$2,'customer_derived',$3,'miniapp derived customer product categories',true,100,$4,$4)
			ON CONFLICT (customer_id, (lower(name))) DO UPDATE SET updated_at=now(), updated_by=excluded.updated_by
		RETURNING id
	`, r.schema), customerID, source.ID, name, actor).Scan(&templateID); err != nil {
		return customerportalapp.CustomerProductClassificationTemplate{}, nil, err
	}
	idMap := map[int64]int64{}
	if source.ID > 0 {
		rows, err := tx.Query(ctx, fmt.Sprintf(`
			SELECT id, COALESCE(parent_id,0), name, COALESCE(level,1), COALESCE(sort_order,100), COALESCE(product_config_template_id,0), COALESCE(gradient_template_id,0), COALESCE(unit_template_id,0)
			FROM %s.product_classification_template_categories
			WHERE template_id=$1 AND active=true
			ORDER BY COALESCE(level,1), COALESCE(parent_id,0), COALESCE(sort_order,100), id
		`, r.schema), source.ID)
		if err != nil {
			return customerportalapp.CustomerProductClassificationTemplate{}, nil, err
		}
		type srcCategory struct {
			id, parentID, productConfigTemplateID, gradientTemplateID, unitTemplateID int64
			name                                                                      string
			level, sortOrder                                                          int
		}
		sourceRows := []srcCategory{}
		for rows.Next() {
			var c srcCategory
			if err := rows.Scan(&c.id, &c.parentID, &c.name, &c.level, &c.sortOrder, &c.productConfigTemplateID, &c.gradientTemplateID, &c.unitTemplateID); err != nil {
				rows.Close()
				return customerportalapp.CustomerProductClassificationTemplate{}, nil, err
			}
			sourceRows = append(sourceRows, c)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return customerportalapp.CustomerProductClassificationTemplate{}, nil, err
		}
		rows.Close()
		for _, c := range sourceRows {
			parentID := int64(0)
			if c.parentID > 0 {
				parentID = idMap[c.parentID]
			}
			var newID int64
			if err := tx.QueryRow(ctx, fmt.Sprintf(`
				INSERT INTO %s.product_classification_template_categories(template_id, parent_id, name, level, sort_order, product_config_template_id, gradient_template_id, unit_template_id, active)
				VALUES($1,$2,$3,$4,$5,$6,$7,$8,true)
					ON CONFLICT (template_id, parent_id, (lower(name))) DO UPDATE SET updated_at=now()
				RETURNING id
			`, r.schema), templateID, parentID, c.name, c.level, c.sortOrder, c.productConfigTemplateID, c.gradientTemplateID, c.unitTemplateID).Scan(&newID); err != nil {
				return customerportalapp.CustomerProductClassificationTemplate{}, nil, err
			}
			idMap[c.id] = newID
		}
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_product_alias_classification_template_usages(customer_id, classification_template_id, active, sort_order, created_by, updated_by)
		VALUES($1,$2,true,100,$3,$3)
		ON CONFLICT(customer_id, classification_template_id) DO UPDATE SET active=true, updated_at=now(), updated_by=excluded.updated_by
	`, r.schema), customerID, templateID, actor); err != nil {
		return customerportalapp.CustomerProductClassificationTemplate{}, nil, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "product_classification_template", &templateID, "mini_derive_customer_product_classification_template", postgresinfra.StrPtr("source_template_id"), postgresinfra.StrPtr(fmt.Sprintf("%d", source.ID)), postgresinfra.StrPtr(fmt.Sprintf("%d", templateID)), postgresinfra.AuditMeta{"customer_id": customerID, "source_template_id": source.ID, "template_id": templateID}); err != nil {
		return customerportalapp.CustomerProductClassificationTemplate{}, nil, err
	}
	row = customerportalapp.CustomerProductClassificationTemplate{ID: templateID, CustomerID: customerID, DerivedFromTemplateID: source.ID, Name: name}
	return row, idMap, nil
}

func (r Repository) ensureCustomerCategoryBelongsToTemplate(ctx context.Context, tx pgx.Tx, templateID, categoryID int64) error {
	var ok bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.product_classification_template_categories WHERE id=$1 AND template_id=$2 AND active=true)`, r.schema), categoryID, templateID).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("classification category not found")
	}
	return nil
}

func (r Repository) nextCustomerCategorySortOrder(ctx context.Context, tx pgx.Tx, templateID, parentID int64) int {
	var maxSort int
	_ = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(MAX(sort_order),0)
		FROM %s.product_classification_template_categories
		WHERE template_id=$1 AND parent_id=$2 AND active=true
	`, r.schema), templateID, parentID).Scan(&maxSort)
	return maxSort + 100
}

func customerCategoryIDAfterDerive(categoryID int64, idMap map[int64]int64) int64 {
	if mapped := idMap[categoryID]; mapped > 0 {
		return mapped
	}
	return categoryID
}

func attachCustomerProductCategoryCounts(categories []customerportalapp.CustomerProductCategory, products []customerportalapp.CustomerProductSummary) []customerportalapp.CustomerProductCategory {
	counts := map[int64]int{}
	for _, product := range products {
		if product.CategoryID > 0 {
			counts[product.CategoryID]++
		}
	}
	for i := range categories {
		categories[i].ProductCount = counts[categories[i].ID]
	}
	return categories
}

func customerProductCountsByListType(products []customerportalapp.CustomerProductSummary) map[string]int {
	out := map[string]int{}
	for _, product := range products {
		listType := strings.TrimSpace(product.ListType)
		if listType == "" {
			listType = customerProductListType(product.ProductKind)
		}
		out[listType]++
	}
	return out
}

func customerPriceTableGroups(rows []customerportalapp.BeanListSummary, productCounts map[string]int, includeVersions bool) []customerportalapp.CustomerPriceTableGroup {
	index := map[string]int{}
	out := []customerportalapp.CustomerPriceTableGroup{}
	for _, row := range rows {
		listType := strings.TrimSpace(row.ListType)
		if listType == "" {
			listType = "commercial"
		}
		idx, ok := index[listType]
		if !ok {
			group := customerportalapp.CustomerPriceTableGroup{
				ListType:        listType,
				ListTypeLabel:   firstNonEmpty(row.ListTypeLabel, beanListTypeLabel(listType)),
				ProductCount:    productCounts[listType],
				PriceTableCount: 0,
				LatestVersion:   row,
			}
			index[listType] = len(out)
			out = append(out, group)
			idx = len(out) - 1
		}
		out[idx].PriceTableCount++
		if includeVersions {
			out[idx].Versions = append(out[idx].Versions, row)
		}
	}
	for listType, count := range productCounts {
		if _, ok := index[listType]; ok {
			continue
		}
		out = append(out, customerportalapp.CustomerPriceTableGroup{
			ListType:        listType,
			ListTypeLabel:   beanListTypeLabel(listType),
			ProductCount:    count,
			PriceTableCount: 0,
		})
	}
	return out
}

func customerProductListType(productKind string) string {
	switch strings.TrimSpace(productKind) {
	case "green_bean", "green":
		return "green"
	case "retail":
		return "retail"
	default:
		return "commercial"
	}
}

func (r Repository) listAuthorizedResaleGradientTemplates(ctx context.Context, customerID int64) ([]customerportalapp.ResaleGradientTemplate, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, name, display_unit
		FROM %s.pricing_gradient_templates t
		LEFT JOIN %s.customer_sku_public_usage u ON u.customer_id=$1
		WHERE t.active=true
		  AND COALESCE(t.allow_customer_resale,false)=true
		  AND (
		    COALESCE(t.customer_id,0)=$1
		    OR (COALESCE(t.customer_id,0)=0 AND COALESCE(u.use_public_gradient_templates,true)=true)
		  )
		ORDER BY COALESCE(t.customer_id,0) DESC, t.name, t.id
	`, r.schema, r.schema), customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]customerportalapp.ResaleGradientTemplate, 0)
	index := map[int64]int{}
	for rows.Next() {
		var row customerportalapp.ResaleGradientTemplate
		if err := rows.Scan(&row.ID, &row.Name, &row.DisplayUnit); err != nil {
			return nil, err
		}
		row.Tiers = []customerportalapp.ResaleGradientTemplateTier{}
		index[row.ID] = len(out)
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}
	ids := make([]int64, 0, len(out))
	for _, row := range out {
		ids = append(ids, row.ID)
	}
	tierRows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, template_id, label, min_weight_g::float8, max_weight_g::float8, position
		FROM %s.pricing_gradient_template_tiers
		WHERE active=true AND template_id = ANY($1)
		ORDER BY template_id, position, min_weight_g, id
	`, r.schema), ids)
	if err != nil {
		return nil, err
	}
	defer tierRows.Close()
	for tierRows.Next() {
		var templateID int64
		var tier customerportalapp.ResaleGradientTemplateTier
		if err := tierRows.Scan(&tier.ID, &templateID, &tier.Label, &tier.MinWeightG, &tier.MaxWeightG, &tier.Position); err != nil {
			return nil, err
		}
		if idx, ok := index[templateID]; ok {
			out[idx].Tiers = append(out[idx].Tiers, tier)
		}
	}
	return out, tierRows.Err()
}

func nextResaleBeanListVersionForRepository(rows []customerportalapp.BeanListSummary) string {
	maxVersion := 0
	for _, row := range rows {
		text := strings.TrimSpace(row.VersionNo)
		if len(text) < 2 || (text[0] != 'V' && text[0] != 'v') {
			continue
		}
		if n, err := strconv.Atoi(strings.TrimSpace(text[1:])); err == nil && n > maxVersion {
			maxVersion = n
		}
	}
	return fmt.Sprintf("V%d", maxVersion+1)
}

type beanListRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func scanBeanListSummaries(rows beanListRows) ([]customerportalapp.BeanListSummary, error) {
	out := make([]customerportalapp.BeanListSummary, 0)
	for rows.Next() {
		var row customerportalapp.BeanListSummary
		var configJSON []byte
		var contentJSON []byte
		if err := rows.Scan(&row.ID, &row.ListType, &row.VersionNo, &row.Status, &row.PublishedAt, &row.Changelog, &configJSON, &contentJSON); err != nil {
			return nil, err
		}
		if err := parseBeanListDisplaySummary(configJSON, contentJSON, &row); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func populateBeanListMetadata(row *customerportalapp.BeanListSummary) {
	if row == nil || row.ID <= 0 {
		return
	}
	row.CacheKey = beanListCacheKey(*row)
}

func beanListCacheKey(row customerportalapp.BeanListSummary) string {
	version := strings.TrimSpace(row.VersionNo)
	if version == "" {
		version = "published"
	}
	return fmt.Sprintf("bean-list:%d:%s", row.ID, version)
}

func parseBeanListDisplaySummary(configJSON, contentJSON []byte, row *customerportalapp.BeanListSummary) error {
	if row == nil {
		return nil
	}
	cfg := map[string]any{}
	if len(configJSON) > 0 {
		if err := json.Unmarshal(configJSON, &cfg); err != nil {
			return err
		}
	}
	var content map[string]any
	if len(contentJSON) > 0 {
		if err := json.Unmarshal(contentJSON, &content); err != nil {
			return err
		}
	}
	if content == nil {
		content = map[string]any{}
	}

	row.BrandName = beanListMapString(cfg, "brandName", "棵凡咖啡")
	row.BrandIntro = beanListMapString(cfg, "brandIntro", "")
	row.Title = beanListMapString(content, "title", buildBeanListDisplayTitle(row.ListType, row.BrandName))
	row.Subtitle = beanListMapString(content, "subtitle", buildBeanListDisplaySubtitle(row.ListType))
	row.ListTypeLabel = beanListTypeLabel(row.ListType)
	if changelog := beanListMapString(cfg, "changelog", ""); changelog != "" {
		row.Changelog = changelog
	}
	row.LayoutStyle = beanListLayoutStyle(beanListMapString(cfg, "layoutStyle", "card"))
	row.CardsPerRow = clampBeanListInt(beanListMapNumber(cfg, "cardsPerRow", 2), 2, 1, 4)
	row.ShowVersion = beanListMapBool(cfg, "showVersion", true)
	row.ShowChangelog = beanListMapBool(cfg, "showChangelog", true)
	row.ShowCategoryNumbers = beanListMapBool(cfg, "showCategoryNumbers", true)
	row.BackgroundColor = beanListHexColor(beanListMapString(cfg, "backgroundColor", "#f8f1e5"), "#f8f1e5")
	row.FontColor = beanListHexColor(beanListMapString(cfg, "fontColor", "#171717"), "#171717")
	row.BackgroundImage = safeBeanListImageURL(beanListMapString(cfg, "backgroundImage", ""))
	row.LogoImage = safeBeanListImageURL(beanListMapString(cfg, "logoImage", ""))

	groups := make([]customerportalapp.BeanListGroupSummary, 0)
	for _, groupMap := range beanListMapsFromAny(content["groups"]) {
		group := customerportalapp.BeanListGroupSummary{
			Category:     beanListMapString(groupMap, "category", ""),
			ShowCategory: beanListMapBool(groupMap, "showCategory", true),
			Items:        make([]customerportalapp.BeanListProductSummary, 0),
		}
		for _, itemMap := range beanListMapsFromAny(groupMap["items"]) {
			highlightTerms := beanListStringList(itemMap["highlightTerms"])
			item := customerportalapp.BeanListProductSummary{
				Code:           beanListMapString(itemMap, "code", ""),
				Name:           beanListMapString(itemMap, "name", ""),
				Badge:          beanListMapString(itemMap, "badge", ""),
				BadgeLabel:     beanListMapString(itemMap, "badgeLabel", ""),
				RecommendedUse: beanListMapString(itemMap, "recommendedUse", ""),
				Flavor:         beanListMapString(itemMap, "flavor", ""),
				Description:    beanListMapString(itemMap, "description", ""),
				BeanListQuality: beanListQualitySummaryFromMap(
					beanListFirstMap(itemMap, "beanListQuality", "bean_list_quality"),
				),
				HighlightTerms: highlightTerms,
				Prices:         make([]customerportalapp.BeanListPriceSummary, 0),
			}
			if strings.TrimSpace(item.Name) == "" {
				continue
			}
			for _, priceMap := range beanListMapsFromAny(itemMap["prices"]) {
				price := customerportalapp.BeanListPriceSummary{
					Label: beanListMapString(priceMap, "label", ""),
					Value: beanListMapString(priceMap, "value", ""),
					Red:   beanListMapBool(priceMap, "red", false),
				}
				if price.Value == "" {
					price.Value = formatBeanListPrice(beanListMapNumber(priceMap, "price", 0), beanListMapString(priceMap, "unit", ""))
				}
				if !price.Red {
					price.Red = beanListContainsHighlight(price.Label, highlightTerms) || beanListContainsHighlight(price.Value, highlightTerms)
				}
				if strings.TrimSpace(price.Label) == "" && strings.TrimSpace(price.Value) == "" {
					continue
				}
				item.Prices = append(item.Prices, price)
			}
			group.Items = append(group.Items, item)
		}
		if len(group.Items) > 0 {
			groups = append(groups, group)
		}
	}
	row.Groups = groups
	populateBeanListMetadata(row)
	return nil
}

func beanListQualitySummaryFromMap(value map[string]any) customerportalapp.BeanListQualitySummary {
	return customerportalapp.BeanListQualitySummary{
		FactoryFlavorDescription: beanListFirstString(value, "factoryFlavorDescription", "factory_flavor_description"),
		Moisture:                 beanListFirstString(value, "moisture"),
		Density:                  beanListFirstString(value, "density"),
		InspectionCreatedAt:      beanListFirstString(value, "inspectionCreatedAt", "inspection_created_at"),
		InspectionReferenceNo:    beanListFirstString(value, "inspectionReferenceNo", "inspection_reference_no"),
	}
}

func beanListFirstMap(m map[string]any, keys ...string) map[string]any {
	for _, key := range keys {
		if row, ok := m[key].(map[string]any); ok {
			return row
		}
	}
	return nil
}

func beanListFirstString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := beanListMapString(m, key, ""); value != "" {
			return value
		}
	}
	return ""
}

func beanListMapsFromAny(value any) []map[string]any {
	switch items := value.(type) {
	case []any:
		out := make([]map[string]any, 0, len(items))
		for _, item := range items {
			if m, ok := item.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	case []map[string]any:
		return items
	default:
		return nil
	}
}

func beanListMapString(m map[string]any, key, fallback string) string {
	if m == nil {
		return fallback
	}
	if v, ok := m[key]; ok {
		switch value := v.(type) {
		case string:
			if strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		case fmt.Stringer:
			if strings.TrimSpace(value.String()) != "" {
				return strings.TrimSpace(value.String())
			}
		default:
			if s := strings.TrimSpace(fmt.Sprint(value)); s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return fallback
}

func beanListMapBool(m map[string]any, key string, fallback bool) bool {
	if m == nil {
		return fallback
	}
	if v, ok := m[key]; ok {
		switch value := v.(type) {
		case bool:
			return value
		case string:
			switch strings.ToLower(strings.TrimSpace(value)) {
			case "true", "1", "yes":
				return true
			case "false", "0", "no":
				return false
			}
		}
	}
	return fallback
}

func beanListMapNumber(m map[string]any, key string, fallback float64) float64 {
	if m == nil {
		return fallback
	}
	if v, ok := m[key]; ok {
		switch value := v.(type) {
		case float64:
			return value
		case int:
			return float64(value)
		case int64:
			return float64(value)
		case json.Number:
			if n, err := value.Float64(); err == nil {
				return n
			}
		case string:
			if n, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
				return n
			}
		}
	}
	return fallback
}

func beanListStringList(value any) []string {
	switch rows := value.(type) {
	case []string:
		out := make([]string, 0, len(rows))
		for _, row := range rows {
			if s := strings.TrimSpace(row); s != "" {
				out = append(out, s)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(rows))
		for _, row := range rows {
			if s := strings.TrimSpace(fmt.Sprint(row)); s != "" && s != "<nil>" {
				out = append(out, s)
			}
		}
		return out
	case string:
		parts := strings.FieldsFunc(rows, func(r rune) bool { return r == ',' || r == '，' || r == '\n' })
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			if s := strings.TrimSpace(part); s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func buildBeanListDisplayTitle(listType, brandName string) string {
	brand := strings.TrimSpace(brandName)
	if brand == "" {
		brand = "棵凡咖啡"
	}
	switch strings.TrimSpace(listType) {
	case "retail":
		return brand + "零售豆单"
	case "green", "green_bean":
		return brand + "生豆豆单"
	}
	return brand + "批发豆单"
}

func buildBeanListDisplaySubtitle(listType string) string {
	switch strings.TrimSpace(listType) {
	case "retail":
		return "报价含税运"
	case "green", "green_bean":
		return "生豆销售报价"
	}
	return "报价不含税、不含运"
}

func beanListTypeLabel(listType string) string {
	switch strings.TrimSpace(listType) {
	case "retail":
		return "零售"
	case "green", "green_bean":
		return "生豆"
	}
	return "商用"
}

func beanListLayoutStyle(value string) string {
	if strings.TrimSpace(value) == "table" {
		return "table"
	}
	return "card"
}

func clampBeanListInt(value float64, fallback, min, max int) int {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return fallback
	}
	n := int(value)
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}

func beanListHexColor(value, fallback string) string {
	value = strings.TrimSpace(value)
	if len(value) != 7 || value[0] != '#' {
		return fallback
	}
	for _, r := range value[1:] {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return fallback
		}
	}
	return value
}

func safeBeanListImageURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "data:image/") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") || strings.HasPrefix(value, "/") {
		return value
	}
	return ""
}

func beanListContainsHighlight(text string, terms []string) bool {
	if text == "" || len(terms) == 0 {
		return false
	}
	for _, term := range terms {
		term = strings.TrimSpace(term)
		if term != "" && strings.Contains(text, term) {
			return true
		}
	}
	return false
}

func formatBeanListPrice(price float64, unit string) string {
	if price <= 0 {
		return ""
	}
	value := strconv.FormatFloat(math.Round(price), 'f', 0, 64)
	out := value
	if unit = strings.TrimSpace(unit); unit != "" {
		out += "/" + unit
	}
	return out
}

func (r Repository) listProducts(ctx context.Context, customerID int64, limit int) ([]customerportalapp.ProductSummary, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, name, roast_level,
		       COALESCE(NULLIF(product_kind,''), 'roasted_bean'),
		       COALESCE(drip_bag_grams,10)::float8,
		       COALESCE(drip_box_bag_count,10),
		       to_char(COALESCE(default_price,0), 'FM999999990.00'),
		       to_char(COALESCE(retail_price_100g,0), 'FM999999990.00'),
		       to_char(COALESCE(retail_price_200g,0), 'FM999999990.00'),
		       to_char(COALESCE(retail_price_227g,0), 'FM999999990.00'),
		       to_char(COALESCE(retail_price_250g,0), 'FM999999990.00')
		FROM %s.products
		WHERE active=true
		  AND %s
		ORDER BY name, id
		LIMIT $1
	`, r.schema, portalProductVisibleToCustomerSQL(r.schema+".products", "$2")), limit, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]customerportalapp.ProductSummary, 0)
	for rows.Next() {
		var row customerportalapp.ProductSummary
		if err := rows.Scan(&row.ID, &row.Name, &row.RoastLevel, &row.ProductKind, &row.DripBagGrams, &row.DripBoxBagCount, &row.DefaultPrice, &row.RetailPrice100, &row.RetailPrice200, &row.RetailPrice227, &row.RetailPrice250); err != nil {
			return nil, err
		}
		row.ProductKind = catalogdomain.NormalizeProductKind(row.ProductKind)
		if row.ProductKind == catalogdomain.ProductKindDripBag {
			row.SalesUnits = []string{"bag", "box"}
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	gradients, err := r.portalUnitPriceGradients(ctx, productSummaryIDs(out))
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].DripPriceGradients = gradients[out[i].ID]
	}
	return out, nil
}

func productSummaryIDs(rows []customerportalapp.ProductSummary) []int64 {
	out := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row.ID > 0 {
			out = append(out, row.ID)
		}
	}
	return out
}

func (r Repository) portalUnitPriceGradients(ctx context.Context, productIDs []int64) (map[int64][]customerportalapp.UnitPriceGradient, error) {
	if len(productIDs) == 0 {
		return map[int64][]customerportalapp.UnitPriceGradient{}, nil
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT product_id, id,
		       COALESCE(NULLIF(product_kind,''), 'roasted_bean'),
		       COALESCE(NULLIF(sales_unit,''), ''),
		       COALESCE(NULLIF(min_qty_units,0),0)::float8,
		       COALESCE(NULLIF(max_qty_units,0), max_qty_lb),
		       COALESCE(price_per_unit,0)::float8,
		       COALESCE(NULLIF(unit_bag_count,0),0)::float8,
		       CASE WHEN COALESCE(price_source_json,'{}'::jsonb) <> '{}'::jsonb THEN 'published_unit_price' ELSE '' END
		FROM %s.product_price_tiers
		WHERE active=true
		  AND product_id=ANY($1)
		  AND COALESCE(NULLIF(product_kind,''), 'roasted_bean')='drip_bag'
		  AND COALESCE(NULLIF(sales_unit,''), '') IN ('bag','box')
		ORDER BY product_id, COALESCE(NULLIF(sales_unit,''), ''), COALESCE(NULLIF(min_qty_units,0),0)
	`, r.schema), productIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64][]customerportalapp.UnitPriceGradient{}
	for rows.Next() {
		var productID int64
		var row customerportalapp.UnitPriceGradient
		if err := rows.Scan(&productID, &row.ID, &row.ProductKind, &row.SalesUnit, &row.MinQty, &row.MaxQty, &row.UnitPrice, &row.UnitBagCount, &row.PriceSource); err != nil {
			return nil, err
		}
		row.ProductKind = catalogdomain.NormalizeProductKind(row.ProductKind)
		out[productID] = append(out[productID], row)
	}
	return out, rows.Err()
}

func portalProductVisibleToCustomerSQL(productTable, customerPlaceholder string) string {
	return portalProductVisibleToCustomerAliasSQL(productTable, "", customerPlaceholder)
}

func portalProductVisibleToCustomerAliasSQL(productTable, productAlias, customerPlaceholder string) string {
	productTable = strings.TrimSpace(productTable)
	if productTable == "" {
		productTable = "products"
	}
	productAlias = strings.TrimSpace(productAlias)
	if productAlias != "" {
		productAlias += "."
	}
	return fmt.Sprintf(`(
		(
			CASE
				WHEN COALESCE(%[1]scustomer_id,0)>0 THEN COALESCE(NULLIF(%[1]svisibility,''),'customer_only')
				ELSE COALESCE(NULLIF(%[1]svisibility,''),'public')
			END <> 'customer_only'
			OR COALESCE(%[1]scustomer_id,0)=%[2]s
		)
		AND NOT (
			COALESCE(%[1]scustomer_id,0)=0
			AND EXISTS (
				SELECT 1 FROM %[3]s alias_products
				WHERE alias_products.active=true
				  AND COALESCE(alias_products.customer_id,0)=%[2]s
				  AND COALESCE(alias_products.base_product_id,0)=%[1]sid
				  AND COALESCE(NULLIF(alias_products.visibility,''),'customer_only')='customer_only'
			)
		)
	)`, productAlias, customerPlaceholder, productTable)
}

func mallProductPublicCatalogSQL(productAlias string) string {
	productAlias = strings.TrimSpace(productAlias)
	if productAlias != "" {
		productAlias += "."
	}
	return fmt.Sprintf(`(COALESCE(%scustomer_id,0)=0 AND COALESCE(NULLIF(%svisibility,''),'public')='public')`, productAlias, productAlias)
}

func (r Repository) listCustomerOrders(ctx context.Context, query customerportalapp.ServicePageQuery, limit int, includeItems bool) ([]customerportalapp.CustomerOrderSummary, error) {
	where := []string{"o.customer_id=$1", "o.is_void=false"}
	args := []any{query.CustomerID}
	if keyword := strings.TrimSpace(query.Query); keyword != "" {
		args = append(args, "%"+strings.ToLower(keyword)+"%")
		placeholder := fmt.Sprintf("$%d", len(args))
		where = append(where, fmt.Sprintf(`(
				LOWER(COALESCE(o.order_no,'')) LIKE %[1]s
				OR LOWER(COALESCE(o.receiver_name,'')) LIKE %[1]s
				OR LOWER(COALESCE(o.receiver_phone,'')) LIKE %[1]s
				OR LOWER(COALESCE(o.receiver_address,'')) LIKE %[1]s
				OR LOWER(COALESCE(c.contact,'')) LIKE %[1]s
				OR LOWER(COALESCE(c.name,'')) LIKE %[1]s
				OR LOWER(COALESCE(c.phone,'')) LIKE %[1]s
				OR LOWER(COALESCE(c.address,'')) LIKE %[1]s
			OR LOWER(COALESCE(c.company_address,'')) LIKE %[1]s
			OR EXISTS (SELECT 1 FROM %s.order_items oi2
				WHERE oi2.order_id=o.id
				  AND (LOWER(COALESCE(oi2.item_name,'')) LIKE %[1]s OR LOWER(COALESCE(oi2.spec,'')) LIKE %[1]s))
		)`, placeholder, r.schema))
	}
	if query.DateFrom != "" {
		args = append(args, query.DateFrom)
		where = append(where, fmt.Sprintf("o.order_date >= $%d::date", len(args)))
	}
	if query.DateTo != "" {
		args = append(args, query.DateTo)
		where = append(where, fmt.Sprintf("o.order_date <= $%d::date", len(args)))
	}
	if status := strings.TrimSpace(query.ProcessStatus); status != "" {
		args = append(args, strings.ToLower(status))
		where = append(where, fmt.Sprintf("LOWER(COALESCE(ops.name,'')) = $%d", len(args)))
	}
	if status := strings.TrimSpace(query.PayStatus); status != "" {
		args = append(args, strings.ToLower(status))
		where = append(where, fmt.Sprintf("LOWER(COALESCE(ps.name,'')) = $%d", len(args)))
	}
	if status := strings.TrimSpace(query.ShipStatus); status != "" {
		args = append(args, strings.ToLower(status))
		where = append(where, fmt.Sprintf("LOWER(COALESCE(ss.name,'')) = $%d", len(args)))
	}
	args = append(args, limit)
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT o.id,
			       COALESCE(o.order_no,''),
			       COALESCE(to_char(o.order_date,'YYYY-MM-DD'),''),
			       COALESCE(NULLIF(o.receiver_name,''), NULLIF(c.contact,''), c.name, ''),
			       COALESCE(NULLIF(o.receiver_phone,''), c.phone, ''),
			       COALESCE(NULLIF(o.receiver_address,''), NULLIF(c.address,''), c.company_address, ''),
			       COALESCE(ops.name,''),
		       COALESCE(ps.name,''),
		       COALESCE(o.payment_method,''),
		       COALESCE(ss.name,''),
		       COALESCE(o.ship_tracking_no,''),
		       to_char(COALESCE(o.grand_total,0), 'FM999999990.00'),
		       to_char(COALESCE(o.shipping_amount,0), 'FM999999990.00')
		FROM %s.orders o
		LEFT JOIN %s.customers c ON c.id=o.customer_id
		LEFT JOIN %s.order_process_statuses ops ON ops.id=o.process_status_id
		LEFT JOIN %s.pay_statuses ps ON ps.id=o.pay_status_id
		LEFT JOIN %s.ship_statuses ss ON ss.id=o.ship_status_id
		WHERE %s
		ORDER BY o.order_date DESC, o.id DESC
		LIMIT $%d
	`, r.schema, r.schema, r.schema, r.schema, r.schema, strings.Join(where, " AND "), len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]customerportalapp.CustomerOrderSummary, 0)
	orderIDs := make([]int64, 0)
	for rows.Next() {
		var row customerportalapp.CustomerOrderSummary
		if err := rows.Scan(&row.ID, &row.OrderNo, &row.OrderDate, &row.ReceiverName, &row.ReceiverPhone, &row.ReceiverAddress, &row.ProcessStatus, &row.PayStatus, &row.PaymentMethod, &row.ShipStatus, &row.ShipTrackingNo, &row.GrandTotal, &row.ShippingAmount); err != nil {
			return nil, err
		}
		orderIDs = append(orderIDs, row.ID)
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	items := map[int64][]customerportalapp.CustomerOrderItemSummary{}
	if includeItems {
		var err error
		items, err = r.listCustomerOrderItems(ctx, orderIDs)
		if err != nil {
			return nil, err
		}
	}
	for i := range out {
		if includeItems {
			out[i].Items = items[out[i].ID]
		}
		out[i].SalesOrderURL = fmt.Sprintf("/api/mini/orders/%d/sales-order-latest.pdf", out[i].ID)
		out[i].DeliveryNoteURL = fmt.Sprintf("/api/mini/orders/%d/delivery-note-latest.pdf", out[i].ID)
	}
	return out, nil
}

func (r Repository) CustomerOwnsOrder(ctx context.Context, customerID, orderID int64) (bool, error) {
	if customerID <= 0 || orderID <= 0 {
		return false, nil
	}
	var ok bool
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT EXISTS(
			SELECT 1
			FROM %s.orders
			WHERE id=$1 AND customer_id=$2 AND is_void=false
		)
	`, r.schema), orderID, customerID).Scan(&ok)
	return ok, err
}

func (r Repository) listCustomerOrderItems(ctx context.Context, orderIDs []int64) (map[int64][]customerportalapp.CustomerOrderItemSummary, error) {
	out := make(map[int64][]customerportalapp.CustomerOrderItemSummary, len(orderIDs))
	if len(orderIDs) == 0 {
		return out, nil
	}
	placeholders := make([]string, 0, len(orderIDs))
	args := make([]any, 0, len(orderIDs))
	for i, id := range orderIDs {
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
		args = append(args, id)
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT oi.order_id,
		       oi.id,
		       COALESCE(oi.item_name,''),
		       COALESCE(NULLIF(oi.product_kind,''), NULLIF(p.product_kind,''), 'roasted'),
		       COALESCE(oi.spec,''),
		       to_char(COALESCE(oi.qty,0), 'FM999999990.##'),
		       COALESCE(oi.unit,''),
		       to_char(COALESCE(oi.unit_price,0), 'FM999999990.00'),
		       to_char(COALESCE(oi.line_total,0), 'FM999999990.00'),
		       COALESCE(oi.bean_list_publication_id,0),
		       COALESCE(oi.bean_list_version_no,'')
		FROM %s.order_items oi
		LEFT JOIN %s.products p ON p.id=oi.product_id
		WHERE oi.order_id IN (`+strings.Join(placeholders, ",")+`)
		ORDER BY oi.order_id, oi.line_no, oi.id
	`, r.schema, r.schema), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var orderID int64
		var row customerportalapp.CustomerOrderItemSummary
		if err := rows.Scan(&orderID, &row.ID, &row.ItemName, &row.ProductKind, &row.Spec, &row.Qty, &row.Unit, &row.UnitPrice, &row.LineTotal, &row.BeanListPublicationID, &row.BeanListVersionNo); err != nil {
			return nil, err
		}
		out[orderID] = append(out[orderID], row)
	}
	return out, rows.Err()
}

func (r Repository) listDirectShipBatches(ctx context.Context, customerID int64, limit int) ([]customerportalapp.DirectShipBatch, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, batch_no, source_name, status, total_rows, valid_rows, invalid_rows, note, to_char(created_at,'YYYY-MM-DD HH24:MI')
		FROM %s.direct_ship_import_batches
		WHERE customer_id=$1
		ORDER BY created_at DESC, id DESC
		LIMIT $2
	`, r.schema), customerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]customerportalapp.DirectShipBatch, 0)
	for rows.Next() {
		var row customerportalapp.DirectShipBatch
		if err := rows.Scan(&row.ID, &row.BatchNo, &row.SourceName, &row.Status, &row.TotalRows, &row.ValidRows, &row.InvalidRows, &row.Note, &row.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) listInventory(ctx context.Context, customerID int64, limit int) ([]customerportalapp.InventoryItem, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, item_type, item_id, item_name, spec_g, warehouse, qty_g, qty_units, status, note, to_char(updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.customer_inventory_items
		WHERE customer_id=$1
		ORDER BY item_type, item_name, warehouse, id
		LIMIT $2
	`, r.schema), customerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]customerportalapp.InventoryItem, 0)
	for rows.Next() {
		var row customerportalapp.InventoryItem
		if err := rows.Scan(&row.ID, &row.ItemType, &row.ItemID, &row.ItemName, &row.SpecG, &row.Warehouse, &row.QtyG, &row.QtyUnits, &row.Status, &row.Note, &row.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) listProcessingRequests(ctx context.Context, customerID int64, limit int) ([]customerportalapp.ProcessingRequest, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT r.id, r.request_no, r.input_material_id, COALESCE(m.name,''), r.input_qty_g,
		       r.target_product_id, COALESCE(p.name,''), r.target_spec_g, r.target_qty,
		       r.status, r.note, to_char(r.created_at,'YYYY-MM-DD HH24:MI'),
		       COALESCE(to_char(r.accepted_at,'YYYY-MM-DD HH24:MI'), ''), r.linked_work_order_id
		FROM %s.processing_job_requests r
		LEFT JOIN %s.materials m ON m.id=r.input_material_id
		LEFT JOIN %s.products p ON p.id=r.target_product_id
		WHERE r.customer_id=$1
		ORDER BY r.created_at DESC, r.id DESC
		LIMIT $2
	`, r.schema, r.schema, r.schema), customerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]customerportalapp.ProcessingRequest, 0)
	for rows.Next() {
		var row customerportalapp.ProcessingRequest
		if err := rows.Scan(&row.ID, &row.RequestNo, &row.InputMaterialID, &row.InputMaterialName, &row.InputQtyG, &row.TargetProductID, &row.TargetProductName, &row.TargetSpecG, &row.TargetQty, &row.Status, &row.Note, &row.CreatedAt, &row.AcceptedAt, &row.LinkedWorkOrderID); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) listFeeItems(ctx context.Context, customerID int64, limit int) ([]customerportalapp.FeeItem, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, source_type, source_id, fee_type, to_char(amount, 'FM999999990.00'), currency,
		       to_char(occurred_at,'YYYY-MM-DD HH24:MI'), settlement_batch_id, status, note
		FROM %s.customer_fee_items
		WHERE customer_id=$1
		ORDER BY occurred_at DESC, id DESC
		LIMIT $2
	`, r.schema), customerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]customerportalapp.FeeItem, 0)
	for rows.Next() {
		var row customerportalapp.FeeItem
		if err := rows.Scan(&row.ID, &row.SourceType, &row.SourceID, &row.FeeType, &row.Amount, &row.Currency, &row.OccurredAt, &row.SettlementBatchID, &row.Status, &row.Note); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) listSettlementBatches(ctx context.Context, customerID int64, limit int) ([]customerportalapp.SettlementBatch, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, settlement_no, COALESCE(to_char(period_from,'YYYY-MM-DD'), ''), COALESCE(to_char(period_to,'YYYY-MM-DD'), ''),
		       status, to_char(total_amount, 'FM999999990.00'), COALESCE(to_char(confirmed_at,'YYYY-MM-DD HH24:MI'), ''),
		       COALESCE(to_char(paid_at,'YYYY-MM-DD HH24:MI'), ''), to_char(created_at,'YYYY-MM-DD HH24:MI')
		FROM %s.customer_settlement_batches
		WHERE customer_id=$1
		ORDER BY created_at DESC, id DESC
		LIMIT $2
	`, r.schema), customerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]customerportalapp.SettlementBatch, 0)
	for rows.Next() {
		var row customerportalapp.SettlementBatch
		if err := rows.Scan(&row.ID, &row.SettlementNo, &row.PeriodFrom, &row.PeriodTo, &row.Status, &row.TotalAmount, &row.ConfirmedAt, &row.PaidAt, &row.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) CreateDirectShipBatch(ctx context.Context, cmd customerportalapp.CreateDirectShipBatchCommand) (customerportalapp.DirectShipBatch, error) {
	sourceName := strings.TrimSpace(cmd.SourceName)
	if sourceName == "" {
		return customerportalapp.DirectShipBatch{}, fmt.Errorf("source_name required")
	}
	if cmd.TotalRows <= 0 {
		return customerportalapp.DirectShipBatch{}, fmt.Errorf("total_rows invalid")
	}
	note := strings.TrimSpace(cmd.Note)
	var id int64
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.direct_ship_import_batches(customer_id, source_name, status, total_rows, valid_rows, invalid_rows, note, created_by_mini_user_id)
		VALUES($1,$2,'submitted',$3,$3,0,$4,$5)
		RETURNING id
	`, r.schema), cmd.CustomerID, sourceName, cmd.TotalRows, note, cmd.CreatedByMiniUserID).Scan(&id); err != nil {
		return customerportalapp.DirectShipBatch{}, err
	}
	var row customerportalapp.DirectShipBatch
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		UPDATE %s.direct_ship_import_batches
		SET batch_no='DS-' || to_char(created_at,'YYYYMMDD') || '-' || lpad(id::text,4,'0')
		WHERE id=$1
		RETURNING id, batch_no, source_name, status, total_rows, valid_rows, invalid_rows, note, to_char(created_at,'YYYY-MM-DD HH24:MI')
	`, r.schema), id).Scan(&row.ID, &row.BatchNo, &row.SourceName, &row.Status, &row.TotalRows, &row.ValidRows, &row.InvalidRows, &row.Note, &row.CreatedAt); err != nil {
		return customerportalapp.DirectShipBatch{}, err
	}
	return row, nil
}

func (r Repository) CreateProcessingRequest(ctx context.Context, cmd customerportalapp.CreateProcessingRequestCommand) (customerportalapp.ProcessingRequest, error) {
	note := strings.TrimSpace(cmd.Note)
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return customerportalapp.ProcessingRequest{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := r.ensureProcessingInputInventoryTx(ctx, tx, cmd.CustomerID, cmd.InputMaterialID, cmd.InputQtyG); err != nil {
		return customerportalapp.ProcessingRequest{}, err
	}
	if err := r.ensureProcessingTargetProductTx(ctx, tx, cmd.CustomerID, cmd.TargetProductID); err != nil {
		return customerportalapp.ProcessingRequest{}, err
	}

	var id int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.processing_job_requests(customer_id, input_material_id, input_qty_g, target_product_id, target_spec_g, target_qty, status, note, created_by_mini_user_id)
		VALUES($1,$2,$3,$4,$5,$6,'submitted',$7,$8)
		RETURNING id
	`, r.schema), cmd.CustomerID, cmd.InputMaterialID, cmd.InputQtyG, cmd.TargetProductID, cmd.TargetSpecG, cmd.TargetQty, note, cmd.CreatedByMiniUserID).Scan(&id); err != nil {
		return customerportalapp.ProcessingRequest{}, err
	}
	var row customerportalapp.ProcessingRequest
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		UPDATE %s.processing_job_requests
		SET request_no='PJ-' || to_char(created_at,'YYYYMMDD') || '-' || lpad(id::text,4,'0')
		WHERE id=$1
		RETURNING id, request_no, input_material_id, input_qty_g, target_product_id, target_spec_g, target_qty, status, note, to_char(created_at,'YYYY-MM-DD HH24:MI'), COALESCE(to_char(accepted_at,'YYYY-MM-DD HH24:MI'), ''), linked_work_order_id
	`, r.schema), id).Scan(&row.ID, &row.RequestNo, &row.InputMaterialID, &row.InputQtyG, &row.TargetProductID, &row.TargetSpecG, &row.TargetQty, &row.Status, &row.Note, &row.CreatedAt, &row.AcceptedAt, &row.LinkedWorkOrderID); err != nil {
		return customerportalapp.ProcessingRequest{}, err
	}
	warehouseCode, err := r.processingWarehouseForCustomerTx(ctx, tx, cmd.CustomerID)
	if err != nil {
		return customerportalapp.ProcessingRequest{}, err
	}
	ct, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_processing_production_demands(
			request_id,request_no,customer_id,product_id,product_name,spec_g,target_qty,need_g,target_warehouse,status,created_at,updated_at
		)
		SELECT $1,$2,$3,$4,COALESCE(p.name,''),$5,$6,$7,$8,'planned',now(),now()
		FROM %s.products p
		WHERE p.id=$4
		  AND p.active=true
		  AND %s
		ON CONFLICT(request_id) DO UPDATE SET
			request_no=excluded.request_no,
			product_id=excluded.product_id,
			product_name=excluded.product_name,
			spec_g=excluded.spec_g,
			target_qty=excluded.target_qty,
			need_g=excluded.need_g,
			target_warehouse=excluded.target_warehouse,
			updated_at=now()
	`, r.schema, r.schema, portalProductVisibleToCustomerAliasSQL(r.schema+".products", "p", "$3")), row.ID, row.RequestNo, cmd.CustomerID, cmd.TargetProductID, cmd.TargetSpecG, int64(cmd.TargetQty), int64(cmd.TargetQty)*cmd.TargetSpecG, warehouseCode)
	if err != nil {
		return customerportalapp.ProcessingRequest{}, err
	}
	if ct.RowsAffected() == 0 {
		return customerportalapp.ProcessingRequest{}, fmt.Errorf("target product unavailable")
	}
	if err := tx.Commit(ctx); err != nil {
		return customerportalapp.ProcessingRequest{}, err
	}
	return row, nil
}

func (r Repository) ensureProcessingInputInventoryTx(ctx context.Context, tx pgx.Tx, customerID, inputMaterialID, inputQtyG int64) error {
	var availableG int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(qty_g),0)
		FROM %s.customer_inventory_items
		WHERE customer_id=$1
		  AND item_id=$2
		  AND item_type IN ('raw_bean','material','green_bean')
		  AND COALESCE(NULLIF(status,''),'available')='available'
	`, r.schema), customerID, inputMaterialID).Scan(&availableG); err != nil {
		return err
	}
	if availableG < inputQtyG {
		return fmt.Errorf("input material unavailable")
	}
	return nil
}

func (r Repository) ensureProcessingTargetProductTx(ctx context.Context, tx pgx.Tx, customerID, targetProductID int64) error {
	var exists bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT EXISTS(
			SELECT 1
			FROM %s.products
			WHERE id=$1 AND active=true
			  AND %s
		)
	`, r.schema, portalProductVisibleToCustomerSQL(r.schema+".products", "$2")), targetProductID, customerID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("target product unavailable")
	}
	return nil
}

type mallOrderLine struct {
	MallProductID   int64
	ProductID       int64
	Title           string
	SpecG           int64
	UnitPrice       float64
	ProductKind     string
	DripBagGrams    float64
	DripBoxBagCount int
}

type portalMallLinePricing struct {
	SalesUnit    string
	DisplayUnit  string
	SpecText     string
	UnitBagCount float64
	UnitBeanG    float64
	UnitPrice    float64
	LineTotal    float64
	PriceSource  string
}

func portalMallLinePricingFor(line mallOrderLine, item customerportalapp.MallOrderItemCommand) (portalMallLinePricing, error) {
	pricing := portalMallLinePricing{
		DisplayUnit: "件",
		SpecText:    fmt.Sprintf("%dg", line.SpecG),
		UnitPrice:   line.UnitPrice,
		LineTotal:   line.UnitPrice * float64(item.Qty),
		PriceSource: "{}",
	}
	if line.ProductKind != catalogdomain.ProductKindDripBag {
		return pricing, nil
	}
	if line.UnitPrice <= 0 {
		return portalMallLinePricing{}, fmt.Errorf("mall price unavailable")
	}
	salesUnit := portalNormalizeSalesUnit(item.SalesUnit)
	if salesUnit == "" {
		salesUnit = "bag"
	}
	if salesUnit != "bag" && salesUnit != "box" {
		return portalMallLinePricing{}, fmt.Errorf("sales_unit invalid")
	}
	unitBagCount := 1.0
	if salesUnit == "box" {
		unitBagCount = float64(line.DripBoxBagCount)
		if unitBagCount <= 0 {
			unitBagCount = 10
		}
	}
	unitBeanG := line.DripBagGrams
	if unitBeanG <= 0 {
		unitBeanG = 10
	}
	unitPrice := line.UnitPrice
	if salesUnit == "box" {
		unitPrice = line.UnitPrice * unitBagCount
	}
	lineTotal := unitPrice * float64(item.Qty)
	pricing.SalesUnit = salesUnit
	pricing.DisplayUnit = portalDisplayUnit(salesUnit)
	pricing.SpecText = portalDripSpecText(salesUnit, unitBeanG, int(unitBagCount))
	pricing.UnitBagCount = unitBagCount
	pricing.UnitBeanG = unitBeanG
	pricing.UnitPrice = unitPrice
	pricing.LineTotal = lineTotal
	pricing.PriceSource = portalPriceSourceSnapshot("mall_price", line.ProductKind, salesUnit, unitPrice, float64(item.Qty), line.UnitPrice, lineTotal, unitBagCount)
	return pricing, nil
}

func (r Repository) CreateMallOrder(ctx context.Context, cmd customerportalapp.CreateMallOrderCommand) (customerportalapp.FulfillmentOrder, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return customerportalapp.FulfillmentOrder{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, fmt.Sprintf("LOCK TABLE %s.orders IN SHARE ROW EXCLUSIVE MODE", r.schema)); err != nil {
		return customerportalapp.FulfillmentOrder{}, err
	}

	ids := make([]int64, 0, len(cmd.Items))
	for _, item := range cmd.Items {
		ids = append(ids, item.MallProductID)
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT m.id, m.product_id, COALESCE(NULLIF(m.title,''), p.name, ''), m.spec_g, m.unit_price,
		       COALESCE(NULLIF(p.product_kind,''), 'roasted_bean'),
		       COALESCE(p.drip_bag_grams,10)::float8,
		       COALESCE(p.drip_box_bag_count,10)
		FROM %s.mall_products m
		JOIN %s.products p ON p.id=m.product_id
		WHERE m.id = ANY($1) AND m.status='published' AND p.active=true
		  AND %s
	`, r.schema, r.schema, mallProductPublicCatalogSQL("p")), ids)
	if err != nil {
		return customerportalapp.FulfillmentOrder{}, err
	}
	linesByMallProduct := map[int64]mallOrderLine{}
	for rows.Next() {
		var line mallOrderLine
		if err := rows.Scan(&line.MallProductID, &line.ProductID, &line.Title, &line.SpecG, &line.UnitPrice, &line.ProductKind, &line.DripBagGrams, &line.DripBoxBagCount); err != nil {
			rows.Close()
			return customerportalapp.FulfillmentOrder{}, err
		}
		line.ProductKind = catalogdomain.NormalizeProductKind(line.ProductKind)
		linesByMallProduct[line.MallProductID] = line
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return customerportalapp.FulfillmentOrder{}, err
	}
	rows.Close()

	totalAmount := 0.0
	for _, item := range cmd.Items {
		line, ok := linesByMallProduct[item.MallProductID]
		if !ok {
			return customerportalapp.FulfillmentOrder{}, fmt.Errorf("mall product unavailable")
		}
		pricing, err := portalMallLinePricingFor(line, item)
		if err != nil {
			return customerportalapp.FulfillmentOrder{}, err
		}
		totalAmount += pricing.LineTotal
	}
	shippingAmount := cmd.ShippingAmount
	if shippingAmount < 0 {
		shippingAmount = 0
	}
	grandTotal := totalAmount + shippingAmount
	senderID, err := r.defaultSenderForCustomerTx(ctx, tx, cmd.CustomerID)
	if err != nil {
		return customerportalapp.FulfillmentOrder{}, err
	}
	orderDate := time.Now()
	orderNo, err := nextCustomerPortalOrderNo(ctx, tx, r.schema, orderDate)
	if err != nil {
		return customerportalapp.FulfillmentOrder{}, err
	}
	payStatusID := customerPortalStatusID(ctx, tx, r.schema, "pay_statuses", "未付款", "未收款")
	shipStatusID := customerPortalStatusID(ctx, tx, r.schema, "ship_statuses", "未发货")
	processStatusID := customerPortalStatusID(ctx, tx, r.schema, "order_process_statuses", "待处理", "待生产")

	var orderID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.orders(
			order_date,customer_id,pay_status_id,ship_status_id,process_status_id,
			receiver_name,receiver_phone,receiver_address,receiver_company,
			portal_service_code,source_warehouse,sender_id,notes,
			total_amount,shipping_amount,grand_total,order_no
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17
		)
		RETURNING id
	`, r.schema),
		orderDate,
		cmd.CustomerID,
		portalNullInt(payStatusID),
		portalNullInt(shipStatusID),
		portalNullInt(processStatusID),
		strings.TrimSpace(cmd.RecipientName),
		strings.TrimSpace(cmd.RecipientPhone),
		strings.TrimSpace(cmd.RecipientAddress),
		strings.TrimSpace(cmd.RecipientCompany),
		customerportalapp.PortalServiceMall,
		"finished_goods",
		senderID,
		strings.TrimSpace(cmd.Note),
		totalAmount,
		shippingAmount,
		grandTotal,
		orderNo,
	).Scan(&orderID); err != nil {
		return customerportalapp.FulfillmentOrder{}, err
	}

	for i, item := range cmd.Items {
		line := linesByMallProduct[item.MallProductID]
		pricing, err := portalMallLinePricingFor(line, item)
		if err != nil {
			return customerportalapp.FulfillmentOrder{}, err
		}
		usage, err := orderbeans.ResolveUsage(ctx, tx, r.schema, cmd.CustomerID, line.ProductID, orderbeans.ListTypeForProductKind(line.ProductKind, true))
		if err != nil {
			return customerportalapp.FulfillmentOrder{}, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.order_items(order_id,line_no,product_id,product_kind,bean_list_publication_id,bean_list_version_no,item_name,qty,unit,spec,unit_price,line_total,sales_unit,unit_bag_count,unit_bean_g,matched_price_qty,price_source_json)
			VALUES($1,$2,$3,$4,NULLIF($5,0),$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17::jsonb)
		`, r.schema), orderID, i+1, line.ProductID, line.ProductKind, usage.PublicationID, usage.VersionNo, line.Title, item.Qty, pricing.DisplayUnit, pricing.SpecText, pricing.UnitPrice, pricing.LineTotal, pricing.SalesUnit, pricing.UnitBagCount, pricing.UnitBeanG, item.Qty, pricing.PriceSource); err != nil {
			return customerportalapp.FulfillmentOrder{}, err
		}
		if line.ProductKind == catalogdomain.ProductKindDripBag {
			if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, portalMiniActor(cmd.CreatedByMiniUserID), "customer_portal_order", &orderID, "mall drip submit", nil, nil, nil, postgresinfra.AuditMeta{
				"product_id":     line.ProductID,
				"sales_unit":     pricing.SalesUnit,
				"qty":            item.Qty,
				"unit_bag_count": pricing.UnitBagCount,
				"unit_bean_g":    pricing.UnitBeanG,
				"price_source":   "mall_price",
				"total":          pricing.LineTotal,
			}); err != nil {
				return customerportalapp.FulfillmentOrder{}, err
			}
		}
	}
	_ = cmd.CreatedByMiniUserID
	if err := tx.Commit(ctx); err != nil {
		return customerportalapp.FulfillmentOrder{}, err
	}
	return customerportalapp.FulfillmentOrder{
		OrderID:           orderID,
		OrderNo:           orderNo,
		PortalServiceCode: customerportalapp.PortalServiceMall,
		SourceWarehouse:   "finished_goods",
	}, nil
}

func (r Repository) CreateFulfillmentOrder(ctx context.Context, cmd customerportalapp.CreateFulfillmentOrderCommand) (customerportalapp.FulfillmentOrder, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return customerportalapp.FulfillmentOrder{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, fmt.Sprintf("LOCK TABLE %s.orders IN SHARE ROW EXCLUSIVE MODE", r.schema)); err != nil {
		return customerportalapp.FulfillmentOrder{}, err
	}

	serviceCode := strings.TrimSpace(cmd.PortalServiceCode)
	sourceWarehouse := "finished_goods"
	if serviceCode == customerportalapp.PortalServiceProcessingShipment {
		sourceWarehouse, err = r.processingWarehouseForCustomerTx(ctx, tx, cmd.CustomerID)
		if err != nil {
			return customerportalapp.FulfillmentOrder{}, err
		}
	}
	senderID, err := r.defaultSenderForCustomerTx(ctx, tx, cmd.CustomerID)
	if err != nil {
		return customerportalapp.FulfillmentOrder{}, err
	}

	var productName, productKind string
	var defaultPrice float64
	var dripBagGrams float64
	var dripBoxBagCount int
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(name,''), COALESCE(default_price,0),
		       COALESCE(NULLIF(product_kind,''), 'roasted_bean'),
		       COALESCE(drip_bag_grams,10)::float8,
		       COALESCE(drip_box_bag_count,10)
		FROM %s.products
		WHERE id=$1 AND active=true
		  AND %s
	`, r.schema, portalProductVisibleToCustomerSQL(r.schema+".products", "$2")), cmd.ProductID, cmd.CustomerID).Scan(&productName, &defaultPrice, &productKind, &dripBagGrams, &dripBoxBagCount); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return customerportalapp.FulfillmentOrder{}, fmt.Errorf("product unavailable")
		}
		return customerportalapp.FulfillmentOrder{}, err
	}
	productKind = catalogdomain.NormalizeProductKind(productKind)
	productName = firstNonEmpty(strings.TrimSpace(cmd.ProductName), productName)
	salesUnit := strings.TrimSpace(cmd.SalesUnit)
	unitBagCount := 0.0
	unitBeanG := 0.0
	matchedPriceQty := 0.0
	priceSourceName := ""
	priceSourceSnapshot := "{}"
	specText := fmt.Sprintf("%dg", cmd.SpecG)
	var unitPrice, totalAmount float64
	if productKind == catalogdomain.ProductKindGreenBean {
		publishedPrice, err := orderbeans.ResolvePublishedUnitPrice(ctx, tx, r.schema, cmd.CustomerID, cmd.ProductID, orderbeans.ListTypeGreen, cmd.SpecG, cmd.Qty)
		if err != nil {
			return customerportalapp.FulfillmentOrder{}, err
		}
		unitPrice = publishedPrice
		if unitPrice <= 0 {
			unitPrice = r.portalFulfillmentUnitPriceTx(ctx, tx, cmd.CustomerID, cmd.ProductID, cmd.SpecG, cmd.Qty, defaultPrice)
		}
		totalAmount = portalLineTotalFromDisplayUnit(unitPrice, cmd.SpecG, cmd.Qty)
		priceSourceName = "green_bean_list"
	} else if productKind == catalogdomain.ProductKindDripBag {
		if err := r.ensurePortalDripProductHasActiveBOMTx(ctx, tx, cmd.ProductID); err != nil {
			return customerportalapp.FulfillmentOrder{}, err
		}
		if salesUnit == "" {
			salesUnit = "bag"
		}
		if salesUnit != "bag" && salesUnit != "box" {
			return customerportalapp.FulfillmentOrder{}, fmt.Errorf("sales_unit invalid")
		}
		unitBagCount = 1
		if salesUnit == "box" {
			unitBagCount = float64(dripBoxBagCount)
		}
		unitBeanG = dripBagGrams
		specText = portalDripSpecText(salesUnit, dripBagGrams, dripBoxBagCount)
		tiers, err := r.portalDripUnitPriceTiersTx(ctx, tx, cmd.ProductID)
		if err != nil {
			return customerportalapp.FulfillmentOrder{}, err
		}
		result, err := salesdomain.CalculateUnitLineTotal(salesdomain.UnitLineInput{
			ProductKind:  productKind,
			SalesUnit:    salesUnit,
			Quantity:     float64(cmd.Qty),
			UnitBagCount: unitBagCount,
			Tiers:        tiers,
		})
		if err != nil {
			return customerportalapp.FulfillmentOrder{}, fmt.Errorf("drip price unpublished")
		}
		unitPrice = result.UnitPrice
		totalAmount = result.LineTotal
		matchedPriceQty = result.MatchedQtyForTier
		priceSourceName = "published_unit_price"
		priceSourceSnapshot = portalUnitLinePriceSourceSnapshot(result)
	} else {
		unitPrice = r.portalFulfillmentUnitPriceTx(ctx, tx, cmd.CustomerID, cmd.ProductID, cmd.SpecG, cmd.Qty, defaultPrice)
		totalAmount = portalLineTotalFromDisplayUnit(unitPrice, cmd.SpecG, cmd.Qty)
	}
	shippingAmount := cmd.ShippingAmount
	if shippingAmount < 0 {
		shippingAmount = 0
	}
	grandTotal := totalAmount + shippingAmount

	orderDate := time.Now()
	orderNo, err := nextCustomerPortalOrderNo(ctx, tx, r.schema, orderDate)
	if err != nil {
		return customerportalapp.FulfillmentOrder{}, err
	}
	payStatusID := customerPortalStatusID(ctx, tx, r.schema, "pay_statuses", "未付款", "未收款")
	shipStatusID := customerPortalStatusID(ctx, tx, r.schema, "ship_statuses", "未发货")
	processStatusNames := []string{"待处理", "待生产"}
	if serviceCode == customerportalapp.PortalServiceProcessingShipment {
		processStatusNames = []string{"无需生产", "库存待发货", "生产完成"}
	}
	processStatusID := customerPortalStatusID(ctx, tx, r.schema, "order_process_statuses", processStatusNames...)

	var orderID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.orders(
			order_date,customer_id,pay_status_id,ship_status_id,process_status_id,
			receiver_name,receiver_phone,receiver_address,receiver_company,
			portal_service_code,source_warehouse,sender_id,notes,
			total_amount,shipping_amount,grand_total,order_no
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17
		)
		RETURNING id
	`, r.schema),
		orderDate,
		cmd.CustomerID,
		portalNullInt(payStatusID),
		portalNullInt(shipStatusID),
		portalNullInt(processStatusID),
		strings.TrimSpace(cmd.RecipientName),
		strings.TrimSpace(cmd.RecipientPhone),
		strings.TrimSpace(cmd.RecipientAddress),
		strings.TrimSpace(cmd.RecipientCompany),
		serviceCode,
		sourceWarehouse,
		senderID,
		strings.TrimSpace(cmd.Note),
		totalAmount,
		shippingAmount,
		grandTotal,
		orderNo,
	).Scan(&orderID); err != nil {
		return customerportalapp.FulfillmentOrder{}, err
	}
	usage, err := orderbeans.ResolveUsage(ctx, tx, r.schema, cmd.CustomerID, cmd.ProductID, orderbeans.ListTypeForProductKind(productKind, false))
	if err != nil {
		return customerportalapp.FulfillmentOrder{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.order_items(order_id,line_no,product_id,item_name,qty,unit,spec,unit_price,line_total,product_kind,sales_unit,unit_bag_count,unit_bean_g,matched_price_qty,price_source_json,bean_list_publication_id,bean_list_version_no)
		VALUES($1,1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14::jsonb,NULLIF($15,0),$16)
	`, r.schema), orderID, cmd.ProductID, productName, cmd.Qty, portalDisplayUnit(salesUnit), specText, unitPrice, totalAmount, productKind, salesUnit, unitBagCount, unitBeanG, matchedPriceQty, priceSourceSnapshot, usage.PublicationID, usage.VersionNo); err != nil {
		return customerportalapp.FulfillmentOrder{}, err
	}
	if productKind == catalogdomain.ProductKindDripBag {
		if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, portalMiniActor(cmd.CreatedByMiniUserID), "customer_portal_order", &orderID, "miniapp fulfillment drip submit", nil, nil, nil, postgresinfra.AuditMeta{
			"product_id":     cmd.ProductID,
			"sales_unit":     salesUnit,
			"qty":            cmd.Qty,
			"unit_bag_count": unitBagCount,
			"unit_bean_g":    unitBeanG,
			"price_source":   priceSourceName,
			"total":          totalAmount,
		}); err != nil {
			return customerportalapp.FulfillmentOrder{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return customerportalapp.FulfillmentOrder{}, err
	}
	return customerportalapp.FulfillmentOrder{
		OrderID:           orderID,
		OrderNo:           orderNo,
		PortalServiceCode: serviceCode,
		SourceWarehouse:   sourceWarehouse,
	}, nil
}

type portalDirectShipCapabilityConfig struct {
	SmallBatchPriceRule customerportalapp.SmallBatchPriceRule `json:"small_batch_price_rule"`
}

func (r Repository) ensurePortalDripProductHasActiveBOMTx(ctx context.Context, tx pgx.Tx, productID int64) error {
	var exists bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT EXISTS (
			SELECT 1
			FROM %s.product_bom b
			WHERE b.product_id=$1 AND b.status='active'
			  AND EXISTS (SELECT 1 FROM %s.product_bom_items bi WHERE bi.product_id=b.product_id)
		)
	`, r.schema, r.schema), productID).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	productionBomExists, err := r.portalProductionBOMConfiguredForProductTx(ctx, tx, productID)
	if err != nil {
		return err
	}
	if productionBomExists {
		return nil
	}
	return fmt.Errorf("product BOM not configured")
}

func (r Repository) portalProductionBOMConfiguredForProductTx(ctx context.Context, tx pgx.Tx, productID int64) (bool, error) {
	for _, relation := range []string{"production_boms", "production_bom_versions", "production_bom_version_items"} {
		if !portalRelationExists(ctx, tx, fmt.Sprintf("%s.%s", r.schema, relation)) {
			return false, nil
		}
	}
	var exists bool
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT EXISTS (
			SELECT 1
			FROM %[1]s.production_boms pb
			JOIN %[1]s.production_bom_versions v ON v.bom_id=pb.id AND v.status='published'
			JOIN %[1]s.production_bom_version_items i ON i.version_id=v.id
			WHERE pb.status='active'
			  AND (pb.output_product_id=$1 OR pb.legacy_product_id=$1)
		)
	`, r.schema), productID).Scan(&exists)
	return exists, err
}

func portalRelationExists(ctx context.Context, tx pgx.Tx, relation string) bool {
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, relation).Scan(&exists); err != nil {
		return false
	}
	return exists
}

func (r Repository) portalDripUnitPriceTiersTx(ctx context.Context, tx pgx.Tx, productID int64) ([]salesdomain.UnitPriceTier, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT COALESCE(NULLIF(product_kind,''), 'roasted_bean'),
		       COALESCE(NULLIF(sales_unit,''), ''),
		       COALESCE(NULLIF(min_qty_units,0),0)::float8,
		       COALESCE(price_per_unit,0)::float8,
		       COALESCE(NULLIF(unit_bag_count,0),0)::float8
		FROM %s.product_price_tiers
		WHERE product_id=$1
		  AND active=true
		  AND COALESCE(NULLIF(product_kind,''), 'roasted_bean')='drip_bag'
		  AND COALESCE(NULLIF(sales_unit,''), '') IN ('bag','box')
		  AND COALESCE(price_per_unit,0)>0
		ORDER BY COALESCE(NULLIF(sales_unit,''), ''), COALESCE(NULLIF(min_qty_units,0),0)
	`, r.schema), productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tiers := make([]salesdomain.UnitPriceTier, 0)
	for rows.Next() {
		var tier salesdomain.UnitPriceTier
		if err := rows.Scan(&tier.ProductKind, &tier.SalesUnit, &tier.MinQty, &tier.PricePerUnit, &tier.UnitBagCount); err != nil {
			return nil, err
		}
		tiers = append(tiers, tier)
	}
	return tiers, rows.Err()
}

func portalUnitLinePriceSourceSnapshot(result salesdomain.UnitLineResult) string {
	return portalPriceSourceSnapshot("published_unit_price", result.Tier.ProductKind, result.Tier.SalesUnit, result.UnitPrice, result.MatchedQtyForTier, result.Tier.PricePerUnit, result.LineTotal, result.Tier.UnitBagCount)
}

func portalPriceSourceSnapshot(source, productKind, salesUnit string, unitPrice, matchedQty, tierPrice, lineTotal, unitBagCount float64) string {
	b, _ := json.Marshal(map[string]any{
		"price_source":         source,
		"product_kind":         productKind,
		"sales_unit":           salesUnit,
		"unit_price":           unitPrice,
		"matched_qty_for_tier": matchedQty,
		"tier_price_per_unit":  tierPrice,
		"unit_bag_count":       unitBagCount,
		"line_total":           lineTotal,
	})
	return string(b)
}

func portalDripSpecText(salesUnit string, bagGrams float64, boxBagCount int) string {
	if bagGrams <= 0 {
		bagGrams = 10
	}
	if boxBagCount <= 0 {
		boxBagCount = 10
	}
	bagText := fmt.Sprintf("%.0fg", bagGrams)
	if math.Abs(bagGrams-math.Round(bagGrams)) > 0.001 {
		bagText = fmt.Sprintf("%.1fg", bagGrams)
	}
	if salesUnit == "box" {
		return fmt.Sprintf("%s*%d袋/盒", bagText, boxBagCount)
	}
	return bagText + "/袋"
}

func portalNormalizeSalesUnit(salesUnit string) string {
	switch strings.TrimSpace(strings.ToLower(salesUnit)) {
	case "bag", "袋":
		return "bag"
	case "box", "盒":
		return "box"
	default:
		return strings.TrimSpace(salesUnit)
	}
}

func portalDisplayUnit(salesUnit string) string {
	switch strings.TrimSpace(salesUnit) {
	case "bag":
		return "袋"
	case "box":
		return "盒"
	default:
		return "件"
	}
}

func portalMiniActor(miniUserID int64) string {
	if miniUserID > 0 {
		return fmt.Sprintf("mini_user:%d", miniUserID)
	}
	return "miniapp"
}

func (r Repository) portalFulfillmentUnitPriceTx(ctx context.Context, tx pgx.Tx, customerID, productID, specG int64, qty int64, defaultPrice float64) float64 {
	if productID <= 0 || specG <= 0 || qty <= 0 {
		return defaultPrice
	}
	rule := r.portalDirectShipSmallBatchPriceRuleTx(ctx, tx, customerID)
	tierQty := portalTierQuantityForSpec(specG, qty)
	qtyLb := float64(specG*qty) / 454.0
	tierQtyLb := qtyLb
	if adjustedQty, ok := portalSmallBatchTierQuantity(specG, qtyLb, rule); ok {
		tierQty = portalTierQuantityForSpec(specG, adjustedQty)
		tierQtyLb = float64(specG*adjustedQty) / 454.0
	}
	var packagePrice, pricePerLb float64
	q := fmt.Sprintf(`
		SELECT
			COALESCE(NULLIF(price_per_unit,0), NULLIF(price_per_lb,0) * COALESCE(NULLIF(spec_g,0),454) / 454.0, 0),
			COALESCE(NULLIF(price_per_lb,0), NULLIF(price_per_unit,0) * 454.0 / COALESCE(NULLIF(spec_g,0),454), 0)
		FROM %s.product_price_tiers
		WHERE product_id=$1 AND active=true
		  AND COALESCE(NULLIF(spec_g,0),454)=$2
		  AND COALESCE(NULLIF(min_qty_units,0), min_qty_lb, 0) <= $3
		  AND (COALESCE(NULLIF(max_qty_units,0), max_qty_lb) IS NULL OR COALESCE(NULLIF(max_qty_units,0), max_qty_lb) >= $3)
		ORDER BY COALESCE(NULLIF(min_qty_units,0), min_qty_lb, 0) DESC
		LIMIT 1
	`, r.schema)
	if err := tx.QueryRow(ctx, q, productID, specG, tierQty).Scan(&packagePrice, &pricePerLb); err == nil && pricePerLb > 0 {
		return portalDisplayUnitPriceFromLb(pricePerLb, specG)
	}
	q = fmt.Sprintf(`
		SELECT
			COALESCE(NULLIF(price_per_unit,0), NULLIF(price_per_lb,0) * COALESCE(NULLIF(spec_g,0),454) / 454.0, 0),
			COALESCE(NULLIF(price_per_lb,0), NULLIF(price_per_unit,0) * 454.0 / COALESCE(NULLIF(spec_g,0),454), 0)
		FROM %s.product_price_tiers
		WHERE product_id=$1 AND active=true
		  AND COALESCE(NULLIF(spec_g,0),454)=$2
		ORDER BY COALESCE(NULLIF(min_qty_units,0), min_qty_lb, 0) ASC
		LIMIT 1
	`, r.schema)
	if err := tx.QueryRow(ctx, q, productID, specG).Scan(&packagePrice, &pricePerLb); err == nil && pricePerLb > 0 {
		return portalDisplayUnitPriceFromLb(pricePerLb, specG)
	}
	q = fmt.Sprintf(`
		SELECT COALESCE(NULLIF(price_per_lb,0), NULLIF(price_per_unit,0) * 454.0 / COALESCE(NULLIF(spec_g,0),454), 0)
		FROM %s.product_price_tiers
		WHERE product_id=$1 AND active=true
		  AND COALESCE(NULLIF(min_qty_lb,0), NULLIF(min_qty_units,0) * COALESCE(NULLIF(spec_g,0),454) / 454.0, 0) <= $2
		  AND (
		    COALESCE(NULLIF(max_qty_lb,0), NULLIF(max_qty_units,0) * COALESCE(NULLIF(spec_g,0),454) / 454.0) IS NULL
		    OR COALESCE(NULLIF(max_qty_lb,0), NULLIF(max_qty_units,0) * COALESCE(NULLIF(spec_g,0),454) / 454.0) >= $2
		  )
		ORDER BY COALESCE(NULLIF(min_qty_lb,0), NULLIF(min_qty_units,0) * COALESCE(NULLIF(spec_g,0),454) / 454.0, 0) DESC
		LIMIT 1
	`, r.schema)
	if err := tx.QueryRow(ctx, q, productID, tierQtyLb).Scan(&pricePerLb); err == nil && pricePerLb > 0 {
		return portalDisplayUnitPriceFromLb(pricePerLb, specG)
	}
	q = fmt.Sprintf(`
		SELECT COALESCE(NULLIF(price_per_lb,0), NULLIF(price_per_unit,0) * 454.0 / COALESCE(NULLIF(spec_g,0),454), 0)
		FROM %s.product_price_tiers
		WHERE product_id=$1 AND active=true
		  AND COALESCE(NULLIF(min_qty_lb,0), NULLIF(min_qty_units,0) * COALESCE(NULLIF(spec_g,0),454) / 454.0, 0) <= $2
		ORDER BY COALESCE(NULLIF(min_qty_lb,0), NULLIF(min_qty_units,0) * COALESCE(NULLIF(spec_g,0),454) / 454.0, 0) DESC
		LIMIT 1
	`, r.schema)
	if err := tx.QueryRow(ctx, q, productID, tierQtyLb).Scan(&pricePerLb); err == nil && pricePerLb > 0 {
		return portalDisplayUnitPriceFromLb(pricePerLb, specG)
	}
	q = fmt.Sprintf(`
		SELECT COALESCE(NULLIF(price_per_lb,0), NULLIF(price_per_unit,0) * 454.0 / COALESCE(NULLIF(spec_g,0),454), 0)
		FROM %s.product_price_tiers
		WHERE product_id=$1 AND active=true
		ORDER BY COALESCE(NULLIF(min_qty_lb,0), NULLIF(min_qty_units,0) * COALESCE(NULLIF(spec_g,0),454) / 454.0, 0) ASC
		LIMIT 1
	`, r.schema)
	if err := tx.QueryRow(ctx, q, productID).Scan(&pricePerLb); err == nil && pricePerLb > 0 {
		return portalDisplayUnitPriceFromLb(pricePerLb, specG)
	}
	return defaultPrice
}

func portalTierQuantityForSpec(specG int64, units int64) float64 {
	if specG >= 1000 {
		return float64(specG*units) / 1000.0
	}
	return float64(units)
}

func portalDisplayUnitG(specG int64) float64 {
	if specG >= 1000 {
		return 1000
	}
	return 454
}

func portalDisplayUnitPriceFromLb(pricePerLb float64, specG int64) float64 {
	if pricePerLb <= 0 || specG <= 0 {
		return 0
	}
	unitG := portalDisplayUnitG(specG)
	displayUnitPrice := pricePerLb * unitG / 454.0
	if unitG == 1000 {
		displayUnitPrice = math.Round(displayUnitPrice)
	}
	return displayUnitPrice
}

func portalLineTotalFromDisplayUnit(unitPrice float64, specG int64, units int64) float64 {
	if unitPrice <= 0 || specG <= 0 || units <= 0 {
		return 0
	}
	return unitPrice * float64(specG*units) / portalDisplayUnitG(specG)
}

func (r Repository) portalDirectShipSmallBatchPriceRuleTx(ctx context.Context, tx pgx.Tx, customerID int64) customerportalapp.SmallBatchPriceRule {
	if customerID <= 0 {
		return customerportalapp.SmallBatchPriceRule{}
	}
	var raw []byte
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT config_json
		FROM %s.customer_service_capabilities
		WHERE customer_id=$1 AND capability_code=$2 AND enabled=true
	`, r.schema), customerID, customerportalapp.CapabilityDirectShip).Scan(&raw)
	if err != nil {
		return customerportalapp.SmallBatchPriceRule{}
	}
	var config portalDirectShipCapabilityConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		return customerportalapp.SmallBatchPriceRule{}
	}
	return portalNormalizeSmallBatchPriceRule(config.SmallBatchPriceRule)
}

func portalNormalizeSmallBatchPriceRule(rule customerportalapp.SmallBatchPriceRule) customerportalapp.SmallBatchPriceRule {
	if !rule.Enabled {
		return customerportalapp.SmallBatchPriceRule{}
	}
	if rule.ThresholdLB <= 0 {
		rule.ThresholdLB = 14
	}
	if rule.TierMinLB <= 0 {
		rule.TierMinLB = 15
	}
	if rule.TierMaxLB <= 0 {
		rule.TierMaxLB = 28
	}
	return rule
}

func portalSmallBatchTierQuantity(specG int64, qtyLb float64, rule customerportalapp.SmallBatchPriceRule) (int64, bool) {
	rule = portalNormalizeSmallBatchPriceRule(rule)
	if !rule.Enabled || specG <= 0 || qtyLb <= 0 || qtyLb >= rule.ThresholdLB {
		return 0, false
	}
	targetUnits := int64(math.Ceil(rule.TierMinLB * 454.0 / float64(specG)))
	if targetUnits < 1 {
		targetUnits = 1
	}
	return targetUnits, true
}

func (r Repository) processingWarehouseForCustomerTx(ctx context.Context, tx pgx.Tx, customerID int64) (string, error) {
	code := ""
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT code
		FROM %s.warehouses
		WHERE active=true
		  AND customer_id=$1
		ORDER BY
		  CASE WHEN kind IN ('customer_processing','customer_finished','customer') THEN 0 ELSE 1 END,
		  sort_order,
		  code
		LIMIT 1
	`, r.schema), customerID).Scan(&code)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("customer warehouse binding required")
	}
	if err != nil {
		return "", err
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return "", fmt.Errorf("customer warehouse binding required")
	}
	return code, nil
}

func (r Repository) defaultSenderForCustomerTx(ctx context.Context, tx pgx.Tx, customerID int64) (int64, error) {
	var senderID int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(default_sender_id,0)
		FROM %s.customer_portal_profiles
		WHERE customer_id=$1
	`, r.schema), customerID).Scan(&senderID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}
	return senderID, nil
}

func nextCustomerPortalOrderNo(ctx context.Context, tx pgx.Tx, schema string, od time.Time) (string, error) {
	ymd := od.Format("20060102")
	prefix := "SO-" + ymd + "-"
	var maxNo int
	q := fmt.Sprintf(`
		SELECT COALESCE(MAX(CAST(right(order_no,4) AS INT)), 0)
		FROM %s.orders
		WHERE order_no LIKE $1
		  AND right(order_no,4) ~ '^[0-9]{4}$'
	`, schema)
	if err := tx.QueryRow(ctx, q, prefix+"%").Scan(&maxNo); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%04d", prefix, maxNo+1), nil
}

func customerPortalStatusID(ctx context.Context, tx pgx.Tx, schema, table string, names ...string) int64 {
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		var id int64
		q := fmt.Sprintf("SELECT id FROM %s.%s WHERE name=$1 ORDER BY id LIMIT 1", schema, table)
		if err := tx.QueryRow(ctx, q, name).Scan(&id); err == nil && id > 0 {
			return id
		}
	}
	return 0
}

func portalNullInt(v int64) any {
	if v <= 0 {
		return nil
	}
	return v
}
