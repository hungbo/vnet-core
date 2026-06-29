package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/middleware"
	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/pagination"
	"github.com/vnet/core/pkg/response"
)

type MachineHandler struct {
	svc *service.MachineService
}

func NewMachineHandler(svc *service.MachineService) *MachineHandler {
	_ = model.Machine{}
	return &MachineHandler{svc: svc}
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
// @Success      200       {object}  response.Response{data=response.PaginatedData{data=[]model.Machine}}
// @Failure      500       {object}  response.Response
// @Router       /api/machines [get]
func (h *MachineHandler) List(c *gin.Context) {
	params := pagination.GetParams(c)
	storeID := middleware.GetStoreID(c)
	result, err := h.svc.List(*params, storeID)
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
	storeID := middleware.GetStoreID(c)
	result, err := h.svc.Create(&req, storeID)
	if err != nil {
		handleCreateError(c, err)
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
// @Router       /api/machines/{id} [delete]
func (h *MachineHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.Delete(id); err != nil {
		response.BadRequest(c, err.Error())
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
	if err := h.svc.Heartbeat(id, req.CPUTemp, req.GPUTemp, req.IP, req.MAC); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
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
// @Param        action  path  string  true  "Action name (restart, shutdown, etc.)"
// @Success      200   {object}  response.Response
// @Failure      400   {object}  response.Response
// @Router       /api/machines/{id}/action/{action} [post]
func (h *MachineHandler) RemoteAction(c *gin.Context) {
	id := c.Param("id")
	action := c.Param("action")

	var payload interface{}
	if err := c.ShouldBindJSON(&payload); err == nil {
	} else {
		payload = nil
	}

	if err := h.svc.RemoteAction(id, action, payload); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
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
	storeID := middleware.GetStoreID(c)
	groups, err := h.svc.ListGroups(storeID)
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
	storeID := middleware.GetStoreID(c)
	result, err := h.svc.CreateGroup(&req, storeID)
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
// @Router       /api/machine-groups/{id} [delete]
func (h *MachineHandler) DeleteGroup(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.DeleteGroup(id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// ListPrices
// @Summary      List Machine Prices
// @Description  Get machine prices, optionally filtered by machine group
// @Tags         MachinePrices
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        machine_group_id  query  string  false  "Filter by machine group ID"
// @Success      200               {object}  response.Response{data=[]model.MachinePrice}
// @Failure      500               {object}  response.Response
// @Router       /api/machine-prices [get]
func (h *MachineHandler) ListPrices(c *gin.Context) {
	machineGroupID := c.Query("machine_group_id")
	prices, err := h.svc.ListPrices(machineGroupID)
	if err != nil {
		response.InternalError(c, "Failed to fetch machine prices")
		return
	}
	response.Success(c, prices)
}

// CreatePrice
// @Summary      Create Machine Price
// @Description  Create a new machine price entry
// @Tags         MachinePrices
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  service.CreateMachinePriceRequest  true  "Machine price data"
// @Success      201   {object}  response.Response{data=model.MachinePrice}
// @Failure      400   {object}  response.Response
// @Router       /api/machine-prices [post]
func (h *MachineHandler) CreatePrice(c *gin.Context) {
	var req service.CreateMachinePriceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	result, err := h.svc.CreatePrice(&req)
	if err != nil {
		handleCreateError(c, err)
		return
	}
	response.Created(c, result)
}

// UpdatePrice
// @Summary      Update Machine Price
// @Description  Update an existing machine price
// @Tags         MachinePrices
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string                        true  "Machine Price ID"
// @Param        body  body  service.UpdateMachinePriceRequest  true  "Machine price update data"
// @Success      200   {object}  response.Response{data=model.MachinePrice}
// @Failure      400   {object}  response.Response
// @Failure      404   {object}  response.Response
// @Router       /api/machine-prices/{id} [put]
func (h *MachineHandler) UpdatePrice(c *gin.Context) {
	id := c.Param("id")
	var req service.UpdateMachinePriceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	result, err := h.svc.UpdatePrice(id, &req)
	if err != nil {
		handleCreateError(c, err)
		return
	}
	response.Success(c, result)
}

// DeletePrice
// @Summary      Delete Machine Price
// @Description  Delete a machine price by its ID
// @Tags         MachinePrices
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  string  true  "Machine Price ID"
// @Success      200   {object}  response.Response
// @Failure      400   {object}  response.Response
// @Failure      404   {object}  response.Response
// @Router       /api/machine-prices/{id} [delete]
func (h *MachineHandler) DeletePrice(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.DeletePrice(id); err != nil {
		response.BadRequest(c, err.Error())
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
	result, err := h.svc.UpdateAsset(id, &req)
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
// @Router       /api/machine-assets/{id} [delete]
func (h *MachineHandler) DeleteAsset(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.DeleteAsset(id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}
