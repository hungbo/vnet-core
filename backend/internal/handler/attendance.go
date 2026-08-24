package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/middleware"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/response"
)

type AttendanceHandler struct {
	svc *service.AttendanceService
}

func NewAttendanceHandler(svc *service.AttendanceService) *AttendanceHandler {
	return &AttendanceHandler{svc: svc}
}

type checkinRequest struct {
	MemberID string `json:"member_id"`
}

// Checkin
// @Summary      Điểm danh hằng ngày
// @Description  Hội viên tự điểm danh cho chính mình; nhân viên điểm danh hộ thì truyền member_id.
// @Tags         Attendance
// @Accept       json
// @Produce      json
// @Success      200  {object}  response.Response
// @Failure      409  {object}  response.Response  "hôm nay đã điểm danh"
// @Router       /api/attendance/checkin [post]
func (h *AttendanceHandler) Checkin(c *gin.Context) {
	var req checkinRequest
	_ = c.ShouldBindJSON(&req)

	// Hội viên chỉ điểm danh được cho chính mình.
	memberID := req.MemberID
	if middleware.GetKind(c) == "member" {
		memberID = middleware.GetUserID(c)
	}

	res, err := h.svc.Checkin(memberID, middleware.GetUserID(c))
	switch {
	case err == nil:
		response.Success(c, res)
	case errors.Is(err, service.ErrAlreadyCheckedIn):
		// 409 chứ không phải 400: yêu cầu hợp lệ, chỉ là đã làm rồi. Giao diện
		// phân biệt được để hiện "đã điểm danh" thay vì báo lỗi đỏ.
		response.Error(c, http.StatusConflict, err.Error())
	default:
		response.BadRequest(c, err.Error())
	}
}

// Status
// @Summary      Trạng thái điểm danh hôm nay và chuỗi ngày
// @Tags         Attendance
// @Produce      json
// @Success      200  {object}  response.Response
// @Router       /api/attendance/status [get]
func (h *AttendanceHandler) Status(c *gin.Context) {
	memberID := c.Query("member_id")
	if middleware.GetKind(c) == "member" {
		memberID = middleware.GetUserID(c)
	}
	if memberID == "" {
		response.BadRequest(c, "thiếu mã hội viên")
		return
	}
	res, err := h.svc.Status(memberID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, res)
}

// List
// @Summary      Danh sách lượt điểm danh
// @Tags         Attendance
// @Produce      json
// @Success      200  {object}  response.Response
// @Router       /api/attendance [get]
func (h *AttendanceHandler) List(c *gin.Context) {
	var req service.AttendanceListRequest
	_ = c.ShouldBindQuery(&req)
	res, err := h.svc.List(&req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Paginated(c, res.Items, res.Total, res.Page, res.PageSize)
}
