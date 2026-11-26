package customerportal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
	"golang.org/x/sync/singleflight"
	"net/http"
	portalapp "orderapp/internal/application/customerportal"
	salesapp "orderapp/internal/application/sales"
	"slices"
	"strconv"
	"sync"
)

type entryResponse map[string]any

// Payloads are immutable after publication. The revision itself names the
// scope, so a client's version can never match another price table or account.
type employeeEntryCache struct {
	mu     sync.Mutex
	data   map[string]entryResponse
	order  []string
	builds singleflight.Group
}

func (c *employeeEntryCache) get(key string) entryResponse {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.data[key]
}
func (c *employeeEntryCache) put(key string, payload entryResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.data[key]; !ok {
		c.order = append(c.order, key)
	}
	c.data[key] = payload
	if len(c.order) > 64 {
		delete(c.data, c.order[0])
		c.order = c.order[1:]
	}
}
func entryRevision(key, version string) string {
	sum := sha256.Sum256([]byte("employee-entry-v1/" + key + "/" + version))
	return hex.EncodeToString(sum[:16])
}

type entrySelection struct {
	customer            int64
	selected, preferred []int64
}

func (s entrySelection) key(part string, employee portalapp.CurrentContext) string {
	if part == "base" {
		return fmt.Sprintf("base/%d/%t/%t/%t", employee.EmployeeID, containsMiniRole(employee.Roles, "admin"), containsMiniRole(employee.Permissions, "customers.read"), containsMiniRole(employee.Permissions, "customers.write"))
	}
	ids := slices.Clone(s.selected)
	slices.Sort(ids)
	return fmt.Sprintf("%s/%d/%v/%v", part, s.customer, ids, s.preferred)
}
func registerEmployeeEntry(e *echo.Echo, portal Service, sales EmployeeSales, warehouses EmployeeWarehouseReader) {
	cache := &employeeEntryCache{data: make(map[string]entryResponse)}
	reader, _ := sales.(salesapp.EmployeeEntryReader)
	e.GET("/api/mini/employee/order-entry/:part", func(c echo.Context) error {
		ctx := c.Request().Context()
		c.Response().Header().Set("Cache-Control", "no-store")
		employee, err := requireMiniEmployee(ctx, c.Request().Header.Get(echo.HeaderAuthorization), portal, "orders.write")
		if err != nil {
			return miniEmployeeAuthError(c, err)
		}
		if reader == nil {
			return miniInternalError(c)
		}
		part := c.Param("part")
		q := entrySelection{}
		switch part {
		case "base":
		case "catalog", "warehouses":
			q.customer, err = strconv.ParseInt(c.QueryParam("customer_id"), 10, 64)
			if err != nil || q.customer <= 0 {
				return c.JSON(400, map[string]string{"error": "客户编号不正确"})
			}
			if part == "catalog" {
				q.selected, err = miniSelectedPriceTableIDs(c.QueryParam("selected_price_table_ids"))
				if err == nil && len(q.selected) == 0 {
					q.preferred, err = miniSelectedPriceTableIDs(c.QueryParam("preferred_price_table_ids"))
				}
				if err != nil {
					return c.JSON(400, map[string]string{"error": err.Error()})
				}
			}
		default:
			return echo.ErrNotFound
		}
		key := q.key(part, employee)
		for attempt := 0; attempt < 3; attempt++ {
			version, err := reader.EmployeeEntryRevision(ctx, part, q.customer)
			if errors.Is(err, pgx.ErrNoRows) {
				return c.JSON(404, map[string]string{"error": "客户不存在"})
			}
			if err != nil {
				return miniInternalError(c)
			}
			rev := entryRevision(key, version)
			if c.QueryParam("known_revision") == rev {
				return c.JSON(200, entryResponse{"unchanged": true, "revision": rev})
			}
			if hit := cache.get(rev); hit != nil {
				return c.JSON(200, hit)
			}
			result, err, _ := cache.builds.Do(rev, func() (any, error) {
				if hit := cache.get(rev); hit != nil {
					return hit, nil
				}
				payload, err := buildEmployeeEntry(ctx, reader, warehouses, part, q, employee)
				if err != nil {
					return nil, err
				}
				after, err := reader.EmployeeEntryRevision(ctx, part, q.customer)
				if err != nil {
					return nil, err
				}
				if version != after {
					return nil, nil
				}
				payload["revision"] = rev
				if part == "catalog" {
					explicit := entrySelection{customer: q.customer, selected: payload["selected_price_table_ids"].([]int64)}
					canonical := entryRevision(explicit.key(part, employee), version)
					payload["selected_revision"] = canonical
					if canonical != rev {
						alias := entryResponse{}
						for k, v := range payload {
							alias[k] = v
						}
						alias["revision"] = canonical
						alias["catalog_updated"] = false
						cache.put(canonical, alias)
					}
				}
				cache.put(rev, payload)
				return payload, nil
			})
			if err != nil {
				var httpErr *echo.HTTPError
				if errors.As(err, &httpErr) {
					return c.JSON(httpErr.Code, map[string]any{"error": httpErr.Message})
				}
				return miniInternalError(c)
			}
			if result != nil {
				return c.JSON(200, result)
			}
		}
		return c.JSON(http.StatusConflict, map[string]string{"error": "配置正在更新，请重试"})
	})
}
func buildEmployeeEntry(ctx context.Context, reader salesapp.EmployeeEntryReader, warehouses EmployeeWarehouseReader, part string, q entrySelection, employee portalapp.CurrentContext) (entryResponse, error) {
	if part == "warehouses" {
		rows, err := employeeEntryWarehouses(ctx, warehouses, q.customer)
		return entryResponse{"delivery_warehouses": rows}, err
	}
	form, err := reader.EmployeeEntryData(ctx, part, q.customer)
	if err != nil {
		return nil, err
	}
	if part == "base" {
		return entryResponse{"customers": employeeEntryCustomers(employee, form), "sources": form.Sources, "pay_statuses": form.PayStatuses, "ship_statuses": form.ShipStatuses, "order_types": form.OrderTypes}, nil
	}
	ids := q.selected
	if len(ids) == 0 && len(q.preferred) > 0 {
		ids = preferredCurrentOrderTables(salesapp.CurrentOrderPriceTableOptions(form.BeanListVersionOptions, q.customer), q.preferred)
	}
	resolved, err := salesapp.ResolveOrderCatalog(form, q.customer, ids, true)
	if err != nil {
		return nil, echo.NewHTTPError(400, err.Error())
	}
	return entryResponse{"product_families": resolved.Catalog.Families, "product_bom_spec_options": resolved.Catalog.BOMSpecOptions, "selected_price_table_ids": resolved.SelectedPriceTableIDs, "price_table_options": resolved.PriceTableOptions, "catalog_updated": resolved.Updated}, nil
}
