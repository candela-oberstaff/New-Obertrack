package models

import "time"

// Tablas del servidor MCP de solo lectura (asistentes de IA). Ver
// internal/mcp y docs/features/mcp-asistentes-ia.md.
//
// Ningún token se guarda en claro: solo su sha256 en hex. Los booleanos no
// llevan `default:true` a propósito (GORM guardaría true al crear con false);
// los valores iniciales los siembra la migración con SQL.

// MCPOAuthClient es un cliente registrado por registro dinámico (RFC 7591):
// ChatGPT, Claude, Grok… Cada instalación de un conector se registra sola.
type MCPOAuthClient struct {
	ClientID                string    `gorm:"primaryKey;size:64" json:"client_id"`
	ClientName              string    `gorm:"size:200" json:"client_name"`
	RedirectURIs            string    `gorm:"type:jsonb;not null" json:"redirect_uris"`
	TokenEndpointAuthMethod string    `gorm:"size:40;not null" json:"token_endpoint_auth_method"`
	ClientSecretHash        *string   `gorm:"size:64" json:"-"`
	CreatedAt               time.Time `json:"created_at"`
}

func (MCPOAuthClient) TableName() string { return "mcp_oauth_clients" }

// MCPAuthCode es un código de autorización: 5 minutos y un solo uso.
type MCPAuthCode struct {
	CodeHash      string     `gorm:"primaryKey;size:64" json:"-"`
	ClientID      string     `gorm:"size:64;not null" json:"client_id"`
	UserID        uint       `gorm:"not null" json:"user_id"`
	RedirectURI   string     `gorm:"type:text;not null" json:"redirect_uri"`
	CodeChallenge string     `gorm:"size:128;not null" json:"-"`
	ExpiresAt     time.Time  `gorm:"not null" json:"expires_at"`
	UsedAt        *time.Time `json:"used_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

func (MCPAuthCode) TableName() string { return "mcp_auth_codes" }

// MCPPersonalClientID marca las conexiones creadas con un token personal
// (Gemini CLI y otros clientes sin OAuth) en vez de con el flujo OAuth.
const MCPPersonalClientID = "personal"

// MCPConnection es una autorización viva de un usuario a un cliente. Revocarla
// corta todos sus tokens de golpe.
type MCPConnection struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	UserID       uint       `gorm:"not null;index" json:"user_id"`
	ClientID     string     `gorm:"size:64;not null" json:"client_id"`
	ClientName   string     `gorm:"size:200" json:"client_name"`
	IP           string     `gorm:"size:64" json:"ip"`
	UserAgent    string     `gorm:"size:400" json:"user_agent"`
	CreatedAt    time.Time  `json:"created_at"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt    time.Time  `gorm:"not null" json:"expires_at"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
	RevokedBy    *uint      `json:"revoked_by,omitempty"`
	RevokeReason string     `gorm:"size:300" json:"revoke_reason,omitempty"`
}

func (MCPConnection) TableName() string { return "mcp_connections" }

const (
	MCPTokenAccess   = "access"
	MCPTokenRefresh  = "refresh"
	MCPTokenPersonal = "personal"
)

type MCPToken struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	ConnectionID uint       `gorm:"not null;index" json:"connection_id"`
	Kind         string     `gorm:"size:10;not null" json:"kind"`
	TokenHash    string     `gorm:"size:64;not null;uniqueIndex" json:"-"`
	ExpiresAt    time.Time  `gorm:"not null" json:"expires_at"`
	UsedAt       *time.Time `json:"used_at,omitempty"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

func (MCPToken) TableName() string { return "mcp_tokens" }

// MCPUsage es una fila por llamada a un tool. Es lo que cuenta la cuota y lo
// que muestra el panel; la auditoría no registra cada llamada.
type MCPUsage struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `gorm:"not null;index:idx_mcp_usage_user_day" json:"user_id"`
	ConnectionID uint      `json:"connection_id"`
	Tool         string    `gorm:"size:60;not null" json:"tool"`
	OK           bool      `gorm:"column:ok;not null" json:"ok"`
	Ms           int       `gorm:"not null" json:"ms"`
	Rows         int       `gorm:"not null" json:"rows"`
	Day          time.Time `gorm:"type:date;not null;index:idx_mcp_usage_user_day" json:"day"`
	CreatedAt    time.Time `json:"created_at"`
}

func (MCPUsage) TableName() string { return "mcp_usage" }

// MCPRoleQuota es la cuota y el permiso de uso por rol. Enabled es el
// equivalente de "accion:usar-mcp": un rol apagado no puede conectar.
type MCPRoleQuota struct {
	RoleName   string    `gorm:"primaryKey;size:30" json:"role_name"`
	Enabled    bool      `gorm:"not null" json:"enabled"`
	DailyCalls int       `gorm:"not null" json:"daily_calls"`
	MaxRows    int       `gorm:"not null" json:"max_rows"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (MCPRoleQuota) TableName() string { return "mcp_role_quotas" }

// MCPSettings es una fila única (ID = 1) con los interruptores globales.
type MCPSettings struct {
	ID                   uint      `gorm:"primaryKey" json:"-"`
	Paused               bool      `gorm:"not null" json:"paused"`
	MinimalData          bool      `gorm:"not null" json:"minimal_data"`
	ExtraRedirectDomains string    `gorm:"type:text;not null" json:"extra_redirect_domains"`
	UpdatedBy            *uint     `json:"updated_by,omitempty"`
	UpdatedAt            time.Time `json:"updated_at"`
}

func (MCPSettings) TableName() string { return "mcp_settings" }

// MCPAlert se ve solo en el panel de admin: no hay correos.
type MCPAlert struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	UserID    uint       `gorm:"index" json:"user_id"`
	Kind      string     `gorm:"size:40;not null" json:"kind"`
	Detail    string     `gorm:"type:text" json:"detail"`
	CreatedAt time.Time  `json:"created_at"`
	SeenAt    *time.Time `json:"seen_at,omitempty"`
}

func (MCPAlert) TableName() string { return "mcp_alerts" }
