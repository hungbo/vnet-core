package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/middleware"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/response"
)

type ReceiptHandler struct {
	svc *service.ReceiptService
}

func NewReceiptHandler(svc *service.ReceiptService) *ReceiptHandler {
	return &ReceiptHandler{svc: svc}
}

type printOrderRequest struct {
	PrinterID string `json:"printer_id"`
}

// PrintReceipt
// @Summary      In hoá đơn cho khách
// @Description  Không truyền printer_id thì dùng máy in mặc định.
// @Tags         Orders
// @Accept       json
// @Produce      json
// @Param        id    path  string  true   "Order ID"
// @Param        body  body  object  false  "printer_id"
// @Success      200  {object}  response.Response
// @Failure      400  {object}  response.Response
// @Failure      502  {object}  response.Response  "không in được"
// @Router       /api/orders/{id}/print [post]
func (h *ReceiptHandler) PrintReceipt(c *gin.Context) {
	var req printOrderRequest
	_ = c.ShouldBindJSON(&req)

	res, err := h.svc.PrintOrder(c.Param("id"), req.PrinterID, middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	respondPrint(c, res)
}

// PrintStations
// @Summary      In phiếu chế biến theo bảng phân luồng máy in
// @Tags         Orders
// @Produce      json
// @Param        id  path  string  true  "Order ID"
// @Success      200  {object}  response.Response
// @Failure      400  {object}  response.Response
// @Failure      502  {object}  response.Response  "không in được"
// @Router       /api/orders/{id}/print-stations [post]
func (h *ReceiptHandler) PrintStations(c *gin.Context) {
	res, err := h.svc.PrintStations(c.Param("id"), middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	respondPrint(c, res)
}

// PreviewReceipt
// @Summary      Xem trước hoá đơn dạng chữ, không gửi xuống máy in
// @Tags         Orders
// @Produce      json
// @Param        id  path  string  true  "Order ID"
// @Success      200  {object}  response.Response
// @Router       /api/orders/{id}/receipt-preview [get]
func (h *ReceiptHandler) PreviewReceipt(c *gin.Context) {
	text, err := h.svc.PreviewOrder(c.Param("id"), c.Query("printer_id"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"text": text})
}

// respondPrint trả 502 khi có máy in nào không nhận được. Lệnh hợp lệ nhưng
// giấy không ra thì không thể báo thành công — đúng bài học từ lệnh điều khiển
// từ xa trước đây.
func respondPrint(c *gin.Context, res *service.PrintResult) {
	if res.Failed() {
		c.JSON(http.StatusBadGateway, gin.H{
			"code":    http.StatusBadGateway,
			"message": "có máy in không nhận được dữ liệu",
			"data":    res,
		})
		return
	}
	response.Success(c, res)
}
