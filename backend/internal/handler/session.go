package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/middleware"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/response"
)

type SessionHandler struct {
	svc *service.SessionService
}

func NewSessionHandler(svc *service.SessionService) *SessionHandler {
	return &SessionHandler{svc: svc}
}

// @Summary List active sessions
// @Description Get all currently active play sessions
// @Tags Sessions
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=[]service.SessionDetail}
// @Failure 500 {object} response.Response
// @Router /sessions/active [get]
// @Security BearerAuth
func (h *SessionHandler) ListActive(c *gin.Context) {
	sessions, err := h.svc.GetActiveSessions()
	if err != nil {
		response.InternalError(c, "Failed to fetch active sessions")
		return
	}
	response.Success(c, sessions)
}

// @Summary Start a session
// @Description Start a new play session for a member on a machine
// @Tags Sessions
// @Accept json
// @Produce json
// @Param body body service.StartRequest true "Session start details"
// @Success 201 {object} response.Response{data=service.SessionDetail}
// @Failure 400 {object} response.Response
// @Router /sessions/start [post]
// @Security BearerAuth
func (h *SessionHandler) Start(c *gin.Context) {
	var req service.StartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	result, err := h.svc.StartSession(&req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Created(c, result)
}

// @Summary End a session
// @Description End an active play session and calculate charges
// @Tags Sessions
// @Accept json
// @Produce json
// @Param id path string true "Session ID"
// @Success 200 {object} response.Response{data=service.EndSessionResponse}
// @Failure 400 {object} response.Response
// @Router /sessions/{id}/end [post]
// @Security BearerAuth
func (h *SessionHandler) End(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.BadRequest(c, "Session ID is required")
		return
	}

	result, err := h.svc.EndSession(id)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, result)
}

// @Summary Get session by ID
// @Description Get details of a specific session
// @Tags Sessions
// @Accept json
// @Produce json
// @Param id path string true "Session ID"
// @Success 200 {object} response.Response{data=service.SessionDetail}
// @Failure 404 {object} response.Response
// @Router /sessions/{id} [get]
// @Security BearerAuth
func (h *SessionHandler) Get(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.BadRequest(c, "Session ID is required")
		return
	}

	result, err := h.svc.GetSession(id)
	if err != nil {
		response.NotFound(c, "Session not found")
		return
	}

	response.Success(c, result)
}

func (h *SessionHandler) GetMySession(c *gin.Context) {
	memberID := middleware.GetUserID(c)
	if memberID == "" {
		response.BadRequest(c, "User not authenticated")
		return
	}

	session, err := h.svc.GetActiveSessionByMember(memberID)
	if err != nil {
		response.InternalError(c, "Failed to fetch session")
		return
	}

	if session == nil {
		response.Success(c, nil)
		return
	}

	response.Success(c, session)
}

// @Summary Switch machine for a session
// @Description Move an active session to a different machine
// @Tags Sessions
// @Accept json
// @Produce json
// @Param id path string true "Session ID"
// @Param body body service.SwitchMachineRequest true "New machine details"
// @Success 200 {object} response.Response{data=service.SessionDetail}
// @Failure 400 {object} response.Response
// @Router /sessions/{id}/switch-machine [post]
// @Security BearerAuth
func (h *SessionHandler) SwitchMachine(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		response.BadRequest(c, "Session ID is required")
		return
	}

	var req service.SwitchMachineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	result, err := h.svc.SwitchMachine(sessionID, req.NewMachineID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, result)
}

// @Summary Calculate session cost
// @Preview Calculate the estimated cost for a session duration on a machine
// @Tags Sessions
// @Accept json
// @Produce json
// @Param machine_id query string true "Machine ID"
// @Param member_id query string false "Member ID (for discount calculation)"
// @Param duration_minutes query int true "Session duration in minutes"
// @Success 200 {object} response.Response{data=service.CostBreakdown}
// @Failure 400 {object} response.Response
// @Router /sessions/calculate-cost [get]
// @Security BearerAuth
func (h *SessionHandler) CalculateCost(c *gin.Context) {
	machineID := c.Query("machine_id")
	memberID := c.Query("member_id")
	durationStr := c.Query("duration_minutes")

	if machineID == "" || durationStr == "" {
		response.BadRequest(c, "machine_id and duration_minutes are required")
		return
	}

	duration := 0
	for _, ch := range durationStr {
		if ch >= '0' && ch <= '9' {
			duration = duration*10 + int(ch-'0')
		} else {
			break
		}
	}

	if duration <= 0 {
		response.BadRequest(c, "duration_minutes must be a positive number")
		return
	}

	result, err := h.svc.CalculateCost(machineID, memberID, duration)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, result)
}
