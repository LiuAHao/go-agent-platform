package settings

import "time"

// UserSettings 用户设置
type UserSettings struct {
	UserID               string     `json:"user_id"`
	RetentionMaxAgeDays  int        `json:"retention_max_age_days"`
	RetentionMaxMessages int        `json:"retention_max_messages"`
	RetentionMaxSizeMB   int64      `json:"retention_max_size_mb"`
	RetentionAutoClean   bool       `json:"retention_auto_clean"`
	BackupEnabled        bool       `json:"backup_enabled"`
	BackupFrequency      string     `json:"backup_frequency"`
	BackupEncrypt        bool       `json:"backup_encrypt"`
	BackupMaxDays        int        `json:"backup_max_days"`
	LastBackupAt         *time.Time `json:"last_backup_at"`
}

// DefaultSettings 返回默认设置
func DefaultSettings(userID string) UserSettings {
	return UserSettings{
		UserID:               userID,
		RetentionMaxAgeDays:  30,
		RetentionMaxMessages: 1000,
		RetentionMaxSizeMB:   500,
		RetentionAutoClean:   false,
		BackupEnabled:        false,
		BackupFrequency:      "manual",
		BackupEncrypt:        true,
		BackupMaxDays:        90,
		LastBackupAt:         nil,
	}
}
