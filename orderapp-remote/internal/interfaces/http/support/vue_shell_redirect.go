package support

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v4"
)

func VueShellRedirect(c echo.Context, view string) error {
	return VueShellRedirectWith(c, view, nil)
}

func VueShellRedirectWith(c echo.Context, view string, extras map[string]string) error {
	target := "/vue-shell?view=" + url.QueryEscape(view)
	values := c.QueryParams()
	values.Del("legacy")
	values.Del("view")
	for k, v := range extras {
		if v == "" {
			values.Del(k)
			continue
		}
		values.Set(k, v)
	}
	if encoded := values.Encode(); encoded != "" {
		target += "&" + encoded
	}
	return c.Redirect(http.StatusFound, PrefixRelativeLocation(c, target))
}

func PrefixRelativeLocation(c echo.Context, absoluteTarget string) string {
	target := strings.TrimSpace(absoluteTarget)
	if target == "" || !strings.HasPrefix(target, "/") {
		return target
	}
	path := "/"
	if c != nil && c.Request() != nil && c.Request().URL != nil {
		path = c.Request().URL.Path
	}
	depth := 0
	if trimmed := strings.Trim(path, "/"); trimmed != "" {
		depth = strings.Count(trimmed, "/") + 1
	}
	relative := strings.TrimPrefix(target, "/")
	if depth <= 1 {
		if path != "/" && strings.HasSuffix(path, "/") {
			return "../" + relative
		}
		return relative
	}
	climb := depth - 1
	if strings.HasSuffix(path, "/") {
		climb = depth
	}
	return strings.Repeat("../", climb) + relative
}
