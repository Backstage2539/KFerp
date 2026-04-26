package main

import (
	"net/http"
	"net/url"

	"github.com/labstack/echo/v4"
)

func vueShellRedirect(c echo.Context, view string) error {
	target := "/vue-shell?view=" + url.QueryEscape(view)
	values := c.QueryParams()
	values.Del("legacy")
	if encoded := values.Encode(); encoded != "" {
		target += "&" + encoded
	}
	return c.Redirect(http.StatusFound, target)
}
