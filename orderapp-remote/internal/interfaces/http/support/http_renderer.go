package support

import (
	"html/template"
	"io"

	"github.com/labstack/echo/v4"
)

type TemplateRenderer struct{ t *template.Template }

func NewTemplateRenderer(t *template.Template) echo.Renderer {
	return &TemplateRenderer{t: t}
}

func (tr *TemplateRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return tr.t.ExecuteTemplate(w, name, data)
}
