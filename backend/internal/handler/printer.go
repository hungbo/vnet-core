package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/response"
)

type PrinterHandler struct {
	svc *service.PrinterService
}

func NewPrinterHandler(svc *service.PrinterService) *PrinterHandler {
	return &PrinterHandler{svc: svc}
}

// @Summary List all printers
// @Description Get a list of all printers
// @Tags Printers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response{data=[]service.PrinterResponse}
// @Failure 500 {object} response.Response
// @Router /printers [get]
func (h *PrinterHandler) List(c *gin.Context) {
	result, err := h.svc.List()
	if err != nil {
		response.InternalError(c, "Failed to fetch printers")
		return
	}

	response.Success(c, result)
}

// @Summary Get printer by ID
// @Description Get a single printer by its ID
// @Tags Printers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Printer ID"
// @Success 200 {object} response.Response{data=service.PrinterResponse}
// @Failure 404 {object} response.Response
// @Router /printers/{id} [get]
func (h *PrinterHandler) GetByID(c *gin.Context) {
	id := c.Param("id")

	result, err := h.svc.GetByID(id)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}

	response.Success(c, result)
}

// @Summary Create a new printer
// @Description Create a new printer
// @Tags Printers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body service.CreatePrinterRequest true "Printer data"
// @Success 201 {object} response.Response{data=service.PrinterResponse}
// @Failure 400 {object} response.Response
// @Router /printers [post]
func (h *PrinterHandler) Create(c *gin.Context) {
	var req service.CreatePrinterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	result, err := h.svc.Create(&req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Created(c, result)
}

// @Summary Update a printer
// @Description Update an existing printer by ID
// @Tags Printers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Printer ID"
// @Param body body service.UpdatePrinterRequest true "Printer update data"
// @Success 200 {object} response.Response{data=service.PrinterResponse}
// @Failure 400 {object} response.Response
// @Router /printers/{id} [put]
func (h *PrinterHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req service.UpdatePrinterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	result, err := h.svc.Update(id, &req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, result)
}

// @Summary Delete a printer
// @Description Delete a printer by ID
// @Tags Printers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Printer ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Router /printers/{id} [delete]
func (h *PrinterHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	if err := h.svc.Delete(id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, nil)
}

// @Summary Test printer connection
// @Description Send a test print job to the printer
// @Tags Printers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Printer ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Router /printers/{id}/test [post]
func (h *PrinterHandler) TestPrint(c *gin.Context) {
	id := c.Param("id")

	if err := h.svc.TestPrint(id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, "Test print sent successfully")
}
