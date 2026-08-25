package common

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/songquanpeng/one-api/common/config"
	"github.com/songquanpeng/one-api/common/logger"
)

func LogQuota(quota int64) string {
	if config.DisplayInCurrencyEnabled {
		return fmt.Sprintf("＄%.6f 额度", float64(quota)/config.QuotaPerUnit)
	} else {
		return fmt.Sprintf("%d 点额度", quota)
	}
}

func Timer(c context.Context, name string) func() {
	start := time.Now()
	return func() {
		tc := time.Since(start)
		logger.Infof(c, "[%s] time cost = %v", name, tc)
	}
}

func IsValidUUIDv4(str string) bool {
	// 1. 移除连字符（支持两种格式）
	// 标准格式: 880cf795-d73e-4319-a9cc-65f15e14b040 (36字符)
	// 紧凑格式: 8948e44366524e6ab09a024e7fd13862 (32字符)
	cleanStr := strings.ReplaceAll(str, "-", "")

	// 2. 检查长度是否为32
	if len(cleanStr) != 32 {
		return false
	}

	// 3. 添加中划线转换为标准 UUID 格式
	uuidStr := cleanStr[0:8] + "-" + cleanStr[8:12] + "-" + cleanStr[12:16] + "-" + cleanStr[16:20] + "-" + cleanStr[20:]

	// 4. 解析并验证
	u, err := uuid.Parse(uuidStr)
	if err != nil {
		return false
	}

	// 5. 验证是否为 v4
	return u.Version() == 4
}
