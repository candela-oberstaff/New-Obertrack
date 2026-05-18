package models

import (
	"time"

	"gorm.io/gorm"
)

type ChannelType string

const (
	ChannelTypePublic  ChannelType = "public"
	ChannelTypePrivate ChannelType = "private"
	ChannelTypeDirect  ChannelType = "direct"
)

type Channel struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	Name          string         `gorm:"size:100;not null;uniqueIndex:idx_channel_name_type" json:"name"`
	Description   string         `gorm:"size:500" json:"description"`
	Type          ChannelType    `gorm:"type:varchar(20);not null;default:'public';uniqueIndex:idx_channel_name_type" json:"type"`
	CreatedBy     uint           `gorm:"not null;index" json:"created_by"`
	CreatedByUser User           `gorm:"foreignKey:CreatedBy" json:"creator,omitempty"`
	IsActive      bool           `gorm:"default:true" json:"is_active"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`

	Members []User `gorm:"many2many:channel_members" json:"members,omitempty"`
}

func (Channel) TableName() string {
	return "channels"
}

type ChannelMember struct {
	ChannelID uint      `gorm:"primaryKey;index:idx_member_user_channel" json:"channel_id"`
	UserID    uint      `gorm:"primaryKey;index:idx_member_user_channel" json:"user_id"`
	Role      string    `gorm:"size:20;default:'member'" json:"role"` // admin, member
	JoinedAt   time.Time  `json:"joined_at"`
	LastReadAt *time.Time `json:"last_read_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

func (ChannelMember) TableName() string {
	return "channel_members"
}

type ChannelMessage struct {
	ID         uint              `gorm:"primaryKey" json:"id"`
	ChannelID  uint              `gorm:"not null;index:idx_channel_msg_channel_deleted" json:"channel_id"`
	UserID     uint              `gorm:"not null;index" json:"user_id"`
	User       User              `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Content    string            `gorm:"type:text" json:"content"`
	Attachment string            `gorm:"size:500" json:"attachment,omitempty"`
	FileName   string            `gorm:"size:255" json:"file_name,omitempty"`
	FileSize   int64             `json:"file_size,omitempty"`
	FileType   string            `gorm:"size:50" json:"file_type,omitempty"`
	IsEdited   bool              `gorm:"default:false" json:"is_edited"`
	IsDeleted  bool              `gorm:"default:false;index:idx_channel_msg_channel_deleted" json:"is_deleted"`
	IsPinned   bool              `gorm:"default:false" json:"is_pinned"`
	ParentID   *uint             `gorm:"index" json:"parent_id,omitempty"`
	Reactions  []MessageReaction `gorm:"foreignKey:MessageID" json:"reactions,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
	DeletedAt  gorm.DeletedAt    `gorm:"index:idx_channel_msg_channel_deleted" json:"-"`
}

type MessageReaction struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	MessageID uint      `gorm:"not null;index" json:"message_id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	User      User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Emoji     string    `gorm:"size:50;not null" json:"emoji"`
	CreatedAt time.Time `json:"created_at"`
}

func (MessageReaction) TableName() string {
	return "message_reactions"
}

type StarredMessage struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	MessageID uint      `gorm:"not null;index" json:"message_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (StarredMessage) TableName() string {
	return "starred_messages"
}

type UserStatus struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"uniqueIndex;not null" json:"user_id"`
	Status    string    `gorm:"size:20;default:'offline'" json:"status"`
	LastSeen  time.Time `json:"last_seen"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (UserStatus) TableName() string {
	return "user_statuses"
}

type Mention struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	MessageID uint      `gorm:"not null;index" json:"message_id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	Notified  bool      `gorm:"default:false" json:"notified"`
	CreatedAt time.Time `json:"created_at"`
}

func (Mention) TableName() string {
	return "mentions"
}

func (ChannelMessage) TableName() string {
	return "channel_messages"
}
