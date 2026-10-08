package handler

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/middleware"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/response"
)

type BookingHandler struct {
	svc *service.BookingService
}

func NewBookingHandler(svc *service.BookingService) *BookingHandler {
	return &BookingHandler{svc: svc}
}

// List retrieves all bookings with pagination
// @Summary List bookings
// @Description Get a paginated list of bookings with optional filters
// @Tags Bookings
// @Accept json
// @Produce json
// @Param request query service.BookingListRequest false "List query params"
// @Success 200 {object} response.Response{data=response.PaginatedData{data=[]service.BookingResponse}}
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /bookings [get]
// @Security BearerAuth
func (h *BookingHandler) List(c *gin.Context) {
	var req service.BookingListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	result, err := h.svc.List(&req)
	if err != nil {
		// Bộ lọc sai (trạng thái lạ, machine_id không phải uuid, ngày sai định
		// dạng) là lỗi của người gọi — trả 400 kèm lý do, không phải 500.
		if errors.Is(err, service.ErrDatChoKhongHopLe) {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, "Failed to fetch bookings")
		return
	}

	response.Success(c, result)
}

// GetByID retrieves a single booking by ID
// @Summary Get booking by ID
// @Description Get detailed information about a specific booking
// @Tags Bookings
// @Accept json
// @Produce json
// @Param id path string true "Booking ID"
// @Success 200 {object} response.Response{data=service.BookingResponse}
// @Failure 404 {object} response.Response
// @Router /bookings/{id} [get]
// @Security BearerAuth
func (h *BookingHandler) GetByID(c *gin.Context) {
	id := c.Param("id")

	result, err := h.svc.GetByID(id)
	if err != nil {
		response.NotFound(c, "Booking not found")
		return
	}

	response.Success(c, result)
}

// Create creates a new booking
// @Summary Create booking
// @Description Create a new machine booking for a member or walk-in customer
// @Tags Bookings
// @Accept json
// @Produce json
// @Param request body service.CreateBookingRequest true "Booking details"
// @Success 201 {object} response.Response{data=service.BookingResponse}
// @Failure 400 {object} response.Response
// @Router /bookings [post]
// @Security BearerAuth
func (h *BookingHandler) Create(c *gin.Context) {
	var req service.CreateBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	userID := c.GetString(middleware.ContextKeyUserID)

	result, err := h.svc.Create(&req, userID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Created(c, result)
}

// Update updates an existing booking
// @Summary Update booking
// @Description Update a pending booking. deposit_amount cannot be changed after creation (400 if it differs); omitted fields keep their value. Changing the time re-runs the create checks (overlap, limits, cancel deadline).
// @Tags Bookings
// @Accept json
// @Produce json
// @Param id path string true "Booking ID"
// @Param request body service.UpdateBookingRequest true "Booking details"
// @Success 200 {object} response.Response{data=service.BookingResponse}
// @Failure 400 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /bookings/{id} [put]
// @Security BearerAuth
func (h *BookingHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req service.UpdateBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	userID := c.GetString(middleware.ContextKeyUserID)

	result, err := h.svc.Update(id, &req, userID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, result)
}

// Delete deletes a booking
// @Summary Delete booking
// @Description Soft delete a booking by ID
// @Tags Bookings
// @Accept json
// @Produce json
// @Param id path string true "Booking ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 409 {object} response.Response "còn dữ liệu phụ thuộc"
// @Router /bookings/{id} [delete]
// @Security BearerAuth
func (h *BookingHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	// Truyền người thao tác để nhật ký ghi được AI đã xoá — bản cũ để trống.
	if err := h.svc.Delete(id, c.GetString(middleware.ContextKeyUserID)); err != nil {
		handleDeleteError(c, err)
		return
	}

	response.Success(c, nil)
}

// CheckIn checks in a booking
// @Summary Check-in booking
// @Description Mark a booking as checked in (allowed from 30 minutes before start until booked_to). The deposit is refunded to the member's balance.
// @Tags Bookings
// @Accept json
// @Produce json
// @Param id path string true "Booking ID"
// @Success 200 {object} response.Response{data=service.BookingResponse}
// @Failure 400 {object} response.Response
// @Router /bookings/{id}/check-in [post]
// @Security BearerAuth
func (h *BookingHandler) CheckIn(c *gin.Context) {
	id := c.Param("id")

	result, err := h.svc.CheckIn(id, c.GetString(middleware.ContextKeyUserID))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, result)
}

// Cancel cancels a booking
// @Summary Cancel booking
// @Description Cancel a booking before check-in
// @Tags Bookings
// @Accept json
// @Produce json
// @Param id path string true "Booking ID"
// @Success 200 {object} response.Response{data=service.BookingResponse}
// @Failure 400 {object} response.Response
// @Router /bookings/{id}/cancel [post]
// @Security BearerAuth
func (h *BookingHandler) Cancel(c *gin.Context) {
	id := c.Param("id")

	result, err := h.svc.Cancel(id, c.GetString(middleware.ContextKeyUserID))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, result)
}

// NoShow marks a booking as no-show
// @Summary No-show booking
// @Description Mark a booking as no-show (allowed from 15 minutes after start, same cutoff as the scheduler). The deposit is forfeited.
// @Tags Bookings
// @Accept json
// @Produce json
// @Param id path string true "Booking ID"
// @Success 200 {object} response.Response{data=service.BookingResponse}
// @Failure 400 {object} response.Response
// @Router /bookings/{id}/no-show [post]
// @Security BearerAuth
func (h *BookingHandler) NoShow(c *gin.Context) {
	id := c.Param("id")

	result, err := h.svc.NoShow(id, c.GetString(middleware.ContextKeyUserID))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, result)
}
