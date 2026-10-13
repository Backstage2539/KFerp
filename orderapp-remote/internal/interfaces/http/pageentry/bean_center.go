package pageentry

import (
	"fmt"
	"github.com/labstack/echo/v4"
)

// The anonymous directory contains only selected official publication metadata, never prices or customer names.
func (h *Handler) beanCenter(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "no-store")
	rows, err := h.Repo.Pool.Query(c.Request().Context(), fmt.Sprintf(`SELECT e.entry_key,e.published->>'name',e.published->>'visibility',p.version_no,p.published_at FROM %[1]s.page_entries e JOIN %[1]s.bean_list_publications p ON p.id=(e.published->>'publication_id')::bigint WHERE e.deleted_at IS NULL AND e.enabled AND e.published->>'kind'='price' AND e.published->>'bean_center'='true' AND p.owner_type='official' AND p.status='published' AND p.deleted_at IS NULL ORDER BY COALESCE((e.published->>'bean_sort')::int,0),e.entry_key`, h.Repo.Schema))
	if err != nil {
		return respond(c, nil, err)
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var key, name, visibility, version string
		var updated any
		if err = rows.Scan(&key, &name, &visibility, &version, &updated); err != nil {
			return respond(c, nil, err)
		}
		out = append(out, map[string]any{"key": key, "name": name, "visibility": visibility, "version": version, "updated_at": updated})
	}
	return respond(c, map[string]any{"rows": out}, rows.Err())
}
