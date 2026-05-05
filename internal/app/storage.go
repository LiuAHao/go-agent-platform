package app

import (
	"errors"
	"time"
)

// StorageStats 存储统计
type StorageStats struct {
	ChatMessages   int64 `json:"chat_messages"`
	ChatSizeMB     int64 `json:"chat_size_mb"`
	SkillsCount    int   `json:"skills_count"`
	SkillsSizeMB   int64 `json:"skills_size_mb"`
	CacheSizeMB    int64 `json:"cache_size_mb"`
	DatabaseSizeMB int64 `json:"database_size_mb"`
	TotalSizeMB    int64 `json:"total_size_mb"`
}

// RetentionPolicy 保留策略
type RetentionPolicy struct {
	MaxAgeDays  int   `json:"max_age_days"`
	MaxMessages int   `json:"max_messages"`
	MaxSizeMB   int64 `json:"max_size_mb"`
	AutoClean   bool  `json:"auto_clean"`
}

// CleanupResult 清理结果
type CleanupResult struct {
	DeletedSessions int   `json:"deleted_sessions"`
	DeletedMessages int   `json:"deleted_messages"`
	FreedBytes      int64 `json:"freed_bytes"`
}

// BackupSettings 备份设置
type BackupSettings struct {
	Enabled       bool       `json:"enabled"`
	Frequency     string     `json:"frequency"`      // realtime/daily/manual
	Encrypt       bool       `json:"encrypt"`
	MaxBackupDays int        `json:"max_backup_days"`
	LastBackupAt  *time.Time `json:"last_backup_at"`
}

// BackupStatus 备份状态
type BackupStatus struct {
	IsSyncing      bool       `json:"is_syncing"`
	LastSyncAt     *time.Time `json:"last_sync_at"`
	PendingChanges int        `json:"pending_changes"`
}

// GetStorageStats 获取存储统计
func (a *Application) GetStorageStats(userID string) (StorageStats, error) {
	// 获取聊天记录统计
	sessions, _ := a.Store.ListSessionsByAgent(userID, "")
	var totalMessages int64
	for _, sess := range sessions {
		messages, _ := a.Store.ListMessages(sess.ID)
		totalMessages += int64(len(messages))
	}

	// 获取 Skill 统计
	skills, _ := a.Store.ListUserSkills(userID)
	platformSkills, _ := a.Store.ListPlatformSkills()
	skillsCount := len(skills) + len(platformSkills)

	// 简化计算：每条消息约 1KB，每个 Skill 约 50KB
	chatSizeMB := totalMessages / 1024
	skillsSizeMB := int64(skillsCount) * 50 / 1024

	// 获取数据库大小（简化）
	databaseSizeMB := int64(8) // 假设 8MB

	// 获取缓存大小（简化）
	cacheSizeMB := int64(12) // 假设 12MB

	totalSizeMB := chatSizeMB + skillsSizeMB + cacheSizeMB + databaseSizeMB

	return StorageStats{
		ChatMessages:   totalMessages,
		ChatSizeMB:     chatSizeMB,
		SkillsCount:    skillsCount,
		SkillsSizeMB:   skillsSizeMB,
		CacheSizeMB:    cacheSizeMB,
		DatabaseSizeMB: databaseSizeMB,
		TotalSizeMB:    totalSizeMB,
	}, nil
}

// DeleteSession 删除会话及其消息
func (a *Application) DeleteSession(userID, sessionID string) error {
	// 验证会话存在且属于用户
	sessions, _ := a.Store.ListSessionsByAgent(userID, "")
	found := false
	for _, sess := range sessions {
		if sess.ID == sessionID {
			found = true
			break
		}
	}
	if !found {
		return errors.New("session not found")
	}

	// 删除会话的所有消息
	messages, _ := a.Store.ListMessages(sessionID)
	for _, msg := range messages {
		_ = a.Store.DeleteMessage(msg.ID)
	}

	// 删除会话
	return a.Store.DeleteSession(sessionID)
}

// DeleteAgentSessions 删除 Agent 的所有会话
func (a *Application) DeleteAgentSessions(userID, agentID string) (CleanupResult, error) {
	sessions, _ := a.Store.ListSessionsByAgent(userID, agentID)
	result := CleanupResult{}

	for _, sess := range sessions {
		messages, _ := a.Store.ListMessages(sess.ID)
		for _, msg := range messages {
			_ = a.Store.DeleteMessage(msg.ID)
			result.DeletedMessages++
		}
		_ = a.Store.DeleteSession(sess.ID)
		result.DeletedSessions++
	}

	// 估算释放空间
	result.FreedBytes = int64(result.DeletedMessages) * 1024 // 每条约 1KB

	return result, nil
}

// ClearAllMessages 清空所有聊天记录
func (a *Application) ClearAllMessages(userID string) (CleanupResult, error) {
	sessions, _ := a.Store.ListSessionsByAgent(userID, "")
	result := CleanupResult{}

	for _, sess := range sessions {
		messages, _ := a.Store.ListMessages(sess.ID)
		for _, msg := range messages {
			_ = a.Store.DeleteMessage(msg.ID)
			result.DeletedMessages++
		}
		_ = a.Store.DeleteSession(sess.ID)
		result.DeletedSessions++
	}

	// 估算释放空间
	result.FreedBytes = int64(result.DeletedMessages) * 1024

	return result, nil
}

// ExecuteCleanup 执行清理
func (a *Application) ExecuteCleanup(userID string) (CleanupResult, error) {
	// 获取保留策略
	policy, _ := a.GetRetentionPolicy(userID)

	if policy.MaxAgeDays <= 0 && policy.MaxMessages <= 0 {
		// 没有设置策略，不清理
		return CleanupResult{}, nil
	}

	sessions, _ := a.Store.ListSessionsByAgent(userID, "")
	result := CleanupResult{}

	for _, sess := range sessions {
		messages, _ := a.Store.ListMessages(sess.ID)

		// 按天数清理
		if policy.MaxAgeDays > 0 {
			cutoff := time.Now().AddDate(0, 0, -policy.MaxAgeDays)
			for _, msg := range messages {
				if msg.CreatedAt.Before(cutoff) {
					_ = a.Store.DeleteMessage(msg.ID)
					result.DeletedMessages++
				}
			}
		}

		// 重新获取消息计数
		remainingMessages, _ := a.Store.ListMessages(sess.ID)

		// 如果会话没有消息了，删除会话
		if len(remainingMessages) == 0 {
			_ = a.Store.DeleteSession(sess.ID)
			result.DeletedSessions++
		}
	}

	// 估算释放空间
	result.FreedBytes = int64(result.DeletedMessages) * 1024

	return result, nil
}

// GetRetentionPolicy 获取保留策略
func (a *Application) GetRetentionPolicy(userID string) (RetentionPolicy, error) {
	// 从数据库获取用户的保留策略
	userSettings, err := a.Store.GetUserSettings(userID)
	if err != nil {
		// 返回默认策略
		return RetentionPolicy{
			MaxAgeDays:  30,
			MaxMessages: 1000,
			MaxSizeMB:   500,
			AutoClean:   false,
		}, nil
	}

	return RetentionPolicy{
		MaxAgeDays:  userSettings.RetentionMaxAgeDays,
		MaxMessages: userSettings.RetentionMaxMessages,
		MaxSizeMB:   userSettings.RetentionMaxSizeMB,
		AutoClean:   userSettings.RetentionAutoClean,
	}, nil
}

// UpdateRetentionPolicy 更新保留策略
func (a *Application) UpdateRetentionPolicy(userID string, policy RetentionPolicy) error {
	userSettings, _ := a.Store.GetUserSettings(userID)
	userSettings.RetentionMaxAgeDays = policy.MaxAgeDays
	userSettings.RetentionMaxMessages = policy.MaxMessages
	userSettings.RetentionMaxSizeMB = policy.MaxSizeMB
	userSettings.RetentionAutoClean = policy.AutoClean
	return a.Store.SaveUserSettings(userSettings)
}

// GetBackupSettings 获取备份设置
func (a *Application) GetBackupSettings(userID string) (BackupSettings, error) {
	userSettings, err := a.Store.GetUserSettings(userID)
	if err != nil {
		// 返回默认设置
		return BackupSettings{
			Enabled:       false,
			Frequency:     "manual",
			Encrypt:       true,
			MaxBackupDays: 90,
			LastBackupAt:  nil,
		}, nil
	}

	return BackupSettings{
		Enabled:       userSettings.BackupEnabled,
		Frequency:     userSettings.BackupFrequency,
		Encrypt:       userSettings.BackupEncrypt,
		MaxBackupDays: userSettings.BackupMaxDays,
		LastBackupAt:  userSettings.LastBackupAt,
	}, nil
}

// UpdateBackupSettings 更新备份设置
func (a *Application) UpdateBackupSettings(userID string, backupSettings BackupSettings) error {
	userSettings, _ := a.Store.GetUserSettings(userID)
	userSettings.BackupEnabled = backupSettings.Enabled
	userSettings.BackupFrequency = backupSettings.Frequency
	userSettings.BackupEncrypt = backupSettings.Encrypt
	userSettings.BackupMaxDays = backupSettings.MaxBackupDays
	return a.Store.SaveUserSettings(userSettings)
}

// TriggerBackup 触发备份
func (a *Application) TriggerBackup(userID string) error {
	userSettings, _ := a.Store.GetUserSettings(userID)
	if !userSettings.BackupEnabled {
		return errors.New("backup is not enabled")
	}

	// 获取所有会话和消息
	sessions, _ := a.Store.ListSessionsByAgent(userID, "")
	allMessages := make(map[string][]interface{})

	for _, sess := range sessions {
		messages, _ := a.Store.ListMessages(sess.ID)
		msgList := make([]interface{}, 0, len(messages))
		for _, msg := range messages {
			msgList = append(msgList, map[string]any{
				"id":         msg.ID,
				"session_id": msg.SessionID,
				"role":       msg.Role,
				"content":    msg.Content,
				"created_at": msg.CreatedAt,
			})
		}
		allMessages[sess.ID] = msgList
	}

	// TODO: 实际的备份逻辑
	// 1. 序列化数据
	// 2. 如果 Encrypt 为 true，加密数据
	// 3. 上传到云端

	// 更新最后备份时间
	now := time.Now().UTC()
	userSettings.LastBackupAt = &now
	_ = a.Store.SaveUserSettings(userSettings)

	return nil
}

// RestoreBackup 恢复备份
func (a *Application) RestoreBackup(userID string) error {
	// TODO: 实际的恢复逻辑
	// 1. 从云端下载备份
	// 2. 如果加密，解密数据
	// 3. 恢复到本地数据库

	return errors.New("backup restore not implemented yet")
}

// GetBackupStatus 获取备份状态
func (a *Application) GetBackupStatus(userID string) (BackupStatus, error) {
	userSettings, _ := a.Store.GetUserSettings(userID)

	return BackupStatus{
		IsSyncing:      false,
		LastSyncAt:     userSettings.LastBackupAt,
		PendingChanges: 0, // TODO: 计算待同步的变更数
	}, nil
}
