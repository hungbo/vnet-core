package service

import (
	"errors"
	"log"
	"time"

	"github.com/vnet/core/internal/hub"
	"github.com/vnet/core/internal/model"
	"github.com/vnet/core/pkg/pagination"
	"gorm.io/gorm"
)

type ChatService struct {
	db    *gorm.DB
	hub   *hub.Hub
	audit *AuditService
}

func NewChatService(db *gorm.DB, wsHub *hub.Hub, audit *AuditService) *ChatService {
	return &ChatService{db: db, hub: wsHub, audit: audit}
}

type CreateConversationRequest struct {
	Title         string   `json:"title"`
	ParticipantID string   `json:"participant_id" binding:"required"`
	ParticipantType string `json:"participant_type" binding:"required"`
}

type SendMessageRequest struct {
	ConversationID string `json:"conversation_id" binding:"required"`
	SenderType     string `json:"sender_type" binding:"required"`
	SenderID       string `json:"sender_id" binding:"required"`
	Message        string `json:"message" binding:"required"`
	MessageType    string `json:"message_type"`
}

type ConversationResponse struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	IsGroup     bool              `json:"is_group"`
	LastMessage *MessageResponse  `json:"last_message,omitempty"`
	UnreadCount int               `json:"unread_count"`
	Participants []ParticipantInfo `json:"participants,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
}

type ParticipantInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type MessageResponse struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	SenderType     string    `json:"sender_type"`
	SenderID       string    `json:"sender_id"`
	Message        string    `json:"message"`
	MessageType    string    `json:"message_type"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

func messageResponseFromModel(m *model.ChatMessage) MessageResponse {
	return MessageResponse{
		ID:             m.ID,
		ConversationID: m.ConversationID,
		SenderType:     m.SenderType,
		SenderID:       m.SenderID,
		Message:        m.Message,
		MessageType:    m.MessageType,
		Status:         m.Status,
		CreatedAt:      m.CreatedAt,
	}
}

func (s *ChatService) ListConversations(participantID string, participantType string, params pagination.Params) ([]ConversationResponse, int64, int, int, error) {
	log.Printf("[ChatSvc] ListConversations: participantID=%s type=%s page=%d", participantID, participantType, params.Page)
	query := s.db.Model(&model.ChatConversation{})

	if participantID != "" {
		var convIDs []string
		if err := s.db.Model(&model.ChatParticipant{}).Where("participant_id = ? AND participant_type = ?", participantID, participantType).Pluck("conversation_id", &convIDs).Error; err != nil {
			return nil, 0, 0, 0, nil
		}
		if len(convIDs) == 0 {
			return []ConversationResponse{}, 0, params.Page, params.PageSize, nil
		}
		query = query.Where("id IN ?", convIDs)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, 0, 0, err
	}

	var conversations []model.ChatConversation
	if err := pagination.Apply(query.Order("created_at DESC"), &params).Find(&conversations).Error; err != nil {
		return nil, 0, 0, 0, err
	}

	result := make([]ConversationResponse, len(conversations))
	for i, conv := range conversations {
		var lastMsg model.ChatMessage
		var lastMsgResp *MessageResponse
		if err := s.db.Where("conversation_id = ?", conv.ID).Order("created_at desc").First(&lastMsg).Error; err == nil {
			resp := messageResponseFromModel(&lastMsg)
			lastMsgResp = &resp
		}

		var unreadCount int64
		if participantID != "" {
			var participantRow model.ChatParticipant
			if err := s.db.Where("conversation_id = ? AND participant_id = ?", conv.ID, participantID).First(&participantRow).Error; err == nil {
				if participantRow.LastReadAt != nil {
					s.db.Model(&model.ChatMessage{}).Where("conversation_id = ? AND created_at > ?", conv.ID, *participantRow.LastReadAt).Count(&unreadCount)
				} else {
					s.db.Model(&model.ChatMessage{}).Where("conversation_id = ?", conv.ID).Count(&unreadCount)
				}
			}
		}

		result[i] = ConversationResponse{
			ID:          conv.ID,
			Title:       conv.Title,
			IsGroup:     conv.IsGroup,
			LastMessage: lastMsgResp,
			UnreadCount: int(unreadCount),
			Participants: s.getParticipants(conv.ID),
			CreatedAt:   conv.CreatedAt,
		}
	}

	return result, total, params.Page, params.PageSize, nil
}

func (s *ChatService) CreateConversation(req *CreateConversationRequest) (*ConversationResponse, error) {
	conv := model.ChatConversation{
		Title:   req.Title,
		IsGroup: false,
	}

	tx := s.db.Begin()

	if err := tx.Create(&conv).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	participant := model.ChatParticipant{
		ConversationID:  conv.ID,
		ParticipantType: req.ParticipantType,
		ParticipantID:   req.ParticipantID,
	}
	if err := tx.Create(&participant).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	tx.Commit()

	s.audit.Log(&LogAuditRequest{
		Action:     "create",
		EntityType: "chat_conversation",
		EntityID:   conv.ID,
		Metadata:   map[string]interface{}{"participant_id": req.ParticipantID, "participant_type": req.ParticipantType},
	})

	if s.hub != nil {
		s.hub.JoinAllAdminsToRoom(conv.ID)
		s.hub.BroadcastToType(hub.Event{
			Type: "conversation:new",
			Data: map[string]string{"conversation_id": conv.ID},
		}, hub.ClientTypeAdmin)
	}

	return &ConversationResponse{
		ID:      conv.ID,
		Title:   conv.Title,
		IsGroup: conv.IsGroup,
		CreatedAt: conv.CreatedAt,
	}, nil
}

func (s *ChatService) getParticipants(conversationID string) []ParticipantInfo {
	var participants []model.ChatParticipant
	if err := s.db.Where("conversation_id = ?", conversationID).Find(&participants).Error; err != nil {
		return nil
	}
	result := make([]ParticipantInfo, 0, len(participants))
	for _, p := range participants {
		info := ParticipantInfo{ID: p.ParticipantID}
		if p.ParticipantType == "member" {
			var member model.Member
			if err := s.db.Select("id, full_name, username").Where("id = ?", p.ParticipantID).First(&member).Error; err == nil {
				info.Name = member.FullName
			}
		}
		result = append(result, info)
	}
	return result
}

func (s *ChatService) GetMessages(conversationID string, params pagination.Params) ([]MessageResponse, int64, int, int, error) {
	query := s.db.Model(&model.ChatMessage{}).Where("conversation_id = ?", conversationID).Order("created_at ASC")

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, 0, 0, err
	}

	var messages []model.ChatMessage
	if err := pagination.Apply(query, &params).Find(&messages).Error; err != nil {
		return nil, 0, 0, 0, err
	}

	result := make([]MessageResponse, len(messages))
	for i, msg := range messages {
		result[i] = messageResponseFromModel(&msg)
	}

	return result, total, params.Page, params.PageSize, nil
}

func (s *ChatService) MarkMessageDelivered(messageID string) error {
	var msg model.ChatMessage
	if err := s.db.Select("conversation_id").Where("id = ?", messageID).First(&msg).Error; err != nil {
		return err
	}
	if err := s.db.Model(&model.ChatMessage{}).Where("id = ? AND status = ?", messageID, "sent").Update("status", "delivered").Error; err != nil {
		return err
	}
	s.broadcastStatus(msg.ConversationID, messageID, "delivered")
	return nil
}

func (s *ChatService) MarkMessageRead(messageID string) error {
	var msg model.ChatMessage
	if err := s.db.Select("conversation_id").Where("id = ?", messageID).First(&msg).Error; err != nil {
		return err
	}
	if err := s.db.Model(&model.ChatMessage{}).Where("id = ?", messageID).Update("status", "read").Error; err != nil {
		return err
	}
	s.broadcastStatus(msg.ConversationID, messageID, "read")
	return nil
}

func (s *ChatService) MarkConversationMessagesRead(conversationID string) (int64, error) {
	result := s.db.Model(&model.ChatMessage{}).
		Where("conversation_id = ? AND status != ?", conversationID, "read").
		Update("status", "read")
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected > 0 {
		s.hub.PublishToRoom(conversationID, hub.Event{
			Type: "conversation:read",
			Data: map[string]interface{}{
				"conversation_id": conversationID,
				"status":          "read",
			},
		}, "")
	}
	return result.RowsAffected, nil
}

func (s *ChatService) broadcastStatus(conversationID, messageID, status string) {
	if s.hub == nil {
		return
	}
	s.hub.PublishToRoom(conversationID, hub.Event{
		Type: "message:status:updated",
		Data: map[string]interface{}{
			"id":     messageID,
			"status": status,
		},
	}, "")
}

func (s *ChatService) SendMessage(req *SendMessageRequest) (*MessageResponse, error) {
	log.Printf("[ChatSvc] SendMessage: conv=%s sender=%s type=%s", req.ConversationID, req.SenderType, req.MessageType)
	var conv model.ChatConversation
	if err := s.db.Where("id = ?", req.ConversationID).First(&conv).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("conversation not found")
		}
		return nil, err
	}

	msgType := req.MessageType
	if msgType == "" {
		msgType = "text"
	}

	msg := model.ChatMessage{
		ConversationID: req.ConversationID,
		SenderType:     req.SenderType,
		SenderID:       req.SenderID,
		Message:        req.Message,
		MessageType:    msgType,
	}

	if err := s.db.Create(&msg).Error; err != nil {
		return nil, err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "send_message",
		EntityType: "chat_message",
		EntityID:   msg.ID,
		Metadata:   map[string]interface{}{"conversation_id": req.ConversationID},
	})

	result := messageResponseFromModel(&msg)

	if s.hub != nil {
		s.hub.JoinRoomByUserID(req.SenderID, req.ConversationID)
		s.hub.PublishToRoom(req.ConversationID, hub.Event{
			Type: "chat:message",
			Data: result,
		}, req.SenderID)
	}

	log.Printf("[ChatSvc] SendMessage done: id=%s status=%s", msg.ID, msg.Status)

	return &result, nil
}

func (s *ChatService) HubRoomSync() {
	if s.hub == nil {
		return
	}
	s.hub.OnConnect(func(client *hub.Client) []string {
		if client.UserID == "" {
			return nil
		}
		if client.ClientType == hub.ClientTypeAdmin {
			var allIDs []string
			s.db.Model(&model.ChatConversation{}).Pluck("id", &allIDs)
			return allIDs
		}
		var convIDs []string
		s.db.Model(&model.ChatParticipant{}).
			Where("participant_id = ?", client.UserID).
			Pluck("conversation_id", &convIDs)
		return convIDs
	})
}

func (s *ChatService) DeleteAllConversations() error {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("1 = 1").Delete(&model.ChatParticipant{}).Error; err != nil {
			return err
		}
		if err := tx.Where("1 = 1").Delete(&model.ChatMessage{}).Error; err != nil {
			return err
		}
		if err := tx.Where("1 = 1").Delete(&model.ChatConversation{}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	if s.hub != nil {
		s.hub.Broadcast(hub.Event{
			Type: "conversations:cleared",
		})
		s.hub.RemoveAllRooms()
	}
	return nil
}

func (s *ChatService) DeleteConversation(conversationID string) error {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("conversation_id = ?", conversationID).Delete(&model.ChatParticipant{}).Error; err != nil {
			return err
		}
		if err := tx.Where("conversation_id = ?", conversationID).Delete(&model.ChatMessage{}).Error; err != nil {
			return err
		}
		if err := tx.Where("id = ?", conversationID).Delete(&model.ChatConversation{}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	if s.hub != nil {
		s.hub.PublishToRoom(conversationID, hub.Event{
			Type: "conversation:deleted",
			Data: map[string]interface{}{
				"conversation_id": conversationID,
			},
		}, "")
		s.hub.RemoveRoom(conversationID)
	}
	return nil
}

func (s *ChatService) MarkRead(conversationID string, participantID string) error {
	now := time.Now()
	if err := s.db.Model(&model.ChatParticipant{}).Where("conversation_id = ? AND participant_id = ?", conversationID, participantID).Update("last_read_at", &now).Error; err != nil {
		return err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "mark_read",
		EntityType: "chat_conversation",
		EntityID:   conversationID,
		Metadata:   map[string]interface{}{"participant_id": participantID},
	})

	return nil
}
