package sync

import (
	"time"

	"go-agent-platform/internal/domain/agent"
	"go-agent-platform/internal/domain/session"
)

// EntityType 同步实体类型
type EntityType string

const (
	EntityAgent   EntityType = "agent"
	EntitySession EntityType = "session"
	EntityMessage EntityType = "message"
)

// SyncMetadata 同步元数据
type SyncMetadata struct {
	EntityType   EntityType `json:"entity_type"`
	EntityID     string     `json:"entity_id"`
	LastSyncedAt *time.Time `json:"last_synced_at"`
	IsDirty      bool       `json:"is_dirty"`
	DeletedAt    *time.Time `json:"deleted_at"`
}

// SyncPullRequest 拉取请求
type SyncPullRequest struct {
	LastSyncAt time.Time `json:"last_sync_at"`
}

// SyncPullResponse 拉取响应
type SyncPullResponse struct {
	Agents   []agent.Agent     `json:"agents"`
	Sessions []session.Session `json:"sessions"`
	Messages []session.Message `json:"messages"`
}

// SyncPushRequest 推送请求
type SyncPushRequest struct {
	Agents   []agent.Agent     `json:"agents"`
	Sessions []session.Session `json:"sessions"`
	Messages []session.Message `json:"messages"`
}

// SyncPushResponse 推送响应
type SyncPushResponse struct {
	Accepted []SyncAccepted `json:"accepted"`
	Rejected []SyncRejected `json:"rejected"`
}

// SyncAccepted 接受的同步项
type SyncAccepted struct {
	EntityType EntityType `json:"entity_type"`
	EntityID   string     `json:"entity_id"`
	SyncedAt   time.Time  `json:"synced_at"`
}

// SyncRejected 拒绝的同步项
type SyncRejected struct {
	EntityType EntityType `json:"entity_type"`
	EntityID   string     `json:"entity_id"`
	Reason     string     `json:"reason"`
}

// SyncStatus 同步状态
type SyncStatus struct {
	LastSyncAt     *time.Time `json:"last_sync_at"`
	PendingChanges int        `json:"pending_changes"`
	IsSyncing      bool       `json:"is_syncing"`
}

// SyncManager 同步管理器接口
type SyncManager interface {
	// Pull 从云端拉取配置
	Pull(lastSyncAt time.Time) (*SyncPullResponse, error)

	// Push 推送本地变更到云端
	Push(req *SyncPushRequest) (*SyncPushResponse, error)

	// GetStatus 获取同步状态
	GetStatus() (*SyncStatus, error)

	// MarkDirty 标记实体为待同步
	MarkDirty(entityType EntityType, entityID string) error

	// MarkSynced 标记实体已同步
	MarkSynced(entityType EntityType, entityID string) error

	// GetDirtyEntities 获取待同步的实体
	GetDirtyEntities() ([]SyncMetadata, error)
}
