-- 为 logs 表添加 session_id 字段和索引
-- 适用于: MySQL, PostgreSQL, SQLite
USE llm_proxy;
-- MySQL 语法
ALTER TABLE logs ADD COLUMN session_id VARCHAR(255) DEFAULT '' COMMENT 'Session ID from X-Session-ID header';
CREATE INDEX idx_session_id ON logs(session_id);

