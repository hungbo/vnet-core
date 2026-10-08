package handler

import (
	"errors"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/hub"
	"github.com/vnet/core/internal/middleware"
	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/internal/remotedesk"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/pagination"
	"github.com/vnet/core/pkg/response"
)

type MachineHandler struct {
	svc *service.MachineService
	hub *hub.Hub
}

func NewMachineHandler(svc *service.MachineService) *MachineHandler {
	_ = model.Machine{}
	return &MachineHandler{svc: svc}
}

// WithHub gắn hub để nâng cấp WebSocket cho remote desktop.
func (h *MachineHandler) WithHub(wsHub *hub.Hub) *MachineHandler {
	h.hub = wsHub
	return h
}

// List
// @Summary      List Machines
// @Description  Get paginated list of machines for the current store
// @Tags         Machines
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page      query  int    false  "Page number"
// @Param        page_size  query  int    false  "Page size"
// @Param        sort      query  string false  "Sort field"
// @Param        order     query  string false  "Sort order (asc/desc)"
// @Param        search    query  string false  "Search keyword"
// @Param        warning  query  int  false  "1 = chỉ máy có cảnh báo (tài khoản Windows quản trị hoặc card mạng lạ)"
// @Success      200       {object}  response.Response{data=response.PaginatedData{data=[]model.Machine}}
// @Failure      500       {object}  response.Response
// @Router       /api/machines [get]
func (h *MachineHandler) List(c *gin.Context) {
	params := pagination.GetParams(c)
	result, err := h.svc.ListFiltered(*params, c.Query("warning") == "1")
	if err != nil {
		response.InternalError(c, "Failed to fetch machines")
		return
	}
	response.Paginated(c, result.Items, result.Total, result.Page, result.PageSize)
}

// GetByID
// @Summary      Get Machine by ID
// @Description  Get a single machine by its ID
// @Tags         Machines
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string  true  "Machine ID"
// @Success      200   {object}  response.Response{data=model.Machine}
// @Failure      404   {object}  response.Response
// @Router       /api/machines/{id} [get]
func (h *MachineHandler) GetByID(c *gin.Context) {
	id := c.Param("id")
	machine, err := h.svc.GetByID(id)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.Success(c, machine)
}

// Create
// @Summary      Create Machine
// @Description  Create a new machine
// @Tags         Machines
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  service.CreateMachineRequest  true  "Machine data"
// @Success      201   {object}  response.Response{data=model.Machine}
// @Failure      400   {object}  response.Response
// @Router       /api/machines [post]
func (h *MachineHandler) Create(c *gin.Context) {
	var req service.CreateMachineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	result, err := h.svc.Create(&req)
	if err != nil {
		handleCreateError(c, err)
		return
	}
	response.Created(c, result)
}

// BatchCreate
// @Summary      Batch create machines
// @Description  Tạo một dải máy theo tiền tố và khoảng số. Vướng một mã thì không tạo máy nào. Đặt dry_run để chỉ kiểm.
// @Tags         Machines
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      service.BatchCreateMachinesRequest  true  "Batch spec"
// @Success      200   {object}  response.Response{data=service.BatchCreateResult}  "dry_run"
// @Success      201   {object}  response.Response{data=service.BatchCreateResult}
// @Failure      400   {object}  response.Response
// @Failure      409   {object}  response.Response
// @Router       /api/machines/batch [post]
func (h *MachineHandler) BatchCreate(c *gin.Context) {
	var req service.BatchCreateMachinesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	result, err := h.svc.BatchCreateMachines(&req)
	if err != nil {
		handleCreateError(c, err)
		return
	}
	// Lần kiểm thử luôn là 200 kể cả khi vướng — giao diện cần đọc danh sách mã
	// vướng để vẽ ra, không phải để bắt lỗi.
	if result.DryRun {
		response.Success(c, result)
		return
	}
	if !result.OK() {
		response.Conflict(c, "không tạo máy nào — "+result.MoTaVuong())
		return
	}
	response.Created(c, result)
}

// Update
// @Summary      Update Machine
// @Description  Update an existing machine
// @Tags         Machines
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string                     true  "Machine ID"
// @Param        body  body  service.UpdateMachineRequest  true  "Machine update data"
// @Success      200   {object}  response.Response{data=model.Machine}
// @Failure      400   {object}  response.Response
// @Failure      404   {object}  response.Response
// @Router       /api/machines/{id} [put]
func (h *MachineHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var req service.UpdateMachineRequest
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

// Delete
// @Summary      Delete Machine
// @Description  Delete a machine by its ID
// @Tags         Machines
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string  true  "Machine ID"
// @Success      200   {object}  response.Response
// @Failure      400   {object}  response.Response
// @Failure      404   {object}  response.Response
// @Failure 409 {object} response.Response "còn dữ liệu phụ thuộc"
// @Router       /api/machines/{id} [delete]
func (h *MachineHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.Delete(id); err != nil {
		handleDeleteError(c, err)
		return
	}
	response.Success(c, nil)
}

// Heartbeat
// @Summary      Machine Heartbeat
// @Description  Send heartbeat from a machine with hardware telemetry
// @Tags         Machines
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string                   true  "Machine ID"
// @Param        body  body  service.HeartbeatRequest  true  "Heartbeat data"
// @Success      200   {object}  response.Response
// @Failure      400   {object}  response.Response
// @Router       /api/machines/{id}/heartbeat [post]
func (h *MachineHandler) Heartbeat(c *gin.Context) {
	id := c.Param("id")
	var req service.HeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	if err := h.svc.Heartbeat(id, req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *MachineHandler) HeartbeatByCode(c *gin.Context) {
	code := c.Param("code")

	// Route ghi dữ liệu nằm ngoài AuthRequired: máy trạm không có tài khoản
	// người. Nhận diện bằng mã máy trong URL — khoá máy trạm đã bỏ.
	var req service.HeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	// Máy chưa khai thì tự thêm: chỉ cần cài máy trạm với địa chỉ máy chủ,
	// mã máy là tên máy Windows.
	machine, _, err := h.svc.RegisterByCode(code)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.svc.Heartbeat(machine.ID, req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	// Chính sách bảo vệ đi kèm phản hồi nhịp tim: máy trạm nhận được cả khi
	// chưa ai đăng nhập, và không cần thêm một đường gọi riêng.
	// Mật khẩu TightVNC cũng đi kèm: máy trạm cấu hình VNC theo nó.
	vncPassword, err := h.svc.VNCPassword(machine.ID)
	if err != nil {
		log.Printf("[VNC] không lấy được mật khẩu VNC của máy %s: %v", code, err)
	}
	response.Success(c, gin.H{"policy": h.svc.ClientPolicy(), "vnc_password": vncPassword})
}

// GetHardware
// @Summary      Get Hardware History
// @Description  Get paginated hardware telemetry history for a machine
// @Tags         Machines
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id        path  string  true  "Machine ID"
// @Param        page      query  int    false  "Page number"
// @Param        page_size  query  int    false  "Page size"
// @Param        sort      query  string false  "Sort field"
// @Param        order     query  string false  "Sort order (asc/desc)"
// @Success      200       {object}  response.Response{data=response.PaginatedData{data=[]model.MachineHardwareSnapshot}}
// @Failure      500       {object}  response.Response
// @Router       /api/machines/{id}/hardware [get]
func (h *MachineHandler) GetHardware(c *gin.Context) {
	id := c.Param("id")
	params := pagination.GetParams(c)
	result, err := h.svc.GetHardwareHistory(id, *params)
	if err != nil {
		response.InternalError(c, "Failed to fetch hardware history")
		return
	}
	response.Paginated(c, result.Items, result.Total, result.Page, result.PageSize)
}

// GetByCode
// @Summary      Get Machine By Code
// @Description  Get a machine by its machine code
// @Tags         Machines
// @Param        code  path  string  true  "Machine Code"
// @Success      200  {object}  response.Response{data=model.Machine}
// @Router       /api/machines/by-code/{code} [get]
func (h *MachineHandler) GetByCode(c *gin.Context) {
	code := c.Param("code")
	machine, err := h.svc.GetByCode(code)
	if err != nil {
		response.NotFound(c, "Machine not found")
		return
	}
	response.Success(c, machine)
}

// RemoteAction
// @Summary      Remote Machine Action
// @Description  Perform a remote action on a machine (e.g. restart, shutdown)
// @Tags         Machines
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id      path  string  true  "Machine ID"
// @Param        action  path  string  true  "Lệnh: lock, unlock, shutdown, restart, message"
// @Success      200   {object}  response.Response
// @Failure      400   {object}  response.Response
// @Failure      409   {object}  response.Response  "máy trạm chưa kết nối"
// @Router       /api/machines/{id}/remote/{action} [post]
func (h *MachineHandler) RemoteAction(c *gin.Context) {
	id := c.Param("id")
	action := c.Param("action")

	var payload interface{}
	if err := c.ShouldBindJSON(&payload); err != nil {
		payload = nil
	}

	result, err := h.svc.RemoteAction(id, action, payload, middleware.GetUserID(c))
	switch {
	case err == nil:
		// Trả kèm kết quả chốt tiền khi lệnh làm kết thúc phiên (tắt máy, khởi
		// động lại), để nhân viên thấy ngay đã thu bao nhiêu.
		response.Success(c, result)
	case errors.Is(err, service.ErrMachineOffline):
		// 409 chứ không phải 400: lệnh hợp lệ, chỉ là máy chưa kết nối. Nhân
		// viên cần phân biệt "gõ sai lệnh" với "máy đang tắt".
		response.Conflict(c, err.Error())
	default:
		response.BadRequest(c, err.Error())
	}
}

// Wake
// @Summary      Wake a machine (Wake-on-LAN)
// @Description  Broadcast a magic packet on the shop LAN to power on a machine that is off. Needs the MAC address the machine reported in a past heartbeat and Wake-on-LAN enabled in its BIOS/NIC.
// @Tags         Machines
// @Produce      json
// @Security     BearerAuth
// @Param        id   path  string  true  "Machine ID"
// @Success      200  {object}  response.Response{data=service.WakeResult}
// @Failure      400  {object}  response.Response
// @Failure      409  {object}  response.Response "máy đang bật"
// @Router       /api/machines/{id}/wake [post]
func (h *MachineHandler) Wake(c *gin.Context) {
	result, err := h.svc.Wake(c.Param("id"), middleware.GetUserID(c))
	switch {
	case err == nil:
		response.Success(c, result)
	case errors.Is(err, service.ErrWakeAlreadyOn):
		response.Conflict(c, err.Error())
	default:
		response.BadRequest(c, err.Error())
	}
}

// ListGroups
// @Summary      List Machine Groups
// @Description  Get all machine groups for the current store
// @Tags         MachineGroups
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200   {object}  response.Response{data=[]model.MachineGroup}
// @Failure      500   {object}  response.Response
// @Router       /api/machine-groups [get]
func (h *MachineHandler) ListGroups(c *gin.Context) {
	groups, err := h.svc.ListGroups(c.Query("search"))
	if err != nil {
		response.InternalError(c, "Failed to fetch machine groups")
		return
	}
	response.Success(c, groups)
}

// CreateGroup
// @Summary      Create Machine Group
// @Description  Create a new machine group
// @Tags         MachineGroups
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  service.CreateMachineGroupRequest  true  "Machine group data"
// @Success      201   {object}  response.Response{data=model.MachineGroup}
// @Failure      400   {object}  response.Response
// @Router       /api/machine-groups [post]
func (h *MachineHandler) CreateGroup(c *gin.Context) {
	var req service.CreateMachineGroupRequest
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

// UpdateGroup
// @Summary      Update Machine Group
// @Description  Update an existing machine group
// @Tags         MachineGroups
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string                        true  "Machine Group ID"
// @Param        body  body  service.UpdateMachineGroupRequest  true  "Machine group update data"
// @Success      200   {object}  response.Response{data=model.MachineGroup}
// @Failure      400   {object}  response.Response
// @Failure      404   {object}  response.Response
// @Router       /api/machine-groups/{id} [put]
func (h *MachineHandler) UpdateGroup(c *gin.Context) {
	id := c.Param("id")
	var req service.UpdateMachineGroupRequest
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

// DeleteGroup
// @Summary      Delete Machine Group
// @Description  Delete a machine group by its ID
// @Tags         MachineGroups
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string  true  "Machine Group ID"
// @Success      200   {object}  response.Response
// @Failure      400   {object}  response.Response
// @Failure      404   {object}  response.Response
// @Failure 409 {object} response.Response "còn dữ liệu phụ thuộc"
// @Router       /api/machine-groups/{id} [delete]
func (h *MachineHandler) DeleteGroup(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.DeleteGroup(id); err != nil {
		handleDeleteError(c, err)
		return
	}
	response.Success(c, nil)
}

// ListAssets
// @Summary      List Machine Assets
// @Description  Get machine assets, optionally filtered by machine
// @Tags         MachineAssets
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        machine_id  query  string  false  "Filter by machine ID"
// @Success      200         {object}  response.Response{data=[]model.MachineAsset}
// @Failure      500         {object}  response.Response
// @Router       /api/machine-assets [get]
func (h *MachineHandler) ListAssets(c *gin.Context) {
	machineID := c.Query("machine_id")
	assets, err := h.svc.ListAssets(machineID)
	if err != nil {
		response.InternalError(c, "Failed to fetch machine assets")
		return
	}
	response.Success(c, assets)
}

// CreateAsset
// @Summary      Create Machine Asset
// @Description  Create a new machine asset record
// @Tags         MachineAssets
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  service.CreateMachineAssetRequest  true  "Machine asset data"
// @Success      201   {object}  response.Response{data=model.MachineAsset}
// @Failure      400   {object}  response.Response
// @Router       /api/machine-assets [post]
func (h *MachineHandler) CreateAsset(c *gin.Context) {
	var req service.CreateMachineAssetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	result, err := h.svc.CreateAsset(&req)
	if err != nil {
		handleCreateError(c, err)
		return
	}
	response.Created(c, result)
}

// UpdateAsset
// @Summary      Update Machine Asset
// @Description  Update an existing machine asset
// @Tags         MachineAssets
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string                        true  "Machine Asset ID"
// @Param        body  body  service.UpdateMachineAssetRequest  true  "Machine asset update data"
// @Success      200   {object}  response.Response{data=model.MachineAsset}
// @Failure      400   {object}  response.Response
// @Failure      404   {object}  response.Response
// @Router       /api/machine-assets/{id} [put]
func (h *MachineHandler) UpdateAsset(c *gin.Context) {
	id := c.Param("id")
	var req service.UpdateMachineAssetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	result, err := h.svc.UpdateAsset(id, &req, middleware.GetUserID(c))
	if err != nil {
		handleCreateError(c, err)
		return
	}
	response.Success(c, result)
}

// DeleteAsset
// @Summary      Delete Machine Asset
// @Description  Delete a machine asset by its ID
// @Tags         MachineAssets
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string  true  "Machine Asset ID"
// @Success      200   {object}  response.Response
// @Failure      400   {object}  response.Response
// @Failure      404   {object}  response.Response
// @Failure 409 {object} response.Response "còn dữ liệu phụ thuộc"
// @Router       /api/machine-assets/{id} [delete]
func (h *MachineHandler) DeleteAsset(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.DeleteAsset(id); err != nil {
		handleDeleteError(c, err)
		return
	}
	response.Success(c, nil)
}

// ReportScreenshot nhận ảnh máy trạm gửi lên sau lệnh remote:screenshot.
func (h *MachineHandler) ReportScreenshot(c *gin.Context) {
	code := c.Param("code")
	var req service.ScreenshotReport
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	if err := h.svc.ReportScreenshot(code, &req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// ReportProcesses nhận danh sách tiến trình máy trạm gửi lên sau
// remote:process-list hoặc remote:process-kill.
func (h *MachineHandler) ReportProcesses(c *gin.Context) {
	code := c.Param("code")
	var req service.ProcessReport
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	if err := h.svc.ReportProcesses(code, &req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// RemoteDesktop
// @Summary      Prepare Remote Desktop
// @Description  Kiểm tra máy đang bật, nối thử được TightVNC (cổng 5900), rồi trả mật khẩu VNC cho trình xem noVNC
// @Tags         Machines
// @Produce      json
// @Security     BearerAuth
// @Param        id   path  string  true  "Machine ID"
// @Success      200  {object}  response.Response{data=map[string]string}  "{ password }"
// @Failure      409  {object}  response.Response  "máy tắt, chưa có IP, hoặc VNC không trả lời"
// @Router       /api/machines/{id}/remote-desktop [get]
func (h *MachineHandler) RemoteDesktop(c *gin.Context) {
	m, err := h.svc.RemoteDesktopTarget(c.Param("id"))
	if err != nil {
		response.Conflict(c, err.Error())
		return
	}
	// Nối thử trước: lỗi trong lúc bắt tay WebSocket trình duyệt không đọc
	// được, nên mọi lỗi có thể đoán trước phải báo ở đây.
	tcp, err := net.DialTimeout("tcp", net.JoinHostPort(m.IPAddress, remotedesk.Port), 3*time.Second)
	if err != nil {
		response.Conflict(c, "không nối được VNC trên máy "+m.MachineCode+" ("+m.IPAddress+"): máy trạm chưa cài TightVNC hoặc tường lửa chặn")
		return
	}
	tcp.Close()
	password, err := h.svc.VNCPassword(m.ID)
	if err != nil {
		response.InternalError(c, "không lấy được mật khẩu VNC")
		return
	}
	response.Success(c, gin.H{"password": password})
}

// RemoteDesktopWS
// @Summary      Remote Desktop WebSocket
// @Description  WebSocket cho noVNC, chuyển byte hai chiều tới TightVNC trên máy trạm. Token qua ?token=
// @Tags         Machines
// @Security     BearerAuth
// @Param        id   path  string  true  "Machine ID"
// @Success      101
// @Router       /api/machines/{id}/remote-desktop/ws [get]
func (h *MachineHandler) RemoteDesktopWS(c *gin.Context) {
	m, err := h.svc.RemoteDesktopTarget(c.Param("id"))
	if err != nil {
		response.Conflict(c, err.Error())
		return
	}
	tcp, err := net.DialTimeout("tcp", net.JoinHostPort(m.IPAddress, remotedesk.Port), 5*time.Second)
	if err != nil {
		response.Error(c, http.StatusBadGateway, "không nối được VNC trên máy "+m.MachineCode)
		return
	}
	ws, err := h.hub.Upgrade(c.Writer, c.Request)
	if err != nil {
		tcp.Close()
		return
	}
	actorID := middleware.GetUserID(c)
	h.svc.LogRemoteDesktop(m, actorID, true)
	remotedesk.Bridge(ws, tcp)
	h.svc.LogRemoteDesktop(m, actorID, false)
}
