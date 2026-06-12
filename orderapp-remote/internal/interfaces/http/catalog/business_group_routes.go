package catalog

import "github.com/labstack/echo/v4"

func registerBusinessGroupRoutes(e *echo.Echo, h productHandler) {
	e.GET("/api/business-groups", h.businessGroupsAPI)
	e.POST("/api/business-groups", h.saveBusinessGroupAPI)
	e.PUT("/api/business-groups/:id", h.saveBusinessGroupAPI)
	e.DELETE("/api/business-groups/:id", h.deleteBusinessGroupAPI)
	e.POST("/api/business-groups/:id/usages", h.ensureBusinessGroupUsageAPI)
	e.POST("/api/business-group-items", h.saveBusinessGroupItemAPI)
	e.PUT("/api/business-group-items/:id", h.saveBusinessGroupItemAPI)
	e.DELETE("/api/business-group-items/:id", h.deleteBusinessGroupItemAPI)
	e.POST("/api/business-group-items/:id/move", h.moveBusinessGroupItemAPI)
	e.GET("/api/business-group-assignments", h.businessGroupAssignmentsAPI)
	e.POST("/api/business-group-assignments", h.saveBusinessGroupAssignmentAPI)
	e.PUT("/api/business-group-assignments/:id", h.saveBusinessGroupAssignmentAPI)
	e.DELETE("/api/business-group-assignments/:id", h.deleteBusinessGroupAssignmentAPI)
}
