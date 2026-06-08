package types

import (
	"time"

	"gorm.io/gorm"
)

// OpenAPIClientStatus enumerates partner credential lifecycle states.
type OpenAPIClientStatus string

const (
	OpenAPIClientStatusActive  OpenAPIClientStatus = "active"
	OpenAPIClientStatusRevoked OpenAPIClientStatus = "revoked"
)

// OpenAPIClient is a tenant-scoped partner credential with KB whitelist.
type OpenAPIClient struct {
	ID           string              `json:"id"             gorm:"type:varchar(36);primaryKey"`
	TenantID     uint64              `json:"tenant_id"      gorm:"not null;index"`
	Name         string              `json:"name"           gorm:"type:varchar(128);not null"`
	APIKeyHash   string              `json:"-"              gorm:"column:api_key_hash;type:varchar(64);not null"`
	AllowedKBIDs StringArray         `json:"allowed_kb_ids" gorm:"column:allowed_kb_ids;type:jsonb;not null"`
	Status       OpenAPIClientStatus `json:"status"         gorm:"type:varchar(20);not null;default:active"`
	CreatedAt    time.Time           `json:"created_at"`
	UpdatedAt    time.Time           `json:"updated_at"`
	DeletedAt    gorm.DeletedAt      `json:"-" gorm:"index"`
}

func (OpenAPIClient) TableName() string { return "open_api_clients" }

// OpenAPIUserMapping links an external partner user to an internal shadow user ID.
type OpenAPIUserMapping struct {
	ID             uint64         `json:"id"               gorm:"primaryKey;autoIncrement"`
	TenantID       uint64         `json:"tenant_id"        gorm:"not null;index"`
	ClientID       string         `json:"client_id"        gorm:"type:varchar(36);not null;index"`
	ExternalUserID string         `json:"external_user_id" gorm:"type:varchar(255);not null"`
	InternalUserID string         `json:"internal_user_id" gorm:"type:varchar(320);not null;index"`
	DisplayName    string         `json:"display_name,omitempty" gorm:"type:varchar(255)"`
	FirstSeenAt    time.Time      `json:"first_seen_at"`
	LastSeenAt     time.Time      `json:"last_seen_at"`
	Metadata       JSONMap        `json:"metadata,omitempty" gorm:"type:jsonb"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`
}

func (OpenAPIUserMapping) TableName() string { return "open_api_user_mappings" }

// OpenAPISessionMapping links a partner session identifier to a WeKnora session.
type OpenAPISessionMapping struct {
	ID                uint64         `json:"id"                  gorm:"primaryKey;autoIncrement"`
	TenantID          uint64         `json:"tenant_id"           gorm:"not null;index"`
	ClientID          string         `json:"client_id"           gorm:"type:varchar(36);not null;index"`
	ExternalUserID    string         `json:"external_user_id"    gorm:"type:varchar(255);not null"`
	ExternalSessionID string         `json:"external_session_id" gorm:"type:varchar(255);not null"`
	InternalSessionID string         `json:"internal_session_id" gorm:"type:varchar(36);not null;index"`
	KnowledgeBaseID   string         `json:"knowledge_base_id,omitempty" gorm:"type:varchar(36)"`
	CreatedAt         time.Time      `json:"created_at"`
	LastActiveAt      time.Time      `json:"last_active_at"`
	DeletedAt         gorm.DeletedAt `json:"-" gorm:"index"`
}

func (OpenAPISessionMapping) TableName() string { return "open_api_session_mappings" }

// OpenAPIChatMode selects the QA pipeline for partner chat.
const (
	// OpenAPIChatModeWikiQA uses the built-in wiki researcher agent (UI「维基问答」).
	OpenAPIChatModeWikiQA = "wiki-qa"
	// OpenAPIChatModeRAGQA uses classic chunk retrieval (KnowledgeQA).
	OpenAPIChatModeRAGQA = "rag-qa"
)

// OpenAPIChatRequest is the partner-facing chat payload.
type OpenAPIChatRequest struct {
	ExternalUserID    string `json:"external_user_id" binding:"required"`
	ExternalSessionID string `json:"external_session_id"`
	SessionID         string `json:"session_id"`
	KnowledgeBaseID   string `json:"knowledge_base_id" binding:"required"`
	Query             string `json:"query" binding:"required"`
	// Mode selects the QA pipeline. Defaults to wiki-qa when omitted.
	Mode   string `json:"mode"`
	Stream bool   `json:"stream"`
}

// OpenAPIReference is a slim citation entry aligned with the web UI docInfo panel.
// Full chunk content is intentionally omitted; use content_preview for the collapsed snippet.
type OpenAPIReference struct {
	ID                string  `json:"id"`
	KnowledgeID       string  `json:"knowledge_id,omitempty"`
	KnowledgeBaseID   string  `json:"knowledge_base_id,omitempty"`
	KnowledgeTitle    string  `json:"knowledge_title,omitempty"`
	KnowledgeFilename string  `json:"knowledge_filename,omitempty"`
	ChunkIndex        int     `json:"chunk_index,omitempty"`
	Score             float64 `json:"score,omitempty"`
	ChunkType         string  `json:"chunk_type,omitempty"`
	ContentPreview    string  `json:"content_preview,omitempty"`
}

// OpenAPIChatResponse is the non-streaming chat result.
type OpenAPIChatResponse struct {
	SessionID         string               `json:"session_id"`
	ExternalSessionID string               `json:"external_session_id,omitempty"`
	Answer            string               `json:"answer"`
	IsFallback        bool                 `json:"is_fallback,omitempty"`
	References        []OpenAPIReference   `json:"references,omitempty"`
}

// CreateOpenAPIClientRequest creates a partner credential.
type CreateOpenAPIClientRequest struct {
	Name         string   `json:"name" binding:"required"`
	AllowedKBIDs []string `json:"allowed_kb_ids" binding:"required"`
}

// CreateOpenAPIClientResponse returns the plaintext key exactly once.
type CreateOpenAPIClientResponse struct {
	Client *OpenAPIClient `json:"client"`
	APIKey string         `json:"api_key"`
}
