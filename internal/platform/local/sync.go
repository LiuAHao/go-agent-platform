package local

import (
	"database/sql"
	"fmt"
	"time"

	"go-agent-platform/internal/domain/sync"
)

// SyncManager 本地同步管理器实现
type SyncManager struct {
	db *sql.DB
}

// NewSyncManager 创建同步管理器
func NewSyncManager(db *sql.DB) *SyncManager {
	return &SyncManager{db: db}
}

// MarkDirty 标记实体为待同步
func (sm *SyncManager) MarkDirty(entityType sync.EntityType, entityID string) error {
	now := time.Now().UTC()
	_, err := sm.db.Exec(`
		INSERT INTO sync_metadata (entity_type, entity_id, is_dirty, last_synced_at)
		VALUES (?, ?, 1, NULL)
		ON CONFLICT (entity_type, entity_id) DO UPDATE SET is_dirty = 1
	`, string(entityType), entityID, now)
	if err != nil {
		return fmt.Errorf("mark dirty: %w", err)
	}
	return nil
}

// MarkSynced 标记实体已同步
func (sm *SyncManager) MarkSynced(entityType sync.EntityType, entityID string) error {
	now := time.Now().UTC()
	_, err := sm.db.Exec(`
		INSERT INTO sync_metadata (entity_type, entity_id, is_dirty, last_synced_at)
		VALUES (?, ?, 0, ?)
		ON CONFLICT (entity_type, entity_id) DO UPDATE SET is_dirty = 0, last_synced_at = ?
	`, string(entityType), entityID, now, now)
	if err != nil {
		return fmt.Errorf("mark synced: %w", err)
	}
	return nil
}

// MarkDeleted 标记实体为已删除
func (sm *SyncManager) MarkDeleted(entityType sync.EntityType, entityID string) error {
	now := time.Now().UTC()
	_, err := sm.db.Exec(`
		UPDATE sync_metadata SET deleted_at = ?, is_dirty = 1
		WHERE entity_type = ? AND entity_id = ?
	`, now, string(entityType), entityID)
	if err != nil {
		return fmt.Errorf("mark deleted: %w", err)
	}
	return nil
}

// GetDirtyEntities 获取待同步的实体
func (sm *SyncManager) GetDirtyEntities() ([]sync.SyncMetadata, error) {
	rows, err := sm.db.Query(`
		SELECT entity_type, entity_id, last_synced_at, is_dirty, deleted_at
		FROM sync_metadata
		WHERE is_dirty = 1
	`)
	if err != nil {
		return nil, fmt.Errorf("get dirty entities: %w", err)
	}
	defer rows.Close()

	var items []sync.SyncMetadata
	for rows.Next() {
		var m sync.SyncMetadata
		var entityType string
		var lastSyncedAt, deletedAt sql.NullTime
		if err := rows.Scan(&entityType, &m.EntityID, &lastSyncedAt, &m.IsDirty, &deletedAt); err != nil {
			return nil, fmt.Errorf("scan sync metadata: %w", err)
		}
		m.EntityType = sync.EntityType(entityType)
		if lastSyncedAt.Valid {
			m.LastSyncedAt = &lastSyncedAt.Time
		}
		if deletedAt.Valid {
			m.DeletedAt = &deletedAt.Time
		}
		items = append(items, m)
	}
	return items, nil
}

// GetLastSyncAt 获取最后同步时间
func (sm *SyncManager) GetLastSyncAt() (*time.Time, error) {
	var lastSyncAt sql.NullTime
	err := sm.db.QueryRow(`
		SELECT MAX(last_synced_at) FROM sync_metadata
	`).Scan(&lastSyncAt)
	if err != nil {
		return nil, fmt.Errorf("get last sync at: %w", err)
	}
	if lastSyncAt.Valid {
		return &lastSyncAt.Time, nil
	}
	return nil, nil
}

// GetPendingChangesCount 获取待同步变更数量
func (sm *SyncManager) GetPendingChangesCount() (int, error) {
	var count int
	err := sm.db.QueryRow(`
		SELECT COUNT(*) FROM sync_metadata WHERE is_dirty = 1
	`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("get pending changes count: %w", err)
	}
	return count, nil
}

// Pull 从云端拉取配置 (本地实现，需要对接云端 API)
func (sm *SyncManager) Pull(lastSyncAt time.Time) (*sync.SyncPullResponse, error) {
	// TODO: 实现云端 API 调用
	// 这里返回空响应，实际需要调用云端 API
	return &sync.SyncPullResponse{}, nil
}

// Push 推送本地变更到云端 (本地实现，需要对接云端 API)
func (sm *SyncManager) Push(req *sync.SyncPushRequest) (*sync.SyncPushResponse, error) {
	// TODO: 实现云端 API 调用
	// 这里返回空响应，实际需要调用云端 API
	return &sync.SyncPushResponse{}, nil
}

// GetStatus 获取同步状态
func (sm *SyncManager) GetStatus() (*sync.SyncStatus, error) {
	lastSyncAt, err := sm.GetLastSyncAt()
	if err != nil {
		return nil, err
	}

	pendingChanges, err := sm.GetPendingChangesCount()
	if err != nil {
		return nil, err
	}

	return &sync.SyncStatus{
		LastSyncAt:     lastSyncAt,
		PendingChanges: pendingChanges,
		IsSyncing:      false,
	}, nil
}
