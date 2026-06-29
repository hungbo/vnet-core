package handler

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/vnet/core/internal/middleware"
	"github.com/vnet/core/internal/service"
	"github.com/vnet/core/pkg/pagination"
	"github.com/vnet/core/pkg/response"
)

type ChatHandler struct {
	svc *service.ChatService
}

func NewChatHandler(svc *service.ChatService) *ChatHandler {
	return &ChatHandler{svc: svc}
}

func (h *ChatHandler) ListRooms(c *gin.Context) {
	participantID := c.Query("participant_id")
	participantType := c.Query("participant_type")
	if participantID == "" {
		participantID = middleware.GetUserID(c)
		role := c.GetString(middleware.ContextKeyRole)
		if role == "" || role == "member" {
			participantType = "member"
		} else {
			participantID = ""
			participantType = ""
		}
	}

	log.Printf("[Chat] ListRooms: participantID=%s type=%s", participantID, participantType)

	params := pagination.GetParams(c)

	rooms, total, page, pageSize, err := h.svc.ListRooms(participantID, participantType, *params)
	if err != nil {
		log.Printf("[Chat] ListRooms error: %v", err)
		response.InternalError(c, "Failed to fetch rooms")
		return
	}
	log.Printf("[Chat] ListRooms result: %d rooms (total=%d)", len(rooms), total)
	response.Paginated(c, rooms, total, page, pageSize)
}

func (h *ChatHandler) GetMessages(c *gin.Context) {
	roomID := c.Param("id")
	params := pagination.GetParams(c)
	log.Printf("[Chat] GetMessages: roomID=%s page=%d size=%d", roomID, params.Page, params.PageSize)

	messages, total, page, pageSize, err := h.svc.GetMessages(roomID, *params)
	if err != nil {
		log.Printf("[Chat] GetMessages error: %v", err)
		response.NotFound(c, "Room not found")
		return
	}
	log.Printf("[Chat] GetMessages result: %d messages (total=%d)", len(messages), total)
	response.Paginated(c, messages, total, page, pageSize)
}

func (h *ChatHandler) CreateRoom(c *gin.Context) {
	var req service.CreateRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	result, err := h.svc.CreateRoom(&req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, result)
}

func (h *ChatHandler) ensureRoom(senderID, senderType string) (string, error) {
	rooms, _, _, _, err := h.svc.ListRooms(senderID, senderType,
		pagination.Params{Page: 1, PageSize: 1, Sort: "created_at", Order: "desc"})
	if err == nil && len(rooms) > 0 {
		return rooms[0].ID, nil
	}
	room, err := h.svc.CreateRoom(&service.CreateRoomRequest{
		Title:           "Hỗ trợ",
		ParticipantID:   senderID,
		ParticipantType: senderType,
	})
	if err != nil {
		return "", err
	}
	return room.ID, nil
}

func (h *ChatHandler) SendMessage(c *gin.Context) {
	var input struct {
		RoomID string `json:"room_id"`
		SenderType     string `json:"sender_type"`
		SenderID       string `json:"sender_id"`
		Message        string `json:"message"`
		MessageType    string `json:"message_type"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Chat] SendMessage bind error: %v", err)
		handleValidationError(c, err)
		return
	}
	log.Printf("[Chat] SendMessage: conv=%s sender=%s type=%s msg_len=%d", input.RoomID, input.SenderType, input.MessageType, len(input.Message))

	if input.Message == "" {
		handleValidationError(c, nil)
		return
	}

	if input.SenderType == "" {
		input.SenderType = "admin"
	}
	if input.SenderID == "" {
		input.SenderID = middleware.GetUserID(c)
	}
	if input.MessageType == "" {
		input.MessageType = "text"
	}

	roomID := input.RoomID
	if roomID == "" && input.SenderType == "member" && input.SenderID != "" {
		var err error
		roomID, err = h.ensureRoom(input.SenderID, "member")
		if err != nil {
			handleValidationError(c, nil)
			return
		}
	}

	if roomID == "" {
		handleValidationError(c, nil)
		return
	}

	req := service.SendMessageRequest{
		RoomID: roomID,
		SenderType:     input.SenderType,
		SenderID:       input.SenderID,
		Message:        input.Message,
		MessageType:    input.MessageType,
	}

	result, err := h.svc.SendMessage(&req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, result)
}

type TopupRequest struct {
	Amount      int64  `json:"amount" binding:"required"`
	MemberID    string `json:"member_id" binding:"required"`
	MachineCode string `json:"machine_code"`
}

func (h *ChatHandler) DeleteRoom(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.DeleteRoom(id); err != nil {
		response.InternalError(c, "Failed to delete room")
		return
	}
	response.Success(c, nil)
}

func (h *ChatHandler) DeleteAllRooms(c *gin.Context) {
	if err := h.svc.DeleteAllRooms(); err != nil {
		response.InternalError(c, "Failed to delete all rooms")
		return
	}
	response.Success(c, nil)
}

func (h *ChatHandler) MarkMessageDelivered(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.MarkMessageDelivered(id); err != nil {
		response.InternalError(c, "Failed to mark delivered")
		return
	}
	response.Success(c, nil)
}

func (h *ChatHandler) MarkMessageRead(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.MarkMessageRead(id); err != nil {
		response.InternalError(c, "Failed to mark read")
		return
	}
	response.Success(c, nil)
}

func (h *ChatHandler) MarkRoomMessagesRead(c *gin.Context) {
	id := c.Param("id")
	count, err := h.svc.MarkRoomMessagesRead(id)
	if err != nil {
		response.InternalError(c, "Failed to mark read")
		return
	}
	if userID := middleware.GetUserID(c); userID != "" {
		if err := h.svc.MarkRead(id, userID); err != nil {
			log.Printf("[Chat] MarkRead: %v", err)
		}
	}
	response.Success(c, map[string]int64{"updated": count})
}

func (h *ChatHandler) RequestTopup(c *gin.Context) {
	response.BadRequest(c, "Topup requests are now handled via orders. Use POST /orders/topup-request")
}

func formatAmount(amount int64) string {
	if amount >= 1000000 {
		mil := amount / 1000000
		rem := (amount % 1000000) / 100000
		if rem > 0 {
			return fmt.Sprintf("%d.%dk", mil, rem)
		}
		return fmt.Sprintf("%dtr", mil)
	}
	return fmt.Sprintf("%dk", amount/1000)
}
