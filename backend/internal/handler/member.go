package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/middleware"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/pagination"
	"github.com/vnet/core/pkg/response"
)

type MemberHandler struct {
	svc *service.MemberService
}

func NewMemberHandler(svc *service.MemberService) *MemberHandler {
	return &MemberHandler{svc: svc}
}

// @Summary List members
// @Description Get a paginated list of members
// @Tags Members
// @Accept json
// @Produce json
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Param sort query string false "Sort field"
// @Param order query string false "Sort order (asc/desc)"
// @Param search query string false "Search keyword"
// @Success 200 {object} response.Response{data=response.PaginatedData{data=[]service.MemberResponse}}
// @Failure 500 {object} response.Response
// @Router /members [get]
// @Security BearerAuth
func (h *MemberHandler) List(c *gin.Context) {
	params := pagination.GetParams(c)
	storeID := middleware.GetStoreID(c)
	result, total, page, pageSize, err := h.svc.List(*params, storeID)
	if err != nil {
		response.InternalError(c, "Failed to fetch members")
		return
	}
	response.Paginated(c, result, total, page, pageSize)
}

// @Summary Get member by ID
// @Description Get a single member by their ID
// @Tags Members
// @Accept json
// @Produce json
// @Param id path string true "Member ID"
// @Success 200 {object} response.Response{data=service.MemberResponse}
// @Failure 404 {object} response.Response
// @Router /members/{id} [get]
// @Security BearerAuth
func (h *MemberHandler) GetByID(c *gin.Context) {
	id := c.Param("id")
	member, err := h.svc.GetByID(id)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.Success(c, member)
}

// @Summary Create a new member
// @Description Create a new member record
// @Tags Members
// @Accept json
// @Produce json
// @Param body body service.CreateMemberRequest true "Member details"
// @Success 201 {object} response.Response{data=service.MemberResponse}
// @Failure 400 {object} response.Response
// @Router /members [post]
// @Security BearerAuth
func (h *MemberHandler) Create(c *gin.Context) {
	var req service.CreateMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	if req.StoreID == "" {
		req.StoreID = middleware.GetStoreID(c)
	}
	result, err := h.svc.Create(&req)
	if err != nil {
		handleCreateError(c, err)
		return
	}
	response.Created(c, result)
}

// @Summary Update a member
// @Description Update an existing member's details
// @Tags Members
// @Accept json
// @Produce json
// @Param id path string true "Member ID"
// @Param body body service.UpdateMemberRequest true "Updated member details"
// @Success 200 {object} response.Response{data=service.MemberResponse}
// @Failure 400 {object} response.Response
// @Router /members/{id} [put]
// @Security BearerAuth
func (h *MemberHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var req service.UpdateMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	result, err := h.svc.Update(id, &req)
	if err != nil {
		handleCreateError(c, err)
		return
	}
	response.Success(c, result)
}

// @Summary Delete a member
// @Description Delete a member by ID
// @Tags Members
// @Accept json
// @Produce json
// @Param id path string true "Member ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Router /members/{id} [delete]
// @Security BearerAuth
func (h *MemberHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.Delete(id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// @Summary Reset member password
// @Description Reset a member's password to a random new one
// @Tags Members
// @Accept json
// @Produce json
// @Param id path string true "Member ID"
// @Success 200 {object} response.Response{data=service.ResetPasswordResponse}
// @Failure 400 {object} response.Response
// @Router /members/{id}/reset-password [post]
// @Security BearerAuth
func (h *MemberHandler) ResetPassword(c *gin.Context) {
	id := c.Param("id")
	result, err := h.svc.ResetPassword(id)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
}

// @Summary Topup member balance
// @Description Add funds to a member's account
// @Tags Members
// @Accept json
// @Produce json
// @Param id path string true "Member ID"
// @Param body body service.TopupRequest true "Topup details"
// @Success 200 {object} response.Response{data=service.MemberResponse}
// @Failure 400 {object} response.Response
// @Router /members/{id}/topup [post]
// @Security BearerAuth
func (h *MemberHandler) Topup(c *gin.Context) {
	id := c.Param("id")
	var req service.TopupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	userID := middleware.GetUserID(c)
	storeID := middleware.GetStoreID(c)
	result, err := h.svc.Topup(id, &req, userID, storeID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
}

// @Summary Refund member balance
// @Description Refund funds from a member's account
// @Tags Members
// @Accept json
// @Produce json
// @Param id path string true "Member ID"
// @Param body body service.RefundRequest true "Refund details"
// @Success 200 {object} response.Response{data=service.MemberResponse}
// @Failure 400 {object} response.Response
// @Router /members/{id}/refund [post]
// @Security BearerAuth
func (h *MemberHandler) Refund(c *gin.Context) {
	id := c.Param("id")
	var req service.RefundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	userID := middleware.GetUserID(c)
	storeID := middleware.GetStoreID(c)
	result, err := h.svc.Refund(id, &req, userID, storeID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
}

// @Summary Get member transactions
// @Description Get a paginated list of transactions for a member
// @Tags Members
// @Accept json
// @Produce json
// @Param id path string true "Member ID"
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Param sort query string false "Sort field"
// @Param order query string false "Sort order (asc/desc)"
// @Success 200 {object} response.Response{data=response.PaginatedData{data=[]service.MemberTransactionResponse}}
// @Failure 500 {object} response.Response
// @Router /members/{id}/transactions [get]
// @Security BearerAuth
func (h *MemberHandler) GetTransactions(c *gin.Context) {
	id := c.Param("id")
	params := pagination.GetParams(c)
	result, total, page, pageSize, err := h.svc.GetTransactions(id, *params)
	if err != nil {
		response.InternalError(c, "Failed to fetch transactions")
		return
	}
	response.Paginated(c, result, total, page, pageSize)
}

// @Summary Get member sessions
// @Description Get a paginated list of play sessions for a member
// @Tags Members
// @Accept json
// @Produce json
// @Param id path string true "Member ID"
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Param sort query string false "Sort field"
// @Param order query string false "Sort order (asc/desc)"
// @Success 200 {object} response.Response{data=response.PaginatedData{data=[]service.SessionResponse}}
// @Failure 500 {object} response.Response
// @Router /members/{id}/sessions [get]
// @Security BearerAuth
func (h *MemberHandler) GetSessions(c *gin.Context) {
	id := c.Param("id")
	params := pagination.GetParams(c)
	result, total, page, pageSize, err := h.svc.GetSessions(id, *params)
	if err != nil {
		response.InternalError(c, "Failed to fetch sessions")
		return
	}
	response.Paginated(c, result, total, page, pageSize)
}

// @Summary Get member combo purchases
// @Description Get a paginated list of combo purchases for a member
// @Tags Members
// @Accept json
// @Produce json
// @Param id path string true "Member ID"
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Param sort query string false "Sort field"
// @Param order query string false "Sort order (asc/desc)"
// @Success 200 {object} response.Response{data=response.PaginatedData{data=[]service.ComboPurchaseResponse}}
// @Failure 500 {object} response.Response
// @Router /members/{id}/combos [get]
// @Security BearerAuth
func (h *MemberHandler) GetCombos(c *gin.Context) {
	id := c.Param("id")
	params := pagination.GetParams(c)
	result, total, page, pageSize, err := h.svc.GetCombos(id, *params)
	if err != nil {
		response.InternalError(c, "Failed to fetch combos")
		return
	}
	response.Paginated(c, result, total, page, pageSize)
}

// @Summary List member groups
// @Description Get all available member groups
// @Tags MemberGroups
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=[]service.GroupResponse}
// @Failure 500 {object} response.Response
// @Router /member-groups [get]
// @Security BearerAuth
func (h *MemberHandler) ListGroups(c *gin.Context) {
	groups, err := h.svc.GetGroups()
	if err != nil {
		response.InternalError(c, "Failed to fetch groups")
		return
	}
	response.Success(c, groups)
}

// @Summary Create a member group
// @Description Create a new member group
// @Tags MemberGroups
// @Accept json
// @Produce json
// @Param body body service.CreateGroupRequest true "Group details"
// @Success 201 {object} response.Response{data=service.GroupResponse}
// @Failure 400 {object} response.Response
// @Router /member-groups [post]
// @Security BearerAuth
func (h *MemberHandler) CreateGroup(c *gin.Context) {
	var req service.CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	result, err := h.svc.CreateGroup(&req)
	if err != nil {
		handleCreateError(c, err)
		return
	}
	response.Created(c, result)
}

// @Summary Update a member group
// @Description Update an existing member group
// @Tags MemberGroups
// @Accept json
// @Produce json
// @Param id path string true "Group ID"
// @Param body body service.UpdateGroupRequest true "Updated group details"
// @Success 200 {object} response.Response{data=service.GroupResponse}
// @Failure 400 {object} response.Response
// @Router /member-groups/{id} [put]
// @Security BearerAuth
func (h *MemberHandler) UpdateGroup(c *gin.Context) {
	id := c.Param("id")
	var req service.UpdateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	result, err := h.svc.UpdateGroup(id, &req)
	if err != nil {
		handleCreateError(c, err)
		return
	}
	response.Success(c, result)
}

// @Summary Delete a member group
// @Description Delete a member group by ID
// @Tags MemberGroups
// @Accept json
// @Produce json
// @Param id path string true "Group ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Router /member-groups/{id} [delete]
// @Security BearerAuth
func (h *MemberHandler) DeleteGroup(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.DeleteGroup(id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}
