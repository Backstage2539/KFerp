package main

import (
	"net/http"
	"net/url"

	"github.com/labstack/echo/v4"
)

func vueShellRedirect(c echo.Context, view string) error {
	return vueShellRedirectWith(c, view, nil)
}

func vueShellRedirectWith(c echo.Context, view string, extras map[string]string) error {
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
	return c.Redirect(http.StatusFound, target)
}
