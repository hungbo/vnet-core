package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/response"
)

type PromotionHandler struct {
	svc *service.PromotionService
}

func NewPromotionHandler(svc *service.PromotionService) *PromotionHandler {
	return &PromotionHandler{svc: svc}
}

// List retrieves all promotions with pagination
// @Summary List promotions
// @Description Get a paginated list of promotions with optional filters
// @Tags Promotions
// @Accept json
// @Produce json
// @Param request query service.PromotionListRequest false "List query params"
// @Success 200 {object} response.Response{data=response.PaginatedData{data=[]service.PromotionResponse}}
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /promotions [get]
// @Security BearerAuth
func (h *PromotionHandler) List(c *gin.Context) {
	var req service.PromotionListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	result, err := h.svc.List(&req)
	if err != nil {
		response.InternalError(c, "Failed to fetch promotions")
		return
	}

	response.Success(c, result)
}

// GetByID retrieves a single promotion by ID
// @Summary Get promotion by ID
// @Description Get detailed information about a specific promotion
// @Tags Promotions
// @Accept json
// @Produce json
// @Param id path string true "Promotion ID"
// @Success 200 {object} response.Response{data=service.PromotionResponse}
// @Failure 404 {object} response.Response
// @Router /promotions/{id} [get]
// @Security BearerAuth
func (h *PromotionHandler) GetByID(c *gin.Context) {
	id := c.Param("id")

	result, err := h.svc.GetByID(id)
	if err != nil {
		response.NotFound(c, "Promotion not found")
		return
	}

	response.Success(c, result)
}

// Create creates a new promotion
// @Summary Create promotion
// @Description Create a new promotion with conditions and rewards
// @Tags Promotions
// @Accept json
// @Produce json
// @Param request body service.CreatePromotionRequest true "Promotion details"
// @Success 201 {object} response.Response{data=service.PromotionResponse}
// @Failure 400 {object} response.Response
// @Router /promotions [post]
// @Security BearerAuth
func (h *PromotionHandler) Create(c *gin.Context) {
	var req service.CreatePromotionRequest
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

// Update updates an existing promotion
// @Summary Update promotion
// @Description Update the details of an existing promotion
// @Tags Promotions
// @Accept json
// @Produce json
// @Param id path string true "Promotion ID"
// @Param request body service.UpdatePromotionRequest true "Promotion details"
// @Success 200 {object} response.Response{data=service.PromotionResponse}
// @Failure 400 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /promotions/{id} [put]
// @Security BearerAuth
func (h *PromotionHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req service.UpdatePromotionRequest
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

// Delete deletes a promotion
// @Summary Delete promotion
// @Description Soft delete a promotion by ID
// @Tags Promotions
// @Accept json
// @Produce json
// @Param id path string true "Promotion ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Router /promotions/{id} [delete]
// @Security BearerAuth
func (h *PromotionHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	if err := h.svc.Delete(id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, nil)
}

// GetLuckySpinRewards retrieves all lucky spin rewards
// @Summary Get lucky spin rewards
// @Description Get the list of available lucky spin rewards
// @Tags LuckySpin
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=[]service.LuckySpinRewardResponse}
// @Failure 500 {object} response.Response
// @Router /lucky-spin/rewards [get]
// @Security BearerAuth
func (h *PromotionHandler) GetLuckySpinRewards(c *gin.Context) {
	// include_inactive=true dành cho màn quản trị: ô đã tắt vẫn phải nhìn thấy
	// để bật lại. Vòng quay gọi không kèm tham số nên vẫn chỉ thấy ô đang bật.
	result, err := h.svc.GetLuckySpinRewards(c.Query("include_inactive") == "true")
	if err != nil {
		response.InternalError(c, "Failed to fetch lucky spin rewards")
		return
	}

	response.Success(c, result)
}

// CreateLuckySpinReward adds a reward slot to the wheel
// @Summary Create lucky spin reward
// @Tags LuckySpin
// @Accept json
// @Produce json
// @Param request body service.LuckySpinRewardRequest true "Reward"
// @Success 201 {object} response.Response{data=service.LuckySpinRewardResponse}
// @Failure 400 {object} response.Response
// @Router /lucky-spin/rewards [post]
// @Security BearerAuth
func (h *PromotionHandler) CreateLuckySpinReward(c *gin.Context) {
	var req service.LuckySpinRewardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	result, err := h.svc.CreateLuckySpinReward(&req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, result)
}

// UpdateLuckySpinReward edits a reward slot
// @Summary Update lucky spin reward
// @Tags LuckySpin
// @Accept json
// @Produce json
// @Param id path string true "Reward ID"
// @Param request body service.LuckySpinRewardRequest true "Reward"
// @Success 200 {object} response.Response{data=service.LuckySpinRewardResponse}
// @Failure 400 {object} response.Response
// @Router /lucky-spin/rewards/{id} [put]
// @Security BearerAuth
func (h *PromotionHandler) UpdateLuckySpinReward(c *gin.Context) {
	var req service.LuckySpinRewardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	result, err := h.svc.UpdateLuckySpinReward(c.Param("id"), &req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
}

// DeleteLuckySpinReward removes a reward slot
// @Summary Delete lucky spin reward
// @Tags LuckySpin
// @Produce json
// @Param id path string true "Reward ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Router /lucky-spin/rewards/{id} [delete]
// @Security BearerAuth
func (h *PromotionHandler) DeleteLuckySpinReward(c *gin.Context) {
	if err := h.svc.DeleteLuckySpinReward(c.Param("id")); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// Spin performs a lucky spin
// @Summary Spin the wheel
// @Description Perform a lucky spin to win a random reward
// @Tags LuckySpin
// @Accept json
// @Produce json
// @Param request body service.SpinRequest true "Spin details"
// @Success 200 {object} response.Response{data=service.SpinResponse}
// @Failure 400 {object} response.Response
// @Router /lucky-spin/spin [post]
// @Security BearerAuth
func (h *PromotionHandler) Spin(c *gin.Context) {
	var req service.SpinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	result, err := h.svc.Spin(&req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, result)
}
