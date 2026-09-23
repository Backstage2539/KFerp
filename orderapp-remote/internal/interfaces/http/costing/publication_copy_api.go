package costing

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	appcosting "orderapp/internal/application/costing"
	"orderapp/internal/interfaces/http/support"

	"github.com/labstack/echo/v4"
)

type publicationCopyService interface {
	PreviewBeanListPublicationCopy(context.Context, appcosting.BeanListPublicationCopyCommand) (appcosting.BeanListPublicationCopyStats, error)
	CopyBeanListPublicationToDraft(context.Context, appcosting.BeanListPublicationCopyCommand) (appcosting.BeanListPublicationCopyResult, error)
}

func registerPublicationCopyAPI(e *echo.Echo, svc Service, authz support.AuthzService) {
	for _, path := range []string{"copy-preview", "copy-to-draft"} {
		copyPath := path
		e.POST("/api/costing/bean-list/publications/:id/"+copyPath, func(c echo.Context) error {
			if err := requireBeanListPublisher(c, authz); err != nil {
				return err
			}
			copySvc, ok := svc.(publicationCopyService)
			if !ok {
				return c.JSON(http.StatusNotImplemented, map[string]string{"error": "price table copy unavailable"})
			}
			sourceID, err := strconv.ParseInt(c.Param("id"), 10, 64)
			if err != nil || sourceID <= 0 {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid source publication id"})
			}
			sourceQuery, err := beanListPublicationQueryFromRequest(c)
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			if sourceQuery.PublicationPurpose != appcosting.BeanListPublicationPurposeFactorySupply || (sourceQuery.OwnerType != "official" && sourceQuery.OwnerType != "customer") {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "来源必须是有权限查看的公共或客户供货价格表"})
			}
			var request struct {
				CustomerID     int64                           `json:"customer_id"`
				TargetTableKey string                          `json:"target_table_key"`
				CopyRequestID  string                          `json:"copy_request_id"`
				Batch          appcosting.BeanListBatchCommand `json:"batch"`
			}
			if err := c.Bind(&request); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
			}
			if request.CustomerID <= 0 || strings.TrimSpace(request.TargetTableKey) == "" {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "目标客户和价格表不能为空"})
			}
			if copyPath == "copy-to-draft" && strings.TrimSpace(request.CopyRequestID) == "" {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "复制请求标识不能为空，请刷新后重试"})
			}
			ownerType, ownerKey, err := beanListOwnerFromScope(c, "customer", request.CustomerID)
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			request.Batch.Scope = "customer"
			request.Batch.CustomerID = request.CustomerID
			request.Batch.OwnerType, request.Batch.OwnerKey = ownerType, ownerKey
			request.Batch.PublicationPurpose = appcosting.BeanListPublicationPurposeFactorySupply
			request.Batch.Actor = support.ActorOf(c)
			cmd := appcosting.BeanListPublicationCopyCommand{
				SourceQuery: sourceQuery, SourcePublicationID: sourceID, CustomerID: request.CustomerID,
				TargetTableKey: request.TargetTableKey, CopyRequestID: request.CopyRequestID, Batch: request.Batch,
			}
			if copyPath == "copy-preview" {
				stats, err := copySvc.PreviewBeanListPublicationCopy(c.Request().Context(), cmd)
				if err != nil {
					return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
				}
				return c.JSON(http.StatusOK, map[string]any{"copy_stats": stats})
			}
			result, err := copySvc.CopyBeanListPublicationToDraft(c.Request().Context(), cmd)
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, result)
		})
	}
}
