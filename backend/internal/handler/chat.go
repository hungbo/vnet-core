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

func (h *ChatHandler) ListConversations(c *gin.Context) {
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

	log.Printf("[Chat] ListConversations: participantID=%s type=%s", participantID, participantType)

	params := pagination.GetParams(c)

	conversations, total, page, pageSize, err := h.svc.ListConversations(participantID, participantType, *params)
	if err != nil {
		log.Printf("[Chat] ListConversations error: %v", err)
		response.InternalError(c, "Failed to fetch conversations")
		return
	}
	log.Printf("[Chat] ListConversations result: %d conversations (total=%d)", len(conversations), total)
	response.Paginated(c, conversations, total, page, pageSize)
}

func (h *ChatHandler) GetMessages(c *gin.Context) {
	conversationID := c.Param("id")
	params := pagination.GetParams(c)
	log.Printf("[Chat] GetMessages: conversationID=%s page=%d size=%d", conversationID, params.Page, params.PageSize)

	messages, total, page, pageSize, err := h.svc.GetMessages(conversationID, *params)
	if err != nil {
		log.Printf("[Chat] GetMessages error: %v", err)
		response.NotFound(c, "Conversation not found")
		return
	}
	log.Printf("[Chat] GetMessages result: %d messages (total=%d)", len(messages), total)
	response.Paginated(c, messages, total, page, pageSize)
}

func (h *ChatHandler) CreateConversation(c *gin.Context) {
	var req service.CreateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}
	result, err := h.svc.CreateConversation(&req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, result)
}

func (h *ChatHandler) ensureConversation(senderID, senderType string) (string, error) {
	convs, _, _, _, err := h.svc.ListConversations(senderID, senderType,
		pagination.Params{Page: 1, PageSize: 1, Sort: "created_at", Order: "desc"})
	if err == nil && len(convs) > 0 {
		return convs[0].ID, nil
	}
	conv, err := h.svc.CreateConversation(&service.CreateConversationRequest{
		Title:           "Hỗ trợ",
		ParticipantID:   senderID,
		ParticipantType: senderType,
	})
	if err != nil {
		return "", err
	}
	return conv.ID, nil
}

func (h *ChatHandler) SendMessage(c *gin.Context) {
	var input struct {
		ConversationID string `json:"conversation_id"`
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
	log.Printf("[Chat] SendMessage: conv=%s sender=%s type=%s msg_len=%d", input.ConversationID, input.SenderType, input.MessageType, len(input.Message))

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

	convID := input.ConversationID
	if convID == "" && input.SenderType == "member" && input.SenderID != "" {
		var err error
		convID, err = h.ensureConversation(input.SenderID, "member")
		if err != nil {
			handleValidationError(c, nil)
			return
		}
	}

	if convID == "" {
		handleValidationError(c, nil)
		return
	}

	req := service.SendMessageRequest{
		ConversationID: convID,
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

func (h *ChatHandler) DeleteConversation(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.DeleteConversation(id); err != nil {
		response.InternalError(c, "Failed to delete conversation")
		return
	}
	response.Success(c, nil)
}

func (h *ChatHandler) DeleteAllConversations(c *gin.Context) {
	if err := h.svc.DeleteAllConversations(); err != nil {
		response.InternalError(c, "Failed to delete all conversations")
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

func (h *ChatHandler) MarkConversationMessagesRead(c *gin.Context) {
	id := c.Param("id")
	count, err := h.svc.MarkConversationMessagesRead(id)
	if err != nil {
		response.InternalError(c, "Failed to mark read")
		return
	}
	response.Success(c, map[string]int64{"updated": count})
}

func (h *ChatHandler) RequestTopup(c *gin.Context) {
	var req TopupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationError(c, err)
		return
	}

	convID, err := h.ensureConversation(req.MemberID, "member")
	if err != nil {
		response.InternalError(c, "Failed to create conversation")
		return
	}

	msg := service.SendMessageRequest{
		ConversationID: convID,
		SenderType:     "member",
		SenderID:       req.MemberID,
		Message:        "Yêu cầu nạp " + formatAmount(req.Amount) + " từ máy " + req.MachineCode,
		MessageType:    "topup_request",
	}

	result, err := h.svc.SendMessage(&msg)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, result)
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
