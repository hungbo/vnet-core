package handler

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/middleware"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/response"
)

type FeedbackHandler struct {
	svc *service.FeedbackService
}

func NewFeedbackHandler(svc *service.FeedbackService) *FeedbackHandler {
	return &FeedbackHandler{svc: svc}
}

// Create
// @Summary      Gửi đánh giá dịch vụ
// @Description  Hội viên tự gửi: máy được suy ra từ phiên đang chơi, không lấy từ request.
// @Tags         Feedback
// @Accept       json
// @Produce      json
// @Param        body  body  service.CreateFeedbackRequest  true  "Điểm và nhận xét"
// @Success      201  {object}  response.Response
// @Failure      409  {object}  response.Response  "đơn đã được đánh giá"
// @Router       /api/feedback [post]
func (h *FeedbackHandler) Create(c *gin.Context) {
	var req service.CreateFeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	// Chỉ hội viên mới có phiên chơi để suy ra máy. Nhân viên gửi hộ thì phải
	// chỉ định máy trong request.
	memberID := ""
	if middleware.GetKind(c) == "member" {
		memberID = middleware.GetUserID(c)
	}

	res, err := h.svc.Create(&req, memberID)
	switch {
	case err == nil:
		response.Created(c, res)
	case errors.Is(err, service.ErrAlreadyRated):
		response.Conflict(c, err.Error())
	default:
		response.BadRequest(c, err.Error())
	}
}

// List
// @Summary      Danh sách đánh giá
// @Tags         Feedback
// @Produce      json
// @Success      200  {object}  response.Response
// @Router       /api/feedback [get]
func (h *FeedbackHandler) List(c *gin.Context) {
	var req service.FeedbackListRequest
	_ = c.ShouldBindQuery(&req)
	res, err := h.svc.List(&req)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Paginated(c, res.Items, res.Total, res.Page, res.PageSize)
}

// Summary
// @Summary      Điểm trung bình và phân bố sao
// @Tags         Feedback
// @Produce      json
// @Success      200  {object}  response.Response
// @Router       /api/feedback/summary [get]
func (h *FeedbackHandler) Summary(c *gin.Context) {
	var req service.FeedbackListRequest
	_ = c.ShouldBindQuery(&req)
	res, err := h.svc.Summary(&req)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, res)
}
