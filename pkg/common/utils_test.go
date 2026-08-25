package common

import (
	"testing"
)

func TestIsValidUUIDv4(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// 有效的 UUID v4 - 标准格式（带连字符）
		{
			name:     "标准格式 UUID v4",
			input:    "880cf795-d73e-4319-a9cc-65f15e14b040",
			expected: true,
		},
		// 有效的 UUID v4 - 紧凑格式（不带连字符）
		{
			name:     "紧凑格式 UUID v4",
			input:    "8948e44366524e6ab09a024e7fd13862",
			expected: true,
		},
		// 另一个标准格式
		{
			name:     "另一个标准格式 UUID v4",
			input:    "550e8400-e29b-41d4-a716-446655440000",
			expected: true,
		},
		// 另一个紧凑格式
		{
			name:     "另一个紧凑格式 UUID v4",
			input:    "550e8400e29b41d4a716446655440000",
			expected: true,
		},
		// 无效：不是 UUID v4（是 v1）
		{
			name:     "UUID v1（不是 v4）",
			input:    "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
			expected: false,
		},
		// 无效：长度不对
		{
			name:     "长度不足",
			input:    "880cf795-d73e-4319-a9cc",
			expected: false,
		},
		// 无效：不是十六进制
		{
			name:     "包含非十六进制字符",
			input:    "880cf795-d73e-4319-a9cc-65f15e14b04g",
			expected: false,
		},
		// 无效：空字符串
		{
			name:     "空字符串",
			input:    "",
			expected: false,
		},
		// 无效：随机字符串
		{
			name:     "随机字符串",
			input:    "test123",
			expected: false,
		},
		// 无效：只是数字
		{
			name:     "纯数字",
			input:    "12345678901234567890123456789012",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidUUIDv4(tt.input)
			if result != tt.expected {
				t.Errorf("IsValidUUIDv4(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}




