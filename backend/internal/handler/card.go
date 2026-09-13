package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/middleware"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/response"
)

type CardHandler struct {
	svc *service.CardService
}

func NewCardHandler(svc *service.CardService) *CardHandler {
	return &CardHandler{svc: svc}
}

// GenerateTopupCards
// @Summary      Sinh lô thẻ nạp
// @Description  Mã bí mật CHỈ trả về ở đây, một lần duy nhất. Lưu hoặc in ngay.
// @Tags         Cards
// @Accept       json
// @Produce      json
// @Param        body  body  service.GenerateTopupCardsRequest  true  "Số lượng và mệnh giá"
// @Success      200  {object}  response.Response
// @Router       /api/topup-cards/generate [post]
func (h *CardHandler) GenerateTopupCards(c *gin.Context) {
	var req service.GenerateTopupCardsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	cards, err := h.svc.GenerateTopupCards(&req, middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"cards": cards})
}

// ListTopupCards
// @Summary      Danh sách thẻ nạp (không kèm mã bí mật)
// @Tags         Cards
// @Produce      json
// @Success      200  {object}  response.Response
// @Router       /api/topup-cards [get]
func (h *CardHandler) ListTopupCards(c *gin.Context) {
	var req service.CardListRequest
	_ = c.ShouldBindQuery(&req)
	res, err := h.svc.ListTopupCards(&req)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Paginated(c, res.Items, res.Total, res.Page, res.PageSize)
}

// RedeemTopupCard
// @Summary      Nạp thẻ vào số dư hội viên
// @Tags         Cards
// @Accept       json
// @Produce      json
// @Param        body  body  service.RedeemTopupCardRequest  true  "Seri và mã"
// @Success      200  {object}  response.Response
// @Failure      400  {object}  response.Response
// @Router       /api/topup-cards/redeem [post]
func (h *CardHandler) RedeemTopupCard(c *gin.Context) {
	var req service.RedeemTopupCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	// Hội viên tự nạp thì chỉ nạp được cho chính mình; nhân viên nạp hộ thì
	// truyền member_id. Không cho hội viên nạp thẻ vào tài khoản người khác.
	if middleware.GetKind(c) == "member" {
		req.MemberID = middleware.GetUserID(c)
	}
	res, err := h.svc.RedeemTopupCard(&req, middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, res)
}

// CancelTopupCard
// @Summary      Huỷ thẻ nạp chưa dùng
// @Tags         Cards
// @Produce      json
// @Param        id  path  string  true  "Card ID"
// @Success      200  {object}  response.Response
// @Router       /api/topup-cards/{id}/cancel [post]
func (h *CardHandler) CancelTopupCard(c *gin.Context) {
	if err := h.svc.CancelCard("topup", c.Param("id"), middleware.GetUserID(c)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// SellTopupCard
// @Summary      Bán thẻ nạp ở quầy
// @Description  Ghi lại thẻ này bán cho hội viên nào và lúc nào. Khác với nạp
// @Description  thẻ: người mua và người nạp có thể là hai người.
// @Tags         Cards
// @Accept       json
// @Produce      json
// @Param        id    path  string                          true  "Card ID"
// @Param        body  body  service.SellTopupCardRequest    true  "Hội viên mua"
// @Success      200  {object}  response.Response{data=model.TopupCard}
// @Failure      400  {object}  response.Response
// @Router       /api/topup-cards/{id}/sell [post]
// @Security     BearerAuth
func (h *CardHandler) SellTopupCard(c *gin.Context) {
	var req service.SellTopupCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	card, err := h.svc.SellTopupCard(c.Param("id"), req.MemberID, req.PaymentMethod, middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, card)
}

// GenerateGiftCards
// @Summary      Sinh lô thẻ quà tặng
// @Description  Mã bí mật CHỈ trả về ở đây, một lần duy nhất.
// @Tags         Cards
// @Accept       json
// @Produce      json
// @Param        body  body  service.GenerateGiftCardsRequest  true  "Số lượng và giá trị"
// @Success      200  {object}  response.Response
// @Router       /api/gift-cards/generate [post]
func (h *CardHandler) GenerateGiftCards(c *gin.Context) {
	var req service.GenerateGiftCardsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	cards, err := h.svc.GenerateGiftCards(&req, middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"cards": cards})
}

// ListGiftCards
// @Summary      Danh sách thẻ quà tặng (không kèm mã bí mật)
// @Tags         Cards
// @Produce      json
// @Success      200  {object}  response.Response
// @Router       /api/gift-cards [get]
func (h *CardHandler) ListGiftCards(c *gin.Context) {
	var req service.CardListRequest
	_ = c.ShouldBindQuery(&req)
	res, err := h.svc.ListGiftCards(&req)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Paginated(c, res.Items, res.Total, res.Page, res.PageSize)
}

type checkGiftCardRequest struct {
	Serial string `json:"serial" binding:"required"`
	Secret string `json:"secret" binding:"required"`
}

// CheckGiftCard
// @Summary      Tra số dư thẻ quà tặng
// @Tags         Cards
// @Accept       json
// @Produce      json
// @Success      200  {object}  response.Response
// @Router       /api/gift-cards/check [post]
func (h *CardHandler) CheckGiftCard(c *gin.Context) {
	var req checkGiftCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	res, err := h.svc.CheckGiftCard(req.Serial, req.Secret)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, res)
}

// CancelGiftCard
// @Summary      Huỷ thẻ quà tặng chưa dùng
// @Tags         Cards
// @Produce      json
// @Param        id  path  string  true  "Card ID"
// @Success      200  {object}  response.Response
// @Router       /api/gift-cards/{id}/cancel [post]
func (h *CardHandler) CancelGiftCard(c *gin.Context) {
	if err := h.svc.CancelCard("gift", c.Param("id"), middleware.GetUserID(c)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}
