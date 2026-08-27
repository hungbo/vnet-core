package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/response"
)

type SystemManageHandler struct {
	svc *service.SystemManageService
}

func NewSystemManageHandler(svc *service.SystemManageService) *SystemManageHandler {
	return &SystemManageHandler{svc: svc}
}

// @Summary Get user list
// @Description Get a paginated list of system users
// @Tags SystemManage
// @Accept json
// @Produce json
// @Param current query int false "Current page"
// @Param size query int false "Page size"
// @Param search query string false "Search keyword"
// @Success 200 {object} response.Response{data=service.PaginatedRecords}
// @Router /systemManage/getUserList [get]
// @Security BearerAuth
func (h *SystemManageHandler) GetUserList(c *gin.Context) {
	var params service.UserListParams
	if err := c.ShouldBindQuery(&params); err != nil {
		response.BadRequest(c, "Invalid params: "+err.Error())
		return
	}

	result, err := h.svc.ListUsers(&params)
	if err != nil {
		response.InternalError(c, "Failed to fetch users")
		return
	}
	response.Success(c, result)
}

// @Summary Add user
// @Description Create a new system user
// @Tags SystemManage
// @Accept json
// @Produce json
// @Param body body service.CreateUserRequest true "User details"
// @Success 201 {object} response.Response{data=service.UserManageResponse}
// @Router /systemManage/addUser [post]
// @Security BearerAuth
func (h *SystemManageHandler) AddUser(c *gin.Context) {
	var req service.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	result, err := h.svc.CreateUser(&req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, result)
}

// @Summary Update user
// @Description Update an existing system user
// @Tags SystemManage
// @Accept json
// @Produce json
// @Param body body service.UpdateUserRequest true "Updated user details"
// @Success 200 {object} response.Response{data=service.UserManageResponse}
// @Router /systemManage/updateUser [post]
// @Security BearerAuth
func (h *SystemManageHandler) UpdateUser(c *gin.Context) {
	var req service.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	result, err := h.svc.UpdateUser(&req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
}

// @Summary Delete user
// @Description Delete a system user by ID
// @Tags SystemManage
// @Accept json
// @Produce json
// @Param id query string false "User ID"
// @Success 200 {object} response.Response
// @Failure 409 {object} response.Response "còn dữ liệu phụ thuộc"
// @Router /systemManage/deleteUser [delete]
// @Security BearerAuth
func (h *SystemManageHandler) DeleteUser(c *gin.Context) {
	var body struct {
		ID string `json:"id" form:"id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		if err := c.ShouldBindQuery(&body); err != nil {
			response.BadRequest(c, "id is required")
			return
		}
	}

	if body.ID == "" {
		response.BadRequest(c, "id is required")
		return
	}

	if err := h.svc.DeleteUser(body.ID); err != nil {
		handleDeleteError(c, err)
		return
	}
	response.Success(c, nil)
}

// @Summary Batch delete users
// @Description Delete multiple system users
// @Tags SystemManage
// @Accept json
// @Produce json
// @Param body body object true "User IDs"
// @Success 200 {object} response.Response
// @Failure 409 {object} response.Response "còn dữ liệu phụ thuộc"
// @Router /systemManage/batchDeleteUser [delete]
// @Security BearerAuth
func (h *SystemManageHandler) BatchDeleteUser(c *gin.Context) {
	var body struct {
		IDs []string `json:"ids"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		handleValidationError(c, err)
		return
	}

	if err := h.svc.BatchDeleteUsers(body.IDs); err != nil {
		handleDeleteError(c, err)
		return
	}
	response.Success(c, nil)
}

// @Summary Get role list
// @Description Get a paginated list of system roles
// @Tags SystemManage
// @Accept json
// @Produce json
// @Param current query int false "Current page"
// @Param size query int false "Page size"
// @Param search query string false "Search keyword"
// @Success 200 {object} response.Response{data=service.PaginatedRecords}
// @Router /systemManage/getRoleList [get]
// @Security BearerAuth
func (h *SystemManageHandler) GetRoleList(c *gin.Context) {
	var params service.RoleListParams
	if err := c.ShouldBindQuery(&params); err != nil {
		response.BadRequest(c, "Invalid params: "+err.Error())
		return
	}

	result, err := h.svc.ListRoles(&params)
	if err != nil {
		response.InternalError(c, "Failed to fetch roles")
		return
	}
	response.Success(c, result)
}

// @Summary Get all roles
// @Description Get all enabled roles for dropdown
// @Tags SystemManage
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=[]service.AllRoleResponse}
// @Router /systemManage/getAllRoles [get]
// @Security BearerAuth
func (h *SystemManageHandler) GetAllRoles(c *gin.Context) {
	roles, err := h.svc.GetAllRoles()
	if err != nil {
		response.InternalError(c, "Failed to fetch roles")
		return
	}
	response.Success(c, roles)
}

// @Summary Get menu list
// @Description Get paginated menu list
// @Tags SystemManage
// @Accept json
// @Produce json
// @Param current query int false "Current page"
// @Param size query int false "Page size"
// @Success 200 {object} response.Response{data=service.PaginatedRecords}
// @Router /systemManage/getMenuList/v2 [get]
// @Security BearerAuth
func (h *SystemManageHandler) GetMenuList(c *gin.Context) {
	var params service.SystemListParams
	if err := c.ShouldBindQuery(&params); err != nil {
		response.BadRequest(c, "Invalid params: "+err.Error())
		return
	}

	result, err := h.svc.GetMenuList(&params)
	if err != nil {
		response.InternalError(c, "Failed to fetch menus")
		return
	}
	response.Success(c, result)
}

// @Summary Get all pages
// @Description Get all available page route names
// @Tags SystemManage
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=[]string}
// @Router /systemManage/getAllPages [get]
// @Security BearerAuth
func (h *SystemManageHandler) GetAllPages(c *gin.Context) {
	pages, err := h.svc.GetAllPages()
	if err != nil {
		response.InternalError(c, "Failed to fetch pages")
		return
	}
	response.Success(c, pages)
}

// @Summary Get menu tree
// @Description Get menu tree structure for role permission assignment
// @Tags SystemManage
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=[]service.MenuTreeResponse}
// @Router /systemManage/getMenuTree [get]
// @Security BearerAuth
func (h *SystemManageHandler) GetMenuTree(c *gin.Context) {
	tree, err := h.svc.GetMenuTree()
	if err != nil {
		response.InternalError(c, "Failed to fetch menu tree")
		return
	}
	response.Success(c, tree)
}

// @Summary Add role
// @Description Create a system role
// @Tags SystemManage
// @Accept json
// @Produce json
// @Param body body service.CreateRoleRequest true "Role details"
// @Success 201 {object} response.Response{data=service.RoleManageResponse}
// @Router /systemManage/addRole [post]
// @Security BearerAuth
func (h *SystemManageHandler) AddRole(c *gin.Context) {
	var req service.CreateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	result, err := h.svc.CreateRole(&req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, result)
}

// @Summary Update role
// @Description Update a system role
// @Tags SystemManage
// @Accept json
// @Produce json
// @Param body body service.UpdateRoleRequest true "Role details"
// @Success 200 {object} response.Response{data=service.RoleManageResponse}
// @Router /systemManage/updateRole [post]
// @Security BearerAuth
func (h *SystemManageHandler) UpdateRole(c *gin.Context) {
	var req service.UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	result, err := h.svc.UpdateRole(&req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
}

// @Summary Delete role
// @Description Delete a system role
// @Tags SystemManage
// @Produce json
// @Param id query string false "Role ID"
// @Success 200 {object} response.Response
// @Failure 409 {object} response.Response "còn dữ liệu phụ thuộc"
// @Router /systemManage/deleteRole [delete]
// @Security BearerAuth
func (h *SystemManageHandler) DeleteRole(c *gin.Context) {
	var body struct {
		ID string `json:"id" form:"id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		if err := c.ShouldBindQuery(&body); err != nil {
			response.BadRequest(c, "id is required")
			return
		}
	}
	if body.ID == "" {
		response.BadRequest(c, "id is required")
		return
	}
	if err := h.svc.DeleteRole(body.ID); err != nil {
		handleDeleteError(c, err)
		return
	}
	response.Success(c, nil)
}

// @Summary Batch delete roles
// @Description Delete multiple system roles
// @Tags SystemManage
// @Accept json
// @Produce json
// @Param body body object true "Role IDs"
// @Success 200 {object} response.Response
// @Failure 409 {object} response.Response "còn dữ liệu phụ thuộc"
// @Router /systemManage/batchDeleteRole [delete]
// @Security BearerAuth
func (h *SystemManageHandler) BatchDeleteRole(c *gin.Context) {
	var body struct {
		IDs []string `json:"ids"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		handleValidationError(c, err)
		return
	}
	if err := h.svc.BatchDeleteRoles(body.IDs); err != nil {
		handleDeleteError(c, err)
		return
	}
	response.Success(c, nil)
}

// @Summary Get all permissions
// @Description List every permission code the system knows about
// @Tags SystemManage
// @Produce json
// @Success 200 {object} response.Response{data=[]service.PermissionResponse}
// @Router /systemManage/getAllPermissions [get]
// @Security BearerAuth
func (h *SystemManageHandler) GetAllPermissions(c *gin.Context) {
	result, err := h.svc.GetAllPermissions()
	if err != nil {
		response.InternalError(c, "Failed to fetch permissions")
		return
	}
	response.Success(c, result)
}

// @Summary Get a role's permissions
// @Description List the permission IDs currently granted to a role
// @Tags SystemManage
// @Produce json
// @Param roleId query string true "Role ID"
// @Success 200 {object} response.Response{data=[]string}
// @Router /systemManage/getRolePermissions [get]
// @Security BearerAuth
func (h *SystemManageHandler) GetRolePermissions(c *gin.Context) {
	roleID := c.Query("roleId")
	if roleID == "" {
		response.BadRequest(c, "roleId is required")
		return
	}
	result, err := h.svc.GetRolePermissions(roleID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
}

// @Summary Replace a role's permissions
// @Description Set the full permission list for a role
// @Tags SystemManage
// @Accept json
// @Produce json
// @Param body body object true "Role ID and permission IDs"
// @Success 200 {object} response.Response
// @Router /systemManage/updateRolePermissions [post]
// @Security BearerAuth
func (h *SystemManageHandler) UpdateRolePermissions(c *gin.Context) {
	var body struct {
		RoleID        string   `json:"roleId"`
		PermissionIDs []string `json:"permissionIds"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		handleValidationError(c, err)
		return
	}
	if body.RoleID == "" {
		response.BadRequest(c, "roleId is required")
		return
	}
	if err := h.svc.UpdateRolePermissions(body.RoleID, body.PermissionIDs); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}
