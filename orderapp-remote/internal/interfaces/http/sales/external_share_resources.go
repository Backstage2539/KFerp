package sales

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	salesapp "orderapp/internal/application/sales"
	support "orderapp/internal/interfaces/http/support"
	"strings"

	"github.com/labstack/echo/v4"
)

type externalShareResourceHandler struct {
	sales *salesapp.Service
}

type externalShareResourceRequest struct {
	ResourceType string `json:"resource_type"`
	OrderID      int64  `json:"order_id"`
	DocumentID   int64  `json:"document_id"`
	Latest       bool   `json:"latest"`
}

func registerExternalShareResourceRoutes(e *echo.Echo, salesSvc *salesapp.Service) {
	h := externalShareResourceHandler{sales: salesSvc}
	e.POST("/api/share-resources", h.create)
	e.GET("/share/:token", h.page)
	e.GET("/share/:token/file", h.file)
}

func (h externalShareResourceHandler) create(c echo.Context) error {
	var req externalShareResourceRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid json"})
	}
	if !req.Latest && req.DocumentID <= 0 {
		req.Latest = true
	}
	share, err := h.sales.CreateExternalShareResource(c.Request().Context(), salesapp.CreateExternalShareResourceCommand{
		Actor:        support.ActorOf(c),
		ResourceType: req.ResourceType,
		OrderID:      req.OrderID,
		DocumentID:   req.DocumentID,
		Latest:       req.Latest,
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, share)
}

func (h externalShareResourceHandler) page(c echo.Context) error {
	file, err := h.sales.LoadExternalShareResourceFile(c.Request().Context(), c.Param("token"))
	if err != nil {
		return c.String(http.StatusNotFound, "share resource not found")
	}
	var buf bytes.Buffer
	data := struct {
		Title       string
		FileURL     string
		Filename    string
		ContentType string
		IsImage     bool
	}{
		Title:       file.Resource.Title,
		FileURL:     file.Resource.FileURL,
		Filename:    file.Resource.Filename,
		ContentType: file.Resource.ContentType,
		IsImage:     strings.HasPrefix(file.Resource.ContentType, "image/"),
	}
	if err := externalSharePageTemplate.Execute(&buf, data); err != nil {
		return c.String(http.StatusInternalServerError, "render share page failed")
	}
	return c.HTMLBlob(http.StatusOK, buf.Bytes())
}

func (h externalShareResourceHandler) file(c echo.Context) error {
	file, err := h.sales.LoadExternalShareResourceFile(c.Request().Context(), c.Param("token"))
	if err != nil {
		return c.String(http.StatusNotFound, "share resource not found")
	}
	c.Response().Header().Set(echo.HeaderContentType, file.Resource.ContentType)
	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf(`inline; filename="%s"`, file.Resource.Filename))
	return c.File(file.Path)
}

var externalSharePageTemplate = template.Must(template.New("external-share").Parse(`<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}}</title>
  <style>
    * { box-sizing: border-box; }
    body { margin: 0; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; color: #171717; background: #f7f4ef; }
    main { width: min(920px, 100%); margin: 0 auto; padding: 18px; }
    .panel { background: #fff; border: 1px solid #e5ded3; border-radius: 8px; padding: 16px; }
    h1 { margin: 0 0 12px; font-size: 22px; line-height: 1.35; }
    .hint { color: #666; margin: 0 0 14px; line-height: 1.6; }
    .action { display: inline-flex; align-items: center; min-height: 42px; padding: 9px 14px; border-radius: 6px; background: #111; color: #fff; text-decoration: none; font-weight: 700; }
    .preview { margin-top: 16px; border-top: 1px solid #eee2d4; padding-top: 16px; }
    img { max-width: 100%; height: auto; display: block; border: 1px solid #eee2d4; border-radius: 6px; background: #fff; }
    iframe { width: 100%; height: min(78vh, 900px); border: 1px solid #eee2d4; border-radius: 6px; background: #fff; }
    @media (max-width: 700px) { main { padding: 10px; } h1 { font-size: 18px; } iframe { height: 74vh; } }
  </style>
</head>
<body>
  <main>
    <section class="panel">
      <h1>{{.Title}}</h1>
      <p class="hint">微信分享资源文件。请优先使用 ERP 内的分享按钮直接发送文件；本页仅用于打开文件。</p>
      <a class="action" href="{{.FileURL}}">打开文件</a>
      <div class="preview">
        {{if .IsImage}}
          <img src="{{.FileURL}}" alt="{{.Filename}}">
        {{else}}
          <iframe src="{{.FileURL}}" title="{{.Filename}}"></iframe>
        {{end}}
      </div>
    </section>
  </main>
</body>
</html>`))
