package support

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

const keyHideCustomerAccountFulfillment = "ui.customer_account.hide_fulfillment_console"
const keyMiniappShareImageNeedShowEntrance = "miniapp.share_image.need_show_entrance"

type UISettings struct {
	HideCustomerAccountFulfillment bool `json:"hide_customer_account_fulfillment"`
}

type AppConfigStore interface {
	Get(ctx context.Context, key string) (string, bool, error)
	Set(ctx context.Context, actor, key, value string) error
}

type uiSettingsStore = AppConfigStore

type pgUISettingsStore struct {
	pool   *pgxpool.Pool
	schema string
}

func newPGUISettingsStore(pool *pgxpool.Pool, schema string) pgUISettingsStore {
	return pgUISettingsStore{pool: pool, schema: schema}
}

func NewAppConfigStore(pool *pgxpool.Pool, schema string) AppConfigStore {
	return newPGUISettingsStore(pool, schema)
}

func ensureAppConfigTable(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.app_config (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_by TEXT NOT NULL DEFAULT ''
);
`, schema))
	return err
}

func (s pgUISettingsStore) Get(ctx context.Context, key string) (string, bool, error) {
	return appConfigValue(ctx, s.pool, s.schema, key)
}

type appConfigQueryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func appConfigValue(ctx context.Context, queryer appConfigQueryer, schema, key string) (string, bool, error) {
	var value string
	err := queryer.QueryRow(ctx, fmt.Sprintf(`SELECT value FROM %s.app_config WHERE key=$1`, schema), key).Scan(&value)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", false, nil
		}
		return "", false, err
	}
	return value, true, nil
}

func (s pgUISettingsStore) Set(ctx context.Context, actor, key, value string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1)::bigint)`, key); err != nil {
		return err
	}

	oldValue, hadOld, err := appConfigValue(ctx, tx, s.schema, key)
	if err != nil {
		return err
	}
	oldValue, hadOld = effectiveAppConfigOldValue(key, oldValue, hadOld)
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
INSERT INTO %s.app_config(key,value,updated_at,updated_by)
VALUES($1,$2,now(),$3)
ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=now(), updated_by=excluded.updated_by
`, s.schema), key, value, strings.TrimSpace(actor)); err != nil {
		return err
	}
	var oldPtr *string
	if hadOld {
		oldPtr = StrPtr(oldValue)
	}
	if err := NewAuditService(tx, s.schema).Insert(ctx, AuditEntry{
		Actor:      actor,
		EntityType: "ui_setting",
		Action:     "update",
		Field:      StrPtr(key),
		OldValue:   oldPtr,
		NewValue:   StrPtr(value),
		Meta:       AuditMeta{"key": key},
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func effectiveAppConfigOldValue(key, raw string, exists bool) (string, bool) {
	defaultValue, hasDefault := map[string]string{
		keyHideCustomerAccountFulfillment:    "true",
		keyMiniappShareImageNeedShowEntrance: "true",
	}[strings.TrimSpace(key)]
	if !hasDefault {
		return raw, exists
	}
	if !exists {
		return defaultValue, true
	}
	return raw, true
}

func defaultUISettings() UISettings {
	return UISettings{HideCustomerAccountFulfillment: true}
}

func loadUISettings(ctx context.Context, store uiSettingsStore) (UISettings, error) {
	settings := defaultUISettings()
	if store == nil {
		return settings, nil
	}
	if raw, ok, err := store.Get(ctx, keyHideCustomerAccountFulfillment); err != nil {
		return UISettings{}, err
	} else if ok {
		parsed, parseErr := strconv.ParseBool(strings.TrimSpace(raw))
		if parseErr == nil {
			settings.HideCustomerAccountFulfillment = parsed
		}
	}
	return settings, nil
}

func registerUISettingsAPI(e *echo.Echo, store uiSettingsStore, authz AuthzService) {
	e.GET("/api/ui-settings", func(c echo.Context) error {
		if _, ok, err := CurrentActor(c, authz); err != nil {
			return c.JSON(http.StatusForbidden, map[string]string{"error": err.Error()})
		} else if !ok {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "auth required"})
		}
		settings, err := loadUISettings(c.Request().Context(), store)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"settings": settings})
	})

	e.PUT("/api/ui-settings", func(c echo.Context) error {
		actor, ok, err := CurrentActor(c, authz)
		if err != nil {
			return c.JSON(http.StatusForbidden, map[string]string{"error": err.Error()})
		}
		if !ok {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "auth required"})
		}
		if !actor.Can("settings.write") {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "permission denied", "permission": "settings.write"})
		}
		var req UISettings
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		value := strconv.FormatBool(req.HideCustomerAccountFulfillment)
		if err := store.Set(c.Request().Context(), actor.Name, keyHideCustomerAccountFulfillment, value); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		settings, err := loadUISettings(c.Request().Context(), store)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"settings": settings})
	})
}
