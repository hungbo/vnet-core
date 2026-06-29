package model

import "time"

type User struct {
	ID           string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Username     string     `gorm:"type:varchar(50);not null;uniqueIndex" json:"username"`
	PasswordHash string     `gorm:"type:varchar(255);not null" json:"-"`
	FullName     string     `gorm:"type:varchar(100)" json:"full_name"`
	Email        string     `gorm:"type:varchar(100)" json:"email"`
	Phone        string     `gorm:"type:varchar(20)" json:"phone"`
	AvatarURL    string     `gorm:"type:text" json:"avatar_url"`
	StoreID      *string    `gorm:"type:uuid;index" json:"store_id"`
	IsActive     bool       `gorm:"default:true" json:"is_active"`
	LastLoginAt  *time.Time `gorm:"type:timestamptz" json:"last_login_at"`
	CreatedAt    time.Time  `gorm:"default:now()" json:"created_at,omitempty"`
	UpdatedAt    time.Time  `gorm:"default:now()" json:"updated_at,omitempty"`
	DeletedAt    *time.Time `gorm:"index" json:"deleted_at,omitempty"`

	Roles []Role `gorm:"many2many:user_roles;foreignKey:ID;joinForeignKey:UserID;References:ID;joinReferences:RoleID" json:"roles,omitempty"`
}

type Role struct {
	ID          string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name        string    `gorm:"type:varchar(50);not null;unique" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	CreatedAt   time.Time `gorm:"default:now()" json:"created_at,omitempty"`

	Permissions []Permission `gorm:"many2many:role_permissions;foreignKey:ID;joinForeignKey:RoleID;References:ID;joinReferences:PermissionID" json:"permissions,omitempty"`
	Users       []User       `gorm:"many2many:user_roles;foreignKey:ID;joinForeignKey:RoleID;References:ID;joinReferences:UserID" json:"users,omitempty"`
}

type Permission struct {
	ID        string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Code      string    `gorm:"type:varchar(100);not null;unique" json:"code"`
	Name      string    `gorm:"type:varchar(100)" json:"name"`
	Module    string    `gorm:"type:varchar(50)" json:"module"`
	CreatedAt time.Time `gorm:"default:now()" json:"created_at,omitempty"`
}

type UserRole struct {
	UserID string `gorm:"type:uuid;primaryKey" json:"user_id"`
	RoleID string `gorm:"type:uuid;primaryKey" json:"role_id"`
}

type RolePermission struct {
	RoleID       string `gorm:"type:uuid;primaryKey" json:"role_id"`
	PermissionID string `gorm:"type:uuid;primaryKey" json:"permission_id"`
}
