package support

import (
	"strconv"

	"github.com/labstack/echo/v4"
)

func IntParam(c echo.Context, name string, def int) int {
	v := c.QueryParam(name)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
