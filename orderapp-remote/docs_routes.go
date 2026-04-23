package main

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/labstack/echo/v4"
)

func registerDocsRoutes(e *echo.Echo) {
	h := docsHandler{dir: "/app/docs"}

	e.GET("/docs", h.index)
	e.GET("/docs/:name", h.view)
}

type docsHandler struct {
	dir string
}

func (h docsHandler) index(c echo.Context) error {
	data := DocsIndexData{}
	files, err := listDocFiles(h.dir)
	if err != nil {
		data.Error = err.Error()
	} else {
		data.Files = files
	}
	return c.Render(http.StatusOK, "docs.html", data)
}

func (h docsHandler) view(c echo.Context) error {
	name, err := safeDocName(c.Param("name"))
	if err != nil {
		return c.String(http.StatusBadRequest, "invalid name")
	}
	bs, err := os.ReadFile(filepath.Join(h.dir, name))
	if err != nil {
		return c.String(http.StatusNotFound, "not found")
	}
	if c.QueryParam("raw") == "1" {
		c.Response().Header().Set(echo.HeaderContentType, "text/plain; charset=utf-8")
		return c.String(http.StatusOK, string(bs))
	}
	return c.Render(http.StatusOK, "doc_view.html", DocViewData{Name: name, Content: string(bs)})
}
