package php

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
	_ "time/tzdata"

	"golang.org/x/crypto/bcrypt"
)

var shanghai = mustLocation("Asia/Shanghai")

func mustLocation(name string) *time.Location {
	location, err := time.LoadLocation(name)
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}

	return location
}

// Now 返回 PHP 的 date('Y-m-d H:i:s')（Asia/Shanghai）。
func Now() string {
	return time.Now().In(shanghai).Format("2006-01-02 15:04:05")
}

// Format 格式化时间（PHP Y-m-d H:i:s）。
func Format(value time.Time) string {
	return value.In(shanghai).Format("2006-01-02 15:04:05")
}

// Sha256Hex 返回小写 SHA-256 十六进制（纹理 hash）。
func Sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// VerifyPassword 兼容 PHP password_verify，包括 $2y$ 前缀。
func VerifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// HashPassword 生成 PHP 风格的 bcrypt（$2y$ 前缀）。
func HashPassword(password string, cost int) (string, error) {
	if cost <= 0 {
		cost = bcrypt.DefaultCost
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", err
	}

	// Go 生成 $2a$；PHP 使用 $2y$，两者算法相同，统一前缀便于兼容。
	return "$2y$" + strings.TrimPrefix(string(hashed), "$2a$")[0:], nil
}
