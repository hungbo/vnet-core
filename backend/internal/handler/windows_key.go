package handler

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/middleware"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/pagination"
	"github.com/vnet/core/pkg/response"
)

type WindowsKeyHandler struct {
	svc *service.WindowsKeyService
}

func NewWindowsKeyHandler(svc *service.WindowsKeyService) *WindowsKeyHandler {
	return &WindowsKeyHandler{svc: svc}
}

// fail đổi lỗi nghiệp vụ đã biết thành 400/404; lỗi lạ là 500.
func (h *WindowsKeyHandler) fail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrWindowsKeyNoMachine), errors.Is(err, service.ErrWindowsKeyNotFound):
		response.NotFound(c, err.Error())
	case errors.Is(err, service.ErrWindowsKeyInvalid), errors.Is(err, service.ErrWindowsKeyEmptyImport),
		errors.Is(err, service.ErrWindowsKeyTooMany):
		response.BadRequest(c, err.Error())
	default:
		response.InternalError(c, "Lỗi Windows key", err)
	}
}

// List
// @Summary      Windows keys by machine
// @Description  One row per machine; keys are masked (last 5 characters)
// @Tags         WindowsKeys
// @Produce      json
// @Security     BearerAuth
// @Param        search  query  string  false  "Machine code"
// @Param        filter  query  string  false  "no_key | mismatch | unlicensed"
// @Success      200  {object}  response.Response{data=response.PaginatedData{data=[]service.WindowsKeyRow}}
// @Router       /api/windows-keys [get]
func (h *WindowsKeyHandler) List(c *gin.Context) {
	params := pagination.GetParams(c)
	res, err := h.svc.List(*params, service.WindowsKeyFilter{Search: params.Search, Filter: c.Query("filter")})
	if err != nil {
		h.fail(c, err)
		return
	}
	response.Paginated(c, res.Items, res.Total, res.Page, res.PageSize)
}

// Summary
// @Summary      Windows key counts
// @Tags         WindowsKeys
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response{data=service.WindowsKeySummary}
// @Router       /api/windows-keys/summary [get]
func (h *WindowsKeyHandler) Summary(c *gin.Context) {
	out, err := h.svc.Summary()
	if err != nil {
		h.fail(c, err)
		return
	}
	response.Success(c, out)
}

type setWindowsKeyRequest struct {
	Key     string `json:"key" binding:"required"`
	Edition string `json:"edition"`
	Note    string `json:"note"`
}

// Set
// @Summary      Set the Windows key of a machine
// @Tags         WindowsKeys
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        machine_id  path  string  true  "Machine ID"
// @Success      200  {object}  response.Response
// @Router       /api/windows-keys/{machine_id} [put]
func (h *WindowsKeyHandler) Set(c *gin.Context) {
	var req setWindowsKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	if err := h.svc.SetKey(c.Param("machine_id"), req.Key, req.Edition, req.Note, c.GetString(middleware.ContextKeyUserID)); err != nil {
		h.fail(c, err)
		return
	}
	response.Success(c, nil)
}

// Delete
// @Summary      Remove the Windows key of a machine
// @Tags         WindowsKeys
// @Produce      json
// @Security     BearerAuth
// @Param        machine_id  path  string  true  "Machine ID"
// @Success      200  {object}  response.Response
// @Router       /api/windows-keys/{machine_id} [delete]
func (h *WindowsKeyHandler) Delete(c *gin.Context) {
	if err := h.svc.DeleteKey(c.Param("machine_id"), c.GetString(middleware.ContextKeyUserID)); err != nil {
		h.fail(c, err)
		return
	}
	response.Success(c, nil)
}

type importWindowsKeysRequest struct {
	Text      string `json:"text" binding:"required"`
	Overwrite bool   `json:"overwrite"`
	DryRun    bool   `json:"dry_run"`
}

// Import
// @Summary      Bulk import Windows keys
// @Description  One "machine-code key" per line; dry_run only reports what would happen
// @Tags         WindowsKeys
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response{data=service.ImportResult}
// @Router       /api/windows-keys/import [post]
func (h *WindowsKeyHandler) Import(c *gin.Context) {
	var req importWindowsKeysRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	// Xem trước không ghi gì, nên không được làm mọi trang quản trị tải lại.
	if req.DryRun {
		middleware.SkipChange(c)
	}
	res, err := h.svc.Import(req.Text, req.Overwrite, req.DryRun, c.GetString(middleware.ContextKeyUserID))
	if err != nil {
		h.fail(c, err)
		return
	}
	response.Success(c, res)
}

// Reveal
// @Summary      Reveal the full Windows key of a machine
// @Description  Requires windows_keys.reveal; every call is written to the audit log
// @Tags         WindowsKeys
// @Produce      json
// @Security     BearerAuth
// @Param        machine_id  path  string  true  "Machine ID"
// @Success      200  {object}  response.Response
// @Router       /api/windows-keys/{machine_id}/reveal [get]
func (h *WindowsKeyHandler) Reveal(c *gin.Context) {
	key, err := h.svc.RevealKey(c.Param("machine_id"), c.GetString(middleware.ContextKeyUserID))
	if err != nil {
		h.fail(c, err)
		return
	}
	response.Success(c, gin.H{"key": key})
}
