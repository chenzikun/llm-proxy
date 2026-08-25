package entity

// AutelMessage 统一的消息结构，用于数据回流
type AutelMessage struct {
	ID        string    `json:"_id"`
	SessionID string    `json:"session_id"`
	TokenName string    `json:"token_name"`
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	CreatedAt int64     `json:"created_at"`
}
