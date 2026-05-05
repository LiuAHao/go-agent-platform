-- 用户设置表
CREATE TABLE IF NOT EXISTS user_settings (
    user_id VARCHAR PRIMARY KEY,
    retention_max_age_days INTEGER NOT NULL DEFAULT 30,
    retention_max_messages INTEGER NOT NULL DEFAULT 1000,
    retention_max_size_mb BIGINT NOT NULL DEFAULT 500,
    retention_auto_clean BOOLEAN NOT NULL DEFAULT FALSE,
    backup_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    backup_frequency VARCHAR NOT NULL DEFAULT 'manual',
    backup_encrypt BOOLEAN NOT NULL DEFAULT TRUE,
    backup_max_days INTEGER NOT NULL DEFAULT 90,
    last_backup_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_user_settings_user_id ON user_settings(user_id);
