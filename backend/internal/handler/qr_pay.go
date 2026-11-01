package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/vnet/core/internal/middleware"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/jwt"
	"github.com/vnet/core/pkg/pagination"
	"github.com/vnet/core/pkg/response"
)

type QRPayHandler struct {
	svc *service.QRPayService
}

func NewQRPayHandler(svc *service.QRPayService) *QRPayHandler {
	return &QRPayHandler{svc: svc}
}

// Service cho main.go chạy việc quét mã hết hạn mỗi phút.
func (h *QRPayHandler) Service() *service.QRPayService { return h.svc }

// @Summary QR top-up status
// @Description Nạp QR có dùng được không: đã bật, đã cài tài khoản, điện thoại báo tiền còn kết nối
// @Tags QR
// @Produce json
// @Success 200 {object} response.Response{data=service.QRStatus}
// @Router /api/qr/status [get]
// @Security BearerAuth
func (h *QRPayHandler) Status(c *gin.Context) {
	response.Success(c, h.svc.Status())
}

// @Summary Create a QR top-up for yourself
// @Description Hội viên (máy trạm) xin nạp bằng QR: tạo đơn nạp chờ + mã VietQR
// @Tags QR
// @Accept json
// @Produce json
// @Param body body service.CreateQRTopupRequest true "amount, machine_code (member_id bỏ qua)"
// @Success 201 {object} response.Response{data=service.QRTopupResponse}
// @Failure 400 {object} response.Response
// @Router /api/qr/topup-request [post]
// @Security BearerAuth
func (h *QRPayHandler) CreateForSelf(c *gin.Context) {
	if middleware.GetKind(c) != jwt.KindMember {
		response.Forbidden(c, "Chỉ hội viên tự nạp được")
		return
	}
	var req service.CreateQRTopupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	req.MemberID = middleware.GetUserID(c)
	h.create(c, req)
}

// @Summary Create a QR top-up for a member (counter)
// @Description Quầy tạo mã QR nạp tiền cho một hội viên
// @Tags QR
// @Accept json
// @Produce json
// @Param body body service.CreateQRTopupRequest true "member_id, amount"
// @Success 201 {object} response.Response{data=service.QRTopupResponse}
// @Failure 400 {object} response.Response
// @Router /api/qr/topups [post]
// @Security BearerAuth
func (h *QRPayHandler) CreateForMember(c *gin.Context) {
	var req service.CreateQRTopupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	h.create(c, req)
}

func (h *QRPayHandler) create(c *gin.Context, req service.CreateQRTopupRequest) {
	res, err := h.svc.CreateTopup(req, middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, res)
}

// @Summary Pay a confirmed order by QR (member)
// @Description Hội viên trả bằng QR một đơn món của chính mình đã xác nhận mà chưa thanh toán. Số tiền lấy từ đơn; đơn đang có mã còn hạn thì trả lại mã đó. Huỷ hay hết hạn mã không huỷ đơn
// @Tags QR
// @Produce json
// @Param id path string true "Order ID"
// @Success 200 {object} response.Response{data=service.QRTopupResponse}
// @Failure 400 {object} response.Response
// @Router /api/qr/orders/{id}/pay [post]
// @Security BearerAuth
func (h *QRPayHandler) PayOrder(c *gin.Context) {
	if middleware.GetKind(c) != jwt.KindMember {
		response.Forbidden(c, "Chỉ hội viên tự trả được")
		return
	}
	res, err := h.svc.PayOrder(c.Param("id"), middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, res)
}

// ownRequest: hội viên chỉ xem/huỷ được mã của chính mình.
func (h *QRPayHandler) ownRequest(c *gin.Context) (string, bool) {
	r, err := h.svc.GetRequest(c.Param("id"))
	if err != nil {
		response.NotFound(c, err.Error())
		return "", false
	}
	if middleware.GetKind(c) != jwt.KindStaff && (r.MemberID == nil || *r.MemberID != middleware.GetUserID(c)) {
		response.Forbidden(c, "Access denied")
		return "", false
	}
	return r.ID, true
}

// @Summary QR top-up request status
// @Tags QR
// @Produce json
// @Param id path string true "Request ID"
// @Success 200 {object} response.Response{data=model.QRRequest}
// @Router /api/qr/requests/{id} [get]
// @Security BearerAuth
func (h *QRPayHandler) GetRequest(c *gin.Context) {
	id, ok := h.ownRequest(c)
	if !ok {
		return
	}
	r, _ := h.svc.GetRequest(id)
	response.Success(c, r)
}

// @Summary Cancel a QR top-up request
// @Description Khách đóng hộp QR trước khi chuyển: huỷ mã và đơn nạp
// @Tags QR
// @Produce json
// @Param id path string true "Request ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Router /api/qr/requests/{id}/cancel [post]
// @Security BearerAuth
func (h *QRPayHandler) CancelRequest(c *gin.Context) {
	id, ok := h.ownRequest(c)
	if !ok {
		return
	}
	if err := h.svc.Cancel(id, middleware.GetUserID(c)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// @Summary Start pairing the counter phone
// @Description Hiện mã 6 số (sống 5 phút) để nhập vào app VNET Bank Notification
// @Tags QR
// @Produce json
// @Success 200 {object} response.Response
// @Router /api/qr/pairing [post]
// @Security BearerAuth
func (h *QRPayHandler) StartPairing(c *gin.Context) {
	code, expires, err := h.svc.StartPairing(middleware.GetUserID(c))
	if err != nil {
		response.InternalError(c, "Không tạo được mã ghép", err)
		return
	}
	response.Success(c, gin.H{"code": code, "expires_at": expires})
}

// @Summary Unpair the counter phone
// @Tags QR
// @Produce json
// @Success 200 {object} response.Response
// @Router /api/qr/device [delete]
// @Security BearerAuth
func (h *QRPayHandler) Unpair(c *gin.Context) {
	if err := h.svc.Unpair(middleware.GetUserID(c)); err != nil {
		response.InternalError(c, "Không gỡ được điện thoại", err)
		return
	}
	response.Success(c, nil)
}

// --- Điện thoại ở quầy (không có tài khoản: ký bằng khoá nhận lúc ghép) -------

type qrPairRequest struct {
	Code string `json:"code" binding:"required"`
}

// @Summary Pair the counter phone
// @Description Điện thoại gửi mã 6 số, nhận khoá ký các lần gửi sau
// @Tags QR
// @Accept json
// @Produce json
// @Param body body qrPairRequest true "Mã ghép"
// @Success 200 {object} response.Response{data=service.QRPairResponse}
// @Failure 400 {object} response.Response
// @Router /api/bank-notify/pair [post]
func (h *QRPayHandler) Pair(c *gin.Context) {
	var req qrPairRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	res, err := h.svc.Pair(req.Code)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, res)
}

// signedBody đọc thân yêu cầu và kiểm chữ ký của điện thoại.
func (h *QRPayHandler) signedBody(c *gin.Context, into interface{}) bool {
	body, err := c.GetRawData()
	if err != nil {
		response.BadRequest(c, "Không đọc được dữ liệu")
		return false
	}
	if err := h.svc.VerifyDevice(c.GetHeader("X-VNET-Timestamp"), c.GetHeader("X-VNET-Signature"), body); err != nil {
		if errors.Is(err, service.ErrQRBadSignature) {
			response.Unauthorized(c, err.Error())
		} else {
			response.Forbidden(c, err.Error())
		}
		return false
	}
	if err := json.Unmarshal(body, into); err != nil {
		response.BadRequest(c, "Dữ liệu không đúng dạng")
		return false
	}
	return true
}

// Điện thoại không phải trình duyệt: không có Origin để kiểm, xác thực bằng chữ ký.
var deviceUpgrader = websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

// @Summary Counter phone live connection
// @Description WebSocket giữ trạng thái online của điện thoại báo tiền. Bắt tay ký HMAC(khoá, timestamp + "\n" + "ws") qua X-VNET-Timestamp / X-VNET-Signature; sau đó điện thoại gửi trạng thái (như heartbeat) dạng JSON, máy chủ ping mỗi 10 giây
// @Tags QR
// @Router /api/bank-notify/ws [get]
func (h *QRPayHandler) WebSocket(c *gin.Context) {
	if err := h.svc.VerifyDevice(c.GetHeader("X-VNET-Timestamp"), c.GetHeader("X-VNET-Signature"), []byte("ws")); err != nil {
		if errors.Is(err, service.ErrQRBadSignature) {
			response.Unauthorized(c, err.Error())
		} else {
			response.Forbidden(c, err.Error())
		}
		return
	}
	conn, err := deviceUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	h.svc.ServeDevice(conn)
}

// @Summary Counter phone heartbeat
// @Description Mỗi 60 giây: điện thoại còn sống, hàng đợi, pin, quyền đọc thông báo
// @Tags QR
// @Accept json
// @Produce json
// @Param body body service.QRHeartbeat true "Trạng thái điện thoại"
// @Success 200 {object} response.Response{data=service.QRStatus}
// @Router /api/bank-notify/heartbeat [post]
func (h *QRPayHandler) Heartbeat(c *gin.Context) {
	var req service.QRHeartbeat
	if !h.signedBody(c, &req) {
		return
	}
	response.Success(c, h.svc.Heartbeat(req))
}

// @Summary Counter phone sends a bank notification
// @Description Nguyên văn một thông báo của app ngân hàng; khớp mã nạp thì duyệt đơn
// @Tags QR
// @Accept json
// @Produce json
// @Param body body service.BankNotifyRequest true "Thông báo"
// @Success 200 {object} response.Response{data=service.BankNotifyResult}
// @Router /api/bank-notify [post]
func (h *QRPayHandler) Notify(c *gin.Context) {
	var req service.BankNotifyRequest
	if !h.signedBody(c, &req) {
		return
	}
	res, err := h.svc.Ingest(req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, res)
}

// --- Trang "Tiền vào" ---------------------------------------------------------

// @Summary Bank notifications ("Tiền vào")
// @Tags QR
// @Produce json
// @Param date_from query string false "YYYY-MM-DD"
// @Param date_to query string false "YYYY-MM-DD"
// @Param status query string false "matched | suggested | amount_mismatch | unmatched | assigned | ignored | unparsed"
// @Param search query string false "Nội dung, mã nạp, hội viên"
// @Param page query int false "Trang"
// @Param page_size query int false "Số dòng"
// @Success 200 {object} response.Response{data=bankNotificationPage}
// @Router /api/bank-notifications [get]
// @Security BearerAuth
func (h *QRPayHandler) List(c *gin.Context) {
	var f service.BankNotificationFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		handleValidationError(c, err)
		return
	}
	params := pagination.GetParams(c)
	rows, total, totalIn, err := h.svc.List(f, *params)
	if err != nil {
		response.InternalError(c, "Không tải được danh sách tiền vào", err)
		return
	}
	response.Success(c, bankNotificationPage{
		PaginatedData: response.PaginatedData{Items: rows, Total: total, Page: params.Page, PageSize: params.PageSize},
		TotalIn:       totalIn,
	})
}

// bankNotificationPage: một trang như response.Paginated, kèm tổng tiền vào của
// mọi dòng khớp bộ lọc.
type bankNotificationPage struct {
	response.PaginatedData
	TotalIn int64 `json:"total_in"`
}

// @Summary QR codes created (log)
// @Description Nhật ký mã QR đã tạo: lúc tạo, lúc tiền vào, và mất bao lâu (duration_seconds)
// @Tags QR
// @Produce json
// @Param date_from query string false "YYYY-MM-DD (theo lúc tạo)"
// @Param date_to query string false "YYYY-MM-DD"
// @Param status query string false "pending | paid | expired | cancelled"
// @Param search query string false "Mã nạp, mã đơn, hội viên"
// @Param page query int false "Trang"
// @Param page_size query int false "Số dòng"
// @Success 200 {object} response.Response{data=pagination.Result}
// @Router /api/qr/requests [get]
// @Security BearerAuth
func (h *QRPayHandler) ListRequests(c *gin.Context) {
	var f service.QRRequestFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		handleValidationError(c, err)
		return
	}
	params := pagination.GetParams(c)
	rows, total, err := h.svc.ListRequests(f, *params)
	if err != nil {
		response.InternalError(c, "Không tải được nhật ký mã QR", err)
		return
	}
	response.Paginated(c, rows, total, params.Page, params.PageSize)
}

// @Summary Confirm a suggested bank notification
// @Tags QR
// @Produce json
// @Param id path string true "Notification ID"
// @Success 200 {object} response.Response
// @Router /api/bank-notifications/{id}/confirm [post]
// @Security BearerAuth
func (h *QRPayHandler) Confirm(c *gin.Context) {
	if err := h.svc.Confirm(c.Param("id"), middleware.GetUserID(c)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

type qrAssignRequest struct {
	MemberID string `json:"member_id" binding:"required"`
}

// @Summary Credit an unmatched bank notification to a member
// @Tags QR
// @Accept json
// @Produce json
// @Param id path string true "Notification ID"
// @Param body body qrAssignRequest true "Hội viên"
// @Success 200 {object} response.Response
// @Router /api/bank-notifications/{id}/assign [post]
// @Security BearerAuth
func (h *QRPayHandler) Assign(c *gin.Context) {
	var req qrAssignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	if err := h.svc.Assign(c.Param("id"), req.MemberID, middleware.GetUserID(c)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

type qrIgnoreRequest struct {
	Note string `json:"note"`
}

// @Summary Ignore a bank notification
// @Tags QR
// @Accept json
// @Produce json
// @Param id path string true "Notification ID"
// @Param body body qrIgnoreRequest false "Ghi chú"
// @Success 200 {object} response.Response
// @Router /api/bank-notifications/{id}/ignore [post]
// @Security BearerAuth
func (h *QRPayHandler) Ignore(c *gin.Context) {
	var req qrIgnoreRequest
	_ = c.ShouldBindJSON(&req)
	if err := h.svc.Ignore(c.Param("id"), middleware.GetUserID(c), req.Note); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}
