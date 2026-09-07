package costing

import (
	"context"
	"github.com/labstack/echo/v4"
	"net/http"
	appcosting "orderapp/internal/application/costing"
	"orderapp/internal/interfaces/http/support"
	"strings"
)

type publicationBatchService interface {
	PublishBeanListBatch(context.Context, appcosting.BeanListBatchCommand) (*appcosting.BeanListBatchResult, error)
	SaveBeanListDraftBatch(context.Context, appcosting.BeanListBatchCommand) (*appcosting.BeanListBatchResult, error)
}

func registerPublicationBatchAPI(e *echo.Echo, svc Service, authz support.AuthzService) {
	for _, publish := range []bool{true, false} {
		path := "/api/costing/bean-list/draft-batches"
		if publish {
			path = "/api/costing/bean-list/publication-batches"
		}
		e.POST(path, func(c echo.Context) error {
			if publish {
				if err := requireBeanListPublisher(c, authz); err != nil {
					return err
				}
			}
			batchSvc, ok := svc.(publicationBatchService)
			if !ok {
				return c.JSON(http.StatusNotImplemented, map[string]string{"error": "batch publication unavailable"})
			}
			var req appcosting.BeanListBatchCommand
			if err := c.Bind(&req); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
			}
			if !publish {
				canPublish, err := currentActorCanPublishBeanList(c, authz)
				if err != nil {
					return err
				}
				if !canPublish {
					req.Scope = "mine"
					req.CustomerID = 0
				}
			}
			if strings.TrimSpace(req.PublicationPurpose) == "" {
				req.PublicationPurpose = appcosting.BeanListPublicationPurposeFactorySupply
			}
			req.Actor = support.ActorOf(c)
			ownerType, ownerKey, err := beanListOwnerFromScope(c, req.Scope, req.CustomerID)
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			req.OwnerType, req.OwnerKey = ownerType, ownerKey
			var result *appcosting.BeanListBatchResult
			if publish {
				result, err = batchSvc.PublishBeanListBatch(c.Request().Context(), req)
			} else {
				result, err = batchSvc.SaveBeanListDraftBatch(c.Request().Context(), req)
			}
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			// The transaction is committed. A retryable asset failure must never look
			// like a failed publication and cause the user to publish a second version.
			if publish {
				result.PDFErrors = map[int64]string{}
				for i := range result.Tables {
					if err := generateBeanListPublicationPDFAsset(c, svc, &result.Tables[i]); err != nil {
						result.PDFErrors[result.Tables[i].ID] = err.Error()
					}
				}
			}
			return c.JSON(http.StatusOK, result)
		})
	}
}
