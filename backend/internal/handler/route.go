package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/middleware"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/response"
)

type RouteHandler struct {
	svc *service.RouteService
}

func NewRouteHandler(svc *service.RouteService) *RouteHandler {
	return &RouteHandler{svc: svc}
}

func (h *RouteHandler) GetConstantRoutes(c *gin.Context) {
	routes := h.svc.GetConstantRoutes()
	response.Success(c, routes)
}

func (h *RouteHandler) GetUserRoutes(c *gin.Context) {
	perms, exists := c.Get(middleware.ContextKeyPermissions)
	var permissions []string
	if exists {
		permissions, _ = perms.([]string)
	}
	if permissions == nil {
		permissions = []string{}
	}

	result := h.svc.GetUserRoutes(permissions)
	response.Success(c, result)
}
