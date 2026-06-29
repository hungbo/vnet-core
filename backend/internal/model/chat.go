package model

import "time"

type ChatRoom struct {
	ID        string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Title     string     `gorm:"type:varchar(200)" json:"title"`
	IsGroup   bool       `gorm:"default:false" json:"is_group"`
	CreatedAt time.Time  `gorm:"default:now()" json:"created_at,omitempty"`
	DeletedAt *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

type ChatParticipant struct {
	ID               string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	RoomID   string     `gorm:"type:uuid;not null;index;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"room_id"`
	ParticipantType  string     `gorm:"type:varchar(20);not null" json:"participant_type"`
	ParticipantID    string     `gorm:"type:uuid;not null" json:"participant_id"`
	LastReadAt       *time.Time `gorm:"type:timestamptz" json:"last_read_at"`
	JoinedAt         time.Time  `gorm:"default:now()" json:"joined_at"`
}

type ChatMessage struct {
	ID             string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	RoomID         string    `gorm:"type:uuid;not null;index:idx_chat_messages_conv" json:"room_id"`
	SenderType     string    `gorm:"type:varchar(20);not null" json:"sender_type"`
	SenderID       string    `gorm:"type:uuid;not null" json:"sender_id"`
	SenderUsername string    `gorm:"type:varchar(100)" json:"sender_username"`
	Message        string    `gorm:"type:text;not null" json:"message"`
	MessageType    string    `gorm:"type:varchar(20);default:text" json:"message_type"`
	Status         string    `gorm:"type:varchar(20);default:sent" json:"status"`
	CreatedAt      time.Time `gorm:"default:now()" json:"created_at,omitempty"`
}

type ServiceFeedback struct {
	ID        string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	MachineID string    `gorm:"type:uuid;not null;index;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"machine_id"`
	MemberID  *string   `gorm:"type:uuid;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"member_id"`
	OrderID   *string   `gorm:"type:uuid;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"order_id"`
	Rating    int       `gorm:"type:integer" json:"rating"`
	Content   string    `gorm:"type:text" json:"content"`
	CreatedAt time.Time `gorm:"default:now()" json:"created_at,omitempty"`
}
