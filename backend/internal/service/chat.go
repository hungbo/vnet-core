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

type CreateRoomRequest struct {
	Title         string   `json:"title"`
	ParticipantID string   `json:"participant_id" binding:"required"`
	ParticipantType string `json:"participant_type" binding:"required"`
}

type SendMessageRequest struct {
	RoomID string `json:"room_id" binding:"required"`
	SenderType     string `json:"sender_type" binding:"required"`
	SenderID       string `json:"sender_id" binding:"required"`
	Message        string `json:"message" binding:"required"`
	MessageType    string `json:"message_type"`
}

type RoomResponse struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	IsGroup     bool              `json:"is_group"`
	LastMessage *MessageResponse  `json:"last_message,omitempty"`
	UnreadCount int               `json:"unread_count"`
	Participants []ParticipantInfo `json:"participants,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
}

type ParticipantInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Username    string `json:"username"`
	MachineCode string `json:"machine_code"`
}

type MessageResponse struct {
	ID             string    `json:"id"`
	RoomID string    `json:"room_id"`
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
		RoomID: m.RoomID,
		SenderType:     m.SenderType,
		SenderID:       m.SenderID,
		Message:        m.Message,
		MessageType:    m.MessageType,
		Status:         m.Status,
		CreatedAt:      m.CreatedAt,
	}
}

func (s *ChatService) ListRooms(participantID string, participantType string, params pagination.Params) ([]RoomResponse, int64, int, int, error) {
	log.Printf("[ChatSvc] ListRooms: participantID=%s type=%s page=%d", participantID, participantType, params.Page)
	query := s.db.Model(&model.ChatRoom{})

	if participantID != "" {
		var roomIDs []string
		if err := s.db.Model(&model.ChatParticipant{}).Where("participant_id = ? AND participant_type = ?", participantID, participantType).Pluck("room_id", &roomIDs).Error; err != nil {
			return nil, 0, 0, 0, nil
		}
		if len(roomIDs) == 0 {
			return []RoomResponse{}, 0, params.Page, params.PageSize, nil
		}
		query = query.Where("id IN ?", roomIDs)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, 0, 0, err
	}

	var rooms []model.ChatRoom
	if err := pagination.Apply(query.Order("created_at DESC"), &params).Find(&rooms).Error; err != nil {
		return nil, 0, 0, 0, err
	}

	result := make([]RoomResponse, len(rooms))
	for i, room := range rooms {
		var lastMsg model.ChatMessage
		var lastMsgResp *MessageResponse
		if err := s.db.Where("room_id = ?", room.ID).Order("created_at desc").First(&lastMsg).Error; err == nil {
			resp := messageResponseFromModel(&lastMsg)
			lastMsgResp = &resp
		}

		var unreadCount int64
		if participantID != "" {
			var participantRow model.ChatParticipant
			if err := s.db.Where("room_id = ? AND participant_id = ?", room.ID, participantID).First(&participantRow).Error; err == nil {
				if participantRow.LastReadAt != nil {
					s.db.Model(&model.ChatMessage{}).Where("room_id = ? AND created_at > ?", room.ID, *participantRow.LastReadAt).Count(&unreadCount)
				} else {
					s.db.Model(&model.ChatMessage{}).Where("room_id = ?", room.ID).Count(&unreadCount)
				}
			}
		}

		result[i] = RoomResponse{
			ID:          room.ID,
			Title:       room.Title,
			IsGroup:     room.IsGroup,
			LastMessage: lastMsgResp,
			UnreadCount: int(unreadCount),
			Participants: s.getParticipants(room.ID),
			CreatedAt:   room.CreatedAt,
		}
	}

	return result, total, params.Page, params.PageSize, nil
}

func (s *ChatService) CreateRoom(req *CreateRoomRequest) (*RoomResponse, error) {
	room := model.ChatRoom{
		Title:   req.Title,
		IsGroup: false,
	}

	tx := s.db.Begin()

	if err := tx.Create(&room).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	participant := model.ChatParticipant{
		RoomID:  room.ID,
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
		EntityType: "chat_room",
		EntityID:   room.ID,
		Metadata:   map[string]interface{}{"participant_id": req.ParticipantID, "participant_type": req.ParticipantType},
	})

	if s.hub != nil {
		s.hub.JoinAllAdminsToRoom(room.ID)
		s.hub.BroadcastToType(hub.Event{
			Type: "room:new",
			Data: map[string]string{"room_id": room.ID},
		}, hub.ClientTypeAdmin)
	}

	return &RoomResponse{
		ID:      room.ID,
		Title:   room.Title,
		IsGroup: room.IsGroup,
		CreatedAt: room.CreatedAt,
	}, nil
}

func (s *ChatService) getParticipants(roomID string) []ParticipantInfo {
	var participants []model.ChatParticipant
	if err := s.db.Where("room_id = ?", roomID).Find(&participants).Error; err != nil {
		return nil
	}
	result := make([]ParticipantInfo, 0, len(participants))
	for _, p := range participants {
		info := ParticipantInfo{ID: p.ParticipantID}
		if p.ParticipantType == "member" {
			var member model.Member
			if err := s.db.Select("id, full_name, username").Where("id = ?", p.ParticipantID).First(&member).Error; err == nil {
				info.Name = member.FullName
				info.Username = member.Username
			}
			var session model.MachineSession
			if err := s.db.Where("member_id = ? AND is_active = ?", p.ParticipantID, true).First(&session).Error; err == nil {
				var machine model.Machine
				if err := s.db.Select("machine_code").Where("id = ?", session.MachineID).First(&machine).Error; err == nil {
					info.MachineCode = machine.MachineCode
				}
			}
		}
		result = append(result, info)
	}
	return result
}

func (s *ChatService) GetMessages(roomID string, params pagination.Params) ([]MessageResponse, int64, int, int, error) {
	query := s.db.Model(&model.ChatMessage{}).Where("room_id = ?", roomID).Order("created_at DESC")

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
	if err := s.db.Select("room_id").Where("id = ?", messageID).First(&msg).Error; err != nil {
		return err
	}
	if err := s.db.Model(&model.ChatMessage{}).Where("id = ? AND status = ?", messageID, "sent").Update("status", "delivered").Error; err != nil {
		return err
	}
	s.broadcastStatus(msg.RoomID, messageID, "delivered")
	return nil
}

func (s *ChatService) MarkMessageRead(messageID string) error {
	var msg model.ChatMessage
	if err := s.db.Select("room_id").Where("id = ?", messageID).First(&msg).Error; err != nil {
		return err
	}
	if err := s.db.Model(&model.ChatMessage{}).Where("id = ?", messageID).Update("status", "read").Error; err != nil {
		return err
	}
	s.broadcastStatus(msg.RoomID, messageID, "read")
	return nil
}

func (s *ChatService) MarkRoomMessagesRead(roomID string) (int64, error) {
	result := s.db.Model(&model.ChatMessage{}).
		Where("room_id = ? AND status != ?", roomID, "read").
		Update("status", "read")
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected > 0 {
		s.hub.PublishToRoom(roomID, hub.Event{
			Type: "room:read",
			Data: map[string]interface{}{
				"room_id": roomID,
				"status":          "read",
			},
		}, "")
	}
	return result.RowsAffected, nil
}

func (s *ChatService) broadcastStatus(roomID, messageID, status string) {
	if s.hub == nil {
		return
	}
	s.hub.PublishToRoom(roomID, hub.Event{
		Type: "message:status:updated",
		Data: map[string]interface{}{
			"id":     messageID,
			"status": status,
		},
	}, "")
}

func (s *ChatService) SendMessage(req *SendMessageRequest) (*MessageResponse, error) {
	log.Printf("[ChatSvc] SendMessage: room=%s sender=%s type=%s", req.RoomID, req.SenderType, req.MessageType)
	var room model.ChatRoom
	if err := s.db.Where("id = ?", req.RoomID).First(&room).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("room not found")
		}
		return nil, err
	}

	msgType := req.MessageType
	if msgType == "" {
		msgType = "text"
	}

	msg := model.ChatMessage{
		RoomID: req.RoomID,
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
		Metadata:   map[string]interface{}{"room_id": req.RoomID},
	})

	result := messageResponseFromModel(&msg)

	if s.hub != nil {
		s.hub.JoinRoomByUserID(req.SenderID, req.RoomID)
		s.hub.PublishToRoom(req.RoomID, hub.Event{
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
			s.db.Model(&model.ChatRoom{}).Pluck("id", &allIDs)
			return allIDs
		}
		var roomIDs []string
		s.db.Model(&model.ChatParticipant{}).
			Where("participant_id = ?", client.UserID).
			Pluck("room_id", &roomIDs)
		return roomIDs
	})
}

func (s *ChatService) DeleteAllRooms() error {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("1 = 1").Delete(&model.ChatParticipant{}).Error; err != nil {
			return err
		}
		if err := tx.Where("1 = 1").Delete(&model.ChatMessage{}).Error; err != nil {
			return err
		}
		if err := tx.Where("1 = 1").Delete(&model.ChatRoom{}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	if s.hub != nil {
		s.hub.Broadcast(hub.Event{
			Type: "rooms:cleared",
		})
		s.hub.RemoveAllRooms()
	}
	return nil
}

func (s *ChatService) DeleteRoom(roomID string) error {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("room_id = ?", roomID).Delete(&model.ChatParticipant{}).Error; err != nil {
			return err
		}
		if err := tx.Where("room_id = ?", roomID).Delete(&model.ChatMessage{}).Error; err != nil {
			return err
		}
		if err := tx.Where("id = ?", roomID).Delete(&model.ChatRoom{}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	if s.hub != nil {
		s.hub.PublishToRoom(roomID, hub.Event{
			Type: "room:deleted",
			Data: map[string]interface{}{
				"room_id": roomID,
			},
		}, "")
		s.hub.RemoveRoom(roomID)
	}
	return nil
}

func (s *ChatService) MarkRead(roomID string, participantID string) error {
	now := time.Now()
	if err := s.db.Model(&model.ChatParticipant{}).Where("room_id = ? AND participant_id = ?", roomID, participantID).Update("last_read_at", &now).Error; err != nil {
		return err
	}

	s.audit.Log(&LogAuditRequest{
		Action:     "mark_read",
		EntityType: "chat_room",
		EntityID:   roomID,
		Metadata:   map[string]interface{}{"participant_id": participantID},
	})

	return nil
}
